package webapp

import (
	"embed"
	"io/fs"
)

//go:embed all:web
var embeddedFS embed.FS

// GetFS returns the embedded filesystem containing the web assets
func GetFS() (fs.FS, error) {
	return fs.Sub(embeddedFS, "web")
}
