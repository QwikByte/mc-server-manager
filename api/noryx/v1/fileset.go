package noryxv1

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits of file sets, which master and agent both check. They keep a call that applies a
// set below the 4 MiB that gRPC accepts by default.
const (
	MaxFileSetFiles    = 100
	MaxFileSetFileSize = 1 << 20
	MaxFileSetSize     = 3 << 20
	MaxFileSetSecrets  = 50
	MaxSecretValue     = 1 << 10
	maxFileSetPath     = 512
)

// FileSetManifest records in the data of a server which file set wrote which file.
const FileSetManifest = "noryx-filesets.json"

var (
	// Placeholder matches the placeholders in the files of sets: the variables the master
	// fills in, {{server.name}}, {{server.id}}, {{server.port}}, {{network.server}} and those
	// of the set, {{var:<name>}}, the connections to databases, {{datastore:<name>.<database>.<field>}},
	// and the secrets only the agent fills in, {{secret:<name>}} and the passwords of
	// databases. Other text in double braces is left alone, as some plugins use it themselves.
	Placeholder = regexp.MustCompile(`\{\{((?:server|network)\.[^{}\s]*|(?:secret|var|datastore):[^{}\n]*)\}\}`)
	// SecretName matches the names of the secrets of a set.
	SecretName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	// DatastorePlaceholder matches what is between the braces of a placeholder of a database:
	// its datastore, itself and one of its fields. The master fills in all but the password,
	// which only the agent fills in, e.g. datastore:main.luckperms.password.
	DatastorePlaceholder = regexp.MustCompile(`^datastore:([a-z0-9][a-z0-9-]{0,31})\.([a-z][a-z0-9_]{0,31})\.(host|port|database|user|password)$`)
)

// fileSetCode are the starts of files that hold code under any name: ZIP archives, such as
// .jar files, Java classes and Linux programs.
var fileSetCode = []string{"PK\x03\x04", "PK\x05\x06", "PK\x07\x08", "\xca\xfe\xba\xbe", "\x7fELF"}

// fileSetRefused are the files that Noryx writes itself, and those that hold secrets of the
// server, which the agent hides; internal/agent/fileset tests that none is missing.
var fileSetRefused = []string{
	"server.properties", "eula.txt", "velocity.toml", "config.yml", "spigot.yml", "config/paper-global.yml",
	"whitelist.json", "ops.json", "forwarding.secret", ".rcon-cli.env", ".rcon-cli.yaml",
	"config/FabricProxy-Lite.toml", "config/proxy-compatible-forge.toml", "plugins/floodgate/key.pem",
	"plugins/Geyser-Velocity/config.yml", "plugins/Geyser-Velocity/saved-auth-chains.json", "plugins/Geyser-Velocity/saved-refresh-tokens.json",
	"plugins/Geyser-BungeeCord/config.yml", "plugins/Geyser-BungeeCord/saved-auth-chains.json", "plugins/Geyser-BungeeCord/saved-refresh-tokens.json",
}

// CleanFileSetPath returns the clean form of the path of a file of a set, relative to the
// data of a server with forward slashes, e.g. plugins/LuckPerms/config.yml, and why a set
// can't have a file there, or "".
func CleanFileSetPath(p string) (string, string) {
	clean := strings.TrimPrefix(path.Clean("/"+strings.TrimSpace(p)), "/")
	segments := strings.Split(clean, "/")
	switch {
	case clean == "" || strings.HasSuffix(p, "/"):
		return clean, "Enter the path of a file, e.g. plugins/LuckPerms/config.yml."
	case len(p) > maxFileSetPath || strings.ContainsFunc(p, func(r rune) bool { return r == '\\' || unicode.IsControl(r) }) ||
		slices.Contains(strings.Split(p, "/"), "..") || !utf8.ValidString(p):
		return clean, fmt.Sprintf("%q is no valid path.", p)
	case slices.Contains(fileSetRefused, clean) || len(segments) == 1 && (strings.HasPrefix(clean, "noryx-") || strings.HasPrefix(clean, "banned-")) ||
		slices.ContainsFunc(segments, func(s string) bool { return strings.HasPrefix(s, ".noryx-") }):
		return clean, fmt.Sprintf("Noryx manages %s itself, or it holds secrets of the server.", clean)
	case slices.Contains([]string{".jar", ".zip", ".class"}, strings.ToLower(path.Ext(clean))):
		return clean, fmt.Sprintf("%s holds code, which a set can't add. Install plugins on the Plugins page.", clean)
	}
	return clean, ""
}

// FileSetContentProblem returns why text can't be the content of a file of a set, or "".
func FileSetContentProblem(name, content string) string {
	switch {
	case len(content) > MaxFileSetFileSize:
		return fmt.Sprintf("%s is larger than %d MiB.", name, MaxFileSetFileSize>>20)
	case !utf8.ValidString(content) || strings.ContainsRune(content, 0):
		return fmt.Sprintf("%s isn't a text file.", name)
	}
	return ""
}

// FileSetDataProblem returns why data can't be the content of a binary file of a set, e.g.
// an image, or "": archives and programs are refused also under other names, so that a set
// can't add code.
func FileSetDataProblem(name string, data []byte) string {
	switch {
	case len(data) > MaxFileSetFileSize:
		return fmt.Sprintf("%s is larger than %d MiB.", name, MaxFileSetFileSize>>20)
	case slices.ContainsFunc(fileSetCode, func(start string) bool { return bytes.HasPrefix(data, []byte(start)) }):
		return fmt.Sprintf("%s is an archive or a program. Install plugins on the Plugins page.", name)
	}
	return ""
}

// SecretValueProblem returns why text can't be the value of a secret, or "": values are a
// single line of printable characters, so that they can't add lines to a configuration.
func SecretValueProblem(value string) string {
	if value == "" || len(value) > MaxSecretValue || !utf8.ValidString(value) || strings.ContainsFunc(value, func(r rune) bool { return !unicode.IsPrint(r) }) {
		return fmt.Sprintf("A secret is a single line of up to %d printable characters.", MaxSecretValue)
	}
	return ""
}

// Slug returns the short lower-case name, e.g. "created" for FILE_SET_ACTION_CREATED.
func (a FileSetAction) Slug() string { return slug(a.String(), "FILE_SET_ACTION_") }
