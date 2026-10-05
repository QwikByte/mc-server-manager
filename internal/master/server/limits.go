package server

import (
	"context"
	"net/http"
	"sync"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

// reservations hold the memory of servers that are being created, changed or moved,
// until their node's agent lists them with it, so that concurrent requests can't exceed
// the memory limit of a node together. Memory is counted as the limits of the servers'
// containers, which include what Java needs besides the heap.
type reservations struct {
	mu    sync.Mutex
	nodes map[string]*reserved
}

// reserved is the memory reserved on a node, whose lock is held while checking its limit.
type reserved struct {
	sync.Mutex
	memoryMB int64
}

func (r *reservations) of(nodeID string) *reserved {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.nodes == nil {
		r.nodes = map[string]*reserved{}
	}
	if r.nodes[nodeID] == nil {
		r.nodes[nodeID] = &reserved{}
	}
	return r.nodes[nodeID]
}

var noRelease = func() {}

// checkLimits enforces the port range and the memory limit of a node for a server that
// is created (serverID is empty) or changed, and reserves its memory until release is
// called, once the operation that creates or changes it ended. A change that needs no
// more memory is always allowed, e.g. on a node whose limit was lowered.
func (h *Handler) checkLimits(ctx context.Context, nodeID, serverID string, port, memoryMB uint32) (release func(), err error) {
	n, err := h.nodes.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if n.PortMin != nil && (port < *n.PortMin || port > *n.PortMax) {
		return nil, httpapi.Errorf(http.StatusBadRequest, "Choose a port from %d to %d, the port range of %s.", *n.PortMin, *n.PortMax, n.Name)
	}
	if n.MemoryReserveMB == nil {
		return noRelease, nil
	}
	reserved := h.reserved.of(nodeID)
	reserved.Lock()
	defer reserved.Unlock()
	conn, err := h.nodes.Conn(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	info, err := noryxv1.NewNodeServiceClient(conn).GetInfo(ctx, &noryxv1.GetInfoRequest{})
	if err != nil {
		return nil, err
	}
	if info.GetMemoryBytes() == 0 { // the runtime is down, creating fails anyway
		return noRelease, nil
	}
	res, err := noryxv1.NewServerServiceClient(conn).ListServers(ctx, &noryxv1.ListServersRequest{})
	if err != nil {
		return nil, err
	}
	left := int64(info.GetMemoryBytes()>>20) - int64(*n.MemoryReserveMB) - reserved.memoryMB //nolint:gosec // memory sizes fit easily
	for _, s := range res.GetServers() {
		switch {
		case s.GetId() != serverID:
			left -= noryxv1.ContainerMemoryMB(s.GetMemoryMb())
		case memoryMB <= s.GetMemoryMb():
			return noRelease, nil
		}
	}
	needed := noryxv1.ContainerMemoryMB(memoryMB)
	if needed > left {
		return nil, httpapi.Errorf(http.StatusConflict, "%s has room for a server with up to %d MB of memory, as Java needs about a quarter of it and 256 MB more. Choose less memory, or change the memory limit in the node's settings.",
			n.Name, noryxv1.MaxHeapMB(left))
	}
	reserved.memoryMB += needed
	return func() {
		reserved.Lock()
		defer reserved.Unlock()
		reserved.memoryMB -= needed
	}, nil
}
