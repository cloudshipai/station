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

type EditInput struct {
	FilePath   string `json:"file_path" jsonschema:"description=Absolute or relative path to the file to edit"`
	OldString  string `json:"old_string" jsonschema:"description=The exact text to replace (must match exactly including whitespace)"`
	NewString  string `json:"new_string" jsonschema:"description=The text to replace it with"`
	ReplaceAll bool   `json:"replace_all,omitempty" jsonschema:"description=Replace all occurrences instead of just the first (default false)"`
}

type EditOutput struct {
	Success      bool   `json:"success"`
	Path         string `json:"path"`
	Replacements int    `json:"replacements"`
}

func NewEditTool(genkitApp *genkit.Genkit, workspacePath string) ai.Tool {
	return NewEditToolWithSandbox(genkitApp, workspacePath, nil)
}

func NewEditToolWithSandbox(genkitApp *genkit.Genkit, workspacePath string, sb sandbox.Sandbox) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"edit",
		`Perform exact string replacements in files.

IMPORTANT:
- oldString must match EXACTLY (including indentation/whitespace)
- The edit will FAIL if oldString is not found
- The edit will FAIL if oldString is found multiple times (unless replaceAll is true)
- Use replaceAll=true for renaming variables across a file

Examples:
- Fix a typo: oldString="teh", newString="the"
- Rename variable: oldString="oldName", newString="newName", replaceAll=true`,
		func(ctx *ai.ToolContext, input EditInput) (EditOutput, error) {
			if input.FilePath == "" {
				return EditOutput{}, fmt.Errorf("file_path is required")
			}
			if input.OldString == "" {
				return EditOutput{}, fmt.Errorf("old_string is required")
			}
			if input.OldString == input.NewString {
				return EditOutput{}, fmt.Errorf("old_string and new_string must be different")
			}

			path := input.FilePath
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspacePath, path)
			}

			var content []byte
			var err error

			if sb != nil {
				content, err = sb.ReadFile(ctx, path)
			} else {
				content, err = os.ReadFile(path)
			}
			if err != nil {
				return EditOutput{}, fmt.Errorf("failed to read file: %w", err)
			}

			fileContent := string(content)
			count := strings.Count(fileContent, input.OldString)

			if count == 0 {
				return EditOutput{}, fmt.Errorf("oldString not found in file content")
			}

			if count > 1 && !input.ReplaceAll {
				return EditOutput{}, fmt.Errorf("oldString found %d times - requires more context to uniquely identify or set replaceAll=true", count)
			}

			var newContent string
			if input.ReplaceAll {
				newContent = strings.ReplaceAll(fileContent, input.OldString, input.NewString)
			} else {
				newContent = strings.Replace(fileContent, input.OldString, input.NewString, 1)
				count = 1
			}

			if sb != nil {
				err = sb.WriteFile(ctx, path, []byte(newContent), 0644)
			} else {
				err = os.WriteFile(path, []byte(newContent), 0644)
			}
			if err != nil {
				return EditOutput{}, fmt.Errorf("failed to write file: %w", err)
			}

			return EditOutput{
				Success:      true,
				Path:         path,
				Replacements: count,
			}, nil
		},
	)
}
