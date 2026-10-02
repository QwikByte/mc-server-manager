package schedule

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const maxTargets = 200

var (
	errNotFound   = httpapi.Errorf(http.StatusNotFound, "Not found. It may have been deleted.")
	serverPattern = regexp.MustCompile(`^[a-z2-7]{26}$`)
)

// Task runs on its target servers at the times of its schedule. Its settings are those of
// its kind, e.g. what a backup job backs up.
type Task struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Enabled  bool            `json:"enabled"`
	Schedule Schedule        `json:"schedule"`
	Targets  []Target        `json:"targets"`
	Settings json.RawMessage `json:"settings"`
	LastRun  *Run            `json:"lastRun,omitempty"`
	// NextRun is the scheduled time of the next run of an enabled task.
	NextRun   *time.Time `json:"nextRun,omitempty"`
	Running   bool       `json:"running"`
	CreatedAt time.Time  `json:"createdAt"`
	kind      string
}

// Target is a server, or all servers of a node if ServerID is empty.
type Target struct {
	NodeID   string `json:"nodeId"`
	ServerID string `json:"serverId"`
}

// Run is the outcome of the latest run of a task.
type Run struct {
	At    time.Time `json:"at"`
	Error string    `json:"error,omitempty"`
}

// Input is a new or changed task.
type Input struct {
	Name     string          `json:"name"`
	Enabled  bool            `json:"enabled"`
	Schedule Schedule        `json:"schedule"`
	Targets  []Target        `json:"targets"`
	Settings json.RawMessage `json:"settings"`
}

func (s *Service) List(ctx context.Context, kind string) ([]Task, error) {
	return s.load(ctx, kind, "")
}

func (s *Service) Get(ctx context.Context, kind, id string) (Task, error) {
	tasks, err := s.load(ctx, kind, id)
	if err == nil && len(tasks) == 0 {
		err = errNotFound
	}
	if err != nil {
		return Task{}, err
	}
	return tasks[0], nil
}

func (s *Service) Create(ctx context.Context, kind string, in Input) (Task, error) {
	t, err := s.build(kind, in)
	if err != nil {
		return t, err
	}
	t.ID, t.CreatedAt = strings.ToLower(rand.Text()), time.Now()
	return s.save(ctx, t, func(tx *sql.Tx, schedule, settings []byte) (sql.Result, error) {
		return tx.ExecContext(ctx, `INSERT INTO tasks (id, kind, name, enabled, schedule, settings, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			t.ID, kind, t.Name, t.Enabled, schedule, settings, t.CreatedAt.Unix())
	})
}

func (s *Service) Update(ctx context.Context, kind, id string, in Input) (Task, error) {
	t, err := s.build(kind, in)
	if err != nil {
		return t, err
	}
	t.ID = id
	return s.save(ctx, t, func(tx *sql.Tx, schedule, settings []byte) (sql.Result, error) {
		return tx.ExecContext(ctx, `UPDATE tasks SET name = ?, enabled = ?, schedule = ?, settings = ? WHERE id = ? AND kind = ?`,
			t.Name, t.Enabled, schedule, settings, id, kind)
	})
}

// Delete removes a task. A run in progress finishes.
func (s *Service) Delete(ctx context.Context, kind, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ? AND kind = ?`, id, kind)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	s.plan(Task{ID: id}) // disabled, so it is no longer scheduled
	return nil
}

// Forget removes a deleted server from the targets of all tasks.
func (s *Service) Forget(ctx context.Context, nodeID, serverID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM task_targets WHERE node_id = ? AND server_id = ?`, nodeID, serverID)
	return err
}

// Move keeps a server that moved to another node a target of its tasks.
func (s *Service) Move(ctx context.Context, serverID, from, to string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE task_targets SET node_id = ? WHERE node_id = ? AND server_id = ?`, to, from, serverID)
	return err
}

// build validates a task and lets its kind check its settings.
func (s *Service) build(kind string, in Input) (Task, error) {
	t := Task{Name: strings.TrimSpace(in.Name), Enabled: in.Enabled, Schedule: in.Schedule, kind: kind}
	k, ok := s.kinds[kind]
	if !ok {
		return t, fmt.Errorf("unknown kind of task %q", kind)
	}
	var validTargets bool
	t.Targets, validTargets = targets(in.Targets)
	switch msg := t.Schedule.normalize(); {
	case t.Name == "" || len(t.Name) > 64:
		return t, httpapi.Errorf(http.StatusBadRequest, "Enter a name with up to 64 characters.")
	case msg != "":
		return t, httpapi.Errorf(http.StatusBadRequest, "%s", msg)
	case !validTargets:
		return t, httpapi.Errorf(http.StatusBadRequest, "Choose 1 to %d nodes or servers.", maxTargets)
	}
	var err error
	t.Settings, err = k.Check(in.Settings)
	return t, err
}

// targets removes duplicates and servers of nodes whose servers are all targets anyway.
func targets(in []Target) ([]Target, bool) {
	var out []Target
	for _, t := range in {
		wholeNode := slices.Contains(in, Target{NodeID: t.NodeID})
		switch {
		case t.NodeID == "" || (t.ServerID != "" && !serverPattern.MatchString(t.ServerID)):
			return nil, false
		case !slices.Contains(out, t) && (t.ServerID == "" || !wholeNode):
			out = append(out, t)
		}
	}
	return out, len(out) > 0 && len(out) <= maxTargets
}

// save writes a task and its targets with the statement write, and schedules it.
func (s *Service) save(ctx context.Context, t Task, write func(tx *sql.Tx, schedule, settings []byte) (sql.Result, error)) (Task, error) {
	schedule, err := json.Marshal(t.Schedule)
	if err != nil {
		return t, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return t, err
	}
	err = func() error {
		res, err := write(tx, schedule, t.Settings)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errNotFound
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM task_targets WHERE task_id = ?`, t.ID); err != nil {
			return err
		}
		for _, target := range t.Targets {
			if _, err := tx.ExecContext(ctx, `INSERT INTO task_targets (task_id, node_id, server_id) VALUES (?, ?, ?)`,
				t.ID, target.NodeID, target.ServerID); err != nil {
				return err
			}
		}
		return tx.Commit()
	}()
	if err != nil {
		return t, errors.Join(constraint(err, t.Name), tx.Rollback())
	}
	s.plan(t)
	return s.Get(ctx, t.kind, t.ID)
}

func constraint(err error, name string) error {
	switch msg := err.Error(); {
	case strings.Contains(msg, "UNIQUE"):
		return httpapi.Errorf(http.StatusConflict, "The name %q is taken already.", name)
	case strings.Contains(msg, "FOREIGN KEY"):
		return httpapi.Errorf(http.StatusBadRequest, "Choose nodes that exist.")
	}
	return err
}

// load reads the tasks of a kind, or one of them if id isn't empty.
func (s *Service) load(ctx context.Context, kind, id string) ([]Task, error) {
	// A run records its outcome before it ends, so reading which tasks run first ensures
	// that a task that isn't running shows the outcome of its latest run.
	running := s.runningTasks()
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, name, enabled, schedule, settings, last_run_at, last_error, created_at
		FROM tasks WHERE ? IN ('', kind) AND ? IN ('', id) ORDER BY name`, kind, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []Task{}
	index := map[string]int{}
	for rows.Next() {
		t := Task{Targets: []Target{}}
		var schedule, settings string
		var lastRun sql.NullInt64
		var lastError sql.NullString
		var createdAt int64
		if err := rows.Scan(&t.ID, &t.kind, &t.Name, &t.Enabled, &schedule, &settings, &lastRun, &lastError, &createdAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(schedule), &t.Schedule); err != nil {
			return nil, err
		}
		t.Settings, t.CreatedAt = json.RawMessage(settings), time.Unix(createdAt, 0)
		if lastRun.Valid {
			t.LastRun = &Run{At: time.Unix(lastRun.Int64, 0), Error: lastError.String}
		}
		t.NextRun, t.Running = s.nextRun(t.ID), running[t.ID]
		index[t.ID] = len(tasks)
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	targets, err := s.db.QueryContext(ctx, `SELECT task_id, node_id, server_id FROM task_targets WHERE ? IN ('', task_id) ORDER BY rowid`, id)
	if err != nil {
		return nil, err
	}
	defer targets.Close()
	for targets.Next() {
		var taskID string
		var t Target
		if err := targets.Scan(&taskID, &t.NodeID, &t.ServerID); err != nil {
			return nil, err
		}
		if i, ok := index[taskID]; ok {
			tasks[i].Targets = append(tasks[i].Targets, t)
		}
	}
	return tasks, targets.Err()
}
