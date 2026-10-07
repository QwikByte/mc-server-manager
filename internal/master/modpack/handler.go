package modpack

import (
	"context"
	"net/http"
	"time"

	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/modrinth"
	"github.com/QwikByte/noryx/internal/master/operation"
	"github.com/QwikByte/noryx/internal/master/plugin"
)

const queryTimeout = 30 * time.Second

// updateNeed is what updating the modpack of a server needs, as it changes the server's
// version and its mods.
var updateNeed = access.All(access.OnServer(access.ServersSettings), access.OnServer(access.Plugins))

// Projects describe the projects of Modrinth as the panel shows them.
type Projects interface {
	Projects(ctx context.Context, ids []string) ([]modrinth.Project, error)
	Describe(p modrinth.Project) plugin.Project
}

// Handler serves the versions of modpacks to choose from, and the modpacks of servers, which
// it moves to other versions; the plugin search finds the modpacks.
type Handler struct {
	svc      *Service
	projects Projects
	ops      *operation.Operations
}

func NewHandler(svc *Service, projects Projects, ops *operation.Operations) *Handler {
	return &Handler{svc: svc, projects: projects, ops: ops}
}

func (h *Handler) Register(mux access.Mux) {
	mux.Handle("GET /api/modpacks/{project}/versions", access.SignedIn, h.versions)
	const base = "/api/nodes/{node}/servers/{id}/modpack"
	mux.Handle("GET "+base, access.OnServer(access.ServersView), h.get)
	mux.Handle("POST "+base, updateNeed, h.update)
}

// Version is a version of a modpack as the panel shows it.
type Version struct {
	ID           string    `json:"id"`
	Number       string    `json:"number"`
	Channel      string    `json:"channel"` // release, beta or alpha
	Published    time.Time `json:"published"`
	GameVersions []string  `json:"gameVersions"`
	Loaders      []string  `json:"loaders"`
}

// versions lists the versions of a modpack for the mod loaders servers run, the newest first.
func (h *Handler) versions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	found, err := h.svc.modrinth.Versions(ctx, r.PathValue("project"), modrinth.AllLoaders("modpacks"), "")
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	versions := make([]Version, 0, len(found))
	for _, v := range found {
		versions = append(versions, Version{v.ID, v.VersionNumber, v.VersionType, v.Published, v.GameVersions, v.Loaders})
	}
	httpapi.WriteJSON(w, http.StatusOK, versions)
}

// Installed is the modpack of a server as the panel shows it.
type Installed struct {
	Project plugin.Project `json:"project"`
	// Version is the ID of the version on Modrinth, and Number its version number.
	Version string `json:"version"`
	Number  string `json:"number"`
}

// get returns the modpack of a server, or null for a server without one the master knows.
// While Modrinth can't be reached, the project has only its ID.
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	i, ok, err := h.svc.installed(ctx, r.PathValue("node"), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if !ok {
		httpapi.WriteJSON(w, http.StatusOK, (*Installed)(nil))
		return
	}
	view := &Installed{Project: plugin.Project{ID: i.project, Slug: i.project, Title: i.project}, Version: i.version, Number: i.number}
	if found, err := h.projects.Projects(ctx, []string{i.project}); err == nil && len(found) == 1 {
		view.Project = h.projects.Describe(found[0])
	}
	httpapi.WriteJSON(w, http.StatusOK, view)
}

// update moves a server to another version of its modpack, newer or older, as an operation.
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	nodeID, serverID := r.PathValue("node"), r.PathValue("id")
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	u, err := h.svc.prepare(ctx, nodeID, serverID, req.Version)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	spec := operation.Spec{
		Kind: "server.modpack", Subject: u.server.GetName(), NodeID: nodeID, ServerID: serverID, Steps: u.steps(),
		Status: http.StatusOK, Timeout: updateTimeout, Category: logging.Plugins, Cancel: updateNeed,
		Visible: func(g access.Grants) bool { return g.On(access.ServersView, nodeID, serverID) },
	}
	h.ops.Run(w, r, spec, func(ctx context.Context) (any, error) {
		defer u.release()
		return h.svc.run(ctx, u)
	})
}
