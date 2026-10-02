package settings

import (
	"net/http"

	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings", h.get)
	mux.HandleFunc("PUT /api/settings", h.update)
}

type view struct {
	Settings Settings `json:"settings"`
	Master   Master   `json:"master"`
}

func (h *Handler) get(w http.ResponseWriter, _ *http.Request) {
	httpapi.WriteJSON(w, http.StatusOK, view{h.svc.Get(), h.svc.Master()})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req Settings
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	updated, err := h.svc.Update(r.Context(), req)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, view{updated, h.svc.Master()})
}
