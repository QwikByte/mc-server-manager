package network

import (
	"errors"
	"io/fs"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/properties"
)

// Files of Geyser and Floodgate on a proxy, which let Bedrock players join a network.
const (
	// FloodgateKeyFile holds the key with which Geyser vouches for Bedrock players to Floodgate,
	// on every type of proxy. Floodgate creates it when it first starts.
	FloodgateKeyFile = "plugins/floodgate/key.pem"
	// geyserConfigVersion is the version of Geyser's configuration that new files get, so
	// that Geyser takes them as they are.
	geyserConfigVersion = 8
)

// GeyserFolder returns the folder of Geyser on a type of proxy, or "".
func GeyserFolder(typ noryxv1.ServerType) string {
	switch {
	case typ == noryxv1.ServerType_SERVER_TYPE_VELOCITY:
		return "plugins/Geyser-Velocity"
	case typ.Bungee():
		return "plugins/Geyser-BungeeCord"
	}
	return ""
}

// WriteGeyser makes Geyser on a proxy listen for Bedrock players at a UDP port and let
// Floodgate vouch for them. Port 0 leaves its configuration alone. It reports whether the
// file changed.
func WriteGeyser(dir *datadir.Dir, typ noryxv1.ServerType, port uint32) (bool, error) {
	folder := GeyserFolder(typ)
	if port == 0 || folder == "" {
		return false, nil
	}
	file := folder + "/config.yml"
	_, err := dir.Lstat(file)
	fresh := errors.Is(err, fs.ErrNotExist)
	return edit(dir, file, true, func(s map[string]any) error {
		if fresh {
			s["config-version"] = geyserConfigVersion
		}
		child(s, "bedrock")["port"] = port
		// Geyser tells Bedrock players this port; it is the same inside the container.
		child(child(s, "advanced"), "bedrock")["broadcast-port"] = port
		// Never online, which would sign players in with Microsoft accounts on Geyser.
		child(s, "java")["auth-type"] = "floodgate"
		return nil
	})
}

// SecureChat makes a game server demand signed chat messages again, Minecraft's default,
// once Bedrock players no longer join it: the environment of its container turned that off.
func SecureChat(dir *datadir.Dir) error {
	return properties.Write(dir, map[string]string{"enforce-secure-profile": "true"})
}
