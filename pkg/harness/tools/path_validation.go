package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PathValidationError represents a path validation failure
type PathValidationError struct {
	Path   string
	Reason string
}

func (e *PathValidationError) Error() string {
	return fmt.Sprintf("path validation failed for %q: %s", e.Path, e.Reason)
}

// ValidatePath ensures a file path is within the workspace and doesn't escape via traversal
func ValidatePath(path, workspacePath string) (string, error) {
	if path == "" {
		return "", &PathValidationError{Path: path, Reason: "path is empty"}
	}

	// Resolve to absolute path
	var absPath string
	if filepath.IsAbs(path) {
		absPath = path
	} else {
		absPath = filepath.Join(workspacePath, path)
	}

	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(absPath)

	// Get clean workspace path
	cleanWorkspace := filepath.Clean(workspacePath)

	// Ensure the clean path is within workspace
	if !strings.HasPrefix(cleanPath, cleanWorkspace+string(filepath.Separator)) && cleanPath != cleanWorkspace {
		return "", &PathValidationError{
			Path:   path,
			Reason: fmt.Sprintf("path escapes workspace (resolved to %q)", cleanPath),
		}
	}

	// Check for suspicious patterns that might indicate attempted traversal
	if strings.Contains(path, "..") {
		// Even if it resolves within workspace, log/warn about traversal attempts
		// The cleaned path is still returned if valid
	}

	return cleanPath, nil
}

// ValidatePathForRead validates a path for read operations
func ValidatePathForRead(path, workspacePath string) (string, error) {
	return ValidatePath(path, workspacePath)
}

// ValidatePathForWrite validates a path for write operations
// Additional checks for write-specific concerns
func ValidatePathForWrite(path, workspacePath string) (string, error) {
	validPath, err := ValidatePath(path, workspacePath)
	if err != nil {
		return "", err
	}

	// Block writing to hidden config files that could be dangerous
	basename := filepath.Base(validPath)
	dangerousFiles := []string{
		".bashrc", ".bash_profile", ".profile", ".zshrc",
		".ssh/authorized_keys", ".ssh/config",
		".gitconfig",
	}

	for _, dangerous := range dangerousFiles {
		if basename == filepath.Base(dangerous) {
			return "", &PathValidationError{
				Path:   path,
				Reason: fmt.Sprintf("writing to %q is not allowed", basename),
			}
		}
	}

	return validPath, nil
}

// IsSafePath checks if a path is safe without returning an error
func IsSafePath(path, workspacePath string) bool {
	_, err := ValidatePath(path, workspacePath)
	return err == nil
}
