package auth

import (
	"net/http"
	"time"

	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
	"github.com/QwikByte/mc-server-manager/internal/master/ratelimit"
)

const (
	// A client, or a signed-in user, has 5 attempts, then one every 12 seconds.
	clientBurst, clientEvery = 5, 12 * time.Second
	// A username has twice as many, which come back four times as fast, so that a single
	// client can't use up the budget of an account and keep its user out.
	usernameBurst, usernameEvery = 10, 3 * time.Second
)

var errTooManyAttempts = httpapi.Errorf(http.StatusTooManyRequests, "Too many attempts. Wait a minute and try again.")

// clientNetwork is the key of the client of a request.
func clientNetwork(r *http.Request) string { return ratelimit.Client(ClientIP(r)) }
