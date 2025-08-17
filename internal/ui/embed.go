//go:build ui

package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

// Embed the UI build files when building with -tags ui
//go:embed static
var uiFiles embed.FS

// GetFileSystem returns the embedded UI filesystem
func GetFileSystem() (http.FileSystem, error) {
	// Get the subdirectory containing the built files
	fsys, err := fs.Sub(uiFiles, "static")
	if err != nil {
		return nil, err
	}
	return http.FS(fsys), nil
}

// IsEmbedded returns true when UI is embedded (build tag enabled)
func IsEmbedded() bool {
	return true
}