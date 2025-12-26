package services

import "context"

type SandboxBackend interface {
	CreateSession(ctx context.Context, key SessionKey, cfg SessionConfig) (*CodeSession, error)
	DestroySession(ctx context.Context, sessionID string) error
	GetSession(ctx context.Context, sessionID string) (*CodeSession, error)

	Exec(ctx context.Context, sessionID string, req ExecRequest) (*ExecResult, error)

	WriteFile(ctx context.Context, sessionID, path string, content []byte) error
	ReadFile(ctx context.Context, sessionID, path string) ([]byte, error)
	ListFiles(ctx context.Context, sessionID, path string, recursive bool) ([]FileEntry, error)
	DeleteFile(ctx context.Context, sessionID, path string, recursive bool) error
}
