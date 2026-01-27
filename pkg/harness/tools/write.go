package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"station/pkg/harness/sandbox"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// sensitiveFilePatterns defines patterns for files that should have restricted permissions
var sensitiveFilePatterns = []string{
	".env", ".env.local", ".env.production", ".env.development",
	".secret", ".secrets", "secrets.yaml", "secrets.json",
	"credentials", "credentials.json", "credentials.yaml",
	".key", ".pem", ".crt", ".p12", ".pfx",
	"id_rsa", "id_ed25519", "id_ecdsa",
	".htpasswd", ".pgpass", ".netrc",
}

// isSensitiveFile checks if a filename matches sensitive patterns
func isSensitiveFile(filename string) bool {
	base := filepath.Base(filename)
	lower := strings.ToLower(base)

	for _, pattern := range sensitiveFilePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return strings.Contains(lower, "secret") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "credential") ||
		strings.Contains(lower, "private")
}

type WriteInput struct {
	FilePath string `json:"file_path" jsonschema:"description=Absolute or relative path to the file to write"`
	Content  string `json:"content" jsonschema:"description=Content to write to the file"`
}

type WriteOutput struct {
	Success bool   `json:"success"`
	Path    string `json:"path"`
	Bytes   int    `json:"bytes"`
}

func NewWriteTool(genkitApp *genkit.Genkit, workspacePath string) ai.Tool {
	return NewWriteToolWithSandbox(genkitApp, workspacePath, nil)
}

func NewWriteToolWithSandbox(genkitApp *genkit.Genkit, workspacePath string, sb sandbox.Sandbox) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"write",
		`Write content to a file, creating it if it doesn't exist or overwriting if it does.

IMPORTANT:
- Creates parent directories automatically if they don't exist
- Overwrites existing files without warning
- Use edit tool for making changes to existing files`,
		func(ctx *ai.ToolContext, input WriteInput) (WriteOutput, error) {
			if input.FilePath == "" {
				return WriteOutput{}, fmt.Errorf("file_path is required")
			}

			path := input.FilePath
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspacePath, path)
			}

			// Validate path is within workspace (no sandbox means host execution)
			if sb == nil {
				validatedPath, err := ValidatePathForWrite(path, workspacePath)
				if err != nil {
					return WriteOutput{}, fmt.Errorf("path validation failed: %w", err)
				}
				path = validatedPath
			}

			// Determine file permissions based on sensitivity
			var perm uint32 = 0644
			if isSensitiveFile(path) {
				perm = 0600 // Restricted permissions for sensitive files
			}

			if sb != nil {
				dir := filepath.Dir(path)
				sb.Exec(ctx, "mkdir", "-p", dir)

				if err := sb.WriteFile(ctx, path, []byte(input.Content), perm); err != nil {
					return WriteOutput{}, fmt.Errorf("failed to write file: %w", err)
				}
			} else {
				dir := filepath.Dir(path)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return WriteOutput{}, fmt.Errorf("failed to create directory: %w", err)
				}

				if err := os.WriteFile(path, []byte(input.Content), os.FileMode(perm)); err != nil {
					return WriteOutput{}, fmt.Errorf("failed to write file: %w", err)
				}
			}

			return WriteOutput{
				Success: true,
				Path:    path,
				Bytes:   len(input.Content),
			}, nil
		},
	)
}
