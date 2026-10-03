package e2e

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QwikByte/mc-server-manager/internal/master/modrinth"
)

// fakeModrinth serves the parts of Modrinth's API (/v2) and CDN (/cdn) the master uses:
//
//   - luckperms: a Paper plugin with the releases 1.0 and 2.0; 2.0 requires vault
//   - vault: a Paper plugin
//   - fabricapi: a Fabric mod
//   - broken: a Paper plugin whose download doesn't match its hash
type fakeModrinth struct {
	*httptest.Server
	projects []modrinth.Project
	versions []fakeVersion
	files    map[string][]byte          // by CDN path
	search   atomic.Pointer[url.Values] // the query of the last search
}

type fakeVersion struct {
	modrinth.Version
	Loaders []string `json:"loaders"`
}

func startModrinth(t *testing.T) *fakeModrinth {
	f := &fakeModrinth{files: map[string][]byte{"/cdn/data/luckperms/icon.png": []byte("\x89PNG")}}
	mux := http.NewServeMux()
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	paper := []string{"paper", "spigot", "bukkit"}
	f.project("luckperms", "LuckPerms", paper)
	f.version("luckperms", "1.0", paper)
	f.version("luckperms", "2.0", paper, "vault")
	f.project("vault", "Vault", paper)
	f.version("vault", "1.7", paper)
	f.project("fabricapi", "Fabric API", []string{"fabric"})
	f.version("fabricapi", "0.119", []string{"fabric"})
	f.project("broken", "Broken", paper)
	f.version("broken", "1.0", paper)
	f.files["/cdn/data/broken/broken-1.0.jar"] = []byte("tampered")

	mux.HandleFunc("GET /v2/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f.search.Store(&q)
		hits := []modrinth.SearchHit{}
		for _, p := range f.projects {
			if strings.Contains(r.URL.Query().Get("facets"), "categories:"+p.Loaders[0]) {
				hits = append(hits, modrinth.SearchHit{ProjectID: p.ID, Slug: p.Slug, Title: p.Title, IconURL: p.IconURL, Categories: p.Loaders})
			}
		}
		writeJSON(w, map[string]any{"hits": hits, "total_hits": len(hits)})
	})
	mux.HandleFunc("GET /v2/projects", func(w http.ResponseWriter, r *http.Request) {
		var ids []string
		_ = json.Unmarshal([]byte(r.URL.Query().Get("ids")), &ids)
		writeJSON(w, slices.DeleteFunc(slices.Clone(f.projects), func(p modrinth.Project) bool { return !slices.Contains(ids, p.ID) }))
	})
	mux.HandleFunc("GET /v2/project/{id}/version", func(w http.ResponseWriter, r *http.Request) {
		var loaders []string
		_ = json.Unmarshal([]byte(r.URL.Query().Get("loaders")), &loaders)
		writeJSON(w, f.find(func(v fakeVersion) bool { return v.ProjectID == r.PathValue("id") && overlaps(v.Loaders, loaders) }))
	})
	mux.HandleFunc("POST /v2/version_files", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, f.byHash(r, func(found fakeVersion, _ []string) (fakeVersion, bool) { return found, true }))
	})
	mux.HandleFunc("POST /v2/version_files/update", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, f.byHash(r, func(found fakeVersion, loaders []string) (fakeVersion, bool) {
			newer := f.find(func(v fakeVersion) bool { return v.ProjectID == found.ProjectID && overlaps(v.Loaders, loaders) })
			if len(newer) == 0 {
				return found, false
			}
			return newer[0], true
		}))
	})
	mux.HandleFunc("GET /v2/tag/game_version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, []map[string]string{{"version": "1.22-pre1", "version_type": "snapshot"}, {"version": "1.21.4", "version_type": "release"}})
	})
	mux.HandleFunc("GET /cdn/", func(w http.ResponseWriter, r *http.Request) {
		data, ok := f.files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	})
	return f
}

func (f *fakeModrinth) project(id, title string, loaders []string) {
	f.projects = append(f.projects, modrinth.Project{ID: id, Slug: id, Title: title, Loaders: loaders, IconURL: f.URL + "/cdn/data/" + id + "/icon.png"})
}

// version adds a version, newer than the ones before, with a file named <project>-<number>.jar.
func (f *fakeModrinth) version(project, number string, loaders []string, requires ...string) {
	content := []byte(project + " " + number)
	sum := sha512.Sum512(content)
	name := project + "-" + number + ".jar"
	file := modrinth.File{URL: f.URL + "/cdn/data/" + project + "/" + name, Filename: name, Primary: true, Size: int64(len(content))}
	file.Hashes.SHA512 = hex.EncodeToString(sum[:])
	f.files["/cdn/data/"+project+"/"+name] = content
	v := modrinth.Version{
		ID: project + strings.ReplaceAll(number, ".", ""), ProjectID: project, VersionNumber: number, VersionType: "release",
		Published: time.Date(2026, 1, len(f.versions)+1, 0, 0, 0, 0, time.UTC), Files: []modrinth.File{file},
	}
	for _, dep := range requires {
		v.Dependencies = append(v.Dependencies, modrinth.Dependency{ProjectID: dep, Type: "required"})
	}
	f.versions = append(f.versions, fakeVersion{v, loaders})
}

// content returns the file of a version, e.g. content("luckperms", "1.0").
func (f *fakeModrinth) content(project, number string) []byte {
	return f.files["/cdn/data/"+project+"/"+project+"-"+number+".jar"]
}

// find returns the matching versions, the newest first.
func (f *fakeModrinth) find(match func(fakeVersion) bool) []fakeVersion {
	found := slices.DeleteFunc(slices.Clone(f.versions), func(v fakeVersion) bool { return !match(v) })
	slices.Reverse(found)
	return found
}

// byHash answers a lookup of files by hash with the version that pick chooses.
func (f *fakeModrinth) byHash(r *http.Request, pick func(found fakeVersion, loaders []string) (fakeVersion, bool)) map[string]fakeVersion {
	var req struct {
		Hashes  []string `json:"hashes"`
		Loaders []string `json:"loaders"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	res := map[string]fakeVersion{}
	for _, hash := range req.Hashes {
		found := f.find(func(v fakeVersion) bool { return v.Files[0].Hashes.SHA512 == hash })
		if len(found) == 0 {
			continue
		}
		if v, ok := pick(found[0], req.Loaders); ok {
			res[hash] = v
		}
	}
	return res
}

func overlaps(a, b []string) bool {
	return slices.ContainsFunc(a, func(s string) bool { return slices.Contains(b, s) })
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
