package docker

import (
	"testing"

	"github.com/moby/moby/api/types/container"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

func TestCrashState(t *testing.T) {
	starting := &container.HealthSummary{Status: container.Starting}
	for _, tc := range []struct {
		name    string
		c       container.Summary
		state   noryxv1.ServerState
		inspect bool
	}{
		{"running", container.Summary{State: container.StateRunning, Status: "Up 2 hours (healthy)"}, noryxv1.ServerState_SERVER_STATE_RUNNING, false},
		// Starting again after a crash, which only inspecting tells.
		{"starting", container.Summary{State: container.StateRunning, Health: starting}, noryxv1.ServerState_SERVER_STATE_STARTING, true},
		{"restarting", container.Summary{State: container.StateRestarting, Status: "Restarting (1) 5 seconds ago"}, noryxv1.ServerState_SERVER_STATE_CRASHING, true},
		{"never started", container.Summary{State: container.StateCreated, Status: "Created"}, noryxv1.ServerState_SERVER_STATE_STOPPED, false},
		{"stopped cleanly", container.Summary{State: container.StateExited, Status: "Exited (0) 3 minutes ago"}, noryxv1.ServerState_SERVER_STATE_STOPPED, false},
		{"stopped after a crash", container.Summary{State: container.StateExited, Status: "Exited (1) 3 minutes ago"}, noryxv1.ServerState_SERVER_STATE_STOPPED, true},
	} {
		if got := state(tc.c); got != tc.state {
			t.Errorf("%s: state %v, want %v", tc.name, got, tc.state)
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
