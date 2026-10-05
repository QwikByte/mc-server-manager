package e2e

import (
	"net/http"
	"os"
	"path/filepath"
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
