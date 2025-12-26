package services

import (
	"encoding/base64"
	"fmt"

	"station/pkg/dotprompt"

	"github.com/firebase/genkit/go/ai"
)

type CodeModeToolFactory struct {
	manager *SessionManager
	backend SandboxBackend
}

func NewCodeModeToolFactory(manager *SessionManager, backend SandboxBackend) *CodeModeToolFactory {
	return &CodeModeToolFactory{
		manager: manager,
		backend: backend,
	}
}

func (f *CodeModeToolFactory) CreateTools(execCtx ExecutionContext, sandboxCfg *dotprompt.SandboxConfig) []ai.Tool {
	scope := SessionScopeWorkflow
	if sandboxCfg.Session == "agent" {
		scope = SessionScopeAgent
	}
	sessionKey := execCtx.SessionKey(scope)

	cfg := SessionConfig{
		Runtime: sandboxCfg.Runtime,
		Image:   sandboxCfg.Image,
	}
	if cfg.Runtime == "" {
		cfg.Runtime = "linux"
	}

	return []ai.Tool{
		f.createOpenTool(sessionKey, cfg),
		f.createExecTool(sessionKey, sandboxCfg),
		f.createFsWriteTool(sessionKey),
		f.createFsReadTool(sessionKey),
		f.createFsListTool(sessionKey),
		f.createFsDeleteTool(sessionKey),
		f.createCloseTool(sessionKey),
	}
}

func (f *CodeModeToolFactory) createOpenTool(key SessionKey, cfg SessionConfig) ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"image": map[string]any{
				"type":        "string",
				"description": "Docker image to use. Shortcuts: 'python' (python:3.11-slim), 'node' (node:20-slim), 'linux' (ubuntu:22.04). Or specify any Docker image directly (e.g., 'golang:1.22', 'rust:1.75'). Default: ubuntu:22.04",
			},
		},
	}

	return ai.NewToolWithInputSchema(
		"sandbox_open",
		"Get or create a persistent Linux sandbox session. This is a full Linux environment where you can run any commands, install packages with apt/pip/npm, compile code, run scripts, and more. Files persist across calls within the same workflow/agent run.",
		schema,
		func(toolCtx *ai.ToolContext, input any) (any, error) {
			inputMap, _ := input.(map[string]any)

			sessionCfg := cfg
			if img, ok := inputMap["image"].(string); ok && img != "" {
				sessionCfg.Runtime = img
			}
			if sessionCfg.Runtime == "" {
				sessionCfg.Runtime = "linux"
			}

			session, err := f.manager.GetOrCreateSession(toolCtx.Context, key, sessionCfg)
			if err != nil {
				return nil, err
			}

			return map[string]any{
				"session_id": session.ID,
				"image":      session.Config.Image,
				"workdir":    "/work",
				"status":     string(session.Status),
			}, nil
		},
	)
}

func (f *CodeModeToolFactory) createExecTool(key SessionKey, sandboxCfg *dotprompt.SandboxConfig) ai.Tool {
	defaultTimeout := 60
	if sandboxCfg.TimeoutSeconds > 0 {
		defaultTimeout = sandboxCfg.TimeoutSeconds
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command to execute (e.g., 'apt update && apt install -y curl', 'python main.py', 'gcc -o app main.c && ./app')",
			},
			"timeout_seconds": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     3600,
				"description": "Execution timeout in seconds",
				"default":     defaultTimeout,
			},
			"workdir": map[string]any{
				"type":        "string",
				"description": "Working directory (default: /work)",
			},
		},
		"required": []string{"command"},
	}

	return ai.NewToolWithInputSchema(
		"sandbox_exec",
		"Execute a shell command in the Linux sandbox. Full shell access - run any command: install packages (apt, pip, npm), compile code, run scripts, curl APIs, process data, etc. Commands run via 'sh -c'.",
		schema,
		func(toolCtx *ai.ToolContext, input any) (any, error) {
			inputMap, ok := input.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid input")
			}

			session, exists := f.manager.GetSession(key)
			if !exists {
				return nil, fmt.Errorf("no sandbox session open - call sandbox_open first")
			}

			command, _ := inputMap["command"].(string)
			if command == "" {
				return nil, fmt.Errorf("command is required")
			}

			timeout := defaultTimeout
			if t, ok := inputMap["timeout_seconds"].(float64); ok {
				timeout = int(t)
			}

			workdir := "/work"
			if w, ok := inputMap["workdir"].(string); ok && w != "" {
				workdir = w
			}

			result, err := f.backend.Exec(toolCtx.Context, session.ContainerID, ExecRequest{
				Command:        command,
				Workdir:        workdir,
				TimeoutSeconds: timeout,
			})
			if err != nil {
				return nil, err
			}

			return map[string]any{
				"exit_code":   result.ExitCode,
				"stdout":      result.Stdout,
				"stderr":      result.Stderr,
				"duration_ms": result.DurationMs,
				"timed_out":   result.TimedOut,
			}, nil
		},
	)
}

func (f *CodeModeToolFactory) createFsWriteTool(key SessionKey) ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "File path relative to /work (e.g., 'main.py' or 'src/utils.py')",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "File content (text or base64-encoded)",
			},
			"encoding": map[string]any{
				"type":        "string",
				"enum":        []string{"text", "base64"},
				"description": "Content encoding",
				"default":     "text",
			},
		},
		"required": []string{"path", "content"},
	}

	return ai.NewToolWithInputSchema(
		"sandbox_fs_write",
		"Write a file to the sandbox filesystem. Use this to create source code files, config files, or data files.",
		schema,
		func(toolCtx *ai.ToolContext, input any) (any, error) {
			inputMap, ok := input.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid input")
			}

			session, exists := f.manager.GetSession(key)
			if !exists {
				return nil, fmt.Errorf("no sandbox session open - call sandbox_open first")
			}

			path, _ := inputMap["path"].(string)
			content, _ := inputMap["content"].(string)
			encoding, _ := inputMap["encoding"].(string)

			if path == "" || content == "" {
				return nil, fmt.Errorf("path and content are required")
			}

			var data []byte
			if encoding == "base64" {
				var err error
				data, err = base64.StdEncoding.DecodeString(content)
				if err != nil {
					return nil, fmt.Errorf("invalid base64: %w", err)
				}
			} else {
				data = []byte(content)
			}

			if err := f.backend.WriteFile(toolCtx.Context, session.ContainerID, path, data); err != nil {
				return nil, err
			}

			return map[string]any{
				"path":       "/work/" + path,
				"size_bytes": len(data),
			}, nil
		},
	)
}

func (f *CodeModeToolFactory) createFsReadTool(key SessionKey) ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "File path relative to /work",
			},
			"encoding": map[string]any{
				"type":        "string",
				"enum":        []string{"text", "base64"},
				"description": "Response encoding",
				"default":     "text",
			},
		},
		"required": []string{"path"},
	}

	return ai.NewToolWithInputSchema(
		"sandbox_fs_read",
		"Read a file from the sandbox filesystem.",
		schema,
		func(toolCtx *ai.ToolContext, input any) (any, error) {
			inputMap, ok := input.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid input")
			}

			session, exists := f.manager.GetSession(key)
			if !exists {
				return nil, fmt.Errorf("no sandbox session open - call sandbox_open first")
			}

			path, _ := inputMap["path"].(string)
			encoding, _ := inputMap["encoding"].(string)

			if path == "" {
				return nil, fmt.Errorf("path is required")
			}

			data, err := f.backend.ReadFile(toolCtx.Context, session.ContainerID, path)
			if err != nil {
				return nil, err
			}

			var content string
			if encoding == "base64" {
				content = base64.StdEncoding.EncodeToString(data)
			} else {
				content = string(data)
			}

			return map[string]any{
				"path":       "/work/" + path,
				"content":    content,
				"size_bytes": len(data),
			}, nil
		},
	)
}

func (f *CodeModeToolFactory) createFsListTool(key SessionKey) ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Directory path relative to /work",
				"default":     ".",
			},
			"recursive": map[string]any{
				"type":        "boolean",
				"description": "List recursively",
				"default":     false,
			},
		},
	}

	return ai.NewToolWithInputSchema(
		"sandbox_fs_list",
		"List files and directories in the sandbox.",
		schema,
		func(toolCtx *ai.ToolContext, input any) (any, error) {
			inputMap, _ := input.(map[string]any)

			session, exists := f.manager.GetSession(key)
			if !exists {
				return nil, fmt.Errorf("no sandbox session open - call sandbox_open first")
			}

			path := "."
			if p, ok := inputMap["path"].(string); ok && p != "" {
				path = p
			}

			recursive := false
			if r, ok := inputMap["recursive"].(bool); ok {
				recursive = r
			}

			entries, err := f.backend.ListFiles(toolCtx.Context, session.ContainerID, path, recursive)
			if err != nil {
				return nil, err
			}

			result := make([]map[string]any, len(entries))
			for i, e := range entries {
				result[i] = map[string]any{
					"name":       e.Name,
					"type":       string(e.Type),
					"size_bytes": e.SizeBytes,
				}
			}

			return map[string]any{
				"entries": result,
			}, nil
		},
	)
}

func (f *CodeModeToolFactory) createFsDeleteTool(key SessionKey) ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "File or directory path relative to /work",
			},
			"recursive": map[string]any{
				"type":        "boolean",
				"description": "Delete recursively (required for directories)",
				"default":     false,
			},
		},
		"required": []string{"path"},
	}

	return ai.NewToolWithInputSchema(
		"sandbox_fs_delete",
		"Delete a file or directory from the sandbox.",
		schema,
		func(toolCtx *ai.ToolContext, input any) (any, error) {
			inputMap, ok := input.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid input")
			}

			session, exists := f.manager.GetSession(key)
			if !exists {
				return nil, fmt.Errorf("no sandbox session open - call sandbox_open first")
			}

			path, _ := inputMap["path"].(string)
			if path == "" {
				return nil, fmt.Errorf("path is required")
			}

			recursive := false
			if r, ok := inputMap["recursive"].(bool); ok {
				recursive = r
			}

			if err := f.backend.DeleteFile(toolCtx.Context, session.ContainerID, path, recursive); err != nil {
				return nil, err
			}

			return map[string]any{
				"deleted": true,
				"path":    "/work/" + path,
			}, nil
		},
	)
}

func (f *CodeModeToolFactory) createCloseTool(key SessionKey) ai.Tool {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}

	return ai.NewToolWithInputSchema(
		"sandbox_close",
		"Explicitly close the sandbox session. Optional - sessions are automatically cleaned up when the workflow completes.",
		schema,
		func(toolCtx *ai.ToolContext, input any) (any, error) {
			session, exists := f.manager.GetSession(key)
			if !exists {
				return map[string]any{
					"closed":     false,
					"session_id": "",
					"message":    "no session was open",
				}, nil
			}

			sessionID := session.ID
			if err := f.manager.DestroySession(toolCtx.Context, key); err != nil {
				return nil, err
			}

			return map[string]any{
				"closed":     true,
				"session_id": sessionID,
			}, nil
		},
	)
}
