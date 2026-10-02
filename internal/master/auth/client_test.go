package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxies(t *testing.T) {
	proxies, err := ParseProxies([]string{"127.0.0.1", "10.0.0.0/8", "::1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"localhost", "10.0.0.0/33", ""} {
		if _, err := ParseProxies([]string{bad}); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	for _, tc := range []struct {
		name, peer, want string
		headers          map[string][]string
	}{
		{"no proxy", "203.0.113.7:1234", "203.0.113.7", map[string][]string{"X-Forwarded-For": {"198.51.100.1"}}},
		{"no header", "127.0.0.1:1234", "127.0.0.1", nil},
		{"proxy", "127.0.0.1:1234", "203.0.113.7", map[string][]string{"X-Forwarded-For": {"203.0.113.7"}}},
		{"forged by the client", "127.0.0.1:1234", "203.0.113.7", map[string][]string{"X-Forwarded-For": {"198.51.100.1, 203.0.113.7"}}},
		{"chain of proxies", "[::1]:1234", "203.0.113.7", map[string][]string{"X-Forwarded-For": {"198.51.100.1", "203.0.113.7, 10.1.2.3"}}},
		{"only proxies", "127.0.0.1:1234", "10.1.2.3", map[string][]string{"X-Forwarded-For": {"10.1.2.3"}}},
		{"garbage", "127.0.0.1:1234", "10.1.2.3", map[string][]string{"X-Forwarded-For": {"evil, 10.1.2.3"}}},
		{"real IP", "127.0.0.1:1234", "2001:db8::7", map[string][]string{"X-Real-Ip": {"2001:db8::7"}}},
		{"forwarded before real IP", "127.0.0.1:1234", "203.0.113.7", map[string][]string{"X-Forwarded-For": {"203.0.113.7"}, "X-Real-Ip": {"198.51.100.1"}}},
	} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr, r.Header = tc.peer, tc.headers
		var got string
		proxies.Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = ClientIP(r) })).ServeHTTP(nil, r)
		if got != tc.want {
			t.Errorf("%s: client = %s, want %s", tc.name, got, tc.want)
		}
	}
}
