package docker

import (
	"testing"

	"github.com/moby/moby/api/types/container"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
)

func TestCrashState(t *testing.T) {
	starting := &container.HealthSummary{Status: container.Starting}
	for _, tc := range []struct {
		name    string
		c       container.Summary
		state   mcsmv1.ServerState
		inspect bool
	}{
		{"running", container.Summary{State: container.StateRunning, Status: "Up 2 hours (healthy)"}, mcsmv1.ServerState_SERVER_STATE_RUNNING, false},
		// Starting again after a crash, which only inspecting tells.
		{"starting", container.Summary{State: container.StateRunning, Health: starting}, mcsmv1.ServerState_SERVER_STATE_STARTING, true},
		{"restarting", container.Summary{State: container.StateRestarting, Status: "Restarting (1) 5 seconds ago"}, mcsmv1.ServerState_SERVER_STATE_CRASHING, true},
		{"never started", container.Summary{State: container.StateCreated, Status: "Created"}, mcsmv1.ServerState_SERVER_STATE_STOPPED, false},
		{"stopped cleanly", container.Summary{State: container.StateExited, Status: "Exited (0) 3 minutes ago"}, mcsmv1.ServerState_SERVER_STATE_STOPPED, false},
		{"stopped after a crash", container.Summary{State: container.StateExited, Status: "Exited (1) 3 minutes ago"}, mcsmv1.ServerState_SERVER_STATE_STOPPED, true},
	} {
		if got := state(tc.c); got != tc.state {
			t.Errorf("%s: state %v, want %v", tc.name, got, tc.state)
		}
		if got := mayHaveCrashed(tc.c); got != tc.inspect {
			t.Errorf("%s: may have crashed %v, want %v", tc.name, got, tc.inspect)
		}
	}
}
