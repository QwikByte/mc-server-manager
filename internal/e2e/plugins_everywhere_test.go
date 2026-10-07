package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/access"
	masterapp "github.com/QwikByte/noryx/internal/master/app"
	"github.com/QwikByte/noryx/internal/master/auth"
	"github.com/QwikByte/noryx/internal/master/plugin"
)

// The plugins of all servers are gathered by project, and a project is updated or removed on
// many servers at once, where the user may manage plugins.
func TestPluginsEverywhere(t *testing.T) {
	m := startMaster(t)
	a1, a2 := m.startAgent(t, "node-1"), m.startAgent(t, "node-2")
	lobby := plugin.Ref(m.createServer(t, a1, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565))
	survival := plugin.Ref(m.createServer(t, a1, "Survival", noryxv1.ServerType_SERVER_TYPE_PAPER, 25566))
	modded := plugin.Ref(m.createServer(t, a1, "Modded", noryxv1.ServerType_SERVER_TYPE_FABRIC, 25567))
	m.createServer(t, a2, "Elsewhere", noryxv1.ServerType_SERVER_TYPE_PAPER, 25568)
	api := apiClient{t: t, url: m.panel(t).URL}
	api.do("POST", "/api/plugins/install", map[string]any{"projects": []string{"luckperms"}, "versions": map[string]string{"luckperms": "luckperms10"}, "servers": []plugin.Ref{lobby, survival}},
		http.StatusOK, nil)
	api.do("POST", "/api/plugins/install", map[string]any{"projects": []string{"fabricapi"}, "servers": []plugin.Ref{modded}}, http.StatusOK, nil)
	api.do("POST", "/api/nodes/"+lobby.NodeID+"/servers/"+lobby.ServerID+"/start", nil, http.StatusNoContent, nil)
	api.do("PUT", "/api/nodes/"+survival.NodeID+"/servers/"+survival.ServerID+"/plugins/pins/luckperms", nil, http.StatusNoContent, nil)

	// An offline node is told, and doesn't hold up the others.
	setDown(a2, true)
	var installed plugin.Everywhere
	api.do("GET", "/api/plugins/installed", nil, http.StatusOK, &installed)
	if len(installed.Projects) != 2 || installed.Projects[0].Project.Title != "Fabric API" || installed.Projects[1].Project.Title != "LuckPerms" ||
		len(installed.Unreachable) != 1 || installed.Unreachable[0].NodeName != "node-2" || installed.Unreachable[0].ServerID != "" {
		t.Fatalf("installed = %+v", installed)
	}
	for _, on := range installed.Projects[1].Servers {
		if on.Version != "1.0" || on.Update != "2.0" || on.Pinned != (on.Ref == survival) || on.Project != nil {
			t.Fatalf("LuckPerms on %+v", on)
		}
	}

	// Updating everywhere leaves the pinned server out and restarts the running one, a game
	// server of a network with a rolling restart.
	proxy := m.createServer(t, a1, "Proxy", velocity, 25577)
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "servers": []plugin.Ref{lobby}}, http.StatusCreated, nil)
	var done struct{ Results []plugin.Result }
	api.do("POST", "/api/plugins/update", map[string]any{"project": "luckperms", "servers": []plugin.Ref{lobby, survival}, "restart": true}, http.StatusOK, &done)
	if r := done.Results[0]; r.Error != "" || len(r.Installed) != 2 || r.Installed[0].Version != "2.0" || r.Installed[1].ProjectID != "vault" || !r.Restarted || r.Restart {
		t.Fatalf("lobby = %+v", r)
	}
	if r := done.Results[1]; r.Error != "" || len(r.Installed) != 0 || !slices.Equal(r.Pinned, []string{"LuckPerms"}) || r.Restarted {
		t.Fatalf("survival = %+v", r)
	}
	if got := a1.runtime.restarted(); !slices.Equal(got, []string{lobby.ServerID}) {
		t.Fatalf("restarted = %v", got)
	}

	// A user who may only manage the plugins of the lobby only removes it there, and may not
	// restart.
	var group access.Group
	api.do("POST", "/api/groups", map[string]any{"name": "Lobby", "permissions": []string{"plugins.manage"}, "targets": []plugin.Ref{lobby}}, http.StatusCreated, &group)
	var invited struct{ User auth.User }
	api.do("POST", "/api/users", map[string]any{"username": "lobby", "groups": []string{group.ID}}, http.StatusCreated, &invited)
	grants, err := access.NewService(m.db).Grants(t.Context(), invited.User.ID)
	check(t, err)
	handler := masterapp.API(m.services(t))
	restricted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(access.WithGrants(r.Context(), grants)))
	}))
	t.Cleanup(restricted.Close)
	lobbyUser := apiClient{t: t, url: restricted.URL}
	lobbyUser.do("GET", "/api/plugins/installed", nil, http.StatusOK, &installed)
	if len(installed.Projects) != 2 || len(installed.Projects[0].Servers) != 1 || installed.Projects[0].Servers[0].Ref != lobby {
		t.Fatalf("installed for the lobby's user = %+v", installed)
	}
	lobbyUser.do("POST", "/api/plugins/remove", map[string]any{"project": "luckperms", "servers": []plugin.Ref{lobby, survival}, "restart": true}, http.StatusForbidden, nil)
	lobbyUser.do("POST", "/api/plugins/remove", map[string]any{"project": "luckperms", "servers": []plugin.Ref{survival}}, http.StatusForbidden, nil)
	lobbyUser.do("POST", "/api/plugins/remove", map[string]any{"project": "luckperms", "servers": []plugin.Ref{lobby, survival}}, http.StatusOK, &done)
	if r := done.Results[0]; r.Error != "" || !slices.Equal(r.Removed, []string{"luckperms-2.0.jar"}) || !r.Restart {
		t.Fatalf("lobby = %+v", r)
	}
	if r := done.Results[1]; r.Ref != survival || !strings.Contains(r.Error, "permission") || len(r.Removed) != 0 {
		t.Fatalf("survival = %+v", r)
	}
	if data, _ := os.ReadFile(filepath.Join(a1.runtime.dir, survival.ServerID, "plugins", "luckperms-1.0.jar")); string(data) != "luckperms 1.0" {
		t.Fatal("LuckPerms was removed from the survival server")
	}
}
