package docker

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/runtime"
	"github.com/QwikByte/noryx/internal/logging"
)

const (
	// Docker starts a crashed server again and again. The agent stops it after maxCrashes
	// in a row, each within stableAfter of its start.
	maxCrashes  = 5
	stableAfter = 10 * time.Minute
	watchRetry  = 10 * time.Second
)

// mayHaveCrashed reports whether a container is worth inspecting for crashes: one that
// runs fine, never ran or stopped cleanly didn't crash.
func mayHaveCrashed(c container.Summary) bool {
	healthy := c.State == container.StateRunning && (c.Health == nil || c.Health.Status != container.Starting)
	return !healthy && c.State != container.StateCreated && !strings.HasPrefix(c.Status, "Exited (0)")
}

// addCrashes adds to srv how often it crashed since it was last started. A server that
// starts again after a crash is crashing.
func (d *Docker) addCrashes(ctx context.Context, containerID string, srv *runtime.Server) {
	res, err := d.cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil || res.Container.RestartCount == 0 {
		return
	}
	c := res.Container
	if srv.State == noryxv1.ServerState_SERVER_STATE_STOPPED && c.State.ExitCode == 0 {
		return // stopped cleanly after it ran again
	}
	srv.Crashes, srv.ExitCode = c.RestartCount, c.State.ExitCode
	if srv.State == noryxv1.ServerState_SERVER_STATE_STARTING {
		srv.State = noryxv1.ServerState_SERVER_STATE_CRASHING
	}
}

// watched is what the agent remembers about the containers it watches, by their IDs.
type watched struct {
	inRow     map[string]int  // crashes in a row
	unhealthy map[string]bool // whose health check failed last
}

// Watch logs crashes and stops servers that keep crashing, logs when the health check of a
// server fails and passes again, and closes the connections to the consoles of servers that
// stop, until ctx is done.
func (d *Docker) Watch(ctx context.Context) {
	w := watched{inRow: map[string]int{}, unhealthy: map[string]bool{}}
	for ctx.Err() == nil {
		err := d.watch(ctx, w)
		if ctx.Err() == nil {
			slog.Debug("Can't watch the servers for crashes", logging.Servers, "err", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(watchRetry): // e.g. while Docker restarts
		}
	}
}

func (d *Docker) watch(ctx context.Context, w watched) error {
	// health_status matches the events of all states of health.
	filters := make(client.Filters).Add("type", string(events.ContainerEventType)).
		Add("event", string(events.ActionDie), string(events.ActionHealthStatus)).Add("label", labelManaged)
	res := d.cli.Events(ctx, client.EventsListOptions{Filters: filters})
	for {
		select {
		case msg := <-res.Messages:
			if msg.Action == events.ActionDie {
				delete(w.unhealthy, msg.Actor.ID)
				d.crashed(ctx, msg.Actor, w.inRow)
			} else {
				health(msg, w.unhealthy)
			}
		case err := <-res.Err:
			return err
		}
	}
}

// health logs when the health check of a server fails, e.g. as it hangs, and when it passes
// again; Docker tells each change. Passing after the start isn't worth an entry.
func health(msg events.Message, unhealthy map[string]bool) {
	spec, ok := specOf(msg.Actor.Attributes)
	switch {
	case !ok:
	case msg.Action == events.ActionHealthStatusUnhealthy && !unhealthy[msg.Actor.ID]:
		unhealthy[msg.Actor.ID] = true
		slog.Warn("A server is unhealthy: it runs, but its health check fails, e.g. as it hangs", logging.Servers, logging.KeyServer, spec.ID)
	case msg.Action == events.ActionHealthStatusHealthy && unhealthy[msg.Actor.ID]:
		delete(unhealthy, msg.Actor.ID)
		slog.Info("A server is healthy again", logging.Servers, logging.KeyServer, spec.ID)
	}
}

// restarts reports whether Docker starts a container that exited again. It starts it again
// right away after its first crash, so it may run already.
func restarts(s *container.State) bool { return s != nil && (s.Restarting || s.Running) }

// crashed closes the console of a container that exited, and logs and counts its crash if
// Docker starts it again, stopping its server after maxCrashes in a row.
func (d *Docker) crashed(ctx context.Context, actor events.Actor, inRow map[string]int) {
	// The event carries the labels of the container; one that doesn't restart was stopped.
	spec, ok := specOf(actor.Attributes)
	if ok {
		d.consoles.Close(spec.ID)
	}
	res, err := d.cli.ContainerInspect(ctx, actor.ID, client.ContainerInspectOptions{})
	if err != nil || !ok || !restarts(res.Container.State) {
		delete(inRow, actor.ID)
		return
	}
	if !noteCrash(inRow, actor, spec.ID, res.Container.RestartCount) {
		return
	}
	if err := d.Stop(ctx, spec.ID); err != nil {
		slog.Warn("Can't stop a crashing server", logging.Servers, logging.KeyServer, spec.ID, "err", err)
	}
}

// noteCrash logs the crash of a server that Docker starts again, with how many crashes in a
// row it was, and reports whether that makes maxCrashes, so that the server is to be stopped.
// restarts is how often Docker restarted it since it was last started, e.g. by a user.
func noteCrash(inRow map[string]int, actor events.Actor, id string, restarts int) bool {
	n := inRow[actor.ID] + 1
	if ran, _ := strconv.Atoi(actor.Attributes["execDuration"]); time.Duration(ran)*time.Second >= stableAfter {
		n = 1
	}
	n = min(n, restarts)
	attrs := []any{logging.Servers, logging.KeyServer, id, "exit_code", actor.Attributes["exitCode"], "crashes", n}
	if n < maxCrashes {
		inRow[actor.ID] = n
		slog.Warn("A server crashed and starts again; its console and crash reports tell why", attrs...)
		return false
	}
	delete(inRow, actor.ID)
	slog.Warn("Stop a server that crashed "+strconv.Itoa(maxCrashes)+" times in a row; its console and crash reports tell why", attrs...)
	return true
}
