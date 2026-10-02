package policy

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/schedule"
)

func TestCheck(t *testing.T) {
	got, err := Policies{}.Check(json.RawMessage(`{"action":"restart","warnings":[1,10,5,10],"command":"op me"}`))
	if want := `{"action":"restart","warnings":[10,5,1],"message":"The server restarts in {minutes} min.","command":""}`; err != nil || string(got) != want {
		t.Fatalf("checked settings = %s, %v; want %s", got, err, want)
	}
	if lead := (Policies{}).Lead(got); lead != 10*time.Minute {
		t.Fatalf("lead = %v", lead)
	}
	for _, bad := range []string{
		`{"action":"explode"}`,
		`{"action":"stop","warnings":[61]}`,
		`{"action":"stop","warnings":[0]}`,
		`{"action":"restart","message":"two\nlines"}`,
		`{"action":"command","command":""}`,
		`{"action":"command","command":"say hi\nop attacker"}`,
	} {
		if _, err := (Policies{}).Check(json.RawMessage(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

// agent records the calls to the ServerService of a node.
type agent struct {
	mu    sync.Mutex
	calls []string
}

func (a *agent) Invoke(_ context.Context, method string, args, _ any, _ ...grpc.CallOption) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	call := method[strings.LastIndex(method, "/")+1:]
	if req, ok := args.(*mcsmv1.SendCommandRequest); ok {
		call += " " + req.GetCommand()
	} else {
		call += " " + args.(interface{ GetId() string }).GetId()
	}
	a.calls = append(a.calls, call)
	return nil
}

func (a *agent) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, errors.New("not supported")
}

func (a *agent) Conn(context.Context, string) (grpc.ClientConnInterface, error) { return a, nil }

func TestRun(t *testing.T) {
	server := func(id string, typ mcsmv1.ServerType, state mcsmv1.ServerState) schedule.Server {
		return schedule.Server{Server: &mcsmv1.Server{Id: id, Name: id, Type: typ, State: state}, NodeID: "n1", NodeName: "node-1"}
	}
	paper, velocity := mcsmv1.ServerType_SERVER_TYPE_PAPER, mcsmv1.ServerType_SERVER_TYPE_VELOCITY
	running, stopped := mcsmv1.ServerState_SERVER_STATE_RUNNING, mcsmv1.ServerState_SERVER_STATE_STOPPED
	servers := func(context.Context) ([]schedule.Server, error) {
		return []schedule.Server{server("lobby", paper, running), server("proxy", velocity, running), server("old", paper, stopped)},
			errors.New("node-2: unreachable")
	}
	for _, tc := range []struct {
		settings string
		want     []string
	}{
		// The 5 minute warning is too late, the 1 minute warning is sent right away.
		{`{"action":"restart","warnings":[5,1],"message":"Restart in {minutes} min"}`,
			[]string{"SendCommand say Restart in 1 min", "RestartServer lobby", "RestartServer proxy"}},
		{`{"action":"start"}`, []string{"StartServer old"}},
		{`{"action":"command","command":"save-all"}`, []string{"SendCommand save-all"}},
	} {
		a := &agent{}
		task := schedule.Task{Settings: json.RawMessage(tc.settings)}
		err := New(a).Run(t.Context(), task, servers, time.Now().Add(50*time.Millisecond))
		slices.Sort(a.calls[1:]) // servers are restarted at the same time, after the warnings
		if err == nil || err.Error() != "node-2: unreachable" || !slices.Equal(a.calls, tc.want) {
			t.Errorf("%s: calls %q, %v; want %q", tc.settings, a.calls, err, tc.want)
		}
	}
}
