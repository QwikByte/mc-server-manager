package e2e

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/plugin"
)

func TestPlugins(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	lobby := m.createServer(t, a, "Lobby", mcsmv1.ServerType_SERVER_TYPE_PAPER, 25565)
	survival := m.createServer(t, a, "Survival", mcsmv1.ServerType_SERVER_TYPE_PAPER, 25566)
	vanilla := m.createServer(t, a, "Vanilla", mcsmv1.ServerType_SERVER_TYPE_VANILLA, 25567)
	api := apiClient{t, m.panel(t).URL}
	pluginsOf := func(ref plugin.Ref) plugin.Listing {
		t.Helper()
		var l plugin.Listing
		api.do("GET", "/api/nodes/"+ref.NodeID+"/servers/"+ref.ServerID+"/plugins", nil, http.StatusOK, &l)
		return l
	}
	file := func(ref plugin.Ref, name string) string {
		data, _ := os.ReadFile(filepath.Join(a.runtime.dir, ref.ServerID, "plugins", name))
		return string(data)
	}
	lobbyRef, survivalRef, vanillaRef := plugin.Ref(lobby), plugin.Ref(survival), plugin.Ref(vanilla)

	// The search only finds plugins for the server type, and icons come through the master.
	var found struct {
		Hits []struct {
			ID   string `json:"id"`
			Icon string `json:"icon"`
		} `json:"hits"`
	}
	api.do("GET", "/api/plugins/search?type=paper&version=LATEST", nil, http.StatusOK, &found)
	if len(found.Hits) != 3 || found.Hits[0].ID != "luckperms" || found.Hits[0].Icon != "/api/plugins/icons/luckperms/icon.png" {
		t.Fatalf("search = %+v", found)
	}
	res, err := http.Get(api.url + found.Hits[0].Icon)
	check(t, err)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/png" || res.Header.Get("Content-Security-Policy") != "sandbox" {
		t.Fatalf("icon: %d %v", res.StatusCode, res.Header)
	}
	api.do("GET", "/api/plugins/icons/luckperms/icon.svg", nil, http.StatusNotFound, nil)

	// Installing on several servers adds the required projects; vanilla servers have no plugins.
	var installed struct{ Results []plugin.Result }
	api.do("POST", "/api/plugins/install", map[string]any{"projects": []string{"luckperms"}, "servers": []plugin.Ref{lobbyRef, survivalRef, vanillaRef}},
		http.StatusOK, &installed)
	for i, r := range installed.Results[:2] {
		if r.Error != "" || len(r.Installed) != 2 || r.Installed[0].Version != "2.0" || r.Installed[1].ProjectID != "vault" {
			t.Fatalf("result %d = %+v", i, r)
		}
	}
	if r := installed.Results[2]; r.Error != "Vanilla servers can't load plugins or mods." {
		t.Fatalf("vanilla result = %+v", r)
	}
	if got := file(lobbyRef, "luckperms-2.0.jar"); got != "luckperms 2.0" {
		t.Fatalf("installed file = %q", got)
	}
	l := pluginsOf(lobbyRef)
	if l.Folder != "plugins" || len(l.Plugins) != 2 || l.Plugins[0].Project.Title != "LuckPerms" || l.Plugins[0].Version != "2.0" || l.Plugins[0].Update != "" {
		t.Fatalf("listing = %+v", l)
	}

	// An older version is recognised by its hash; updating replaces its file.
	base := "/api/nodes/" + survival.NodeID + "/servers/" + survival.ServerID + "/plugins/"
	api.do("DELETE", base+"luckperms-2.0.jar", nil, http.StatusNoContent, nil)
	api.do("PUT", base+"LuckPerms-old.jar", m.modrinth.content("luckperms", "1.0"), http.StatusCreated, nil)
	api.do("PUT", base+"Custom Plugin.jar", []byte("not from Modrinth"), http.StatusCreated, nil)
	l = pluginsOf(survivalRef)
	if p := l.Plugins[1]; p.FileName != "LuckPerms-old.jar" || p.Version != "1.0" || p.Update != "2.0" || l.Plugins[0].Project != nil {
		t.Fatalf("listing = %+v", l)
	}
	api.do("POST", "/api/plugins/install", map[string]any{"projects": []string{"luckperms"}, "servers": []plugin.Ref{survivalRef}}, http.StatusOK, &installed)
	if file(survivalRef, "LuckPerms-old.jar") != "" || file(survivalRef, "luckperms-2.0.jar") == "" {
		t.Fatal("the old version was not replaced")
	}
	names := []string{}
	for _, p := range pluginsOf(survivalRef).Plugins {
		names = append(names, p.FileName)
	}
	if want := []string{"Custom Plugin.jar", "luckperms-2.0.jar", "vault-1.7.jar"}; !slices.Equal(names, want) {
		t.Fatalf("plugins = %q, want %q", names, want)
	}

	// Mods for other loaders, corrupted downloads and invalid file names are refused.
	api.do("POST", "/api/plugins/install", map[string]any{"projects": []string{"fabricapi", "broken"}, "servers": []plugin.Ref{lobbyRef}}, http.StatusOK, &installed)
	if r := installed.Results[0]; r.Error != "Fabric API has no version for Paper 1.21.4." {
		t.Fatalf("result = %+v", r)
	}
	api.do("POST", "/api/plugins/install", map[string]any{"projects": []string{"broken"}, "servers": []plugin.Ref{lobbyRef}}, http.StatusOK, &installed)
	if r := installed.Results[0]; r.Error != "The download of broken-1.0.jar is corrupted. Try again." || file(lobbyRef, "broken-1.0.jar") != "" {
		t.Fatalf("result = %+v", r)
	}
	api.do("POST", "/api/plugins/install", map[string]any{"projects": []string{"../x"}, "servers": []plugin.Ref{lobbyRef}}, http.StatusBadRequest, nil)
	api.do("POST", "/api/plugins/install", map[string]any{"projects": []string{"vault"}, "servers": []plugin.Ref{lobbyRef, lobbyRef}}, http.StatusBadRequest, nil)
	base = "/api/nodes/" + lobby.NodeID + "/servers/" + lobby.ServerID + "/plugins/"
	api.do("PUT", base+"notes.txt", []byte("x"), http.StatusBadRequest, nil)
	api.do("PUT", base+".hidden.jar", []byte("x"), http.StatusBadRequest, nil)
	api.do("PUT", base+"empty.jar", []byte{}, http.StatusBadRequest, nil)
	api.do("DELETE", base+"missing.jar", nil, http.StatusNotFound, nil)
	api.do("GET", "/api/nodes/"+vanilla.NodeID+"/servers/"+vanilla.ServerID+"/plugins", nil, http.StatusConflict, nil)
}
