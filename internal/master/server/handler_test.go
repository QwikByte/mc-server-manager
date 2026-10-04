package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
)

// Once the agent deleted a server, every reference to it is removed, even if one fails or
// the request is cancelled, and the deletion succeeds.
func TestDeleteForgetsEverything(t *testing.T) {
	failing, other := &fakeRefs{err: errors.New("database is locked")}, &fakeRefs{}
	h := NewHandler(fakeNodes{}, fakeNetworks{}, nil, NewMoves(), failing, other)
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/nodes/{node}/servers/{id}", h.delete)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/nodes/n1/servers/s1", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	for _, refs := range []*fakeRefs{failing, other} {
		if refs.forgot != "n1/s1" || refs.ctxErr != nil {
			t.Errorf("forgot %q with context error %v", refs.forgot, refs.ctxErr)
		}
	}
}

type fakeNodes struct{ Nodes }

func (fakeNodes) Conn(context.Context, string) (grpc.ClientConnInterface, error) {
	return fakeConn{}, nil
}

// fakeConn is an agent that does whatever it is asked.
type fakeConn struct{ grpc.ClientConnInterface }

func (fakeConn) Invoke(context.Context, string, any, any, ...grpc.CallOption) error { return nil }

type fakeNetworks struct{ Networks }

func (fakeNetworks) CheckRemovable(context.Context, string, string) error { return nil }

type fakeRefs struct {
	References
	err    error
	forgot string
	ctxErr error
}

func (f *fakeRefs) Forget(ctx context.Context, nodeID, serverID string) error {
	f.forgot, f.ctxErr = nodeID+"/"+serverID, ctx.Err()
	return f.err
}
