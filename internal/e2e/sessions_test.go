package e2e

import (
	"net/http"
	"net/http/httptest"
	"testing"

	masterapp "github.com/QwikByte/noryx/internal/master/app"
	"github.com/QwikByte/noryx/internal/master/auth"
)

// Users see where they are signed in and end their own sessions, never those of others.
func TestSessions(t *testing.T) {
	m := startMaster(t)
	svc := m.services(t)
	srv := httptest.NewTLSServer(masterapp.Handler(svc))
	t.Cleanup(srv.Close)
	const password = "a-long-enough-password"
	for _, name := range []string{"alice", "bob"} {
		_, err := svc.Users.CreateUser(t.Context(), name, password)
		check(t, err)
	}
	signIn := func(username string) apiClient {
		c := browser(t, srv)
		c.do("POST", "/api/auth/login", map[string]string{"username": username, "password": password}, http.StatusOK, nil)
		return c
	}
	laptop, phone, bob := signIn("alice"), signIn("alice"), signIn("bob")

	var sessions, bobs []auth.Session
	laptop.do("GET", "/api/auth/sessions", nil, http.StatusOK, &sessions)
	bob.do("GET", "/api/auth/sessions", nil, http.StatusOK, &bobs)
	if len(sessions) != 2 || !sessions[0].Current || sessions[1].Current || sessions[0].IP != "127.0.0.1" || len(bobs) != 1 {
		t.Fatalf("sessions = %+v, bob's = %+v", sessions, bobs)
	}
	laptop.do("DELETE", "/api/auth/sessions/"+bobs[0].ID, nil, http.StatusNotFound, nil)
	bob.do("GET", "/api/auth/me", nil, http.StatusOK, nil)

	laptop.do("DELETE", "/api/auth/sessions/"+sessions[1].ID, nil, http.StatusNoContent, nil)
	phone.do("GET", "/api/auth/me", nil, http.StatusUnauthorized, nil)
	phone = signIn("alice")
	laptop.do("DELETE", "/api/auth/sessions", nil, http.StatusNoContent, nil)
	phone.do("GET", "/api/auth/me", nil, http.StatusUnauthorized, nil)
	laptop.do("GET", "/api/auth/me", nil, http.StatusOK, nil)
	bob.do("GET", "/api/auth/me", nil, http.StatusOK, nil)
}
