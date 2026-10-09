package notify

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

// Handler serves the channels and rules of notifications to the panel.
type Handler struct {
	svc *Service
	// tidy removes the references to what was deleted, e.g. the channel a workflow notifies.
	tidy func(context.Context)
}

func NewHandler(svc *Service, tidy func(context.Context)) *Handler {
	return &Handler{svc: svc, tidy: tidy}
}

// Register adds the routes. Rules send entries about every node and server, so managing them
// also needs the permission to see the log everywhere.
func (h *Handler) Register(mux access.Mux) {
	manage := access.All(access.Everywhere(access.NotificationsManage), access.Everywhere(access.LogsView))
	mux.Handle("GET /api/notifications", manage, func(w http.ResponseWriter, r *http.Request) {
		channels, err := h.svc.Channels(r.Context())
		var rules []Rule
		if err == nil {
			rules, err = h.svc.Rules(r.Context())
		}
		write(w, r, http.StatusOK, map[string]any{"channels": channels, "rules": rules}, err)
	})
	mux.Handle("POST /api/notifications/channels", manage, func(w http.ResponseWriter, r *http.Request) {
		var in ChannelInput
		if read(w, r, &in) {
			logging.Note(r.Context(), slog.String("channel", in.Name), slog.String("kind", in.Kind))
			ch, err := h.svc.CreateChannel(r.Context(), in)
			write(w, r, http.StatusCreated, ch, err)
		}
	})
	mux.Handle("PUT /api/notifications/channels/{id}", manage, func(w http.ResponseWriter, r *http.Request) {
		var in ChannelInput
		if read(w, r, &in) {
			logging.Note(r.Context(), slog.String("channel", in.Name), slog.String("channel_id", r.PathValue("id")),
				slog.Bool("new_secret", in.URL != "" || in.Password != ""))
			ch, err := h.svc.UpdateChannel(r.Context(), r.PathValue("id"), in)
			write(w, r, http.StatusOK, ch, err)
		}
	})
	mux.Handle("DELETE /api/notifications/channels/{id}", manage, func(w http.ResponseWriter, r *http.Request) {
		ch, err := h.svc.DeleteChannel(r.Context(), r.PathValue("id"))
		logging.Note(r.Context(), slog.String("channel", ch.Name), slog.String("channel_id", r.PathValue("id")))
		if err == nil {
			h.tidy(r.Context())
		}
		write(w, r, http.StatusNoContent, nil, err)
	})
	mux.Handle("POST /api/notifications/channels/{id}/test", manage, func(w http.ResponseWriter, r *http.Request) {
		ch, err := h.svc.Test(r.Context(), r.PathValue("id"))
		logging.Note(r.Context(), slog.String("channel", ch.Name), slog.String("channel_id", r.PathValue("id")))
		write(w, r, http.StatusNoContent, nil, err)
	})
	mux.Handle("POST /api/notifications/rules", manage, func(w http.ResponseWriter, r *http.Request) {
		var in RuleInput
		if read(w, r, &in) {
			logging.Note(r.Context(), slog.String("channel_id", in.ChannelID))
			rule, err := h.svc.CreateRule(r.Context(), in)
			write(w, r, http.StatusCreated, rule, err)
		}
	})
	mux.Handle("PUT /api/notifications/rules/{id}", manage, func(w http.ResponseWriter, r *http.Request) {
		var in RuleInput
		if read(w, r, &in) {
			logging.Note(r.Context(), slog.String("channel_id", in.ChannelID), slog.Bool("enabled", in.Enabled))
			rule, err := h.svc.UpdateRule(r.Context(), r.PathValue("id"), in)
			write(w, r, http.StatusOK, rule, err)
		}
	})
	mux.Handle("DELETE /api/notifications/rules/{id}", manage, func(w http.ResponseWriter, r *http.Request) {
		write(w, r, http.StatusNoContent, nil, h.svc.DeleteRule(r.Context(), r.PathValue("id")))
	})
}

func read(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := httpapi.ReadJSON(w, r, v); err != nil {
		httpapi.WriteError(w, r, err)
		return false
	}
	return true
}

func write(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, status, v)
}
