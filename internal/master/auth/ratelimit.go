package auth

import (
	"maps"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const (
	// A client, or a signed-in user, has 5 attempts, then one every 12 seconds.
	clientBurst, clientEvery = 5, 12 * time.Second
	// A username has twice as many, which come back four times as fast, so that a single
	// client can't use up the budget of an account and keep its user out.
	usernameBurst, usernameEvery = 10, 3 * time.Second
	// maxKeys bounds the memory of a limiter. Keys whose budget is full again are dropped,
	// which changes nothing; if that isn't enough, new keys share one budget, so that no
	// budget is ever reset.
	maxKeys = 10_000
)

var errTooManyAttempts = httpapi.Errorf(http.StatusTooManyRequests, "Too many attempts. Wait a minute and try again.")

// limiter throttles attempts per key to slow down password guessing: every key has burst
// attempts, then one per interval.
type limiter struct {
	burst int
	every time.Duration
	mu    sync.Mutex
	keys  map[string]*rate.Limiter
	swept time.Time
}

func newLimiter(burst int, every time.Duration) *limiter {
	return &limiter{burst: burst, every: every, keys: make(map[string]*rate.Limiter)}
}

// allow takes an attempt from the budget of key and reports whether one was left.
func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.keys[key]
	if !ok && len(l.keys) >= maxKeys {
		l.sweep()
		if len(l.keys) >= maxKeys {
			key = "" // the shared budget
		}
		lim, ok = l.keys[key]
	}
	if !ok {
		lim = rate.NewLimiter(rate.Every(l.every), l.burst)
		l.keys[key] = lim
	}
	return lim.Allow()
}

// sweep drops the keys whose budget is full again, at most once per attempt interval, so
// that clients that don't fit anymore can't make every request scan all keys.
func (l *limiter) sweep() {
	now := time.Now()
	if now.Sub(l.swept) < l.every {
		return
	}
	l.swept = now
	maps.DeleteFunc(l.keys, func(_ string, lim *rate.Limiter) bool { return lim.TokensAt(now) >= float64(l.burst) })
}

// clientNetwork is the key of the client of a request: its IPv4 address, or the /64
// network of its IPv6 address, as providers assign them as a whole.
func clientNetwork(r *http.Request) string {
	ip := ClientIP(r)
	if addr, err := netip.ParseAddr(ip); err == nil && !addr.Unmap().Is4() {
		if network, err := addr.WithZone("").Prefix(64); err == nil {
			return network.String()
		}
	}
	return ip
}
