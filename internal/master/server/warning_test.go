package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"google.golang.org/grpc"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/auth"
	"github.com/QwikByte/noryx/internal/master/operation"
)

// A stop or restart with a warning warns the players of the running game servers, again 5
// minutes and 1 minute before, and only then stops or restarts. Until then, it can be
// cancelled, which stops nothing.
func TestWarning(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		agent := &recorder{servers: []*noryxv1.Server{
			{Id: "s1", Type: noryxv1.ServerType_SERVER_TYPE_PAPER, State: noryxv1.ServerState_SERVER_STATE_RUNNING},
			{Id: "p1", Type: noryxv1.ServerType_SERVER_TYPE_VELOCITY, State: noryxv1.ServerState_SERVER_STATE_RUNNING},
		}}
		ops := operation.New(0)
		h := NewHandler(fakeNodes{conn: agent}, fakeNetworks{}, nil, nil, nil, ops, NewMoves(), nil)
		mux := http.NewServeMux()
		mux.HandleFunc("POST /api/nodes/{node}/servers/{id}/restart", h.power("restart"))
		mux.HandleFunc("POST /api/servers/actions", h.bulk)
		admin := access.WithGrants(t.Context(), access.Admin())
		post := func(ctx context.Context, path, body string, status int) operation.Operation {
			t.Helper()
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodPost, path, strings.NewReader(body)))
			var op operation.Operation
			if rec.Code != status || status == http.StatusAccepted && json.Unmarshal(rec.Body.Bytes(), &op) != nil {
				t.Fatalf("%s %s: status %d: %s", path, body, rec.Code, rec.Body)
			}
			return op
		}

		op := post(admin, "/api/nodes/n1/servers/s1/restart", `{"warning": {"minutes": 10}}`, http.StatusAccepted)
		if !slices.Equal(op.Steps, []string{"warn", "restart"}) || !op.Cancellable {
			t.Fatalf("operation = %+v", op)
		}
		for _, wait := range []time.Duration{0, 5 * time.Minute, 4 * time.Minute, time.Minute} {
			time.Sleep(wait)
			synctest.Wait()
		}
		want := []string{"ListServers", "SendCommand s1 say The server restarts in 10 min.", "SendCommand s1 say The server restarts in 5 min.",
			"SendCommand s1 say The server restarts in 1 min.", "RestartServer s1"}
		if got := agent.taken(); !slices.Equal(got, want) {
			t.Fatalf("calls %q, want %q", got, want)
		}

		// Proxies get no warning.
		op = post(admin, "/api/servers/actions", `{"action": "stop", "servers": [{"nodeId": "n1", "serverId": "s1"}, {"nodeId": "n1", "serverId": "p1"}],
			"warning": {"minutes": 2, "message": "Bye in {minutes} min"}}`, http.StatusAccepted)
		synctest.Wait()
		if _, err := ops.Cancel(admin, op.ID, auth.User{}, access.Admin()); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if got := agent.taken(); !slices.Equal(got, []string{"ListServers", "SendCommand s1 say Bye in 2 min"}) {
			t.Fatalf("calls %q", got)
		}
		if ops := ops.List(auth.User{}, access.Admin()); !ops[0].Cancelled || ops[0].FinishedAt == nil || ops[0].Steps[ops[0].Step] != "warn" {
			t.Fatalf("cancelled operation = %+v", ops[0])
		}

		// Warnings take 1 to 10 minutes, only before stops and restarts, and a message of one's
		// own needs the permission to send console commands.
		post(admin, "/api/nodes/n1/servers/s1/restart", `{"warning": {"minutes": 11}}`, http.StatusBadRequest)
		post(admin, "/api/nodes/n1/servers/s1/restart", `{"warning": {"minutes": 1, "message": "two\nlines"}}`, http.StatusBadRequest)
		post(admin, "/api/servers/actions", `{"action": "start", "servers": [{"nodeId": "n1", "serverId": "s1"}], "warning": {"minutes": 1}}`, http.StatusBadRequest)
		post(t.Context(), "/api/nodes/n1/servers/s1/restart", `{"warning": {"minutes": 1, "message": "op me"}}`, http.StatusForbidden)
	})
}

// recorder is an agent with servers that records the calls to it.
type recorder struct {
	fakeConn
	servers []*noryxv1.Server
	mu      sync.Mutex
	calls   []string
}

func (a *recorder) Invoke(_ context.Context, method string, args, reply any, _ ...grpc.CallOption) error {
	call := method[strings.LastIndex(method, "/")+1:]
	switch req := args.(type) {
	case *noryxv1.SendCommandRequest:
		call += " " + req.GetId() + " " + req.GetCommand()
	case interface{ GetId() string }:
		call += " " + req.GetId()
	}
	if reply, ok := reply.(*noryxv1.ListServersResponse); ok {
		reply.Servers = a.servers
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, call)
	return nil
}

// taken returns the calls so far and forgets them.
func (a *recorder) taken() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	calls := a.calls
	a.calls = nil
	return calls
}
