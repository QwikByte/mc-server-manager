// Package server exposes the Minecraft servers of all nodes to the panel. Agents are
// the source of truth: every request is forwarded to the agent of the addressed node.
package server

import (
	"context"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
	"github.com/QwikByte/mc-server-manager/internal/master/node"
)

const (
	probeTimeout  = 3 * time.Second // for listing the servers of all nodes
	queryTimeout  = 10 * time.Second
	actionTimeout = 2 * time.Minute
	createTimeout = 10 * time.Minute // includes pulling the server image
)

// Nodes provides connections to node agents.
type Nodes interface {
	List(ctx context.Context) ([]node.Node, error)
	Conn(ctx context.Context, nodeID string) (grpc.ClientConnInterface, error)
}

// Networks tells whether a server can be deleted without breaking a network.
type Networks interface {
	CheckRemovable(ctx context.Context, nodeID, serverID string) error
}

type Handler struct {
	nodes    Nodes
	networks Networks
}

func NewHandler(nodes Nodes, networks Networks) *Handler {
	return &Handler{nodes: nodes, networks: networks}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/servers", h.listAll)
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
	deleteServer := h.lifecycle(func(ctx context.Context, c mcsmv1.ServerServiceClient, id string) error {
		_, err := c.DeleteServer(ctx, &mcsmv1.DeleteServerRequest{Id: id})
		return err
	})
	mux.HandleFunc("DELETE /api/nodes/{node}/servers/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := h.networks.CheckRemovable(r.Context(), r.PathValue("node"), r.PathValue("id")); err != nil {
			httpapi.WriteError(w, r, err)
			return
		}
		deleteServer(w, r)
	})
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
	Storage  string `json:"storage"`
}

func toView(s *mcsmv1.Server) view {
	return view{
		ID: s.GetId(), Name: s.GetName(), Version: s.GetVersion(), MemoryMB: s.GetMemoryMb(), Port: s.GetPort(),
		Type: s.GetType().Slug(), State: s.GetState().Slug(), Storage: s.GetStorage(),
	}
}

// nodeServer is a server with the node it runs on.
type nodeServer struct {
	view
	NodeID   string `json:"nodeId"`
	NodeName string `json:"nodeName"`
}

// listAll returns the servers of all reachable nodes, e.g. to choose the servers of a network.
func (h *Handler) listAll(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.nodes.List(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	perNode := make([][]nodeServer, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		if n.EnrolledAt == nil {
			continue
		}
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(r.Context(), probeTimeout)
			defer cancel()
			conn, err := h.nodes.Conn(ctx, n.ID)
			if err != nil {
				return
			}
			res, err := mcsmv1.NewServerServiceClient(conn).ListServers(ctx, &mcsmv1.ListServersRequest{})
			if err != nil {
				return // offline nodes are left out
			}
			for _, s := range res.GetServers() {
				perNode[i] = append(perNode[i], nodeServer{toView(s), n.ID, n.Name})
			}
		})
	}
	wg.Wait()
	all := []nodeServer{}
	for _, servers := range perNode {
		all = append(all, servers...)
	}
	httpapi.WriteJSON(w, http.StatusOK, all)
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
		Storage    string `json:"storage"`
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
		Storage:    req.Storage,
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
