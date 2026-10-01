package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

// The __Host- prefix makes browsers enforce Secure, Path=/ and no Domain attribute.
const cookieName = "__Host-mcsm_session"

type userKey struct{}

type Handler struct {
	svc     *Service
	limiter *limiter
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc, limiter: newLimiter()} }

// Register adds the routes that require a session.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/me", h.me)
	mux.HandleFunc("POST /api/auth/logout", h.logout)
}

// Login is the only public API route.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.limiter.allow(r) {
		httpapi.WriteError(w, r, httpapi.Errorf(http.StatusTooManyRequests, "Too many sign-in attempts. Wait a minute and try again."))
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	user, token, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		err = httpapi.Errorf(http.StatusUnauthorized, "Username or password is incorrect.")
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	http.SetCookie(w, sessionCookie(token, int(sessionTTL.Seconds())))
	httpapi.WriteJSON(w, http.StatusOK, user)
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
