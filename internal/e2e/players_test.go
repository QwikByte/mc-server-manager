package e2e

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/network"
	"github.com/QwikByte/noryx/internal/master/player"
)

func TestPlayers(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	proxy := m.createServer(t, a, "Proxy", noryxv1.ServerType_SERVER_TYPE_VELOCITY, 25577)
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	game := m.createServer(t, a, "Game", noryxv1.ServerType_SERVER_TYPE_PAPER, 25566)
	api := apiClient{t: t, url: m.panel(t).URL}
	var n network.Network
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "servers": []network.Ref{lobby, game}}, http.StatusCreated, &n)
	check(t, a.runtime.Start(t.Context(), lobby.ServerID))

	// A ban reaches the running server at once, and the stopped one once it runs.
	var changed struct{ Results []player.Result }
	ban := map[string]any{"action": "ban", "name": "Alex", "reason": "Cheating", "servers": []network.Ref{lobby, game}}
	api.do("POST", "/api/players/actions", ban, http.StatusOK, &changed)
	if r := changed.Results; len(r) != 2 || r[0].Output != "ran minecraft:ban Alex Cheating" || r[0].Pending || !r[1].Pending || r[1].Error != "" {
		t.Fatalf("results = %+v", r)
	}
	if got := a.runtime.commandsTo(lobby.ServerID); !slices.Equal(got, []string{"minecraft:ban Alex Cheating"}) {
		t.Fatalf("lobby commands = %q", got)
	}
	for _, invalid := range []map[string]any{
		{"action": "ban", "name": "@a", "servers": []network.Ref{lobby}},
		{"action": "pardon", "name": "Alex", "reason": "sorry", "servers": []network.Ref{lobby}},
		{"action": "whitelist_on", "name": "Alex", "servers": []network.Ref{lobby}},
		{"action": "smite", "name": "Alex", "servers": []network.Ref{lobby}},
		{"action": "kick", "name": "Alex", "servers": []network.Ref{}},
	} {
		api.do("POST", "/api/players/actions", invalid, http.StatusBadRequest, nil)
	}

	// The lists of a network join those of its servers, with the changes that wait.
	check(t, os.WriteFile(filepath.Join(a.runtime.dir, lobby.ServerID, "banned-players.json"),
		[]byte(`[{"uuid":"u1","name":"Alex","created":"2026-05-01 10:00:00 +0000","source":"Server","expires":"forever","reason":"Cheating"}]`), 0o600))
	var lists player.Lists
	api.do("GET", "/api/players/lists?network="+n.ID, nil, http.StatusOK, &lists)
	if len(lists.Banned) != 1 || lists.Banned[0].Reason != "Cheating" || !slices.Equal(lists.Banned[0].Servers, []network.Ref{lobby}) {
		t.Fatalf("banned = %+v", lists.Banned)
	}
	if s := lists.Servers; len(s) != 2 || len(s[0].Pending) != 0 || len(s[1].Pending) != 1 || s[1].Pending[0].Action != "ban" {
		t.Fatalf("servers = %+v", s)
	}
	api.do("GET", "/api/players/lists", nil, http.StatusOK, &lists)
	if len(lists.Servers) != 2 {
		t.Fatalf("all game servers = %+v", lists.Servers)
	}

	// Bedrock players are whitelisted with their ID from GeyserMC, as the servers behind the
	// proxy can't look them up: their file changes, and a running server reloads it.
	whitelist := map[string]any{"action": "whitelist_add", "name": ".Tim_203", "servers": []network.Ref{lobby, game}}
	var added struct{ Results []player.Result }
	api.do("POST", "/api/players/actions", whitelist, http.StatusOK, &added)
	if r := added.Results; len(r) != 2 || r[0].Output != "Added .Tim_203 to the whitelist" || r[1].Pending || r[1].Error != "" {
		t.Fatalf("results = %+v", r)
	}
	api.do("GET", "/api/players/lists?network="+n.ID, nil, http.StatusOK, &lists)
	if w := lists.Whitelisted; len(w) != 1 || w[0].UUID != "00000000-0000-0000-0009-01f64f65c7c3" || len(w[0].Servers) != 2 {
		t.Fatalf("whitelisted = %+v", w)
	}
	if got := a.runtime.commandsTo(lobby.ServerID); got[len(got)-1] != "minecraft:whitelist reload" {
		t.Fatalf("lobby commands = %q", got)
	}
	whitelist["name"] = ".Nobody"
	api.do("POST", "/api/players/actions", whitelist, http.StatusNotFound, nil)

	// The proxy sends players to servers of its network.
	api.do("POST", "/api/networks/"+n.ID+"/players/send", map[string]string{"name": "Alex", "server": "game"}, http.StatusNoContent, nil)
	if got := a.runtime.commandsTo(proxy.ServerID); !slices.Equal(got, []string{"send Alex game"}) {
		t.Fatalf("proxy commands = %q", got)
	}
	api.do("POST", "/api/networks/"+n.ID+"/players/send", map[string]string{"name": "Alex", "server": "elsewhere"}, http.StatusBadRequest, nil)
	api.do("POST", "/api/networks/"+n.ID+"/players/send", map[string]string{"name": "Alex lobby", "server": "game"}, http.StatusBadRequest, nil)
}

func TestMaintenance(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	proxy := m.createServer(t, a, "Proxy", noryxv1.ServerType_SERVER_TYPE_VELOCITY, 25577)
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	api := apiClient{t: t, url: m.panel(t).URL}
	var n network.Network
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "servers": []network.Ref{lobby}}, http.StatusCreated, &n)
	path := "/api/networks/" + n.ID + "/maintenance"

	// The proxy acts like the Maintenance plugin: it loads once it restarted with the plugin.
	// The agent calls it, so it leaves failures to the checks of the test.
	folder := filepath.Join(a.runtime.dir, proxy.ServerID, "plugins", "maintenance")
	write := func(name, content string) { _ = os.WriteFile(filepath.Join(folder, name), []byte(content), 0o600) }
	a.runtime.on = func(id, what string) {
		switch {
		case id != proxy.ServerID:
		case what == "restart":
			if _, err := os.Stat(filepath.Join(folder, "..", "VCAqN1ln-5.1.0.jar")); err == nil {
				_ = os.MkdirAll(folder, 0o750)
				write("config.yml", "maintenance-enabled: false\n")
			}
		case what == "maintenance on":
			write("config.yml", "maintenance-enabled: true\n")
		case what == "maintenance off":
			write("config.yml", "maintenance-enabled: false\n")
		case what == "maintenance add Steve":
			write("WhitelistedPlayers.yml", "8667ba71-b85a-4004-af54-457a9734eed7: Steve\n")
		}
	}

	var got network.Maintenance
	api.do("GET", path, nil, http.StatusOK, &got)
	if got.Installed || got.Enabled || got.ProxyRunning {
		t.Fatalf("maintenance of a new network = %+v", got)
	}
	api.do("POST", path, map[string]bool{"enabled": true}, http.StatusConflict, nil)

	// The first time, the plugin is installed and the proxy restarts to load it.
	check(t, a.runtime.Start(t.Context(), proxy.ServerID))
	api.do("POST", path, map[string]bool{"enabled": true}, http.StatusOK, &got)
	if !got.Installed || !got.Enabled || !slices.Equal(a.runtime.restarted(), []string{proxy.ServerID}) {
		t.Fatalf("maintenance = %+v, restarts %q", got, a.runtime.restarted())
	}
	api.do("POST", path+"/players", map[string]any{"name": "Steve", "add": true}, http.StatusOK, &got)
	if len(got.Players) != 1 || got.Players[0].Name != "Steve" {
		t.Fatalf("players = %+v", got.Players)
	}
	api.do("POST", path+"/players", map[string]any{"name": "@a", "add": true}, http.StatusBadRequest, nil)
	api.do("POST", path, map[string]bool{"enabled": false}, http.StatusOK, &got)
	if got.Enabled || len(a.runtime.restarted()) != 1 {
		t.Fatalf("maintenance after turning it off = %+v", got)
	}
	want := []string{"maintenance on", "maintenance add Steve", "maintenance off"}
	if cmds := a.runtime.commandsTo(proxy.ServerID); !slices.Equal(cmds, want) {
		t.Fatalf("proxy commands = %q, want %q", cmds, want)
	}
}

func TestRollingRestart(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	proxy := m.createServer(t, a, "Proxy", noryxv1.ServerType_SERVER_TYPE_VELOCITY, 25577)
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	games := []network.Ref{
		m.createServer(t, a, "Game 1", noryxv1.ServerType_SERVER_TYPE_PAPER, 25566),
		m.createServer(t, a, "Game 2", noryxv1.ServerType_SERVER_TYPE_PAPER, 25567),
		m.createServer(t, a, "Game 3", noryxv1.ServerType_SERVER_TYPE_PAPER, 25568),
	}
	api := apiClient{t: t, url: m.panel(t).URL}
	var n network.Network
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "servers": append([]network.Ref{lobby}, games...)}, http.StatusCreated, &n)
	path := "/api/networks/" + n.ID + "/rolling-restart"
	api.do("POST", path, map[string]int{"batch": 1}, http.StatusConflict, nil) // nothing runs
	for _, s := range []network.Ref{proxy, lobby, games[0], games[1]} {
		check(t, a.runtime.Start(t.Context(), s.ServerID))
	}

	// The running game servers restart two at a time, the lobby players join last; the
	// stopped one stays stopped, and the proxy keeps running.
	api.do("POST", path, map[string]int{"batch": 0}, http.StatusBadRequest, nil)
	api.do("POST", path, map[string]int{"batch": 2}, http.StatusNoContent, nil)
	got := a.runtime.restarted()
	if len(got) != 3 || got[2] != lobby.ServerID || !slices.Contains(got, games[0].ServerID) || !slices.Contains(got, games[1].ServerID) {
		t.Fatalf("restarts = %q", got)
	}
}
