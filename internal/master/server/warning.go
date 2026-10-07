package server

import (
	"cmp"
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/operation"
	"github.com/QwikByte/noryx/internal/master/tag"
)

// Warnings tell the players of game servers in the chat that their server restarts or stops
// soon, with say: before restarts and stops by hand and before those of schedules. Proxies
// get none, as they have no say for their players.

const (
	maxWarningMessage = 200
	maxWarningMinutes = 10
	// lateWarning is how late a warning may be sent, e.g. when the master just started.
	lateWarning = time.Minute
)

var defaultWarnings = map[string]string{
	"restart": "The server restarts in {minutes} min.",
	"stop":    "The server stops in {minutes} min.",
}

// WarningMessage returns the message that warns the players of an action, restart or stop: the
// one given without the spaces around it, or the default if it is empty. {minutes} in it
// stands for the minutes left.
func WarningMessage(action, message string) (string, error) {
	message = cmp.Or(strings.TrimSpace(message), defaultWarnings[action])
	if len(message) > maxWarningMessage || strings.ContainsFunc(message, unicode.IsControl) {
		return "", httpapi.Errorf(http.StatusBadRequest, "Keep the warning on one line with up to %d characters.", maxWarningMessage)
	}
	return message, nil
}

// SayWarning returns the console command that warns the players that minutes are left.
func SayWarning(message string, minutes uint32) string {
	return "say " + strings.ReplaceAll(message, "{minutes}", strconv.FormatUint(uint64(minutes), 10))
}

// Warnable tells whether the players of a server can be warned: it is a running game server.
func Warnable(srv *noryxv1.Server) bool {
	return srv.GetState() == noryxv1.ServerState_SERVER_STATE_RUNNING && !srv.GetType().Proxy()
}

// Countdown calls warn at each of the minutes before at, the most first, and waits until at.
// Warnings more than a minute late are left out, e.g. as the master just started. It ends
// early with the error of ctx.
func Countdown(ctx context.Context, at time.Time, minutes []uint32, warn func(minutes uint32)) error {
	for _, m := range minutes {
		warnAt := at.Add(-time.Duration(m) * time.Minute)
		if time.Until(warnAt) < -lateWarning {
			continue
		}
		if err := sleepUntil(ctx, warnAt); err != nil {
			return err
		}
		warn(m)
	}
	return sleepUntil(ctx, at)
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

// warning warns the players before servers stop or restart by hand: Minutes before, and again
// 5 minutes and 1 minute before.
type warning struct {
	Minutes uint32 `json:"minutes"`
	// Message has {minutes} for the minutes left; empty is the default of the action.
	Message string `json:"message"`
}

// check validates the warning of an action on servers, if there is one. A message of one's
// own needs the permission to send console commands, as say is one.
func (w *warning) check(r *http.Request, action string, servers []tag.Server) error {
	if w == nil {
		return nil
	}
	w.Message = strings.TrimSpace(w.Message)
	switch {
	case defaultWarnings[action] == "":
		return httpapi.Errorf(http.StatusBadRequest, "Only stopping and restarting warn the players.")
	case w.Minutes < 1 || w.Minutes > maxWarningMinutes:
		return httpapi.Errorf(http.StatusBadRequest, "Warn the players 1 to %d minutes before.", maxWarningMinutes)
	case w.Message != "" && !onAll(access.From(r.Context()), access.ConsoleCommands, servers):
		return access.Denied(access.ConsoleCommands)
	}
	var err error
	w.Message, err = WarningMessage(action, w.Message)
	if err == nil {
		logging.Note(r.Context(), slog.Uint64("warning_minutes", uint64(w.Minutes)), slog.String("warning", w.Message))
	}
	return err
}

// steps are the minutes before the action at which the players are warned.
func (w *warning) steps() []uint32 {
	return append([]uint32{w.Minutes}, slices.DeleteFunc([]uint32{5, 1}, func(m uint32) bool { return m >= w.Minutes })...)
}

// duration is how long the warning holds up the action.
func (w *warning) duration() time.Duration {
	if w == nil {
		return 0
	}
	return time.Duration(w.Minutes) * time.Minute
}

// countdown warns the players of the running game servers among servers, as w says, in the
// step warn of the operation of ctx, and waits until the time is up. Without a warning or a
// server to warn, it returns at once. It ends early if the operation is cancelled.
func (h *Handler) countdown(ctx context.Context, w *warning, servers []tag.Server) error {
	if w == nil {
		return nil
	}
	operation.Step(ctx, "warn")
	warned := h.warnable(ctx, servers)
	if len(warned) == 0 {
		return nil
	}
	total := int64(w.Minutes)
	return Countdown(ctx, time.Now().Add(w.duration()), w.steps(), func(minutes uint32) {
		operation.Count(ctx, total-int64(minutes), total, "minutes")
		var wg sync.WaitGroup
		for _, s := range warned {
			wg.Go(func() {
				ctx, cancel := context.WithTimeout(ctx, queryTimeout)
				defer cancel()
				c, err := h.serverClient(ctx, s.NodeID)
				if err == nil {
					err = serverActions["command"].call(ctx, c, s.ServerID, SayWarning(w.Message, minutes))
				}
				if err != nil { // the server stops anyway
					slog.Debug("Can't warn the players of a server", logging.KeyNode, s.NodeID, logging.KeyServer, s.ServerID, "err", err)
				}
			})
		}
		wg.Wait()
	})
}

// warnable returns the servers among servers whose players can be warned. Nodes that can't be
// reached are left out.
func (h *Handler) warnable(ctx context.Context, servers []tag.Server) []tag.Server {
	var nodes []string
	var out []tag.Server
	for _, s := range servers {
		if !slices.Contains(nodes, s.NodeID) {
			nodes = append(nodes, s.NodeID)
		}
	}
	for _, nodeID := range nodes {
		ctx, cancel := context.WithTimeout(ctx, queryTimeout)
		c, err := h.serverClient(ctx, nodeID)
		var res *noryxv1.ListServersResponse
		if err == nil {
			res, _ = c.ListServers(ctx, &noryxv1.ListServersRequest{})
		}
		cancel()
		for _, srv := range res.GetServers() {
			if s := (tag.Server{NodeID: nodeID, ServerID: srv.GetId()}); Warnable(srv) && slices.Contains(servers, s) {
				out = append(out, s)
			}
		}
	}
	return out
}
