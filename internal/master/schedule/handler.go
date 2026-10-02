package schedule

import (
	"context"
	"net/http"
	"time"

	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const timeout = 30 * time.Second

// Handler serves the tasks of one kind under a path, e.g. /api/backup-jobs.
type Handler struct {
	svc  *Service
	kind string
}

func NewHandler(svc *Service, kind string) *Handler { return &Handler{svc: svc, kind: kind} }

func (h *Handler) Register(mux *http.ServeMux, base string) {
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		tasks, err := h.svc.List(r.Context(), h.kind)
		write(w, r, http.StatusOK, tasks, err)
	})
	mux.HandleFunc("POST "+base, h.save(http.StatusCreated, func(ctx context.Context, _ string, in Input) (Task, error) {
		return h.svc.Create(ctx, h.kind, in)
	}))
	mux.HandleFunc("GET "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		t, err := h.svc.Get(r.Context(), h.kind, r.PathValue("id"))
		write(w, r, http.StatusOK, t, err)
	})
	mux.HandleFunc("PUT "+base+"/{id}", h.save(http.StatusOK, func(ctx context.Context, id string, in Input) (Task, error) {
		return h.svc.Update(ctx, h.kind, id, in)
	}))
	mux.HandleFunc("DELETE "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		write(w, r, http.StatusNoContent, nil, h.svc.Delete(r.Context(), h.kind, r.PathValue("id")))
	})
	// The run continues in the background; the task shows when it is done.
	mux.HandleFunc("POST "+base+"/{id}/run", func(w http.ResponseWriter, r *http.Request) {
		t, err := h.svc.RunNow(r.Context(), h.kind, r.PathValue("id"))
		write(w, r, http.StatusAccepted, t, err)
	})
}

func (h *Handler) save(status int, op func(ctx context.Context, id string, in Input) (Task, error)) http.HandlerFunc {
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
