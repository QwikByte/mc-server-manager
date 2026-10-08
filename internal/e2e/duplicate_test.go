package e2e

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/runtime"
	"github.com/QwikByte/noryx/internal/master/network"
)

func TestDuplicateServer(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	m.createServer(t, a, "Survival", noryxv1.ServerType_SERVER_TYPE_PAPER, 25566)
	api := apiClient{t: t, url: m.panel(t).URL}
	base := "/api/nodes/" + lobby.NodeID + "/servers/" + lobby.ServerID
	data := filepath.Join(a.runtime.dir, lobby.ServerID)
	check(t, os.MkdirAll(filepath.Join(data, "world", "region"), 0o750))
	check(t, os.WriteFile(filepath.Join(data, "world", "region", "r.0.0.mca"), []byte("chunks"), 0o600))
	check(t, os.Symlink(t.TempDir(), filepath.Join(data, "outside")))

	// A running server saves its worlds before they are copied, and keeps running.
	api.do("POST", base+"/start", nil, http.StatusNoContent, nil)
	var copied struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Port     uint32 `json:"port"`
		State    string `json:"state"`
		MemoryMB uint32 `json:"memoryMb"`
	}
	api.do("POST", base+"/duplicate", map[string]any{"name": "Lobby 2", "port": 25567}, http.StatusCreated, &copied)
	if copied.Name != "Lobby 2" || copied.Port != 25567 || copied.State != "stopped" || copied.MemoryMB != 1024 {
		t.Fatalf("copy = %+v", copied)
	}
	if got, want := a.runtime.consoleCommands(), []string{"save-off", "save-all flush", "save-on"}; !slices.Equal(got, want) {
		t.Fatalf("console commands = %q, want %q", got, want)
	}
	region := filepath.Join(a.runtime.dir, copied.ID, "world", "region", "r.0.0.mca")
	if content, err := os.ReadFile(region); err != nil || string(content) != "chunks" {
		t.Fatalf("copied world = %q, %v", content, err)
	}
	if info, err := os.Stat(region); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("copied file mode = %v, %v", info.Mode(), err)
	}
	if _, err := os.Lstat(filepath.Join(a.runtime.dir, copied.ID, "outside")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the copy contains a symbolic link")
	}
	if spec := a.runtime.spec(copied.ID); spec.Type != noryxv1.ServerType_SERVER_TYPE_PAPER || spec.ID == lobby.ServerID {
		t.Fatalf("copied spec = %+v", spec)
	}

	// The copy needs a free port and a valid name.
	api.do("POST", base+"/duplicate", map[string]any{"name": "Lobby 3", "port": 25566}, http.StatusConflict, nil)
	api.do("POST", base+"/duplicate", map[string]any{"name": "../x", "port": 25568}, http.StatusBadRequest, nil)
	api.do("POST", "/api/nodes/"+lobby.NodeID+"/servers/"+strings.Repeat("a", 26)+"/duplicate",
		map[string]any{"name": "X", "port": 25568}, http.StatusNotFound, nil)
}

func TestDuplicateIntoNetwork(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	proxy := m.createServer(t, a, "Proxy", noryxv1.ServerType_SERVER_TYPE_VELOCITY, 25577)
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	game := m.createServer(t, a, "Game", noryxv1.ServerType_SERVER_TYPE_PAPER, 25566)
	solo := m.createServer(t, a, "Solo", noryxv1.ServerType_SERVER_TYPE_PAPER, 25567)
	api := apiClient{t: t, url: m.panel(t).URL}
	var n network.Network
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "servers": []network.Ref{lobby, game}}, http.StatusCreated, &n)
	change := network.Change{
		Name: "Main", Forwarding: network.Modern, Backends: n.Backends, Try: []string{"lobby", "game"},
		ForcedHosts: []network.ForcedHost{{Host: "play.example.com", Servers: []string{"lobby"}}},
	}
	api.do("PUT", "/api/networks/"+n.ID, change, http.StatusOK, &n)
	duplicate := func(ref network.Ref) string {
		return "/api/nodes/" + ref.NodeID + "/servers/" + ref.ServerID + "/duplicate"
	}

	// The copy of a lobby takes its place next to it: among the servers, those players join
	// and those of host names. It trusts the proxy, which knows it.
	var copied struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	api.do("POST", duplicate(lobby), map[string]any{"name": "Lobby 2", "port": 25568, "network": true}, http.StatusCreated, &copied)
	api.do("GET", "/api/networks/"+n.ID, nil, http.StatusOK, &n)
	copyRef := network.Ref{NodeID: lobby.NodeID, ServerID: copied.ID}
	if len(n.Backends) != 3 || n.Backends[1].Ref != copyRef || n.Backends[1].Name != "lobby-2" ||
		!slices.Equal(n.Try, []string{"lobby", "lobby-2", "game"}) || !slices.Equal(n.ForcedHosts[0].Servers, []string{"lobby", "lobby-2"}) || n.ApplyError != "" {
		t.Fatalf("network = %+v", n)
	}
	if got := a.runtime.network(copied.ID); got.Forwarding != runtime.ForwardingModern || got.ForwardingSecret != a.runtime.network(lobby.ServerID).ForwardingSecret {
		t.Fatalf("network of the copy = %+v", got)
	}
	if got := a.runtime.network(proxy.ServerID); len(got.Backends) != 3 || got.Backends[1] != (runtime.NetworkBackend{Name: "lobby-2", ServerID: copied.ID}) {
		t.Fatalf("network of the proxy = %+v", got)
	}

	// A copy of the copy is the next lobby.
	api.do("POST", duplicate(copyRef), map[string]any{"name": "Lobby 3", "port": 25569, "network": true}, http.StatusCreated, nil)
	api.do("GET", "/api/networks/"+n.ID, nil, http.StatusOK, &n)
	if len(n.Backends) != 4 || n.Backends[2].Name != "lobby-3" || !slices.Equal(n.Try, []string{"lobby", "lobby-2", "lobby-3", "game"}) {
		t.Fatalf("network = %+v", n)
	}

	// Only game servers of networks, and only with the permission to manage networks, before
	// anything is copied.
	api.do("POST", duplicate(proxy), map[string]any{"name": "Proxy 2", "port": 25578, "network": true}, http.StatusConflict, nil)
	api.do("POST", duplicate(solo), map[string]any{"name": "Solo 2", "port": 25578, "network": true}, http.StatusConflict, nil)
	creator := m.withGrants(t, map[string]any{"name": "Creators", "permissions": []string{"servers.create", "files.read"}, "allServers": true})
	creator.do("POST", duplicate(game), map[string]any{"name": "Game 2", "port": 25578, "network": true}, http.StatusForbidden, nil)
	var servers []struct{ ID string }
	api.do("GET", "/api/nodes/"+lobby.NodeID+"/servers", nil, http.StatusOK, &servers)
	if len(servers) != 6 {
		t.Fatalf("servers = %+v", servers)
	}
}

// A copy that joined a network whose proxy can't be configured stays, and the network tells
// to apply it again.
func TestDuplicateIntoNetworkOffline(t *testing.T) {
	m := startMaster(t)
	a1, a2 := m.startAgent(t, "node-1"), m.startAgent(t, "node-2")
	proxy := m.createServer(t, a2, "Proxy", noryxv1.ServerType_SERVER_TYPE_VELOCITY, 25577)
	lobby := m.createServer(t, a1, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	api := apiClient{t: t, url: m.panel(t).URL}
	var n network.Network
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "servers": []network.Ref{lobby}, "firewalled": true}, http.StatusCreated, &n)
	a2.runtime.mu.Lock()
	a2.runtime.down = true
	a2.runtime.mu.Unlock()
	body := api.do("POST", "/api/nodes/"+lobby.NodeID+"/servers/"+lobby.ServerID+"/duplicate",
		map[string]any{"name": "Lobby 2", "port": 25568, "network": true}, http.StatusBadGateway, nil)
	if !strings.Contains(body, "Lobby 2 was created.") || !strings.Contains(body, "Apply the network again") {
		t.Fatalf("answer = %s", body)
	}
	api.do("GET", "/api/networks/"+n.ID, nil, http.StatusOK, &n)
	if len(n.Backends) != 2 || n.Backends[1].Name != "lobby-2" || n.ApplyError == "" {
		t.Fatalf("network = %+v", n)
	}
}
