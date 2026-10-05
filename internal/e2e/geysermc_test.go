package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// fakeGeyserMC serves the parts of GeyserMC's download server and global API (/v2) the
// master uses: the newest build of Floodgate, which a test can replace with publish, its
// files, and the XUID of the Bedrock player Tim 203.
type fakeGeyserMC struct {
	*httptest.Server
	mu    sync.Mutex
	build int
	files map[string][]byte // by platform
}

func startGeyserMC(t *testing.T) *fakeGeyserMC {
	f := &fakeGeyserMC{}
	mux := http.NewServeMux()
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	f.publish(141)
	mux.HandleFunc("GET /v2/projects/floodgate/versions/latest/builds/latest", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		http.Redirect(w, r, fmt.Sprintf("/v2/projects/floodgate/versions/2.2.5/builds/%d", f.build), http.StatusFound)
	})
	mux.HandleFunc("GET /v2/projects/floodgate/versions/2.2.5/builds/{build}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		downloads := map[string]any{}
		for platform, content := range f.files {
			downloads[platform] = map[string]string{"name": "floodgate-" + platform + ".jar", "sha256": sha256Hex(content)}
		}
		writeJSON(w, map[string]any{"version": "2.2.5", "build": f.build, "time": time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC), "downloads": downloads})
	})
	mux.HandleFunc("GET /v2/projects/floodgate/versions/2.2.5/builds/{build}/downloads/{platform}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if content, ok := f.files[r.PathValue("platform")]; ok && r.PathValue("build") == fmt.Sprint(f.build) {
			_, _ = w.Write(content)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("GET /v2/xbox/xuid/{gamertag}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("gamertag") != "Tim 203" {
			w.WriteHeader(http.StatusServiceUnavailable) // as GeyserMC answers
			writeJSON(w, map[string]string{"message": "Unable to find user in our cache."})
			return
		}
		writeJSON(w, map[string]any{"xuid": 2535432196048835})
	})
	return f
}

// publish makes a build the newest one, with a jar for each proxy.
func (f *fakeGeyserMC) publish(build int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.build, f.files = build, map[string][]byte{}
	for _, platform := range []string{"velocity", "bungee"} {
		f.files[platform] = fmt.Appendf(nil, "floodgate %s %d", platform, build)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
