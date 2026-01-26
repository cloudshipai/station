package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"station/pkg/harness/sandbox"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

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

			if sb != nil {
				dir := filepath.Dir(path)
				sb.Exec(ctx, "mkdir", "-p", dir)

				if err := sb.WriteFile(ctx, path, []byte(input.Content), 0644); err != nil {
					return WriteOutput{}, fmt.Errorf("failed to write file: %w", err)
				}
			} else {
				dir := filepath.Dir(path)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return WriteOutput{}, fmt.Errorf("failed to create directory: %w", err)
				}

				if err := os.WriteFile(path, []byte(input.Content), 0644); err != nil {
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
