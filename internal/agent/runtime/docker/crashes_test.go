package docker

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

func TestCrashState(t *testing.T) {
	health := func(s container.HealthStatus) *container.HealthSummary { return &container.HealthSummary{Status: s} }
	for _, tc := range []struct {
		name      string
		c         container.Summary
		state     noryxv1.ServerState
		unhealthy bool
		inspect   bool
	}{
		{"running", container.Summary{State: container.StateRunning, Status: "Up 2 hours (healthy)", Health: health(container.Healthy)}, noryxv1.ServerState_SERVER_STATE_RUNNING, false, false},
		{"running without a health check", container.Summary{State: container.StateRunning, Status: "Up 2 hours"}, noryxv1.ServerState_SERVER_STATE_RUNNING, false, false},
		// It runs, so restarts, stops and the console work as usual.
		{"unhealthy", container.Summary{State: container.StateRunning, Status: "Up 2 hours (unhealthy)", Health: health(container.Unhealthy)}, noryxv1.ServerState_SERVER_STATE_RUNNING, true, false},
		// Starting again after a crash, which only inspecting tells.
		{"starting", container.Summary{State: container.StateRunning, Health: health(container.Starting)}, noryxv1.ServerState_SERVER_STATE_STARTING, false, true},
		{"restarting", container.Summary{State: container.StateRestarting, Status: "Restarting (1) 5 seconds ago", Health: health(container.Unhealthy)}, noryxv1.ServerState_SERVER_STATE_CRASHING, false, true},
		{"never started", container.Summary{State: container.StateCreated, Status: "Created"}, noryxv1.ServerState_SERVER_STATE_STOPPED, false, false},
		{"stopped cleanly", container.Summary{State: container.StateExited, Status: "Exited (0) 3 minutes ago"}, noryxv1.ServerState_SERVER_STATE_STOPPED, false, false},
		{"stopped after a crash", container.Summary{State: container.StateExited, Status: "Exited (1) 3 minutes ago"}, noryxv1.ServerState_SERVER_STATE_STOPPED, false, true},
	} {
		if got := state(tc.c); got != tc.state {
			t.Errorf("%s: state %v, want %v", tc.name, got, tc.state)
		}
		if got := unhealthy(tc.c); got != tc.unhealthy {
			t.Errorf("%s: unhealthy %v, want %v", tc.name, got, tc.unhealthy)
		}
		if got := mayHaveCrashed(tc.c); got != tc.inspect {
			t.Errorf("%s: may have crashed %v, want %v", tc.name, got, tc.inspect)
		}
	}
}

// Docker starts a container again right away after its first crash, before the agent inspects
// it, and later after a delay.
func TestRestarts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state container.State
		want  bool
	}{
		{"running again", container.State{Running: true}, true},
		{"waiting to start again", container.State{Running: true, Restarting: true}, true},
		{"stopped", container.State{Status: container.StateExited, ExitCode: 143}, false},
	} {
		if got := restarts(&tc.state); got != tc.want {
			t.Errorf("%s: restarts = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Every crash is logged once, the last of maxCrashes in a row as the server is stopped.
func TestNoteCrash(t *testing.T) {
	logged := captureLog(t)
	inRow := map[string]int{}
	crash := func(ran string, restarts int) bool {
		return noteCrash(inRow, events.Actor{ID: "c1", Attributes: map[string]string{"execDuration": ran, "exitCode": "1"}}, "s1", restarts)
	}
	for i := 1; i < maxCrashes; i++ {
		if crash("3", i) {
			t.Fatalf("crash %d stops the server", i)
		}
	}
	if !crash("3", maxCrashes) {
		t.Fatal("the last crash in a row doesn't stop the server")
	}
	// One that ran long enough starts a new row.
	if crash(strconv.Itoa(int(stableAfter.Seconds())), maxCrashes+1) || inRow["c1"] != 1 {
		t.Fatalf("crash after a stable run: %d in a row", inRow["c1"])
	}
	entries := logged()
	if len(entries) != maxCrashes+1 {
		t.Fatalf("logged %d entries, want %d: %v", len(entries), maxCrashes+1, entries)
	}
	for i, e := range entries {
		want := min(i+1, maxCrashes)
		if i == maxCrashes {
			want = 1
		}
		if e["level"] != "WARN" || e["server"] != "s1" || e["exit_code"] != "1" || e["crashes"] != float64(want) {
			t.Errorf("entry %d = %v, want crash %d of s1", i, e, want)
		}
	}
	if !strings.HasPrefix(entries[maxCrashes-1]["msg"].(string), "Stop a server") {
		t.Errorf("last crash in a row: %v", entries[maxCrashes-1])
	}
}

// A failing health check is logged once, and passing again only after it failed.
func TestHealth(t *testing.T) {
	logged := captureLog(t)
	unhealthy := map[string]bool{}
	labels := map[string]string{labelSpec: `{"ID":"s1"}`}
	for _, action := range []events.Action{
		events.ActionHealthStatusHealthy, // after the start
		events.ActionHealthStatusUnhealthy,
		events.ActionHealthStatusUnhealthy,
		events.ActionHealthStatusHealthy,
	} {
		health(events.Message{Action: action, Actor: events.Actor{ID: "c1", Attributes: labels}}, unhealthy)
	}
	entries := logged()
	if len(entries) != 2 || entries[0]["level"] != "WARN" || entries[1]["level"] != "INFO" || entries[0]["server"] != "s1" {
		t.Fatalf("logged %v, want a warning and the recovery", entries)
	}
}

// captureLog sends the default logger's entries to a buffer, whose entries the returned
// function decodes.
func captureLog(t *testing.T) func() []map[string]any {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return func() []map[string]any {
		var entries []map[string]any
		for line := range strings.Lines(buf.String()) {
			var e map[string]any
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				t.Fatal(err)
			}
			entries = append(entries, e)
		}
		return entries
	}
}
