//go:build !ui

package web

import "io/fs"

// dist is nil in development builds, where the Vite dev server serves the panel.
var dist fs.FS
