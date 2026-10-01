// Package web embeds the built admin panel into the master binary.
// Build with `-tags ui` after `npm run build` to include it (see Makefile).
package web

import (
	"io/fs"
	"net/http"
	"strings"
)

// Handler serves the admin panel. Unknown paths fall back to index.html so that
// client-side routes survive a page reload.
func Handler() http.Handler {
	if dist == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "The admin panel is not part of this build. Build with `make build` or use the Vite dev server.", http.StatusNotFound)
		})
	}
	files := http.FileServerFS(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // file names are content hashed
		} else if _, err := fs.Stat(dist, strings.TrimPrefix(r.URL.Path, "/")); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		files.ServeHTTP(w, r)
	})
}
