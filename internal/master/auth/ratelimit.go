package auth

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// limiter throttles sign-in attempts per client IP to slow down password guessing.
// Behind a reverse proxy all clients share the proxy's IP and therefore one budget.
type limiter struct {
	mu      sync.Mutex
	clients map[string]*rate.Limiter
}

func newLimiter() *limiter { return &limiter{clients: make(map[string]*rate.Limiter)} }

func (l *limiter) allow(r *http.Request) bool {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.clients) > 10_000 {
		clear(l.clients) // bound memory; a reset only grants a fresh small budget
	}
	lim, ok := l.clients[ip]
	if !ok {
		lim = rate.NewLimiter(rate.Every(12*time.Second), 5)
		l.clients[ip] = lim
	}
	return lim.Allow()
}
