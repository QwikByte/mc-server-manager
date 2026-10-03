package ratelimit

import (
	"fmt"
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	const burst = 5
	l := New(burst, time.Hour)
	for range burst {
		if !l.Allow("a") {
			t.Fatal("attempt within the budget refused")
		}
	}
	if l.Allow("a") || !l.Allow("b") {
		t.Fatal("budgets aren't per key")
	}
	// Once full, new keys share a budget, and none is reset.
	for i := len(l.keys); i < maxKeys; i++ {
		l.Allow(fmt.Sprint(i))
	}
	for i := range burst {
		if !l.Allow(fmt.Sprint("new", i)) {
			t.Fatal("new key refused")
		}
	}
	if l.Allow("new") || l.Allow("a") {
		t.Fatal("budgets were reset or new keys don't share one")
	}
}

func TestClient(t *testing.T) {
	for ip, want := range map[string]string{
		"203.0.113.7":          "203.0.113.7",
		"2001:db8:1:2:3:4:5:6": "2001:db8:1:2::/64",
		"2001:db8:1:2::9":      "2001:db8:1:2::/64",
		"fe80::1%eth0":         "fe80::/64",
		"::ffff:203.0.113.7":   "::ffff:203.0.113.7",
	} {
		if got := Client(ip); got != want {
			t.Errorf("Client(%s) = %s, want %s", ip, got, want)
		}
	}
}
