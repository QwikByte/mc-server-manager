package plugin

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/modrinth"
	"github.com/QwikByte/noryx/internal/master/operation"
)

const (
	queryTimeout = 30 * time.Second
	// installTimeout covers downloading and sending the files to all servers.
	installTimeout = 10 * time.Minute
	maxServers     = 100
)

var gameVersion = regexp.MustCompile(`^[A-Za-z0-9._-]{0,32}$`)

type Handler struct {
	svc *Service
	ops *operation.Operations
}

func NewHandler(svc *Service, ops *operation.Operations) *Handler {
	return &Handler{svc: svc, ops: ops}
}

// Register adds the routes. Searching Modrinth needs no permission; installing on many
// servers at once checks the permission for each of them.
func (h *Handler) Register(mux access.Mux) {
	mux.Handle("GET /api/plugins/search", access.SignedIn, h.search)
	mux.Handle("GET /api/plugins/game-versions", access.SignedIn, h.gameVersions)
	mux.Handle("GET /api/plugins/projects/{project}/versions", access.SignedIn, h.versions)
	mux.Handle("GET /api/plugins/icons/{project}/{file}", access.SignedIn, h.icon)
	mux.Handle("POST /api/plugins/install", access.SignedIn, h.install)
	const base = "/api/nodes/{node}/servers/{id}/plugins"
	mux.Handle("GET "+base, access.OnServer(access.ServersView), h.list)
	mux.Handle("PUT "+base+"/{file}", access.OnServer(access.Plugins), h.upload)
	mux.Handle("DELETE "+base+"/{file}", access.OnServer(access.Plugins), h.remove)
}

type hit struct {
	Project
	Description string    `json:"description"`
	Author      string    `json:"author"`
	Downloads   int64     `json:"downloads"`
	Follows     int64     `json:"follows"`
	Updated     time.Time `json:"updated"`
	// ClientSide tells whether players need the project too: required, optional, unsupported or unknown.
	ClientSide string   `json:"clientSide"`
	Loaders    []string `json:"loaders"`
	Categories []string `json:"categories"` // the main ones, e.g. economy
}

// search finds plugins or mods, or both, for a server type and Minecraft version, both
// optional, in categories, sorted and limited to those players don't need; on Modrinth, or
// plugins on Hangar with the source hangar.
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	q := r.URL.Query()
	s := modrinth.Search{
		Query: q.Get("query"), Kind: q.Get("kind"), Categories: q["category"], Sort: q.Get("sort"), ServerOnly: q.Get("serverOnly") == "true",
	}
	s.Offset, _ = strconv.Atoi(q.Get("offset"))
	t := target{gameVersion: q.Get("version")}
	var err error
	switch {
	case !s.Valid() || !gameVersion.MatchString(t.gameVersion) || s.Offset < 0 || s.Offset > 10_000:
		err = httpapi.Errorf(http.StatusBadRequest, "invalid search")
	case q.Get("type") != "":
		t, err = h.svc.target(ctx, noryxv1.ParseServerType(q.Get("type")), t.gameVersion)
	case t.gameVersion == "LATEST":
		t.gameVersion, err = h.svc.catalogue.modrinth.LatestRelease(ctx)
	}
	var res modrinth.SearchResult
	if err == nil {
		s.Loaders, s.GameVersion = t.loaders, t.gameVersion
		if len(s.Loaders) == 0 {
			s.Loaders = modrinth.AllLoaders(s.Kind)
		}
		if q.Get("source") == "hangar" {
			res, err = h.svc.catalogue.hangar.Search(ctx, s)
		} else {
			res, err = h.svc.catalogue.modrinth.Search(ctx, s)
		}
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	hits := make([]hit, 0, len(res.Hits))
	loader := func(c string) bool { return slices.Contains(s.Loaders, c) }
	for _, m := range res.Hits {
		project := h.svc.Describe(modrinth.Project{ID: m.ProjectID, Slug: m.Slug, Title: m.Title, IconURL: m.IconURL})
		// The categories include the loaders; only those searched for are of interest, so
		// that a project for plugins and mods only shows as either. Both are lists in the
		// JSON, also when Modrinth leaves them out.
		loaders := slices.DeleteFunc(append([]string{}, m.Categories...), func(c string) bool { return !loader(c) })
		categories := slices.DeleteFunc(append([]string{}, m.DisplayCategories...), loader)
		hits = append(hits, hit{project, m.Description, m.Author, m.Downloads, m.Follows, m.Updated, m.ClientSide, loaders, categories})
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"hits": hits, "total": res.Total})
}

// gameVersions lists the releases of Minecraft, the newest first.
func (h *Handler) gameVersions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	releases, err := h.svc.catalogue.modrinth.Releases(ctx)
	write(w, r, http.StatusOK, releases, err)
}

// versions lists the versions of a project for a server type and Minecraft version.
func (h *Handler) versions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	q := r.URL.Query()
	if !gameVersion.MatchString(q.Get("version")) {
		httpapi.WriteError(w, r, httpapi.Errorf(http.StatusBadRequest, "invalid Minecraft version"))
		return
	}
	versions, err := h.svc.Versions(ctx, r.PathValue("project"), noryxv1.ParseServerType(q.Get("type")), q.Get("version"))
	write(w, r, http.StatusOK, versions, err)
}

// icon serves a project icon from Modrinth's CDN, so the browser doesn't contact Modrinth.
func (h *Handler) icon(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	icon := h.svc.catalogue.modrinth.Icon
	name := r.PathValue("project") + "/" + r.PathValue("file")
	if r.PathValue("project")+"/" == hangarIcons {
		icon, name = h.svc.catalogue.hangar.Icon, r.PathValue("file")
	}
	data, contentType, err := icon(ctx, name)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	_, _ = w.Write(data) //nolint:gosec // an image of a fixed type, sandboxed, see above
}

func (h *Handler) install(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Projects []string `json:"projects"`
		// Versions are the IDs of the versions chosen for projects, by project ID.
		Versions map[string]string `json:"versions"`
		Servers  []Ref             `json:"servers"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	badVersion := func(project string) bool {
		return !slices.Contains(req.Projects, project) || !ValidProjectID(req.Versions[project])
	}
	grants := access.From(r.Context())
	switch err := checkProjects(req.Projects); {
	case err != nil:
		httpapi.WriteError(w, r, err)
		return
	case slices.ContainsFunc(slices.Collect(maps.Keys(req.Versions)), badVersion):
		httpapi.WriteError(w, r, httpapi.Errorf(http.StatusBadRequest, "Choose versions only for the projects to install."))
		return
	case len(req.Servers) == 0 || len(req.Servers) > maxServers || len(slices.Compact(slices.SortedFunc(slices.Values(req.Servers), compareRefs))) != len(req.Servers):
		httpapi.WriteError(w, r, httpapi.Errorf(http.StatusBadRequest, "Choose up to %d different servers.", maxServers))
		return
	case !onAll(grants, access.Plugins, req.Servers):
		httpapi.WriteError(w, r, access.Denied(access.Plugins))
		return
	}
	spec := operation.Spec{
		Kind: "plugins.install", Subject: strconv.Itoa(len(req.Servers)), Steps: []string{"plugins"}, Status: http.StatusOK,
		Timeout: installTimeout, Category: logging.Plugins,
		Visible: func(g access.Grants) bool { return onAll(g, access.ServersView, req.Servers) },
		Cancel: func(_ *http.Request, g access.Grants) (access.Permission, bool) {
			return access.Plugins, onAll(g, access.Plugins, req.Servers)
		},
	}
	if len(req.Servers) == 1 {
		spec.NodeID, spec.ServerID = req.Servers[0].NodeID, req.Servers[0].ServerID
	}
	h.ops.Run(w, r, spec, func(ctx context.Context) (any, error) {
		return map[string]any{"results": h.svc.Install(ctx, req.Projects, req.Versions, req.Servers)}, nil
	})
}

// checkProjects checks the Modrinth projects to install, before anything is looked up.
func checkProjects(projects []string) error {
	if len(projects) == 0 || len(projects) > maxProjects || slices.ContainsFunc(projects, func(id string) bool { return !ValidProjectID(id) }) {
		return httpapi.Errorf(http.StatusBadRequest, "Choose up to %d projects to install.", maxProjects)
	}
	return nil
}

func compareRefs(a, b Ref) int {
	return strings.Compare(a.NodeID+"/"+a.ServerID, b.NodeID+"/"+b.ServerID)
}

// onAll reports whether the grants allow p on all servers.
func onAll(g access.Grants, p access.Permission, servers []Ref) bool {
	return !slices.ContainsFunc(servers, func(s Ref) bool { return !g.On(p, s.NodeID, s.ServerID) })
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	l, err := h.svc.List(ctx, ref(r))
	write(w, r, http.StatusOK, l, err)
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context()) // cancelling discards the partial file
	defer cancel()
	p, err := h.svc.Upload(ctx, ref(r), r.PathValue("file"), http.MaxBytesReader(w, r.Body, modrinth.MaxFileSize))
	if tooLarge := new(http.MaxBytesError); errors.As(err, &tooLarge) {
		err = httpapi.Errorf(http.StatusRequestEntityTooLarge, "Plugins can have up to %d MB.", modrinth.MaxFileSize>>20)
	}
	write(w, r, http.StatusCreated, p, err)
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	write(w, r, http.StatusNoContent, nil, h.svc.Remove(ctx, ref(r), r.PathValue("file")))
}

func ref(r *http.Request) Ref { return Ref{NodeID: r.PathValue("node"), ServerID: r.PathValue("id")} }

func write(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, status, v)
}
