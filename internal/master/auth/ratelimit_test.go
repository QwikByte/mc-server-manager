package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QwikByte/mc-server-manager/internal/master/database"
)

func TestLimiter(t *testing.T) {
	l := newLimiter(clientBurst, clientEvery)
	for range clientBurst {
		if !l.allow("a") {
			t.Fatal("attempt within the budget refused")
		}
	}
	if l.allow("a") || !l.allow("b") {
		t.Fatal("budgets aren't per key")
	}
	// Once full, new keys share a budget, and none is reset.
	for i := len(l.keys); i < maxKeys; i++ {
		l.allow(fmt.Sprint(i))
	}
	for i := range clientBurst {
		if !l.allow(fmt.Sprint("new", i)) {
			t.Fatal("new key refused")
		}
	}
	if l.allow("new") || l.allow("a") {
		t.Fatal("budgets were reset or new keys don't share one")
	}
}

func TestClientNetwork(t *testing.T) {
	for addr, want := range map[string]string{
		"203.0.113.7:1234":            "203.0.113.7",
		"[2001:db8:1:2:3:4:5:6]:1234": "2001:db8:1:2::/64",
		"[2001:db8:1:2::9]:1234":      "2001:db8:1:2::/64",
		"[fe80::1%eth0]:1234":         "fe80::/64",
	} {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.RemoteAddr = addr
		if got := clientNetwork(r); got != want {
			t.Errorf("clientNetwork(%s) = %s, want %s", addr, got, want)
		}
	}
}

// Behind a trusted proxy, clients have budgets of their own, and guessing the password of
// one account from many addresses ends with that account's budget.
func TestSignInLimits(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := NewService(db)
	for _, name := range []string{"alice", "bob"} {
		if _, err := svc.CreateUser(t.Context(), name, name+"s-long-password"); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(svc, func() time.Duration { return time.Hour })
	// No attempt comes back during the test, however slow hashing passwords is.
	h.clients, h.usernames = newLimiter(clientBurst, time.Hour), newLimiter(usernameBurst, time.Hour)
	mux := http.NewServeMux()
	h.RegisterPublic(mux)
	proxies, _ := ParseProxies([]string{"127.0.0.1"})
	login := func(client, username, password string) int {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)))
		r.RemoteAddr = "127.0.0.1:4567"
		r.Header.Set("X-Forwarded-For", client)
		rec := httptest.NewRecorder()
		proxies.Handler(mux).ServeHTTP(rec, r)
		return rec.Code
	}

	// A single client can't keep a user out.
	for range clientBurst {
		if code := login("192.0.2.1", "alice", "wrong"); code != http.StatusUnauthorized {
			t.Fatalf("wrong password: %d", code)
		}
	}
	if code := login("192.0.2.1", "alice", "alices-long-password"); code != http.StatusTooManyRequests {
		t.Fatalf("client over its budget: %d", code)
	}
	if code := login("192.0.2.2", "alice", "alices-long-password"); code != http.StatusOK {
		t.Fatalf("user after a single client guessed: %d", code)
	}

	// Many clients can't guess the password of an account for long.
	for i := range usernameBurst - clientBurst - 1 {
		if code := login(fmt.Sprint("203.0.113.", i), "ALICE", "wrong"); code != http.StatusUnauthorized {
			t.Fatalf("wrong password %d: %d", i, code)
		}
	}
	if code := login("198.51.100.1", "alice", "alices-long-password"); code != http.StatusTooManyRequests {
		t.Fatalf("guessed account: %d", code)
	}
	if code := login("198.51.100.1", "bob", "bobs-long-password"); code != http.StatusOK {
		t.Fatalf("other account: %d", code)
	}

	// The addresses of an IPv6 network share a budget.
	for i := range clientBurst {
		if code := login(fmt.Sprintf("2001:db8::%d", i+1), fmt.Sprint("nobody", i), "wrong"); code != http.StatusUnauthorized {
			t.Fatalf("wrong username %d: %d", i, code)
		}
	}
	if code := login("2001:db8::ff", "bob", "bobs-long-password"); code != http.StatusTooManyRequests {
		t.Fatalf("same network: %d", code)
	}
	if code := login("2001:db8:0:1::1", "bob", "bobs-long-password"); code != http.StatusOK {
		t.Fatalf("other network: %d", code)
	}
}
