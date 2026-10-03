package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Browsers keep to HTTPS when the master serves a certificate they trust; behind a proxy, it
// decides, and a self-signed certificate would lock them out once it changes.
func TestStrictTransportSecurity(t *testing.T) {
	for _, hsts := range []bool{true, false} {
		h := securityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), hsts)
		for url, want := range map[string]string{"https://panel.example/": "max-age=31536000", "http://panel.example/": ""} {
			if !hsts {
				want = ""
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
			if got := rec.Header().Get("Strict-Transport-Security"); got != want {
				t.Errorf("%s with HSTS %v: Strict-Transport-Security = %q, want %q", url, hsts, got, want)
			}
		}
	}
}
