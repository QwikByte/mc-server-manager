package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

// The __Host- prefix makes browsers enforce Secure, Path=/ and no Domain attribute.
const cookieName = "__Host-mcsm_session"

type userKey struct{}

type Handler struct {
	svc        *Service
	sessionTTL func() time.Duration
	limiter    *limiter
}

// NewHandler returns the handler of the sign-in. sessionTTL tells how long new sessions last.
func NewHandler(svc *Service, sessionTTL func() time.Duration) *Handler {
	return &Handler{svc: svc, sessionTTL: sessionTTL, limiter: newLimiter()}
}

// UserFrom returns the signed in user of a request that passed Require.
func UserFrom(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userKey{}).(User)
	return user, ok
}

// Register adds the routes for the signed-in user's own account. They need a session but
// no permission.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/me", h.me)
	mux.HandleFunc("POST /api/auth/logout", h.logout)
	mux.HandleFunc("PUT /api/auth/password", h.changePassword)
}

// RegisterPublic adds the routes that work without a session: signing in and setting a
// password with a setup link. They are rate limited per client together.
func (h *Handler) RegisterPublic(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", h.limited(h.login))
	mux.HandleFunc("POST /api/auth/setup/check", h.limited(h.checkSetup))
	mux.HandleFunc("POST /api/auth/setup", h.limited(h.setup))
}

func (h *Handler) limited(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.limiter.allow(r) {
			httpapi.WriteError(w, r, httpapi.Errorf(http.StatusTooManyRequests, "Too many attempts. Wait a minute and try again."))
			return
		}
		next(w, r)
	}
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	ttl := h.sessionTTL()
	user, token, err := h.svc.Login(r.Context(), req.Username, req.Password, ttl)
	if errors.Is(err, ErrInvalidCredentials) {
		err = httpapi.Errorf(http.StatusUnauthorized, "Username or password is incorrect.")
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
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

// setup sets the password with a setup link and signs the user in.
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
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	slog.Info("password set with a setup link", "user", user.Username)
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
	var keep string
	if c, err := r.Cookie(cookieName); err == nil {
		keep = c.Value
	}
	if err := h.svc.ChangePassword(r.Context(), user.ID, req.Current, req.New, keep); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		if err := h.svc.Logout(r.Context(), c.Value); err != nil {
			httpapi.WriteError(w, r, err)
			return
		}
	}
	http.SetCookie(w, sessionCookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteJSON(w, http.StatusOK, r.Context().Value(userKey{}))
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
