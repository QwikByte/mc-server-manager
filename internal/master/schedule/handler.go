package schedule

import (
	"context"
	"net/http"
	"time"

	"github.com/QwikByte/mc-server-manager/internal/master/access"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const timeout = 30 * time.Second

// Handler serves the tasks of one kind under a path, e.g. /api/backup-jobs. Tasks run on
// any server, so their permissions apply everywhere.
type Handler struct {
	svc          *Service
	kind         string
	view, manage access.Permission
}

func NewHandler(svc *Service, kind string, view, manage access.Permission) *Handler {
	return &Handler{svc: svc, kind: kind, view: view, manage: manage}
}

func (h *Handler) Register(mux access.Mux, base string) {
	view, manage := access.Everywhere(h.view), access.Everywhere(h.manage)
	mux.Handle("GET "+base, view, func(w http.ResponseWriter, r *http.Request) {
		tasks, err := h.svc.List(r.Context(), h.kind)
		write(w, r, http.StatusOK, tasks, err)
	})
	mux.Handle("POST "+base, manage, h.save(http.StatusCreated, func(ctx context.Context, _ string, in Input) (Task, error) {
		return h.svc.Create(ctx, h.kind, in)
	}))
	mux.Handle("GET "+base+"/{id}", view, func(w http.ResponseWriter, r *http.Request) {
		t, err := h.svc.Get(r.Context(), h.kind, r.PathValue("id"))
		write(w, r, http.StatusOK, t, err)
	})
	mux.Handle("PUT "+base+"/{id}", manage, h.save(http.StatusOK, func(ctx context.Context, id string, in Input) (Task, error) {
		return h.svc.Update(ctx, h.kind, id, in)
	}))
	mux.Handle("DELETE "+base+"/{id}", manage, func(w http.ResponseWriter, r *http.Request) {
		write(w, r, http.StatusNoContent, nil, h.svc.Delete(r.Context(), h.kind, r.PathValue("id")))
	})
	// The run continues in the background; the task shows when it is done.
	mux.Handle("POST "+base+"/{id}/run", manage, func(w http.ResponseWriter, r *http.Request) {
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
