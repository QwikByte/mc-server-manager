package settings

import (
	"net/http"
	"strings"

	"github.com/QwikByte/mc-server-manager/internal/master/access"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

type Handler struct {
	svc     *Service
	restart func() error
}

// NewHandler returns the handler of the settings. restart restarts the master; it is nil if
// the master can't restart itself.
func NewHandler(svc *Service, restart func() error) *Handler {
	return &Handler{svc: svc, restart: restart}
}

// Register adds the routes. Only administrators may change the panel's address and HTTPS,
// which can expose the panel to other networks, and restart the master.
func (h *Handler) Register(mux access.Mux) {
	mux.Handle("GET /api/settings", access.Everywhere(access.SettingsView), h.get)
	mux.Handle("PUT /api/settings", access.Everywhere(access.SettingsEdit), h.update)
	mux.Handle("POST /api/master/restart", access.AdminsOnly, h.restartMaster)
}

type view struct {
	Settings Settings `json:"settings"`
	Master   Master   `json:"master"`
}

func (h *Handler) get(w http.ResponseWriter, _ *http.Request) {
	httpapi.WriteJSON(w, http.StatusOK, view{h.svc.Get(), h.svc.Master()})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	req := h.svc.Get() // settings the request leaves out stay as they are
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	cur := h.svc.Get()
	if p, ok := access.AdminsOnly(r, access.From(r.Context())); !ok &&
		(strings.TrimSpace(req.PanelAddr) != cur.PanelAddr || req.PanelHTTPS != cur.PanelHTTPS || strings.TrimSpace(req.PanelDomain) != cur.PanelDomain) {
		httpapi.WriteError(w, r, access.Denied(p))
		return
	}
	updated, err := h.svc.Update(r.Context(), req)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, view{updated, h.svc.Master()})
}

// restartMaster restarts the master once the response is sent.
func (h *Handler) restartMaster(w http.ResponseWriter, r *http.Request) {
	err := httpapi.Errorf(http.StatusConflict, "This master can't restart itself. Restart it on its host, e.g. with: systemctl restart mcsm-master")
	if h.restart != nil {
		err = h.restart()
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
