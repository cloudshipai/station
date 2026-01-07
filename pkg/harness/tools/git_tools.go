package tools

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

type GitStatusInput struct {
	Path string `json:"path,omitempty" jsonschema:"description=Subdirectory path relative to workspace (optional)"`
}

type GitStatusOutput struct {
	Branch    string   `json:"branch"`
	Ahead     int      `json:"ahead"`
	Behind    int      `json:"behind"`
	Staged    []string `json:"staged"`
	Modified  []string `json:"modified"`
	Untracked []string `json:"untracked"`
	Clean     bool     `json:"clean"`
	Output    string   `json:"output"`
}

func NewGitStatusTool(g *genkit.Genkit, workspacePath string) ai.Tool {
	return genkit.DefineTool(g, "git_status",
		"Get the current git status including branch, staged files, modified files, and untracked files",
		func(ctx *ai.ToolContext, input GitStatusInput) (GitStatusOutput, error) {
			repoPath := workspacePath
			if input.Path != "" {
				repoPath = filepath.Join(workspacePath, input.Path)
			}

			execCtx, cancel := context.WithTimeout(ctx, defaultBashTimeout)
			defer cancel()

			cmd := exec.CommandContext(execCtx, "git", "status", "--porcelain=v2", "--branch")
			cmd.Dir = repoPath
			output, err := cmd.Output()
			if err != nil {
				return GitStatusOutput{}, fmt.Errorf("git status failed: %w", err)
			}

			result := GitStatusOutput{
				Output: string(output),
			}

			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "# branch.head ") {
					result.Branch = strings.TrimPrefix(line, "# branch.head ")
				} else if strings.HasPrefix(line, "# branch.ab ") {
					parts := strings.Fields(line)
					if len(parts) >= 4 {
						fmt.Sscanf(parts[2], "+%d", &result.Ahead)
						fmt.Sscanf(parts[3], "-%d", &result.Behind)
					}
				} else if len(line) > 2 {
					xy := line[:2]
					path := ""
					if strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 ") {
						parts := strings.Fields(line)
						if len(parts) >= 9 {
							path = parts[8]
						}
					} else if strings.HasPrefix(line, "? ") {
						path = strings.TrimPrefix(line, "? ")
						result.Untracked = append(result.Untracked, path)
						continue
					}

					if path == "" {
						continue
					}

					if xy[0] != '.' && xy[0] != '?' {
						result.Staged = append(result.Staged, path)
					}
					if xy[1] != '.' && xy[1] != '?' {
						result.Modified = append(result.Modified, path)
					}
				}
			}

			result.Clean = len(result.Staged) == 0 && len(result.Modified) == 0 && len(result.Untracked) == 0

			return result, nil
		},
	)
}

type GitDiffInput struct {
	Path   string `json:"path,omitempty" jsonschema:"description=Subdirectory path relative to workspace (optional)"`
	Staged bool   `json:"staged,omitempty" jsonschema:"description=Show staged changes instead of working directory changes"`
	File   string `json:"file,omitempty" jsonschema:"description=Specific file to diff"`
}

type GitDiffOutput struct {
	Diff       string `json:"diff"`
	FilesCount int    `json:"files_count"`
	Insertions int    `json:"insertions"`
	Deletions  int    `json:"deletions"`
}

func NewGitDiffTool(g *genkit.Genkit, workspacePath string) ai.Tool {
	return genkit.DefineTool(g, "git_diff",
		"Show changes in the working directory or staged changes. Use staged=true to see staged changes.",
		func(ctx *ai.ToolContext, input GitDiffInput) (GitDiffOutput, error) {
			repoPath := workspacePath
			if input.Path != "" {
				repoPath = filepath.Join(workspacePath, input.Path)
			}

			execCtx, cancel := context.WithTimeout(ctx, defaultBashTimeout)
			defer cancel()

			args := []string{"diff"}
			if input.Staged {
				args = append(args, "--staged")
			}
			args = append(args, "--stat")

			if input.File != "" {
				args = append(args, "--", input.File)
			}

			cmd := exec.CommandContext(execCtx, "git", args...)
			cmd.Dir = repoPath
			stat, _ := cmd.Output()

			args = []string{"diff"}
			if input.Staged {
				args = append(args, "--staged")
			}
			if input.File != "" {
				args = append(args, "--", input.File)
			}

			cmd = exec.CommandContext(execCtx, "git", args...)
			cmd.Dir = repoPath
			diff, err := cmd.Output()
			if err != nil {
				return GitDiffOutput{}, fmt.Errorf("git diff failed: %w", err)
			}

			result := GitDiffOutput{
				Diff: string(diff),
			}

			statLines := strings.Split(string(stat), "\n")
			for _, line := range statLines {
				if strings.Contains(line, "files changed") || strings.Contains(line, "file changed") {
					fmt.Sscanf(line, " %d file", &result.FilesCount)
					if strings.Contains(line, "insertion") {
						fmt.Sscanf(strings.Split(line, ",")[1], " %d insertion", &result.Insertions)
					}
					if strings.Contains(line, "deletion") {
						parts := strings.Split(line, ",")
						for _, p := range parts {
							if strings.Contains(p, "deletion") {
								fmt.Sscanf(p, " %d deletion", &result.Deletions)
							}
						}
					}
				}
			}

			if len(result.Diff) > maxBashOutputLength {
				result.Diff = result.Diff[:maxBashOutputLength] + "\n... (truncated)"
			}

			return result, nil
		},
	)
}

type GitLogInput struct {
	Path  string `json:"path,omitempty" jsonschema:"description=Subdirectory path relative to workspace (optional)"`
	Count int    `json:"count,omitempty" jsonschema:"description=Number of commits to show (default 10, max 50)"`
	File  string `json:"file,omitempty" jsonschema:"description=Show commits affecting this file"`
}

type GitLogCommit struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
}

type GitLogOutput struct {
	Commits []GitLogCommit `json:"commits"`
}

func NewGitLogTool(g *genkit.Genkit, workspacePath string) ai.Tool {
	return genkit.DefineTool(g, "git_log",
		"Show recent commit history. Use count to limit results (default 10).",
		func(ctx *ai.ToolContext, input GitLogInput) (GitLogOutput, error) {
			repoPath := workspacePath
			if input.Path != "" {
				repoPath = filepath.Join(workspacePath, input.Path)
			}

			execCtx, cancel := context.WithTimeout(ctx, defaultBashTimeout)
			defer cancel()

			count := input.Count
			if count <= 0 {
				count = 10
			}
			if count > 50 {
				count = 50
			}

			format := "%H%n%an%n%ai%n%s%n---"
			args := []string{"log", fmt.Sprintf("-n%d", count), fmt.Sprintf("--format=%s", format)}

			if input.File != "" {
				args = append(args, "--", input.File)
			}

			cmd := exec.CommandContext(execCtx, "git", args...)
			cmd.Dir = repoPath
			output, err := cmd.Output()
			if err != nil {
				return GitLogOutput{}, fmt.Errorf("git log failed: %w", err)
			}

			result := GitLogOutput{}
			entries := strings.Split(string(output), "---\n")

			for _, entry := range entries {
				entry = strings.TrimSpace(entry)
				if entry == "" {
					continue
				}

				lines := strings.Split(entry, "\n")
				if len(lines) >= 4 {
					result.Commits = append(result.Commits, GitLogCommit{
						Hash:    lines[0],
						Author:  lines[1],
						Date:    lines[2],
						Subject: lines[3],
					})
				}
			}

			return result, nil
		},
	)
}

type GitAddInput struct {
	Path  string   `json:"path,omitempty" jsonschema:"description=Subdirectory path relative to workspace (optional)"`
	Files []string `json:"files,omitempty" jsonschema:"description=Specific files to stage"`
	All   bool     `json:"all,omitempty" jsonschema:"description=Stage all changes including untracked files"`
}

type GitAddOutput struct {
	Added   []string `json:"added"`
	Message string   `json:"message"`
}

func NewGitAddTool(g *genkit.Genkit, workspacePath string) ai.Tool {
	return genkit.DefineTool(g, "git_add",
		"Stage files for commit. Use files array for specific files, or all=true to stage all changes.",
		func(ctx *ai.ToolContext, input GitAddInput) (GitAddOutput, error) {
			repoPath := workspacePath
			if input.Path != "" {
				repoPath = filepath.Join(workspacePath, input.Path)
			}

			execCtx, cancel := context.WithTimeout(ctx, defaultBashTimeout)
			defer cancel()

			args := []string{"add"}
			if input.All {
				args = append(args, "-A")
			} else if len(input.Files) > 0 {
				args = append(args, input.Files...)
			} else {
				return GitAddOutput{}, fmt.Errorf("must specify files or all=true")
			}

			cmd := exec.CommandContext(execCtx, "git", args...)
			cmd.Dir = repoPath
			if err := cmd.Run(); err != nil {
				return GitAddOutput{}, fmt.Errorf("git add failed: %w", err)
			}

			result := GitAddOutput{
				Message: "Files staged successfully",
			}

			if input.All {
				result.Added = []string{"all changes"}
			} else {
				result.Added = input.Files
			}

			return result, nil
		},
	)
}

type GitCommitInput struct {
	Path    string `json:"path,omitempty" jsonschema:"description=Subdirectory path relative to workspace (optional)"`
	Message string `json:"message" jsonschema:"description=Commit message (required)"`
}

type GitCommitOutput struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Output  string `json:"output"`
}

func NewGitCommitTool(g *genkit.Genkit, workspacePath string) ai.Tool {
	return genkit.DefineTool(g, "git_commit",
		"Create a commit with staged changes. Requires a commit message.",
		func(ctx *ai.ToolContext, input GitCommitInput) (GitCommitOutput, error) {
			repoPath := workspacePath
			if input.Path != "" {
				repoPath = filepath.Join(workspacePath, input.Path)
			}

			if input.Message == "" {
				return GitCommitOutput{}, fmt.Errorf("commit message is required")
			}

			execCtx, cancel := context.WithTimeout(ctx, defaultBashTimeout)
			defer cancel()

			cmd := exec.CommandContext(execCtx, "git", "commit", "-m", input.Message)
			cmd.Dir = repoPath
			output, err := cmd.CombinedOutput()
			if err != nil {
				return GitCommitOutput{}, fmt.Errorf("git commit failed: %s", string(output))
			}

			cmd = exec.CommandContext(execCtx, "git", "rev-parse", "HEAD")
			cmd.Dir = repoPath
			hashBytes, _ := cmd.Output()

			return GitCommitOutput{
				Hash:    strings.TrimSpace(string(hashBytes)),
				Message: input.Message,
				Output:  string(output),
			}, nil
		},
	)
}

type GitBranchInput struct {
	Path       string `json:"path,omitempty" jsonschema:"description=Subdirectory path relative to workspace (optional)"`
	Name       string `json:"name,omitempty" jsonschema:"description=Branch name for create/checkout operations"`
	Create     bool   `json:"create,omitempty" jsonschema:"description=Create a new branch with the given name"`
	Checkout   bool   `json:"checkout,omitempty" jsonschema:"description=Switch to the branch with the given name"`
	ListRemote bool   `json:"list_remote,omitempty" jsonschema:"description=Include remote branches in listing"`
}

type GitBranch struct {
	Name    string `json:"name"`
	Current bool   `json:"current"`
	Remote  bool   `json:"remote"`
}

type GitBranchOutput struct {
	Branches []GitBranch `json:"branches"`
	Current  string      `json:"current"`
	Message  string      `json:"message,omitempty"`
}

func NewGitBranchTool(g *genkit.Genkit, workspacePath string) ai.Tool {
	return genkit.DefineTool(g, "git_branch",
		"List, create, or switch branches. Use name+create to create new branch, name+checkout to switch.",
		func(ctx *ai.ToolContext, input GitBranchInput) (GitBranchOutput, error) {
			repoPath := workspacePath
			if input.Path != "" {
				repoPath = filepath.Join(workspacePath, input.Path)
			}

			execCtx, cancel := context.WithTimeout(ctx, defaultBashTimeout)
			defer cancel()

			if input.Name != "" && input.Create {
				args := []string{"checkout", "-b", input.Name}
				cmd := exec.CommandContext(execCtx, "git", args...)
				cmd.Dir = repoPath
				if err := cmd.Run(); err != nil {
					return GitBranchOutput{}, fmt.Errorf("git branch create failed: %w", err)
				}
				return GitBranchOutput{
					Current: input.Name,
					Message: fmt.Sprintf("Created and switched to branch '%s'", input.Name),
				}, nil
			}

			if input.Name != "" && input.Checkout {
				cmd := exec.CommandContext(execCtx, "git", "checkout", input.Name)
				cmd.Dir = repoPath
				if err := cmd.Run(); err != nil {
					return GitBranchOutput{}, fmt.Errorf("git checkout failed: %w", err)
				}
				return GitBranchOutput{
					Current: input.Name,
					Message: fmt.Sprintf("Switched to branch '%s'", input.Name),
				}, nil
			}

			args := []string{"branch"}
			if input.ListRemote {
				args = append(args, "-a")
			}

			cmd := exec.CommandContext(execCtx, "git", args...)
			cmd.Dir = repoPath
			output, err := cmd.Output()
			if err != nil {
				return GitBranchOutput{}, fmt.Errorf("git branch list failed: %w", err)
			}

			result := GitBranchOutput{}
			lines := strings.Split(string(output), "\n")

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				branch := GitBranch{}
				if strings.HasPrefix(line, "* ") {
					branch.Current = true
					branch.Name = strings.TrimPrefix(line, "* ")
					result.Current = branch.Name
				} else {
					branch.Name = line
				}

				if strings.HasPrefix(branch.Name, "remotes/") {
					branch.Remote = true
					branch.Name = strings.TrimPrefix(branch.Name, "remotes/")
				}

				result.Branches = append(result.Branches, branch)
			}

			return result, nil
		},
	)
}

type GitResetInput struct {
	Path  string   `json:"path,omitempty" jsonschema:"description=Subdirectory path relative to workspace (optional)"`
	Files []string `json:"files,omitempty" jsonschema:"description=Specific files to unstage (empty = all)"`
}

type GitResetOutput struct {
	Reset   []string `json:"reset"`
	Message string   `json:"message"`
}

func NewGitResetTool(g *genkit.Genkit, workspacePath string) ai.Tool {
	return genkit.DefineTool(g, "git_reset",
		"Unstage files (soft reset). Specify files to unstage, or leave empty to unstage all.",
		func(ctx *ai.ToolContext, input GitResetInput) (GitResetOutput, error) {
			repoPath := workspacePath
			if input.Path != "" {
				repoPath = filepath.Join(workspacePath, input.Path)
			}

			execCtx, cancel := context.WithTimeout(ctx, defaultBashTimeout)
			defer cancel()

			args := []string{"reset", "HEAD"}
			if len(input.Files) > 0 {
				args = append(args, "--")
				args = append(args, input.Files...)
			}

			cmd := exec.CommandContext(execCtx, "git", args...)
			cmd.Dir = repoPath
			if err := cmd.Run(); err != nil {
				return GitResetOutput{}, fmt.Errorf("git reset failed: %w", err)
			}

			result := GitResetOutput{
				Message: "Files unstaged successfully",
			}

			if len(input.Files) > 0 {
				result.Reset = input.Files
			} else {
				result.Reset = []string{"all staged files"}
			}

			return result, nil
		},
	)
}

func (r *ToolRegistry) RegisterGitTools() {
	r.tools["git_status"] = NewGitStatusTool(r.genkitApp, r.workspacePath)
	r.tools["git_diff"] = NewGitDiffTool(r.genkitApp, r.workspacePath)
	r.tools["git_log"] = NewGitLogTool(r.genkitApp, r.workspacePath)
	r.tools["git_add"] = NewGitAddTool(r.genkitApp, r.workspacePath)
	r.tools["git_commit"] = NewGitCommitTool(r.genkitApp, r.workspacePath)
	r.tools["git_branch"] = NewGitBranchTool(r.genkitApp, r.workspacePath)
	r.tools["git_reset"] = NewGitResetTool(r.genkitApp, r.workspacePath)
}
