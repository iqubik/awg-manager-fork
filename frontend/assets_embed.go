//go:build embed_frontend

package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:build
var files embed.FS

func FS() (fs.FS, error) {
	return fs.Sub(files, "build")
}
