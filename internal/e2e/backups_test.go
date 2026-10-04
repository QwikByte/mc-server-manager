package e2e

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/schedule"
)

type backupView struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Paths []string `json:"paths"`
	JobID string   `json:"jobId"`
}

func TestBackups(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	srv := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	api := apiClient{t: t, url: m.panel(t).URL}
	base := "/api/nodes/" + srv.NodeID + "/servers/" + srv.ServerID
	data := filepath.Join(a.runtime.dir, srv.ServerID)
	write := func(name, content string) {
		check(t, os.MkdirAll(filepath.Dir(filepath.Join(data, name)), 0o750))
		check(t, os.WriteFile(filepath.Join(data, name), []byte(content), 0o600))
	}
	read := func(name string) string {
		content, _ := os.ReadFile(filepath.Join(data, name))
		return string(content)
	}
	write("world/level.dat", "level")
	write("world/region/r.0.0.mca", "chunks")
	write("server.properties", "motd=hi\nrcon.password=s3cret\n")
	write("plugins/LuckPerms.jar", "jar")

	// A running server saves its worlds before they are archived, and keeps running.
	api.do("POST", base+"/start", nil, http.StatusNoContent, nil)
	var b backupView
	api.do("POST", base+"/backups", map[string]any{"label": "Before the update", "selection": map[string]any{"worlds": true, "config": true}},
		http.StatusCreated, &b)
	if b.Label != "Before the update" || !slices.Equal(b.Paths, []string{"server.properties", "world"}) {
		t.Fatalf("backup = %+v", b)
	}
	if got, want := a.runtime.consoleCommands(), []string{"save-off", "save-all flush", "save-on"}; !slices.Equal(got, want) {
		t.Fatalf("console commands = %q, want %q", got, want)
	}
	archive := api.do("GET", base+"/backups/"+b.ID+"/download", nil, http.StatusOK, nil)
	zr, err := zip.NewReader(bytes.NewReader([]byte(archive)), int64(len(archive)))
	check(t, err)
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	if want := []string{"server.properties", "world/", "world/level.dat", "world/region/", "world/region/r.0.0.mca"}; !slices.Equal(names, want) {
		t.Fatalf("archive = %q, want %q", names, want)
	}
	// Downloads hide the secrets; the backup itself keeps them.
	properties, err := zr.Open("server.properties")
	check(t, err)
	if content, _ := io.ReadAll(properties); string(content) != "motd=hi\nrcon.password=<hidden>\n" {
		t.Fatalf("downloaded server.properties = %q", content)
	}

	// Restoring brings back the worlds and settings, but leaves the plugins.
	write("world/region/r.0.0.mca", "griefed")
	write("server.properties", "motd=changed\n")
	write("plugins/Other.jar", "jar")
	api.do("POST", base+"/backups/"+b.ID+"/restore", nil, http.StatusNoContent, nil)
	if read("world/region/r.0.0.mca") != "chunks" || read("server.properties") != "motd=hi\nrcon.password=s3cret\n" || read("plugins/Other.jar") != "jar" {
		t.Fatal("the backup was not restored as selected")
	}
	if servers, _ := a.runtime.List(t.Context()); servers[0].State != noryxv1.ServerState_SERVER_STATE_RUNNING {
		t.Fatal("the server was not started again after restoring")
	}

	// Invalid selections and locations are refused.
	for _, selection := range []map[string]any{{}, {"paths": []string{"../other"}}} {
		api.do("POST", base+"/backups", map[string]any{"selection": selection}, http.StatusBadRequest, nil)
	}
	api.do("POST", base+"/backups", map[string]any{"selection": map[string]any{"everything": true}, "location": "elsewhere"}, http.StatusBadRequest, nil)
	api.do("GET", base+"/backups/20260101-000000-aaaaaa/download", nil, http.StatusNotFound, nil)

	// A job backs up all servers of the node and keeps the newest backup only.
	job := map[string]any{
		"name": "Nightly", "enabled": true,
		"schedule": map[string]any{"days": []int{}, "times": []string{"03:00"}, "timeZone": "Europe/Berlin"},
		"targets":  []map[string]string{{"nodeId": srv.NodeID}},
		"settings": map[string]any{"selection": map[string]any{"everything": true}, "keep": 1},
	}
	var task schedule.Task
	api.do("POST", "/api/backup-jobs", job, http.StatusCreated, &task)
	for range 2 {
		run(t, api, "/api/backup-jobs/"+task.ID)
	}
	var backups []backupView
	api.do("GET", base+"/backups", nil, http.StatusOK, &backups)
	if len(backups) != 2 || backups[0].JobID != task.ID || backups[0].Label != "Nightly" || !slices.Equal(backups[0].Paths, []string{"."}) || backups[1].ID != b.ID {
		t.Fatalf("backups = %+v", backups)
	}
	api.do("DELETE", base+"/backups/"+b.ID, nil, http.StatusNoContent, nil)
	api.do("DELETE", base+"/backups/"+b.ID, nil, http.StatusNotFound, nil)

	// A deleted server is no longer a target, and its backups are gone.
	policy := map[string]any{
		"name": "Broadcast", "enabled": true, "schedule": job["schedule"],
		"targets":  []map[string]string{{"nodeId": srv.NodeID, "serverId": srv.ServerID}},
		"settings": map[string]any{"action": "command", "command": "say Vote for us!"},
	}
	api.do("POST", "/api/policies", policy, http.StatusCreated, &task)
	run(t, api, "/api/policies/"+task.ID)
	if got := a.runtime.consoleCommands(); got[len(got)-1] != "say Vote for us!" {
		t.Fatalf("console commands = %q", got)
	}
	api.do("DELETE", base, nil, http.StatusNoContent, nil)
	api.do("GET", "/api/policies/"+task.ID, nil, http.StatusOK, &task)
	if len(task.Targets) != 0 {
		t.Fatalf("targets after deleting the server = %+v", task.Targets)
	}
	if _, err := os.Stat(filepath.Join(a.dir, "backups", srv.ServerID)); !os.IsNotExist(err) {
		t.Fatalf("the backups of the deleted server remain: %v", err)
	}
}

func TestPolicies(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	survival := m.createServer(t, a, "Survival", noryxv1.ServerType_SERVER_TYPE_PAPER, 25566)
	api := apiClient{t: t, url: m.panel(t).URL}
	api.do("POST", "/api/nodes/"+lobby.NodeID+"/servers/"+lobby.ServerID+"/start", nil, http.StatusNoContent, nil)
	policy := func(name string, settings map[string]any) map[string]any {
		return map[string]any{
			"name": name, "enabled": true, "settings": settings,
			"schedule": map[string]any{"days": []int{1, 3, 5}, "times": []string{"04:00"}, "timeZone": "UTC"},
			"targets":  []map[string]string{{"nodeId": lobby.NodeID}},
		}
	}

	// Operating hours: the node's stopped servers start, running ones stop.
	var task schedule.Task
	api.do("POST", "/api/policies", policy("Opening", map[string]any{"action": "start"}), http.StatusCreated, &task)
	if task.NextRun == nil || task.NextRun.UTC().Hour() != 4 || !slices.Contains([]time.Weekday{1, 3, 5}, task.NextRun.UTC().Weekday()) {
		t.Fatalf("next run = %v", task.NextRun)
	}
	run(t, api, "/api/policies/"+task.ID)
	if servers, _ := a.runtime.List(t.Context()); servers[1].ID != survival.ServerID || servers[1].State != noryxv1.ServerState_SERVER_STATE_RUNNING {
		t.Fatal("the stopped server was not started")
	}
	api.do("POST", "/api/policies", policy("Closing", map[string]any{"action": "stop"}), http.StatusCreated, &task)
	run(t, api, "/api/policies/"+task.ID)
	servers, _ := a.runtime.List(t.Context())
	for _, s := range servers {
		if s.State != noryxv1.ServerState_SERVER_STATE_STOPPED {
			t.Fatalf("%s still runs", s.Name)
		}
	}

	// Policies are validated, and backup jobs are separate from them.
	api.do("POST", "/api/policies", policy("Closing", map[string]any{"action": "stop"}), http.StatusConflict, nil)
	api.do("POST", "/api/policies", policy("Bad", map[string]any{"action": "explode"}), http.StatusBadRequest, nil)
	api.do("POST", "/api/policies", policy("Bad", map[string]any{"action": "command", "command": "say a\nop b"}), http.StatusBadRequest, nil)
	bad := policy("Bad", map[string]any{"action": "start"})
	bad["targets"] = []map[string]string{{"nodeId": "unknown"}}
	api.do("POST", "/api/policies", bad, http.StatusBadRequest, nil)
	api.do("GET", "/api/backup-jobs/"+task.ID, nil, http.StatusNotFound, nil)
	var list []schedule.Task
	api.do("GET", "/api/policies", nil, http.StatusOK, &list)
	if len(list) != 2 || list[0].Name != "Closing" || list[1].LastRun == nil || list[1].LastRun.Error != "" {
		t.Fatalf("policies = %+v", list)
	}
	api.do("DELETE", "/api/policies/"+task.ID, nil, http.StatusNoContent, nil)
}

// run runs a task now and waits until it is done.
func run(t *testing.T, api apiClient, path string) {
	t.Helper()
	var task schedule.Task
	api.do("POST", path+"/run", nil, http.StatusAccepted, &task)
	for deadline := time.Now().Add(10 * time.Second); task.Running; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the task didn't finish")
		}
		api.do("GET", path, nil, http.StatusOK, &task)
	}
	if task.LastRun == nil || task.LastRun.Error != "" {
		t.Fatalf("run of %s: %+v", task.Name, task.LastRun)
	}
}
