// Package server exposes the Minecraft servers of all nodes to the panel. Agents are
// the source of truth: every request is forwarded to the agent of the addressed node.
package server

import (
	"context"
	"net/http"
	"time"

	"google.golang.org/grpc"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const (
	queryTimeout  = 10 * time.Second
	actionTimeout = 2 * time.Minute
	createTimeout = 10 * time.Minute // includes pulling the server image
)

// Nodes provides connections to node agents.
type Nodes interface {
	Conn(ctx context.Context, nodeID string) (grpc.ClientConnInterface, error)
}

type Handler struct{ nodes Nodes }

func NewHandler(nodes Nodes) *Handler { return &Handler{nodes: nodes} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/nodes/{node}/servers", h.list)
	mux.HandleFunc("POST /api/nodes/{node}/servers", h.create)
	mux.HandleFunc("POST /api/nodes/{node}/servers/{id}/start", h.lifecycle(func(ctx context.Context, c mcsmv1.ServerServiceClient, id string) error {
		_, err := c.StartServer(ctx, &mcsmv1.StartServerRequest{Id: id})
		return err
	}))
	mux.HandleFunc("POST /api/nodes/{node}/servers/{id}/stop", h.lifecycle(func(ctx context.Context, c mcsmv1.ServerServiceClient, id string) error {
		_, err := c.StopServer(ctx, &mcsmv1.StopServerRequest{Id: id})
		return err
	}))
	mux.HandleFunc("DELETE /api/nodes/{node}/servers/{id}", h.lifecycle(func(ctx context.Context, c mcsmv1.ServerServiceClient, id string) error {
		_, err := c.DeleteServer(ctx, &mcsmv1.DeleteServerRequest{Id: id})
		return err
	}))
	mux.HandleFunc("GET /api/nodes/{node}/servers/{id}/logs", h.logs)
	mux.HandleFunc("POST /api/nodes/{node}/servers/{id}/command", h.command)
}

type view struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Version  string `json:"version"`
	MemoryMB uint32 `json:"memoryMb"`
	Port     uint32 `json:"port"`
	State    string `json:"state"`
}

func toView(s *mcsmv1.Server) view {
	return view{
		ID: s.GetId(), Name: s.GetName(), Version: s.GetVersion(), MemoryMB: s.GetMemoryMb(), Port: s.GetPort(),
		Type: s.GetType().Slug(), State: s.GetState().Slug(),
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	c, err := h.client(ctx, r)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	res, err := c.ListServers(ctx, &mcsmv1.ListServersRequest{})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	views := make([]view, 0, len(res.GetServers()))
	for _, s := range res.GetServers() {
		views = append(views, toView(s))
	}
	httpapi.WriteJSON(w, http.StatusOK, views)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Version    string `json:"version"`
		MemoryMB   uint32 `json:"memoryMb"`
		Port       uint32 `json:"port"`
		AcceptEULA bool   `json:"acceptEula"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), createTimeout)
	defer cancel()
	c, err := h.client(ctx, r)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	res, err := c.CreateServer(ctx, &mcsmv1.CreateServerRequest{
		Name:       req.Name,
		Type:       mcsmv1.ParseServerType(req.Type),
		Version:    req.Version,
		MemoryMb:   req.MemoryMB,
		Port:       req.Port,
		AcceptEula: req.AcceptEULA,
	})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, toView(res.GetServer()))
}

// lifecycle wraps an operation on a single server that returns no data.
func (h *Handler) lifecycle(op func(context.Context, mcsmv1.ServerServiceClient, string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), actionTimeout)
		defer cancel()
		c, err := h.client(ctx, r)
		if err == nil {
			err = op(ctx, c, r.PathValue("id"))
		}
		if err != nil {
			httpapi.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) client(ctx context.Context, r *http.Request) (mcsmv1.ServerServiceClient, error) {
	conn, err := h.nodes.Conn(ctx, r.PathValue("node"))
	if err != nil {
		return nil, err
	}
	return mcsmv1.NewServerServiceClient(conn), nil
}
