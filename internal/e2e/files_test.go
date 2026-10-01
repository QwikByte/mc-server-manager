package e2e

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
)

func TestFiles(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	srv := m.createServer(t, a, "Lobby", mcsmv1.ServerType_SERVER_TYPE_PAPER, 25565)
	panel := m.panel(t)
	api := apiClient{t, panel.URL}
	base := "/api/nodes/" + srv.NodeID + "/servers/" + srv.ServerID + "/files"
	q := func(p string) string { return "?path=" + url.QueryEscape(p) }

	// Upload a file into a new folder; existing files are only replaced on request.
	api.do("POST", base+"/directories", map[string]string{"path": "plugins/Example"}, http.StatusNoContent, nil)
	api.do("PUT", base+"/content"+q("plugins/Example/config.yml"), []byte("motd: hi\n"), http.StatusCreated, nil)
	api.do("PUT", base+"/content"+q("plugins/Example/config.yml"), []byte("lost\n"), http.StatusConflict, nil)
	api.do("PUT", base+"/content"+q("plugins/Example/config.yml")+"&overwrite=true", []byte("motd: hello\n"), http.StatusCreated, nil)
	api.do("PUT", base+"/content"+q("index.html"), []byte("<script>alert(1)</script>"), http.StatusCreated, nil)

	var list struct {
		Files []struct {
			Name      string `json:"name"`
			Directory bool   `json:"directory"`
			Size      int64  `json:"size"`
		} `json:"files"`
	}
	api.do("GET", base+q(""), nil, http.StatusOK, &list)
	if len(list.Files) != 2 || list.Files[0].Name != "plugins" || !list.Files[0].Directory || list.Files[1].Name != "index.html" {
		t.Fatalf("listing = %+v, want the plugins folder before index.html", list.Files)
	}

	// Downloads are attachments, so a file from a server can't run scripts in the panel.
	res, err := http.Get(panel.URL + base + "/content" + q("index.html"))
	check(t, err)
	res.Body.Close()
	if h := res.Header; h.Get("Content-Type") != "application/octet-stream" || h.Get("Content-Security-Policy") != "sandbox" ||
		h.Get("Content-Disposition") != "attachment; filename=index.html" {
		t.Fatalf("download headers = %v", h)
	}
	if got := api.do("GET", base+"/content"+q("plugins/Example/config.yml"), nil, http.StatusOK, nil); got != "motd: hello\n" {
		t.Fatalf("content = %q", got)
	}

	// Rename, then download the folder as a ZIP archive.
	api.do("POST", base+"/move", map[string]string{"from": "plugins/Example/config.yml", "to": "plugins/Example/settings.yml"}, http.StatusNoContent, nil)
	archive := api.do("GET", base+"/archive"+q("plugins"), nil, http.StatusOK, nil)
	zr, err := zip.NewReader(bytes.NewReader([]byte(archive)), int64(len(archive)))
	check(t, err)
	f, err := zr.Open("Example/settings.yml")
	check(t, err)
	content, err := io.ReadAll(f)
	check(t, err)
	if string(content) != "motd: hello\n" {
		t.Fatalf("archived file = %q", content)
	}

	// Paths can't leave the server's directory, not even through a symbolic link.
	check(t, os.Symlink(t.TempDir(), filepath.Join(a.runtime.dir, srv.ServerID, "outside")))
	for _, p := range []string{"../" + srv.ServerID, "/../../etc/passwd", `..\x`, "outside/x"} {
		res, err := http.Get(panel.URL + base + "/content" + q(p))
		check(t, err)
		res.Body.Close()
		if res.StatusCode < 400 {
			t.Errorf("read %q: status %d", p, res.StatusCode)
		}
	}
	api.do("PUT", base+"/content"+q("outside/x"), []byte("x"), http.StatusBadRequest, nil)

	api.do("DELETE", base+q("plugins"), nil, http.StatusNoContent, nil)
	api.do("DELETE", base+q("plugins"), nil, http.StatusNotFound, nil)
	api.do("DELETE", base+q(""), nil, http.StatusBadRequest, nil)
}
