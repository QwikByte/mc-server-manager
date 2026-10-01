package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	mcnet "github.com/QwikByte/mc-server-manager/internal/agent/network"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
)

func TestStandalone(t *testing.T) {
	// A copied backend no longer trusts the proxy of the original.
	backend := t.TempDir()
	config, _, err := mcnet.PaperGlobal(nil, "s3cretS3cretS3cret")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(backend, "config"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backend, "config", "paper-global.yml"), config, 0o600); err != nil {
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
}
