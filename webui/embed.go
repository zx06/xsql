package webui

import (
	"embed"
	"io/fs"
)

// DistFiles contains the built frontend assets. CI/release builds populate dist/.
//
//go:embed all:dist
var DistFiles embed.FS

// Dist returns the embedded frontend dist filesystem.
func Dist() fs.FS {
	sub, err := fs.Sub(DistFiles, "dist")
	if err != nil {
		return DistFiles
	}
	return sub
}
