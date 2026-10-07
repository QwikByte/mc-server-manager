package e2e

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/access"
	masterapp "github.com/QwikByte/noryx/internal/master/app"
	"github.com/QwikByte/noryx/internal/master/plugin"
	"github.com/QwikByte/noryx/internal/master/schedule"
)

// A schedule backs up before it restarts and updates plugins and images, with the
// permissions these need by hand, and its runs tell what each step did and changed.
func TestScheduleSteps(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	serverData{t, filepath.Join(a.runtime.dir, lobby.ServerID)}.write("world/level.dat", "level")
	svc := m.services(t)
	srv := httptest.NewTLSServer(masterapp.Handler(svc))
	t.Cleanup(srv.Close)
	admin, err := svc.Users.CreateUser(t.Context(), "admin", "the-admins-password")
	check(t, err)
	check(t, svc.Access.MakeAdmin(t.Context(), admin.ID))
	api := browser(t, srv)
	api.do("POST", "/api/auth/login", map[string]string{"username": "admin", "password": "the-admins-password"}, http.StatusOK, nil)
	api.do("POST", "/api/plugins/install", map[string]any{"projects": []string{"luckperms"}, "versions": map[string]string{"luckperms": "luckperms10"}, "servers": []plugin.Ref{plugin.Ref(lobby)}},
		http.StatusOK, nil)
	api.do("POST", "/api/nodes/"+lobby.NodeID+"/servers/"+lobby.ServerID+"/start", nil, http.StatusNoContent, nil)
	policy := func(name string, settings map[string]any) map[string]any {
		return map[string]any{
			"name": name, "enabled": true, "settings": settings,
			"schedule": map[string]any{"times": []string{"04:00"}, "timeZone": "UTC"}, "targets": []map[string]string{{"nodeId": lobby.NodeID}},
		}
	}
	backUpFirst := map[string]any{"selection": map[string]any{"worlds": true}, "keep": 2}

	// Someone who may manage schedules, but not back up or update, can't make them do so.
	var group access.Group
	api.do("POST", "/api/groups", map[string]any{"name": "Schedulers", "permissions": []string{"policies.manage"}}, http.StatusCreated, &group)
	var invited struct{ User struct{ ID int64 } }
	api.do("POST", "/api/users", map[string]any{"username": "scheduler", "groups": []string{group.ID}}, http.StatusCreated, &invited)
	restrictedGrants, err := svc.Access.Grants(t.Context(), invited.User.ID)
	check(t, err)
	handler := masterapp.API(svc)
	restricted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(access.WithGrants(r.Context(), restrictedGrants)))
	}))
	t.Cleanup(restricted.Close)
	scheduler := apiClient{t: t, url: restricted.URL}
	for _, settings := range []map[string]any{
		{"action": "restart", "backup": backUpFirst},
		{"action": "image"},
		{"action": "plugins"},
	} {
		scheduler.do("POST", "/api/policies", policy("Denied", settings), http.StatusForbidden, nil)
	}
	scheduler.do("POST", "/api/policies", policy("Restart", map[string]any{"action": "restart"}), http.StatusCreated, nil)

	// The lobby is backed up, as a backup job would, before it restarts.
	var task schedule.Task
	api.do("POST", "/api/policies", policy("Nightly", map[string]any{"action": "restart", "backup": backUpFirst}), http.StatusCreated, &task)
	if task.SavedBy != "admin" {
		t.Fatalf("saved by %q", task.SavedBy)
	}
	restarts := len(a.runtime.restarted())
	steps := runSteps(t, api, "/api/policies/"+task.ID)
	if !slices.Equal(stepNames(steps), []string{"Back up server succeeded", "Restart server succeeded"}) || len(a.runtime.restarted()) != restarts+1 {
		t.Fatalf("steps = %+v", steps)
	}
	var backups []backupView
	api.do("GET", "/api/nodes/"+lobby.NodeID+"/servers/"+lobby.ServerID+"/backups", nil, http.StatusOK, &backups)
	if len(backups) != 1 || backups[0].JobID != task.ID || backups[0].Label != "Nightly" {
		t.Fatalf("backups = %+v", backups)
	}

	// Updates tell what they changed.
	api.do("POST", "/api/policies", policy("Plugins", map[string]any{"action": "plugins"}), http.StatusCreated, &task)
	steps = runSteps(t, api, "/api/policies/"+task.ID)
	if len(steps) != 1 || steps[0].Outcome != schedule.Succeeded || !strings.Contains(steps[0].Change, "Updated") || !strings.Contains(steps[0].Change, "once it restarts") {
		t.Fatalf("steps = %+v", steps)
	}
	api.do("POST", "/api/policies", policy("Image", map[string]any{"action": "image"}), http.StatusCreated, &task)
	if steps = runSteps(t, api, "/api/policies/"+task.ID); !slices.Equal(stepNames(steps), []string{"Update image succeeded"}) {
		t.Fatalf("steps = %+v", steps)
	}
}

// runSteps runs a task now, waits until it is done and returns the steps of the run.
func runSteps(t *testing.T, api apiClient, path string) []schedule.Step {
	t.Helper()
	var task schedule.Task
	api.do("POST", path+"/run", nil, http.StatusAccepted, &task)
	for deadline := time.Now().Add(10 * time.Second); task.Running; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the task didn't finish")
		}
		api.do("GET", path, nil, http.StatusOK, &task)
	}
	var runs []schedule.Run
	api.do("GET", path+"/runs", nil, http.StatusOK, &runs)
	if len(runs) == 0 || runs[0].Error != "" {
		t.Fatalf("runs of %s: %+v", task.Name, runs)
	}
	return runs[0].Steps
}

func stepNames(steps []schedule.Step) []string {
	var names []string
	for _, s := range steps {
		names = append(names, s.Action+" "+s.Outcome)
	}
	return names
}
