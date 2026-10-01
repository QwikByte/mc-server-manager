// Package properties lets the panel edit the server.properties of a game server
// through the agent of its node.
package properties

import (
	"context"
	"net/http"
	"time"

	"google.golang.org/grpc"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const timeout = 30 * time.Second

// Nodes provides connections to node agents.
type Nodes interface {
	Conn(ctx context.Context, nodeID string) (grpc.ClientConnInterface, error)
}

type Handler struct{ nodes Nodes }

func NewHandler(nodes Nodes) *Handler { return &Handler{nodes: nodes} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/nodes/{node}/servers/{id}/properties", h.get)
	mux.HandleFunc("PUT /api/nodes/{node}/servers/{id}/properties", h.update)
}

type locked struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	c, err := h.client(ctx, r)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	res, err := c.GetServerProperties(ctx, &mcsmv1.GetServerPropertiesRequest{ServerId: r.PathValue("id")})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	view := struct {
		Exists     bool              `json:"exists"`
		Properties map[string]string `json:"properties"`
		Locked     []locked          `json:"locked"`
	}{res.GetExists(), res.GetProperties(), []locked{}}
	if view.Properties == nil {
		view.Properties = map[string]string{}
	}
	for _, l := range res.GetLocked() {
		view.Locked = append(view.Locked, locked{l.GetKey(), l.GetReason()})
	}
	httpapi.WriteJSON(w, http.StatusOK, view)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Properties map[string]string `json:"properties"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	c, err := h.client(ctx, r)
	if err == nil {
		_, err = c.UpdateServerProperties(ctx, &mcsmv1.UpdateServerPropertiesRequest{ServerId: r.PathValue("id"), Properties: req.Properties})
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) client(ctx context.Context, r *http.Request) (mcsmv1.PropertiesServiceClient, error) {
	conn, err := h.nodes.Conn(ctx, r.PathValue("node"))
	if err != nil {
		return nil, err
	}
	return mcsmv1.NewPropertiesServiceClient(conn), nil
}
