//go:build !embed_frontend

package frontend

import (
	"errors"
	"io/fs"
)

func FS() (fs.FS, error) {
	return nil, errors.New("frontend is not embedded: rebuild with -tags embed_frontend")
}
