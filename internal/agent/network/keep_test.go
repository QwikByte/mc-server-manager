package network

import (
	"os"
	"strings"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

// A restored Velocity configuration keeps the forwarding of the proxy as it is now, also
// without the secret that Velocity 1 kept in it, and the rest as it was.
func TestKeepForwardingVelocity(t *testing.T) {
	old := "forwarding-secret = \"old\"\n" + strings.Replace(defaultVelocity, `"NONE"`, `"MODERN"`, 1)
	current := dataDir(t, map[string]string{"velocity.toml": defaultVelocity})
	restored := dataDir(t, map[string]string{"velocity.toml": old})
	if got := read(t, restored, "velocity.toml"); got["forwarding-secret"] != "old" {
		t.Fatalf("the backup has no secret of Velocity 1: %v", got)
	}
	if err := KeepForwarding(current, restored, noryxv1.ServerType_SERVER_TYPE_VELOCITY); err != nil {
		t.Fatal(err)
	}
	got := read(t, restored, "velocity.toml")
	want(t, got, map[string]any{"player-info-forwarding-mode": "NONE", "forwarding-secret-file": "forwarding.secret", "servers/lobby": "127.0.0.1:30066"})
	if _, ok := got["forwarding-secret"]; ok {
		t.Error("the restored configuration kept the secret of Velocity 1")
	}
}

// A game server that left its network since its backup was made gets back neither the
// forwarding secret nor the trust in its proxy, nor offline mode.
func TestKeepForwardingLeftNetwork(t *testing.T) {
	const left = "proxies:\n  velocity:\n    enabled: false\n    online-mode: true\n    secret: ''\n"
	current := dataDir(t, map[string]string{
		PaperGlobalFile: left, "spigot.yml": "settings:\n  bungeecord: false\n", "server.properties": "online-mode=true\nmotd=now\n",
	})
	restored := dataDir(t, map[string]string{
		PaperGlobalFile: paperGlobal, "spigot.yml": "settings:\n  bungeecord: true\n  timeout-time: 60\n",
		"server.properties": "motd=then\nonline-mode=false\nenforce-secure-profile=false\n",
	})
	if err := KeepForwarding(current, restored, noryxv1.ServerType_SERVER_TYPE_PAPER); err != nil {
		t.Fatal(err)
	}
	want(t, read(t, restored, PaperGlobalFile), map[string]any{
		"proxies/velocity": map[string]any{"enabled": false, "online-mode": true, "secret": ""},
		"chunk-system":     map[string]any{"io-threads": 4},
	})
	want(t, read(t, restored, "spigot.yml"), map[string]any{"settings": map[string]any{"bungeecord": false, "timeout-time": 60}})
	// Properties that current lacks have Minecraft's defaults.
	if data, _ := restored.ReadFile("server.properties"); string(data) != "motd=then\nonline-mode=true\nenforce-secure-profile=true\n" {
		t.Errorf("server.properties = %q", data)
	}
}

// A game server keeps the newer secret of its network, and a setting that current lacks,
// also in a file it can't read, is removed; files that the backup lacks stay missing.
func TestKeepForwardingCurrentSecret(t *testing.T) {
	current := dataDir(t, map[string]string{FabricProxyFile: "secret = \"new\"\n", ForgeProxyFile: "[forwarding\n"})
	restored := dataDir(t, map[string]string{FabricProxyFile: "secret = \"old\"\nhackOnlineMode = true\n"})
	if err := KeepForwarding(current, restored, noryxv1.ServerType_SERVER_TYPE_QUILT); err != nil {
		t.Fatal(err)
	}
	want(t, read(t, restored, FabricProxyFile), map[string]any{"secret": "new", "hackOnlineMode": true})
	if _, err := restored.Lstat("server.properties"); !os.IsNotExist(err) {
		t.Errorf("server.properties was created: %v", err)
	}

	restored = dataDir(t, map[string]string{ForgeProxyFile: "[forwarding]\nenabled = true\nmode = \"MODERN\"\nsecret = \"old\"\n"})
	if err := KeepForwarding(current, restored, noryxv1.ServerType_SERVER_TYPE_NEOFORGE); err != nil {
		t.Fatal(err)
	}
	if got := read(t, restored, ForgeProxyFile); len(got["forwarding"].(map[string]any)) != 0 {
		t.Errorf("forwarding = %v", got["forwarding"])
	}

	// An invalid file of the backup fails, as its settings can't be kept.
	restored = dataDir(t, map[string]string{FabricProxyFile: "secret = \n"})
	if err := KeepForwarding(current, restored, noryxv1.ServerType_SERVER_TYPE_FABRIC); err == nil {
		t.Error("an invalid file was restored")
	}
}
