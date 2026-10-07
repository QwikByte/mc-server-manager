package e2e

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/runtime"
	"github.com/QwikByte/noryx/internal/master/modpack"
	"github.com/QwikByte/noryx/internal/master/modrinth"
)

func TestModpacks(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	api := apiClient{t: t, url: m.panel(t).URL}
	fabricAPI := m.modrinth.find(func(v modrinth.Version) bool { return v.ProjectID == "fabricapi" })[0].Files[0]
	file := func(path, url, hash string, size int64) map[string]any {
		return map[string]any{"path": path, "hashes": map[string]string{"sha512": hash}, "downloads": []string{url}, "fileSize": size}
	}
	index := func(files ...map[string]any) map[string]any {
		return map[string]any{
			"formatVersion": 1, "game": "minecraft", "versionId": "1.0", "name": "Pack", "files": files,
			"dependencies": map[string]string{"minecraft": "1.21.4", "fabric-loader": "0.16.10"},
		}
	}
	clientOnly := file("mods/shaders.jar", m.modrinth.URL+"/cdn/data/missing.jar", fabricAPI.Hashes.SHA512, 1)
	clientOnly["env"] = map[string]string{"client": "required", "server": "unsupported"}
	pack := m.modrinth.modpack(t, "pack", index(file("mods/fabric-api.jar", fabricAPI.URL, fabricAPI.Hashes.SHA512, fabricAPI.Size), clientOnly),
		map[string]string{
			"overrides/config/pack.toml":        "for everyone",
			"server-overrides/config/pack.toml": "for servers",
			"overrides/server.properties":       "rcon.password=known",
		})

	// The search finds modpacks for the mod loaders of servers, and lists their versions.
	var found struct {
		Hits []struct {
			ID string `json:"id"`
		} `json:"hits"`
	}
	api.do("GET", "/api/plugins/search?kind=modpacks", nil, http.StatusOK, &found)
	if len(found.Hits) != 1 || found.Hits[0].ID != "pack" {
		t.Fatalf("search = %+v", found)
	}
	var versions []modpack.Version
	api.do("GET", "/api/modpacks/pack/versions", nil, http.StatusOK, &versions)
	if len(versions) != 1 || versions[0].ID != pack.ID || versions[0].GameVersions[0] != "1.21.4" || versions[0].Loaders[0] != "fabric" {
		t.Fatalf("versions = %+v", versions)
	}

	// The pack decides the software and versions, and its files are checked and written.
	path := "/api/nodes/" + a.node.ID + "/servers"
	list := func() []runtime.Server {
		servers, err := a.runtime.List(t.Context())
		check(t, err)
		return servers
	}
	create := func(name, project, version string, status int) (created struct{ ID, Type, Version, LoaderVersion string }) {
		t.Helper()
		server := map[string]any{
			"name": name, "memoryMb": 1024, "port": 25565 + len(list()), "acceptEula": true,
			"modpack": map[string]string{"project": project, "version": version},
		}
		api.do("POST", path, server, status, &created)
		return created
	}
	created := create("Pack", "pack", pack.ID, http.StatusCreated)
	if created.Type != "fabric" || created.Version != "1.21.4" || created.LoaderVersion != "0.16.10" {
		t.Fatalf("created = %+v", created)
	}
	if spec := a.runtime.spec(created.ID); spec.Type != noryxv1.ServerType_SERVER_TYPE_FABRIC || spec.LoaderVersion != "0.16.10" {
		t.Fatalf("spec = %+v", spec)
	}
	read := func(name string) string {
		data, _ := os.ReadFile(filepath.Join(a.runtime.dir, created.ID, name))
		return string(data)
	}
	if read("mods/fabric-api.jar") != string(m.modrinth.content("fabricapi", "0.119")) || read("config/pack.toml") != "for servers" {
		t.Fatalf("mod = %q, config = %q", read("mods/fabric-api.jar"), read("config/pack.toml"))
	}
	if read("mods/shaders.jar") != "" || read("server.properties") == "rcon.password=known" {
		t.Fatal("the pack installed a client mod or replaced server.properties")
	}

	// Packs with files elsewhere than on Modrinth's CDN are refused before a server is created.
	elsewhere := m.modrinth.modpack(t, "elsewhere", index(file("mods/x.jar", "https://example.com/x.jar", fabricAPI.Hashes.SHA512, 1)), map[string]string{})
	create("Elsewhere", "elsewhere", elsewhere.ID, http.StatusBadRequest)
	escape := m.modrinth.modpack(t, "escape", index(), map[string]string{"overrides/../escape.txt": "x"})
	create("Escape", "escape", escape.ID, http.StatusBadRequest)
	create("Mismatch", "pack", elsewhere.ID, http.StatusBadRequest) // a version of another project
	// A server that didn't get all files of its pack is deleted again.
	corrupt := m.modrinth.modpack(t, "corrupt", index(file("mods/fabric-api.jar", fabricAPI.URL, sha512Hex([]byte("other")), fabricAPI.Size)), map[string]string{})
	create("Corrupt", "corrupt", corrupt.ID, http.StatusBadGateway)
	if servers := list(); len(servers) != 1 {
		t.Fatalf("servers = %+v", servers)
	}
}

// A server created from a modpack moves to other versions of it, newer and older, after a
// backup: mods follow the pack, also those of its projects installed by hand, files that
// the administrator changed stay as they are and are listed, and the server gets the
// Minecraft and loader version of the pack.
func TestModpackUpdate(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	api := apiClient{t: t, url: m.panel(t).URL}
	project := func(id string) modrinth.File {
		return m.modrinth.find(func(v modrinth.Version) bool { return v.ProjectID == id })[0].Files[0]
	}
	file := func(path string, f modrinth.File) map[string]any {
		return map[string]any{"path": path, "hashes": map[string]string{"sha512": f.Hashes.SHA512}, "downloads": []string{f.URL}, "fileSize": f.Size}
	}
	index := func(minecraft, loader string, files ...map[string]any) map[string]any {
		return map[string]any{"formatVersion": 1, "game": "minecraft", "files": files, "dependencies": map[string]string{"minecraft": minecraft, "fabric-loader": loader}}
	}
	v1 := m.modrinth.modpackVersion(t, "pack", "1.0", index("1.21.4", "0.16.10", file("mods/fabric-api.jar", project("fabricapi"))),
		map[string]string{"overrides/config/a.toml": "a1", "overrides/config/b.toml": "b1", "overrides/config/c.toml": "c1"})
	v2 := m.modrinth.modpackVersion(t, "pack", "2.0", index("1.21.5", "0.16.14", file("mods/proxy.jar", project("8dI2tmqs"))),
		map[string]string{"overrides/config/a.toml": "a2", "overrides/config/b.toml": "b2"})
	v3 := m.modrinth.modpackVersion(t, "pack", "3.0", index("1.21.5", "0.16.14", file("mods/proxy.jar", project("8dI2tmqs"))),
		map[string]string{"overrides/config/a.toml": "a3", "overrides/config/b.toml": "b2"})

	var created struct{ ID string }
	api.do("POST", "/api/nodes/"+a.node.ID+"/servers", map[string]any{
		"name": "Pack", "memoryMb": 1024, "port": 25565, "acceptEula": true, "modpack": map[string]string{"project": "pack", "version": v1.ID},
	}, http.StatusCreated, &created)
	base := "/api/nodes/" + a.node.ID + "/servers/" + created.ID
	var installed modpack.Installed
	api.do("GET", base+"/modpack", nil, http.StatusOK, &installed)
	if installed.Number != "1.0" || installed.Version != v1.ID || installed.Project.Title != "pack" {
		t.Fatalf("modpack = %+v", installed)
	}

	data := filepath.Join(a.runtime.dir, created.ID)
	write := func(name, content string) { check(t, os.WriteFile(filepath.Join(data, name), []byte(content), 0o600)) }
	read := func(name string) string {
		content, _ := os.ReadFile(filepath.Join(data, name))
		return string(content)
	}
	write("config/b.toml", "b by the administrator")
	write("mods/fabric-api-0.120.jar", string(m.modrinth.content("fabricapi", "0.119"))) // installed by hand
	api.do("POST", base+"/start", nil, http.StatusNoContent, nil)

	var change modpack.Change
	api.do("POST", base+"/modpack", map[string]string{"version": v2.ID}, http.StatusOK, &change)
	if !slices.Equal(change.Written, []string{"config/a.toml", "mods/proxy.jar"}) ||
		!slices.Equal(change.Removed, []string{"config/c.toml", "mods/fabric-api-0.120.jar", "mods/fabric-api.jar"}) ||
		!slices.Equal(change.Kept, []string{"config/b.toml"}) || change.Number != "2.0" || change.Backup != "Before modpack 2.0" {
		t.Fatalf("change = %+v", change)
	}
	if read("config/a.toml") != "a2" || read("config/b.toml") != "b by the administrator" || read("config/c.toml") != "" ||
		read("mods/fabric-api.jar") != "" || read("mods/proxy.jar") != string(m.modrinth.content("8dI2tmqs", "2.10")) {
		t.Fatal("the files don't follow the pack")
	}
	spec := a.runtime.spec(created.ID)
	if spec.Version != "1.21.5" || spec.LoaderVersion != "0.16.14" || spec.Type != noryxv1.ServerType_SERVER_TYPE_FABRIC {
		t.Fatalf("spec = %+v", spec)
	}
	if servers, _ := a.runtime.List(t.Context()); servers[0].State != noryxv1.ServerState_SERVER_STATE_RUNNING {
		t.Fatal("the server didn't start again")
	}
	var backups []struct{ Label string }
	api.do("GET", base+"/backups", nil, http.StatusOK, &backups)
	if len(backups) != 1 || backups[0].Label != "Before modpack 2.0" {
		t.Fatalf("backups = %+v", backups)
	}

	// A loader version set by hand stays while the pack keeps its own.
	spec.LoaderVersion = "0.16.15"
	check(t, a.runtime.Update(t.Context(), spec))
	api.do("POST", base+"/modpack", map[string]string{"version": v3.ID}, http.StatusOK, &change)
	if spec := a.runtime.spec(created.ID); read("config/a.toml") != "a3" || spec.LoaderVersion != "0.16.15" || spec.Version != "1.21.5" {
		t.Fatalf("after 3.0: a.toml = %q, spec = %+v", read("config/a.toml"), spec)
	}

	// Going back to an older version works the same way.
	api.do("POST", base+"/modpack", map[string]string{"version": v1.ID}, http.StatusOK, &change)
	if read("config/a.toml") != "a1" || read("config/b.toml") != "b by the administrator" || read("config/c.toml") != "c1" ||
		read("mods/fabric-api.jar") == "" || read("mods/proxy.jar") != "" || a.runtime.spec(created.ID).LoaderVersion != "0.16.10" ||
		a.runtime.spec(created.ID).Version != "1.21.4" {
		t.Fatalf("after going back: change = %+v", change)
	}
	api.do("GET", base+"/modpack", nil, http.StatusOK, &installed)
	if installed.Number != "1.0" {
		t.Fatalf("modpack = %+v", installed)
	}
	api.do("POST", base+"/modpack", map[string]string{"version": v1.ID}, http.StatusConflict, nil)

	// A copy keeps the pack; servers that weren't created from one have none.
	var copied struct{ ID string }
	api.do("POST", base+"/duplicate", map[string]any{"name": "Copy", "port": 25566}, http.StatusCreated, &copied)
	api.do("GET", "/api/nodes/"+a.node.ID+"/servers/"+copied.ID+"/modpack", nil, http.StatusOK, &installed)
	if installed.Number != "1.0" {
		t.Fatalf("modpack of the copy = %+v", installed)
	}
	plain := m.createServer(t, a, "Plain", noryxv1.ServerType_SERVER_TYPE_FABRIC, 25567)
	plainBase := "/api/nodes/" + a.node.ID + "/servers/" + plain.ServerID
	if got := api.do("GET", plainBase+"/modpack", nil, http.StatusOK, nil); strings.TrimSpace(got) != "null" {
		t.Fatalf("modpack of a plain server = %s", got)
	}
	api.do("POST", plainBase+"/modpack", map[string]string{"version": v2.ID}, http.StatusNotFound, nil)

	// A deleted server's pack is forgotten.
	api.do("DELETE", base, nil, http.StatusNoContent, nil)
	var left int
	check(t, m.db.QueryRow(`SELECT COUNT(*) FROM server_modpacks WHERE server_id = ?`, created.ID).Scan(&left))
	if left != 0 {
		t.Fatal("the pack of a deleted server is remembered")
	}
}
