package e2e

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/network"
	"github.com/QwikByte/noryx/internal/master/operation"
	"github.com/QwikByte/noryx/internal/master/tag"
)

func TestUpdateImage(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	api := apiClient{t: t, url: m.panel(t).URL}
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	servers := "/api/nodes/" + a.node.ID + "/servers/"

	var res struct{ Updated bool }
	api.do("POST", servers+lobby.ServerID+"/update-image", nil, http.StatusOK, &res)
	if !res.Updated {
		t.Error("the newer image wasn't reported")
	}
	api.do("POST", servers+"aaaaaaaaaaaaaaaaaaaaaaaaaa/update-image", nil, http.StatusNotFound, nil)
}

func TestBulkActionsAndTags(t *testing.T) {
	m := startMaster(t)
	a, b := m.startAgent(t, "node-1"), m.startAgent(t, "node-2")
	api := apiClient{t: t, url: m.panel(t).URL}
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	game := m.createServer(t, b, "Game", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	both := []network.Ref{lobby, game}
	type listed struct {
		ID, Name, State string
		Tags            []string
	}
	list := func() map[string]listed {
		var servers []listed
		api.do("GET", "/api/servers", nil, http.StatusOK, &servers)
		byName := map[string]listed{}
		for _, s := range servers {
			byName[s.Name] = s
		}
		return byName
	}

	// Tags are normalized, and listed with the servers.
	api.do("POST", "/api/servers/tags", map[string]any{"servers": both, "add": []string{" EU ", "lobby"}}, http.StatusNoContent, nil)
	api.do("POST", "/api/servers/tags", map[string]any{"servers": []network.Ref{game}, "add": []string{"bedwars"}, "remove": []string{"lobby"}}, http.StatusNoContent, nil)
	if s := list(); !slices.Equal(s["Lobby"].Tags, []string{"eu", "lobby"}) || !slices.Equal(s["Game"].Tags, []string{"bedwars", "eu"}) {
		t.Fatalf("servers = %+v", s)
	}
	api.do("POST", "/api/servers/tags", map[string]any{"servers": both, "add": []string{"two words"}}, http.StatusBadRequest, nil)
	api.do("POST", "/api/servers/tags", map[string]any{"servers": []network.Ref{{NodeID: a.node.ID, ServerID: "aaaaaaaaaaaaaaaaaaaaaaaaaa"}}, "add": []string{"x"}}, http.StatusNotFound, nil)
	api.do("POST", "/api/servers/tags", map[string]any{"servers": []network.Ref{lobby, lobby}, "add": []string{"x"}}, http.StatusBadRequest, nil)

	// Actions run on all servers and tell how they ended on each.
	var res struct {
		Results []struct {
			network.Ref
			Error string
		}
	}
	api.do("POST", "/api/servers/actions", map[string]any{"action": "start", "servers": both}, http.StatusOK, &res)
	if len(res.Results) != 2 || res.Results[0].Ref != lobby || res.Results[0].Error != "" || res.Results[1].Error != "" {
		t.Fatalf("results = %+v", res.Results)
	}
	if s := list(); s["Lobby"].State != "running" || s["Game"].State != "running" {
		t.Fatalf("servers = %+v", s)
	}
	api.do("POST", "/api/servers/actions", map[string]any{"action": "command", "command": "save-all", "servers": both}, http.StatusOK, &res)
	if got := a.runtime.consoleCommands(); !slices.Equal(got, []string{"save-all"}) {
		t.Fatalf("console commands = %q", got)
	}
	missing := network.Ref{NodeID: a.node.ID, ServerID: "aaaaaaaaaaaaaaaaaaaaaaaaaa"}
	api.do("POST", "/api/servers/actions", map[string]any{"action": "stop", "servers": []network.Ref{lobby, missing}}, http.StatusOK, &res)
	if res.Results[0].Error != "" || res.Results[1].Error == "" {
		t.Fatalf("results = %+v", res.Results)
	}
	api.do("POST", "/api/servers/actions", map[string]any{"action": "delete", "servers": both}, http.StatusBadRequest, nil)
	api.do("POST", "/api/servers/actions", map[string]any{"action": "command", "servers": both}, http.StatusBadRequest, nil)
	api.do("POST", "/api/servers/actions", map[string]any{"action": "start", "servers": []network.Ref{}}, http.StatusBadRequest, nil)

	// A copy gets the tags of the original; a deleted server loses them.
	var copied struct{ ID string }
	api.do("POST", "/api/nodes/"+game.NodeID+"/servers/"+game.ServerID+"/duplicate", map[string]any{"name": "Game 2", "port": 25566}, http.StatusCreated, &copied)
	api.do("DELETE", "/api/nodes/"+game.NodeID+"/servers/"+game.ServerID, nil, http.StatusNoContent, nil)
	if tags, err := tag.NewStore(m.db).All(t.Context()); err != nil || len(tags) != 2 || !slices.Equal(tags[tag.Server{NodeID: b.node.ID, ServerID: copied.ID}], []string{"bedwars", "eu"}) {
		t.Fatalf("tags = %v, %v", tags, err)
	}
}

// waitForOperation waits until an operation is as ok wants it.
func waitForOperation(t *testing.T, api apiClient, id string, ok func(operation.Operation) bool) operation.Operation {
	t.Helper()
	var op operation.Operation
	for range 500 {
		api.do("GET", "/api/operations/"+id, nil, http.StatusOK, &op)
		if ok(op) {
			return op
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("operation = %+v", op)
	return op
}

// Long operations are answered right away and go on, with the progress the agent reports.
func TestOperations(t *testing.T) {
	m := startMaster(t)
	m.quick = 0
	a := m.startAgent(t, "node-1")
	api := apiClient{t: t, url: m.panel(t).URL}
	hold := make(chan struct{})
	a.runtime.mu.Lock()
	a.runtime.hold = hold
	a.runtime.mu.Unlock()
	create := func(name string, port int) operation.Operation {
		var op operation.Operation
		api.do("POST", "/api/nodes/"+a.node.ID+"/servers",
			map[string]any{"name": name, "type": "paper", "memoryMb": 1024, "port": port, "acceptEula": true}, http.StatusAccepted, &op)
		if op.Kind != "server.create" || op.Subject != name || op.FinishedAt != nil || !slices.Equal(op.Steps, []string{"image", "container"}) {
			t.Fatalf("operation = %+v", op)
		}
		return op
	}
	op := create("Lobby", 25565)
	got := waitForOperation(t, api, op.ID, func(op operation.Operation) bool { return op.Done == 50 })
	if got.Steps[got.Step] != "image" || got.Total != 100 || got.Unit != "bytes" {
		t.Fatalf("operation = %+v", got)
	}
	close(hold)
	got = waitForOperation(t, api, op.ID, func(op operation.Operation) bool { return op.FinishedAt != nil })
	result, _ := got.Result.(map[string]any)
	if got.Error != "" || result["name"] != "Lobby" || got.ServerID != result["id"] || got.NodeID != a.node.ID {
		t.Fatalf("operation = %+v", got)
	}
	var listed []operation.Operation
	api.do("GET", "/api/operations", nil, http.StatusOK, &listed)
	if len(listed) != 1 || listed[0].ID != op.ID {
		t.Fatalf("operations = %+v", listed)
	}

	// A failed operation tells why.
	a.runtime.mu.Lock()
	a.runtime.createErr = errors.New("no space left on device")
	a.runtime.mu.Unlock()
	op = create("Survival", 25566)
	if got := waitForOperation(t, api, op.ID, func(op operation.Operation) bool { return op.FinishedAt != nil }); got.Error == "" || got.Steps[got.Step] != "container" {
		t.Fatalf("operation = %+v", got)
	}
	api.do("GET", "/api/operations/unknown", nil, http.StatusNotFound, nil)
}

// Servers created at the same time, e.g. after a double click, can't take the same port,
// nor more memory together than the node has left.
func TestConcurrentCreates(t *testing.T) {
	m := startMaster(t)
	m.quick = 0
	a := m.startAgent(t, "node-1")
	api := apiClient{t: t, url: m.panel(t).URL}
	path := "/api/nodes/" + a.node.ID
	// The node has 8 GB, of which 4 GB are kept free.
	api.do("PUT", path, map[string]any{"name": "node-1", "address": a.node.Address, "defaultStorage": "default", "memoryReserveMb": 4096}, http.StatusOK, nil)
	hold := make(chan struct{})
	a.runtime.mu.Lock()
	a.runtime.hold = hold
	a.runtime.mu.Unlock()
	create := func(name string, memory, port, status int) operation.Operation {
		t.Helper()
		var op operation.Operation
		api.do("POST", path+"/servers", map[string]any{"name": name, "type": "paper", "memoryMb": memory, "port": port, "acceptEula": true}, status, &op)
		return op
	}
	finished := func(op operation.Operation) bool { return op.FinishedAt != nil }

	// While the lobby is being created, its port and memory are taken.
	lobby := create("Lobby", 2048, 25565, http.StatusAccepted)
	waitForOperation(t, api, lobby.ID, func(op operation.Operation) bool { return op.Done == 50 })
	if got := waitForOperation(t, api, create("Lobby", 1024, 25565, http.StatusAccepted).ID, finished); !strings.Contains(got.Error, "Port 25565") {
		t.Fatalf("second server at the same port: %+v", got)
	}
	create("Survival", 3072, 25566, http.StatusConflict)

	close(hold)
	if got := waitForOperation(t, api, lobby.ID, finished); got.Error != "" {
		t.Fatalf("lobby: %+v", got)
	}
	if got := waitForOperation(t, api, create("Survival", 2048, 25566, http.StatusAccepted).ID, finished); got.Error != "" {
		t.Fatalf("survival: %+v", got)
	}
	var servers []map[string]any
	api.do("GET", path+"/servers", nil, http.StatusOK, &servers)
	if len(servers) != 2 {
		t.Fatalf("servers = %v", servers)
	}
}
