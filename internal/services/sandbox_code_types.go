package services

import (
	"fmt"
	"time"
)

// =============================================================================
// Session Types
// =============================================================================

// SessionKey uniquely identifies a sandbox session within a namespace
type SessionKey struct {
	Namespace string // "workflow", "agent", or "user"
	ID        string // workflow_run_id, agent_run_id, etc.
	Key       string // session name within namespace (default: "default")
}

// String returns the canonical string representation of the session key
func (k SessionKey) String() string {
	return fmt.Sprintf("%s:%s:%s", k.Namespace, k.ID, k.Key)
}

// Session represents a running sandbox session
type Session struct {
	ID            string            // Unique session ID (e.g., "sbx_abc123")
	Key           SessionKey        // The key used to create/lookup this session
	ContainerID   string            // Docker container ID
	Image         string            // Container image
	Workdir       string            // Working directory inside container
	WorkspacePath string            // Host path to workspace bind mount
	Env           map[string]string // Environment variables
	Limits        ResourceLimits    // Resource limits
	CreatedAt     time.Time         // When the session was created
	LastUsedAt    time.Time         // Last time session was used
}

// SessionOptions configures a new sandbox session
type SessionOptions struct {
	Image          string            // Container image (e.g., "python:3.11-slim")
	Workdir        string            // Working directory inside container (default: "/workspace")
	Env            map[string]string // Environment variables
	Limits         ResourceLimits    // Resource limits
	NetworkEnabled bool              // Whether to enable network access
}

// DefaultSessionOptions returns sensible defaults for session creation
func DefaultSessionOptions() SessionOptions {
	return SessionOptions{
		Image:   "python:3.11-slim",
		Workdir: "/workspace",
		Env:     make(map[string]string),
		Limits: ResourceLimits{
			CPUMillicores:  1000, // 1 CPU
			MemoryMB:       2048, // 2GB
			TimeoutSeconds: 900,  // 15 minutes
			WorkspaceMB:    500,  // 500MB
		},
		NetworkEnabled: false,
	}
}

// ResourceLimits defines resource constraints for a sandbox session
type ResourceLimits struct {
	CPUMillicores  int // CPU limit in millicores (1000 = 1 CPU)
	MemoryMB       int // Memory limit in megabytes
	TimeoutSeconds int // Session idle timeout
	WorkspaceMB    int // Maximum workspace size in megabytes
}

// =============================================================================
// Execution Types
// =============================================================================

// ExecRequest defines a command to execute in a sandbox
type ExecRequest struct {
	Cmd            []string          // Command and arguments (e.g., ["python", "main.py"])
	Cwd            string            // Working directory (default: /workspace)
	Env            map[string]string // Additional environment variables
	TimeoutSeconds int               // Execution timeout (0 = use session default)
}

// ExecHandle represents a running execution that can be waited on or read from
type ExecHandle struct {
	ID        string    // Unique exec ID
	SessionID string    // Parent session ID
	Cmd       []string  // Command being executed
	StartedAt time.Time // When execution started
}

// ExecResult contains the result of a completed execution
type ExecResult struct {
	ID        string        // Exec ID
	ExitCode  int           // Process exit code
	Stdout    string        // Standard output
	Stderr    string        // Standard error
	Duration  time.Duration // How long execution took
	Truncated bool          // Whether output was truncated
	Cancelled bool          // Whether execution was cancelled
}

// OutputChunk represents a chunk of output from a streaming execution
type OutputChunk struct {
	Seq    int    // Sequence number for ordering
	Stream string // "stdout" or "stderr"
	Text   string // Output text
}

// ExecChunks contains chunks of output from a streaming read
type ExecChunks struct {
	Chunks []OutputChunk
	Done   bool // Whether execution has completed
}

// =============================================================================
// Filesystem Types
// =============================================================================

// FileEntry represents a file or directory in the sandbox filesystem
type FileEntry struct {
	Path      string // Relative path from workspace root
	Type      string // "file" or "dir"
	Size      int64  // File size in bytes (0 for directories)
	Mode      string // Unix permission string (e.g., "0644")
	MtimeUnix int64  // Modification time as Unix timestamp
}

// =============================================================================
// Backend Configuration
// =============================================================================

// CodeModeConfig holds configuration for the code mode sandbox backend
type CodeModeConfig struct {
	Enabled          bool          // Whether code mode is enabled
	WorkspaceBaseDir string        // Base directory for workspace bind mounts
	AllowedImages    []string      // Allowed container images
	DefaultImage     string        // Default image if not specified
	DefaultTimeout   time.Duration // Default execution timeout
	MaxStdoutBytes   int           // Maximum stdout bytes to capture
	MaxStderrBytes   int           // Maximum stderr bytes to capture
	CleanupInterval  time.Duration // How often to cleanup idle sessions
	IdleTimeout      time.Duration // How long before idle session is cleaned up
	DockerHost       string        // Docker host (empty = default)
}

// DefaultCodeModeConfig returns sensible defaults
func DefaultCodeModeConfig() CodeModeConfig {
	return CodeModeConfig{
		Enabled:          false,
		WorkspaceBaseDir: "/tmp/station-sandboxes",
		AllowedImages:    []string{"python:3.11-slim", "python:3.12-slim", "node:20-slim", "node:22-slim", "ubuntu:22.04", "ubuntu:24.04"},
		DefaultImage:     "python:3.11-slim",
		DefaultTimeout:   2 * time.Minute,
		MaxStdoutBytes:   1024 * 1024, // 1MB
		MaxStderrBytes:   1024 * 1024, // 1MB
		CleanupInterval:  5 * time.Minute,
		IdleTimeout:      30 * time.Minute,
		DockerHost:       "",
	}
}

// =============================================================================
// Error Types
// =============================================================================

// SandboxError represents an error from sandbox operations
type SandboxError struct {
	Op      string // Operation that failed (e.g., "CreateSession", "Exec")
	Session string // Session ID if applicable
	Err     error  // Underlying error
}

func (e *SandboxError) Error() string {
	if e.Session != "" {
		return fmt.Sprintf("sandbox %s [%s]: %v", e.Op, e.Session, e.Err)
	}
	return fmt.Sprintf("sandbox %s: %v", e.Op, e.Err)
}

func (e *SandboxError) Unwrap() error {
	return e.Err
}

// Common sentinel errors
var (
	ErrSessionNotFound = fmt.Errorf("session not found")
	ErrSessionClosed   = fmt.Errorf("session is closed")
	ErrExecNotFound    = fmt.Errorf("exec not found")
	ErrImageNotAllowed = fmt.Errorf("image not in allowed list")
	ErrTimeout         = fmt.Errorf("operation timed out")
)
