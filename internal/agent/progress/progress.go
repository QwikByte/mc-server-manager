// Package progress tells the master how long calls of the agent are getting on. The master
// names a call with the metadata key noryxv1.OperationKey; the call reports its steps and bytes
// through its context, and WatchProgress sends them to the master as they change.
package progress

import (
	"context"
	"io"
	"regexp"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

// interval is the least time between two messages to a watcher; changes in between are
// sent together.
const interval = 250 * time.Millisecond

var validID = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

type state struct {
	step        string
	done, total int64
}

// tracker holds the progress of a call, for the call and the watchers of its name.
type tracker struct {
	mu      sync.Mutex
	state   state
	ended   bool
	changed chan struct{} // closed and replaced on every change
	users   int           // the call and its watchers; the tracker goes when none is left
}

func (t *tracker) update(change func(*state)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	change(&t.state)
	close(t.changed)
	t.changed = make(chan struct{})
}

func (t *tracker) end() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ended = true
	close(t.changed)
	t.changed = make(chan struct{})
}

func (t *tracker) snapshot() (state, bool, <-chan struct{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state, t.ended, t.changed
}

// Registry knows the progress of the named calls in progress, and serves it.
type Registry struct {
	noryxv1.UnimplementedProgressServiceServer
	mu       sync.Mutex
	trackers map[string]*tracker
}

func NewRegistry() *Registry { return &Registry{trackers: map[string]*tracker{}} }

func (r *Registry) acquire(id string) *tracker {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.trackers[id]
	if t == nil {
		t = &tracker{changed: make(chan struct{})}
		r.trackers[id] = t
	}
	t.users++
	return t
}

func (r *Registry) release(id string, t *tracker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t.users--; t.users == 0 {
		delete(r.trackers, id)
	}
}

// Interceptor gives named calls the tracker of their name in their context.
func (r *Registry) Interceptor() grpc.ServerOption { return grpc.ChainUnaryInterceptor(r.intercept) }

func (r *Registry) intercept(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	ids := md.Get(noryxv1.OperationKey)
	if len(ids) != 1 || !validID.MatchString(ids[0]) {
		return handler(ctx, req)
	}
	t := r.acquire(ids[0])
	defer r.release(ids[0], t)
	defer t.end()
	return handler(context.WithValue(ctx, key{}, t), req)
}

// WatchProgress sends the progress of a named call until it ends, at most every interval.
func (r *Registry) WatchProgress(req *noryxv1.WatchProgressRequest, stream noryxv1.ProgressService_WatchProgressServer) error {
	id := req.GetOperationId()
	if !validID.MatchString(id) {
		return nil
	}
	t := r.acquire(id)
	defer r.release(id, t)
	// The header tells the master that the call may start, as its progress is followed now.
	if err := stream.SendHeader(metadata.MD{}); err != nil {
		return err
	}
	var sent state
	for {
		s, ended, changed := t.snapshot()
		if s != sent && s.step != "" {
			if err := stream.Send(&noryxv1.WatchProgressResponse{Step: s.step, Done: s.done, Total: s.total}); err != nil {
				return err
			}
			sent = s
		}
		if ended {
			return nil
		}
		select {
		case <-changed:
		case <-stream.Context().Done():
			return nil
		}
		select {
		case <-time.After(interval):
		case <-stream.Context().Done():
			return nil
		}
	}
}

type key struct{}

func from(ctx context.Context) *tracker {
	t, _ := ctx.Value(key{}).(*tracker)
	return t
}

// Active tells whether the progress of a call is watched, e.g. to measure what only the
// progress needs.
func Active(ctx context.Context) bool { return from(ctx) != nil }

// Step starts a step of a call, with the bytes it takes, or 0 if unknown.
func Step(ctx context.Context, step string, total int64) {
	if t := from(ctx); t != nil {
		t.update(func(s *state) { *s = state{step: step, total: total} })
	}
}

// Set sets the bytes of the current step done and in all, e.g. as a download learns its size.
func Set(ctx context.Context, done, total int64) {
	if t := from(ctx); t != nil {
		t.update(func(s *state) { s.done, s.total = done, total })
	}
}

// Reader counts what is read through it as done in the current step.
func Reader(ctx context.Context, r io.Reader) io.Reader {
	if t := from(ctx); t != nil {
		return &reader{Reader: r, t: t}
	}
	return r
}

// batch is how many bytes a reader counts before it tells; watchers only look every interval.
const batch = 1 << 20

type reader struct {
	io.Reader
	t       *tracker
	pending int64
}

func (r *reader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if r.pending += int64(n); r.pending >= batch || (err != nil && r.pending > 0) {
		read := r.pending
		r.t.update(func(s *state) { s.done += read })
		r.pending = 0
	}
	return n, err
}
