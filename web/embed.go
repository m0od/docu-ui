// Package web exposes the built UI (web/dist) to the Go binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the built UI rooted at dist/.
func Dist() fs.FS {
	distRoot, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // "dist" is a literal embedded directory; Sub only fails on an invalid path
	}
	return distRoot
}
