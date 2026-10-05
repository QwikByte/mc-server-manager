package e2e

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/access"
	masterapp "github.com/QwikByte/noryx/internal/master/app"
	"github.com/QwikByte/noryx/internal/master/auth"
	"github.com/QwikByte/noryx/internal/master/logs"
	"github.com/QwikByte/noryx/internal/master/network"
)

// logEntry is an entry as the panel gets it.
type logEntry struct {
	logs.Entry
	Level string `json:"level"`
}

func TestLogs(t *testing.T) {
	m := startMaster(t)
	prev := slog.Default()
	slog.SetDefault(slog.New(m.logs.Handler(slog.LevelInfo)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	a := m.startAgent(t, "node-1")
	go m.logs.Collect(t.Context(), m.nodes)
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	survival := m.createServer(t, a, "Survival", noryxv1.ServerType_SERVER_TYPE_PAPER, 25566)
	path := func(s network.Ref) string { return "/api/nodes/" + s.NodeID + "/servers/" + s.ServerID }

	svc := m.services(t)
	srv := httptest.NewTLSServer(masterapp.Handler(svc))
	t.Cleanup(srv.Close)
	admin, err := svc.Users.CreateUser(t.Context(), "admin", "the-admins-password")
	check(t, err)
	check(t, svc.Access.MakeAdmin(t.Context(), admin.ID))
	root := browser(t, srv)
	root.do("POST", "/api/auth/login", map[string]string{"username": "admin", "password": "a-wrong-password"}, http.StatusUnauthorized, nil)
	root.do("POST", "/api/auth/login", map[string]string{"username": "admin", "password": "the-admins-password"}, http.StatusOK, nil)
	invite := func(name string, groups ...string) apiClient {
		var invited struct {
			SetupLink auth.SetupLink `json:"setupLink"`
		}
		root.do("POST", "/api/users", map[string]any{"username": name, "groups": groups}, http.StatusCreated, &invited)
		user := browser(t, srv)
		user.do("POST", "/api/auth/setup", map[string]string{"token": invited.SetupLink.Token, "password": "a-good-password"}, http.StatusOK, nil)
		return user
	}
	var watchers access.Group
	root.do("POST", "/api/groups", map[string]any{
		"name": "Lobby watchers", "permissions": []string{"logs.view", "servers.view", "terminal.use"}, "targets": []network.Ref{lobby},
	}, http.StatusCreated, &watchers)
	watcher, nobody := invite("watcher", watchers.ID), invite("nobody")

	// Requests that change something are logged with the user and what they concern,
	// those that are denied as warnings, invalid ones as information for the user.
	root.do("POST", "/api/groups", map[string]any{"name": "", "permissions": []string{}}, http.StatusBadRequest, nil)
	root.do("POST", path(lobby)+"/start", nil, http.StatusNoContent, nil)
	watcher.do("POST", path(survival)+"/stop", nil, http.StatusForbidden, nil)
	watcher.do("POST", path(lobby)+"/restart", nil, http.StatusForbidden, nil)
	nobody.do("GET", "/api/logs", nil, http.StatusForbidden, nil)

	// Entries are written in the background, and those of agents collected.
	waitFor := func(c apiClient, query string, ok func([]logEntry) bool) []logEntry {
		t.Helper()
		var entries []logEntry
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			c.do("GET", "/api/logs?"+query, nil, http.StatusOK, &entries)
			if ok(entries) {
				return entries
			}
		}
		t.Fatalf("%s: entries = %+v", query, entries)
		return nil
	}
	find := func(entries []logEntry, source, message string) *logEntry {
		i := slices.IndexFunc(entries, func(e logEntry) bool { return e.Source == source && e.Message == message })
		if i < 0 {
			return nil
		}
		return &entries[i]
	}
	all := waitFor(root, "", func(entries []logEntry) bool {
		return find(entries, logs.FromAgent, "Start server") != nil && find(entries, logs.FromMaster, "Restart server denied") != nil &&
			find(entries, logs.FromMaster, "Create group failed") != nil
	})
	started, agentStarted := find(all, logs.FromMaster, "Start server"), find(all, logs.FromAgent, "Start server")
	if started == nil || started.User != "admin" || started.NodeName != "node-1" || started.ServerName != "Lobby" ||
		started.Category != "servers" || started.Attrs["status"] != "204" || started.Attrs["ip"] != "127.0.0.1" {
		t.Errorf("audit entry = %+v", started)
	}
	if agentStarted.NodeID != a.node.ID || agentStarted.ServerName != "Lobby" || agentStarted.Attrs["origin"] != "master" {
		t.Errorf("agent entry = %+v", agentStarted)
	}
	if denied := find(all, logs.FromMaster, "Stop server denied"); denied == nil || denied.Level != "warn" || denied.User != "watcher" {
		t.Errorf("denied entry = %+v", denied)
	}
	if invalid := find(all, logs.FromMaster, "Create group failed"); invalid == nil || invalid.Level != "info" || invalid.Attrs["err"] == "" {
		t.Errorf("entry of the invalid request = %+v", invalid)
	}
	for _, want := range []struct{ message, user string }{
		{"Sign in failed", "admin"}, {"Sign in", "admin"}, {"Set password with setup link", "nobody"},
		{"Create group", "admin"}, {"Invite user", "admin"},
	} {
		if e := find(all, logs.FromMaster, want.message); e == nil || e.User != want.user {
			t.Errorf("%s: entry = %+v", want.message, e)
		}
	}
	if e := find(all, logs.FromMaster, "Create group"); e.Attrs["group"] != "Lobby watchers" {
		t.Errorf("notes of the handler are missing: %+v", e.Attrs)
	}

	// Users only see the entries about the nodes and servers of their scope.
	seen := waitFor(watcher, "", func(entries []logEntry) bool { return len(entries) > 0 })
	for _, e := range seen {
		if e.ServerID != lobby.ServerID {
			t.Errorf("the watcher sees %+v", e)
		}
	}
	if find(seen, logs.FromMaster, "Restart server denied") == nil {
		t.Errorf("the watcher misses their own entry: %+v", seen)
	}

	// Filters, paging and statistics.
	warnings := waitFor(root, "level=warn&category=servers", func([]logEntry) bool { return true })
	if len(warnings) != 2 {
		t.Errorf("warnings about servers = %+v", warnings)
	}
	page := waitFor(root, "limit=2", func([]logEntry) bool { return true })
	older := waitFor(root, "limit=2&before="+strconv.FormatInt(page[1].ID, 10), func([]logEntry) bool { return true })
	if len(page) != 2 || len(older) != 2 || older[0].ID >= page[1].ID {
		t.Errorf("pages %+v and %+v", page, older)
	}
	root.do("GET", "/api/logs?level=loud", nil, http.StatusBadRequest, nil)
	var buckets []logs.Bucket
	root.do("GET", "/api/logs/stats", nil, http.StatusOK, &buckets)
	var infos, warns int
	for _, b := range buckets {
		infos, warns = infos+b.Info, warns+b.Warn
	}
	if len(buckets) != 24 || infos == 0 || warns < 3 {
		t.Errorf("stats = %+v", buckets)
	}

	// New entries stream in as they are added.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/logs/stream?source=master&category=servers", nil)
	check(t, err)
	res, err := root.client.Do(req)
	check(t, err)
	defer res.Body.Close()
	root.do("POST", path(lobby)+"/stop", nil, http.StatusNoContent, nil)
	lines := bufio.NewScanner(res.Body)
	var streamed logEntry
	for streamed.Message != "Stop server" && lines.Scan() {
		if data, ok := strings.CutPrefix(lines.Text(), "data: "); ok {
			check(t, json.Unmarshal([]byte(data), &streamed))
		}
	}
	if streamed.Message != "Stop server" || streamed.User != "admin" {
		t.Errorf("streamed entry = %+v", streamed)
	}

	// The log can be exported, and read in the terminal.
	body := root.do("GET", "/api/logs/export?format=csv&server="+lobby.ServerID, nil, http.StatusOK, nil)
	rows, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	check(t, err)
	if rows[0][0] != "time" || !slices.ContainsFunc(rows, func(r []string) bool { return r[4] == "Start server" && r[8] == "Lobby" }) {
		t.Errorf("export = %q", body)
	}
	if out, errMsg := runTerminal(t, root.client, srv.URL, "master", "logs --level warn --category servers"); errMsg != "" ||
		!strings.Contains(out, "Stop server denied") || strings.Contains(out, "Start server") {
		t.Errorf("master logs: %q, %q", out, errMsg)
	}
	if out, errMsg := runTerminal(t, root.client, srv.URL, a.node.ID, "logs"); errMsg != "" || !strings.Contains(out, "Start server") {
		t.Errorf("agent logs: %q, %q", out, errMsg)
	}
	if _, errMsg := runTerminal(t, watcher.client, srv.URL, a.node.ID, "logs"); !strings.Contains(errMsg, "See logs") {
		t.Errorf("the watcher reads the agent's whole log: %q", errMsg)
	}
}
