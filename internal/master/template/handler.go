package template

import (
	"context"
	"net/http"
	"time"

	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const timeout = 30 * time.Second // saving looks up the plugins on Modrinth

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/templates", func(w http.ResponseWriter, r *http.Request) {
		templates, err := h.svc.List(r.Context())
		write(w, r, http.StatusOK, templates, err)
	})
	mux.HandleFunc("POST /api/templates", h.save(http.StatusCreated, func(ctx context.Context, _ string, in Input) (Template, error) {
		return h.svc.Create(ctx, in)
	}))
	mux.HandleFunc("GET /api/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		t, err := h.svc.Get(r.Context(), r.PathValue("id"))
		write(w, r, http.StatusOK, t, err)
	})
	mux.HandleFunc("PUT /api/templates/{id}", h.save(http.StatusOK, h.svc.Update))
	mux.HandleFunc("DELETE /api/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		write(w, r, http.StatusNoContent, nil, h.svc.Delete(r.Context(), r.PathValue("id")))
	})
}

// save reads a template and creates or updates it.
func (h *Handler) save(status int, op func(ctx context.Context, id string, in Input) (Template, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if err := httpapi.ReadJSON(w, r, &in); err != nil {
			httpapi.WriteError(w, r, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		t, err := op(ctx, r.PathValue("id"), in)
		write(w, r, status, t, err)
	}
}

func write(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, status, v)
}
