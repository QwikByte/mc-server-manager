package terminal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A running command sends empty lines, so that a proxy in front of the master keeps waiting
// for one that writes nothing for a while, and none once it ended.
func TestKeepAlive(t *testing.T) {
	rec := httptest.NewRecorder()
	out := &events{enc: json.NewEncoder(rec), flusher: http.NewResponseController(rec)}
	stop := out.keepAlive(time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	stop()
	body := rec.Body.String()
	if n := strings.Count(body, "{}\n"); n == 0 || body != strings.Repeat("{}\n", n) {
		t.Fatalf("sent %q", body)
	}
	time.Sleep(5 * time.Millisecond)
	if rec.Body.String() != body {
		t.Fatalf("sent %q after the command ended", strings.TrimPrefix(rec.Body.String(), body))
	}
}
