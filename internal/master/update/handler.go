package update

import (
	"net/http"

	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register adds the routes, which only administrators may use. Each responds with the status.
func (h *Handler) Register(mux access.Mux) {
	mux.Handle("GET /api/update", access.AdminsOnly, func(w http.ResponseWriter, r *http.Request) { h.respond(w, r, nil) })
	mux.Handle("POST /api/update/check", access.AdminsOnly, func(w http.ResponseWriter, r *http.Request) {
		h.svc.Check(r.Context())
		h.respond(w, r, nil)
	})
	mux.Handle("POST /api/update/master", access.AdminsOnly, func(w http.ResponseWriter, r *http.Request) {
		h.respond(w, r, h.svc.UpdateMaster(r.Context()))
	})
	mux.Handle("POST /api/update/agents", access.AdminsOnly, func(w http.ResponseWriter, r *http.Request) {
		h.respond(w, r, h.svc.UpdateAgents(r.Context()))
	})
	mux.Handle("POST /api/update/agents/{node}", access.AdminsOnly, func(w http.ResponseWriter, r *http.Request) {
		h.respond(w, r, h.svc.UpdateAgent(r.Context(), r.PathValue("node")))
	})
}

// respond writes the status, or err.
func (h *Handler) respond(w http.ResponseWriter, r *http.Request, err error) {
	var st Status
	if err == nil {
		st, err = h.svc.Status(r.Context())
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, st)
}
