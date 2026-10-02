// Package server exposes the Minecraft servers of all nodes to the panel. Agents are
// the source of truth: every request is forwarded to the agent of the addressed node.
package server

import (
	"context"
	"math"
	"net/http"
	"slices"
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

// Nodes provides the nodes and connections to their agents.
type Nodes interface {
	List(ctx context.Context) ([]node.Node, error)
	Get(ctx context.Context, id string) (node.Node, error)
	Conn(ctx context.Context, nodeID string) (grpc.ClientConnInterface, error)
}

// Networks tells whether a server can be deleted without breaking a network.
type Networks interface {
	CheckRemovable(ctx context.Context, nodeID, serverID string) error
}

// Tasks forget deleted servers, so that backup jobs and policies no longer run on them.
type Tasks interface {
	Forget(ctx context.Context, nodeID, serverID string) error
}

type Handler struct {
	nodes    Nodes
	networks Networks
	tasks    Tasks
}

func NewHandler(nodes Nodes, networks Networks, tasks Tasks) *Handler {
	return &Handler{nodes: nodes, networks: networks, tasks: tasks}
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
	mux.HandleFunc("POST /api/nodes/{node}/servers/{id}/restart", h.lifecycle(func(ctx context.Context, c mcsmv1.ServerServiceClient, id string) error {
		_, err := c.RestartServer(ctx, &mcsmv1.RestartServerRequest{Id: id})
		return err
	}))
	mux.HandleFunc("DELETE /api/nodes/{node}/servers/{id}", h.delete)
	mux.HandleFunc("PUT /api/nodes/{node}/servers/{id}", h.update)
	mux.HandleFunc("POST /api/nodes/{node}/servers/{id}/duplicate", h.duplicate)
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
	settings
}

// settings are the settings of a server that can be changed after it was created.
type settings struct {
	Java          string   `json:"java"`
	RestartPolicy string   `json:"restartPolicy"`
	AikarFlags    bool     `json:"aikarFlags"`
	JVMOptions    []string `json:"jvmOptions"`
	CPULimit      float64  `json:"cpuLimit"` // in cores, 0 means no limit
}

// check validates the settings that the agent can't, and converts the restart policy
// and CPU limit; an empty restart policy means the default.
func (s settings) check() (mcsmv1.RestartPolicy, uint32, error) {
	policy := mcsmv1.ParseRestartPolicy(s.RestartPolicy)
	switch {
	case s.CPULimit < 0 || s.CPULimit > 1024:
		return policy, 0, httpapi.Errorf(http.StatusBadRequest, "Enter a CPU limit in cores, or 0 for no limit.")
	case policy == mcsmv1.RestartPolicy_RESTART_POLICY_UNSPECIFIED && s.RestartPolicy != "":
		return policy, 0, httpapi.Errorf(http.StatusBadRequest, "Choose when the server starts on its own.")
	}
	return policy, uint32(math.Round(s.CPULimit * 1000)), nil
}

func toView(s *mcsmv1.Server) view {
	return view{
		ID: s.GetId(), Name: s.GetName(), Version: s.GetVersion(), MemoryMB: s.GetMemoryMb(), Port: s.GetPort(),
		Type: s.GetType().Slug(), State: s.GetState().Slug(), Storage: s.GetStorage(),
		settings: settings{
			Java: s.GetJava(), RestartPolicy: s.GetRestartPolicy().Slug(), AikarFlags: s.GetAikarFlags(),
			JVMOptions: append([]string{}, s.GetJvmOptions()...), CPULimit: float64(s.GetCpuMillis()) / 1000,
		},
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

// create creates a server. Besides the basics, it takes the settings and server.properties
// a template provides.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Version    string `json:"version"`
		MemoryMB   uint32 `json:"memoryMb"`
		Port       uint32 `json:"port"`
		AcceptEULA bool   `json:"acceptEula"`
		Storage    string `json:"storage"`
		settings
		Properties map[string]string `json:"properties"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), createTimeout)
	defer cancel()
	policy, cpuMillis, err := req.check()
	var c mcsmv1.ServerServiceClient
	if err == nil {
		c, err = h.client(ctx, r)
	}
	if err == nil {
		err = h.checkLimits(ctx, r.PathValue("node"), "", req.Port, req.MemoryMB)
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	res, err := c.CreateServer(ctx, &mcsmv1.CreateServerRequest{
		Name:          req.Name,
		Type:          mcsmv1.ParseServerType(req.Type),
		Version:       req.Version,
		MemoryMb:      req.MemoryMB,
		Port:          req.Port,
		AcceptEula:    req.AcceptEULA,
		Storage:       req.Storage,
		Java:          req.Java,
		RestartPolicy: policy,
		AikarFlags:    req.AikarFlags,
		JvmOptions:    req.JVMOptions,
		CpuMillis:     cpuMillis,
		Properties:    req.Properties,
	})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, toView(res.GetServer()))
}

// duplicate copies a server with its data into a new server on the same node.
func (h *Handler) duplicate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Port uint32 `json:"port"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), createTimeout) // copying worlds takes a while
	defer cancel()
	c, err := h.client(ctx, r)
	var list *mcsmv1.ListServersResponse
	if err == nil {
		list, err = c.ListServers(ctx, &mcsmv1.ListServersRequest{})
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	i := slices.IndexFunc(list.GetServers(), func(s *mcsmv1.Server) bool { return s.GetId() == r.PathValue("id") })
	if i < 0 {
		httpapi.WriteError(w, r, httpapi.Errorf(http.StatusNotFound, "Server not found."))
		return
	}
	err = h.checkLimits(ctx, r.PathValue("node"), "", req.Port, list.GetServers()[i].GetMemoryMb())
	var res *mcsmv1.DuplicateServerResponse
	if err == nil {
		res, err = c.DuplicateServer(ctx, &mcsmv1.DuplicateServerRequest{Id: r.PathValue("id"), Name: req.Name, Port: req.Port})
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, toView(res.GetServer()))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Version  string `json:"version"`
		MemoryMB uint32 `json:"memoryMb"`
		Port     uint32 `json:"port"`
		settings
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), createTimeout) // a new Java version pulls an image
	defer cancel()
	policy, cpuMillis, err := req.check()
	var c mcsmv1.ServerServiceClient
	if err == nil {
		c, err = h.client(ctx, r)
	}
	if err == nil {
		err = h.checkLimits(ctx, r.PathValue("node"), r.PathValue("id"), req.Port, req.MemoryMB)
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	res, err := c.UpdateServer(ctx, &mcsmv1.UpdateServerRequest{
		Id: r.PathValue("id"), Name: req.Name, Version: req.Version, MemoryMb: req.MemoryMB, Port: req.Port,
		Java: req.Java, RestartPolicy: policy, AikarFlags: req.AikarFlags,
		JvmOptions: req.JVMOptions, CpuMillis: cpuMillis,
	})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, toView(res.GetServer()))
}

// delete deletes a server with its data and backups, unless a network needs it.
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	nodeID, id := r.PathValue("node"), r.PathValue("id")
	ctx, cancel := context.WithTimeout(r.Context(), actionTimeout)
	defer cancel()
	err := h.networks.CheckRemovable(ctx, nodeID, id)
	var c mcsmv1.ServerServiceClient
	if err == nil {
		c, err = h.client(ctx, r)
	}
	if err == nil {
		_, err = c.DeleteServer(ctx, &mcsmv1.DeleteServerRequest{Id: id})
	}
	if err == nil {
		err = h.tasks.Forget(ctx, nodeID, id)
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
