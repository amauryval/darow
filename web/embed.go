// Package web serves the built Vue application embedded in the binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the built frontend, rooted at the directory index.html sits in.
func Assets() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}
