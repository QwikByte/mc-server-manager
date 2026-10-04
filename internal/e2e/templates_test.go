package e2e

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/template"
)

func TestTemplates(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	api := apiClient{t: t, url: m.panel(t).URL}
	input := func(change func(map[string]any)) map[string]any {
		in := map[string]any{
			"name": "Survival", "description": "Paper with permissions", "type": "paper", "version": "", "memoryMb": 4096,
			"java": "21", "restartPolicy": "on_crash", "aikarFlags": true, "jvmOptions": []string{"-Dfile.encoding=UTF-8"}, "cpuLimit": 2,
			"properties": map[string]string{"difficulty": "hard", "motd": "§aWelcome"}, "plugins": []string{"luckperms"},
		}
		change(in)
		return in
	}

	var tmpl template.Template
	api.do("POST", "/api/templates", input(func(map[string]any) {}), http.StatusCreated, &tmpl)
	if tmpl.Version != "LATEST" || len(tmpl.Plugins) != 1 || tmpl.Plugins[0].Title != "LuckPerms" || tmpl.Plugins[0].Icon != "/api/plugins/icons/luckperms/icon.png" {
		t.Fatalf("template = %+v", tmpl)
	}
	api.do("GET", "/api/templates/"+tmpl.ID, nil, http.StatusOK, &tmpl)
	if tmpl.Properties["difficulty"] != "hard" || tmpl.RestartPolicy != "on_crash" || tmpl.CPULimit != 2 {
		t.Fatalf("stored template = %+v", tmpl)
	}

	// Plugins must exist and run on the template's software.
	api.do("POST", "/api/templates", input(func(in map[string]any) {}), http.StatusConflict, nil)
	api.do("POST", "/api/templates", input(func(in map[string]any) { in["name"], in["plugins"] = "Mods", []string{"fabricapi"} }), http.StatusBadRequest, nil)
	api.do("POST", "/api/templates", input(func(in map[string]any) { in["name"], in["plugins"] = "Missing", []string{"missing"} }), http.StatusBadRequest, nil)
	api.do("POST", "/api/templates", input(func(in map[string]any) { in["name"], in["type"] = "Proxy", "velocity" }), http.StatusBadRequest, nil)
	api.do("POST", "/api/templates", input(func(in map[string]any) { in["name"], in["properties"] = "Bad", map[string]string{"a b": "c"} }), http.StatusBadRequest, nil)
	api.do("PUT", "/api/templates/"+tmpl.ID, input(func(in map[string]any) { in["name"], in["memoryMb"] = "Survival 2", 2048 }), http.StatusOK, &tmpl)
	if tmpl.Name != "Survival 2" || tmpl.MemoryMB != 2048 {
		t.Fatalf("updated template = %+v", tmpl)
	}

	// A server created with a template's settings gets its server.properties before it starts.
	path := "/api/nodes/" + a.node.ID + "/servers"
	server := map[string]any{
		"name": "Survival", "type": tmpl.Type, "version": tmpl.Version, "memoryMb": tmpl.MemoryMB, "port": 25565, "acceptEula": true,
		"java": tmpl.Java, "restartPolicy": tmpl.RestartPolicy, "aikarFlags": tmpl.AikarFlags, "jvmOptions": tmpl.JVMOptions,
		"cpuLimit": tmpl.CPULimit, "properties": tmpl.Properties,
	}
	var created struct{ ID string }
	api.do("POST", path, server, http.StatusCreated, &created)
	spec := a.runtime.spec(created.ID)
	if spec.Java != "21" || spec.RestartPolicy != noryxv1.RestartPolicy_RESTART_POLICY_ON_CRASH || !spec.AikarFlags || spec.CPUMillis != 2000 {
		t.Fatalf("spec = %+v", spec)
	}
	props, err := os.ReadFile(filepath.Join(a.runtime.dir, created.ID, "server.properties"))
	check(t, err)
	if string(props) != "difficulty=hard\nmotd=\\u00A7aWelcome\n" {
		t.Fatalf("server.properties = %q", props)
	}

	// Properties the manager sets itself are refused before anything is created.
	server["port"], server["properties"] = 25566, map[string]string{"server-port": "1"}
	api.do("POST", path, server, http.StatusBadRequest, nil)
	server["type"], server["properties"] = "velocity", map[string]string{"motd": "x"}
	api.do("POST", path, server, http.StatusBadRequest, nil)
	if n := len(a.runtime.servers); n != 1 {
		t.Fatalf("%d servers, want 1", n)
	}

	api.do("DELETE", "/api/templates/"+tmpl.ID, nil, http.StatusNoContent, nil)
	api.do("GET", "/api/templates/"+tmpl.ID, nil, http.StatusNotFound, nil)
	if body := api.do("GET", "/api/templates", nil, http.StatusOK, nil); body != "[]\n" {
		t.Fatalf("templates after delete = %s", body)
	}
}
