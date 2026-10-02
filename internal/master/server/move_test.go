package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMovesGuard(t *testing.T) {
	moves := NewMoves()
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
	call := func(pattern, method string) int {
		mux := http.NewServeMux()
		mux.HandleFunc(pattern, moves.Guard(pattern, ok))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, "/api/nodes/n1/servers/s1/start", nil))
		return rec.Code
	}
	if !moves.start(&Move{ServerID: "s1", StartedAt: time.Now()}) || moves.start(&Move{ServerID: "s1"}) {
		t.Fatal("a server moved twice at once")
	}
	if code := call("POST /api/nodes/{node}/servers/{id}/start", "POST"); code != http.StatusConflict {
		t.Fatalf("change while moving: %d", code)
	}
	if code := call("GET /api/nodes/{node}/servers/{id}/start", "GET"); code != http.StatusNoContent {
		t.Fatalf("read while moving: %d", code)
	}
	moves.update("s1", func(m *Move) { now := time.Now(); m.FinishedAt = &now })
	if code := call("POST /api/nodes/{node}/servers/{id}/start", "POST"); code != http.StatusNoContent || moves.Busy("s1") {
		t.Fatalf("change after the move: %d", code)
	}
}
