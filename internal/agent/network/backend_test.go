package network

import (
	"errors"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

const paperGlobal = "chunk-system:\n  io-threads: 4\nproxies:\n  velocity:\n    enabled: true\n    online-mode: true\n    secret: old\n"

func TestWriteBackendPaper(t *testing.T) {
	dir := dataDir(t, map[string]string{"config/paper-global.yml": paperGlobal})
	write := func(f runtime.Forwarding, secret string) bool {
		t.Helper()
		changed, err := WriteBackend(dir, noryxv1.ServerType_SERVER_TYPE_PAPER, f, secret)
		if err != nil {
			t.Fatal(err)
		}
		return changed
	}
	if !write(runtime.ForwardingModern, "s3cret") || write(runtime.ForwardingModern, "s3cret") {
		t.Fatal("joining changed nothing, or joining again changed something")
	}
	want(t, read(t, dir, "config/paper-global.yml"), map[string]any{
		"proxies/velocity": map[string]any{"enabled": true, "online-mode": true, "secret": "s3cret"},
		"chunk-system":     map[string]any{"io-threads": 4},
	})
	if _, err := dir.ReadFile("spigot.yml"); err == nil {
		t.Fatal("modern forwarding wrote spigot.yml")
	}

	// Legacy forwarding is BungeeCord's, which Spigot turns on.
	if !write(runtime.ForwardingLegacy, "s3cret") {
		t.Fatal("switching to legacy forwarding changed nothing")
	}
	want(t, read(t, dir, "config/paper-global.yml"), map[string]any{"proxies/velocity/enabled": false, "proxies/velocity/secret": ""})
	want(t, read(t, dir, "spigot.yml"), map[string]any{"settings/bungeecord": true})

	if !write(runtime.ForwardingNone, "") {
		t.Fatal("leaving changed nothing")
	}
	want(t, read(t, dir, "spigot.yml"), map[string]any{"settings/bungeecord": false})
}

func TestWriteBackendKeepsUntouchedFiles(t *testing.T) {
	const commented = "# Paper's comments\nproxies:\n  velocity:\n    enabled: false\n    online-mode: true\n    secret: ''\n"
	dir := dataDir(t, map[string]string{"config/paper-global.yml": commented})
	if changed, err := WriteBackend(dir, noryxv1.ServerType_SERVER_TYPE_PURPUR, runtime.ForwardingNone, ""); changed || err != nil {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	if data, _ := dir.ReadFile("config/paper-global.yml"); string(data) != commented {
		t.Fatalf("file was rewritten:\n%s", data)
	}
}

// Forks of Paper and Quilt are configured like Paper and Fabric.
func TestWriteBackendForks(t *testing.T) {
	for _, typ := range []noryxv1.ServerType{noryxv1.ServerType_SERVER_TYPE_FOLIA, noryxv1.ServerType_SERVER_TYPE_LEAF} {
		dir := dataDir(t, nil)
		if _, err := WriteBackend(dir, typ, runtime.ForwardingModern, "s3cret"); err != nil {
			t.Fatal(err)
		}
		want(t, read(t, dir, PaperGlobalFile), map[string]any{"proxies/velocity/secret": "s3cret"})
	}
	dir := dataDir(t, nil)
	if _, err := WriteBackend(dir, noryxv1.ServerType_SERVER_TYPE_QUILT, runtime.ForwardingModern, "s3cret"); err != nil {
		t.Fatal(err)
	}
	want(t, read(t, dir, FabricProxyFile), map[string]any{"secret": "s3cret"})
	if _, err := WriteBackend(dir, noryxv1.ServerType_SERVER_TYPE_QUILT, runtime.ForwardingLegacy, ""); !errors.Is(err, ErrModernOnly) {
		t.Fatalf("legacy forwarding on Quilt: err = %v", err)
	}
}

func TestWriteBackendMods(t *testing.T) {
	dir := dataDir(t, nil)
	if changed, err := WriteBackend(dir, noryxv1.ServerType_SERVER_TYPE_FABRIC, runtime.ForwardingModern, "s3cret"); !changed || err != nil {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	want(t, read(t, dir, "config/FabricProxy-Lite.toml"), map[string]any{"secret": "s3cret"})
	if _, err := WriteBackend(dir, noryxv1.ServerType_SERVER_TYPE_FABRIC, runtime.ForwardingLegacy, ""); !errors.Is(err, ErrModernOnly) {
		t.Fatalf("legacy forwarding on Fabric: err = %v", err)
	}

	for f, mode := range map[runtime.Forwarding]string{runtime.ForwardingModern: "MODERN", runtime.ForwardingLegacy: "LEGACY"} {
		if _, err := WriteBackend(dir, noryxv1.ServerType_SERVER_TYPE_NEOFORGE, f, "s3cret"); err != nil {
			t.Fatal(err)
		}
		secret := map[runtime.Forwarding]string{runtime.ForwardingModern: "s3cret"}[f]
		want(t, read(t, dir, "config/proxy-compatible-forge.toml"), map[string]any{"forwarding": map[string]any{"enabled": true, "mode": mode, "secret": secret}})
	}
	if _, err := WriteBackend(dir, noryxv1.ServerType_SERVER_TYPE_FORGE, runtime.ForwardingNone, ""); err != nil {
		t.Fatal(err)
	}
	want(t, read(t, dir, "config/proxy-compatible-forge.toml"), map[string]any{"forwarding/enabled": false, "forwarding/secret": ""})

	// A server that never joined a network gets no configuration of a mod it doesn't have.
	dir = dataDir(t, nil)
	if changed, err := WriteBackend(dir, noryxv1.ServerType_SERVER_TYPE_FABRIC, runtime.ForwardingNone, ""); changed || err != nil {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	if _, err := WriteBackend(dir, noryxv1.ServerType_SERVER_TYPE_VANILLA, runtime.ForwardingModern, "s3cret"); !errors.Is(err, runtime.ErrUnsupported) {
		t.Fatalf("vanilla: err = %v", err)
	}
}
