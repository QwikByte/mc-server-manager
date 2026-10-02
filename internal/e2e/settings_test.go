package e2e

import (
	"net/http"
	"testing"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
)

func TestUpdateServer(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	lobby := m.createServer(t, a, "Lobby", mcsmv1.ServerType_SERVER_TYPE_PAPER, 25565)
	m.createServer(t, a, "Survival", mcsmv1.ServerType_SERVER_TYPE_PAPER, 25566)
	api := apiClient{t: t, url: m.panel(t).URL}
	path := "/api/nodes/" + lobby.NodeID + "/servers/" + lobby.ServerID
	settings := func(change func(map[string]any)) map[string]any {
		s := map[string]any{
			"name": "Hub", "version": "1.21.4", "memoryMb": 4096, "port": 25570, "java": "21",
			"restartPolicy": "on_crash", "aikarFlags": true, "jvmOptions": []string{"-Dfile.encoding=UTF-8"}, "cpuLimit": 1.5,
		}
		change(s)
		return s
	}

	var got struct {
		Name          string   `json:"name"`
		Port          uint32   `json:"port"`
		Java          string   `json:"java"`
		RestartPolicy string   `json:"restartPolicy"`
		JVMOptions    []string `json:"jvmOptions"`
		CPULimit      float64  `json:"cpuLimit"`
	}
	api.do("PUT", path, settings(func(map[string]any) {}), http.StatusOK, &got)
	if got.Name != "Hub" || got.Port != 25570 || got.Java != "21" || got.RestartPolicy != "on_crash" || got.CPULimit != 1.5 || len(got.JVMOptions) != 1 {
		t.Fatalf("updated server = %+v", got)
	}
	spec := a.runtime.servers[0].Spec
	if spec.CPUMillis != 1500 || !spec.AikarFlags || spec.MemoryMB != 4096 {
		t.Fatalf("runtime spec = %+v", spec)
	}

	api.do("PUT", path, settings(func(s map[string]any) { s["port"] = 25566 }), http.StatusConflict, nil)
	api.do("PUT", path, settings(func(s map[string]any) { s["jvmOptions"] = []string{"-Dx=$(id)"} }), http.StatusBadRequest, nil)
	api.do("PUT", path, settings(func(s map[string]any) { s["restartPolicy"] = "sometimes" }), http.StatusBadRequest, nil)
	api.do("PUT", path, settings(func(s map[string]any) { s["cpuLimit"] = -1 }), http.StatusBadRequest, nil)
}
