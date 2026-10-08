package e2e

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/network"
)

// zipOf packs files into a ZIP archive, by their names in it.
func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		check(t, err)
		_, err = w.Write([]byte(content))
		check(t, err)
	}
	check(t, zw.Close())
	return buf.Bytes()
}

// A world uploaded as a ZIP archive is extracted into the server's folder, copied and searched,
// and archives that would write secrets or leave the folder write nothing.
func TestExtractCopyAndSearch(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	srv := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	api := apiClient{t: t, url: m.panel(t).URL}
	base := "/api/nodes/" + srv.NodeID + "/servers/" + srv.ServerID + "/files"
	q := func(p string) string { return "?path=" + url.QueryEscape(p) }
	data := serverData{t, filepath.Join(a.runtime.dir, srv.ServerID)}
	data.write("server.properties", "motd=Hello\nrcon.password=hello-secret\n")

	api.do("PUT", base+"/content"+q("world.zip"), zipOf(t, map[string]string{"world/level.dat": "level", "world/region/r.0.0.mca": "chunks"}),
		http.StatusCreated, nil)
	var extracted struct{ Files, Size int64 }
	api.do("POST", base+"/extract", map[string]any{"path": "world.zip"}, http.StatusOK, &extracted)
	if extracted.Files != 2 || data.read("world/region/r.0.0.mca") != "chunks" {
		t.Fatalf("extracted %+v", extracted)
	}
	api.do("POST", base+"/extract", map[string]any{"path": "world.zip"}, http.StatusConflict, nil)
	api.do("POST", base+"/extract", map[string]any{"path": "world.zip", "destination": "imports/old"}, http.StatusOK, nil)

	for name, files := range map[string]map[string]string{
		"secrets.zip": {"plugins/a.yml": "a", "server.properties": "rcon.password=mine\n"},
		"slip.zip":    {"plugins/b.yml": "b", "../../outside": "x"},
	} {
		api.do("PUT", base+"/content"+q(name), zipOf(t, files), http.StatusCreated, nil)
		api.do("POST", base+"/extract", map[string]any{"path": name}, http.StatusBadRequest, nil)
	}
	if _, err := os.Stat(filepath.Join(data.dir, "plugins")); err == nil || data.read("server.properties") != "motd=Hello\nrcon.password=hello-secret\n" {
		t.Fatal("a refused archive wrote files")
	}

	// Copies and searches don't show secrets.
	api.do("POST", base+"/copy", map[string]string{"from": "world", "to": "world_backup"}, http.StatusNoContent, nil)
	api.do("POST", base+"/copy", map[string]string{"from": "server.properties", "to": "props.txt"}, http.StatusForbidden, nil)
	if data.read("world_backup/level.dat") != "level" {
		t.Fatal("the folder wasn't copied")
	}
	var found struct {
		Matches []struct {
			Path string `json:"path"`
			Line int64  `json:"line"`
			Text string `json:"text"`
		} `json:"matches"`
		Truncated bool `json:"truncated"`
	}
	api.do("GET", base+"/search?path=&query=hello", nil, http.StatusOK, &found)
	if len(found.Matches) != 1 || found.Matches[0].Path != "server.properties" || found.Matches[0].Text != "motd=Hello" {
		t.Fatalf("found %+v", found)
	}

	// Reading files allows searching, but not extracting or copying.
	reader := m.withGrants(t, map[string]any{"name": "Readers", "permissions": []string{"files.read"}, "targets": []network.Ref{srv}})
	reader.do("GET", base+"/search?query=level", nil, http.StatusOK, nil)
	reader.do("POST", base+"/extract", map[string]any{"path": "world.zip", "destination": "x"}, http.StatusForbidden, nil)
	reader.do("POST", base+"/copy", map[string]string{"from": "world", "to": "x"}, http.StatusForbidden, nil)
}

// A backup downloaded from Noryx can be uploaded and restored, which keeps the server's own
// secrets; archives that leave the server's data are refused.
func TestUploadBackup(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	srv := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	api := apiClient{t: t, url: m.panel(t).URL}
	base := "/api/nodes/" + srv.NodeID + "/servers/" + srv.ServerID + "/backups"
	data := serverData{t, filepath.Join(a.runtime.dir, srv.ServerID)}
	data.write("world/level.dat", "level")
	data.write("server.properties", "motd=lobby\nrcon.password=lobby-secret\n")
	data.write(".rcon-cli.env", "password=lobby-secret")

	var b backupView
	api.do("POST", base, map[string]any{"selection": map[string]any{"everything": true}}, http.StatusCreated, &b)
	downloaded := api.do("GET", base+"/"+b.ID+"/download", nil, http.StatusOK, nil)
	if strings.Contains(downloaded, "lobby-secret") {
		t.Fatal("the download holds a secret")
	}
	data.write("world/level.dat", "griefed")

	// Uploading needs the permissions to back up and to restore.
	for _, perms := range [][]string{{"backups.view", "backups.create"}, {"backups.view", "backups.restore"}} {
		user := m.withGrants(t, map[string]any{"name": strings.Join(perms, " "), "permissions": perms, "targets": []network.Ref{srv}})
		user.do("POST", base+"/upload", []byte(downloaded), http.StatusForbidden, nil)
	}
	var uploaded struct {
		backupView
		Untrusted bool `json:"untrusted"`
	}
	api.do("POST", base+"/upload?label=Downloaded", []byte(downloaded), http.StatusCreated, &uploaded)
	if !uploaded.Untrusted || uploaded.Label != "Downloaded" || !slices.Contains(uploaded.Paths, "world") {
		t.Fatalf("uploaded %+v", uploaded)
	}
	api.do("POST", base+"/upload", zipOf(t, map[string]string{"world/level.dat": "x", "../../outside": "x"}), http.StatusBadRequest, nil)
	api.do("POST", base+"/upload", []byte("no archive"), http.StatusBadRequest, nil)
	var list []backupView
	api.do("GET", base, nil, http.StatusOK, &list)
	if len(list) != 2 {
		t.Fatalf("backups = %+v", list)
	}

	api.do("POST", base+"/"+uploaded.ID+"/restore", map[string]any{"snapshotFirst": true}, http.StatusOK, nil)
	if data.read("world/level.dat") != "level" || data.read("server.properties") != "motd=lobby\nrcon.password=lobby-secret\n" ||
		data.read(".rcon-cli.env") != "password=lobby-secret" {
		t.Fatalf("restored world %q, server.properties %q", data.read("world/level.dat"), data.read("server.properties"))
	}
}

// A server can be created from a ZIP archive of a server from elsewhere, which keeps neither
// its secrets nor its trust in a proxy; archives with links create no server.
func TestImportServer(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	api := apiClient{t: t, url: m.panel(t).URL}
	path := "/api/nodes/" + a.node.ID + "/servers/import"
	settings := map[string]any{"name": "Imported", "type": "paper", "memoryMb": 1024, "port": 25570, "acceptEula": true}
	archive := zipOf(t, map[string]string{
		"world/level.dat":         "level",
		"server.properties":       "motd=imported\nrcon.password=theirs\nonline-mode=false\n",
		".rcon-cli.env":           "password=theirs",
		"noryx-filesets.json":     `{"marked":[]}`,
		"config/paper-global.yml": "proxies:\n  velocity:\n    enabled: true\n    secret: their-secret\n",
		"plugins/Essentials.jar":  "jar",
	})

	creator := m.withGrants(t, map[string]any{"name": "Viewers", "permissions": []string{"servers.view"}, "allServers": true})
	importServer(t, creator.url+path, settings, archive, http.StatusForbidden)
	importServer(t, api.url+path, map[string]any{"name": "No EULA", "type": "paper", "memoryMb": 1024, "port": 25571}, archive, http.StatusBadRequest)
	importServer(t, api.url+path, settings, zipOf(t, map[string]string{"world/level.dat": "x", "../outside": "x"}), http.StatusBadRequest)
	importServer(t, api.url+"/api/nodes/missing/servers/import", settings, archive, http.StatusNotFound)
	if servers, _ := a.runtime.List(t.Context()); len(servers) != 0 {
		t.Fatalf("servers after refused imports = %+v", servers)
	}

	var created struct {
		ID      string   `json:"id"`
		Name    string   `json:"name"`
		LeftOut []string `json:"leftOut"`
	}
	check(t, json.Unmarshal([]byte(importServer(t, api.url+path, settings, archive, http.StatusCreated)), &created))
	if created.Name != "Imported" || !slices.Equal(slices.Sorted(slices.Values(created.LeftOut)), []string{".rcon-cli.env", "noryx-filesets.json"}) {
		t.Fatalf("created %+v", created)
	}
	data := serverData{t, filepath.Join(a.runtime.dir, created.ID)}
	for name, want := range map[string]string{
		"world/level.dat":         "level",
		"plugins/Essentials.jar":  "jar",
		"server.properties":       "motd=imported\nrcon.password=\nonline-mode=true\n",
		".rcon-cli.env":           "",
		"noryx-filesets.json":     "",
		"config/paper-global.yml": "proxies:\n    velocity: {}\n",
	} {
		if got := data.read(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

// importServer sends the settings and the archive of a server as the panel does, and returns
// the answer.
func importServer(t *testing.T, url string, settings map[string]any, archive []byte, wantStatus int) string {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormField("server")
	check(t, err)
	check(t, json.NewEncoder(part).Encode(settings))
	part, err = mw.CreateFormFile("archive", "server.zip")
	check(t, err)
	_, err = part.Write(archive)
	check(t, err)
	check(t, mw.Close())
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, &body)
	check(t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	res, err := http.DefaultClient.Do(req)
	check(t, err)
	defer res.Body.Close()
	answer, err := io.ReadAll(res.Body)
	check(t, err)
	if res.StatusCode != wantStatus {
		t.Fatalf("import: status %d, want %d: %s", res.StatusCode, wantStatus, answer)
	}
	return string(answer)
}
