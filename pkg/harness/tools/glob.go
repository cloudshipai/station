package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"station/pkg/harness/sandbox"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

const (
	maxGlobResults = 100
	globTimeout    = 60 * time.Second
)

type GlobInput struct {
	Pattern string `json:"pattern" jsonschema:"description=Glob pattern to match files (e.g., '**/*.go', 'src/**/*.ts')"`
	Path    string `json:"path,omitempty" jsonschema:"description=Base directory to search from (default: workspace root)"`
}

type GlobOutput struct {
	Files     []string `json:"files"`
	Count     int      `json:"count"`
	Truncated bool     `json:"truncated,omitempty"`
}

func NewGlobTool(genkitApp *genkit.Genkit, workspacePath string) ai.Tool {
	return NewGlobToolWithSandbox(genkitApp, workspacePath, nil)
}

func NewGlobToolWithSandbox(genkitApp *genkit.Genkit, workspacePath string, sb sandbox.Sandbox) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"glob",
		`Find files matching a glob pattern.

Supports patterns like:
- "**/*.go" - all Go files recursively
- "src/**/*.ts" - TypeScript files under src/
- "*.json" - JSON files in current directory
- "test_*.py" - Python test files

Results are sorted by modification time (newest first).
Limited to 100 results with 60s timeout.`,
		func(ctx *ai.ToolContext, input GlobInput) (GlobOutput, error) {
			if input.Pattern == "" {
				return GlobOutput{}, fmt.Errorf("pattern is required")
			}

			basePath := workspacePath
			if input.Path != "" {
				if filepath.IsAbs(input.Path) {
					basePath = input.Path
				} else {
					basePath = filepath.Join(workspacePath, input.Path)
				}
			}

			var matches []fileInfo
			pattern := input.Pattern
			hasDoublestar := strings.Contains(pattern, "**")

			if sb != nil {
				matches = globViaSandbox(ctx, sb, basePath, pattern, hasDoublestar)
			} else {
				matches = globDirect(basePath, pattern, hasDoublestar)
			}

			sort.Slice(matches, func(i, j int) bool {
				return matches[i].modTime.After(matches[j].modTime)
			})

			truncated := false
			if len(matches) > maxGlobResults {
				matches = matches[:maxGlobResults]
				truncated = true
			}

			files := make([]string, len(matches))
			for i, m := range matches {
				files[i] = m.path
			}

			return GlobOutput{
				Files:     files,
				Count:     len(files),
				Truncated: truncated,
			}, nil
		},
	)
}

func globViaSandbox(ctx *ai.ToolContext, sb sandbox.Sandbox, basePath, pattern string, hasDoublestar bool) []fileInfo {
	var matches []fileInfo
	walkSandboxDir(ctx, sb, basePath, basePath, pattern, hasDoublestar, &matches)
	return matches
}

func walkSandboxDir(ctx *ai.ToolContext, sb sandbox.Sandbox, basePath, currentPath, pattern string, hasDoublestar bool, matches *[]fileInfo) {
	files, err := sb.ListFiles(ctx, currentPath)
	if err != nil {
		return
	}

	for _, f := range files {
		fullPath := filepath.Join(currentPath, f.Name)
		relPath, _ := filepath.Rel(basePath, fullPath)

		if f.IsDir {
			walkSandboxDir(ctx, sb, basePath, fullPath, pattern, hasDoublestar, matches)
			continue
		}

		var matched bool
		if hasDoublestar {
			matched = matchDoublestar(pattern, relPath)
		} else {
			matched, _ = filepath.Match(pattern, filepath.Base(relPath))
		}

		if matched {
			*matches = append(*matches, fileInfo{
				path:    relPath,
				modTime: f.ModTime,
			})
		}
	}
}

func globDirect(basePath, pattern string, hasDoublestar bool) []fileInfo {
	var matches []fileInfo

	filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return nil
		}

		var matched bool
		if hasDoublestar {
			matched = matchDoublestar(pattern, relPath)
		} else {
			matched, _ = filepath.Match(pattern, filepath.Base(relPath))
		}

		if matched {
			matches = append(matches, fileInfo{
				path:    relPath,
				modTime: info.ModTime(),
			})
		}

		return nil
	})

	return matches
}

type fileInfo struct {
	path    string
	modTime time.Time
}

func matchDoublestar(pattern, path string) bool {
	patternParts := strings.Split(pattern, "**")
	if len(patternParts) == 1 {
		matched, _ := filepath.Match(pattern, path)
		return matched
	}

	if len(patternParts) == 2 {
		prefix := strings.TrimSuffix(patternParts[0], "/")
		suffix := strings.TrimPrefix(patternParts[1], "/")

		if prefix != "" && !strings.HasPrefix(path, prefix) {
			return false
		}

		if suffix != "" {
			matched, _ := filepath.Match(suffix, filepath.Base(path))
			return matched
		}

		return true
	}

	return false
}
