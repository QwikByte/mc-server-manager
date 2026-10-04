package e2e

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

func TestServerProperties(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	proxy := m.createServer(t, a, "Proxy", noryxv1.ServerType_SERVER_TYPE_VELOCITY, 25577)
	api := apiClient{t: t, url: m.panel(t).URL}
	path := "/api/nodes/" + lobby.NodeID + "/servers/" + lobby.ServerID + "/properties"
	change := func(props map[string]string, status int) {
		t.Helper()
		api.do("PUT", path, map[string]any{"properties": props}, status, nil)
	}

	// The file only exists after the first start.
	var got struct {
		Exists     bool              `json:"exists"`
		Properties map[string]string `json:"properties"`
		Locked     []struct{ Key string }
	}
	api.do("GET", path, nil, http.StatusOK, &got)
	if got.Exists {
		t.Fatal("server.properties exists before the first start")
	}
	change(map[string]string{"motd": "x"}, http.StatusConflict)

	file := filepath.Join(a.runtime.dir, lobby.ServerID, "server.properties")
	check(t, os.WriteFile(file, []byte("#Minecraft server properties\nmotd=A Minecraft Server\nrcon.password=s3cret\nmax-players=20\n"), 0o600))
	api.do("GET", path, nil, http.StatusOK, &got)
	if !got.Exists || got.Properties["motd"] != "A Minecraft Server" || got.Properties["rcon.password"] != "" || len(got.Locked) == 0 {
		t.Fatalf("properties = %+v", got)
	}

	change(map[string]string{"motd": "§aWelcome", "white-list": "true"}, http.StatusNoContent)
	data, err := os.ReadFile(file)
	check(t, err)
	want := "#Minecraft server properties\nmotd=\\u00A7aWelcome\nrcon.password=s3cret\nmax-players=20\nwhite-list=true\n"
	if string(data) != want {
		t.Fatalf("server.properties =\n%s\nwant\n%s", data, want)
	}

	// Properties the manager relies on, and line breaks that would add properties, are refused.
	change(map[string]string{"server-port": "25566"}, http.StatusBadRequest)
	change(map[string]string{"motd": "x\rrcon.password=y"}, http.StatusBadRequest)
	change(map[string]string{"bad key": "x"}, http.StatusBadRequest)
	api.do("GET", "/api/nodes/"+proxy.NodeID+"/servers/"+proxy.ServerID+"/properties", nil, http.StatusConflict, nil)

	// A line break in the MOTD is escaped, so it stays one property.
	change(map[string]string{"motd": "Line 1\nrcon.password=y"}, http.StatusNoContent)
	data, err = os.ReadFile(file)
	check(t, err)
	if strings.Count(string(data), "rcon.password=") != 1 {
		t.Fatalf("a line break added a property:\n%s", data)
	}

	api.do("POST", "/api/nodes/"+lobby.NodeID+"/servers/"+lobby.ServerID+"/restart", nil, http.StatusNoContent, nil)
}
