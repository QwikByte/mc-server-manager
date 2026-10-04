// Package policy is the kind of scheduled task that rules servers or whole nodes: restarts
// with a countdown for the players, operating hours by starting and stopping servers, and
// console commands.
package policy

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"google.golang.org/grpc"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/schedule"
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
	maxMessage  = 200
	maxCommand  = 1000
	// actionTimeout covers a graceful stop, in which a server saves its worlds.
	actionTimeout = 3 * time.Minute
	// lateWarning is how late a warning may be sent, e.g. when the master just started.
	lateWarning = time.Minute
)

// actions are what the log says a policy did to a server.
var actions = map[string]string{restart: "Restart server", stop: "Stop server", start: "Start server", command: "Send console command"}

var defaultMessages = map[string]string{
	restart: "The server restarts in {minutes} min.",
	stop:    "The server stops in {minutes} min.",
}

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
}

// Nodes provides connections to node agents.
type Nodes interface {
	Conn(ctx context.Context, nodeID string) (grpc.ClientConnInterface, error)
}

// Policies is the kind of task that runs policies.
type Policies struct{ nodes Nodes }

func New(nodes Nodes) Policies { return Policies{nodes: nodes} }

func (Policies) Check(raw json.RawMessage) (json.RawMessage, error) {
	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, httpapi.Errorf(http.StatusBadRequest, "Choose what the policy does.")
	}
	if _, warns := defaultMessages[s.Action]; warns {
		slices.SortFunc(s.Warnings, func(a, b uint32) int { return cmp.Compare(b, a) })
		s.Warnings, s.Message = slices.Compact(s.Warnings), strings.TrimSpace(s.Message)
		if s.Message == "" {
			s.Message = defaultMessages[s.Action]
		}
	} else {
		s.Warnings, s.Message = nil, ""
	}
	if s.Action != command {
		s.Command = ""
	}
	s.Command = strings.TrimSpace(s.Command)
	switch {
	case !slices.Contains([]string{restart, stop, start, command}, s.Action):
		return nil, httpapi.Errorf(http.StatusBadRequest, "Choose what the policy does.")
	case len(s.Warnings) > maxWarnings || slices.ContainsFunc(s.Warnings, func(m uint32) bool { return m == 0 || m > maxMinutes }):
		return nil, httpapi.Errorf(http.StatusBadRequest, "Warn the players up to %d times, 1 to %d minutes before.", maxWarnings, maxMinutes)
	case len(s.Message) > maxMessage || strings.ContainsFunc(s.Message, unicode.IsControl):
		return nil, httpapi.Errorf(http.StatusBadRequest, "Keep the warning on one line with up to %d characters.", maxMessage)
	case s.Action == command && (s.Command == "" || len(s.Command) > maxCommand || strings.ContainsFunc(s.Command, unicode.IsControl)):
		return nil, httpapi.Errorf(http.StatusBadRequest, "Enter a single console command with up to %d characters.", maxCommand)
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
// concerns: running ones for a restart, stop or command, stopped ones for a start.
func (p Policies) Run(ctx context.Context, t schedule.Task, servers schedule.Servers, at time.Time) error {
	var s Settings
	if err := json.Unmarshal(t.Settings, &s); err != nil {
		return err
	}
	list, err := servers(ctx)
	for _, minutes := range s.Warnings {
		warnAt := at.Add(-time.Duration(minutes) * time.Minute)
		if time.Until(warnAt) < -lateWarning {
			continue
		}
		if err := sleepUntil(ctx, warnAt); err != nil {
			return err
		}
		say := "say " + strings.ReplaceAll(s.Message, "{minutes}", strconv.FormatUint(uint64(minutes), 10))
		for _, srv := range list {
			if srv.GetState() == noryxv1.ServerState_SERVER_STATE_RUNNING && !srv.GetType().Proxy() && concerns(s, srv) {
				_ = p.call(ctx, srv, command, say) // a server that misses a warning restarts anyway
			}
		}
	}
	if err := sleepUntil(ctx, at); err != nil {
		return err
	}
	if len(s.Warnings) > 0 { // servers may have been started or stopped during the countdown
		list, err = servers(ctx)
	}
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs = []error{err}
	)
	for _, srv := range list {
		if concerns(s, srv) {
			wg.Go(func() {
				err := srv.Report(t, logging.Policies, actions[s.Action], p.call(ctx, srv, s.Action, s.Command))
				mu.Lock()
				defer mu.Unlock()
				errs = append(errs, err)
			})
		}
	}
	wg.Wait()
	return errors.Join(errs...)
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

func sleepUntil(ctx context.Context, t time.Time) error {
	timer := time.NewTimer(time.Until(t))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
