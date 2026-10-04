package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
	mcnet "github.com/QwikByte/mc-server-manager/internal/agent/network"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
)

func TestStandalone(t *testing.T) {
	// A copied backend no longer trusts the proxy of the original.
	backend := t.TempDir()
	dir, err := datadir.Open(backend)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if _, err := mcnet.WriteBackend(dir, mcsmv1.ServerType_SERVER_TYPE_PAPER, runtime.ForwardingModern, "s3cretS3cretS3cret"); err != nil {
		t.Fatal(err)
	}
	if err := standalone(backend, runtime.Spec{Type: mcsmv1.ServerType_SERVER_TYPE_PAPER, BehindProxy: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(backend, "config", "paper-global.yml"))
	if err != nil || strings.Contains(string(data), "s3cret") || !strings.Contains(string(data), "enabled: false") {
		t.Fatalf("paper-global.yml = %s, %v", data, err)
	}

	// A copied proxy loses the forwarding secret; Velocity creates a new one.
	proxy := t.TempDir()
	if err := os.WriteFile(filepath.Join(proxy, mcnet.ForwardingSecretFile), []byte("s3cret"), 0o600); err != nil {
		t.Fatal(err)
	}
	spec := runtime.Spec{Type: mcsmv1.ServerType_SERVER_TYPE_VELOCITY}
	if err := standalone(proxy, spec); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(proxy, mcnet.ForwardingSecretFile)); !os.IsNotExist(err) {
		t.Fatalf("the forwarding secret was copied: %v", err)
	}
	if err := standalone(proxy, spec); err != nil {
		t.Fatalf("proxy without a secret: %v", err)
	}

	// A copied BungeeCord proxy stops forwarding, as only the original may reach the backends.
	bungee := t.TempDir()
	if err := os.WriteFile(filepath.Join(bungee, "config.yml"), []byte("ip_forward: true\nservers:\n  lobby:\n    address: 203.0.113.7:25565\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := standalone(bungee, runtime.Spec{Type: mcsmv1.ServerType_SERVER_TYPE_WATERFALL}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(bungee, "config.yml")); err != nil || !strings.Contains(string(data), "ip_forward: false") {
		t.Fatalf("config.yml = %s, %v", data, err)
	}
}
