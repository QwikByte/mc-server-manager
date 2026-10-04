package network

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

// pluginRuntime has one Velocity proxy whose console acts like the Maintenance plugin.
type pluginRuntime struct {
	runtime.Runtime
	dir      string
	mu       sync.Mutex
	srv      runtime.Server
	commands []string
}

const proxyID = "aaaaaaaaaaaaaaaaaaaaaaaaaa"

func (p *pluginRuntime) List(context.Context) ([]runtime.Server, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return []runtime.Server{p.srv}, nil
}

func (p *pluginRuntime) Data(context.Context, string) (*datadir.Dir, error) {
	return datadir.Open(p.dir)
}

func (p *pluginRuntime) SendCommand(_ context.Context, _, command string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.commands = append(p.commands, command)
	switch command {
	case "maintenance on":
		p.write("config.yml", "# Enables maintenance mode.\nmaintenance-enabled: true\n")
	case "maintenance add Steve":
		p.write("WhitelistedPlayers.yml", "8667ba71-b85a-4004-af54-457a9734eed7: Steve\n")
	}
	return "", nil
}

func (p *pluginRuntime) write(name, content string) {
	_ = os.WriteFile(filepath.Join(p.dir, "plugins", "maintenance", name), []byte(content), 0o600)
}

func TestMaintenance(t *testing.T) {
	ctx := t.Context()
	rt := &pluginRuntime{dir: t.TempDir(), srv: runtime.Server{
		Spec: runtime.Spec{ID: proxyID, Type: noryxv1.ServerType_SERVER_TYPE_VELOCITY}, State: noryxv1.ServerState_SERVER_STATE_RUNNING,
	}}
	s := NewService(rt)
	change := func(c noryxv1.MaintenanceChange, player string) (*noryxv1.Maintenance, error) {
		res, err := s.ChangeMaintenance(ctx, &noryxv1.ChangeMaintenanceRequest{ServerId: proxyID, Change: c, Player: player})
		return res.GetMaintenance(), err
	}

	// Without its files, the plugin isn't loaded and can't be told anything.
	res, err := s.GetMaintenance(ctx, &noryxv1.GetMaintenanceRequest{ServerId: proxyID})
	if err != nil || res.GetMaintenance().GetInstalled() {
		t.Fatalf("maintenance without the plugin = %v, %v", res, err)
	}
	if _, err := change(noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ON, ""); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("turning on without the plugin: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(rt.dir, "plugins", "maintenance"), 0o750); err != nil {
		t.Fatal(err)
	}
	rt.write("config.yml", "maintenance-enabled: false\n")

	for _, tc := range []struct {
		change noryxv1.MaintenanceChange
		player string
	}{
		{noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ON, "Steve"},
		{noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ADD, ""},
		{noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ADD, "@a"},
		{noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_UNSPECIFIED, ""},
	} {
		if _, err := change(tc.change, tc.player); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%v %q: %v", tc.change, tc.player, err)
		}
	}

	// A change returns once the plugin saved it; one that is done already runs no command.
	if m, err := change(noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ON, ""); err != nil || !m.GetEnabled() {
		t.Fatalf("turning on = %v, %v", m, err)
	}
	m, err := change(noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ADD, "Steve")
	if err != nil || len(m.GetPlayers()) != 1 || m.GetPlayers()[0].GetName() != "Steve" {
		t.Fatalf("adding a player = %v, %v", m, err)
	}
	if _, err := change(noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ON, ""); err != nil {
		t.Fatal(err)
	}
	if want := []string{"maintenance on", "maintenance add Steve"}; !slices.Equal(rt.commands, want) {
		t.Fatalf("commands = %q, want %q", rt.commands, want)
	}

	// The proxy must run.
	rt.srv.State = noryxv1.ServerState_SERVER_STATE_STOPPED
	if _, err := change(noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_OFF, ""); status.Code(err) != codes.FailedPrecondition || !strings.Contains(err.Error(), "Start") {
		t.Fatalf("change on a stopped proxy: %v", err)
	}
}
