package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

const (
	defaultReadLimit = 2000
	maxLineLength    = 2000
)

type ReadInput struct {
	FilePath string `json:"file_path" jsonschema:"description=Absolute or relative path to the file to read"`
	Offset   int    `json:"offset,omitempty" jsonschema:"description=Line number to start reading from (0-based, default 0)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"description=Maximum number of lines to read (default 2000)"`
}

type ReadOutput struct {
	Content    string `json:"content"`
	TotalLines int    `json:"total_lines"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
}

func NewReadTool(genkitApp *genkit.Genkit, workspacePath string) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"read",
		`Read a file from the filesystem.

Returns file content with line numbers (cat -n format).
By default reads up to 2000 lines from the beginning.
Use offset and limit for reading specific sections of large files.
Lines longer than 2000 characters are truncated.`,
		func(ctx *ai.ToolContext, input ReadInput) (ReadOutput, error) {
			if input.FilePath == "" {
				return ReadOutput{}, fmt.Errorf("file_path is required")
			}

			path := input.FilePath
			if !filepath.IsAbs(path) {
				path = filepath.Join(workspacePath, path)
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return ReadOutput{}, fmt.Errorf("failed to read file: %w", err)
			}

			lines := strings.Split(string(content), "\n")
			totalLines := len(lines)

			offset := input.Offset
			if offset < 0 {
				offset = 0
			}
			if offset >= totalLines {
				return ReadOutput{
					Content:    "",
					TotalLines: totalLines,
					StartLine:  offset,
					EndLine:    offset,
				}, nil
			}

			limit := input.Limit
			if limit <= 0 {
				limit = defaultReadLimit
			}

			endLine := offset + limit
			if endLine > totalLines {
				endLine = totalLines
			}

			var result strings.Builder
			for i := offset; i < endLine; i++ {
				line := lines[i]
				if len(line) > maxLineLength {
					line = line[:maxLineLength] + "..."
				}
				result.WriteString(fmt.Sprintf("%6d\t%s\n", i+1, line))
			}

			return ReadOutput{
				Content:    result.String(),
				TotalLines: totalLines,
				StartLine:  offset + 1,
				EndLine:    endLine,
			}, nil
		},
	)
}
