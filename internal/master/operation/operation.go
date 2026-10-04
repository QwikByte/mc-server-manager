// Package operation runs long actions of the panel in the background, e.g. creating a server,
// which may download its image first. The panel follows their steps and progress, and learns
// how they ended, also if the browser went away meanwhile or a proxy in front of the master
// gave up waiting. Operations live in memory: the master shows those of the last hour.
package operation

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/auth"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

const (
	keep    = time.Hour // how long finished operations are listed
	maxKept = 200       // finished operations listed at most
)

// Operation is an action in progress, or one that ended within the last hour.
type Operation struct {
	ID string `json:"id"`
	// Kind tells what the operation does, e.g. server.create; the panel names it.
	Kind string `json:"kind"`
	// Subject is what it is about, e.g. the name of the server it creates.
	Subject   string `json:"subject"`
	NodeID    string `json:"nodeId,omitempty"`
	ServerID  string `json:"serverId,omitempty"`
	NetworkID string `json:"networkId,omitempty"`
	// User is who started it.
	User string `json:"user"`
	// Steps are the steps it takes, and Step the index of the one it is at, or failed at.
	Steps []string `json:"steps"`
	Step  int      `json:"step"`
	// Done and Total measure the current step in Unit, bytes or servers; Total is 0 if unknown.
	Done       int64      `json:"done"`
	Total      int64      `json:"total"`
	Unit       string     `json:"unit,omitempty"`
	Error      string     `json:"error,omitempty"`
	Result     any        `json:"result,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

// Spec describes an operation to start.
type Spec struct {
	Kind, Subject               string
	NodeID, ServerID, NetworkID string
	// Steps it plans; steps it reports besides are added.
	Steps []string
	// Status answers a request whose operation ended right away, e.g. 201 Created.
	Status  int
	Timeout time.Duration
	// Visible tells whether users other than the one who started it may see it.
	Visible func(access.Grants) bool
	// Category is that of its entry in the log, if it ends after the request was answered.
	Category slog.Attr
}

// Task does the work of an operation. Its context carries the operation, to report progress.
type Task func(ctx context.Context) (result any, err error)

type entry struct {
	Operation
	userID   int64
	visible  func(access.Grants) bool
	category slog.Attr
	err      error
	answered bool // the request was answered before the operation ended
	done     chan struct{}
}

// Operations are the operations in progress and those that ended within the last hour.
type Operations struct {
	quick time.Duration
	mu    sync.Mutex
	ops   []*entry // oldest first
}

// New returns the operations. Requests whose operation ends within quick are answered like
// ordinary requests, with the result or the error; 0 always answers 202 Accepted at once.
func New(quick time.Duration) *Operations { return &Operations{quick: quick} }

// Run starts an operation and answers the request: like an ordinary request if the
// operation ends within quick, otherwise with 202 Accepted and the operation, which goes on.
func (o *Operations) Run(w http.ResponseWriter, r *http.Request, spec Spec, task Task) {
	user, _ := auth.UserFrom(r.Context())
	e := &entry{
		Operation: Operation{
			ID: strings.ToLower(rand.Text()), Kind: spec.Kind, Subject: spec.Subject, NodeID: spec.NodeID, ServerID: spec.ServerID,
			NetworkID: spec.NetworkID, User: user.Username, Steps: slices.Clone(spec.Steps), StartedAt: time.Now(),
		},
		userID: user.ID, visible: spec.Visible, category: spec.Category, done: make(chan struct{}),
	}
	o.add(e)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), spec.Timeout)
	go func() {
		defer cancel()
		result, err := task(context.WithValue(ctx, key{}, &reporter{o, e}))
		o.finish(e, result, err)
	}()

	if o.quick > 0 {
		select {
		case <-e.done:
		case <-time.After(o.quick):
		}
	}
	o.mu.Lock()
	finished, view := e.FinishedAt != nil, e.view()
	e.answered = !finished
	o.mu.Unlock()
	switch {
	case !finished:
		logging.Note(r.Context(), slog.String("operation", e.ID))
		httpapi.WriteJSON(w, http.StatusAccepted, view)
	case e.err != nil:
		httpapi.WriteError(w, r, e.err)
	case spec.Status == http.StatusNoContent:
		w.WriteHeader(http.StatusNoContent)
	default:
		httpapi.WriteJSON(w, spec.Status, e.Result)
	}
}

func (o *Operations) add(e *entry) {
	o.mu.Lock()
	defer o.mu.Unlock()
	finished := 0
	for _, old := range o.ops {
		if old.FinishedAt != nil {
			finished++
		}
	}
	o.ops = slices.DeleteFunc(o.ops, func(old *entry) bool {
		gone := old.FinishedAt != nil && (time.Since(*old.FinishedAt) > keep || finished > maxKept)
		if gone {
			finished--
		}
		return gone
	})
	o.ops = append(o.ops, e)
}

// finish records how an operation ended, and logs it if the request was answered before.
func (o *Operations) finish(e *entry, result any, err error) {
	o.mu.Lock()
	now := time.Now()
	e.FinishedAt, e.Result, e.err = &now, result, err
	if err != nil {
		e.Error = httpapi.Message(err)
	}
	answered := e.answered
	o.mu.Unlock()
	close(e.done)
	if !answered {
		return
	}
	attrs := []any{e.category, logging.KeyUser, e.User, "operation", e.ID, "kind", e.Kind, "subject", e.Subject}
	if e.NodeID != "" {
		attrs = append(attrs, logging.KeyNode, e.NodeID)
	}
	if e.ServerID != "" {
		attrs = append(attrs, logging.KeyServer, e.ServerID)
	}
	if err != nil {
		slog.Warn("Operation failed", append(attrs, "err", e.Error)...)
	} else {
		slog.Info("Operation finished", attrs...)
	}
}

// view returns a copy of an operation, while o.mu is held.
func (e *entry) view() Operation {
	op := e.Operation
	op.Steps = slices.Clone(e.Steps)
	return op
}

// List returns the operations a user may see, the newest first: those the user started,
// and those of others about what the user may see.
func (o *Operations) List(userID int64, grants access.Grants) []Operation {
	o.mu.Lock()
	defer o.mu.Unlock()
	ops := []Operation{}
	for _, e := range slices.Backward(o.ops) {
		if e.userID == userID || e.visible(grants) {
			ops = append(ops, e.view())
		}
	}
	return ops
}

// CheckIdle refuses to cut off operations in progress, e.g. by restarting the master.
func (o *Operations) CheckIdle() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, e := range o.ops {
		if e.FinishedAt == nil {
			return httpapi.Errorf(http.StatusConflict, "%s isn't finished yet. Try again when it is done.", e.Subject)
		}
	}
	return nil
}

type key struct{}

// reporter changes the progress of an operation for its task.
type reporter struct {
	o *Operations
	e *entry
}

func from(ctx context.Context) *reporter {
	r, _ := ctx.Value(key{}).(*reporter)
	return r
}

func (r *reporter) update(change func(op *Operation)) {
	r.o.mu.Lock()
	defer r.o.mu.Unlock()
	change(&r.e.Operation)
}

// Step moves the operation of a task to a step, which starts with nothing done. A step it
// didn't plan is added.
func Step(ctx context.Context, step string) {
	if r := from(ctx); r != nil {
		r.update(func(op *Operation) {
			i := slices.Index(op.Steps, step)
			if i < 0 {
				op.Steps = append(op.Steps, step)
				i = len(op.Steps) - 1
			}
			if i != op.Step {
				op.Step, op.Done, op.Total, op.Unit = i, 0, 0, ""
			}
		})
	}
}

// Count tells how much of the current step of an operation is done, and of how much, in unit.
func Count(ctx context.Context, done, total int64, unit string) {
	if r := from(ctx); r != nil {
		r.update(func(op *Operation) { op.Done, op.Total, op.Unit = done, total, unit })
	}
}

// Target names the server an operation is about once it is known, e.g. a server it created.
func Target(ctx context.Context, nodeID, serverID string) {
	if r := from(ctx); r != nil {
		r.update(func(op *Operation) { op.NodeID, op.ServerID = nodeID, serverID })
	}
}
