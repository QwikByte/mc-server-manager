// Package backup backs up and restores servers through the agents of their nodes, which
// keep the backups, and runs backup jobs on a schedule.
package backup

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/access"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const (
	queryTimeout = 30 * time.Second
	// backupTimeout covers backing up and restoring servers with large worlds.
	backupTimeout = 2 * time.Hour
	maxPaths      = 20
)

// Nodes provides connections to node agents.
type Nodes interface {
	Conn(ctx context.Context, nodeID string) (grpc.ClientConnInterface, error)
}

// Selection chooses what of a server is backed up; the agent finds the matching files.
type Selection struct {
	Everything bool     `json:"everything"`
	Worlds     bool     `json:"worlds"`
	Plugins    bool     `json:"plugins"`
	Config     bool     `json:"config"`
	Paths      []string `json:"paths"` // relative to the server's folder
}

// check returns a message for the administrator if the selection is invalid.
func (s *Selection) check() string {
	if s.Paths == nil {
		s.Paths = []string{}
	}
	switch {
	case !s.Everything && !s.Worlds && !s.Plugins && !s.Config && len(s.Paths) == 0:
		return "Choose what to back up."
	case len(s.Paths) > maxPaths:
		return "Enter at most 20 files or folders."
	}
	for _, p := range s.Paths {
		if p == "" || len(p) > 1024 || strings.ContainsAny(p, "\x00\\") || slices.Contains(strings.Split(p, "/"), "..") {
			return "The path " + p + " is invalid. Enter paths inside the server's folder, like plugins/LuckPerms."
		}
	}
	return ""
}

func (s Selection) proto() *mcsmv1.BackupSelection {
	return &mcsmv1.BackupSelection{Everything: s.Everything, Worlds: s.Worlds, Plugins: s.Plugins, Config: s.Config, Paths: s.Paths}
}

type view struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"createdAt"`
	Size      int64     `json:"size"`
	Location  string    `json:"location"`
	Paths     []string  `json:"paths"` // "." is everything
	JobID     string    `json:"jobId,omitempty"`
}

func toView(b *mcsmv1.Backup) view {
	return view{b.GetId(), b.GetLabel(), time.Unix(b.GetCreatedUnix(), 0), b.GetSize(), b.GetLocation(), append([]string{}, b.GetPaths()...), b.GetJobId()}
}

type Handler struct{ nodes Nodes }

func NewHandler(nodes Nodes) *Handler { return &Handler{nodes: nodes} }

func (h *Handler) Register(mux access.Mux) {
	const base = "/api/nodes/{node}/servers/{id}/backups"
	mux.Handle("GET "+base, access.OnServer(access.BackupsView), h.list)
	mux.Handle("POST "+base, access.OnServer(access.BackupsCreate), h.create)
	mux.Handle("POST "+base+"/{backup}/restore", access.OnServer(access.BackupsRestore), h.restore)
	mux.Handle("DELETE "+base+"/{backup}", access.OnServer(access.BackupsDelete), h.delete)
	mux.Handle("GET "+base+"/{backup}/download", access.OnServer(access.BackupsView), h.download)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	c, err := h.client(ctx, r)
	var res *mcsmv1.ListBackupsResponse
	if err == nil {
		res, err = c.ListBackups(ctx, &mcsmv1.ListBackupsRequest{ServerId: r.PathValue("id")})
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	views := make([]view, 0, len(res.GetBackups()))
	for _, b := range res.GetBackups() {
		views = append(views, toView(b))
	}
	httpapi.WriteJSON(w, http.StatusOK, views)
}

// create backs up a server. It continues if the browser goes away, as large worlds take a while.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label     string    `json:"label"`
		Selection Selection `json:"selection"`
		Location  string    `json:"location"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if msg := req.Selection.check(); msg != "" {
		httpapi.WriteError(w, r, httpapi.Errorf(http.StatusBadRequest, "%s", msg))
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), backupTimeout)
	defer cancel()
	c, err := h.client(ctx, r)
	var res *mcsmv1.CreateBackupResponse
	if err == nil {
		res, err = c.CreateBackup(ctx, &mcsmv1.CreateBackupRequest{
			ServerId: r.PathValue("id"), Label: req.Label, Selection: req.Selection.proto(), Location: req.Location,
		})
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, toView(res.GetBackup()))
}

// restore replaces the data of a server with a backup. It finishes if the browser goes away,
// so the server isn't left stopped.
func (h *Handler) restore(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), backupTimeout)
	defer cancel()
	c, err := h.client(ctx, r)
	if err == nil {
		_, err = c.RestoreBackup(ctx, &mcsmv1.RestoreBackupRequest{ServerId: r.PathValue("id"), BackupId: r.PathValue("backup")})
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	c, err := h.client(ctx, r)
	if err == nil {
		_, err = c.DeleteBackup(ctx, &mcsmv1.DeleteBackupRequest{ServerId: r.PathValue("id"), BackupId: r.PathValue("backup")})
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	c, err := h.client(r.Context(), r)
	var stream grpc.ServerStreamingClient[mcsmv1.DownloadBackupResponse]
	if err == nil {
		stream, err = c.DownloadBackup(r.Context(), &mcsmv1.DownloadBackupRequest{ServerId: r.PathValue("id"), BackupId: r.PathValue("backup"), HideSecrets: true})
	}
	var first *mcsmv1.DownloadBackupResponse
	if err == nil {
		first, err = stream.Recv() // errors such as an unknown backup arrive here
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.Attachment(w, "backup-"+r.PathValue("backup")+".zip")
	if size := first.GetSize(); size > 0 { // unknown while the agent hides the secrets
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	httpapi.Relay(w, first, stream)
}

func (h *Handler) client(ctx context.Context, r *http.Request) (mcsmv1.BackupServiceClient, error) {
	conn, err := h.nodes.Conn(ctx, r.PathValue("node"))
	if err != nil {
		return nil, err
	}
	return mcsmv1.NewBackupServiceClient(conn), nil
}
