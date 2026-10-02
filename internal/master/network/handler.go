package network

import (
	"context"
	"net/http"

	"github.com/QwikByte/mc-server-manager/internal/master/access"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux access.Mux) {
	view, manage := access.Everywhere(access.NetworksView), access.Everywhere(access.NetworksManage)
	mux.Handle("GET /api/networks", view, func(w http.ResponseWriter, r *http.Request) {
		networks, err := h.svc.List(r.Context())
		write(w, r, http.StatusOK, networks, err)
	})
	mux.Handle("POST /api/networks", manage, h.create)
	mux.Handle("GET /api/networks/{id}", view, withNetwork(h.svc.Get))
	mux.Handle("DELETE /api/networks/{id}", manage, func(w http.ResponseWriter, r *http.Request) {
		write(w, r, http.StatusNoContent, nil, h.svc.Delete(r.Context(), r.PathValue("id")))
	})
	mux.Handle("POST /api/networks/{id}/apply", manage, withNetwork(h.svc.Apply))
	mux.Handle("POST /api/networks/{id}/backends", manage, h.addBackend)
	mux.Handle("DELETE /api/networks/{id}/backends/{server}", manage, withBackend(h.svc.RemoveBackend))
	mux.Handle("POST /api/networks/{id}/backends/{server}/default", manage, withBackend(h.svc.MakeDefault))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Proxy Ref    `json:"proxy"`
		Lobby Ref    `json:"lobby"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	n, err := h.svc.Create(r.Context(), req.Name, req.Proxy, req.Lobby)
	write(w, r, http.StatusCreated, n, err)
}

func (h *Handler) addBackend(w http.ResponseWriter, r *http.Request) {
	var ref Ref
	if err := httpapi.ReadJSON(w, r, &ref); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	n, err := h.svc.AddBackend(r.Context(), r.PathValue("id"), ref)
	write(w, r, http.StatusOK, n, err)
}

// withNetwork wraps an operation on a network that returns it.
func withNetwork(op func(ctx context.Context, id string) (Network, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := op(r.Context(), r.PathValue("id"))
		write(w, r, http.StatusOK, n, err)
	}
}

// withBackend wraps an operation on a backend of a network that returns the network.
func withBackend(op func(ctx context.Context, id, serverID string) (Network, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := op(r.Context(), r.PathValue("id"), r.PathValue("server"))
		write(w, r, http.StatusOK, n, err)
	}
}

func write(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, status, v)
}
