package template

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

const timeout = 30 * time.Second // saving looks up the plugins and the versions it keeps

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux access.Mux) {
	view, manage := access.Everywhere(access.TemplatesView), access.Everywhere(access.TemplatesManage)
	mux.Handle("GET /api/templates", view, func(w http.ResponseWriter, r *http.Request) {
		templates, err := h.svc.List(r.Context())
		write(w, r, http.StatusOK, templates, err)
	})
	create := func(ctx context.Context, _ string, in Input) (Template, error) { return h.svc.Create(ctx, in) }
	mux.Handle("POST /api/templates", manage, h.save(http.StatusCreated, readInput, create))
	// An imported file is saved like a template from the panel.
	mux.Handle("POST /api/templates/import", manage, h.save(http.StatusCreated, readFile, create))
	mux.Handle("GET /api/templates/{id}", view, func(w http.ResponseWriter, r *http.Request) {
		t, err := h.svc.Get(r.Context(), r.PathValue("id"))
		write(w, r, http.StatusOK, t, err)
	})
	mux.Handle("GET /api/templates/{id}/export", view, h.export)
	mux.Handle("PUT /api/templates/{id}", manage, h.save(http.StatusOK, readInput, h.svc.Update))
	mux.Handle("DELETE /api/templates/{id}", manage, func(w http.ResponseWriter, r *http.Request) {
		write(w, r, http.StatusNoContent, nil, h.svc.Delete(r.Context(), r.PathValue("id")))
	})
}

// save reads a template and creates or updates it.
func (h *Handler) save(status int, read func(http.ResponseWriter, *http.Request) (Input, error), op func(ctx context.Context, id string, in Input) (Template, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := read(w, r)
		if err != nil {
			httpapi.WriteError(w, r, err)
			return
		}
		logging.Note(r.Context(), slog.String("name", in.Name))
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		t, err := op(ctx, r.PathValue("id"), in)
		write(w, r, status, t, err)
	}
}

func readInput(w http.ResponseWriter, r *http.Request) (in Input, err error) {
	err = httpapi.ReadJSON(w, r, &in)
	return in, err
}

// export downloads a template as a file, e.g. to import it on another master.
func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.Attachment(w, t.Name+".json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(Export(t))
}

func write(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, status, v)
}
