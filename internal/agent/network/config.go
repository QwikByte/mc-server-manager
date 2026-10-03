// Package network writes the configuration that joins servers into a network behind a
// Velocity proxy. Players connect to the proxy, which forwards their verified identity
// to the backends using Velocity's modern forwarding and a shared secret; backends
// reject players who do not come through the proxy.
package network

import (
	"bytes"
	"fmt"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

const (
	// velocityConfigVersion is the config version Velocity 4 writes. Velocity migrates
	// older versions on start, but treats a missing version as a legacy config.
	velocityConfigVersion = "2.9"
	// ForwardingSecretFile is the file next to velocity.toml that holds the secret.
	ForwardingSecretFile = "forwarding.secret"
)

// Backend is a server behind the proxy.
type Backend struct {
	Name    string // used by players, e.g. in /server lobby
	Address string // host:port the proxy connects to
}

// velocityKeys are the settings of velocity.toml that belong to the network.
var velocityKeys = []string{"servers", "forced-hosts", "player-info-forwarding-mode", "forwarding-secret-file"}

// VelocityConfig returns velocity.toml with modern forwarding and the given backends,
// keeping every other setting of current, which may be empty. Players join the first
// backend. changed reports whether the network settings differ from current.
func VelocityConfig(current []byte, backends []Backend) (config []byte, changed bool, err error) {
	settings := map[string]any{"config-version": velocityConfigVersion}
	if err := toml.Unmarshal(current, &settings); err != nil {
		return nil, false, fmt.Errorf("read velocity.toml: %w", err)
	}
	before, err := toml.Marshal(pick(settings, velocityKeys))
	if err != nil {
		return nil, false, err
	}
	servers := map[string]any{}
	try := []string{}
	for _, b := range backends {
		servers[b.Name] = b.Address
	}
	if len(backends) > 0 {
		try = append(try, backends[0].Name)
	}
	servers["try"] = try
	settings["servers"] = servers
	settings["forced-hosts"] = map[string]any{} // the defaults point to example servers
	settings["player-info-forwarding-mode"] = "MODERN"
	settings["forwarding-secret-file"] = ForwardingSecretFile
	after, err := toml.Marshal(pick(settings, velocityKeys))
	if err != nil {
		return nil, false, err
	}
	config, err = toml.Marshal(settings)
	return config, !bytes.Equal(before, after), err
}

// VelocityBind returns velocity.toml listening on all addresses at port, keeping every
// other setting of current, which may be empty, and whether that changed anything.
func VelocityBind(current []byte, port int) (config []byte, changed bool, err error) {
	settings := map[string]any{"config-version": velocityConfigVersion}
	if err := toml.Unmarshal(current, &settings); err != nil {
		return nil, false, fmt.Errorf("read velocity.toml: %w", err)
	}
	bind := fmt.Sprintf("0.0.0.0:%d", port)
	if settings["bind"] == bind {
		return current, false, nil
	}
	settings["bind"] = bind
	config, err = toml.Marshal(settings)
	return config, true, err
}

// VelocityBackends returns the addresses of the backends in velocity.toml.
func VelocityBackends(config []byte) []string {
	var settings struct{ Servers map[string]any }
	if toml.Unmarshal(config, &settings) != nil {
		return nil
	}
	var addresses []string
	for name, address := range settings.Servers {
		if s, ok := address.(string); ok && name != "try" {
			addresses = append(addresses, s)
		}
	}
	return addresses
}

// PaperGlobal returns paper-global.yml with Velocity forwarding enabled for secret,
// keeping every other setting of current, which may be empty. An empty secret disables
// forwarding. Paper fills in missing settings when it starts. changed reports whether
// the forwarding settings differ from current.
func PaperGlobal(current []byte, secret string) (config []byte, changed bool, err error) {
	settings := map[string]any{}
	if err := yaml.Unmarshal(current, &settings); err != nil {
		return nil, false, fmt.Errorf("read paper-global.yml: %w", err)
	}
	before, err := yaml.Marshal(settings["proxies"])
	if err != nil {
		return nil, false, err
	}
	velocity := child(child(settings, "proxies"), "velocity")
	velocity["enabled"] = secret != ""
	velocity["online-mode"] = true
	velocity["secret"] = secret
	after, err := yaml.Marshal(settings["proxies"])
	if err != nil {
		return nil, false, err
	}
	config, err = yaml.Marshal(settings)
	return config, !bytes.Equal(before, after), err
}

func pick(m map[string]any, keys []string) map[string]any {
	picked := map[string]any{}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			picked[k] = v
		}
	}
	return picked
}

func child(parent map[string]any, key string) map[string]any {
	m, ok := parent[key].(map[string]any)
	if !ok {
		m = map[string]any{}
		parent[key] = m
	}
	return m
}
