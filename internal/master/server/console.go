package server

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const (
	logTail = 300
	// maxTail is the most lines a browser that connects again gets of what it missed.
	maxTail        = 1000
	commandTimeout = 30 * time.Second
	// A console stream ends after maxStream with a "renew" event, so that the browser
	// connects again, which checks the session and the permissions again.
	maxStream = 5 * time.Minute
)

// lineBreaks would let a log line forge additional events in the stream.
var lineBreaks = strings.NewReplacer("\r", " ", "\n", " ")

// logs streams the console of a server as Server-Sent Events, one line per event, whose ID
// is the time the line was written. A browser that connects again with the ID of the last
// line it got, as Last-Event-ID or after, continues after it. An "end" event tells the
// browser not to connect again, e.g. because the server stopped.
func (h *Handler) logs(w http.ResponseWriter, r *http.Request) {
	after, err := strconv.ParseInt(cmp.Or(r.Header.Get("Last-Event-ID"), r.URL.Query().Get("after"), "0"), 10, 64)
	if err != nil {
		httpapi.WriteError(w, r, httpapi.Errorf(http.StatusBadRequest, "The console can't continue after that line."))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), maxStream)
	defer cancel()
	c, err := h.client(ctx, r)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	req := &mcsmv1.StreamLogsRequest{Id: r.PathValue("id"), Tail: logTail, AfterUnixNano: after}
	if after > 0 {
		req.Tail = maxTail
	}
	stream, err := c.StreamLogs(ctx, req)
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
		if t := res.GetTimeUnixNano(); t > 0 {
			fmt.Fprintf(w, "id: %d\n", t)
		}
		// Plain text for EventSource, served with nosniff and rendered as text by the panel.
		fmt.Fprintf(w, "data: %s\n\n", lineBreaks.Replace(res.GetLine())) //nolint:gosec // not HTML, see above
		if flusher.Flush() != nil {
			return
		}
	}
	switch {
	case errors.Is(err, io.EOF):
		fmt.Fprint(w, "event: end\ndata:\n\n")
	case r.Context().Err() == nil && errors.Is(ctx.Err(), context.DeadlineExceeded):
		fmt.Fprint(w, "event: renew\ndata:\n\n")
	}
	_ = flusher.Flush()
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
