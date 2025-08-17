//go:build !ui

package ui

import (
	"net/http"
	"fmt"
)

// GetFileSystem returns nil when UI is not embedded
func GetFileSystem() (http.FileSystem, error) {
	return nil, fmt.Errorf("UI not embedded in this build. Build with 'make build-with-ui' to include UI")
}

// IsEmbedded returns false when UI is not embedded
func IsEmbedded() bool {
	return false
}