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
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
	"github.com/QwikByte/mc-server-manager/internal/master/node"
)

const (
	listTimeout = 30 * time.Second
	maxError    = 4000
)

// Kind is a type of task, e.g. backup jobs. Its settings are JSON.
type Kind interface {
	// Check validates the settings of a task and returns them normalized.
	Check(settings json.RawMessage) (json.RawMessage, error)
	// Lead is how long before its scheduled time a task starts, e.g. to warn players.
	Lead(settings json.RawMessage) time.Duration
	// Run runs a task for its scheduled time at on the servers that servers returns.
	Run(ctx context.Context, t Task, servers Servers, at time.Time) error
}

// Servers returns the target servers of a task in their current state. Its error names the
// targets that couldn't be reached; the others are returned anyway.
type Servers func(ctx context.Context) ([]Server, error)

// Server is a target server of a task.
type Server struct {
	*mcsmv1.Server
	NodeID   string
	NodeName string
}

// Wrap names the server in an error from an operation on it.
func (s Server) Wrap(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s on %s: %s", s.GetName(), s.NodeName, status.Convert(err).Message())
}

// Nodes provides the nodes and connections to their agents.
type Nodes interface {
	Get(ctx context.Context, id string) (node.Node, error)
	Conn(ctx context.Context, id string) (grpc.ClientConnInterface, error)
}

type Service struct {
	db    *sql.DB
	nodes Nodes
	kinds map[string]Kind

	mu      sync.Mutex
	ctx     context.Context // ends the runs when the master stops
	slots   map[string]*slot
	running map[string]bool
	wake    chan struct{}
}

// slot is the next run of an enabled task.
type slot struct {
	schedule Schedule
	lead     time.Duration
	at       time.Time
}

func NewService(db *sql.DB, nodes Nodes, kinds map[string]Kind) *Service {
	return &Service{
		db: db, nodes: nodes, kinds: kinds, ctx: context.Background(),
		slots: map[string]*slot{}, running: map[string]bool{}, wake: make(chan struct{}, 1),
	}
}

// Start runs the enabled tasks at their times until ctx is done. Runs that were missed
// while the master was down are skipped.
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
			if !s.run(id, sl.at) {
				slog.Warn("skipped a scheduled run, the previous one is still running", "task", id)
			}
			after := sl.at
			if now.After(after) {
				after = now
			}
			sl.at = sl.schedule.Next(after)
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
			s.slots[t.ID] = &slot{schedule: t.Schedule, lead: k.Lead(t.Settings), at: at}
		}
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
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

// RunNow runs a task right away, after its lead time if it has one, e.g. to warn players.
func (s *Service) RunNow(ctx context.Context, kind, id string) (Task, error) {
	t, err := s.Get(ctx, kind, id)
	if err != nil {
		return t, err
	}
	s.mu.Lock()
	started := s.run(t.ID, time.Now().Add(s.kinds[kind].Lead(t.Settings)))
	s.mu.Unlock()
	if !started {
		return t, httpapi.Errorf(http.StatusConflict, "%s is running already.", t.Name)
	}
	t.Running = true
	return t, nil
}

// run starts a run in the background unless the task is running already. The caller holds s.mu.
func (s *Service) run(id string, at time.Time) bool {
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
		s.execute(ctx, id, at)
	}()
	return true
}

// execute runs the current version of a task and records the outcome.
func (s *Service) execute(ctx context.Context, id string, at time.Time) {
	t, err := s.Get(ctx, "", id)
	if errors.Is(err, errNotFound) {
		return // deleted meanwhile
	}
	if err == nil {
		err = s.kinds[t.kind].Run(ctx, t, s.servers(t.Targets), at)
	}
	var msg string
	if err != nil {
		if msg = err.Error(); len(msg) > maxError {
			msg = msg[:maxError]
		}
		slog.Warn("scheduled task failed", "task", t.Name, "err", msg)
	}
	if _, err := s.db.ExecContext(context.WithoutCancel(ctx), `UPDATE tasks SET last_run_at = ?, last_error = ? WHERE id = ?`,
		at.Unix(), msg, id); err != nil {
		slog.Error("record the run of a task", "task", t.Name, "err", err)
	}
}

// servers returns a function that finds the servers of the targets on their nodes.
func (s *Service) servers(targets []Target) Servers {
	return func(ctx context.Context) ([]Server, error) {
		byNode := map[string][]string{}
		for _, t := range targets {
			byNode[t.NodeID] = append(byNode[t.NodeID], t.ServerID)
		}
		var (
			mu      sync.Mutex
			wg      sync.WaitGroup
			servers []Server
			errs    []error
		)
		for nodeID, ids := range byNode {
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
		return servers, errors.Join(errs...)
	}
}

// nodeServers returns the servers of a node with the given IDs; an empty ID stands for all.
func (s *Service) nodeServers(ctx context.Context, nodeID string, ids []string) ([]Server, error) {
	n, err := s.nodes.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	conn, err := s.nodes.Conn(ctx, nodeID)
	var res *mcsmv1.ListServersResponse
	if err == nil {
		res, err = mcsmv1.NewServerServiceClient(conn).ListServers(ctx, &mcsmv1.ListServersRequest{})
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %s", n.Name, status.Convert(err).Message())
	}
	var servers []Server
	for _, srv := range res.GetServers() {
		if slices.Contains(ids, "") || slices.Contains(ids, srv.GetId()) {
			servers = append(servers, Server{srv, n.ID, n.Name})
		}
	}
	for _, id := range ids {
		if id != "" && !slices.ContainsFunc(servers, func(srv Server) bool { return srv.GetId() == id }) {
			err = errors.Join(err, fmt.Errorf("%s: the server %s no longer exists", n.Name, id))
		}
	}
	return servers, err
}
