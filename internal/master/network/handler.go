package network

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/operation"
)

type Handler struct {
	svc *Service
	ops *operation.Operations
}

func NewHandler(svc *Service, ops *operation.Operations) *Handler {
	return &Handler{svc: svc, ops: ops}
}

// networkTimeout covers configuring all servers of a large network one after the other.
const networkTimeout = time.Hour

// run runs an action on a network as an operation, which those see who may see networks.
func (h *Handler) run(w http.ResponseWriter, r *http.Request, kind, name, id string, status int, task operation.Task) {
	h.ops.Run(w, r, operation.Spec{
		Kind: kind, Subject: name, NetworkID: id, Status: status, Timeout: networkTimeout, Category: logging.Networks,
		Visible: func(g access.Grants) bool { return g.Has(access.NetworksView) },
	}, task)
}

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
			h.run(w, r, "network.create", d.Name, "", http.StatusCreated, func(ctx context.Context) (any, error) { return h.svc.Create(ctx, d) })
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
			id := r.PathValue("id")
			h.run(w, r, "network.update", c.Name, id, http.StatusOK, func(ctx context.Context) (any, error) { return h.svc.Update(ctx, id, c) })
		}
	})
	mux.Handle("DELETE /api/networks/{id}", manage, h.onNetwork("network.delete", http.StatusNoContent, func(ctx context.Context, n Network) (any, error) {
		return nil, h.svc.Delete(ctx, n.ID)
	}))
	mux.Handle("POST /api/networks/{id}/apply", manage, h.onNetwork("network.apply", http.StatusOK, func(ctx context.Context, n Network) (any, error) {
		return h.svc.Apply(ctx, n.ID)
	}))
	// Actions on all servers need the permission for each server, which they check.
	for action, p := range powers {
		mux.Handle("POST /api/networks/{id}/"+action, view, func(w http.ResponseWriter, r *http.Request) {
			n, err := h.allowed(r, p)
			if err != nil {
				httpapi.WriteError(w, r, err)
				return
			}
			h.run(w, r, "network."+action, n.Name, n.ID, http.StatusNoContent, func(ctx context.Context) (any, error) {
				return nil, h.svc.Power(ctx, n, action)
			})
		})
	}
	mux.Handle("POST /api/networks/{id}/broadcast", view, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Message string `json:"message"`
		}
		n, err := h.allowed(r, access.ConsoleCommands)
		if err == nil {
			err = httpapi.ReadJSON(w, r, &req)
		}
		if err == nil {
			err = h.svc.Broadcast(r.Context(), n, req.Message)
		}
		write(w, r, http.StatusNoContent, nil, err)
	})
	mux.Handle("GET /api/nodes/{node}/servers/{id}/proxy", access.OnServer(access.Properties), h.proxySettings)
	mux.Handle("PUT /api/nodes/{node}/servers/{id}/proxy", access.OnServer(access.Properties), h.updateProxySettings)
}

// onNetwork runs an action on the network of a request as an operation.
func (h *Handler) onNetwork(kind string, status int, action func(context.Context, Network) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := h.svc.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			httpapi.WriteError(w, r, err)
			return
		}
		logging.Note(r.Context(), slog.String("name", n.Name))
		h.run(w, r, kind, n.Name, n.ID, status, func(ctx context.Context) (any, error) { return action(ctx, n) })
	}
}

// allowed returns the network of a request if the user has p on each of its servers.
func (h *Handler) allowed(r *http.Request, p access.Permission) (Network, error) {
	n, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		return n, err
	}
	logging.Note(r.Context(), slog.String("name", n.Name))
	grants := access.From(r.Context())
	for _, ref := range append([]Ref{n.Proxy}, refs(n.Backends)...) {
		if !grants.On(p, ref.NodeID, ref.ServerID) {
			return n, access.Denied(p)
		}
	}
	return n, nil
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
