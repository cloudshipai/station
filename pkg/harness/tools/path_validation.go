package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidatePath ensures a path is within the workspace and resolves to a clean path.
// Returns the validated absolute path or an error if the path is outside the workspace.
func ValidatePath(path, workspacePath string) (string, error) {
	if path == "" {
		return workspacePath, nil
	}

	// Make path absolute if relative
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(workspacePath, path)
	}

	// Clean the path to resolve . and ..
	cleanPath := filepath.Clean(absPath)

	// Clean the workspace path as well
	cleanWorkspace := filepath.Clean(workspacePath)

	// Ensure the path is within the workspace
	if !strings.HasPrefix(cleanPath, cleanWorkspace) {
		return "", fmt.Errorf("path %q is outside workspace %q", path, workspacePath)
	}

	// Additional check for symlink attacks could go here

	return cleanPath, nil
}

// ValidatePathForWrite validates a path for write operations.
// This is the same as ValidatePath but named explicitly for write operations.
func ValidatePathForWrite(path, workspacePath string) (string, error) {
	return ValidatePath(path, workspacePath)
}
