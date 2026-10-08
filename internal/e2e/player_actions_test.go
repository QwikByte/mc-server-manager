package e2e

import (
	"bytes"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/network"
	"github.com/QwikByte/noryx/internal/master/player"
)

// Several players at once, temporary bans, the players who joined, and messages to players.
func TestPlayerActions(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	proxy := m.createServer(t, a, "Proxy", noryxv1.ServerType_SERVER_TYPE_VELOCITY, 25577)
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	game := m.createServer(t, a, "Game", noryxv1.ServerType_SERVER_TYPE_PAPER, 25566)
	api := apiClient{t: t, url: m.panel(t).URL}
	var n network.Network
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "servers": []network.Ref{lobby, game}}, http.StatusCreated, &n)
	check(t, a.runtime.Start(t.Context(), lobby.ServerID))
	both := []network.Ref{lobby, game}

	// Players are banned for a time: on the running server at once, on the stopped one once it
	// runs, and the pardons wait for the end.
	until := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	var changed struct{ Results []player.Result }
	ban := map[string]any{"action": "ban", "names": []string{"Alex", "Steve", "alex"}, "reason": "Spam", "until": until, "servers": both}
	api.do("POST", "/api/players/actions", ban, http.StatusOK, &changed)
	if r := changed.Results; len(r) != 4 || r[0].Name != "Alex" || r[1].Name != "Steve" || r[0].Pending || !r[2].Pending || r[3].Error != "" {
		t.Fatalf("results = %+v", r)
	}
	if got := a.runtime.commandsTo(lobby.ServerID); !slices.Equal(got, []string{"minecraft:ban Alex Spam", "minecraft:ban Steve Spam"}) {
		t.Fatalf("lobby commands = %q", got)
	}
	check(t, os.WriteFile(filepath.Join(a.runtime.dir, lobby.ServerID, "banned-players.json"), []byte(`[{"name":"Alex","created":"2026-10-08 10:00:00 +0000","expires":"forever","reason":"Spam"}]`), 0o600))
	var lists player.Lists
	api.do("GET", "/api/players/lists?node="+lobby.NodeID+"&server="+lobby.ServerID, nil, http.StatusOK, &lists)
	if b := lists.Banned; len(b) != 1 || b[0].Until == nil || !b[0].Until.Equal(until) {
		t.Fatalf("banned = %+v", b)
	}
	if p := lists.Servers[0].Pending; len(p) != 2 || p[0].Action != "pardon" || p[0].Due == nil || !p[0].Due.Equal(until) {
		t.Fatalf("waiting = %+v", p)
	}
	for _, invalid := range []map[string]any{
		{"action": "pardon", "names": []string{"Alex"}, "until": until, "servers": both},
		{"action": "ban", "names": []string{"Alex"}, "until": time.Now().Add(-time.Hour), "servers": both},
		{"action": "ban", "names": []string{"Alex", "@a"}, "servers": both},
		{"action": "ban", "names": []string{"Alex"}, "servers": []network.Ref{}},
		{"action": "whitelist_on", "names": []string{"Alex"}, "servers": both},
		{"action": "ban", "names": manyNames(player.MaxPlayers + 1), "servers": both},
	} {
		api.do("POST", "/api/players/actions", invalid, http.StatusBadRequest, nil)
	}

	// The players who joined come along when asked for, with valid names.
	check(t, os.WriteFile(filepath.Join(a.runtime.dir, game.ServerID, "usercache.json"), []byte(`[{"name":"Kai","uuid":"u1"},{"name":"@a","uuid":"u2"}]`), 0o600))
	api.do("GET", "/api/players/lists", nil, http.StatusOK, &lists)
	if lists.Joined != nil {
		t.Fatalf("joined without asking = %+v", lists.Joined)
	}
	api.do("GET", "/api/players/lists?joined=1", nil, http.StatusOK, &lists)
	if j := lists.Joined; len(j) != 1 || j[0].Name != "Kai" || !slices.Equal(j[0].Servers, []network.Ref{game}) {
		t.Fatalf("joined = %+v", j)
	}

	// Players get titles and whispers on their servers, and a network's players action bars.
	message := map[string]any{"kind": "title", "message": "Welcome", "subtitle": "to Main", "names": []string{"Alex"}, "servers": []network.Ref{lobby}}
	api.do("POST", "/api/players/message", message, http.StatusOK, nil)
	message["kind"], message["subtitle"] = "chat", ""
	api.do("POST", "/api/players/message", message, http.StatusOK, nil)
	api.do("POST", "/api/networks/"+n.ID+"/broadcast", map[string]string{"kind": "actionbar", "message": "Restart soon"}, http.StatusNoContent, nil)
	want := []string{
		`minecraft:title Alex subtitle {"text":"to Main"}`,
		`minecraft:title Alex title {"text":"Welcome"}`,
		`minecraft:tellraw Alex {"translate":"commands.message.display.incoming","with":["Server","Welcome"],"color":"gray","italic":true}`,
		`minecraft:title @a actionbar {"text":"Restart soon"}`,
	}
	if got := a.runtime.commandsTo(lobby.ServerID)[2:]; !slices.Equal(got, want) {
		t.Fatalf("lobby commands = %q, want %q", got, want)
	}
	message["names"] = []string{"@a"}
	api.do("POST", "/api/players/message", message, http.StatusBadRequest, nil)
	message["names"], message["kind"] = []string{"Alex"}, "bossbar"
	api.do("POST", "/api/players/message", message, http.StatusBadRequest, nil)

	// Messages need the permission to send console commands.
	moderator := m.withGrants(t, map[string]any{"name": "Moderators", "permissions": []string{"servers.view", "players.manage"}, "allServers": true})
	message["kind"] = "chat"
	moderator.do("POST", "/api/players/message", message, http.StatusForbidden, nil)
	moderator.do("POST", "/api/players/actions", map[string]any{"action": "kick", "names": []string{"Alex"}, "servers": []network.Ref{lobby}}, http.StatusOK, nil)
}

// The master serves the faces of players from their skins, of Bedrock players through
// GeyserMC, and keeps them.
func TestPlayerFaces(t *testing.T) {
	m := startMaster(t)
	api := apiClient{t: t, url: m.panel(t).URL}
	face := func(name string) {
		t.Helper()
		img, err := png.Decode(bytes.NewReader([]byte(api.do("GET", "/api/players/faces/"+name, nil, http.StatusOK, nil))))
		if err != nil || img.Bounds().Dx() != 8 || img.Bounds().Dy() != 8 {
			t.Fatalf("face of %s: %v, %v", name, img, err)
		}
		if r, _, b, _ := img.At(0, 0).RGBA(); b == 0 || r != 0 { // the blue pixel of the hat
			t.Fatalf("face of %s: %v", name, img.At(0, 0))
		}
	}
	face("Alex")
	face("ALEX")
	face(".Tim_203")
	if n := m.mojang.lookups.Load(); n != 1 {
		t.Fatalf("%d lookups at Mojang, want 1", n)
	}
	api.do("GET", "/api/players/faces/Plain", nil, http.StatusNotFound, nil)
	api.do("GET", "/api/players/faces/Nobody", nil, http.StatusNotFound, nil)
	api.do("GET", "/api/players/faces/"+strings.Repeat("a", 17), nil, http.StatusBadRequest, nil)
}

func manyNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = "Player" + strings.Repeat("x", i%10) + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	return names
}
