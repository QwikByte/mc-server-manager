//go:build ui

package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

var dist, _ = fs.Sub(embedded, "dist")
