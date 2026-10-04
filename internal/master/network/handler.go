package network

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// powers are the permissions the actions on all servers of a network need on each of them.
var powers = map[string]access.Permission{Start: access.ServersStart, Stop: access.ServersStop, Restart: access.ServersRestart}

func (h *Handler) Register(mux access.Mux) {
	view, manage := access.Everywhere(access.NetworksView), access.Everywhere(access.NetworksManage)
	mux.Handle("GET /api/networks", view, func(w http.ResponseWriter, r *http.Request) {
		networks, err := h.svc.List(r.Context())
		write(w, r, http.StatusOK, networks, err)
	})
	mux.Handle("POST /api/networks", manage, func(w http.ResponseWriter, r *http.Request) {
		var d Draft
		if read(w, r, &d) {
			logging.Note(r.Context(), slog.String("name", d.Name))
			n, err := h.svc.Create(r.Context(), d)
			write(w, r, http.StatusCreated, n, err)
		}
	})
	mux.Handle("GET /api/networks/{id}", view, func(w http.ResponseWriter, r *http.Request) {
		n, err := h.svc.Get(r.Context(), r.PathValue("id"))
		write(w, r, http.StatusOK, n, err)
	})
	mux.Handle("PUT /api/networks/{id}", manage, func(w http.ResponseWriter, r *http.Request) {
		var c Change
		if read(w, r, &c) {
			logging.Note(r.Context(), slog.String("name", c.Name))
			n, err := h.svc.Update(r.Context(), r.PathValue("id"), c)
			write(w, r, http.StatusOK, n, err)
		}
	})
	mux.Handle("DELETE /api/networks/{id}", manage, func(w http.ResponseWriter, r *http.Request) {
		write(w, r, http.StatusNoContent, nil, h.svc.Delete(r.Context(), r.PathValue("id")))
	})
	mux.Handle("POST /api/networks/{id}/apply", manage, func(w http.ResponseWriter, r *http.Request) {
		n, err := h.svc.Apply(r.Context(), r.PathValue("id"))
		write(w, r, http.StatusOK, n, err)
	})
	// Actions on all servers need the permission for each server, which they check.
	for action, p := range powers {
		mux.Handle("POST /api/networks/{id}/"+action, view, h.onServers(p, func(ctx context.Context, n Network, _ http.ResponseWriter, _ *http.Request) error {
			return h.svc.Power(ctx, n, action)
		}))
	}
	mux.Handle("POST /api/networks/{id}/broadcast", view, h.onServers(access.ConsoleCommands, func(ctx context.Context, n Network, w http.ResponseWriter, r *http.Request) error {
		var req struct {
			Message string `json:"message"`
		}
		if err := httpapi.ReadJSON(w, r, &req); err != nil {
			return err
		}
		return h.svc.Broadcast(ctx, n, req.Message)
	}))
	mux.Handle("GET /api/nodes/{node}/servers/{id}/proxy", access.OnServer(access.Properties), h.proxySettings)
	mux.Handle("PUT /api/nodes/{node}/servers/{id}/proxy", access.OnServer(access.Properties), h.updateProxySettings)
}

// onServers wraps an action on the servers of a network, which needs p on each of them.
func (h *Handler) onServers(p access.Permission, action func(context.Context, Network, http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := h.svc.Get(r.Context(), r.PathValue("id"))
		if err == nil {
			logging.Note(r.Context(), slog.String("name", n.Name))
			grants := access.From(r.Context())
			for _, ref := range append([]Ref{n.Proxy}, refs(n.Backends)...) {
				if !grants.On(p, ref.NodeID, ref.ServerID) {
					err = access.Denied(p)
				}
			}
		}
		if err == nil {
			err = action(r.Context(), n, w, r)
		}
		write(w, r, http.StatusNoContent, nil, err)
	}
}

func refs(backends []Backend) []Ref {
	out := make([]Ref, len(backends))
	for i, b := range backends {
		out[i] = b.Ref
	}
	return out
}

// read reads the JSON body of a request into v, or answers with the error.
func read(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := httpapi.ReadJSON(w, r, v); err != nil {
		httpapi.WriteError(w, r, err)
		return false
	}
	return true
}

func write(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	switch {
	case err != nil:
		httpapi.WriteError(w, r, err)
	case v == nil:
		w.WriteHeader(status)
	default:
		httpapi.WriteJSON(w, status, v)
	}
}
