package e2e

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/mc-server-manager/api/noryx/v1"
	"github.com/QwikByte/mc-server-manager/internal/buildinfo"
	"github.com/QwikByte/mc-server-manager/internal/master/access"
	masterapp "github.com/QwikByte/mc-server-manager/internal/master/app"
	"github.com/QwikByte/mc-server-manager/internal/master/update"
)

func TestUpdates(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	var answer atomic.Int32 // what GitHub's API answers
	answer.Store(http.StatusNotFound)
	var page atomic.Value // where the release page redirects
	page.Store(buildinfo.Repository + "/releases")
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/page" {
			http.Redirect(w, r, page.Load().(string), http.StatusFound)
			return
		}
		if code := int(answer.Load()); code != http.StatusOK {
			w.WriteHeader(code)
			return
		}
		fmt.Fprint(w, `{"tag_name": "v99.0.0", "body": "Faster backups", "published_at": "2026-10-01T12:00:00Z", "html_url": "https://example.com"}`)
	}))
	t.Cleanup(github.Close)
	m.update.API, m.update.Page, m.update.Unit = github.URL, github.URL+"/page", filepath.Join(t.TempDir(), "noryx-master-update.path")
	api := apiClient{t: t, url: m.panel(t).URL}
	post := func(path string) (st update.Status) {
		api.do("POST", path, nil, http.StatusOK, &st)
		return st
	}

	// Without a release, there is nothing to install; a failed check shows why.
	st := post("/api/update/check")
	if st.Latest != nil || st.CheckedAt == nil || st.CheckError != "" || st.Updatable || len(st.Agents) != 0 {
		t.Fatalf("status without a release = %+v", st)
	}
	answer.Store(http.StatusBadGateway)
	if st = post("/api/update/check"); st.CheckError != "GitHub answered 502 Bad Gateway" {
		t.Fatalf("check error = %q", st.CheckError)
	}

	// While the API limits requests, the version comes from the release page, without notes.
	answer.Store(http.StatusForbidden)
	if st = post("/api/update/check"); st.Latest != nil || st.CheckError != "" {
		t.Fatalf("status without a release on the page = %+v", st)
	}
	page.Store(buildinfo.Repository + "/releases/tag/v99.0.1")
	st = post("/api/update/check")
	if want := (update.Release{Version: "v99.0.1", URL: buildinfo.Repository + "/releases/tag/v99.0.1"}); st.Latest == nil || *st.Latest != want {
		t.Fatalf("latest release from the page = %+v, want %+v", st.Latest, want)
	}

	// A newer release is offered with a link to its tag, wherever the API points.
	answer.Store(http.StatusOK)
	st = post("/api/update/check")
	want := update.Release{
		Version: "v99.0.0", Notes: "Faster backups", URL: buildinfo.Repository + "/releases/tag/v99.0.0",
		PublishedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
	}
	if st.Latest == nil || *st.Latest != want || st.CheckError != "" {
		t.Fatalf("latest release = %+v, want %+v", st.Latest, want)
	}

	// A master that wasn't installed from a package is updated on its host. Otherwise it asks
	// systemd for the update, and remembers to update the agents after its restart.
	api.do("POST", "/api/update/master", nil, http.StatusConflict, nil)
	check(t, os.WriteFile(m.update.Unit, nil, 0o600))
	if st = post("/api/update/master"); !st.Updatable || st.Master == nil || st.Master.Error != "" {
		t.Fatalf("status of the master's update = %+v", st)
	}
	for _, file := range []string{"update-request", "update-agents"} {
		_, err := os.Stat(filepath.Join(m.update.DataDir, file))
		check(t, err)
	}

	// Only administrators see and install updates.
	handler := masterapp.API(m.services(t))
	user := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(access.WithGrants(r.Context(), access.Grants{})))
	}))
	t.Cleanup(user.Close)
	for _, path := range []string{"/api/update", "/api/update/check", "/api/update/master", "/api/update/agents"} {
		method := "POST"
		if path == "/api/update" {
			method = "GET"
		}
		apiClient{t: t, url: user.URL}.do(method, path, nil, http.StatusForbidden, nil)
	}

	// An agent only installs releases newer than itself, and only from its package.
	conn, err := m.nodes.Conn(t.Context(), a.node.ID)
	check(t, err)
	for version, want := range map[string]codes.Code{
		"dev":              codes.InvalidArgument,
		"v1":               codes.InvalidArgument,
		"v99.0.0 --reboot": codes.InvalidArgument,
		"v99.0.0":          codes.FailedPrecondition,
	} {
		_, err := noryxv1.NewNodeServiceClient(conn).Update(t.Context(), &noryxv1.UpdateRequest{Version: version})
		if got := status.Code(err); got != want {
			t.Errorf("update to %q: got %v, want %v", version, got, want)
		}
	}
}
