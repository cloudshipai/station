package memory

import (
	"os"
	"path/filepath"
)

// FSBackend implements Backend using the local filesystem
type FSBackend struct{}

// NewFSBackend creates a new filesystem backend
func NewFSBackend() *FSBackend {
	return &FSBackend{}
}

// ReadFile reads a file's content
func (b *FSBackend) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(filepath.Clean(path))
}
