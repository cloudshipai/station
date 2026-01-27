package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"station/pkg/harness/sandbox"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

const (
	maxGrepMatches   = 100
	maxGrepFileSize  = 10 * 1024 * 1024
	grepContextLines = 2
)

type GrepInput struct {
	Pattern string `json:"pattern" jsonschema:"description=Regular expression pattern to search for"`
	Path    string `json:"path,omitempty" jsonschema:"description=Directory to search in (default: workspace root)"`
	Include string `json:"include,omitempty" jsonschema:"description=File pattern to include (e.g., '*.go', '*.{ts,tsx}')"`
}

type GrepMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

type GrepOutput struct {
	Matches   []GrepMatch `json:"matches"`
	Count     int         `json:"count"`
	Truncated bool        `json:"truncated,omitempty"`
}

func NewGrepTool(genkitApp *genkit.Genkit, workspacePath string) ai.Tool {
	return NewGrepToolWithSandbox(genkitApp, workspacePath, nil)
}

func NewGrepToolWithSandbox(genkitApp *genkit.Genkit, workspacePath string, sb sandbox.Sandbox) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"grep",
		`Search file contents using regular expressions.

Supports full regex syntax:
- "log.*Error" - matches "logError", "log_Error", etc.
- "function\s+\w+" - matches function definitions
- "TODO|FIXME" - matches either word

Filter files with the include parameter:
- "*.go" - Go files only
- "*.{ts,tsx}" - TypeScript files

Returns file paths with matches sorted by modification time.
Limited to 100 matches.`,
		func(ctx *ai.ToolContext, input GrepInput) (GrepOutput, error) {
			if input.Pattern == "" {
				return GrepOutput{}, fmt.Errorf("pattern is required")
			}

			re, err := regexp.Compile(input.Pattern)
			if err != nil {
				return GrepOutput{}, fmt.Errorf("invalid regex pattern: %w", err)
			}

			basePath := workspacePath
			if input.Path != "" {
				if filepath.IsAbs(input.Path) {
					basePath = input.Path
				} else {
					basePath = filepath.Join(workspacePath, input.Path)
				}
			}

			var filesSearched []fileWithTime

			if sb != nil {
				filesSearched = collectFilesViaSandbox(ctx, sb, basePath, input.Include)
			} else {
				filesSearched = collectFilesDirect(basePath, input.Include)
			}

			sort.Slice(filesSearched, func(i, j int) bool {
				return filesSearched[i].modTime.After(filesSearched[j].modTime)
			})

			var matches []GrepMatch
			for _, f := range filesSearched {
				if len(matches) >= maxGrepMatches {
					break
				}

				var fileMatches []GrepMatch
				if sb != nil {
					fileMatches = searchFileViaSandbox(ctx, sb, f.path, re, basePath)
				} else {
					fileMatches, _ = searchFile(f.path, re, basePath)
				}

				for _, m := range fileMatches {
					if len(matches) >= maxGrepMatches {
						break
					}
					matches = append(matches, m)
				}
			}

			truncated := len(matches) >= maxGrepMatches

			return GrepOutput{
				Matches:   matches,
				Count:     len(matches),
				Truncated: truncated,
			}, nil
		},
	)
}

func collectFilesViaSandbox(ctx *ai.ToolContext, sb sandbox.Sandbox, basePath, include string) []fileWithTime {
	var files []fileWithTime
	walkSandboxDirForGrep(ctx, sb, basePath, basePath, include, &files)
	return files
}

func walkSandboxDirForGrep(ctx *ai.ToolContext, sb sandbox.Sandbox, basePath, currentPath, include string, files *[]fileWithTime) {
	entries, err := sb.ListFiles(ctx, currentPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir {
			if shouldSkipDir(entry.Name) {
				continue
			}
			walkSandboxDirForGrep(ctx, sb, basePath, filepath.Join(currentPath, entry.Name), include, files)
			continue
		}

		if entry.Size > maxGrepFileSize {
			continue
		}

		if include != "" && !matchInclude(include, entry.Name) {
			continue
		}

		*files = append(*files, fileWithTime{
			path:    filepath.Join(currentPath, entry.Name),
			modTime: entry.ModTime,
		})
	}
}

func searchFileViaSandbox(ctx *ai.ToolContext, sb sandbox.Sandbox, path string, re *regexp.Regexp, basePath string) []GrepMatch {
	content, err := sb.ReadFile(ctx, path)
	if err != nil {
		return nil
	}

	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		relPath = path
	}

	var matches []GrepMatch
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		if re.MatchString(line) {
			matches = append(matches, GrepMatch{
				File:    relPath,
				Line:    lineNum + 1,
				Content: truncateLine(line, 200),
			})
		}
	}

	return matches
}

func collectFilesDirect(basePath, include string) []fileWithTime {
	var files []fileWithTime

	filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if info.Size() > maxGrepFileSize {
			return nil
		}

		if include != "" && !matchInclude(include, info.Name()) {
			return nil
		}

		files = append(files, fileWithTime{
			path:    path,
			modTime: info.ModTime(),
		})

		return nil
	})

	return files
}

type fileWithTime struct {
	path    string
	modTime time.Time
}

func searchFile(path string, re *regexp.Regexp, basePath string) ([]GrepMatch, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		relPath = path
	}

	var matches []GrepMatch
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if re.MatchString(line) {
			matches = append(matches, GrepMatch{
				File:    relPath,
				Line:    lineNum,
				Content: truncateLine(line, 200),
			})
		}
	}

	return matches, scanner.Err()
}

func shouldSkipDir(name string) bool {
	skipDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		"__pycache__":  true,
		".venv":        true,
		"venv":         true,
		".idea":        true,
		".vscode":      true,
		"dist":         true,
		"build":        true,
		".next":        true,
		"coverage":     true,
	}
	return skipDirs[name]
}

func matchInclude(pattern, filename string) bool {
	if strings.Contains(pattern, "{") {
		start := strings.Index(pattern, "{")
		end := strings.Index(pattern, "}")
		if start != -1 && end != -1 && end > start {
			prefix := pattern[:start]
			suffix := pattern[end+1:]
			extensions := strings.Split(pattern[start+1:end], ",")

			for _, ext := range extensions {
				fullPattern := prefix + strings.TrimSpace(ext) + suffix
				if matched, _ := filepath.Match(fullPattern, filename); matched {
					return true
				}
			}
			return false
		}
	}

	matched, _ := filepath.Match(pattern, filename)
	return matched
}

func truncateLine(line string, maxLen int) string {
	if len(line) <= maxLen {
		return line
	}
	return line[:maxLen] + "..."
}
