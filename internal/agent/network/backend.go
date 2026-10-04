package network

import (
	"errors"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

// Files of game servers that hold the forwarding secret of their network.
const (
	PaperGlobalFile = "config/paper-global.yml"
	FabricProxyFile = "config/FabricProxy-Lite.toml"
	ForgeProxyFile  = "config/proxy-compatible-forge.toml"
)

// ErrModernOnly is returned for Fabric servers with legacy forwarding, as FabricProxy-Lite
// only supports Velocity's modern forwarding.
var ErrModernOnly = errors.New("Fabric servers only support Velocity's modern forwarding.") //nolint:staticcheck // shown to the operator

// WriteBackend writes how a game server accepts the players its proxy forwards, or that it
// accepts players directly again with runtime.ForwardingNone: Paper and Purpur in their own
// configuration, Fabric, Forge and NeoForge in that of the forwarding mod the master
// installs, FabricProxy-Lite or Proxy-Compatible-Forge. It reports whether a file changed.
func WriteBackend(dir *datadir.Dir, typ noryxv1.ServerType, f runtime.Forwarding, secret string) (bool, error) {
	modern, legacy, joined := f == runtime.ForwardingModern, f == runtime.ForwardingLegacy, f != runtime.ForwardingNone
	if !modern {
		secret = ""
	}
	switch typ {
	case noryxv1.ServerType_SERVER_TYPE_PAPER, noryxv1.ServerType_SERVER_TYPE_PURPUR:
		// Paper fills in missing settings when it starts.
		paper, err := edit(dir, PaperGlobalFile, true, func(s map[string]any) error {
			velocity := child(child(s, "proxies"), "velocity")
			velocity["enabled"], velocity["online-mode"], velocity["secret"] = modern, true, secret
			return nil
		})
		if err != nil {
			return false, err
		}
		spigot, err := edit(dir, "spigot.yml", legacy, func(s map[string]any) error {
			child(s, "settings")["bungeecord"] = legacy
			return nil
		})
		return paper || spigot, err
	case noryxv1.ServerType_SERVER_TYPE_FABRIC:
		if legacy {
			return false, ErrModernOnly
		}
		return edit(dir, FabricProxyFile, joined, func(s map[string]any) error {
			s["secret"] = secret
			return nil
		})
	case noryxv1.ServerType_SERVER_TYPE_FORGE, noryxv1.ServerType_SERVER_TYPE_NEOFORGE:
		return edit(dir, ForgeProxyFile, joined, func(s map[string]any) error {
			forwarding := child(s, "forwarding")
			forwarding["enabled"], forwarding["secret"] = joined, secret
			if joined {
				forwarding["mode"] = forwardingModes[f]
			}
			return nil
		})
	}
	return false, runtime.ErrUnsupported
}
