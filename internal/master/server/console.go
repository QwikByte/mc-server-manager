package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const (
	logTail        = 300
	commandTimeout = 30 * time.Second
)

// lineBreaks would let a log line forge additional events in the stream.
var lineBreaks = strings.NewReplacer("\r", " ", "\n", " ")

// logs streams the console of a server as Server-Sent Events, one line per event.
// An "end" event tells the browser not to reconnect, e.g. because the server stopped.
func (h *Handler) logs(w http.ResponseWriter, r *http.Request) {
	c, err := h.client(r.Context(), r)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	stream, err := c.StreamLogs(r.Context(), &mcsmv1.StreamLogsRequest{Id: r.PathValue("id"), Tail: logTail})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	// Errors such as an unknown server arrive with the first message, before any output.
	res, err := stream.Recv()
	if err != nil && !errors.Is(err, io.EOF) {
		httpapi.WriteError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no") // keeps nginx from buffering the stream
	flusher := http.NewResponseController(w)
	for ; err == nil; res, err = stream.Recv() {
		// Plain text for EventSource, served with nosniff and rendered as text by the panel.
		fmt.Fprintf(w, "data: %s\n\n", lineBreaks.Replace(res.GetLine())) //nolint:gosec // not HTML, see above
		if flusher.Flush() != nil {
			return
		}
	}
	if errors.Is(err, io.EOF) {
		fmt.Fprint(w, "event: end\ndata:\n\n")
		_ = flusher.Flush()
	}
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
	res, err := c.SendCommand(ctx, &mcsmv1.SendCommandRequest{Id: r.PathValue("id"), Command: req.Command})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"output": res.GetOutput()})
}
