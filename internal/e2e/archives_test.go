package e2e

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
