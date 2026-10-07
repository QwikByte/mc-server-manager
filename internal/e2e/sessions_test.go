package e2e

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QwikByte/noryx/internal/master/access"
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

// Users whom the settings require to use two-factor authentication may only set it up, and
// use their own account, until they did; the requirement applies to all users or to groups.
func TestRequiredTwoFactorAuthentication(t *testing.T) {
	m := startMaster(t)
	svc := m.services(t)
	srv := httptest.NewTLSServer(masterapp.Handler(svc))
	t.Cleanup(srv.Close)
	const password = "a-long-enough-password"
	ids := map[string]int64{}
	for _, name := range []string{"admin", "alice", "bob"} {
		user, err := svc.Users.CreateUser(t.Context(), name, password)
		check(t, err)
		ids[name] = user.ID
	}
	check(t, svc.Access.MakeAdmin(t.Context(), ids["admin"]))
	signIn := func(username string) (apiClient, auth.User) {
		c := browser(t, srv)
		var user auth.User
		c.do("POST", "/api/auth/login", map[string]string{"username": username, "password": password}, http.StatusOK, &user)
		return c, user
	}
	admin, _ := signIn("admin")
	var mods access.Group
	admin.do("POST", "/api/groups", map[string]any{"name": "Moderators", "permissions": []string{"servers.view"}, "allServers": true}, http.StatusCreated, &mods)
	admin.do("PUT", "/api/users/"+strconv.FormatInt(ids["alice"], 10), map[string]any{"groups": []string{mods.ID}}, http.StatusNoContent, nil)
	admin.do("PUT", "/api/settings", map[string]any{"requireMfa": map[string]any{"groups": []string{mods.ID}}}, http.StatusOK, nil)

	// Alice, a moderator, is told to set it up and can do nothing else; Bob isn't covered.
	alice, user := signIn("alice")
	if !user.MustSetUpMFA {
		t.Fatalf("sign-in of alice = %+v", user)
	}
	denied := func(c apiClient, method, path string, body any) {
		t.Helper()
		if answer := c.do(method, path, body, http.StatusForbidden, nil); !strings.Contains(answer, `"mfa-setup-required"`) {
			t.Errorf("%s %s: %s", method, path, answer)
		}
	}
	denied(alice, "GET", "/api/servers", nil)
	denied(alice, "GET", "/api/access/me", nil)
	denied(alice, "POST", "/api/terminal", map[string]string{"command": "status"})
	var mfa auth.MFA
	alice.do("GET", "/api/auth/mfa", nil, http.StatusOK, &mfa)
	alice.do("GET", "/api/auth/sessions", nil, http.StatusOK, nil)
	alice.do("GET", "/api/preferences", nil, http.StatusOK, nil)
	if !mfa.Required || mfa.Enabled {
		t.Fatalf("mfa = %+v", mfa)
	}
	bob, user := signIn("bob")
	bob.do("GET", "/api/servers", nil, http.StatusOK, nil)
	if user.MustSetUpMFA {
		t.Fatal("bob has to set up two-factor authentication")
	}

	// Once she set it up, she may use the panel.
	var setup auth.MFASetup
	alice.do("POST", "/api/auth/mfa/setup", nil, http.StatusOK, &setup)
	alice.do("POST", "/api/auth/mfa", map[string]string{"password": password, "code": totpCode(t, setup.Secret, time.Now())}, http.StatusOK, nil)
	alice.do("GET", "/api/servers", nil, http.StatusOK, nil)
	alice.do("GET", "/api/auth/me", nil, http.StatusOK, &user)
	if user.MustSetUpMFA || !user.MFA {
		t.Fatalf("me = %+v", user)
	}

	// Required of all, it applies to those signed in at once, the administrator who required
	// it too, who can still set it up.
	admin.do("PUT", "/api/settings", map[string]any{"requireMfa": map[string]any{"all": true}}, http.StatusOK, nil)
	denied(bob, "GET", "/api/servers", nil)
	denied(admin, "GET", "/api/settings", nil)
	admin.do("POST", "/api/auth/mfa/setup", nil, http.StatusOK, nil)
	alice.do("GET", "/api/servers", nil, http.StatusOK, nil)
}
