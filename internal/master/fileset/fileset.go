// Package fileset keeps file sets: named, versioned collections of text files, such as the
// configuration of a plugin, that the master puts on the servers of tags and networks. It
// fills in the variables of each server; the agents fill in the secrets of a set, which the
// API never returns, and hide the files that hold them.
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
	"time"
	"unicode"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/tag"
)

const (
	maxDescription = 500
	maxTargets     = 50
	// keepVersions is how many versions of a set are kept.
	keepVersions = 20

	KindTag     = "tag"
	KindNetwork = "network"
	// Roles of the servers of a network that a set is for.
	RoleServers = "servers"
	RoleProxy   = "proxy"
)

// File is a file of a set, at a path relative to the data of a server.
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	// OnlyIfMissing writes the file only to servers that don't have it, for files that
	// plugins rewrite.
	OnlyIfMissing bool `json:"onlyIfMissing"`
}

// Target names servers that a set is for: those with a tag, or the game servers or the
// proxy of a network.
type Target struct {
	Kind  string `json:"kind"`
	Value string `json:"value"` // the tag, or the ID of the network
	Role  string `json:"role"`  // for networks
}

// Secret is a secret of a set; its value never leaves the master but to the agents.
type Secret struct {
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Version is a saved state of the files of a set.
type Version struct {
	Version   int64     `json:"version"`
	User      string    `json:"user"`
	CreatedAt time.Time `json:"createdAt"`
	Files     []File    `json:"files,omitempty"`
}

type Set struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     int64    `json:"version"`
	Files       []File   `json:"files"`
	Targets     []Target `json:"targets"`
	Secrets     []Secret `json:"secrets"`
	// Versions are the kept versions, newest first, without their files.
	Versions  []Version `json:"versions"`
	CreatedAt time.Time `json:"createdAt"`
}

// Summary describes a set in the list of sets.
type Summary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     int64     `json:"version"`
	Paths       []string  `json:"paths"`
	Targets     []Target  `json:"targets"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Input is a new set, or a change of one.
type Input struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Files       []File   `json:"files"`
	Targets     []Target `json:"targets"`
	// Version is the version a change is based on; if the set has a newer one, the change
	// is refused, so that it doesn't undo what someone else saved.
	Version int64 `json:"version"`
}

// check validates a set and puts it into its canonical form. Targets of networks that
// don't exist, e.g. as they were deleted, are left out.
func (in *Input) check(exists func(network string) bool) error {
	in.Name, in.Description = strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)
	switch {
	case in.Name == "" || len(in.Name) > 64 || strings.ContainsFunc(in.Name, unicode.IsControl):
		return httpapi.Errorf(http.StatusBadRequest, "Enter a name with up to 64 characters.")
	case len(in.Description) > maxDescription:
		return httpapi.Errorf(http.StatusBadRequest, "Keep the description below %d characters.", maxDescription)
	case len(in.Files) > noryxv1.MaxFileSetFiles:
		return httpapi.Errorf(http.StatusBadRequest, "A file set has up to %d files.", noryxv1.MaxFileSetFiles)
	case len(in.Targets) > maxTargets:
		return httpapi.Errorf(http.StatusBadRequest, "A file set has up to %d targets.", maxTargets)
	}
	size := 0
	for i, f := range in.Files {
		p, problem := noryxv1.CleanFileSetPath(f.Path)
		if size += len(f.Content); problem == "" && size > noryxv1.MaxFileSetSize {
			problem = fmt.Sprintf("A file set has up to %d MiB.", noryxv1.MaxFileSetSize>>20)
		}
		problem = cmp.Or(problem, noryxv1.FileSetContentProblem(p, f.Content), placeholderProblem(p, f.Content))
		if problem == "" && slices.ContainsFunc(in.Files[:i], func(o File) bool { return o.Path == p }) {
			problem = p + " is there twice."
		}
		if problem != "" {
			return httpapi.Errorf(http.StatusBadRequest, "%s", problem)
		}
		in.Files[i].Path = p
	}
	slices.SortFunc(in.Files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	for i, t := range in.Targets {
		switch t.Kind {
		case KindTag:
			tags, err := tag.Normalize([]string{t.Value})
			if err != nil {
				return err
			}
			in.Targets[i].Value, in.Targets[i].Role = tags[0], ""
		case KindNetwork:
			if t.Role != RoleServers && t.Role != RoleProxy {
				return httpapi.Errorf(http.StatusBadRequest, "Choose the game servers or the proxy of a network.")
			}
		default:
			return httpapi.Errorf(http.StatusBadRequest, "Choose a tag or a network as target.")
		}
	}
	in.Targets = slices.DeleteFunc(in.Targets, func(t Target) bool { return t.Kind == KindNetwork && !exists(t.Value) })
	slices.SortFunc(in.Targets, compareTargets)
	in.Targets = slices.Compact(in.Targets)
	if in.Files == nil {
		in.Files = []File{}
	}
	return nil
}

func compareTargets(a, b Target) int {
	return cmp.Or(strings.Compare(a.Kind, b.Kind), strings.Compare(a.Value, b.Value), strings.Compare(a.Role, b.Role))
}

// variables are those the master fills in, and whether they need the server's network.
var variables = map[string]bool{"server.name": false, "server.id": false, "server.port": false, "network.server": true}

// placeholderProblem returns why a file has a placeholder that isn't known, or "".
func placeholderProblem(name, content string) string {
	for _, m := range noryxv1.Placeholder.FindAllStringSubmatch(content, -1) {
		key := m[1]
		secret, isSecret := strings.CutPrefix(key, "secret:")
		_, variable := variables[key]
		switch {
		case isSecret && !noryxv1.SecretName.MatchString(secret):
			return fmt.Sprintf("%s: %s names no valid secret. Secret names have up to 64 lower-case letters, digits, - and _.", name, m[0])
		case !isSecret && !variable:
			return fmt.Sprintf("%s: %s is unknown. Use {{server.name}}, {{server.id}}, {{server.port}}, {{network.server}} or {{secret:<name>}}.", name, m[0])
		}
	}
	return ""
}

// member is a server that a set may be for, with what its variables need.
type member struct {
	tag.Server
	Name string
	Port uint32
	// Network is the server's name in its network, empty for none or the proxy.
	Network string
}

// rendered is a set for one server: its files with the variables filled in, the values of
// the secrets they use, and the revision that tells both apart from other states.
type rendered struct {
	files    []*noryxv1.FileSetFile
	secrets  map[string]string
	revision string
}

// render fills in the variables of a set for a server, and picks the secrets its files use.
func render(id string, files []File, values map[string]string, m member) (rendered, error) {
	r := rendered{secrets: map[string]string{}}
	h := sha256.New()
	fmt.Fprintf(h, "%s\n", id)
	var missing error
	for _, f := range files {
		content := noryxv1.Placeholder.ReplaceAllStringFunc(f.Content, func(p string) string {
			key := p[2 : len(p)-2]
			if name, ok := strings.CutPrefix(key, "secret:"); ok {
				value, ok := values[name]
				if !ok {
					missing = cmp.Or(missing, httpapi.Errorf(http.StatusConflict, "The secret %s of %s has no value. Set it in the file set.", name, f.Path))
				}
				r.secrets[key] = value
				return p
			}
			switch key {
			case "server.name":
				return m.Name
			case "server.id":
				return m.ServerID
			case "server.port":
				return strconv.FormatUint(uint64(m.Port), 10)
			}
			if m.Network == "" {
				missing = cmp.Or(missing, httpapi.Errorf(http.StatusConflict, "%s uses %s, but %s is no game server of a network.", f.Path, p, m.Name))
			}
			return m.Network
		})
		r.files = append(r.files, &noryxv1.FileSetFile{Path: f.Path, Content: content, OnlyIfMissing: f.OnlyIfMissing})
		fmt.Fprintf(h, "%q %q %t\n", f.Path, content, f.OnlyIfMissing)
	}
	for _, key := range slices.Sorted(maps.Keys(r.secrets)) {
		fmt.Fprintf(h, "%q %q\n", key, r.secrets[key])
	}
	r.revision = hex.EncodeToString(h.Sum(nil))
	return r, missing
}

// shown returns a file of a set as the panel shows it for a server: with the variables
// filled in and the secrets as placeholders.
func (r rendered) shown(path string) (string, bool) {
	i := slices.IndexFunc(r.files, func(f *noryxv1.FileSetFile) bool { return f.GetPath() == path })
	if i < 0 {
		return "", false
	}
	return r.files[i].GetContent(), true
}
