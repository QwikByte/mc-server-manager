package e2e

import (
	"net/http"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

func TestUsage(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	api := apiClient{t: t, url: m.panel(t).URL}
	api.do("POST", "/api/nodes/"+lobby.NodeID+"/servers/"+lobby.ServerID+"/start", nil, http.StatusNoContent, nil)

	var latest struct {
		Node *struct {
			CPUCount         uint32 `json:"cpuCount"`
			MemoryTotalBytes uint64 `json:"memoryTotalBytes"`
		} `json:"node"`
		Servers []struct {
			ID          string `json:"id"`
			Running     bool   `json:"running"`
			MemoryBytes uint64 `json:"memoryBytes"`
		} `json:"servers"`
	}
	api.do("GET", "/api/nodes/"+lobby.NodeID+"/usage", nil, http.StatusOK, &latest)
	if latest.Node == nil || latest.Node.MemoryTotalBytes == 0 || len(latest.Servers) != 1 ||
		latest.Servers[0].ID != lobby.ServerID || !latest.Servers[0].Running || latest.Servers[0].MemoryBytes != 512<<20 {
		t.Fatalf("latest = %+v", latest)
	}
	api.do("GET", "/api/nodes/"+lobby.NodeID+"/servers/"+lobby.ServerID+"/usage/history?range=week", nil, http.StatusOK, nil)
	api.do("GET", "/api/nodes/"+lobby.NodeID+"/usage/history?range=year", nil, http.StatusBadRequest, nil)
}
