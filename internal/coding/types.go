// Package coding provides a backend for delegating coding tasks to AI-powered
// coding assistants like OpenCode.
package coding

import (
	"time"
)

// Session represents an active coding session with a backend.
type Session struct {
	ID               string            `json:"id"`
	BackendSessionID string            `json:"backend_session_id"`
	WorkspacePath    string            `json:"workspace_path"`
	Title            string            `json:"title,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	LastUsedAt       time.Time         `json:"last_used_at"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// Task represents a coding task to be executed by the backend.
type Task struct {
	Instruction string        `json:"instruction"`
	Context     string        `json:"context,omitempty"`
	Files       []string      `json:"files,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`
}

// Result represents the outcome of a coding task.
type Result struct {
	Success      bool         `json:"success"`
	Summary      string       `json:"summary"`
	FilesChanged []FileChange `json:"files_changed,omitempty"`
	Error        string       `json:"error,omitempty"`
	Trace        *Trace       `json:"trace,omitempty"`
}

// FileChange represents a change made to a file.
type FileChange struct {
	Path         string `json:"path"`
	Action       string `json:"action"` // "created", "modified", "deleted"
	LinesAdded   int    `json:"lines_added,omitempty"`
	LinesRemoved int    `json:"lines_removed,omitempty"`
}

// Trace contains detailed execution trace for observability.
type Trace struct {
	MessageID    string        `json:"message_id,omitempty"`
	SessionID    string        `json:"session_id,omitempty"`
	Model        string        `json:"model,omitempty"`
	Provider     string        `json:"provider,omitempty"`
	Cost         float64       `json:"cost,omitempty"`
	Tokens       TokenUsage    `json:"tokens"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`
	Duration     time.Duration `json:"duration"`
	ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`
	Reasoning    []string      `json:"reasoning,omitempty"`
	FinishReason string        `json:"finish_reason,omitempty"`
}

// TokenUsage contains token usage breakdown.
type TokenUsage struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	Reasoning  int `json:"reasoning,omitempty"`
	CacheRead  int `json:"cache_read,omitempty"`
	CacheWrite int `json:"cache_write,omitempty"`
}

// Total returns the total token count.
func (t TokenUsage) Total() int {
	return t.Input + t.Output + t.Reasoning
}

// ToolCall represents a tool invocation during task execution.
type ToolCall struct {
	Tool     string                 `json:"tool"`
	Input    map[string]interface{} `json:"input,omitempty"`
	Output   string                 `json:"output,omitempty"`
	ExitCode int                    `json:"exit_code,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Duration time.Duration          `json:"duration,omitempty"`
}

// GitCommitResult represents the result of a git commit operation.
type GitCommitResult struct {
	Success      bool   `json:"success"`
	CommitHash   string `json:"commit_hash,omitempty"`
	Message      string `json:"message"`
	FilesChanged int    `json:"files_changed,omitempty"`
	Insertions   int    `json:"insertions,omitempty"`
	Deletions    int    `json:"deletions,omitempty"`
	Error        string `json:"error,omitempty"`
}

// GitPushResult represents the result of a git push operation.
type GitPushResult struct {
	Success bool   `json:"success"`
	Remote  string `json:"remote"`
	Branch  string `json:"branch"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}
