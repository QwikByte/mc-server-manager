package e2e

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/QwikByte/mc-server-manager/internal/enrollment"
	"github.com/QwikByte/mc-server-manager/internal/master/settings"
)

func TestMasterSettings(t *testing.T) {
	m := startMaster(t)
	api := apiClient{t: t, url: m.panel(t).URL}

	var got struct {
		Settings settings.Settings `json:"settings"`
		Master   settings.Master   `json:"master"`
	}
	api.do("GET", "/api/settings", nil, http.StatusOK, &got)
	if s := got.Settings; s.SessionHours != 12 || s.JoinTokenMinutes != 60 || *s.NodeDefaults.MemoryReserveMB != 1024 || s.LogDays != 30 || got.Master.EnrollAddr != m.enrollAddr {
		t.Fatalf("default settings = %+v, master = %+v", s, got.Master)
	}

	valid := func(change func(map[string]any)) map[string]any {
		s := map[string]any{
			"enrollAddr": "panel.example.com:9443", "sessionHours": 24, "joinTokenMinutes": 30,
			"nodeDefaults": map[string]any{"portMin": 25565, "portMax": 25600, "memoryReserveMb": 2048}, "logDays": 90,
		}
		change(s)
		return s
	}
	for name, change := range map[string]func(map[string]any){
		"address without port": func(s map[string]any) { s["enrollAddr"] = "panel.example.com" },
		"address with a path":  func(s map[string]any) { s["enrollAddr"] = "panel.example.com/x:9443" },
		"panel at a host name": func(s map[string]any) { s["panelAddr"] = "panel.example.com:8080" },
		"endless sessions":     func(s map[string]any) { s["sessionHours"] = 24 * 365 },
		"short join tokens":    func(s map[string]any) { s["joinTokenMinutes"] = 1 },
		"reversed port range":  func(s map[string]any) { s["nodeDefaults"] = map[string]any{"portMin": 30000, "portMax": 20000} },
		"logs kept forever":    func(s map[string]any) { s["logDays"] = 0 },
	} {
		if body := api.do("PUT", "/api/settings", valid(change), http.StatusBadRequest, nil); body == "" {
			t.Errorf("%s: no error message", name)
		}
	}
	api.do("PUT", "/api/settings", valid(func(map[string]any) {}), http.StatusOK, &got)

	// New nodes get the default limits and join tokens with the new address and validity.
	var created struct {
		Node struct {
			PortMin         *int `json:"portMin"`
			MemoryReserveMB *int `json:"memoryReserveMb"`
		} `json:"node"`
		JoinToken          string    `json:"joinToken"`
		JoinTokenExpiresAt time.Time `json:"joinTokenExpiresAt"`
		InstallCommand     string    `json:"installCommand"`
	}
	api.do("POST", "/api/nodes", map[string]string{"name": "node-1", "address": "127.0.0.1:7443"}, http.StatusCreated, &created)
	token, err := enrollment.ParseToken(created.JoinToken)
	check(t, err)
	if token.Master != "panel.example.com:9443" || *created.Node.PortMin != 25565 || *created.Node.MemoryReserveMB != 2048 {
		t.Fatalf("created node = %+v, token for %s", created.Node, token.Master)
	}
	if left := time.Until(created.JoinTokenExpiresAt); left < 29*time.Minute || left > 30*time.Minute {
		t.Fatalf("join token expires in %v, want 30 minutes", left)
	}
	if !strings.HasSuffix(created.InstallCommand, "/install.sh && sudo bash install.sh agent --join "+created.JoinToken) {
		t.Fatalf("install command = %q", created.InstallCommand)
	}

	// The settings are kept, and an empty address means the one from the command line.
	reloaded, err := settings.Load(t.Context(), m.db, settings.Master{EnrollAddr: m.enrollAddr}, m.cert)
	check(t, err)
	if !reflect.DeepEqual(reloaded.Get(), got.Settings) {
		t.Fatalf("reloaded settings = %+v, want %+v", reloaded.Get(), got.Settings)
	}
	api.do("PUT", "/api/settings", valid(func(s map[string]any) { s["enrollAddr"] = " " }), http.StatusOK, nil)
	if addr := m.settings.EnrollAddr(); addr != m.enrollAddr {
		t.Fatalf("enrollment address = %s, want %s", addr, m.enrollAddr)
	}
}
