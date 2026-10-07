package usage

import (
	"cmp"
	"context"
	"net/http"
	"time"

	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

type Handler struct{ store *Store }

func NewHandler(store *Store) *Handler { return &Handler{store: store} }

// Register adds the routes. The latest measurement and the warnings only contain what the user
// may see. Changing the thresholds of a node needs the permission to change the node, and of a
// server the permission to change its settings.
func (h *Handler) Register(m access.Mux) {
	m.Handle("GET /api/nodes/{node}/usage", access.SignedIn, h.latest)
	m.Handle("GET /api/nodes/{node}/usage/history", access.OnNode(access.NodesView, "node"), h.history)
	m.Handle("GET /api/nodes/{node}/servers/{id}/usage/history", access.OnServer(access.ServersView), h.history)
	m.Handle("GET /api/usage/warnings", access.SignedIn, h.warnings)
	m.Handle("GET /api/nodes/{node}/usage/thresholds", access.OnNode(access.NodesView, "node"), h.thresholds)
	m.Handle("PUT /api/nodes/{node}/usage/thresholds", access.OnNode(access.NodesEdit, "node"), h.setThresholds)
	m.Handle("GET /api/nodes/{node}/servers/{id}/usage/thresholds", access.OnServer(access.ServersView), h.thresholds)
	m.Handle("PUT /api/nodes/{node}/servers/{id}/usage/thresholds", access.OnServer(access.ServersSettings), h.setThresholds)
}

type latestView struct {
	Time    time.Time    `json:"time"`
	Node    *nodeView    `json:"node,omitempty"`
	Servers []serverView `json:"servers"`
}

type nodeView struct {
	CPUMillis        uint32 `json:"cpuMillis"`
	CPUCount         uint32 `json:"cpuCount"`
	MemoryUsedBytes  uint64 `json:"memoryUsedBytes"`
	MemoryTotalBytes uint64 `json:"memoryTotalBytes"`
}

type serverView struct {
	ID               string   `json:"id"`
	Running          bool     `json:"running"`
	CPUMillis        uint32   `json:"cpuMillis"`
	MemoryBytes      uint64   `json:"memoryBytes"`
	MemoryLimitBytes uint64   `json:"memoryLimitBytes"`
	NetworkReceived  uint64   `json:"networkReceived"`
	NetworkSent      uint64   `json:"networkSent"`
	DiskBytes        uint64   `json:"diskBytes"`
	Players          *players `json:"players,omitempty"`
	TPS              float64  `json:"tps,omitempty"`
	OfflineMode      bool     `json:"offlineMode,omitempty"`
}

type players struct {
	Online uint32   `json:"online"`
	Max    uint32   `json:"max"`
	Names  []string `json:"names"`
}

// latest returns the latest measurement of the agent of a node.
func (h *Handler) latest(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("node")
	stats, err := h.store.Latest(r.Context(), nodeID)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	grants := access.From(r.Context())
	view := latestView{Time: time.Unix(stats.GetTimeUnix(), 0), Servers: []serverView{}}
	if n := stats.GetNode(); grants.On(access.NodesView, nodeID, "") {
		view.Node = &nodeView{n.GetCpuMillis(), n.GetCpuCount(), n.GetMemoryUsedBytes(), n.GetMemoryTotalBytes()}
	}
	for _, s := range stats.GetServers() {
		if !grants.On(access.ServersView, nodeID, s.GetId()) {
			continue
		}
		v := serverView{
			ID: s.GetId(), Running: s.GetRunning(), CPUMillis: s.GetCpuMillis(), MemoryBytes: s.GetMemoryBytes(),
			MemoryLimitBytes: s.GetMemoryLimitBytes(), NetworkReceived: s.GetNetworkReceivedBytesPerSecond(),
			NetworkSent: s.GetNetworkSentBytesPerSecond(), DiskBytes: s.GetDiskBytes(), TPS: s.GetTps(), OfflineMode: s.GetOfflineMode(),
		}
		if p := s.GetPlayers(); p != nil {
			v.Players = &players{p.GetOnline(), p.GetMax(), append([]string{}, p.GetNames()...)}
		}
		view.Servers = append(view.Servers, v)
	}
	httpapi.WriteJSON(w, http.StatusOK, view)
}

// history returns the usage of a node, or of a server if the path names one, over the
// range "day" (the default) or "week".
func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	rng, ok := Ranges[cmp.Or(r.URL.Query().Get("range"), "day")]
	if !ok {
		httpapi.WriteError(w, r, httpapi.Errorf(http.StatusBadRequest, "Choose the range day or week."))
		return
	}
	points, err := h.store.History(r.Context(), r.PathValue("node"), r.PathValue("id"), rng.Span, rng.Step)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"step": rng.Step.Seconds(), "points": points})
}

// thresholdsView are the thresholds that a node or server has instead of the defaults, and the
// defaults, which apply to the measures it has none for.
type thresholdsView struct {
	Own      Thresholds `json:"own"`
	Defaults Thresholds `json:"defaults"`
}

func (h *Handler) thresholdsView(own Thresholds, serverID string) thresholdsView {
	if own == nil {
		own = Thresholds{}
	}
	d := h.store.conf.Thresholds()
	if serverID == "" {
		return thresholdsView{own, d.Nodes}
	}
	return thresholdsView{own, d.Servers}
}

// thresholds returns the thresholds of a node, or of a server if the path names one.
func (h *Handler) thresholds(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("id")
	own, err := h.store.Own(r.Context(), r.PathValue("node"), serverID)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, h.thresholdsView(own, serverID))
}

// setThresholds replaces the thresholds of a node, or of a server if the path names one.
func (h *Handler) setThresholds(w http.ResponseWriter, r *http.Request) {
	nodeID, serverID := r.PathValue("node"), r.PathValue("id")
	own := Thresholds{}
	if err := httpapi.ReadJSON(w, r, &own); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), statsTimeout)
	defer cancel()
	err := h.store.exists(ctx, nodeID, serverID)
	if err == nil {
		err = h.store.SetOwn(ctx, nodeID, serverID, own)
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, h.thresholdsView(own, serverID))
}

// warnings returns the current warnings about the nodes and servers the user may see.
func (h *Handler) warnings(w http.ResponseWriter, r *http.Request) {
	g := access.From(r.Context())
	httpapi.WriteJSON(w, http.StatusOK, h.store.Warnings(func(nodeID, serverID string) bool {
		if serverID == "" {
			return g.On(access.NodesView, nodeID, "")
		}
		return g.On(access.ServersView, nodeID, serverID)
	}))
}
