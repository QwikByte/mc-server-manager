package overlay

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/operation"
)

// Networks reach the servers of other nodes over the private network, if both are members.
type Networks interface {
	// CheckLeave fails if a node can't leave, as a network with legacy forwarding would then
	// reach servers on other nodes at ports that no firewall protects.
	CheckLeave(ctx context.Context, nodeID string) error
	// ApplyAcross configures the networks again that have servers on the node and on others.
	ApplyAcross(ctx context.Context, nodeID string) error
}

type Handler struct {
	svc      *Service
	networks Networks
	ops      *operation.Operations
}

func NewHandler(svc *Service, networks Networks, ops *operation.Operations) *Handler {
	return &Handler{svc: svc, networks: networks, ops: ops}
}

// Overview is the network as a user sees it: the settings, and the members on the nodes
// the user may see.
type Overview struct {
	Settings
	Members []Member `json:"members"`
}

func (h *Handler) Register(mux access.Mux) {
	manage := access.Everywhere(access.OverlayManage)
	mux.Handle("GET /api/overlay", access.SignedIn, func(w http.ResponseWriter, r *http.Request) {
		settings, err := h.svc.Settings(r.Context())
		var members []Member
		if err == nil {
			members, err = h.svc.Members(r.Context())
		}
		grants := access.From(r.Context())
		members = slices.DeleteFunc(members, func(m Member) bool { return !grants.SeesNode(m.NodeID) })
		write(w, r, Overview{settings, members}, err)
	})
	mux.Handle("PUT /api/overlay", manage, func(w http.ResponseWriter, r *http.Request) {
		var s Settings
		err := httpapi.ReadJSON(w, r, &s)
		if err == nil {
			s, err = h.svc.UpdateSettings(r.Context(), s)
		}
		write(w, r, s, err)
	})
	// Like the overview, the status only names the peers on the nodes the user may see.
	mux.Handle("GET /api/nodes/{id}/overlay", access.OnNode(access.NodesView, "id"), func(w http.ResponseWriter, r *http.Request) {
		st, err := h.svc.Status(r.Context(), r.PathValue("id"))
		grants := access.From(r.Context())
		st.Peers = slices.DeleteFunc(st.Peers, func(p Peer) bool { return !grants.SeesNode(p.NodeID) })
		write(w, r, st, err)
	})
	mux.Handle("POST /api/nodes/{id}/overlay", manage, h.withEndpoint(h.svc.Join))
	mux.Handle("PUT /api/nodes/{id}/overlay", manage, h.withEndpoint(h.svc.SetEndpoint))
	mux.Handle("POST /api/nodes/{id}/overlay/rotate", manage, h.rotate)
	mux.Handle("DELETE /api/nodes/{id}/overlay", manage, h.leave)
}

func (h *Handler) withEndpoint(fn func(ctx context.Context, nodeID, endpoint string) (Member, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Endpoint string `json:"endpoint"`
		}
		var m Member
		err := httpapi.ReadJSON(w, r, &req)
		if err == nil {
			logging.Note(r.Context(), slog.String("endpoint", req.Endpoint))
			m, err = fn(r.Context(), r.PathValue("id"), req.Endpoint)
		}
		write(w, r, m, err)
	}
}

// rotate replaces the key of a member and then configures the networks again that reach its
// servers, or servers from it, over the network: the others let into the ports of their
// servers only the key a node had when it got access, so that a node that later gets the
// address of a removed one has none.
func (h *Handler) rotate(w http.ResponseWriter, r *http.Request) {
	h.change(w, r, "overlay.rotate", "key", http.StatusOK, func(ctx context.Context, id string) (any, error) { return h.svc.Rotate(ctx, id) })
}

// leave removes a node from the network and then configures the networks that reached its
// servers, or servers from it, over the network: they reach them at public ports again,
// which restarts these servers.
func (h *Handler) leave(w http.ResponseWriter, r *http.Request) {
	if err := h.networks.CheckLeave(r.Context(), r.PathValue("id")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	h.change(w, r, "overlay.leave", "overlay", http.StatusNoContent, func(ctx context.Context, id string) (any, error) { return nil, h.svc.Leave(ctx, id) })
}

// change changes a member in an operation, in the step it names, and then configures the
// networks again that reach servers across it.
func (h *Handler) change(w http.ResponseWriter, r *http.Request, kind, step string, status int, fn func(ctx context.Context, nodeID string) (any, error)) {
	id := r.PathValue("id")
	n, err := h.svc.nodes.Get(r.Context(), id)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	h.ops.Run(w, r, operation.Spec{
		Kind: kind, Subject: n.Name, NodeID: id, Status: status, Timeout: time.Hour, Category: logging.Nodes,
		Visible: func(g access.Grants) bool { return g.On(access.NodesView, id, "") },
	}, func(ctx context.Context) (any, error) {
		operation.Step(ctx, step)
		result, err := fn(ctx, id)
		if err != nil {
			return nil, err
		}
		operation.Step(ctx, "networks")
		return result, h.networks.ApplyAcross(ctx, id)
	})
}

func write(w http.ResponseWriter, r *http.Request, v any, err error) {
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, v)
}
