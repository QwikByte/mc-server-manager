package operation

import (
	"net/http"
	"slices"

	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/auth"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

type Handler struct{ ops *Operations }

func NewHandler(ops *Operations) *Handler { return &Handler{ops: ops} }

// Register adds the routes. Users see the operations they started, and those of others
// about what they may see.
func (h *Handler) Register(mux access.Mux) {
	mux.Handle("GET /api/operations", access.SignedIn, func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, h.visible(r))
	})
	mux.Handle("GET /api/operations/{id}", access.SignedIn, func(w http.ResponseWriter, r *http.Request) {
		ops := h.visible(r)
		i := slices.IndexFunc(ops, func(op Operation) bool { return op.ID == r.PathValue("id") })
		if i < 0 {
			httpapi.WriteError(w, r, httpapi.Errorf(http.StatusNotFound, "Operation not found."))
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, ops[i])
	})
}

func (h *Handler) visible(r *http.Request) []Operation {
	user, _ := auth.UserFrom(r.Context())
	return h.ops.List(user.ID, access.From(r.Context()))
}
