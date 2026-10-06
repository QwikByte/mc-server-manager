package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/access"
	masterapp "github.com/QwikByte/noryx/internal/master/app"
	"github.com/QwikByte/noryx/internal/master/fileset"
	"github.com/QwikByte/noryx/internal/master/network"
)

func TestFileSets(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	proxy := m.createServer(t, a, "Proxy", noryxv1.ServerType_SERVER_TYPE_VELOCITY, 25577)
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	game := m.createServer(t, a, "Game", noryxv1.ServerType_SERVER_TYPE_PAPER, 25566)
	api := apiClient{t: t, url: m.panel(t).URL}
	read := func(srv network.Ref, name string) string {
		data, err := os.ReadFile(filepath.Join(a.runtime.dir, srv.ServerID, filepath.FromSlash(name)))
		if os.IsNotExist(err) {
			return "<none>"
		}
		check(t, err)
		return string(data)
	}
	const secret = "s3cr3t-Value"
	var n network.Network
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "servers": []network.Ref{lobby, game}}, http.StatusCreated, &n)

	lp, chat := "plugins/LuckPerms/config.yml", "plugins/Chat/config.yml"
	in := fileset.Input{
		Name: "Plugins",
		Files: []fileset.File{
			{Path: lp, Content: "server: {{network.server}}\npassword: {{secret:db}}\n"},
			{Path: "/" + chat, Content: "name: {{server.name}} on {{server.port}}, {{player}}\n"},
		},
		Targets: []fileset.Target{{Kind: fileset.KindNetwork, Value: n.ID, Role: fileset.RoleServers}},
	}
	var set fileset.Set
	api.do("POST", "/api/filesets", in, http.StatusCreated, &set)
	if set.Version != 1 || len(set.Files) != 2 || set.Files[0].Path != chat || len(set.Versions) != 1 {
		t.Fatalf("set = %+v", set)
	}
	// Lists have no nulls, also for a set without targets.
	api.do("POST", "/api/filesets", fileset.Input{Name: "Empty"}, http.StatusCreated, nil)
	if body := api.do("GET", "/api/filesets", nil, http.StatusOK, nil); strings.Contains(body, "null") {
		t.Fatalf("sets = %s", body)
	}
	for name, f := range map[string]fileset.File{
		"managed file":          {Path: "server.properties", Content: "motd=x"},
		"secret of the server":  {Path: "forwarding.secret", Content: "x"},
		"plugin":                {Path: "plugins/Evil.jar", Content: "x"},
		"unknown variable":      {Path: "a.yml", Content: "{{server.nmae}}"},
		"datastore not yet":     {Path: "a.yml", Content: "{{datastore:main.lp.password}}"},
		"path outside":          {Path: "../a.yml", Content: "x"},
		"invalid secret's name": {Path: "a.yml", Content: "{{secret:A B}}"},
	} {
		bad := in
		bad.Name, bad.Files = "Bad", []fileset.File{f}
		if body := api.do("POST", "/api/filesets", bad, http.StatusBadRequest, nil); body == "" {
			t.Errorf("%s was accepted", name)
		}
	}

	// Without a value for its secret, the set can't be applied.
	path := "/api/filesets/" + set.ID
	var preview fileset.Preview
	api.do("POST", path+"/preview", map[string]int64{"version": 1}, http.StatusOK, &preview)
	if len(preview.Servers) != 2 || !strings.Contains(preview.Servers[0].Error, "db") {
		t.Fatalf("preview without secret = %+v", preview.Servers)
	}
	api.do("PUT", path+"/secrets/db", map[string]string{"value": "two\nlines"}, http.StatusBadRequest, nil)
	api.do("PUT", path+"/secrets/db", map[string]string{"value": secret}, http.StatusOK, nil)

	// The preview tells what each server gets, without the secret.
	preview = fileset.Preview{}
	body := api.do("POST", path+"/preview", map[string]int64{"version": 1}, http.StatusOK, &preview)
	if strings.Contains(body, secret) {
		t.Fatal("the preview shows a secret")
	}
	first := preview.Servers[0]
	if first.Error != "" || !first.FirstSecrets || len(first.Files) != 2 || first.Files[1].Action != "created" || !first.Files[1].Secret {
		t.Fatalf("preview = %+v", first)
	}
	if got := preview.Contents[first.Files[1].After]; got != "server: lobby\npassword: {{secret:db}}\n" && got != "server: game\npassword: {{secret:db}}\n" {
		t.Fatalf("new LuckPerms config shown as %q", got)
	}
	api.do("POST", path+"/apply", map[string]int64{"version": 2}, http.StatusConflict, nil)
	var applied struct{ Results []fileset.Result }
	api.do("POST", path+"/apply", map[string]int64{"version": 1}, http.StatusOK, &applied)
	if len(applied.Results) != 2 || applied.Results[0].Error != "" {
		t.Fatalf("results = %+v", applied.Results)
	}
	if got := read(lobby, lp); got != "server: lobby\npassword: "+secret+"\n" {
		t.Fatalf("LuckPerms config of the lobby = %q", got)
	}
	if got := read(game, chat); got != "name: Game on 25566, {{player}}\n" {
		t.Fatalf("chat config of the game = %q", got)
	}
	status := func() map[string]string {
		var list []fileset.ServerStatus
		api.do("GET", path+"/status", nil, http.StatusOK, &list)
		states := map[string]string{}
		for _, st := range list {
			states[st.Name] = st.State
		}
		return states
	}
	if got := status(); got["Lobby"] != fileset.Current || got["Game"] != fileset.Current {
		t.Fatalf("status = %v", got)
	}

	// The file manager hides the file with the secret and tells which set wrote the others.
	files := "/api/nodes/" + a.node.ID + "/servers/" + lobby.ServerID + "/files"
	api.do("GET", files+"/content?path="+lp, nil, http.StatusForbidden, nil)
	api.do("GET", files+"/content?path="+noryxv1.FileSetManifest, nil, http.StatusForbidden, nil)
	api.do("DELETE", files+"?path="+noryxv1.FileSetManifest, nil, http.StatusForbidden, nil)
	type listing struct {
		Files []struct{ Name, FileSet string }
	}
	var listed listing
	api.do("GET", files+"?path=plugins/Chat", nil, http.StatusOK, &listed)
	if len(listed.Files) != 1 || listed.Files[0].FileSet != "Plugins" {
		t.Fatalf("listed = %+v", listed)
	}
	listed = listing{}
	api.do("GET", files+"?path=plugins/LuckPerms", nil, http.StatusOK, &listed)
	if len(listed.Files) != 0 {
		t.Fatalf("listed = %+v", listed)
	}
	if body := api.do("GET", files+"/archive?path=plugins", nil, http.StatusOK, nil); strings.Contains(body, secret) {
		t.Fatal("a folder download has the secret")
	}

	// Changes on the server and to the set show in the status.
	check(t, os.WriteFile(filepath.Join(a.runtime.dir, game.ServerID, filepath.FromSlash(chat)), []byte("edited\n"), 0o640))
	if got := status(); got["Game"] != fileset.Changed {
		t.Fatalf("status after an edit = %v", got)
	}
	in.Version, in.Files[1].Content = 1, "name: {{server.name}}\n"
	api.do("PUT", path, in, http.StatusOK, &set)
	api.do("PUT", path, in, http.StatusConflict, nil) // based on a version that isn't the newest
	if got := status(); got["Lobby"] != fileset.Outdated || set.Version != 2 {
		t.Fatalf("status after a change = %v", got)
	}

	// A copy of a server loses the files with secrets.
	var copied struct{ ID string }
	api.do("POST", "/api/nodes/"+a.node.ID+"/servers/"+lobby.ServerID+"/duplicate", map[string]any{"name": "Lobby 2", "port": 25567}, http.StatusCreated, &copied)
	if got := read(network.Ref{ServerID: copied.ID}, lp); got != "<none>" {
		t.Fatalf("the copy has the LuckPerms config: %q", got)
	}

	// A server that leaves the network loses the file with the secret right away; the next
	// apply removes the others that didn't change.
	change := network.Change{Name: n.Name, Forwarding: n.Forwarding, Backends: n.Backends[:1], Try: []string{n.Backends[0].Name}}
	api.do("PUT", "/api/networks/"+n.ID, change, http.StatusOK, nil)
	if read(game, lp) != "<none>" || read(game, chat) != "edited\n" {
		t.Fatal("the server that left kept the secret or lost a changed file")
	}
	if got := status(); got["Game"] != fileset.Left {
		t.Fatalf("status after leaving = %v", got)
	}

	// Previewing and applying need the permission to change the files of every server.
	writer := m.withGrants(t, map[string]any{"name": "Lobby files", "permissions": []string{"filesets.manage", "files.write"}, "targets": []network.Ref{lobby}})
	writer.do("POST", path+"/preview", map[string]int64{"version": 2}, http.StatusForbidden, nil)
	api.do("POST", path+"/apply", map[string]any{"version": 2}, http.StatusOK, &applied)
	if read(game, chat) != "edited\n" || read(lobby, chat) != "name: Lobby\n" {
		t.Fatal("applying again went wrong")
	}
	if got := status(); got["Game"] != "" || got["Lobby"] != fileset.Current {
		t.Fatalf("status after applying again = %v", got)
	}
	writer.do("POST", path+"/preview", map[string]int64{"version": 2}, http.StatusOK, nil)

	// Losing the tag that made it a target removes the file with the secret, in the background.
	// The game is no longer in a network, so the file can't name it there.
	in.Version, in.Files[0].Content = 2, "server: {{server.name}}\npassword: {{secret:db}}\n"
	in.Targets = append(in.Targets, fileset.Target{Kind: fileset.KindTag, Value: "extra"})
	api.do("PUT", path, in, http.StatusOK, &set)
	api.do("POST", "/api/servers/tags", map[string]any{"servers": []network.Ref{game}, "add": []string{"extra"}}, http.StatusNoContent, nil)
	api.do("POST", path+"/apply", map[string]any{"version": set.Version}, http.StatusOK, nil)
	if !strings.HasPrefix(read(game, lp), "server: Game") {
		t.Fatalf("LuckPerms config of the tagged game = %q", read(game, lp))
	}
	api.do("POST", "/api/servers/tags", map[string]any{"servers": []network.Ref{game}, "remove": []string{"extra"}}, http.StatusNoContent, nil)
	eventually(t, "the server that lost its tag lost the secret", func() bool { return read(game, lp) == "<none>" })

	// Deleting the set removes its files with secrets; the others stay.
	api.do("DELETE", path, nil, http.StatusNoContent, nil)
	eventually(t, "deleting the set removed its secrets", func() bool {
		return read(lobby, lp) == "<none>" && !strings.Contains(read(lobby, noryxv1.FileSetManifest), set.ID)
	})
	if read(lobby, chat) != "name: Lobby\n" {
		t.Fatal("deleting the set removed other files")
	}
}

// eventually waits up to 5 seconds for done, which something does in the background.
func eventually(t *testing.T, what string, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !done(); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("not so after 5 seconds: %s", what)
		}
	}
}

// withGrants returns a client of the panel of a user in a new group, as the group's input says.
func (m *master) withGrants(t *testing.T, group map[string]any) apiClient {
	svc := m.services(t)
	admin := apiClient{t: t, url: m.panel(t).URL}
	var g access.Group
	admin.do("POST", "/api/groups", group, http.StatusCreated, &g)
	user, err := svc.Users.CreateUser(t.Context(), "user-"+g.ID, "a-long-enough-password")
	check(t, err)
	admin.do("PUT", "/api/users/"+strconv.FormatInt(user.ID, 10), map[string]any{"groups": []string{g.ID}}, http.StatusNoContent, nil)
	grants, err := svc.Access.Grants(t.Context(), user.ID)
	check(t, err)
	api := masterapp.API(svc)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.ServeHTTP(w, r.WithContext(access.WithGrants(r.Context(), grants)))
	}))
	t.Cleanup(srv.Close)
	return apiClient{t: t, url: srv.URL}
}
