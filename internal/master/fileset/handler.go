package fileset

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/auth"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/operation"
	"github.com/QwikByte/noryx/internal/master/tag"
)

// applyOperationTimeout covers applying a set to many servers and restarting them.
const applyOperationTimeout = time.Hour

type Handler struct {
	svc *Service
	ops *operation.Operations
}

func NewHandler(svc *Service, ops *operation.Operations) *Handler {
	return &Handler{svc: svc, ops: ops}
}

// Register adds the routes. Previewing and applying a set also need the permission to change
// the files of every server it touches, as its files can configure plugins that run code.
func (h *Handler) Register(mux access.Mux) {
	view, manage := access.Everywhere(access.FileSetsView), access.Everywhere(access.FileSetsManage)
	mux.Handle("GET /api/filesets", view, func(w http.ResponseWriter, r *http.Request) {
		list, err := h.svc.List(r.Context())
		write(w, r, http.StatusOK, list, err)
	})
	mux.Handle("POST /api/filesets", manage, func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if read(w, r, &in) {
			logging.Note(r.Context(), slog.String("name", in.Name))
			set, err := h.svc.Create(r.Context(), in, username(r))
			write(w, r, http.StatusCreated, set, err)
		}
	})
	// The states of all sets, for the list of sets, on the servers the user may see.
	mux.Handle("GET /api/filesets/status", view, func(w http.ResponseWriter, r *http.Request) {
		all, err := h.svc.StatusAll(r.Context())
		for id, list := range all {
			all[id] = visible(r, list)
		}
		write(w, r, http.StatusOK, all, err)
	})
	mux.Handle("GET /api/filesets/{id}", view, func(w http.ResponseWriter, r *http.Request) {
		set, err := h.svc.Get(r.Context(), r.PathValue("id"))
		write(w, r, http.StatusOK, set, err)
	})
	mux.Handle("PUT /api/filesets/{id}", manage, func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if read(w, r, &in) {
			logging.Note(r.Context(), slog.String("name", in.Name))
			set, err := h.svc.Update(r.Context(), r.PathValue("id"), in, username(r))
			write(w, r, http.StatusOK, set, err)
		}
	})
	mux.Handle("DELETE /api/filesets/{id}", manage, func(w http.ResponseWriter, r *http.Request) {
		write(w, r, http.StatusNoContent, nil, h.svc.Delete(r.Context(), r.PathValue("id")))
	})
	mux.Handle("GET /api/filesets/{id}/versions/{version}", view, func(w http.ResponseWriter, r *http.Request) {
		version, err := strconv.ParseInt(r.PathValue("version"), 10, 64)
		var v Version
		if err == nil {
			v, err = h.svc.Version(r.Context(), r.PathValue("id"), version)
		} else {
			err = httpapi.Errorf(http.StatusNotFound, "Version not found.")
		}
		write(w, r, http.StatusOK, v, err)
	})
	// Values can be set but never read.
	mux.Handle("PUT /api/filesets/{id}/secrets/{name}", manage, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Value    string `json:"value"`
			Generate bool   `json:"generate"`
		}
		if read(w, r, &req) {
			sec, err := h.svc.SetSecret(r.Context(), r.PathValue("id"), r.PathValue("name"), req.Value, req.Generate)
			write(w, r, http.StatusOK, sec, err)
		}
	})
	mux.Handle("DELETE /api/filesets/{id}/secrets/{name}", manage, func(w http.ResponseWriter, r *http.Request) {
		write(w, r, http.StatusNoContent, nil, h.svc.DeleteSecret(r.Context(), r.PathValue("id"), r.PathValue("name")))
	})
	mux.Handle("GET /api/filesets/{id}/status", view, func(w http.ResponseWriter, r *http.Request) {
		list, err := h.svc.Status(r.Context(), r.PathValue("id"))
		write(w, r, http.StatusOK, visible(r, list), err)
	})
	mux.Handle("POST /api/filesets/{id}/preview", manage, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Version int64 `json:"version"`
		}
		if read(w, r, &req) {
			p, err := h.svc.Preview(r.Context(), r.PathValue("id"), req.Version, allowed(r, false))
			write(w, r, http.StatusOK, p, err)
		}
	})
	mux.Handle("POST /api/filesets/{id}/apply", manage, func(w http.ResponseWriter, r *http.Request) {
		var req ApplyRequest
		set, err := h.svc.Get(r.Context(), r.PathValue("id"))
		if err == nil {
			err = httpapi.ReadJSON(w, r, &req)
		}
		if err != nil {
			httpapi.WriteError(w, r, err)
			return
		}
		logging.Note(r.Context(), slog.String("name", set.Name), slog.Int64("version", req.Version), slog.Bool("restart", req.Restart))
		if len(req.Servers) > 0 {
			logging.Note(r.Context(), slog.Int("servers", len(req.Servers)))
		}
		check := allowed(r, req.Restart)
		steps := []string{"files"}
		// Others may cancel it if they may do the same on all servers, as the servers of the
		// set are only known as it runs.
		cancel := []access.Need{access.Everywhere(access.FileSetsManage), access.Everywhere(access.FilesWrite)}
		if req.Restart {
			steps = append(steps, "restart")
			cancel = append(cancel, access.Everywhere(access.ServersRestart))
		}
		h.ops.Run(w, r, operation.Spec{
			Kind: "fileset.apply", Subject: set.Name, Steps: steps, Status: http.StatusOK, Timeout: applyOperationTimeout, Category: logging.Files,
			// Its results name the servers of the set, which not everyone who sees sets may see.
			Visible: func(g access.Grants) bool { return g.Has(access.FileSetsView) && g.Has(access.ServersView) },
			Cancel:  access.All(cancel...),
		}, func(ctx context.Context) (any, error) {
			results, err := h.svc.Apply(ctx, set.ID, req, check)
			return map[string]any{"results": results}, err
		})
	})
}

// allowed returns the check that the user may change the files of a server, and restart it
// if restart is set.
func allowed(r *http.Request, restart bool) func(tag.Server) error {
	grants := access.From(r.Context())
	return func(ref tag.Server) error {
		for _, p := range []access.Permission{access.FilesWrite, access.ServersRestart} {
			if (p == access.FilesWrite || restart) && !grants.On(p, ref.NodeID, ref.ServerID) {
				return access.Denied(p)
			}
		}
		return nil
	}
}

// visible returns the states of the servers the user may see.
func visible(r *http.Request, list []ServerStatus) []ServerStatus {
	grants, out := access.From(r.Context()), []ServerStatus{}
	for _, st := range list {
		if grants.On(access.ServersView, st.NodeID, st.ServerID) {
			out = append(out, st)
		}
	}
	return out
}

func username(r *http.Request) string {
	user, _ := auth.UserFrom(r.Context())
	return user.Username
}

// read reads the JSON body of a request into v, or answers with the error.
func read(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := httpapi.ReadJSON(w, r, v); err != nil {
		httpapi.WriteError(w, r, err)
		return false
	}
	return true
}

func write(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	switch {
	case err != nil:
		httpapi.WriteError(w, r, err)
	case v == nil:
		w.WriteHeader(status)
	default:
		httpapi.WriteJSON(w, status, v)
	}
}
