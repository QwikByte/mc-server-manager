package e2e

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
)

func TestDuplicateServer(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	lobby := m.createServer(t, a, "Lobby", mcsmv1.ServerType_SERVER_TYPE_PAPER, 25565)
	m.createServer(t, a, "Survival", mcsmv1.ServerType_SERVER_TYPE_PAPER, 25566)
	api := apiClient{t, m.panel(t).URL}
	base := "/api/nodes/" + lobby.NodeID + "/servers/" + lobby.ServerID
	data := filepath.Join(a.runtime.dir, lobby.ServerID)
	check(t, os.MkdirAll(filepath.Join(data, "world", "region"), 0o750))
	check(t, os.WriteFile(filepath.Join(data, "world", "region", "r.0.0.mca"), []byte("chunks"), 0o600))
	check(t, os.Symlink(t.TempDir(), filepath.Join(data, "outside")))

	// A running server saves its worlds before they are copied, and keeps running.
	api.do("POST", base+"/start", nil, http.StatusNoContent, nil)
	var copied struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Port     uint32 `json:"port"`
		State    string `json:"state"`
		MemoryMB uint32 `json:"memoryMb"`
	}
	api.do("POST", base+"/duplicate", map[string]any{"name": "Lobby 2", "port": 25567}, http.StatusCreated, &copied)
	if copied.Name != "Lobby 2" || copied.Port != 25567 || copied.State != "stopped" || copied.MemoryMB != 1024 {
		t.Fatalf("copy = %+v", copied)
	}
	if got, want := a.runtime.consoleCommands(), []string{"save-off", "save-all flush", "save-on"}; !slices.Equal(got, want) {
		t.Fatalf("console commands = %q, want %q", got, want)
	}
	region := filepath.Join(a.runtime.dir, copied.ID, "world", "region", "r.0.0.mca")
	if content, err := os.ReadFile(region); err != nil || string(content) != "chunks" {
		t.Fatalf("copied world = %q, %v", content, err)
	}
	if info, err := os.Stat(region); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("copied file mode = %v, %v", info.Mode(), err)
	}
	if _, err := os.Lstat(filepath.Join(a.runtime.dir, copied.ID, "outside")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the copy contains a symbolic link")
	}
	if spec := a.runtime.spec(copied.ID); spec.Type != mcsmv1.ServerType_SERVER_TYPE_PAPER || spec.ID == lobby.ServerID {
		t.Fatalf("copied spec = %+v", spec)
	}

	// The copy needs a free port and a valid name.
	api.do("POST", base+"/duplicate", map[string]any{"name": "Lobby 3", "port": 25566}, http.StatusConflict, nil)
	api.do("POST", base+"/duplicate", map[string]any{"name": "../x", "port": 25568}, http.StatusBadRequest, nil)
	api.do("POST", "/api/nodes/"+lobby.NodeID+"/servers/"+strings.Repeat("a", 26)+"/duplicate",
		map[string]any{"name": "X", "port": 25568}, http.StatusNotFound, nil)
}
