package server

import (
	"context"
	"net/http"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

const commandTimeout = 30 * time.Second

// logs streams the console of a server, see httpapi.StreamLog, until the server stops.
func (h *Handler) logs(w http.ResponseWriter, r *http.Request) {
	httpapi.StreamLog(w, r, func(ctx context.Context, tail uint32, after int64) (httpapi.Stream[*noryxv1.StreamLogsResponse], error) {
		c, err := h.client(ctx, r)
		if err != nil {
			return nil, err
		}
		return c.StreamLogs(ctx, &noryxv1.StreamLogsRequest{Id: r.PathValue("id"), Tail: tail, AfterUnixNano: after})
	})
}

func (h *Handler) command(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Command string `json:"command"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), commandTimeout)
	defer cancel()
	c, err := h.client(ctx, r)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	res, err := c.SendCommand(ctx, &noryxv1.SendCommandRequest{Id: r.PathValue("id"), Command: req.Command})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"output": res.GetOutput()})
}
