package services

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"
	"unicode/utf8"

	"station/pkg/dotprompt"

	"github.com/firebase/genkit/go/ai"
)

type CodeModeToolFactory struct {
	sessionManager *SessionManager
	config         CodeModeConfig
}

func NewCodeModeToolFactory(sessionManager *SessionManager, config CodeModeConfig) *CodeModeToolFactory {
	return &CodeModeToolFactory{
		sessionManager: sessionManager,
		config:         config,
	}
}

func (f *CodeModeToolFactory) IsEnabled() bool {
	return f.config.Enabled
}

func (f *CodeModeToolFactory) ShouldAddTools(sandbox *dotprompt.SandboxConfig) bool {
	if sandbox == nil {
		return false
	}
	return sandbox.Mode == "code" && f.config.Enabled
}

type ExecutionContext struct {
	WorkflowRunID      string
	AgentRunID         string
	SandboxSessionName string
}

func (f *CodeModeToolFactory) CreateAllTools(agentDefaults *dotprompt.SandboxConfig, execCtx ExecutionContext) []ai.Tool {
	return []ai.Tool{
		f.CreateOpenTool(agentDefaults, execCtx),
		f.CreateExecTool(agentDefaults),
		f.CreateFsWriteTool(),
		f.CreateFsReadTool(),
		f.CreateFsListTool(),
		f.CreateFsDeleteTool(),
		f.CreateCloseTool(),
	}
}

type SandboxOpenInput struct {
	SessionKey string `json:"session_key,omitempty"`
}

type SandboxOpenOutput struct {
	SandboxID string `json:"sandbox_id"`
	Image     string `json:"image"`
	Workdir   string `json:"workdir"`
	Created   bool   `json:"created"`
}

func (f *CodeModeToolFactory) CreateOpenTool(agentDefaults *dotprompt.SandboxConfig, execCtx ExecutionContext) ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_key": map[string]any{
				"type":        "string",
				"description": "Optional session key override. Usually auto-resolved from workflow/agent context.",
			},
		},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, _ := input.(map[string]any)
		sessionKeyOverride, _ := inputMap["session_key"].(string)

		sessionName := execCtx.SandboxSessionName
		if sessionKeyOverride != "" {
			sessionName = sessionKeyOverride
		}

		key := ResolveSessionKey(execCtx.WorkflowRunID, execCtx.AgentRunID, sessionName)

		opts := f.buildSessionOptions(agentDefaults)

		session, created, err := f.sessionManager.GetOrCreateSession(toolCtx.Context, key, &opts)
		if err != nil {
			return nil, fmt.Errorf("sandbox_open: %w", err)
		}

		return SandboxOpenOutput{
			SandboxID: session.ID,
			Image:     session.Image,
			Workdir:   session.Workdir,
			Created:   created,
		}, nil
	}

	return ai.NewToolWithInputSchema(
		"sandbox_open",
		"Open or resume a persistent sandbox session. Call this first to get a sandbox_id for other sandbox operations. Files persist across calls within the same workflow.",
		schema,
		toolFunc,
	)
}

type SandboxExecInput struct {
	SandboxID      string            `json:"sandbox_id"`
	Cmd            []string          `json:"cmd"`
	Cwd            string            `json:"cwd,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
}

type SandboxExecOutput struct {
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Truncated bool   `json:"truncated,omitempty"`
}

func (f *CodeModeToolFactory) CreateExecTool(agentDefaults *dotprompt.SandboxConfig) ai.Tool {
	defaultTimeout := int(f.config.DefaultTimeout.Seconds())
	if agentDefaults != nil && agentDefaults.TimeoutSeconds > 0 {
		defaultTimeout = agentDefaults.TimeoutSeconds
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sandbox_id": map[string]any{
				"type":        "string",
				"description": "Sandbox session ID from sandbox_open",
			},
			"cmd": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Command and arguments to execute, e.g. [\"python\", \"main.py\"]",
			},
			"cwd": map[string]any{
				"type":        "string",
				"description": "Working directory (default: /workspace)",
			},
			"env": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
				"description":          "Additional environment variables",
			},
			"timeout_seconds": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     3600,
				"default":     defaultTimeout,
				"description": "Execution timeout in seconds",
			},
		},
		"required": []string{"sandbox_id", "cmd"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("sandbox_exec: expected map input")
		}

		sandboxID, _ := inputMap["sandbox_id"].(string)
		if sandboxID == "" {
			return nil, fmt.Errorf("sandbox_exec: sandbox_id is required")
		}

		cmdRaw, ok := inputMap["cmd"].([]any)
		if !ok || len(cmdRaw) == 0 {
			return nil, fmt.Errorf("sandbox_exec: cmd is required and must be non-empty array")
		}

		cmd := make([]string, 0, len(cmdRaw))
		for _, v := range cmdRaw {
			if s, ok := v.(string); ok {
				cmd = append(cmd, s)
			}
		}

		cwd, _ := inputMap["cwd"].(string)
		timeout := defaultTimeout
		if v, ok := inputMap["timeout_seconds"].(float64); ok {
			timeout = int(v)
		}

		env := make(map[string]string)
		if envRaw, ok := inputMap["env"].(map[string]any); ok {
			for k, v := range envRaw {
				if s, ok := v.(string); ok {
					env[k] = s
				}
			}
		}

		session, err := f.sessionManager.GetSessionByID(toolCtx.Context, sandboxID)
		if err != nil {
			return nil, fmt.Errorf("sandbox_exec: %w", err)
		}

		result, err := f.sessionManager.backend.Exec(toolCtx.Context, session.ID, ExecRequest{
			Cmd:            cmd,
			Cwd:            cwd,
			Env:            env,
			TimeoutSeconds: timeout,
		})
		if err != nil {
			return nil, fmt.Errorf("sandbox_exec: %w", err)
		}

		return SandboxExecOutput{
			ExitCode:  result.ExitCode,
			Stdout:    result.Stdout,
			Stderr:    result.Stderr,
			Truncated: result.Truncated,
		}, nil
	}

	return ai.NewToolWithInputSchema(
		"sandbox_exec",
		"Execute a command in the sandbox. Returns exit code, stdout, and stderr.",
		schema,
		toolFunc,
	)
}

type SandboxFsWriteInput struct {
	SandboxID   string `json:"sandbox_id"`
	Path        string `json:"path"`
	Contents    string `json:"contents,omitempty"`
	ContentsB64 string `json:"contents_b64,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Overwrite   bool   `json:"overwrite,omitempty"`
}

type SandboxFsWriteOutput struct {
	OK   bool   `json:"ok"`
	Path string `json:"path"`
}

func (f *CodeModeToolFactory) CreateFsWriteTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sandbox_id": map[string]any{
				"type":        "string",
				"description": "Sandbox session ID from sandbox_open",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "File path relative to /workspace",
			},
			"contents": map[string]any{
				"type":        "string",
				"description": "File contents as plain text",
			},
			"contents_b64": map[string]any{
				"type":        "string",
				"description": "File contents as base64 (for binary files)",
			},
			"mode": map[string]any{
				"type":        "string",
				"description": "Unix permission mode (default: 0644)",
				"default":     "0644",
			},
			"overwrite": map[string]any{
				"type":        "boolean",
				"description": "Whether to overwrite existing file",
				"default":     true,
			},
		},
		"required": []string{"sandbox_id", "path"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("sandbox_fs_write: expected map input")
		}

		sandboxID, _ := inputMap["sandbox_id"].(string)
		path, _ := inputMap["path"].(string)
		contents, _ := inputMap["contents"].(string)
		contentsB64, _ := inputMap["contents_b64"].(string)

		if sandboxID == "" {
			return nil, fmt.Errorf("sandbox_fs_write: sandbox_id is required")
		}
		if path == "" {
			return nil, fmt.Errorf("sandbox_fs_write: path is required")
		}

		var content []byte
		if contentsB64 != "" {
			decoded, err := base64.StdEncoding.DecodeString(contentsB64)
			if err != nil {
				return nil, fmt.Errorf("sandbox_fs_write: invalid base64: %w", err)
			}
			content = decoded
		} else {
			content = []byte(contents)
		}

		mode := os.FileMode(0644)
		if modeStr, ok := inputMap["mode"].(string); ok && modeStr != "" {
			var m uint32
			if _, err := fmt.Sscanf(modeStr, "%o", &m); err == nil {
				mode = os.FileMode(m)
			}
		}

		session, err := f.sessionManager.GetSessionByID(toolCtx.Context, sandboxID)
		if err != nil {
			return nil, fmt.Errorf("sandbox_fs_write: %w", err)
		}

		if err := f.sessionManager.backend.WriteFile(toolCtx.Context, session.ID, path, content, mode); err != nil {
			return nil, fmt.Errorf("sandbox_fs_write: %w", err)
		}

		return SandboxFsWriteOutput{
			OK:   true,
			Path: path,
		}, nil
	}

	return ai.NewToolWithInputSchema(
		"sandbox_fs_write",
		"Write a file to the sandbox workspace. Use contents for text files, contents_b64 for binary.",
		schema,
		toolFunc,
	)
}

type SandboxFsReadInput struct {
	SandboxID string `json:"sandbox_id"`
	Path      string `json:"path"`
	MaxBytes  int    `json:"max_bytes,omitempty"`
}

type SandboxFsReadOutput struct {
	Contents    string `json:"contents,omitempty"`
	ContentsB64 string `json:"contents_b64,omitempty"`
	Truncated   bool   `json:"truncated"`
	SizeBytes   int64  `json:"size_bytes"`
}

func (f *CodeModeToolFactory) CreateFsReadTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sandbox_id": map[string]any{
				"type":        "string",
				"description": "Sandbox session ID from sandbox_open",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "File path relative to /workspace",
			},
			"max_bytes": map[string]any{
				"type":        "integer",
				"description": "Maximum bytes to read (default: 256KB)",
				"default":     262144,
			},
		},
		"required": []string{"sandbox_id", "path"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("sandbox_fs_read: expected map input")
		}

		sandboxID, _ := inputMap["sandbox_id"].(string)
		path, _ := inputMap["path"].(string)
		maxBytes := 262144
		if v, ok := inputMap["max_bytes"].(float64); ok {
			maxBytes = int(v)
		}

		if sandboxID == "" {
			return nil, fmt.Errorf("sandbox_fs_read: sandbox_id is required")
		}
		if path == "" {
			return nil, fmt.Errorf("sandbox_fs_read: path is required")
		}

		session, err := f.sessionManager.GetSessionByID(toolCtx.Context, sandboxID)
		if err != nil {
			return nil, fmt.Errorf("sandbox_fs_read: %w", err)
		}

		content, truncated, err := f.sessionManager.backend.ReadFile(toolCtx.Context, session.ID, path, maxBytes)
		if err != nil {
			return nil, fmt.Errorf("sandbox_fs_read: %w", err)
		}

		output := SandboxFsReadOutput{
			Truncated: truncated,
			SizeBytes: int64(len(content)),
		}

		if utf8.Valid(content) {
			output.Contents = string(content)
		} else {
			output.ContentsB64 = base64.StdEncoding.EncodeToString(content)
		}

		return output, nil
	}

	return ai.NewToolWithInputSchema(
		"sandbox_fs_read",
		"Read a file from the sandbox workspace. Returns contents as text or base64 for binary files.",
		schema,
		toolFunc,
	)
}

type SandboxFsListInput struct {
	SandboxID string `json:"sandbox_id"`
	Path      string `json:"path,omitempty"`
	Recursive bool   `json:"recursive,omitempty"`
}

type SandboxFsListOutput struct {
	Entries []FileEntry `json:"entries"`
}

func (f *CodeModeToolFactory) CreateFsListTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sandbox_id": map[string]any{
				"type":        "string",
				"description": "Sandbox session ID from sandbox_open",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory path relative to /workspace (default: root)",
				"default":     ".",
			},
			"recursive": map[string]any{
				"type":        "boolean",
				"description": "List recursively",
				"default":     false,
			},
		},
		"required": []string{"sandbox_id"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("sandbox_fs_list: expected map input")
		}

		sandboxID, _ := inputMap["sandbox_id"].(string)
		path, _ := inputMap["path"].(string)
		recursive, _ := inputMap["recursive"].(bool)

		if sandboxID == "" {
			return nil, fmt.Errorf("sandbox_fs_list: sandbox_id is required")
		}
		if path == "" {
			path = "."
		}

		session, err := f.sessionManager.GetSessionByID(toolCtx.Context, sandboxID)
		if err != nil {
			return nil, fmt.Errorf("sandbox_fs_list: %w", err)
		}

		entries, err := f.sessionManager.backend.ListFiles(toolCtx.Context, session.ID, path, recursive)
		if err != nil {
			return nil, fmt.Errorf("sandbox_fs_list: %w", err)
		}

		return SandboxFsListOutput{Entries: entries}, nil
	}

	return ai.NewToolWithInputSchema(
		"sandbox_fs_list",
		"List files and directories in the sandbox workspace.",
		schema,
		toolFunc,
	)
}

type SandboxFsDeleteInput struct {
	SandboxID string `json:"sandbox_id"`
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
}

type SandboxFsDeleteOutput struct {
	OK      bool   `json:"ok"`
	Deleted string `json:"deleted"`
}

func (f *CodeModeToolFactory) CreateFsDeleteTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sandbox_id": map[string]any{
				"type":        "string",
				"description": "Sandbox session ID from sandbox_open",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "File or directory path relative to /workspace",
			},
			"recursive": map[string]any{
				"type":        "boolean",
				"description": "Delete directory recursively (required for directories)",
				"default":     false,
			},
		},
		"required": []string{"sandbox_id", "path"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("sandbox_fs_delete: expected map input")
		}

		sandboxID, _ := inputMap["sandbox_id"].(string)
		path, _ := inputMap["path"].(string)
		recursive, _ := inputMap["recursive"].(bool)

		if sandboxID == "" {
			return nil, fmt.Errorf("sandbox_fs_delete: sandbox_id is required")
		}
		if path == "" {
			return nil, fmt.Errorf("sandbox_fs_delete: path is required")
		}

		session, err := f.sessionManager.GetSessionByID(toolCtx.Context, sandboxID)
		if err != nil {
			return nil, fmt.Errorf("sandbox_fs_delete: %w", err)
		}

		if err := f.sessionManager.backend.DeleteFile(toolCtx.Context, session.ID, path, recursive); err != nil {
			return nil, fmt.Errorf("sandbox_fs_delete: %w", err)
		}

		return SandboxFsDeleteOutput{
			OK:      true,
			Deleted: path,
		}, nil
	}

	return ai.NewToolWithInputSchema(
		"sandbox_fs_delete",
		"Delete a file or directory from the sandbox workspace.",
		schema,
		toolFunc,
	)
}

type SandboxCloseInput struct {
	SandboxID       string `json:"sandbox_id"`
	DeleteWorkspace bool   `json:"delete_workspace,omitempty"`
}

type SandboxCloseOutput struct {
	OK bool `json:"ok"`
}

func (f *CodeModeToolFactory) CreateCloseTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sandbox_id": map[string]any{
				"type":        "string",
				"description": "Sandbox session ID from sandbox_open",
			},
			"delete_workspace": map[string]any{
				"type":        "boolean",
				"description": "Whether to delete the workspace files (default: true)",
				"default":     true,
			},
		},
		"required": []string{"sandbox_id"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("sandbox_close: expected map input")
		}

		sandboxID, _ := inputMap["sandbox_id"].(string)
		if sandboxID == "" {
			return nil, fmt.Errorf("sandbox_close: sandbox_id is required")
		}

		if err := f.sessionManager.CloseSession(toolCtx.Context, sandboxID); err != nil {
			return nil, fmt.Errorf("sandbox_close: %w", err)
		}

		return SandboxCloseOutput{OK: true}, nil
	}

	return ai.NewToolWithInputSchema(
		"sandbox_close",
		"Close a sandbox session and clean up resources. Usually not needed as sessions auto-cleanup when workflow completes.",
		schema,
		toolFunc,
	)
}

func (f *CodeModeToolFactory) buildSessionOptions(agentDefaults *dotprompt.SandboxConfig) SessionOptions {
	opts := DefaultSessionOptions()

	if agentDefaults == nil {
		return opts
	}

	if agentDefaults.Image != "" {
		opts.Image = agentDefaults.Image
	} else if agentDefaults.Runtime != "" {
		opts.Image = runtimeToDefaultImage(agentDefaults.Runtime)
	}

	opts.NetworkEnabled = agentDefaults.AllowNetwork

	if agentDefaults.Limits != nil {
		if agentDefaults.Limits.TimeoutSeconds > 0 {
			opts.Limits.TimeoutSeconds = agentDefaults.Limits.TimeoutSeconds
		}
	}

	return opts
}

func runtimeToDefaultImage(runtime string) string {
	switch runtime {
	case "python":
		return "python:3.11-slim"
	case "node":
		return "node:20-slim"
	case "bash":
		return "ubuntu:22.04"
	default:
		return "python:3.11-slim"
	}
}

func (f *CodeModeToolFactory) StartCleanupRoutine(interval, idleTimeout time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			f.sessionManager.CleanupIdleSessions(nil, idleTimeout)
		}
	}()
}
