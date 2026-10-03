package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Browsers keep to HTTPS when the master serves TLS itself; behind a proxy, it decides.
func TestStrictTransportSecurity(t *testing.T) {
	h := securityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for url, want := range map[string]string{"https://panel.example/": "max-age=31536000", "http://panel.example/": ""} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
		if got := rec.Header().Get("Strict-Transport-Security"); got != want {
			t.Errorf("%s: Strict-Transport-Security = %q, want %q", url, got, want)
		}
	}
}
