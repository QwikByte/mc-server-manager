package httpapi

import (
	"mime"
	"net/http"
)

// Attachment makes the browser save the response instead of rendering it, so files from a
// server can't run scripts in the panel.
func Attachment(w http.ResponseWriter, name string) {
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	h.Set("Content-Security-Policy", "sandbox")
	h.Set("Cache-Control", "no-store")
}

// Chunk is a message of a stream from an agent that carries part of a file.
type Chunk interface{ GetData() []byte }

// Relay copies a stream of chunks to the response. Once the response has started, an
// error can only cut it short, which the browser reports as a failed download.
func Relay[T Chunk](w http.ResponseWriter, first T, stream interface{ Recv() (T, error) }) {
	for msg, err := first, error(nil); err == nil; msg, err = stream.Recv() {
		if _, err := w.Write(msg.GetData()); err != nil { //nolint:gosec // an attachment, see Attachment
			return
		}
	}
}
