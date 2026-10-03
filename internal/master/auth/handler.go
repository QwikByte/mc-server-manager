package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QwikByte/mc-server-manager/internal/logging"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
	"github.com/QwikByte/mc-server-manager/internal/master/ratelimit"
)

// The __Host- prefix makes browsers enforce Secure, Path=/ and no Domain attribute.
const cookieName = "__Host-mcsm_session"

type userKey struct{}

// Handler limits the attempts to guess a password or code per client, per username and,
// for signed-in users, per user, each on its own: anonymous requests can't use up the
// budget of signed-in users.
type Handler struct {
	svc        *Service
	sessionTTL func() time.Duration
	clients    *ratelimit.Limiter
	usernames  *ratelimit.Limiter
	users      *ratelimit.Limiter
}

// NewHandler returns the handler of the sign-in. sessionTTL tells how long new sessions last.
func NewHandler(svc *Service, sessionTTL func() time.Duration) *Handler {
	return &Handler{
		svc: svc, sessionTTL: sessionTTL, clients: ratelimit.New(clientBurst, clientEvery),
		usernames: ratelimit.New(usernameBurst, usernameEvery), users: ratelimit.New(clientBurst, clientEvery),
	}
}

// UserFrom returns the signed in user of a request that passed Require.
func UserFrom(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userKey{}).(User)
	return user, ok
}

// Register adds the routes for the signed-in user's own account. They need a session but
// no permission. Those that check the password are rate limited per user.
func (h *Handler) Register(mux *http.ServeMux) {
	perUser := limitedBy(h.users, func(r *http.Request) string {
		user, _ := UserFrom(r.Context())
		return strconv.FormatInt(user.ID, 10)
	})
	mux.HandleFunc("GET /api/auth/me", h.me)
	mux.HandleFunc("POST /api/auth/logout", h.logout)
	mux.HandleFunc("PUT /api/auth/password", perUser(h.changePassword))
	mux.HandleFunc("PUT /api/auth/language", h.setLanguage)
	mux.HandleFunc("GET /api/auth/mfa", h.mfa)
	mux.HandleFunc("POST /api/auth/mfa/setup", h.setUpMFA)
	mux.HandleFunc("POST /api/auth/mfa", perUser(h.enableMFA))
	mux.HandleFunc("DELETE /api/auth/mfa", perUser(h.disableMFA))
	mux.HandleFunc("POST /api/auth/mfa/recovery-codes", perUser(h.newRecoveryCodes))
}

// RegisterPublic adds the routes that work without a session: signing in and setting a
// password with a setup link. They are rate limited per client together, signing in also
// per username.
func (h *Handler) RegisterPublic(mux *http.ServeMux) {
	perClient := limitedBy(h.clients, clientNetwork)
	mux.HandleFunc("POST /api/auth/login", perClient(h.login))
	mux.HandleFunc("POST /api/auth/setup/check", perClient(h.checkSetup))
	mux.HandleFunc("POST /api/auth/setup", perClient(h.setup))
}

// limitedBy returns a wrapper that rate limits requests by the key of each.
func limitedBy(l *ratelimit.Limiter, key func(*http.Request) string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(key(r)) {
				tooManyAttempts(w, r)
				return
			}
			next(w, r)
		}
	}
}

func tooManyAttempts(w http.ResponseWriter, r *http.Request, attrs ...any) {
	slog.Warn("Too many sign-in attempts", append([]any{logging.Auth, "ip", ClientIP(r), "route", r.Pattern}, attrs...)...)
	httpapi.WriteError(w, r, errTooManyAttempts)
}

// login signs a user in. With two-factor authentication, a request without a code only
// tells that one is needed.
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	// Names that don't fit the pattern belong to no account. Usernames are case-insensitive.
	if usernamePattern.MatchString(req.Username) && !h.usernames.Allow(strings.ToLower(req.Username)) {
		tooManyAttempts(w, r, logging.KeyUser, req.Username)
		return
	}
	ttl := h.sessionTTL()
	user, token, err := h.svc.Login(r.Context(), req.Username, req.Password, req.Code, ttl)
	switch {
	case errors.Is(err, ErrCodeRequired):
		httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"mfaRequired": true})
		return
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, errWrongCode), errors.Is(err, errCodeLocked):
		slog.Warn("Sign in failed", logging.Auth, logging.KeyUser, req.Username, "ip", ClientIP(r), "reason", err)
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	slog.Info("Sign in", logging.Auth, logging.KeyUser, user.Username, "ip", ClientIP(r))
	http.SetCookie(w, sessionCookie(token, int(ttl.Seconds())))
	httpapi.WriteJSON(w, http.StatusOK, user)
}

// checkSetup tells whose password a setup link sets, before the user chooses one.
func (h *Handler) checkSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	user, err := h.svc.SetupUser(r.Context(), req.Token)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, user)
}

// setup sets the password with a setup link and signs the user in, unless signing in needs
// a code too.
func (h *Handler) setup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	ttl := h.sessionTTL()
	user, token, err := h.svc.Setup(r.Context(), req.Token, req.Password, ttl)
	if errors.Is(err, errInvalidSetup) {
		slog.Warn("Set password with setup link failed", logging.Auth, "ip", ClientIP(r), "err", err)
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	slog.Info("Set password with setup link", logging.Auth, logging.KeyUser, user.Username, "ip", ClientIP(r))
	if token == "" {
		httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"mfaRequired": true})
		return
	}
	http.SetCookie(w, sessionCookie(token, int(ttl.Seconds())))
	httpapi.WriteJSON(w, http.StatusOK, user)
}

// changePassword changes the signed-in user's password; other sessions of the user end.
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	user, _ := UserFrom(r.Context())
	err := h.svc.ChangePassword(r.Context(), user.ID, req.Current, req.New, sessionToken(r))
	reply(w, r, "Change password", nil, err)
}

// mfa tells whether two-factor authentication is on for the signed-in user.
func (h *Handler) mfa(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFrom(r.Context())
	m, err := h.svc.MFA(r.Context(), user.ID)
	reply(w, r, "", m, err)
}

// setUpMFA returns a new secret for the user's authenticator app.
func (h *Handler) setUpMFA(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFrom(r.Context())
	setup, err := h.svc.SetUpMFA(r.Context(), user)
	reply(w, r, "", setup, err)
}

// enableMFA turns on two-factor authentication with a code of the app that was set up, and
// returns the recovery codes. The user's other sessions end.
func (h *Handler) enableMFA(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	user, _ := UserFrom(r.Context())
	codes, err := h.svc.EnableMFA(r.Context(), user.ID, req.Password, req.Code, sessionToken(r))
	reply(w, r, "Turn on two-factor authentication", recoveryCodes{codes}, err)
}

func (h *Handler) disableMFA(w http.ResponseWriter, r *http.Request) {
	password, ok := readPassword(w, r)
	if !ok {
		return
	}
	user, _ := UserFrom(r.Context())
	reply(w, r, "Turn off two-factor authentication", nil, h.svc.DisableMFA(r.Context(), user.ID, password))
}

func (h *Handler) newRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	password, ok := readPassword(w, r)
	if !ok {
		return
	}
	user, _ := UserFrom(r.Context())
	codes, err := h.svc.NewRecoveryCodes(r.Context(), user.ID, password)
	reply(w, r, "Create recovery codes", recoveryCodes{codes}, err)
}

type recoveryCodes struct {
	Codes []string `json:"recoveryCodes"`
}

// readPassword reads a request that confirms a change with the user's password.
func readPassword(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req struct {
		Password string `json:"password"`
	}
	err := httpapi.ReadJSON(w, r, &req)
	if err != nil {
		httpapi.WriteError(w, r, err)
	}
	return req.Password, err == nil
}

// reply writes v, or no content if v is nil, or err. A change of the signed-in user's
// account, named by action, is logged.
func reply(w http.ResponseWriter, r *http.Request, action string, v any, err error) {
	if action != "" {
		user, _ := UserFrom(r.Context())
		if err != nil {
			slog.Warn(action+" failed", logging.Auth, logging.KeyUser, user.Username, "ip", ClientIP(r), "err", err)
		} else {
			slog.Info(action, logging.Auth, logging.KeyUser, user.Username, "ip", ClientIP(r))
		}
	}
	switch {
	case err != nil:
		httpapi.WriteError(w, r, err)
	case v == nil:
		w.WriteHeader(http.StatusNoContent)
	default:
		httpapi.WriteJSON(w, http.StatusOK, v)
	}
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(r.Context(), sessionToken(r)); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if user, ok := UserFrom(r.Context()); ok {
		slog.Info("Sign out", logging.Auth, logging.KeyUser, user.Username, "ip", ClientIP(r))
	}
	http.SetCookie(w, sessionCookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteJSON(w, http.StatusOK, r.Context().Value(userKey{}))
}

// setLanguage stores the language of the panel the user chose.
func (h *Handler) setLanguage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Language string `json:"language"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	user, _ := UserFrom(r.Context())
	if err := h.svc.SetLanguage(r.Context(), user.ID, req.Language); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	user.Language = req.Language
	httpapi.WriteJSON(w, http.StatusOK, user)
}

// Require rejects requests without a valid session.
func (h *Handler) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			httpapi.WriteError(w, r, httpapi.Errorf(http.StatusUnauthorized, "Sign in to continue."))
			return
		}
		user, err := h.svc.Authenticate(r.Context(), c.Value)
		if errors.Is(err, ErrNoSession) {
			err = httpapi.Errorf(http.StatusUnauthorized, "Your session has expired. Sign in again.")
		}
		if err != nil {
			httpapi.WriteError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, user)))
	})
}

// sessionToken is the token of the request's session, if any.
func sessionToken(r *http.Request) string {
	if c, err := r.Cookie(cookieName); err == nil {
		return c.Value
	}
	return ""
}

func sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}
