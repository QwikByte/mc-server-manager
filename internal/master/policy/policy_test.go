package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/network"
	"github.com/QwikByte/noryx/internal/master/schedule"
)

func TestCheck(t *testing.T) {
	got, err := Policies{}.Check(json.RawMessage(`{"action":"restart","warnings":[1,10,5,10],"command":"op me"}`))
	if want := `{"action":"restart","warnings":[10,5,1],"message":"The server restarts in {minutes} min.","command":""}`; err != nil || string(got) != want {
		t.Fatalf("checked settings = %s, %v; want %s", got, err, want)
	}
	if lead := (Policies{}).Lead(got); lead != 10*time.Minute {
		t.Fatalf("lead = %v", lead)
	}
	// Only restarts go server by server.
	got, err = Policies{}.Check(json.RawMessage(`{"action":"stop","rolling":2,"message":"  Bye in {minutes} min "}`))
	if want := `{"action":"stop","warnings":[],"message":"Bye in {minutes} min","command":""}`; err != nil || string(got) != want {
		t.Fatalf("checked settings = %s, %v; want %s", got, err, want)
	}
	if got, err = (Policies{}).Check(json.RawMessage(`{"action":"restart","rolling":2}`)); err != nil || !strings.Contains(string(got), `"rolling":2`) {
		t.Fatalf("checked settings = %s, %v", got, err)
	}
	for _, bad := range []string{
		`{"action":"explode"}`,
		`{"action":"stop","warnings":[61]}`,
		`{"action":"stop","warnings":[0]}`,
		`{"action":"restart","message":"two\nlines"}`,
		`{"action":"restart","rolling":51}`,
		`{"action":"restart","rolling":-1}`,
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
	call := method[strings.LastIndex(method, "/")+1:]
	if req, ok := args.(*noryxv1.SendCommandRequest); ok {
		call += " " + req.GetCommand()
	} else {
		call += " " + args.(interface{ GetId() string }).GetId()
	}
	a.record(call)
	return nil
}

func (a *agent) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, errors.New("not supported")
}

func (a *agent) Conn(context.Context, string) (grpc.ClientConnInterface, error) { return a, nil }

func (a *agent) record(call string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, call)
}

// networks restart the servers of their networks server by server, which the agent records.
type networks struct {
	*agent
	list []network.Network
}

func (n networks) List(context.Context) ([]network.Network, error) { return n.list, nil }

func (n networks) RollingRestart(_ context.Context, nw network.Network, batch int, only ...network.Ref) error {
	ids := make([]string, len(only))
	for i, ref := range only {
		ids[i] = ref.ServerID
	}
	n.record(fmt.Sprintf("RollingRestart %s %d %s", nw.Name, batch, strings.Join(ids, ",")))
	return nil
}

var (
	paper, velocity  = noryxv1.ServerType_SERVER_TYPE_PAPER, noryxv1.ServerType_SERVER_TYPE_VELOCITY
	running, stopped = noryxv1.ServerState_SERVER_STATE_RUNNING, noryxv1.ServerState_SERVER_STATE_STOPPED
)

func target(id string, typ noryxv1.ServerType, state noryxv1.ServerState) schedule.Server {
	return schedule.Server{Server: &noryxv1.Server{Id: id, Name: id, Type: typ, State: state}, NodeID: "n1", NodeName: "node-1"}
}

func TestRun(t *testing.T) {
	servers := func(context.Context) ([]schedule.Server, error) {
		return []schedule.Server{target("lobby", paper, running), target("proxy", velocity, running), target("old", paper, stopped)},
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
		err := New(a, networks{agent: a}).Run(t.Context(), task, servers, time.Now().Add(50*time.Millisecond))
		slices.Sort(a.calls[1:]) // servers are restarted at the same time, after the warnings
		if err == nil || err.Error() != "node-2: unreachable" || !slices.Equal(a.calls, tc.want) {
			t.Errorf("%s: calls %q, %v; want %q", tc.settings, a.calls, err, tc.want)
		}
	}
}

// Server by server, the running game servers of each network restart with a rolling restart,
// and then its proxy. The other servers restart at once, also those of networks that don't
// run and the proxy of a network whose servers don't.
func TestRunServerByServer(t *testing.T) {
	ref := func(id string) network.Ref { return network.Ref{NodeID: "n1", ServerID: id} }
	main := network.Network{Name: "Main", Proxy: ref("proxy"), Backends: []network.Backend{
		{Ref: ref("lobby")}, {Ref: ref("game")}, {Ref: ref("old")}, {Ref: ref("starting")}, {Ref: ref("other")},
	}}
	idle := network.Network{Name: "Idle", Proxy: ref("idle-proxy"), Backends: []network.Backend{{Ref: ref("idle")}}}
	servers := func(context.Context) ([]schedule.Server, error) {
		return []schedule.Server{ // other isn't a target
			target("proxy", velocity, running), target("lobby", paper, running), target("game", paper, running),
			target("old", paper, stopped), target("starting", paper, noryxv1.ServerState_SERVER_STATE_STARTING),
			target("solo", paper, running), target("idle-proxy", velocity, running), target("idle", paper, stopped),
		}, nil
	}
	a := &agent{}
	task := schedule.Task{Settings: json.RawMessage(`{"action":"restart","rolling":2}`)}
	if err := New(a, networks{a, []network.Network{main, idle}}).Run(t.Context(), task, servers, time.Now()); err != nil {
		t.Fatal(err)
	}
	if rolling, proxy := slices.Index(a.calls, "RollingRestart Main 2 lobby,game"), slices.Index(a.calls, "RestartServer proxy"); rolling < 0 || proxy < rolling {
		t.Errorf("calls %q: the proxy didn't restart after its servers", a.calls)
	}
	slices.Sort(a.calls)
	want := []string{"RestartServer idle-proxy", "RestartServer proxy", "RestartServer solo", "RestartServer starting", "RollingRestart Main 2 lobby,game"}
	if !slices.Equal(a.calls, want) {
		t.Errorf("calls %q, want %q", a.calls, want)
	}
}
