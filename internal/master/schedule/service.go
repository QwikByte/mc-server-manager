package schedule

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/network"
	"github.com/QwikByte/noryx/internal/master/node"
	"github.com/QwikByte/noryx/internal/master/tag"
)

const (
	listTimeout = 30 * time.Second
	maxError    = 4000
	// maxUpcoming is the most scheduled runs that Upcoming returns.
	maxUpcoming = 1000
)

// Kind is a type of task, e.g. backup jobs. Its settings are JSON.
type Kind interface {
	// Check validates the settings of a task and returns them normalized.
	Check(settings json.RawMessage) (json.RawMessage, error)
	// Lead is how long before its scheduled time a task starts, e.g. to warn players.
	Lead(settings json.RawMessage) time.Duration
	// Run runs a task for its scheduled time at on the servers that servers returns.
	Run(ctx context.Context, t Task, servers Servers, at time.Time) error
	// Category is that of the log entries of the tasks.
	Category() slog.Attr
}

// OptionalTargets is implemented by the kinds of tasks that may target no servers, depending
// on their settings, e.g. backup jobs that only dump datastores.
type OptionalTargets interface {
	TargetsOptional(settings json.RawMessage) bool
}

// Servers returns the target servers of a task in their current state. Its error names the
// targets that couldn't be reached; the others are returned anyway.
type Servers func(ctx context.Context) ([]Server, error)

// Server is a target server of a task.
type Server struct {
	*noryxv1.Server
	NodeID   string
	NodeName string
	notes    *notes // of the run
}

// Skipped is the error of an action that left a server out rather than failed on it, e.g. a
// backup of a server without data yet. Report notes it on the run, which succeeds.
type Skipped string

func (s Skipped) Error() string { return string(s) }

// Report logs the outcome of an action of a task on the server, e.g. "Restart server", and
// returns its error with the name of the server.
func (s Server) Report(t Task, category slog.Attr, action string, err error) error {
	attrs := []any{category, "task", t.Name, logging.KeyNode, s.NodeID, logging.KeyNodeName, s.NodeName,
		logging.KeyServer, s.GetId(), logging.KeyServerName, s.GetName()}
	var skipped Skipped
	switch {
	case err == nil:
		slog.Info(action, attrs...)
		return nil
	case errors.As(err, &skipped):
		slog.Info(action+" skipped", append(attrs, "reason", string(skipped))...)
		s.notes.add(fmt.Sprintf("%s on %s: %s", s.GetName(), s.NodeName, skipped))
		return nil
	}
	msg := status.Convert(err).Message()
	slog.Warn(action+" failed", append(attrs, "err", msg)...)
	return fmt.Errorf("%s on %s: %s", s.GetName(), s.NodeName, msg)
}

// notes are what a run tells besides its errors, e.g. the servers it skipped.
type notes struct {
	mu   sync.Mutex
	list []string
}

func (n *notes) add(note string) {
	if n == nil { // a server outside of a run
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.list = append(n.list, note)
}

// Nodes provides the nodes and connections to their agents.
type Nodes interface {
	Get(ctx context.Context, id string) (node.Node, error)
	Conn(ctx context.Context, id string) (grpc.ClientConnInterface, error)
}

// Tags are the tags of servers, which tag targets resolve with.
type Tags interface {
	All(ctx context.Context) (map[tag.Server][]string, error)
}

// Networks are the networks that network targets resolve with.
type Networks interface {
	List(ctx context.Context) ([]network.Network, error)
}

type Service struct {
	db       *sql.DB
	nodes    Nodes
	tags     Tags
	networks Networks
	kinds    map[string]Kind
	busy     func(serverID string) bool

	mu      sync.Mutex
	ctx     context.Context // ends the runs when the master stops
	slots   map[string]*slot
	running map[string]bool
	wake    chan struct{}
}

// slot is the next run of an enabled task.
type slot struct {
	task Task
	lead time.Duration
	at   time.Time
}

// NewService returns the service of the tasks. busy tells which servers tasks leave out for
// now, e.g. while they move to another node.
func NewService(db *sql.DB, nodes Nodes, tags Tags, networks Networks, kinds map[string]Kind, busy func(serverID string) bool) *Service {
	return &Service{
		db: db, nodes: nodes, tags: tags, networks: networks, kinds: kinds, busy: busy, ctx: context.Background(),
		slots: map[string]*slot{}, running: map[string]bool{}, wake: make(chan struct{}, 1),
	}
}

// Start runs the enabled tasks at their times until ctx is done. Runs that were missed
// while the master was down are skipped, and tasks whose dates passed meanwhile turn off.
func (s *Service) Start(ctx context.Context) error {
	tasks, err := s.load(ctx, "", "")
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	for _, t := range tasks {
		s.plan(t)
	}
	go s.loop(ctx)
	return nil
}

func (s *Service) loop(ctx context.Context) {
	for {
		now := time.Now()
		wait := time.Minute // also notices when the clock is changed
		s.mu.Lock()
		for id, sl := range s.slots {
			if start := sl.at.Add(-sl.lead); start.After(now) {
				wait = min(wait, start.Sub(now))
				continue
			}
			if !s.run(id, sl.at, "") {
				slog.Warn("Skipped a scheduled run, the previous one is still running", s.kinds[sl.task.kind].Category(), "task", sl.task.Name)
			}
			after := sl.at
			if now.After(after) {
				after = now
			}
			if sl.at = sl.task.Schedule.Next(after); sl.at.IsZero() {
				delete(s.slots, id)
				go s.finish(s.ctx, sl.task)
			}
		}
		s.mu.Unlock()
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		case <-s.wake:
			timer.Stop()
		}
	}
}

// plan schedules the next run of an enabled task and unschedules a disabled one.
func (s *Service) plan(t Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.slots, t.ID)
	if k, ok := s.kinds[t.kind]; ok && t.Enabled {
		if at := t.Schedule.Next(time.Now()); !at.IsZero() {
			s.slots[t.ID] = &slot{task: t, lead: k.Lead(t.Settings), at: at}
		} else {
			go s.finish(s.ctx, t)
		}
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// finish turns off a task whose dates all passed, unless its schedule changed meanwhile.
func (s *Service) finish(ctx context.Context, t Task) {
	schedule, err := json.Marshal(t.Schedule)
	var res sql.Result
	if err == nil {
		res, err = s.db.ExecContext(ctx, `UPDATE tasks SET enabled = 0 WHERE id = ? AND enabled AND schedule = ?`, t.ID, schedule)
	}
	category := s.kinds[t.kind].Category()
	if err != nil {
		slog.Warn("Can't turn off a task after its last date", category, "task", t.Name, "err", err)
	} else if n, _ := res.RowsAffected(); n > 0 {
		slog.Info("Turned off a task after its last date", category, "task", t.Name)
	}
}

// Upcoming is a scheduled run of a task.
type Upcoming struct {
	TaskID string    `json:"id"`
	At     time.Time `json:"at"`
}

// Upcoming returns the scheduled runs of the enabled tasks of a kind until a time, in order,
// at most maxUpcoming.
func (s *Service) Upcoming(kind string, until time.Time) []Upcoming {
	list := []Upcoming{}
	s.mu.Lock()
	for id, sl := range s.slots {
		for at := sl.at; sl.task.kind == kind && !at.IsZero() && !at.After(until); at = sl.task.Schedule.Next(at) {
			list = append(list, Upcoming{TaskID: id, At: at})
		}
	}
	s.mu.Unlock()
	slices.SortFunc(list, func(a, b Upcoming) int { return cmp.Or(a.At.Compare(b.At), cmp.Compare(a.TaskID, b.TaskID)) })
	return list[:min(len(list), maxUpcoming)]
}

// nextRun returns when an enabled task runs next.
func (s *Service) nextRun(id string) *time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sl, ok := s.slots[id]; ok {
		at := sl.at
		return &at
	}
	return nil
}

// runningTasks returns the IDs of the tasks that are running.
func (s *Service) runningTasks() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.running)
}

// RunNow runs a task right away for a user, after its lead time if it has one, e.g. to warn
// players.
func (s *Service) RunNow(ctx context.Context, kind, id, user string) (Task, error) {
	t, err := s.Get(ctx, kind, id)
	if err != nil {
		return t, err
	}
	s.mu.Lock()
	started := s.run(t.ID, time.Now().Add(s.kinds[kind].Lead(t.Settings)), user)
	s.mu.Unlock()
	if !started {
		return t, httpapi.Errorf(http.StatusConflict, "%s is running already.", t.Name)
	}
	t.Running = true
	return t, nil
}

// run starts a run for its scheduled time in the background, by a user or by the schedule if
// user is empty, unless the task is running already. The caller holds s.mu.
func (s *Service) run(id string, at time.Time, user string) bool {
	if s.running[id] {
		return false
	}
	s.running[id] = true
	ctx := s.ctx
	go func() {
		defer func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			delete(s.running, id)
		}()
		s.execute(ctx, id, at, user)
	}()
	return true
}

// execute runs the current version of a task and records the run.
func (s *Service) execute(ctx context.Context, id string, at time.Time, user string) {
	r := Run{StartedAt: time.Now(), StartedBy: user, Outcome: Succeeded}
	t, err := s.Get(ctx, "", id)
	switch {
	case errors.Is(err, errNotFound):
		return // deleted meanwhile
	case err != nil: // e.g. as the master stops; without the task, its kind is unknown
		slog.Warn("Can't run a scheduled task", "task_id", id, "err", err)
		return
	}
	run := &notes{}
	k := s.kinds[t.kind]
	if optional, _ := k.(OptionalTargets); len(t.Targets) == 0 && (optional == nil || !optional.TargetsOptional(t.Settings)) {
		run.add("It has no targets left, e.g. as their network was deleted. Choose servers, tags or networks for it.")
	}
	err = k.Run(ctx, t, s.servers(t.Targets, run), at)
	category := k.Category()
	if err != nil {
		r.Outcome, r.Error = Failed, clip(err.Error())
		slog.Warn("Run scheduled task failed", category, "task", t.Name, "err", r.Error)
	} else {
		slog.Info("Run scheduled task", category, "task", t.Name)
	}
	slices.Sort(run.list)
	r.EndedAt, r.Note = time.Now(), clip(strings.Join(slices.Compact(run.list), "\n"))
	if err := s.record(context.WithoutCancel(ctx), id, r); err != nil {
		slog.Error("Can't record the run of a task", category, "task", t.Name, "err", err)
	}
}

// record stores a run of a task, unless the task was deleted meanwhile, and forgets the
// oldest runs beyond maxRuns.
func (s *Service) record(ctx context.Context, id string, r Run) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO task_runs (task_id, started_at, ended_at, started_by, outcome, error, note)
		SELECT id, ?, ?, ?, ?, ?, ? FROM tasks WHERE id = ?`,
		r.StartedAt.Unix(), r.EndedAt.Unix(), r.StartedBy, r.Outcome, r.Error, r.Note, id)
	if err == nil {
		_, err = s.db.ExecContext(ctx, `DELETE FROM task_runs WHERE task_id = ? AND id <= (
			SELECT id FROM task_runs WHERE task_id = ? ORDER BY id DESC LIMIT 1 OFFSET ?)`, id, id, maxRuns)
	}
	return err
}

// clip shortens the error or note of a run to what is stored.
func clip(msg string) string { return msg[:min(len(msg), maxError)] }

// servers returns a function that finds the servers of the targets on their nodes for a run
// with its notes. Tags and networks have the servers they have when it is called.
func (s *Service) servers(targets []Target, run *notes) Servers {
	return func(ctx context.Context) ([]Server, error) {
		g, err := s.groups(ctx, targets)
		if err != nil {
			return nil, fmt.Errorf("can't find the servers of tags and networks: %w", err)
		}
		var (
			mu      sync.Mutex
			wg      sync.WaitGroup
			servers []Server
			errs    []error
		)
		for nodeID, ids := range g.resolve(targets, run) {
			wg.Go(func() {
				found, err := s.nodeServers(ctx, nodeID, ids)
				mu.Lock()
				defer mu.Unlock()
				servers, errs = append(servers, found...), append(errs, err)
			})
		}
		wg.Wait()
		slices.SortFunc(servers, func(a, b Server) int {
			return cmp.Or(cmp.Compare(a.NodeName, b.NodeName), cmp.Compare(a.GetName(), b.GetName()))
		})
		for i := range servers {
			servers[i].notes = run
		}
		return servers, errors.Join(errs...)
	}
}

// nodeServers returns the servers of a node with the given IDs; an empty ID stands for all.
// A server that is named on its own must exist.
func (s *Service) nodeServers(ctx context.Context, nodeID string, ids map[string]bool) ([]Server, error) {
	n, err := s.nodes.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	conn, err := s.nodes.Conn(ctx, nodeID)
	var res *noryxv1.ListServersResponse
	if err == nil {
		res, err = noryxv1.NewServerServiceClient(conn).ListServers(ctx, &noryxv1.ListServersRequest{})
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %s", n.Name, status.Convert(err).Message())
	}
	var servers []Server
	for _, srv := range res.GetServers() {
		_, all := ids[""]
		_, listed := ids[srv.GetId()]
		switch {
		case !all && !listed:
		case s.busy(srv.GetId()):
			err = errors.Join(err, fmt.Errorf("%s: %s is moving to another node", n.Name, srv.GetName()))
		default:
			servers = append(servers, Server{Server: srv, NodeID: n.ID, NodeName: n.Name})
		}
	}
	for _, id := range slices.Sorted(maps.Keys(ids)) {
		if ids[id] && !slices.ContainsFunc(res.GetServers(), func(srv *noryxv1.Server) bool { return srv.GetId() == id }) {
			err = errors.Join(err, fmt.Errorf("%s: the server %s no longer exists", n.Name, id))
		}
	}
	return servers, err
}

// groups are what tag and network targets resolve with: the tags of servers and the networks.
type groups struct {
	tags     map[tag.Server][]string
	networks []network.Network
}

// groups loads what the tag and network targets among targets resolve with.
func (s *Service) groups(ctx context.Context, targets []Target) (g groups, err error) {
	has := func(kind string) bool {
		return slices.ContainsFunc(targets, func(t Target) bool { return t.Kind == kind })
	}
	if has(KindTag) {
		g.tags, err = s.tags.All(ctx)
	}
	if err == nil && has(KindNetwork) {
		g.networks, err = s.networks.List(ctx)
	}
	return g, err
}

// selection are servers by node: their IDs, "" for all servers of the node, and whether each
// must exist, as one that is named on its own must.
type selection map[string]map[string]bool

func (sel selection) add(nodeID, serverID string, named bool) {
	if sel[nodeID] == nil {
		sel[nodeID] = map[string]bool{}
	}
	sel[nodeID][serverID] = sel[nodeID][serverID] || named
}

func (sel selection) covers(srv tag.Server) bool {
	_, all := sel[srv.NodeID][""]
	_, listed := sel[srv.NodeID][srv.ServerID]
	return all || listed
}

// resolve returns the servers that targets name now, and notes the tags that no server has.
func (g groups) resolve(targets []Target, run *notes) selection {
	sel := selection{}
	for _, t := range targets {
		switch t.Kind {
		case KindServer:
			sel.add(t.NodeID, t.ServerID, t.ServerID != "")
		case KindTag:
			found := false
			for srv, tags := range g.tags {
				if slices.Contains(tags, t.Value) {
					sel.add(srv.NodeID, srv.ServerID, false)
					found = true
				}
			}
			if !found {
				run.add(fmt.Sprintf("No server has the tag %s.", t.Value))
			}
		case KindNetwork:
			for _, n := range g.networks {
				if n.ID != t.Value {
					continue
				}
				if t.Role != RoleServers {
					sel.add(n.Proxy.NodeID, n.Proxy.ServerID, false)
				}
				for _, b := range n.Backends {
					if t.Role != RoleProxy {
						sel.add(b.NodeID, b.ServerID, false)
					}
				}
			}
		}
	}
	return sel
}
