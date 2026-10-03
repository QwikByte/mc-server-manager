package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
)

func TestTerminal(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	lobby := m.createServer(t, a, "Lobby", mcsmv1.ServerType_SERVER_TYPE_PAPER, 25565)
	url := m.panel(t).URL
	run := func(target, command string) (string, string) {
		t.Helper()
		return runTerminal(t, http.DefaultClient, url, target, command)
	}

	// The commands of the agent's CLI run over the master's connection to the agent.
	for command, want := range map[string]string{
		"server list": "Lobby",
		"status":      "Cert     valid until",
		"mcsm-agent server start " + lobby.ServerID:                "",
		`server command ` + lobby.ServerID + ` say "hello  world"`: "ran say hello  world",
		"server logs " + lobby.ServerID:                            "[INFO]: Starting\n[INFO]: Done\n",
		"help":                                                     "backup",
	} {
		if out, err := run(a.node.ID, command); err != "" || !strings.Contains(out, want) {
			t.Errorf("%s: output %q, error %q; want %q", command, out, err, want)
		}
	}
	for command, want := range map[string]string{
		"server start ../../etc":   "invalid server ID", // without the gRPC prefix
		"storage add ssd /mnt/ssd": `unknown command "storage"`,
		"enroll mcsm1_token":       `unknown command "enroll"`,
		"server list; rm -rf /":    "unknown shorthand flag",
		"server start":             "accepts 1 arg(s)",
	} {
		if _, err := run(a.node.ID, command); !strings.Contains(err, want) {
			t.Errorf("%s: error %q, want %q", command, err, want)
		}
	}

	// While Docker is down, status leaves out what it can't know and says what to do.
	a.runtime.mu.Lock()
	a.runtime.down = true
	a.runtime.mu.Unlock()
	out, err := run(a.node.ID, "status")
	if want := "can't reach Docker on this node, see: systemctl status docker"; err != want || !strings.Contains(out, "Runtime  unavailable") || strings.Contains(out, "System") {
		t.Errorf("status while Docker is down: output %q, error %q; want %q", out, err, want)
	}
	if _, err := run(a.node.ID, "server list"); err != "Docker isn't running on node-1, or its agent can't connect to it." {
		t.Errorf("server list while Docker is down: error %q", err)
	}
	a.runtime.mu.Lock()
	a.runtime.down = false
	a.runtime.mu.Unlock()

	// The master's commands describe the master and its nodes.
	for command, want := range map[string]string{
		"status":            "Nodes    1 of 1 online",
		"node list":         "node-1",
		"node renew NODE-1": "Renewed the certificate of node-1",
	} {
		if out, err := run("master", command); err != "" || !strings.Contains(out, want) {
			t.Errorf("master %s: output %q, error %q; want %q", command, out, err, want)
		}
	}
	if _, err := run("master", "node renew node-2"); !strings.Contains(err, `no node is named "node-2"`) {
		t.Errorf("unknown node: error %q", err)
	}

	api := apiClient{t: t, url: url}
	api.do("POST", "/api/terminal", map[string]string{"target": "unknown", "command": "status"}, http.StatusNotFound, nil)
	api.do("POST", "/api/terminal", map[string]string{"target": a.node.ID, "command": "status\nserver list"}, http.StatusBadRequest, nil)
	api.do("POST", "/api/terminal", map[string]string{"target": a.node.ID, "command": `server command x say "hi`}, http.StatusBadRequest, nil)
}

// runTerminal runs a command in the panel's terminal and returns its output and error.
func runTerminal(t *testing.T, client *http.Client, url, target, command string) (output, errMsg string) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"target": target, "command": command})
	check(t, err)
	res, err := client.Post(url+"/api/terminal", "application/json", bytes.NewReader(body))
	check(t, err)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%s: status %d", command, res.StatusCode)
	}
	var out strings.Builder
	lines := bufio.NewScanner(res.Body)
	for lines.Scan() {
		var ev struct {
			Output string `json:"output"`
			Done   bool   `json:"done"`
			Error  string `json:"error"`
		}
		check(t, json.Unmarshal(lines.Bytes(), &ev))
		out.WriteString(ev.Output)
		if ev.Done {
			return out.String(), ev.Error
		}
	}
	t.Fatalf("%s: the output ended without done", command)
	return "", ""
}
