package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type HostWorkspace struct {
	basePath       string
	allowedPaths   []string
	deniedPatterns []string
	initialized    bool
}

type HostWorkspaceOption func(*HostWorkspace)

func WithAllowedPaths(paths []string) HostWorkspaceOption {
	return func(w *HostWorkspace) {
		w.allowedPaths = paths
	}
}

func WithDeniedPatterns(patterns []string) HostWorkspaceOption {
	return func(w *HostWorkspace) {
		w.deniedPatterns = patterns
	}
}

func NewHostWorkspace(basePath string, opts ...HostWorkspaceOption) *HostWorkspace {
	w := &HostWorkspace{
		basePath: basePath,
		deniedPatterns: []string{
			".git/objects/*",
			".git/hooks/*",
			"node_modules/*",
			"*.env",
			"*.pem",
			"*.key",
			"*credentials*",
			"*secrets*",
		},
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

func (w *HostWorkspace) Initialize(ctx context.Context) error {
	absPath, err := filepath.Abs(w.basePath)
	if err != nil {
		return fmt.Errorf("failed to resolve workspace path: %w", err)
	}
	w.basePath = absPath

	if _, err := os.Stat(w.basePath); os.IsNotExist(err) {
		if err := os.MkdirAll(w.basePath, 0755); err != nil {
			return fmt.Errorf("failed to create workspace directory: %w", err)
		}
	}

	w.initialized = true
	return nil
}

func (w *HostWorkspace) Path() string {
	return w.basePath
}

func (w *HostWorkspace) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if !w.initialized {
		return nil, fmt.Errorf("workspace not initialized")
	}

	fullPath, err := w.resolvePath(path)
	if err != nil {
		return nil, err
	}

	if err := w.checkAccess(fullPath); err != nil {
		return nil, err
	}

	return os.ReadFile(fullPath)
}

func (w *HostWorkspace) WriteFile(ctx context.Context, path string, data []byte) error {
	if !w.initialized {
		return fmt.Errorf("workspace not initialized")
	}

	fullPath, err := w.resolvePath(path)
	if err != nil {
		return err
	}

	if err := w.checkAccess(fullPath); err != nil {
		return err
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(fullPath, data, 0644)
}

func (w *HostWorkspace) DeleteFile(ctx context.Context, path string) error {
	if !w.initialized {
		return fmt.Errorf("workspace not initialized")
	}

	fullPath, err := w.resolvePath(path)
	if err != nil {
		return err
	}

	if err := w.checkAccess(fullPath); err != nil {
		return err
	}

	return os.Remove(fullPath)
}

func (w *HostWorkspace) ListFiles(ctx context.Context, dir string) ([]string, error) {
	if !w.initialized {
		return nil, fmt.Errorf("workspace not initialized")
	}

	fullPath, err := w.resolvePath(dir)
	if err != nil {
		return nil, err
	}

	if err := w.checkAccess(fullPath); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		files = append(files, entry.Name())
	}

	return files, nil
}

func (w *HostWorkspace) Exists(ctx context.Context, path string) (bool, error) {
	if !w.initialized {
		return false, fmt.Errorf("workspace not initialized")
	}

	fullPath, err := w.resolvePath(path)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

func (w *HostWorkspace) Close(ctx context.Context) error {
	w.initialized = false
	return nil
}

func (w *HostWorkspace) resolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	fullPath := filepath.Join(w.basePath, path)
	fullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	if !strings.HasPrefix(fullPath, w.basePath) {
		return "", fmt.Errorf("path traversal detected: %s is outside workspace", path)
	}

	return fullPath, nil
}

func (w *HostWorkspace) checkAccess(path string) error {
	relPath, err := filepath.Rel(w.basePath, path)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	for _, pattern := range w.deniedPatterns {
		matched, _ := filepath.Match(pattern, relPath)
		if matched {
			return fmt.Errorf("access denied: path matches denied pattern %s", pattern)
		}

		if strings.Contains(pattern, "*") {
			dir := filepath.Dir(relPath)
			for dir != "." && dir != "/" {
				matched, _ = filepath.Match(pattern, dir)
				if matched {
					return fmt.Errorf("access denied: path is under denied directory pattern %s", pattern)
				}
				dir = filepath.Dir(dir)
			}
		}
	}

	if len(w.allowedPaths) > 0 {
		allowed := false
		for _, allowedPath := range w.allowedPaths {
			if strings.HasPrefix(relPath, allowedPath) || relPath == allowedPath {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("access denied: path not in allowed paths")
		}
	}

	return nil
}

func (w *HostWorkspace) IsInWorkspace(path string) bool {
	fullPath, err := w.resolvePath(path)
	if err != nil {
		return false
	}
	return strings.HasPrefix(fullPath, w.basePath)
}
