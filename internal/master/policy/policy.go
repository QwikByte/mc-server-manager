// Package policy is the kind of scheduled task that rules servers or whole nodes: restarts
// with a countdown for the players, also server by server in networks, operating hours by
// starting and stopping servers, and console commands.
package policy

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"google.golang.org/grpc"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/network"
	"github.com/QwikByte/noryx/internal/master/schedule"
	"github.com/QwikByte/noryx/internal/master/server"
)

// TaskKind identifies policies among the scheduled tasks.
const TaskKind = "policy"

const (
	restart = "restart"
	stop    = "stop"
	start   = "start"
	command = "command"

	maxWarnings = 5
	maxMinutes  = 60
	maxCommand  = 1000
	// actionTimeout covers a graceful stop, in which a server saves its worlds, which may take
	// the longest stop timeout.
	actionTimeout = 2*time.Minute + noryxv1.MaxStopTimeout
)

// actions are what the log says a policy did to a server.
var actions = map[string]string{restart: "Restart server", stop: "Stop server", start: "Start server", command: "Send console command"}

// Settings say what a policy does.
type Settings struct {
	// Action is restart, stop, start or command.
	Action string `json:"action"`
	// Warnings are the minutes before a restart or stop at which players are warned.
	Warnings []uint32 `json:"warnings"`
	// Message warns the players; {minutes} is replaced by the minutes left.
	Message string `json:"message"`
	// Command is the console command of the command action.
	Command string `json:"command"`
	// Rolling, unless 0, restarts the running game servers of networks this many at a time,
	// so that their players move to other servers first; see network.RollingRestart.
	Rolling int `json:"rolling,omitempty"`
}

// Nodes provides connections to node agents.
type Nodes interface {
	Conn(ctx context.Context, nodeID string) (grpc.ClientConnInterface, error)
}

// Networks restart the game servers of networks server by server.
type Networks interface {
	List(ctx context.Context) ([]network.Network, error)
	RollingRestart(ctx context.Context, n network.Network, batch int, only ...network.Ref) error
}

// Policies is the kind of task that runs policies.
type Policies struct {
	nodes    Nodes
	networks Networks
}

func New(nodes Nodes, networks Networks) Policies { return Policies{nodes: nodes, networks: networks} }

func (Policies) Check(raw json.RawMessage) (json.RawMessage, error) {
	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, httpapi.Errorf(http.StatusBadRequest, "Choose what the schedule does.")
	}
	var err error
	if s.Action == restart || s.Action == stop {
		slices.SortFunc(s.Warnings, func(a, b uint32) int { return cmp.Compare(b, a) })
		s.Warnings = slices.Compact(s.Warnings)
		s.Message, err = server.WarningMessage(s.Action, s.Message)
	} else {
		s.Warnings, s.Message = nil, ""
	}
	if s.Action != command {
		s.Command = ""
	}
	if s.Action != restart {
		s.Rolling = 0
	}
	s.Command = strings.TrimSpace(s.Command)
	switch {
	case !slices.Contains([]string{restart, stop, start, command}, s.Action):
		return nil, httpapi.Errorf(http.StatusBadRequest, "Choose what the schedule does.")
	case len(s.Warnings) > maxWarnings || slices.ContainsFunc(s.Warnings, func(m uint32) bool { return m == 0 || m > maxMinutes }):
		return nil, httpapi.Errorf(http.StatusBadRequest, "Warn the players up to %d times, 1 to %d minutes before.", maxWarnings, maxMinutes)
	case err != nil:
		return nil, err
	case s.Action == command && (s.Command == "" || len(s.Command) > maxCommand || strings.ContainsFunc(s.Command, unicode.IsControl)):
		return nil, httpapi.Errorf(http.StatusBadRequest, "Enter a single console command with up to %d characters.", maxCommand)
	case s.Rolling < 0 || s.Rolling > network.MaxBatch:
		return nil, httpapi.Errorf(http.StatusBadRequest, "Restart from 1 to %d servers of a network at a time.", network.MaxBatch)
	}
	if s.Warnings == nil {
		s.Warnings = []uint32{}
	}
	return json.Marshal(s)
}

func (Policies) Category() slog.Attr { return logging.Policies }

// Lead is the time of the earliest warning before a restart or stop.
func (Policies) Lead(raw json.RawMessage) time.Duration {
	var s Settings
	if json.Unmarshal(raw, &s) != nil || len(s.Warnings) == 0 {
		return 0
	}
	return time.Duration(slices.Max(s.Warnings)) * time.Minute
}

// Run warns the players, then applies the policy at its scheduled time to the servers it
// concerns: running ones for a restart, stop or command, stopped ones for a start. A restart
// server by server restarts the running game servers of each network with RollingRestart
// and the network's proxy after them; the other servers at the same time.
func (p Policies) Run(ctx context.Context, t schedule.Task, servers schedule.Servers, at time.Time) error {
	var s Settings
	if err := json.Unmarshal(t.Settings, &s); err != nil {
		return err
	}
	list, err := servers(ctx)
	if err := server.Countdown(ctx, at, s.Warnings, func(minutes uint32) {
		for _, srv := range list {
			if server.Warnable(srv.Server) && concerns(s, srv) {
				_ = p.call(ctx, srv, command, server.SayWarning(s.Message, minutes)) // a server that misses a warning restarts anyway
			}
		}
	}); err != nil {
		return err
	}
	if len(s.Warnings) > 0 { // servers may have been started or stopped during the countdown
		list, err = servers(ctx)
	}
	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		errs   = []error{err}
		groups []group
	)
	report := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
	}
	if s.Action == restart && s.Rolling > 0 {
		list, groups, err = p.group(ctx, list)
		report(err)
	}
	for _, srv := range list {
		if concerns(s, srv) {
			wg.Go(func() {
				report(srv.Report(t, logging.Policies, actions[s.Action], p.call(ctx, srv, s.Action, s.Command)))
			})
		}
	}
	for _, g := range groups {
		wg.Go(func() { report(p.restartNetwork(ctx, t, s.Rolling, g)) })
	}
	wg.Wait()
	return errors.Join(errs...)
}

// group is a network whose running game servers restart server by server, and its proxy, if
// it restarts too.
type group struct {
	network network.Network
	servers []schedule.Server
	proxy   *schedule.Server
}

// group takes the running game servers of networks out of servers, and the proxies of these
// networks, to restart them network by network.
func (p Policies) group(ctx context.Context, servers []schedule.Server) ([]schedule.Server, []group, error) {
	networks, err := p.networks.List(ctx)
	if err != nil { // they restart at the same time then
		return servers, nil, fmt.Errorf("the servers of networks restarted at the same time: %w", err)
	}
	var groups []group
	for _, n := range networks {
		g := group{network: n}
		servers = slices.DeleteFunc(servers, func(srv schedule.Server) bool {
			ref := network.Ref{NodeID: srv.NodeID, ServerID: srv.GetId()}
			switch {
			case ref == n.Proxy:
				g.proxy = &srv
			case srv.GetState() == noryxv1.ServerState_SERVER_STATE_RUNNING && slices.ContainsFunc(n.Backends, func(b network.Backend) bool { return b.Ref == ref }):
				g.servers = append(g.servers, srv)
			default:
				return false
			}
			return true
		})
		switch {
		case len(g.servers) > 0:
			groups = append(groups, g)
		case g.proxy != nil:
			servers = append(servers, *g.proxy)
		}
	}
	return servers, groups, nil
}

// restartNetwork restarts the game servers of a group server by server, then its proxy.
func (p Policies) restartNetwork(ctx context.Context, t schedule.Task, batch int, g group) error {
	only := make([]network.Ref, len(g.servers))
	for i, srv := range g.servers {
		only[i] = network.Ref{NodeID: srv.NodeID, ServerID: srv.GetId()}
	}
	err := p.networks.RollingRestart(ctx, g.network, batch, only...)
	if err == nil {
		for _, srv := range g.servers {
			_ = srv.Report(t, logging.Policies, actions[restart], nil)
		}
	} else {
		msg := httpapi.Message(err)
		slog.Warn("Restart servers of a network one after the other failed", logging.Policies, "task", t.Name, "network", g.network.Name, "err", msg)
		err = fmt.Errorf("%s: %s", g.network.Name, msg)
	}
	if g.proxy != nil {
		err = errors.Join(err, g.proxy.Report(t, logging.Policies, actions[restart], p.call(ctx, *g.proxy, restart, "")))
	}
	return err
}

// concerns tells whether a policy applies to a server in its current state. Console commands
// go to game servers only, as proxies don't know theirs, such as say.
func concerns(s Settings, srv schedule.Server) bool {
	stopped := srv.GetState() == noryxv1.ServerState_SERVER_STATE_STOPPED
	switch s.Action {
	case start:
		return stopped
	case command:
		return srv.GetState() == noryxv1.ServerState_SERVER_STATE_RUNNING && !srv.GetType().Proxy()
	}
	return !stopped
}

func (p Policies) call(ctx context.Context, srv schedule.Server, action, cmd string) error {
	ctx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()
	conn, err := p.nodes.Conn(ctx, srv.NodeID)
	if err != nil {
		return err
	}
	c, id := noryxv1.NewServerServiceClient(conn), srv.GetId()
	switch action {
	case restart:
		_, err = c.RestartServer(ctx, &noryxv1.RestartServerRequest{Id: id})
	case stop:
		_, err = c.StopServer(ctx, &noryxv1.StopServerRequest{Id: id})
	case start:
		_, err = c.StartServer(ctx, &noryxv1.StartServerRequest{Id: id})
	default:
		_, err = c.SendCommand(ctx, &noryxv1.SendCommandRequest{Id: id, Command: cmd})
	}
	return err
}
