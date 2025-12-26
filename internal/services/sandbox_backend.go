package services

import (
	"context"
	"os"
	"time"
)

// SandboxBackend abstracts the container runtime for code mode sandboxes.
// Implementations include DockerBackend (V2) and potentially ContainerdBackend (V3).
type SandboxBackend interface {
	// Session lifecycle
	CreateSession(ctx context.Context, opts SessionOptions) (*Session, error)
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	DestroySession(ctx context.Context, sessionID string) error

	// Execution
	Exec(ctx context.Context, sessionID string, req ExecRequest) (*ExecResult, error)
	ExecAsync(ctx context.Context, sessionID string, req ExecRequest) (*ExecHandle, error)
	ExecWait(ctx context.Context, sessionID, execID string, timeout time.Duration) (*ExecResult, error)
	ExecRead(ctx context.Context, sessionID, execID string, sinceSeq int, maxChunks int) (*ExecChunks, error)

	// Filesystem operations
	WriteFile(ctx context.Context, sessionID, path string, content []byte, mode os.FileMode) error
	ReadFile(ctx context.Context, sessionID, path string, maxBytes int) ([]byte, bool, error)
	ListFiles(ctx context.Context, sessionID, path string, recursive bool) ([]FileEntry, error)
	DeleteFile(ctx context.Context, sessionID, path string, recursive bool) error

	// Health check
	Ping(ctx context.Context) error
}
