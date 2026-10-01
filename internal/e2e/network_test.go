package e2e

import (
	"bytes"
	"encoding/json"
	"io"
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

// createServer creates a server on the agent of a node.
func (m *master) createServer(t *testing.T, a agent, name string, typ mcsmv1.ServerType, port uint32) network.Ref {
	conn, err := m.nodes.Conn(t.Context(), a.node.ID)
	check(t, err)
	res, err := mcsmv1.NewServerServiceClient(conn).CreateServer(t.Context(), &mcsmv1.CreateServerRequest{
		Name: name, Type: typ, MemoryMb: 1024, Port: port, AcceptEula: true,
	})
	check(t, err)
	return network.Ref{NodeID: a.node.ID, ServerID: res.GetServer().GetId()}
}

type apiClient struct {
	t   *testing.T
	url string
}

// do sends a request with an optional JSON body, checks the status and decodes the
// response into out unless it is nil. It returns the response body.
func (c apiClient) do(method, path string, in any, wantStatus int, out any) string {
	c.t.Helper()
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		check(c.t, err)
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(c.t.Context(), method, c.url+path, body)
	check(c.t, err)
	res, err := http.DefaultClient.Do(req)
	check(c.t, err)
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	check(c.t, err)
	if res.StatusCode != wantStatus {
		c.t.Fatalf("%s %s: status %d, want %d: %s", method, path, res.StatusCode, wantStatus, data)
	}
	if out != nil {
		check(c.t, json.Unmarshal(data, out))
	}
	return string(data)
}
