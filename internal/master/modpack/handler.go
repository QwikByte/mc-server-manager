package modpack

import (
	"context"
	"net/http"
	"time"

	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/modrinth"
)

const queryTimeout = 30 * time.Second

// Handler serves the versions of modpacks to choose from for a new server; the plugin search
// finds the modpacks.
type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux access.Mux) {
	mux.Handle("GET /api/modpacks/{project}/versions", access.SignedIn, h.versions)
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
