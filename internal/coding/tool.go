package coding

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
)

type ToolFactory struct {
	backend          Backend
	workspaceManager *WorkspaceManager
}

type ToolFactoryOption func(*ToolFactory)

func WithWorkspaceManager(wm *WorkspaceManager) ToolFactoryOption {
	return func(f *ToolFactory) {
		f.workspaceManager = wm
	}
}

func NewToolFactory(backend Backend, opts ...ToolFactoryOption) *ToolFactory {
	f := &ToolFactory{backend: backend}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

type CodeInput struct {
	SessionID   string   `json:"session_id"`
	Instruction string   `json:"instruction"`
	Context     string   `json:"context,omitempty"`
	Files       []string `json:"files,omitempty"`
}

type CodeOutput struct {
	Success      bool         `json:"success"`
	Summary      string       `json:"summary"`
	FilesChanged []FileChange `json:"files_changed,omitempty"`
	Error        string       `json:"error,omitempty"`
	TokensUsed   int          `json:"tokens_used,omitempty"`
	Cost         float64      `json:"cost,omitempty"`
}

func (f *ToolFactory) CreateCodeTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "Coding session ID from coding_open",
			},
			"instruction": map[string]any{
				"type":        "string",
				"description": "What you want the coding assistant to do, e.g. 'Fix the null pointer exception in auth.go'",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "Additional context about the task (optional)",
			},
			"files": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Specific files to focus on (optional)",
			},
		},
		"required": []string{"session_id", "instruction"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("code: expected map input")
		}

		sessionID, _ := inputMap["session_id"].(string)
		instruction, _ := inputMap["instruction"].(string)
		context, _ := inputMap["context"].(string)

		if sessionID == "" {
			return nil, fmt.Errorf("code: session_id is required")
		}
		if instruction == "" {
			return nil, fmt.Errorf("code: instruction is required")
		}

		var files []string
		if filesRaw, ok := inputMap["files"].([]any); ok {
			for _, f := range filesRaw {
				if s, ok := f.(string); ok {
					files = append(files, s)
				}
			}
		}

		task := Task{
			Instruction: instruction,
			Context:     context,
			Files:       files,
		}

		result, err := f.backend.Execute(toolCtx.Context, sessionID, task)
		if err != nil {
			return nil, fmt.Errorf("code: %w", err)
		}

		output := CodeOutput{
			Success:      result.Success,
			Summary:      result.Summary,
			FilesChanged: result.FilesChanged,
			Error:        result.Error,
		}

		if result.Trace != nil {
			output.TokensUsed = result.Trace.Tokens.Total()
			output.Cost = result.Trace.Cost
		}

		return output, nil
	}

	return ai.NewToolWithInputSchema(
		"code",
		"Delegate a coding task to the AI coding assistant. Use for complex code changes, bug fixes, or feature implementations.",
		schema,
		toolFunc,
	)
}

type CodingOpenInput struct {
	WorkspacePath string `json:"workspace_path,omitempty"`
	Title         string `json:"title,omitempty"`
	Scope         string `json:"scope,omitempty"`
	ScopeID       string `json:"scope_id,omitempty"`
}

type CodingOpenOutput struct {
	SessionID     string `json:"session_id"`
	WorkspacePath string `json:"workspace_path"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	Managed       bool   `json:"managed"`
}

func (f *ToolFactory) CreateOpenTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"workspace_path": map[string]any{
				"type":        "string",
				"description": "Path to workspace. If omitted and WorkspaceManager is configured, a managed workspace is created automatically.",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Optional session title for identification",
			},
			"scope": map[string]any{
				"type":        "string",
				"enum":        []string{"agent", "workflow"},
				"description": "Workspace scope: 'agent' (cleanup after session) or 'workflow' (persist across steps). Default: agent",
			},
			"scope_id": map[string]any{
				"type":        "string",
				"description": "Scope identifier (workflow ID for workflow scope, auto-generated for agent scope)",
			},
		},
		"required": []string{},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("coding_open: expected map input")
		}

		workspacePath, _ := inputMap["workspace_path"].(string)
		title, _ := inputMap["title"].(string)
		scopeStr, _ := inputMap["scope"].(string)
		scopeID, _ := inputMap["scope_id"].(string)

		var workspaceID string
		managed := false

		if workspacePath == "" {
			if f.workspaceManager == nil {
				return nil, fmt.Errorf("coding_open: workspace_path is required (no WorkspaceManager configured)")
			}

			scope := ScopeAgent
			if scopeStr == "workflow" {
				scope = ScopeWorkflow
			}

			if scopeID == "" {
				scopeID = fmt.Sprintf("session_%d", time.Now().UnixNano())
			}

			ws, err := f.workspaceManager.Create(toolCtx.Context, scope, scopeID)
			if err != nil {
				return nil, fmt.Errorf("coding_open: create workspace: %w", err)
			}

			if err := f.workspaceManager.InitGit(toolCtx.Context, ws); err != nil {
				return nil, fmt.Errorf("coding_open: init git: %w", err)
			}

			workspacePath = ws.Path
			workspaceID = ws.ID
			managed = true
		}

		session, err := f.backend.CreateSession(toolCtx.Context, workspacePath, title)
		if err != nil {
			return nil, fmt.Errorf("coding_open: %w", err)
		}

		return CodingOpenOutput{
			SessionID:     session.ID,
			WorkspacePath: session.WorkspacePath,
			WorkspaceID:   workspaceID,
			Managed:       managed,
		}, nil
	}

	return ai.NewToolWithInputSchema(
		"coding_open",
		"Open a coding session. Provide workspace_path for existing repos, or omit for auto-managed workspace.",
		schema,
		toolFunc,
	)
}

type CodingCloseInput struct {
	SessionID   string `json:"session_id"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Success     bool   `json:"success,omitempty"`
}

type CodingCloseOutput struct {
	OK           bool         `json:"ok"`
	FilesChanged []FileChange `json:"files_changed,omitempty"`
	CleanedUp    bool         `json:"cleaned_up"`
}

func (f *ToolFactory) CreateCloseTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "Coding session ID from coding_open",
			},
			"workspace_id": map[string]any{
				"type":        "string",
				"description": "Managed workspace ID from coding_open (if managed)",
			},
			"success": map[string]any{
				"type":        "boolean",
				"description": "Whether the coding task succeeded (affects cleanup policy). Default: true",
			},
		},
		"required": []string{"session_id"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("coding_close: expected map input")
		}

		sessionID, _ := inputMap["session_id"].(string)
		workspaceID, _ := inputMap["workspace_id"].(string)
		success := true
		if v, ok := inputMap["success"].(bool); ok {
			success = v
		}

		if sessionID == "" {
			return nil, fmt.Errorf("coding_close: session_id is required")
		}

		output := CodingCloseOutput{OK: true}

		if workspaceID != "" && f.workspaceManager != nil {
			ws, err := f.workspaceManager.Get(workspaceID)
			if err == nil {
				changes, _ := f.workspaceManager.CollectChanges(toolCtx.Context, ws)
				output.FilesChanged = changes

				wsPath := ws.Path
				f.workspaceManager.CleanupByPolicy(toolCtx.Context, ws, success)

				if _, statErr := os.Stat(wsPath); os.IsNotExist(statErr) {
					output.CleanedUp = true
				}
			}
		}

		if err := f.backend.CloseSession(toolCtx.Context, sessionID); err != nil {
			return nil, fmt.Errorf("coding_close: %w", err)
		}

		return output, nil
	}

	return ai.NewToolWithInputSchema(
		"coding_close",
		"Close a coding session. Collects file changes and cleans up managed workspaces based on policy.",
		schema,
		toolFunc,
	)
}

type CodingCommitInput struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	AddAll    bool   `json:"add_all,omitempty"`
}

func (f *ToolFactory) CreateCommitTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "Coding session ID from coding_open",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "Commit message describing the changes",
			},
			"add_all": map[string]any{
				"type":        "boolean",
				"description": "Whether to stage all changes before committing (git add -A). Default: true",
			},
		},
		"required": []string{"session_id", "message"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("coding_commit: expected map input")
		}

		sessionID, _ := inputMap["session_id"].(string)
		message, _ := inputMap["message"].(string)
		addAll := true
		if v, ok := inputMap["add_all"].(bool); ok {
			addAll = v
		}

		if sessionID == "" {
			return nil, fmt.Errorf("coding_commit: session_id is required")
		}
		if message == "" {
			return nil, fmt.Errorf("coding_commit: message is required")
		}

		session, err := f.backend.GetSession(toolCtx.Context, sessionID)
		if err != nil {
			return nil, fmt.Errorf("coding_commit: %w", err)
		}

		workDir := session.WorkspacePath
		if !filepath.IsAbs(workDir) {
			return nil, fmt.Errorf("coding_commit: workspace path must be absolute: %s", workDir)
		}

		if addAll {
			addCmd := exec.CommandContext(toolCtx.Context, "git", "add", "-A")
			addCmd.Dir = workDir
			if out, err := addCmd.CombinedOutput(); err != nil {
				return GitCommitResult{
					Success: false,
					Message: message,
					Error:   fmt.Sprintf("git add failed: %s: %s", err, string(out)),
				}, nil
			}
		}

		var stdout, stderr bytes.Buffer
		commitCmd := exec.CommandContext(toolCtx.Context, "git", "commit", "-m", message)
		commitCmd.Dir = workDir
		commitCmd.Stdout = &stdout
		commitCmd.Stderr = &stderr

		if err := commitCmd.Run(); err != nil {
			combined := strings.TrimSpace(stdout.String() + stderr.String())
			if strings.Contains(combined, "nothing to commit") {
				return GitCommitResult{
					Success: true,
					Message: message,
					Error:   "nothing to commit, working tree clean",
				}, nil
			}
			return GitCommitResult{
				Success: false,
				Message: message,
				Error:   fmt.Sprintf("git commit failed: %s: %s", err, combined),
			}, nil
		}

		result := GitCommitResult{
			Success: true,
			Message: message,
		}

		hashCmd := exec.CommandContext(toolCtx.Context, "git", "rev-parse", "HEAD")
		hashCmd.Dir = workDir
		if hashOut, err := hashCmd.Output(); err == nil {
			result.CommitHash = strings.TrimSpace(string(hashOut))
		}

		result.FilesChanged, result.Insertions, result.Deletions = parseGitCommitStats(stdout.String())

		return result, nil
	}

	return ai.NewToolWithInputSchema(
		"coding_commit",
		"Commit changes in the coding session workspace. Stages all changes by default.",
		schema,
		toolFunc,
	)
}

func parseGitCommitStats(output string) (files, insertions, deletions int) {
	re := regexp.MustCompile(`(\d+) files? changed(?:, (\d+) insertions?\(\+\))?(?:, (\d+) deletions?\(-\))?`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		files, _ = strconv.Atoi(matches[1])
	}
	if len(matches) >= 3 && matches[2] != "" {
		insertions, _ = strconv.Atoi(matches[2])
	}
	if len(matches) >= 4 && matches[3] != "" {
		deletions, _ = strconv.Atoi(matches[3])
	}
	return
}

type CodingPushInput struct {
	SessionID   string `json:"session_id"`
	Remote      string `json:"remote,omitempty"`
	Branch      string `json:"branch,omitempty"`
	SetUpstream bool   `json:"set_upstream,omitempty"`
}

func (f *ToolFactory) CreatePushTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "Coding session ID from coding_open",
			},
			"remote": map[string]any{
				"type":        "string",
				"description": "Remote name (default: origin)",
			},
			"branch": map[string]any{
				"type":        "string",
				"description": "Branch to push (default: current branch)",
			},
			"set_upstream": map[string]any{
				"type":        "boolean",
				"description": "Set upstream tracking (-u flag). Default: false",
			},
		},
		"required": []string{"session_id"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("coding_push: expected map input")
		}

		sessionID, _ := inputMap["session_id"].(string)
		remote, _ := inputMap["remote"].(string)
		branch, _ := inputMap["branch"].(string)
		setUpstream, _ := inputMap["set_upstream"].(bool)

		if sessionID == "" {
			return nil, fmt.Errorf("coding_push: session_id is required")
		}

		session, err := f.backend.GetSession(toolCtx.Context, sessionID)
		if err != nil {
			return nil, fmt.Errorf("coding_push: %w", err)
		}

		workDir := session.WorkspacePath
		if !filepath.IsAbs(workDir) {
			return nil, fmt.Errorf("coding_push: workspace path must be absolute: %s", workDir)
		}

		if remote == "" {
			remote = "origin"
		}

		if branch == "" {
			branchCmd := exec.CommandContext(toolCtx.Context, "git", "rev-parse", "--abbrev-ref", "HEAD")
			branchCmd.Dir = workDir
			if out, err := branchCmd.Output(); err == nil {
				branch = strings.TrimSpace(string(out))
			} else {
				branch = "HEAD"
			}
		}

		args := []string{"push"}
		if setUpstream {
			args = append(args, "-u")
		}
		args = append(args, remote, branch)

		var stdout, stderr bytes.Buffer
		pushCmd := exec.CommandContext(toolCtx.Context, "git", args...)
		pushCmd.Dir = workDir
		pushCmd.Stdout = &stdout
		pushCmd.Stderr = &stderr

		if f.workspaceManager != nil && f.workspaceManager.GetGitCredentials() != nil {
			creds := f.workspaceManager.GetGitCredentials()
			if creds.HasToken() {
				askpassScript, cleanup, err := createGitAskpassScript(creds.Token)
				if err == nil {
					defer cleanup()
					pushCmd.Env = append(os.Environ(), "GIT_ASKPASS="+askpassScript, "GIT_TERMINAL_PROMPT=0")
				}
			}
		}

		if err := pushCmd.Run(); err != nil {
			combined := RedactString(strings.TrimSpace(stdout.String() + stderr.String()))
			return GitPushResult{
				Success: false,
				Remote:  remote,
				Branch:  branch,
				Error:   fmt.Sprintf("git push failed: %s: %s", err, combined),
			}, nil
		}

		return GitPushResult{
			Success: true,
			Remote:  remote,
			Branch:  branch,
			Message: strings.TrimSpace(stderr.String()),
		}, nil
	}

	return ai.NewToolWithInputSchema(
		"coding_push",
		"Push commits to the remote repository.",
		schema,
		toolFunc,
	)
}

func (f *ToolFactory) CreateAllTools() []ai.Tool {
	return []ai.Tool{
		f.CreateOpenTool(),
		f.CreateCodeTool(),
		f.CreateCloseTool(),
		f.CreateCommitTool(),
		f.CreatePushTool(),
	}
}
