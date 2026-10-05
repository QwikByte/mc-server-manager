package network

import (
	"errors"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/properties"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

// Files of game servers that hold the forwarding secret of their network.
const (
	PaperGlobalFile = "config/paper-global.yml"
	FabricProxyFile = "config/FabricProxy-Lite.toml"
	ForgeProxyFile  = "config/proxy-compatible-forge.toml"
)

// ErrModernOnly is returned for Fabric and Quilt servers with legacy forwarding, as
// FabricProxy-Lite only supports Velocity's modern forwarding.
var ErrModernOnly = errors.New("Fabric and Quilt servers only support Velocity's modern forwarding.") //nolint:staticcheck // shown to the operator

// WriteBackend writes how a game server accepts the players its proxy forwards, or that it
// accepts players directly again with runtime.ForwardingNone: Paper and its forks in their
// own configuration, Fabric, Quilt, Forge and NeoForge in that of the forwarding mod the
// master installs, FabricProxy-Lite or Proxy-Compatible-Forge. It reports whether a file changed.
func WriteBackend(dir *datadir.Dir, typ noryxv1.ServerType, f runtime.Forwarding, secret string) (bool, error) {
	modern, legacy, joined := f == runtime.ForwardingModern, f == runtime.ForwardingLegacy, f != runtime.ForwardingNone
	if !modern {
		secret = ""
	}
	switch {
	case typ.Paper():
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
	case typ.Fabric():
		if legacy {
			return false, ErrModernOnly
		}
		return edit(dir, FabricProxyFile, joined, func(s map[string]any) error {
			s["secret"] = secret
			return nil
		})
	case typ == noryxv1.ServerType_SERVER_TYPE_FORGE, typ == noryxv1.ServerType_SERVER_TYPE_NEOFORGE:
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

// Leave gives a game server back Minecraft's defaults that the environment of its
// container turned off while it was in a network: online mode once it left its proxy,
// which authenticated the players, and signed chat once Bedrock players, who can't sign
// their messages, no longer join it. They are written once, so the operator may turn them
// off again.
func Leave(dir *datadir.Dir, proxy, bedrock bool) error {
	changes := map[string]string{}
	if proxy {
		changes["online-mode"] = "true"
	}
	if bedrock {
		changes["enforce-secure-profile"] = "true"
	}
	if len(changes) == 0 {
		return nil
	}
	return properties.Write(dir, changes)
}
