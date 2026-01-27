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

var sensitiveFilePatterns = []string{
	".env", ".env.*", "*.key", "*.pem", "*.crt", "*.p12",
	"*secret*", "*credential*", "*password*", "*.token",
	"id_rsa", "id_ed25519", "id_ecdsa",
}

func isSensitiveFile(path string) bool {
	basename := filepath.Base(path)
	lowerBasename := strings.ToLower(basename)
	for _, pattern := range sensitiveFilePatterns {
		if matched, _ := filepath.Match(strings.ToLower(pattern), lowerBasename); matched {
			return true
		}
		if strings.Contains(pattern, "*") {
			cleanPattern := strings.ReplaceAll(pattern, "*", "")
			if strings.Contains(lowerBasename, cleanPattern) {
				return true
			}
		}
	}
	return false
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

			path, err := ValidatePathForWrite(input.FilePath, workspacePath)
			if err != nil {
				return WriteOutput{}, err
			}

			var fileMode os.FileMode = 0644
			if isSensitiveFile(path) {
				fileMode = 0600
			}

			if sb != nil {
				dir := filepath.Dir(path)
				sb.Exec(ctx, "mkdir", "-p", dir)

				if err := sb.WriteFile(ctx, path, []byte(input.Content), uint32(fileMode)); err != nil {
					return WriteOutput{}, fmt.Errorf("failed to write file: %w", err)
				}
			} else {
				dir := filepath.Dir(path)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return WriteOutput{}, fmt.Errorf("failed to create directory: %w", err)
				}

				if err := os.WriteFile(path, []byte(input.Content), fileMode); err != nil {
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
