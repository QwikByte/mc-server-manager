package e2e

import (
	"net/http"
	"testing"
)

func TestNodeSettings(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	m.startAgent(t, "node-2")
	api := apiClient{t: t, url: m.panel(t).URL}
	path := "/api/nodes/" + a.node.ID
	settings := func(change func(map[string]any)) map[string]any {
		s := map[string]any{"name": "Frankfurt 1", "address": a.node.Address, "defaultStorage": "default", "portMin": 25565, "portMax": 25570, "memoryReserveMb": 4096}
		change(s)
		return s
	}

	var got struct {
		Name            string `json:"name"`
		PortMin         *int   `json:"portMin"`
		MemoryReserveMB *int   `json:"memoryReserveMb"`
		Status          string `json:"status"`
	}
	api.do("PUT", path, settings(func(map[string]any) {}), http.StatusOK, &got)
	if got.Name != "Frankfurt 1" || got.PortMin == nil || *got.PortMin != 25565 || *got.MemoryReserveMB != 4096 || got.Status != "online" {
		t.Fatalf("updated node = %+v", got)
	}
	api.do("PUT", path, settings(func(s map[string]any) { s["name"] = "node-2" }), http.StatusConflict, nil)
	api.do("PUT", path, settings(func(s map[string]any) { s["portMin"] = 30000 }), http.StatusBadRequest, nil)
	api.do("PUT", path, settings(func(s map[string]any) { s["portMax"] = nil }), http.StatusBadRequest, nil)

	// The node has 8 GB, of which 4 GB are kept free, and allows ports 25565 to 25570.
	create := func(name string, memory, port, status int) string {
		var srv struct{ ID string }
		api.do("POST", path+"/servers", map[string]any{"name": name, "type": "paper", "memoryMb": memory, "port": port, "acceptEula": true}, status, &srv)
		return srv.ID
	}
	create("Outside", 1024, 25600, http.StatusBadRequest)
	lobby := create("Lobby", 2048, 25565, http.StatusCreated)
	create("Big", 4096, 25566, http.StatusConflict)
	create("Survival", 2048, 25566, http.StatusCreated)
	update := map[string]any{"name": "Lobby", "version": "LATEST", "memoryMb": 3072, "port": 25565, "restartPolicy": "always"}
	api.do("PUT", path+"/servers/"+lobby, update, http.StatusConflict, nil)

	// Without a reserve, memory isn't limited.
	api.do("PUT", path, settings(func(s map[string]any) { s["memoryReserveMb"] = nil }), http.StatusOK, nil)
	api.do("PUT", path+"/servers/"+lobby, update, http.StatusOK, nil)
}
