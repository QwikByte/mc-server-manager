package e2e

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
	"github.com/QwikByte/mc-server-manager/internal/master/access"
	"github.com/QwikByte/mc-server-manager/internal/master/network"
	masterserver "github.com/QwikByte/mc-server-manager/internal/master/server"
)

func TestMoveServer(t *testing.T) {
	m := startMaster(t)
	a1, a2 := m.startAgent(t, "node-1"), m.startAgent(t, "node-2")
	lobby := m.createServer(t, a1, "Lobby", mcsmv1.ServerType_SERVER_TYPE_PAPER, 25565)
	m.createServer(t, a2, "Survival", mcsmv1.ServerType_SERVER_TYPE_PAPER, 25566)
	api := apiClient{t: t, url: m.panel(t).URL}
	path := func(r network.Ref) string { return "/api/nodes/" + r.NodeID + "/servers/" + r.ServerID }
	data := filepath.Join(a1.runtime.dir, lobby.ServerID)
	check(t, os.MkdirAll(filepath.Join(data, "world", "region"), 0o750))
	check(t, os.WriteFile(filepath.Join(data, "world", "region", "r.0.0.mca"), []byte("chunks"), 0o600))
	api.do("POST", path(lobby)+"/backups", map[string]any{"label": "Before the move", "selection": map[string]any{"paths": []string{"world"}}}, http.StatusCreated, nil)
	var mods access.Group
	api.do("POST", "/api/groups", map[string]any{"name": "Lobby moderators", "permissions": []string{"servers.restart"}, "targets": []network.Ref{lobby}},
		http.StatusCreated, &mods)
	api.do("POST", path(lobby)+"/start", nil, http.StatusNoContent, nil)

	// What would fail is refused before the server stops.
	api.do("POST", path(lobby)+"/move", map[string]any{"node": a1.node.ID}, http.StatusBadRequest, nil)
	api.do("POST", path(lobby)+"/move", map[string]any{"node": a2.node.ID, "port": 25566}, http.StatusConflict, nil)
	api.do("POST", path(lobby)+"/move", map[string]any{"node": a2.node.ID, "storage": "ssd"}, http.StatusBadRequest, nil)
	if state(t, a1, lobby.ServerID) != mcsmv1.ServerState_SERVER_STATE_RUNNING {
		t.Fatal("the server stopped")
	}

	// The server moves with its data and backups, keeps its ID and runs again.
	api.do("POST", path(lobby)+"/move", map[string]any{"node": a2.node.ID, "backups": true}, http.StatusAccepted, nil)
	if mv := waitForMove(t, api, lobby.ServerID); mv.Phase != "done" || len(mv.Warnings) != 0 || mv.Backups != 1 || mv.Bytes == 0 {
		t.Fatalf("move = %+v", mv)
	}
	moved := network.Ref{NodeID: a2.node.ID, ServerID: lobby.ServerID}
	if state(t, a2, lobby.ServerID) != mcsmv1.ServerState_SERVER_STATE_RUNNING || slices.ContainsFunc(must(a1.runtime.List(t.Context())), isServer(lobby.ServerID)) {
		t.Fatal("the server is not on the new node only, running")
	}
	if chunks, err := os.ReadFile(filepath.Join(a2.runtime.dir, lobby.ServerID, "world", "region", "r.0.0.mca")); string(chunks) != "chunks" {
		t.Fatalf("world = %q, %v", chunks, err)
	}
	var backups []struct{ Label string }
	api.do("GET", path(moved)+"/backups", nil, http.StatusOK, &backups)
	if len(backups) != 1 || backups[0].Label != "Before the move" {
		t.Fatalf("backups = %+v", backups)
	}
	api.do("GET", "/api/groups/"+mods.ID, nil, http.StatusOK, &mods)
	if len(mods.Targets) != 1 || mods.Targets[0].NodeID != a2.node.ID {
		t.Fatalf("group targets = %+v", mods.Targets)
	}

	// If the new node fails, the server stays where it was and runs again.
	a1.runtime.mu.Lock()
	a1.runtime.createErr = errors.New("no space left on device")
	a1.runtime.mu.Unlock()
	api.do("POST", path(moved)+"/move", map[string]any{"node": a1.node.ID}, http.StatusAccepted, nil)
	if mv := waitForMove(t, api, lobby.ServerID); mv.Phase != "failed" || mv.Error == "" {
		t.Fatalf("move = %+v", mv)
	}
	if state(t, a2, lobby.ServerID) != mcsmv1.ServerState_SERVER_STATE_RUNNING || slices.ContainsFunc(must(a1.runtime.List(t.Context())), isServer(lobby.ServerID)) {
		t.Fatal("the failed move left the server elsewhere")
	}
}

func waitForMove(t *testing.T, api apiClient, serverID string) masterserver.Move {
	t.Helper()
	for range 100 {
		var moves []masterserver.Move
		api.do("GET", "/api/moves", nil, http.StatusOK, &moves)
		if i := slices.IndexFunc(moves, func(mv masterserver.Move) bool { return mv.ServerID == serverID }); i >= 0 && moves[i].FinishedAt != nil {
			return moves[i]
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the move didn't finish")
	return masterserver.Move{}
}

func state(t *testing.T, a agent, id string) mcsmv1.ServerState {
	servers := must(a.runtime.List(t.Context()))
	if i := slices.IndexFunc(servers, isServer(id)); i >= 0 {
		return servers[i].State
	}
	return mcsmv1.ServerState_SERVER_STATE_UNSPECIFIED
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func isServer(id string) func(runtime.Server) bool {
	return func(s runtime.Server) bool { return s.ID == id }
}
