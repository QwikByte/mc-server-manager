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

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
	"github.com/QwikByte/mc-server-manager/internal/logging"
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
	if srv.State == mcsmv1.ServerState_SERVER_STATE_STOPPED && c.State.ExitCode == 0 {
		return // stopped cleanly after it ran again
	}
	srv.Crashes, srv.ExitCode = c.RestartCount, c.State.ExitCode
	if srv.State == mcsmv1.ServerState_SERVER_STATE_STARTING {
		srv.State = mcsmv1.ServerState_SERVER_STATE_CRASHING
	}
}

// Watch keeps the servers in order until ctx is done: it moves servers of older agents
// into the current networks whenever it connects to Docker, and stops servers that keep
// crashing.
func (d *Docker) Watch(ctx context.Context) {
	inRow := map[string]int{} // crashes in a row by container ID
	for ctx.Err() == nil {
		if err := d.adopt(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("Can't move servers into the current networks", logging.Servers, "err", err)
		}
		err := d.watch(ctx, inRow)
		if ctx.Err() == nil {
			slog.Debug("Can't watch the servers for crashes", logging.Servers, "err", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(watchRetry): // e.g. while Docker restarts
		}
	}
}

func (d *Docker) watch(ctx context.Context, inRow map[string]int) error {
	filters := make(client.Filters).Add("type", string(events.ContainerEventType)).Add("event", string(events.ActionDie)).Add("label", labelManaged)
	res := d.cli.Events(ctx, client.EventsListOptions{Filters: filters})
	for {
		select {
		case msg := <-res.Messages:
			d.crashed(ctx, msg.Actor, inRow)
		case err := <-res.Err:
			return err
		}
	}
}

// crashed counts a container that exited, and stops its server after maxCrashes in a row.
func (d *Docker) crashed(ctx context.Context, actor events.Actor, inRow map[string]int) {
	// The event carries the labels of the container; one that doesn't restart was stopped.
	res, err := d.cli.ContainerInspect(ctx, actor.ID, client.ContainerInspectOptions{})
	spec, ok := specOf(actor.Attributes)
	if err != nil || !ok || !res.Container.State.Restarting {
		delete(inRow, actor.ID)
		return
	}
	n := inRow[actor.ID] + 1
	if ran, _ := strconv.Atoi(actor.Attributes["execDuration"]); time.Duration(ran)*time.Second >= stableAfter {
		n = 1
	}
	// Docker counts the restarts since the server was last started, e.g. by a user.
	inRow[actor.ID] = min(n, res.Container.RestartCount)
	if inRow[actor.ID] < maxCrashes {
		return
	}
	delete(inRow, actor.ID)
	slog.Warn("Stop a server that crashed "+strconv.Itoa(maxCrashes)+" times in a row; the console shows why", logging.Servers,
		logging.KeyServer, spec.ID, "exit_code", actor.Attributes["exitCode"])
	if err := d.Stop(ctx, spec.ID); err != nil {
		slog.Warn("Can't stop a crashing server", logging.Servers, logging.KeyServer, spec.ID, "err", err)
	}
}
