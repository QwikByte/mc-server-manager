package e2e

import (
	"net"
	"net/http"
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
	survival := m.createServer(t, a2, "Survival", mcsmv1.ServerType_SERVER_TYPE_PURPUR, 25570)
	api := apiClient{t, m.panel(t).URL}

	var n network.Network
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "lobby": lobby}, http.StatusCreated, &n)
	secret := a1.runtime.network(lobby.ServerID).ForwardingSecret
	if secret == "" {
		t.Fatal("the lobby does not trust the proxy")
	}
	wantProxy := func(backends ...runtime.NetworkBackend) {
		t.Helper()
		got := a1.runtime.network(proxy.ServerID)
		if got.ForwardingSecret != secret || !slices.Equal(got.Backends, backends) {
			t.Fatalf("proxy network = %+v, want backends %+v", got, backends)
		}
	}
	lobbyBackend := runtime.NetworkBackend{Name: "lobby", ServerID: lobby.ServerID}
	wantProxy(lobbyBackend)

	// A server on another node is reached at its node's address.
	api.do("POST", "/api/networks/"+n.ID+"/backends", survival, http.StatusOK, &n)
	if got := a2.runtime.network(survival.ServerID).ForwardingSecret; got != secret {
		t.Fatalf("survival secret = %q, want the network's", got)
	}
	host, _, _ := net.SplitHostPort(a2.node.Address)
	survivalBackend := runtime.NetworkBackend{Name: "survival", Address: net.JoinHostPort(host, "25570")}
	wantProxy(lobbyBackend, survivalBackend)

	api.do("POST", "/api/networks/"+n.ID+"/backends/"+survival.ServerID+"/default", nil, http.StatusOK, &n)
	wantProxy(survivalBackend, lobbyBackend)

	// Invalid changes are rejected, and the secret never leaves the master.
	api.do("POST", "/api/networks/"+n.ID+"/backends", vanilla, http.StatusBadRequest, nil)
	api.do("POST", "/api/networks/"+n.ID+"/backends", proxy, http.StatusBadRequest, nil)
	api.do("POST", "/api/networks", map[string]any{"name": "main", "proxy": proxy, "lobby": lobby}, http.StatusConflict, nil)
	api.do("DELETE", "/api/nodes/"+lobby.NodeID+"/servers/"+lobby.ServerID, nil, http.StatusConflict, nil)
	if body := api.do("GET", "/api/networks", nil, http.StatusOK, nil); strings.Contains(body, secret) {
		t.Fatal("the API exposes the forwarding secret")
	}

	// Removed servers stop trusting the proxy; the last one stays.
	api.do("DELETE", "/api/networks/"+n.ID+"/backends/"+lobby.ServerID, nil, http.StatusOK, &n)
	if a1.runtime.network(lobby.ServerID).ForwardingSecret != "" {
		t.Fatal("the removed lobby still trusts the proxy")
	}
	wantProxy(survivalBackend)
	api.do("DELETE", "/api/networks/"+n.ID+"/backends/"+survival.ServerID, nil, http.StatusConflict, nil)

	api.do("DELETE", "/api/networks/"+n.ID, nil, http.StatusNoContent, nil)
	if a2.runtime.network(survival.ServerID).ForwardingSecret != "" {
		t.Fatal("survival still trusts the proxy of the deleted network")
	}
	wantProxy()
	if body := api.do("GET", "/api/networks", nil, http.StatusOK, nil); body != "[]\n" {
		t.Fatalf("networks after delete = %s", body)
	}
}
