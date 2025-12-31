package services

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"station/internal/storage"
	"station/pkg/dotprompt"

	"github.com/firebase/genkit/go/ai"
)

// CodeModeEnvVarPrefix is the prefix for environment variables that should be
// propagated to sandbox containers. Variables with this prefix will have the
// prefix stripped and be passed to the container.
// Example: STN_CODE_DATABASE_URL=postgres://... -> DATABASE_URL=postgres://...
const CodeModeEnvVarPrefix = "STN_CODE_"

type CodeModeToolFactory struct {
	sessionManager *SessionManager
	config         CodeModeConfig
	fileStore      storage.FileStore
}

func NewCodeModeToolFactory(sessionManager *SessionManager, config CodeModeConfig) *CodeModeToolFactory {
	return &CodeModeToolFactory{
		sessionManager: sessionManager,
		config:         config,
	}
}

func (f *CodeModeToolFactory) SetFileStore(store storage.FileStore) {
	f.fileStore = store
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
	tools := []ai.Tool{
		f.CreateOpenTool(agentDefaults, execCtx),
		f.CreateExecTool(agentDefaults),
		f.CreateFsWriteTool(),
		f.CreateFsReadTool(),
		f.CreateFsListTool(),
		f.CreateFsDeleteTool(),
		f.CreateCloseTool(),
	}

	if f.fileStore != nil {
		tools = append(tools,
			f.CreateStageFileTool(),
			f.CreatePublishFileTool(),
		)
	}

	return tools
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

type SandboxStageFileInput struct {
	SandboxID   string `json:"sandbox_id"`
	FileKey     string `json:"file_key"`
	Destination string `json:"destination"`
}

type SandboxStageFileOutput struct {
	OK        bool   `json:"ok"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
}

func (f *CodeModeToolFactory) CreateStageFileTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sandbox_id": map[string]any{
				"type":        "string",
				"description": "Sandbox session ID from sandbox_open",
			},
			"file_key": map[string]any{
				"type":        "string",
				"description": "File key from file upload (e.g., files/f_abc123)",
			},
			"destination": map[string]any{
				"type":        "string",
				"description": "Destination path in sandbox relative to /workspace (e.g., input/data.csv)",
			},
		},
		"required": []string{"sandbox_id", "file_key", "destination"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("sandbox_stage_file: expected map input")
		}

		sandboxID, _ := inputMap["sandbox_id"].(string)
		fileKey, _ := inputMap["file_key"].(string)
		destination, _ := inputMap["destination"].(string)

		if sandboxID == "" {
			return nil, fmt.Errorf("sandbox_stage_file: sandbox_id is required")
		}
		if fileKey == "" {
			return nil, fmt.Errorf("sandbox_stage_file: file_key is required")
		}
		if destination == "" {
			return nil, fmt.Errorf("sandbox_stage_file: destination is required")
		}

		if strings.Contains(destination, "..") {
			return nil, fmt.Errorf("sandbox_stage_file: path traversal not allowed")
		}

		session, err := f.sessionManager.GetSessionByID(toolCtx.Context, sandboxID)
		if err != nil {
			return nil, fmt.Errorf("sandbox_stage_file: %w", err)
		}

		reader, info, err := f.fileStore.Get(toolCtx.Context, fileKey)
		if err != nil {
			if storage.IsNotFound(err) {
				return nil, fmt.Errorf("sandbox_stage_file: file not found: %s", fileKey)
			}
			return nil, fmt.Errorf("sandbox_stage_file: failed to fetch file: %w", err)
		}
		defer reader.Close()

		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("sandbox_stage_file: failed to read file: %w", err)
		}

		destDir := filepath.Dir(destination)
		if destDir != "." && destDir != "/" {
			mkdirResult, err := f.sessionManager.backend.Exec(toolCtx.Context, session.ID, ExecRequest{
				Cmd:            []string{"mkdir", "-p", destDir},
				TimeoutSeconds: 10,
			})
			if err != nil || mkdirResult.ExitCode != 0 {
				return nil, fmt.Errorf("sandbox_stage_file: failed to create directory %s", destDir)
			}
		}

		if err := f.sessionManager.backend.WriteFile(toolCtx.Context, session.ID, destination, content, 0644); err != nil {
			return nil, fmt.Errorf("sandbox_stage_file: failed to write file: %w", err)
		}

		return SandboxStageFileOutput{
			OK:        true,
			Path:      destination,
			SizeBytes: info.Size,
		}, nil
	}

	return ai.NewToolWithInputSchema(
		"sandbox_stage_file",
		"Stage a file from the file store into the sandbox. Use this to bring uploaded files into the sandbox for processing.",
		schema,
		toolFunc,
	)
}

type SandboxPublishFileInput struct {
	SandboxID string `json:"sandbox_id"`
	Source    string `json:"source"`
	FileKey   string `json:"file_key,omitempty"`
}

type SandboxPublishFileOutput struct {
	OK        bool   `json:"ok"`
	FileKey   string `json:"file_key"`
	SizeBytes int64  `json:"size_bytes"`
}

func (f *CodeModeToolFactory) CreatePublishFileTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sandbox_id": map[string]any{
				"type":        "string",
				"description": "Sandbox session ID from sandbox_open",
			},
			"source": map[string]any{
				"type":        "string",
				"description": "Source file path in sandbox relative to /workspace (e.g., output/result.csv)",
			},
			"file_key": map[string]any{
				"type":        "string",
				"description": "Optional custom file key. If not provided, a unique key is generated.",
			},
		},
		"required": []string{"sandbox_id", "source"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("sandbox_publish_file: expected map input")
		}

		sandboxID, _ := inputMap["sandbox_id"].(string)
		source, _ := inputMap["source"].(string)
		fileKey, _ := inputMap["file_key"].(string)

		if sandboxID == "" {
			return nil, fmt.Errorf("sandbox_publish_file: sandbox_id is required")
		}
		if source == "" {
			return nil, fmt.Errorf("sandbox_publish_file: source is required")
		}

		if strings.Contains(source, "..") {
			return nil, fmt.Errorf("sandbox_publish_file: path traversal not allowed")
		}

		session, err := f.sessionManager.GetSessionByID(toolCtx.Context, sandboxID)
		if err != nil {
			return nil, fmt.Errorf("sandbox_publish_file: %w", err)
		}

		content, _, err := f.sessionManager.backend.ReadFile(toolCtx.Context, session.ID, source, 0)
		if err != nil {
			return nil, fmt.Errorf("sandbox_publish_file: failed to read file: %w", err)
		}

		if fileKey == "" {
			fileID := storage.GenerateFileID()
			fileKey = storage.GenerateUserFileKey(fileID)
		}

		contentType := detectContentTypeFromPath(source)

		info, err := f.fileStore.Put(toolCtx.Context, fileKey, strings.NewReader(string(content)), storage.PutOptions{
			ContentType: contentType,
			Description: filepath.Base(source),
			Metadata: map[string]string{
				"original_path": source,
				"sandbox_id":    sandboxID,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("sandbox_publish_file: failed to upload file: %w", err)
		}

		return SandboxPublishFileOutput{
			OK:        true,
			FileKey:   info.Key,
			SizeBytes: info.Size,
		}, nil
	}

	return ai.NewToolWithInputSchema(
		"sandbox_publish_file",
		"Publish a file from the sandbox to the file store. Use this to make output files available for download.",
		schema,
		toolFunc,
	)
}

func detectContentTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".txt":
		return "text/plain"
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	default:
		return "application/octet-stream"
	}
}

func (f *CodeModeToolFactory) buildSessionOptions(agentDefaults *dotprompt.SandboxConfig) SessionOptions {
	opts := DefaultSessionOptions()

	opts.Env = collectCodeModeEnvVars()

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

func collectCodeModeEnvVars() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, CodeModeEnvVarPrefix) {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimPrefix(parts[0], CodeModeEnvVarPrefix)
				env[key] = parts[1]
			}
		}
	}
	return env
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
