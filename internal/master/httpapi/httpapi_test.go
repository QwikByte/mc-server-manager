package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCanceledRequestsAreNotLogged(t *testing.T) {
	var logged strings.Builder
	defer slog.SetDefault(slog.Default())
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	gone := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/servers/s1/logs", nil)
	for _, err := range []error{status.Error(codes.Canceled, "context canceled"), fmt.Errorf("query: %w", context.Canceled)} {
		w := httptest.NewRecorder()
		if WriteError(w, gone, err); w.Code != statusClientClosed {
			t.Errorf("%v: status %d", err, w.Code)
		}
	}
	if msg := Message(fmt.Errorf("query: %w", context.Canceled)); msg != "canceled" {
		t.Errorf("Message = %q", msg)
	}
	if logged.Len() != 0 {
		t.Fatalf("logged %q", logged.String())
	}

	// The connection to an agent closed while the client still waits.
	w := httptest.NewRecorder()
	WriteError(w, httptest.NewRequest(http.MethodGet, "/api/servers/s1/logs", nil), status.Error(codes.Canceled, "grpc: the client connection is closing"))
	if w.Code != http.StatusInternalServerError || !strings.Contains(logged.String(), "level=ERROR") {
		t.Fatalf("status %d, logged %q", w.Code, logged.String())
	}
}
