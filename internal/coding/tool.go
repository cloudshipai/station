package coding

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/firebase/genkit/go/ai"
)

type ExecutionContext struct {
	WorkflowRunID string
	AgentRunID    string
}

type ToolFactory struct {
	backend          Backend
	workspaceManager *WorkspaceManager
	execContext      ExecutionContext
}

type ToolFactoryOption func(*ToolFactory)

func WithWorkspaceManager(wm *WorkspaceManager) ToolFactoryOption {
	return func(f *ToolFactory) {
		f.workspaceManager = wm
	}
}

func WithExecutionContext(ctx ExecutionContext) ToolFactoryOption {
	return func(f *ToolFactory) {
		f.execContext = ctx
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
	RepoURL       string `json:"repo_url,omitempty"`
	Branch        string `json:"branch,omitempty"`
	Title         string `json:"title,omitempty"`
	Scope         string `json:"scope,omitempty"`
	ScopeID       string `json:"scope_id,omitempty"`
}

type CodingOpenOutput struct {
	SessionID     string `json:"session_id"`
	WorkspacePath string `json:"workspace_path"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	RepoCloned    bool   `json:"repo_cloned,omitempty"`
	Managed       bool   `json:"managed"`
}

func (f *ToolFactory) CreateOpenTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"repo_url": map[string]any{
				"type":        "string",
				"description": "Git repository URL to clone (e.g., https://github.com/org/repo.git). The backend will clone this repo.",
			},
			"branch": map[string]any{
				"type":        "string",
				"description": "Git branch to checkout after cloning (optional, defaults to default branch)",
			},
			"workspace_path": map[string]any{
				"type":        "string",
				"description": "Explicit workspace path. Usually not needed - the backend creates workspaces automatically.",
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

		repoURL, _ := inputMap["repo_url"].(string)
		branch, _ := inputMap["branch"].(string)
		workspacePath, _ := inputMap["workspace_path"].(string)
		title, _ := inputMap["title"].(string)
		scopeStr, _ := inputMap["scope"].(string)
		scopeID, _ := inputMap["scope_id"].(string)

		if scopeStr == "" && f.execContext.WorkflowRunID != "" {
			scopeStr = "workflow"
			scopeID = f.execContext.WorkflowRunID
		}

		var workspaceID string
		managed := false
		var gitCreds *GitCredentials

		if workspacePath == "" && f.workspaceManager != nil {
			scope := ScopeAgent
			if scopeStr == "workflow" {
				scope = ScopeWorkflow
			}

			if scopeID == "" {
				scopeID = fmt.Sprintf("session_%d", time.Now().UnixNano())
			}

			if scope == ScopeWorkflow {
				if existingWs, err := f.workspaceManager.GetByScope(scope, scopeID); err == nil {
					workspacePath = existingWs.Path
					workspaceID = existingWs.ID
					managed = true
					gitCreds = f.workspaceManager.GetGitCredentials()
				}
			}

			if workspacePath == "" {
				ws, err := f.workspaceManager.Create(toolCtx.Context, scope, scopeID)
				if err != nil {
					return nil, fmt.Errorf("coding_open: create workspace: %w", err)
				}

				workspacePath = ws.Path
				workspaceID = ws.ID
				managed = true
				gitCreds = f.workspaceManager.GetGitCredentials()
			}
		}

		opts := SessionOptions{
			WorkspacePath:  workspacePath,
			Title:          title,
			RepoURL:        repoURL,
			Branch:         branch,
			GitCredentials: gitCreds,
		}

		session, err := f.backend.CreateSession(toolCtx.Context, opts)
		if err != nil {
			return nil, fmt.Errorf("coding_open: %w", err)
		}

		return CodingOpenOutput{
			SessionID:     session.ID,
			WorkspacePath: session.WorkspacePath,
			WorkspaceID:   workspaceID,
			RepoCloned:    repoURL != "",
			Managed:       managed,
		}, nil
	}

	return ai.NewToolWithInputSchema(
		"coding_open",
		"Open a coding session. Provide repo_url to clone a repository, or workspace_path for existing local repos.",
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

		result, err := f.backend.GitCommit(toolCtx.Context, sessionID, message, addAll)
		if err != nil {
			return nil, fmt.Errorf("coding_commit: %w", err)
		}

		return result, nil
	}

	return ai.NewToolWithInputSchema(
		"coding_commit",
		"Commit changes in the coding session workspace. The backend executes git commands.",
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

		result, err := f.backend.GitPush(toolCtx.Context, sessionID, remote, branch, setUpstream)
		if err != nil {
			return nil, fmt.Errorf("coding_push: %w", err)
		}

		return result, nil
	}

	return ai.NewToolWithInputSchema(
		"coding_push",
		"Push commits to the remote repository. The backend executes git push.",
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
