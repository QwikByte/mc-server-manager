package fileset

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/datastore"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/tag"
)

// serverVariables are the variables of each server, which the master fills in.
var serverVariables = []string{"server.name", "server.id", "server.port", "network.server"}

// placeholderProblem returns why a file has a placeholder that isn't known, or "".
func placeholderProblem(name, content string) string {
	for _, m := range noryxv1.Placeholder.FindAllStringSubmatch(content, -1) {
		kind, rest, _ := strings.Cut(m[1], ":")
		switch {
		case kind == "secret" && !noryxv1.SecretName.MatchString(rest):
			return fmt.Sprintf("%s: %s names no valid secret. Secret names have up to 64 lower-case letters, digits, - and _.", name, m[0])
		case kind == "var" && !noryxv1.SecretName.MatchString(rest):
			return fmt.Sprintf("%s: %s names no valid variable. Variable names have up to 64 lower-case letters, digits, - and _.", name, m[0])
		case kind == "datastore" && !noryxv1.DatastorePlaceholder.MatchString(m[1]):
			return fmt.Sprintf("%s: %s names no database. Use {{datastore:<datastore>.<database>.<field>}} with host, port, database, user or password as field.", name, m[0])
		case !slices.Contains([]string{"secret", "var", "datastore"}, kind) && !slices.Contains(serverVariables, m[1]):
			return fmt.Sprintf("%s: %s is unknown. Use {{server.name}}, {{server.id}}, {{server.port}}, {{network.server}}, {{var:<name>}}, {{datastore:<datastore>.<database>.<field>}} or {{secret:<name>}}.", name, m[0])
		}
	}
	return ""
}

// passwords returns the placeholders of passwords of databases in the files that before
// doesn't have as they are, sorted.
func passwords(files, before []File) []string {
	var found []string
	for _, f := range files {
		same := func(b File) bool {
			return b.Path == f.Path && b.Content == f.Content && b.OnlyIfMissing == f.OnlyIfMissing
		}
		if slices.ContainsFunc(before, same) {
			continue
		}
		for _, m := range noryxv1.Placeholder.FindAllStringSubmatch(f.Content, -1) {
			if d := noryxv1.DatastorePlaceholder.FindStringSubmatch(m[1]); d != nil && d[3] == "password" {
				found = append(found, m[0])
			}
		}
	}
	slices.Sort(found)
	return slices.Compact(found)
}

// member is a server that a set may be for, with what its placeholders need.
type member struct {
	tag.Server
	Name string
	Port uint32
	// Network is the server's name in its network, empty for none or the proxy.
	Network string
	// NetworkID is the network the server is part of, also as its proxy.
	NetworkID string
	Tags      []string
	// connect returns how the server reaches a database of a datastore of its network.
	connect func(datastore, database string) (datastore.Connection, error)
}

// rendered is a set for one server: its files with the variables filled in, the values of
// the secrets and passwords they use, and the revision that tells all apart from other states.
type rendered struct {
	files    []*noryxv1.FileSetFile
	secrets  map[string]string
	revision string
}

// render fills in the variables of a set and the connections to databases in files of the
// set for a server, and picks the secrets and passwords they use. Filled in, the files must
// keep to the limits of a set, as the agent checks them.
func render(set Set, files []File, values map[string]string, m member) (rendered, error) {
	r := rendered{secrets: map[string]string{}}
	h := sha256.New()
	fmt.Fprintf(h, "%s\n", set.ID)
	var missing error
	size := 0
	for _, f := range files {
		file := &noryxv1.FileSetFile{Path: f.Path, Data: f.Data, OnlyIfMissing: f.OnlyIfMissing}
		if f.Data == nil {
			file.Content = noryxv1.Placeholder.ReplaceAllStringFunc(f.Content, func(p string) string {
				value, err := r.fill(set, m, values, p)
				if err != nil {
					missing = cmp.Or(missing, httpapi.Errorf(http.StatusConflict, "%s uses %s: %s", f.Path, p, httpapi.Message(err)))
				}
				return value
			})
		}
		content := cmp.Or(file.Content, string(f.Data))
		if size += len(content); len(content) > noryxv1.MaxFileSetFileSize || size > noryxv1.MaxFileSetSize {
			missing = cmp.Or(missing, httpapi.Errorf(http.StatusConflict, "With its placeholders filled in, %s is larger than a file set allows: %d MiB a file, %d MiB in all.",
				f.Path, noryxv1.MaxFileSetFileSize>>20, noryxv1.MaxFileSetSize>>20))
		}
		r.files = append(r.files, file)
		fmt.Fprintf(h, "%q %q %t\n", f.Path, content, f.OnlyIfMissing)
	}
	for _, key := range slices.Sorted(maps.Keys(r.secrets)) {
		fmt.Fprintf(h, "%q %q\n", key, r.secrets[key])
	}
	r.revision = hex.EncodeToString(h.Sum(nil))
	return r, missing
}

// fill returns what a placeholder stands for on a server, or the placeholder itself for
// secrets and passwords, which the agent fills in and r keeps, also those without value.
func (r rendered) fill(set Set, m member, values map[string]string, p string) (string, error) {
	key := p[2 : len(p)-2]
	kind, name, _ := strings.Cut(key, ":")
	switch kind {
	case "secret":
		value, ok := values[name]
		if r.secrets[key] = value; !ok {
			return p, httpapi.Errorf(http.StatusConflict, "The secret %s has no value. Set it in the file set.", name)
		}
		return p, nil
	case "var":
		i := slices.IndexFunc(set.Variables, func(v Variable) bool { return v.Name == name })
		if i < 0 {
			return p, httpapi.Errorf(http.StatusConflict, "The file set has no variable %s.", name)
		}
		value, ok := set.Variables[i].value(m)
		if !ok {
			return p, httpapi.Errorf(http.StatusConflict, "The variable %s has no value for %s. Give it one for the server, or for all servers.", name, m.Name)
		}
		return value, nil
	case "datastore":
		d := noryxv1.DatastorePlaceholder.FindStringSubmatch(key)
		if m.NetworkID == "" {
			return p, httpapi.Errorf(http.StatusConflict, "%s is in no network, whose databases it could use.", m.Name)
		}
		c, err := m.connect(d[1], d[2])
		switch {
		case err != nil:
			return p, err
		case d[3] == "host":
			return c.Host, nil
		case d[3] == "port":
			return strconv.FormatUint(uint64(c.Port), 10), nil
		case d[3] == "password":
			r.secrets[key] = c.Password
			return p, nil
		}
		return c.Database, nil // also the name of its user
	}
	switch key {
	case "server.name":
		return m.Name, nil
	case "server.id":
		return m.ServerID, nil
	case "server.port":
		return strconv.FormatUint(uint64(m.Port), 10), nil
	}
	if m.Network == "" {
		return p, httpapi.Errorf(http.StatusConflict, "%s is no game server of a network.", m.Name)
	}
	return m.Network, nil
}

// passwords reports whether the files hold passwords of databases.
func (r rendered) passwords() bool {
	for key := range r.secrets {
		if strings.HasPrefix(key, "datastore:") {
			return true
		}
	}
	return false
}

// file returns a file of a set as it is for a server, or nil: text as the panel shows it, with
// the variables filled in and the secrets as placeholders, or binary.
func (r rendered) file(path string) *noryxv1.FileSetFile {
	i := slices.IndexFunc(r.files, func(f *noryxv1.FileSetFile) bool { return f.GetPath() == path })
	if i < 0 {
		return nil
	}
	return r.files[i]
}
