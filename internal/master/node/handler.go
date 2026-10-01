package node

import (
	"context"
	"net/http"
	"sync"
	"time"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const probeTimeout = 3 * time.Second

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/nodes", h.list)
	mux.HandleFunc("POST /api/nodes", h.create)
	mux.HandleFunc("GET /api/nodes/{id}", h.get)
	mux.HandleFunc("DELETE /api/nodes/{id}", h.delete)
	mux.HandleFunc("POST /api/nodes/{id}/join-token", h.joinToken)
}

type view struct {
	Node
	Status string `json:"status"` // pending, online or offline
	Info   *info  `json:"info,omitempty"`
}

type info struct {
	AgentVersion string `json:"agentVersion"`
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	CPUCount     uint32 `json:"cpuCount"`
	MemoryBytes  uint64 `json:"memoryBytes"`
	Runtime      string `json:"runtime"`
}

// probe asks the agent for its machine info, which also tells whether it is reachable.
func (h *Handler) probe(ctx context.Context, n Node) view {
	v := view{Node: n, Status: "pending"}
	if n.EnrolledAt == nil {
		return v
	}
	v.Status = "offline"
	conn, err := h.svc.Conn(ctx, n.ID)
	if err != nil {
		return v
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	res, err := mcsmv1.NewNodeServiceClient(conn).GetInfo(ctx, &mcsmv1.GetInfoRequest{})
	if err != nil {
		return v
	}
	v.Status = "online"
	v.Info = &info{res.GetAgentVersion(), res.GetHostname(), res.GetOs(), res.GetCpuCount(), res.GetMemoryBytes(), res.GetRuntime()}
	return v
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.svc.List(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	views := make([]view, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		wg.Go(func() { views[i] = h.probe(r.Context(), n) })
	}
	wg.Wait()
	httpapi.WriteJSON(w, http.StatusOK, views)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, h.probe(r.Context(), n))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	n, token, err := h.svc.Create(r.Context(), req.Name, req.Address)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"node": view{Node: n, Status: "pending"}, "joinToken": token.String()})
}

func (h *Handler) joinToken(w http.ResponseWriter, r *http.Request) {
	token, err := h.svc.NewJoinToken(r.Context(), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"joinToken": token.String()})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
