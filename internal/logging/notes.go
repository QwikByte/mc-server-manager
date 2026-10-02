package logging

import (
	"context"
	"log/slog"
	"sync"
)

type notesKey struct{}

type notes struct {
	mu    sync.Mutex
	attrs []slog.Attr
}

// WithNotes returns a context to which Note adds attributes, e.g. one for a request that
// is logged when it is done, and a function that returns the attributes added so far.
func WithNotes(ctx context.Context) (context.Context, func() []slog.Attr) {
	n := &notes{}
	return context.WithValue(ctx, notesKey{}, n), func() []slog.Attr {
		n.mu.Lock()
		defer n.mu.Unlock()
		return n.attrs
	}
}

// Note adds details to the log entry of what ctx belongs to, e.g. the name of a created
// server to the entry of the request. Without WithNotes, it does nothing.
func Note(ctx context.Context, attrs ...slog.Attr) {
	if n, ok := ctx.Value(notesKey{}).(*notes); ok {
		n.mu.Lock()
		defer n.mu.Unlock()
		n.attrs = append(n.attrs, attrs...)
	}
}
