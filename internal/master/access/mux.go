package access

import (
	"log/slog"
	"net/http"

	"github.com/QwikByte/mc-server-manager/internal/master/auth"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

// Need tells whether the grants of a user allow a request. If not, it returns the
// permission that is missing.
type Need func(r *http.Request, g Grants) (Permission, bool)

// Mux registers the routes of the API. Every route states what it needs, so none can be
// added without deciding who may use it.
type Mux struct{ mux *http.ServeMux }

func NewMux(mux *http.ServeMux) Mux { return Mux{mux: mux} }

// Handle registers a route that runs h only if need allows the request.
func (m Mux) Handle(pattern string, need Need, h http.HandlerFunc) {
	m.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if p, ok := need(r, From(r.Context())); !ok {
			httpapi.WriteError(w, r, Denied(p))
			return
		}
		h(w, r)
	})
}

// Denied is the error for a request that lacks a permission.
func Denied(p Permission) error {
	info, _ := lookup(p)
	return httpapi.Errorf(http.StatusForbidden, "You need the permission %q for this. Ask an administrator for it.", info.Label)
}

// Everywhere needs p on all servers, or simply p if it isn't scoped.
func Everywhere(p Permission) Need {
	return func(_ *http.Request, g Grants) (Permission, bool) { return p, g.Has(p) }
}

// OnServer needs p on the server of a route with the path values {node} and {id}.
func OnServer(p Permission) Need {
	return func(r *http.Request, g Grants) (Permission, bool) {
		return p, g.On(p, r.PathValue("node"), r.PathValue("id"))
	}
}

// OnNode needs p on the whole node whose ID is the path value with the given name.
func OnNode(p Permission, pathValue string) Need {
	return func(r *http.Request, g Grants) (Permission, bool) { return p, g.On(p, r.PathValue(pathValue), "") }
}

// All needs everything that needs require.
func All(needs ...Need) Need {
	return func(r *http.Request, g Grants) (Permission, bool) {
		for _, need := range needs {
			if p, ok := need(r, g); !ok {
				return p, false
			}
		}
		return "", true
	}
}

// SignedIn needs no permission. Handlers using it only show what the user may see, or
// check the permissions for what a request names, e.g. the servers of a bulk install.
func SignedIn(*http.Request, Grants) (Permission, bool) { return "", true }

// Middleware loads the grants of the signed-in user for every request, so changes to
// groups apply right away.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.UserFrom(r.Context())
		if !ok {
			httpapi.WriteError(w, r, httpapi.Errorf(http.StatusUnauthorized, "Sign in to continue."))
			return
		}
		g, err := s.Grants(r.Context(), user.ID)
		if err != nil {
			slog.Error("load permissions", "user", user.Username, "err", err)
			httpapi.WriteError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithGrants(r.Context(), g)))
	})
}
