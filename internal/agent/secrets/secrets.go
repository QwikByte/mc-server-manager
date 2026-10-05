// Package secrets keeps the secrets in the data of servers from the users of the panel: the
// RCON password, with which clients run console commands, and the forwarding secret of a
// network, with which they can sign in to every server of the network as any player. The
// file manager hides files that only hold secrets and shows the others with the secrets
// replaced, and saving such a file keeps them. Downloads of folders and backups do the same.
package secrets

import (
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/network"
	"github.com/QwikByte/noryx/internal/agent/properties"
)

// Placeholder replaces the secrets in the files the panel shows.
const Placeholder = "<hidden>"

// hidden are the files that only hold secrets: those of rcon-cli, which the server image
// writes, the forwarding secret of a proxy, and those of Geyser and Floodgate.
var hidden = append([]string{".rcon-cli.env", ".rcon-cli.yaml", network.ForwardingSecretFile}, network.BedrockSecretFiles...)

var (
	geyserVelocity = network.GeyserFolder(noryxv1.ServerType_SERVER_TYPE_VELOCITY)
	geyserBungee   = network.GeyserFolder(noryxv1.ServerType_SERVER_TYPE_BUNGEECORD)
	// geyserSecrets are the keys of Geyser's configuration for external signaling and HTTPS.
	geyserSecrets = []string{"token", "password", "private-key"}
)

// redacted are the files with secrets among other settings, with the pattern of a line with
// a secret: the key with its separator, the key, and the value.
var redacted = map[string]*regexp.Regexp{
	"server.properties": lines(slices.Collect(maps.Keys(properties.Secret))),
	// The forwarding secret of a network on game servers: in Paper's configuration or in
	// that of the forwarding mod of Fabric, Quilt, Forge or NeoForge.
	network.PaperGlobalFile:        lines([]string{"secret"}),
	network.FabricProxyFile:        lines([]string{"secret"}),
	network.ForgeProxyFile:         lines([]string{"secret"}),
	geyserVelocity + "/config.yml": lines(slices.Clone(geyserSecrets)),
	geyserBungee + "/config.yml":   lines(slices.Clone(geyserSecrets)),
}

func lines(keys []string) *regexp.Regexp {
	for i, k := range keys {
		keys[i] = regexp.QuoteMeta(k)
	}
	return regexp.MustCompile(`(?m)^([ \t]*(` + strings.Join(keys, "|") + `)[ \t]*[=:][ \t]*)([^\r\n]*[^\s])`)
}

// Hidden reports whether name, a clean path in the data directory of a server, is a file
// that only holds secrets.
func Hidden(name string) bool { return slices.Contains(hidden, name) }

// Redacted reports whether name is a file with secrets that Redact replaces.
func Redacted(name string) bool { return redacted[name] != nil }

// Under returns the files that may hold secrets at name or in the folder name.
func Under(name string) []string {
	return slices.DeleteFunc(slices.Concat(hidden, slices.Collect(maps.Keys(redacted))), func(p string) bool {
		return name != "." && name != p && !strings.HasPrefix(p, name+"/")
	})
}

// Redact returns data, the content of the file name, with its secrets replaced by Placeholder.
func Redact(name string, data []byte) []byte {
	if re := redacted[name]; re != nil {
		return re.ReplaceAll(data, []byte("${1}"+Placeholder))
	}
	return data
}

// Restore returns data, which replaces the file name whose content is current, with each
// Placeholder replaced by the secret of the same key in current, or by none.
func Restore(name string, data, current []byte) []byte {
	re := redacted[name]
	if re == nil {
		return data
	}
	secrets := map[string][]byte{}
	for _, m := range re.FindAllSubmatch(current, -1) {
		secrets[string(m[2])] = m[3]
	}
	return re.ReplaceAllFunc(data, func(line []byte) []byte {
		m := re.FindSubmatch(line)
		if string(m[3]) != Placeholder {
			return line
		}
		return slices.Concat(m[1], secrets[string(m[2])])
	})
}

// Censor returns the censor of an archive of the folder dir of a server's data, e.g. "."
// for all of it.
func Censor(dir string) datadir.Censor {
	return func(name string) (bool, func([]byte) []byte) {
		name = path.Join(dir, name)
		if !Redacted(name) {
			return Hidden(name), nil
		}
		return false, func(data []byte) []byte { return Redact(name, data) }
	}
}
