package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/database"
	"github.com/QwikByte/noryx/internal/master/node"
	"github.com/QwikByte/noryx/internal/master/operation"
	"github.com/QwikByte/noryx/internal/master/tag"
)

// Once the agent deleted a server, every reference to it is removed, even if one fails or
// the request is cancelled, and the deletion succeeds.
func TestDeleteForgetsEverything(t *testing.T) {
	failing, other := &fakeRefs{err: errors.New("database is locked")}, &fakeRefs{}
	h := NewHandler(fakeNodes{}, fakeNetworks{}, nil, nil, nil, nil, NewMoves(), nil, failing, other)
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

// An agent that doesn't know the stop timeout and the time zone yet leaves them out of its
// answer, which the panel then shows as a warning.
func TestUpdateWarnsOfOlderAgents(t *testing.T) {
	h := NewHandler(fakeNodes{conn: olderAgent{}}, fakeNetworks{}, nil, nil, nil, operation.New(time.Second), NewMoves(), nil)
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/nodes/{node}/servers/{id}", h.update)
	for body, warns := range map[string]bool{
		`{"name": "Lobby", "memoryMb": 1024, "port": 25565}`:                           false,
		`{"name": "Lobby", "memoryMb": 1024, "port": 25565, "stopTimeout": 60}`:        false,
		`{"name": "Lobby", "memoryMb": 1024, "port": 25565, "stopTimeout": 300}`:       true,
		`{"name": "Lobby", "memoryMb": 1024, "port": 25565, "timeZone": "Asia/Tokyo"}`: true,
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/nodes/n1/servers/s1", strings.NewReader(body)))
		var res struct {
			StopTimeout uint32
			TimeZone    string
			Warning     string
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", body, rec.Code, rec.Body)
		}
		if res.StopTimeout != 60 || res.TimeZone != "" || (res.Warning != "") != warns {
			t.Errorf("%s: %+v", body, res)
		}
	}
}

// Notes are kept for servers that exist, and need no restart.
func TestSetNotes(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO nodes (id, name, address, created_at) VALUES ('n1', 'n1', 'host:7443', 0)`); err != nil {
		t.Fatal(err)
	}
	tags := tag.NewStore(db)
	h := NewHandler(fakeNodes{conn: olderAgent{}}, fakeNetworks{}, tags, nil, nil, nil, NewMoves(), nil)
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/nodes/{node}/servers/{id}/notes", h.setNotes)
	for _, tc := range []struct {
		path, notes string
		code        int
	}{
		{"/api/nodes/n1/servers/s1/notes", "Test server, ask Alex", http.StatusNoContent},
		{"/api/nodes/n1/servers/s2/notes", "Unknown server", http.StatusNotFound},
		{"/api/nodes/n1/servers/s1/notes", strings.Repeat("x", 501), http.StatusBadRequest},
	} {
		body, _ := json.Marshal(map[string]string{"notes": tc.notes})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, tc.path, strings.NewReader(string(body))))
		if rec.Code != tc.code {
			t.Errorf("%s %q: status %d: %s", tc.path, tc.notes, rec.Code, rec.Body)
		}
	}
	if notes, err := tags.Notes(t.Context()); err != nil || len(notes) != 1 || notes[tag.Server{NodeID: "n1", ServerID: "s1"}] != "Test server, ask Alex" {
		t.Fatalf("notes = %q, %v", notes, err)
	}
}

type fakeNodes struct {
	Nodes
	conn grpc.ClientConnInterface // fakeConn if nil
}

func (n fakeNodes) Conn(context.Context, string) (grpc.ClientConnInterface, error) {
	if n.conn == nil {
		return fakeConn{}, nil
	}
	return n.conn, nil
}

// Get returns a node without port range and memory limit.
func (fakeNodes) Get(_ context.Context, id string) (node.Node, error) {
	return node.Node{ID: id, Name: id}, nil
}

// fakeConn is an agent that does whatever it is asked.
type fakeConn struct{ grpc.ClientConnInterface }

func (fakeConn) Invoke(context.Context, string, any, any, ...grpc.CallOption) error { return nil }

// NewStream refuses streams, like an agent that can't tell the progress of calls.
func (fakeConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, status.Error(codes.Unimplemented, "unknown service")
}

// olderAgent has the server s1, and answers without the settings it doesn't know yet.
type olderAgent struct{ fakeConn }

func (olderAgent) Invoke(_ context.Context, _ string, _, reply any, _ ...grpc.CallOption) error {
	srv := &noryxv1.Server{Id: "s1", Name: "Lobby", MemoryMb: 1024, Port: 25565}
	switch reply := reply.(type) {
	case *noryxv1.ListServersResponse:
		reply.Servers = []*noryxv1.Server{srv}
	case *noryxv1.UpdateServerResponse:
		reply.Server = srv
	}
	return nil
}

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
