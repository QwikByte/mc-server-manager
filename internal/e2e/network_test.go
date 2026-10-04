package e2e

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
	"github.com/QwikByte/mc-server-manager/internal/master/network"
)

func TestNetwork(t *testing.T) {
	m := startMaster(t)
	a1, a2 := m.startAgent(t, "node-1"), m.startAgent(t, "node-2")
	proxy := m.createServer(t, a1, "Proxy", mcsmv1.ServerType_SERVER_TYPE_VELOCITY, 25577)
	lobby := m.createServer(t, a1, "Lobby", mcsmv1.ServerType_SERVER_TYPE_PAPER, 25565)
	vanilla := m.createServer(t, a1, "Vanilla", mcsmv1.ServerType_SERVER_TYPE_VANILLA, 25566)
	fabric := m.createServer(t, a1, "Fabric", mcsmv1.ServerType_SERVER_TYPE_FABRIC, 25567)
	survival := m.createServer(t, a2, "Survival", mcsmv1.ServerType_SERVER_TYPE_PURPUR, 25570)
	api := apiClient{t: t, url: m.panel(t).URL}

	var n network.Network
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "servers": []network.Ref{lobby, survival}}, http.StatusCreated, &n)
	if n.Forwarding != network.Modern || n.ProxyType != "velocity" || !slices.Equal(n.Try, []string{"lobby"}) {
		t.Fatalf("network = %+v", n)
	}
	secret := a1.runtime.network(lobby.ServerID).ForwardingSecret
	if got := a1.runtime.network(lobby.ServerID); secret == "" || got.Forwarding != runtime.ForwardingModern || !got.ProxyOnNode {
		t.Fatalf("lobby network = %+v", got)
	}
	if got := a2.runtime.network(survival.ServerID); got.ForwardingSecret != secret || got.ProxyOnNode {
		t.Fatalf("survival network = %+v", got)
	}
	host, _, _ := net.SplitHostPort(a2.node.Address)
	lobbyBackend := runtime.NetworkBackend{Name: "lobby", ServerID: lobby.ServerID}
	survivalBackend := runtime.NetworkBackend{Name: "survival", Address: net.JoinHostPort(host, "25570")}
	wantProxy := func(forwarding runtime.Forwarding, try []string, hosts []runtime.ForcedHost, backends ...runtime.NetworkBackend) {
		t.Helper()
		got := a1.runtime.network(proxy.ServerID)
		if got.Forwarding != forwarding || !slices.Equal(got.Try, try) || !slices.EqualFunc(got.ForcedHosts, hosts, sameHost) ||
			!slices.Equal(got.Backends, backends) {
			t.Fatalf("proxy network = %+v, want %v %v %v %+v", got, forwarding, try, hosts, backends)
		}
	}
	wantProxy(runtime.ForwardingModern, []string{"lobby"}, nil, lobbyBackend, survivalBackend)

	// A Fabric server gets FabricProxy-Lite and the Fabric API it requires; renames, the
	// servers players try and the forced hosts go into the proxy's configuration.
	change := network.Change{
		Name: "Main", Forwarding: network.Modern,
		Backends: []network.Backend{
			{Ref: lobby, Name: "lobby"}, {Ref: survival, Name: "smp"}, {Ref: fabric, Name: "fabric"},
		},
		Try:         []string{"lobby", "smp"},
		ForcedHosts: []network.ForcedHost{{Host: "SMP.Example.com", Servers: []string{"smp", "lobby"}}},
	}
	api.do("PUT", "/api/networks/"+n.ID, change, http.StatusOK, &n)
	if !slices.Equal(n.ForcedHosts[0].Servers, []string{"smp", "lobby"}) || n.ForcedHosts[0].Host != "smp.example.com" {
		t.Fatalf("forced hosts = %+v", n.ForcedHosts)
	}
	if mods := files(t, filepath.Join(a1.runtime.dir, fabric.ServerID, "mods")); !slices.Equal(mods, []string{"8dI2tmqs-2.10.jar", "fabricapi-0.119.jar"}) {
		t.Fatalf("mods of the Fabric server = %q", mods)
	}
	smp := runtime.NetworkBackend{Name: "smp", Address: survivalBackend.Address}
	fabricBackend := runtime.NetworkBackend{Name: "fabric", ServerID: fabric.ServerID}
	wantProxy(runtime.ForwardingModern, []string{"lobby", "smp"}, []runtime.ForcedHost{{Host: "smp.example.com", Servers: []string{"smp", "lobby"}}},
		lobbyBackend, smp, fabricBackend)

	// Invalid changes are rejected, and the secret never leaves the master.
	invalid := func(status int, edit func(c *network.Change)) {
		t.Helper()
		c := change
		c.Backends, c.Try = slices.Clone(change.Backends), slices.Clone(change.Try)
		edit(&c)
		api.do("PUT", "/api/networks/"+n.ID, c, status, nil)
	}
	invalid(http.StatusBadRequest, func(c *network.Change) {
		c.Backends = append(c.Backends, network.Backend{Ref: vanilla, Name: "vanilla"})
	})
	invalid(http.StatusBadRequest, func(c *network.Change) { c.Backends = append(c.Backends, network.Backend{Ref: proxy, Name: "proxy"}) })
	invalid(http.StatusBadRequest, func(c *network.Change) { c.Try = []string{"nope"} })
	invalid(http.StatusBadRequest, func(c *network.Change) { c.Backends[1].Name = "lobby" })
	invalid(http.StatusBadRequest, func(c *network.Change) { c.Forwarding = network.Legacy; c.Firewalled = true }) // Fabric
	invalid(http.StatusConflict, func(c *network.Change) { c.Forwarding = network.Legacy; c.Backends = c.Backends[:2] })
	api.do("POST", "/api/networks", map[string]any{"name": "main", "proxy": proxy, "servers": []network.Ref{lobby}}, http.StatusConflict, nil)
	api.do("DELETE", "/api/nodes/"+lobby.NodeID+"/servers/"+lobby.ServerID, nil, http.StatusConflict, nil)
	if body := api.do("GET", "/api/networks", nil, http.StatusOK, nil); strings.Contains(body, secret) {
		t.Fatal("the API exposes the forwarding secret")
	}

	// A server that leaves loses its forwarding mod and accepts players directly again.
	change.Backends, change.Forwarding, change.Firewalled = change.Backends[:2], network.Legacy, true
	api.do("PUT", "/api/networks/"+n.ID, change, http.StatusOK, &n)
	if mods := files(t, filepath.Join(a1.runtime.dir, fabric.ServerID, "mods")); !slices.Equal(mods, []string{"fabricapi-0.119.jar"}) {
		t.Fatalf("mods of the Fabric server after leaving = %q", mods)
	}
	if got := a1.runtime.network(fabric.ServerID); got.Forwarding != runtime.ForwardingNone {
		t.Fatalf("fabric network after leaving = %+v", got)
	}
	// Legacy forwarding has no secret.
	if got := a2.runtime.network(survival.ServerID); got.Forwarding != runtime.ForwardingLegacy || got.ForwardingSecret != "" {
		t.Fatalf("survival network with legacy forwarding = %+v", got)
	}
	wantProxy(runtime.ForwardingLegacy, []string{"lobby", "smp"}, []runtime.ForcedHost{{Host: "smp.example.com", Servers: []string{"smp", "lobby"}}}, lobbyBackend, smp)

	// A server that moves to the proxy's node is reached there and still trusts the proxy.
	api.do("POST", "/api/nodes/"+survival.NodeID+"/servers/"+survival.ServerID+"/move", map[string]any{"node": a1.node.ID}, http.StatusAccepted, nil)
	if mv := waitForMove(t, api, survival.ServerID); mv.Phase != "done" || len(mv.Warnings) != 0 {
		t.Fatalf("move = %+v", mv)
	}
	smp = runtime.NetworkBackend{Name: "smp", ServerID: survival.ServerID}
	wantProxy(runtime.ForwardingLegacy, []string{"lobby", "smp"}, []runtime.ForcedHost{{Host: "smp.example.com", Servers: []string{"smp", "lobby"}}}, lobbyBackend, smp)
	if got := a1.runtime.network(survival.ServerID); got.Forwarding != runtime.ForwardingLegacy || !got.ProxyOnNode {
		t.Fatalf("moved survival network = %+v", got)
	}
	// Without the confirmation of a firewall, no server of a legacy network moves away.
	change.Firewalled = false
	change.Backends[1].Ref = network.Ref{NodeID: a1.node.ID, ServerID: survival.ServerID}
	api.do("PUT", "/api/networks/"+n.ID, change, http.StatusOK, &n)
	api.do("POST", "/api/nodes/"+lobby.NodeID+"/servers/"+lobby.ServerID+"/move", map[string]any{"node": a2.node.ID}, http.StatusConflict, nil)

	// Actions on the whole network.
	api.do("POST", "/api/networks/"+n.ID+"/start", nil, http.StatusNoContent, nil)
	for _, ref := range []network.Ref{proxy, lobby} {
		if state := serverState(t, api, ref); state != "running" {
			t.Fatalf("%s after starting the network: %s", ref.ServerID, state)
		}
	}
	api.do("POST", "/api/networks/"+n.ID+"/broadcast", map[string]string{"message": "Restart in 5 minutes"}, http.StatusNoContent, nil)
	if got := a1.runtime.consoleCommands(); !slices.Contains(got, "say Restart in 5 minutes") {
		t.Fatalf("console commands = %q", got)
	}
	api.do("POST", "/api/networks/"+n.ID+"/broadcast", map[string]string{"message": " "}, http.StatusBadRequest, nil)

	// The proxy's own settings are edited in its configuration, which it reloads.
	settingsPath := "/api/nodes/" + proxy.NodeID + "/servers/" + proxy.ServerID + "/proxy"
	var settings network.ProxySettings
	api.do("GET", settingsPath, nil, http.StatusOK, &settings)
	if settings.Exists || settings.File != "velocity.toml" {
		t.Fatalf("settings before the first start = %+v", settings)
	}
	check(t, os.WriteFile(filepath.Join(a1.runtime.dir, proxy.ServerID, "velocity.toml"), []byte("motd = \"Hi\"\nbind = \"0.0.0.0:25565\"\n[advanced]\nlogin-ratelimit = 3000\n"), 0o600))
	api.do("GET", settingsPath, nil, http.StatusOK, &settings)
	if string(settings.Settings["advanced.login-ratelimit"]) != "3000" || len(settings.Locked) != 1 || settings.Locked[0].Key != "bind" {
		t.Fatalf("settings = %+v", settings)
	}
	var updated struct{ Reloaded bool }
	api.do("PUT", settingsPath, map[string]any{"settings": map[string]any{"motd": "<green>Welcome", "advanced.login-ratelimit": 1000}}, http.StatusOK, &updated)
	if config, _ := os.ReadFile(filepath.Join(a1.runtime.dir, proxy.ServerID, "velocity.toml")); !updated.Reloaded ||
		!strings.Contains(string(config), "<green>Welcome") || !slices.Contains(a1.runtime.reloaded(), proxy.ServerID) {
		t.Fatalf("reloaded = %v, velocity.toml = %s", updated.Reloaded, config)
	}
	api.do("PUT", settingsPath, map[string]any{"settings": map[string]any{"bind": "0.0.0.0:1"}}, http.StatusBadRequest, nil)
	api.do("PUT", settingsPath, map[string]any{"settings": map[string]any{"advanced.login-ratelimit": "fast"}}, http.StatusBadRequest, nil)

	api.do("POST", "/api/networks/"+n.ID+"/stop", nil, http.StatusNoContent, nil)
	if state := serverState(t, api, proxy); state != "stopped" {
		t.Fatalf("proxy after stopping the network: %s", state)
	}

	api.do("DELETE", "/api/networks/"+n.ID, nil, http.StatusNoContent, nil)
	for _, ref := range []network.Ref{lobby, {NodeID: a1.node.ID, ServerID: survival.ServerID}, proxy} {
		if got := a1.runtime.network(ref.ServerID); got.Forwarding != runtime.ForwardingNone {
			t.Fatalf("%s still forwards after the network was deleted: %+v", ref.ServerID, got)
		}
	}
	if body := api.do("GET", "/api/networks", nil, http.StatusOK, nil); body != "[]\n" {
		t.Fatalf("networks after delete = %s", body)
	}
}

func TestBungeeNetwork(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	proxy := m.createServer(t, a, "Proxy", mcsmv1.ServerType_SERVER_TYPE_WATERFALL, 25577)
	lobby := m.createServer(t, a, "Lobby", mcsmv1.ServerType_SERVER_TYPE_PAPER, 25565)
	forge := m.createServer(t, a, "Forge", mcsmv1.ServerType_SERVER_TYPE_FORGE, 25566)
	api := apiClient{t: t, url: m.panel(t).URL}

	api.do("POST", "/api/networks", map[string]any{"name": "Bungee", "proxy": proxy, "forwarding": "modern", "servers": []network.Ref{lobby}}, http.StatusBadRequest, nil)
	var n network.Network
	api.do("POST", "/api/networks", map[string]any{"name": "Bungee", "proxy": proxy, "servers": []network.Ref{lobby}}, http.StatusCreated, &n)
	if n.Forwarding != network.Legacy || n.ProxyType != "waterfall" {
		t.Fatalf("network = %+v", n)
	}
	change := network.Change{
		Name: n.Name, Forwarding: network.Legacy,
		Backends:    []network.Backend{{Ref: lobby, Name: "lobby", Motd: "&aLobby", Restricted: true}},
		Try:         []string{"lobby"},
		ForcedHosts: []network.ForcedHost{{Host: "play.example.com", Servers: []string{"lobby"}}},
	}
	api.do("PUT", "/api/networks/"+n.ID, change, http.StatusOK, &n)
	got := a.runtime.network(proxy.ServerID)
	if want := (runtime.NetworkBackend{Name: "lobby", ServerID: lobby.ServerID, Restricted: true, Motd: "&aLobby"}); got.Forwarding != runtime.ForwardingLegacy || !slices.Equal(got.Backends, []runtime.NetworkBackend{want}) {
		t.Fatalf("proxy network = %+v", got)
	}
	// BungeeCord sends a host name to one server. A Forge server can't join before its mod
	// is found on Modrinth, which the fake doesn't have.
	change.ForcedHosts[0].Servers = []string{"lobby", "lobby"}
	api.do("PUT", "/api/networks/"+n.ID, change, http.StatusBadRequest, nil)
	change.ForcedHosts = nil
	change.Backends = append(change.Backends, network.Backend{Ref: forge, Name: "forge"})
	api.do("PUT", "/api/networks/"+n.ID, change, http.StatusBadGateway, nil)
}

func sameHost(a, b runtime.ForcedHost) bool {
	return a.Host == b.Host && slices.Equal(a.Servers, b.Servers)
}

// files returns the names of the files in a folder.
func files(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	check(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// serverState returns the state of a server as the panel shows it.
func serverState(t *testing.T, api apiClient, ref network.Ref) string {
	t.Helper()
	var servers []struct{ ID, State string }
	api.do("GET", "/api/nodes/"+ref.NodeID+"/servers", nil, http.StatusOK, &servers)
	for _, s := range servers {
		if s.ID == ref.ServerID {
			return s.State
		}
	}
	return ""
}
