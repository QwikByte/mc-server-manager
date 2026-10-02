package node

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/QwikByte/mc-server-manager/internal/master/access"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const probeTimeout = 3 * time.Second

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register adds the routes. Users see the nodes on which they may see the node or servers.
func (h *Handler) Register(mux access.Mux) {
	mux.Handle("GET /api/nodes", access.SignedIn, h.list)
	mux.Handle("POST /api/nodes", access.Everywhere(access.NodesEnroll), h.create)
	mux.Handle("GET /api/nodes/{id}", access.SignedIn, h.get)
	mux.Handle("PUT /api/nodes/{id}", access.OnNode(access.NodesEdit, "id"), h.update)
	mux.Handle("DELETE /api/nodes/{id}", access.OnNode(access.NodesDelete, "id"), h.delete)
	mux.Handle("POST /api/nodes/{id}/join-token", access.Everywhere(access.NodesEnroll), h.joinToken)
	mux.Handle("POST /api/nodes/{id}/certificate", access.OnNode(access.NodesCertificates, "id"), h.renewCertificate)
}

type view struct {
	Node
	Status               string     `json:"status"` // pending, online or offline
	Info                 *info      `json:"info,omitempty"`
	CertificateExpiresAt *time.Time `json:"certificateExpiresAt,omitempty"`
}

type info struct {
	AgentVersion string            `json:"agentVersion"`
	Hostname     string            `json:"hostname"`
	OS           string            `json:"os"`
	CPUCount     uint32            `json:"cpuCount"`
	MemoryBytes  uint64            `json:"memoryBytes"`
	Runtime      string            `json:"runtime"`
	Storage      []storageLocation `json:"storage"`
}

type storageLocation struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	FreeBytes  uint64 `json:"freeBytes"`
	TotalBytes uint64 `json:"totalBytes"`
}

// probe asks the agent for its machine info, which also tells whether it is reachable.
func (h *Handler) probe(ctx context.Context, n Node) view {
	v := view{Node: n, Status: "pending"}
	if n.EnrolledAt == nil {
		return v
	}
	v.Status = "offline"
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	res, cert, err := h.svc.Status(ctx, n.ID)
	if err != nil {
		return v
	}
	v.Status = "online"
	v.Info = &info{res.GetAgentVersion(), res.GetHostname(), res.GetOs(), res.GetCpuCount(), res.GetMemoryBytes(), res.GetRuntime(), []storageLocation{}}
	for _, l := range res.GetStorage() {
		v.Info.Storage = append(v.Info.Storage, storageLocation{l.GetName(), l.GetPath(), l.GetFreeBytes(), l.GetTotalBytes()})
	}
	v.CertificateExpiresAt = &cert.NotAfter
	return v
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.svc.List(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	grants := access.From(r.Context())
	nodes = slices.DeleteFunc(nodes, func(n Node) bool { return !grants.SeesNode(n.ID) })
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
	if err == nil && !access.From(r.Context()).SeesNode(n.ID) {
		err = access.Denied(access.NodesView)
	}
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
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{
		"node": view{Node: n, Status: "pending"}, "joinToken": token.String(), "joinTokenExpiresAt": token.ExpiresAt,
	})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Address string `json:"address"`
		Settings
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	n, err := h.svc.Update(r.Context(), Node{ID: r.PathValue("id"), Name: req.Name, Address: req.Address, Settings: req.Settings})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, h.probe(r.Context(), n))
}

func (h *Handler) joinToken(w http.ResponseWriter, r *http.Request) {
	token, err := h.svc.NewJoinToken(r.Context(), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"joinToken": token.String(), "joinTokenExpiresAt": token.ExpiresAt})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) renewCertificate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), renewTimeout)
	defer cancel()
	cert, err := h.svc.RenewCertificate(ctx, r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]time.Time{"certificateExpiresAt": cert.NotAfter})
}
