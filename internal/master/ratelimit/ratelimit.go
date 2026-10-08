// Package ratelimit throttles attempts per key, e.g. per client, to slow down guessing.
package ratelimit

import (
	"maps"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// maxKeys bounds the memory of a limiter. Keys whose budget is full again are dropped,
// which changes nothing; if that isn't enough, new keys share one budget, so that no
// budget is ever reset.
const maxKeys = 10_000

// Limiter gives every key burst attempts, then one per interval.
type Limiter struct {
	burst int
	every time.Duration
	mu    sync.Mutex
	keys  map[string]*rate.Limiter
	swept time.Time
}

func New(burst int, every time.Duration) *Limiter {
	return &Limiter{burst: burst, every: every, keys: make(map[string]*rate.Limiter)}
}

// Allow takes an attempt from the budget of key and reports whether one was left.
func (l *Limiter) Allow(key string) bool {
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

// Blocked reports whether key has no attempt left, without taking one, e.g. to take attempts
// only when they fail.
func (l *Limiter) Blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.keys[key]
	if !ok && len(l.keys) >= maxKeys {
		lim, ok = l.keys[""] // the shared budget
	}
	return ok && lim.Tokens() < 1
}

// sweep drops the keys whose budget is full again, at most once per attempt interval, so
// that clients that don't fit anymore can't make every request scan all keys.
func (l *Limiter) sweep() {
	now := time.Now()
	if now.Sub(l.swept) < l.every {
		return
	}
	l.swept = now
	maps.DeleteFunc(l.keys, func(_ string, lim *rate.Limiter) bool { return lim.TokensAt(now) >= float64(l.burst) })
}

// Client is the key of a client by its IP address: the IPv4 address, or the /64 network of
// an IPv6 address, as providers assign them as a whole.
func Client(ip string) string {
	if addr, err := netip.ParseAddr(ip); err == nil && !addr.Unmap().Is4() {
		if network, err := addr.WithZone("").Prefix(64); err == nil {
			return network.String()
		}
	}
	return ip
}
