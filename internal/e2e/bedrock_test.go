package e2e

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/network"
	"github.com/QwikByte/noryx/internal/master/plugin"
)

func TestBedrock(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	proxy := m.createServer(t, a, "Proxy", noryxv1.ServerType_SERVER_TYPE_VELOCITY, 25577)
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	m.createServer(t, a, "Other", noryxv1.ServerType_SERVER_TYPE_PAPER, 19133)
	api := apiClient{t: t, url: m.panel(t).URL}
	var n network.Network
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "servers": []network.Ref{lobby}}, http.StatusCreated, &n)
	api.do("POST", "/api/nodes/"+proxy.NodeID+"/servers/"+proxy.ServerID+"/start", nil, http.StatusNoContent, nil)
	plugins := filepath.Join(a.runtime.dir, proxy.ServerID, "plugins")
	file := func(name string) string {
		data, _ := os.ReadFile(filepath.Join(plugins, name))
		return string(data)
	}
	change := network.Change{Name: "Main", Forwarding: network.Modern, Backends: n.Backends, Try: n.Try, ForcedHosts: n.ForcedHosts}
	update := func(port uint32, status int) {
		t.Helper()
		change.BedrockPort = port
		api.do("PUT", "/api/networks/"+n.ID, change, status, &n)
	}

	// Letting Bedrock players in installs Geyser and Floodgate on the proxy, which restarts
	// once to publish the port and load them, and the servers stop demanding signed chat.
	update(19132, http.StatusOK)
	if n.BedrockPort != 19132 || file("Geyser-Velocity.jar") != "geyser velocity" || file("floodgate-velocity.jar") != "floodgate velocity 141" {
		t.Fatalf("network = %+v, plugins = %q", n, files(t, plugins))
	}
	if !a.runtime.spec(lobby.ServerID).BedrockPlayers || a.runtime.spec(proxy.ServerID).BedrockPlayers {
		t.Fatalf("lobby = %+v, proxy = %+v", a.runtime.spec(lobby.ServerID), a.runtime.spec(proxy.ServerID))
	}
	if a.runtime.spec(proxy.ServerID).BedrockPort != 19132 || a.runtime.network(proxy.ServerID).BedrockPort != 19132 {
		t.Fatalf("proxy = %+v", a.runtime.spec(proxy.ServerID))
	}
	if restarts := a.runtime.restarted(); !slices.Equal(restarts, []string{proxy.ServerID}) {
		t.Fatalf("restarts = %q", restarts)
	}
	// Applying it again changes nothing. A new build of Floodgate waits for the network to be
	// applied again, rather than restarting the proxy when it is changed, and then restarts it.
	api.do("POST", "/api/networks/"+n.ID+"/apply", nil, http.StatusOK, nil)
	if restarts := a.runtime.restarted(); len(restarts) != 1 {
		t.Fatalf("restarts = %q", restarts)
	}
	m.geysermc.publish(142)
	update(19132, http.StatusOK)
	if restarts := a.runtime.restarted(); len(restarts) != 1 || file("floodgate-velocity.jar") != "floodgate velocity 141" {
		t.Fatalf("restarts = %q, Floodgate = %q", restarts, file("floodgate-velocity.jar"))
	}
	api.do("POST", "/api/networks/"+n.ID+"/apply", nil, http.StatusOK, nil)
	if restarts := a.runtime.restarted(); len(restarts) != 2 || file("floodgate-velocity.jar") != "floodgate velocity 142" {
		t.Fatalf("restarts = %q, Floodgate = %q", restarts, file("floodgate-velocity.jar"))
	}
	// The plugins of the proxy name Geyser from Modrinth and Floodgate from GeyserMC.
	var listing plugin.Listing
	api.do("GET", "/api/nodes/"+proxy.NodeID+"/servers/"+proxy.ServerID+"/plugins", nil, http.StatusOK, &listing)
	titles := map[string]string{}
	for _, p := range listing.Plugins {
		if p.Project != nil {
			titles[p.FileName] = p.Project.Title + " " + p.Version
		}
	}
	if titles["Geyser-Velocity.jar"] != "Geyser 2.11.3-velocity" || titles["floodgate-velocity.jar"] != "Floodgate 2.2.5-b142" {
		t.Fatalf("plugins = %+v", listing)
	}

	// Floodgate's key never reaches the panel.
	check(t, os.MkdirAll(filepath.Join(plugins, "floodgate"), 0o750))
	check(t, os.WriteFile(filepath.Join(plugins, "floodgate", "key.pem"), []byte("0123456789abcdef"), 0o600))
	api.do("GET", "/api/nodes/"+proxy.NodeID+"/servers/"+proxy.ServerID+"/files/content?path=plugins/floodgate/key.pem", nil, http.StatusForbidden, nil)

	// The port must be free on the node and in range.
	update(19133, http.StatusConflict) // the port of Other
	update(25565, http.StatusConflict) // the port of Lobby
	update(25577, http.StatusConflict) // the proxy's own port
	update(80, http.StatusBadRequest)

	// Without Bedrock, the proxy loses the plugins and the port, and the servers demand
	// signed chat again.
	update(0, http.StatusOK)
	if file("Geyser-Velocity.jar") != "" || file("floodgate-velocity.jar") != "" || a.runtime.spec(proxy.ServerID).BedrockPort != 0 ||
		a.runtime.spec(lobby.ServerID).BedrockPlayers {
		t.Fatalf("plugins = %q, proxy = %+v", files(t, plugins), a.runtime.spec(proxy.ServerID))
	}
	// Deleting a network with Bedrock players removes them too.
	update(19140, http.StatusOK)
	api.do("DELETE", "/api/networks/"+n.ID, nil, http.StatusNoContent, nil)
	if file("Geyser-Velocity.jar") != "" || file("floodgate-velocity.jar") != "" || a.runtime.spec(proxy.ServerID).BedrockPort != 0 {
		t.Fatalf("plugins = %q, proxy = %+v", files(t, plugins), a.runtime.spec(proxy.ServerID))
	}

	// A network whose proxy is gone can still be deleted.
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "servers": []network.Ref{lobby}}, http.StatusCreated, &n)
	change.Backends, change.Try = n.Backends, n.Try
	update(19132, http.StatusOK)
	check(t, a.runtime.Remove(t.Context(), proxy.ServerID))
	api.do("DELETE", "/api/networks/"+n.ID, nil, http.StatusNoContent, nil)
}
