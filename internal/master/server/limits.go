package server

import (
	"context"
	"net/http"

	noryxv1 "github.com/QwikByte/mc-server-manager/api/noryx/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

// checkLimits enforces the port range and the memory limit of a node for a server that
// is created (serverID is empty) or changed.
func (h *Handler) checkLimits(ctx context.Context, nodeID, serverID string, port, memoryMB uint32) error {
	n, err := h.nodes.Get(ctx, nodeID)
	if err != nil {
		return err
	}
	if n.PortMin != nil && (port < *n.PortMin || port > *n.PortMax) {
		return httpapi.Errorf(http.StatusBadRequest, "Choose a port from %d to %d, the port range of %s.", *n.PortMin, *n.PortMax, n.Name)
	}
	if n.MemoryReserveMB == nil {
		return nil
	}
	conn, err := h.nodes.Conn(ctx, nodeID)
	if err != nil {
		return err
	}
	info, err := noryxv1.NewNodeServiceClient(conn).GetInfo(ctx, &noryxv1.GetInfoRequest{})
	if err != nil || info.GetMemoryBytes() == 0 { // 0: the runtime is down, creating fails anyway
		return err
	}
	res, err := noryxv1.NewServerServiceClient(conn).ListServers(ctx, &noryxv1.ListServersRequest{})
	if err != nil {
		return err
	}
	limit := int64(info.GetMemoryBytes()>>20) - int64(*n.MemoryReserveMB) //nolint:gosec // memory sizes fit easily
	assigned := int64(memoryMB)
	for _, s := range res.GetServers() {
		if s.GetId() != serverID {
			assigned += int64(s.GetMemoryMb())
		}
	}
	if assigned > limit {
		return httpapi.Errorf(http.StatusConflict, "%s has %d MB of memory left for servers. Choose less memory, or change the memory limit in the node's settings.",
			n.Name, max(0, limit-assigned+int64(memoryMB)))
	}
	return nil
}
