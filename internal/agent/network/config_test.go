package network

import (
	"reflect"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

// Excerpt of the default configuration of Velocity 4.2.
const defaultVelocity = `
config-version = "2.9"
bind = "0.0.0.0:25565"
motd = "<#09add3>My Network"
player-info-forwarding-mode = "NONE"
forwarding-secret-file = "forwarding.secret"

[servers]
lobby = "127.0.0.1:30066"
factions = "127.0.0.1:30067"
try = [
    "lobby"
]

[forced-hosts]
"lobby.example.com" = [
    "lobby"
]

[advanced]
compression-threshold = 256
`

func TestVelocityConfig(t *testing.T) {
	backends := []Backend{{"lobby", "mcsm-a:25565"}, {"survival", "203.0.113.7:25566"}}
	tests := []struct {
		name    string
		current string
		want    map[string]any // expected values of selected keys
	}{
		{"new proxy", "", map[string]any{"config-version": "2.9"}},
		{"keeps other settings", defaultVelocity, map[string]any{"motd": "<#09add3>My Network", "bind": "0.0.0.0:25565"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, changed, err := VelocityConfig([]byte(tt.current), backends)
			if err != nil || !changed {
				t.Fatalf("changed = %v, err = %v", changed, err)
			}
			// Applying the same network again changes nothing, so nothing restarts.
			if _, again, err := VelocityConfig(out, backends); err != nil || again {
				t.Fatalf("second run: changed = %v, err = %v", again, err)
			}
			var got map[string]any
			if err := toml.Unmarshal(out, &got); err != nil {
				t.Fatalf("invalid TOML: %v\n%s", err, out)
			}
			tt.want["player-info-forwarding-mode"] = "MODERN"
			tt.want["forwarding-secret-file"] = "forwarding.secret"
			tt.want["forced-hosts"] = map[string]any{}
			tt.want["servers"] = map[string]any{"lobby": "mcsm-a:25565", "survival": "203.0.113.7:25566", "try": []any{"lobby"}}
			for key, want := range tt.want {
				if !reflect.DeepEqual(got[key], want) {
					t.Errorf("%s = %#v, want %#v", key, got[key], want)
				}
			}
		})
	}
}

func TestVelocityConfigWithoutBackends(t *testing.T) {
	out, _, err := VelocityConfig([]byte(defaultVelocity), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ Servers map[string]any }
	if err := toml.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if want := map[string]any{"try": []any{}}; !reflect.DeepEqual(got.Servers, want) {
		t.Fatalf("servers = %#v, want %#v", got.Servers, want)
	}
}

func TestPaperGlobal(t *testing.T) {
	existing := "chunk-system:\n  io-threads: 4\nproxies:\n  velocity:\n    enabled: true\n    secret: old\n"
	tests := []struct {
		name, current, secret string
		wantEnabled           bool
	}{
		{"join a network", "", "s3cret", true},
		{"leave a network", existing, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, changed, err := PaperGlobal([]byte(tt.current), tt.secret)
			if err != nil || !changed {
				t.Fatalf("changed = %v, err = %v", changed, err)
			}
			if _, again, err := PaperGlobal(out, tt.secret); err != nil || again {
				t.Fatalf("second run: changed = %v, err = %v", again, err)
			}
			var got struct {
				ChunkSystem map[string]any `yaml:"chunk-system"`
				Proxies     struct {
					Velocity struct {
						Enabled    bool   `yaml:"enabled"`
						OnlineMode bool   `yaml:"online-mode"`
						Secret     string `yaml:"secret"`
					} `yaml:"velocity"`
				} `yaml:"proxies"`
			}
			if err := yaml.Unmarshal(out, &got); err != nil {
				t.Fatal(err)
			}
			v := got.Proxies.Velocity
			if v.Enabled != tt.wantEnabled || !v.OnlineMode || v.Secret != tt.secret {
				t.Fatalf("velocity settings = %+v", v)
			}
			if tt.current != "" && got.ChunkSystem["io-threads"] != 4 {
				t.Fatal("other settings were lost")
			}
		})
	}
}
