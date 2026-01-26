package skills

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

// ListDir lists directory entries
func (b *FSBackend) ListDir(path string) ([]DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	result := make([]DirEntry, len(entries))
	for i, e := range entries {
		result[i] = DirEntry{
			Name:  e.Name(),
			IsDir: e.IsDir(),
		}
	}
	return result, nil
}

// ReadFile reads a file's content
func (b *FSBackend) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(filepath.Clean(path))
}
