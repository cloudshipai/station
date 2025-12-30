package coding

import (
	"context"
)

type Backend interface {
	Ping(ctx context.Context) error
	CreateSession(ctx context.Context, workspacePath, title string) (*Session, error)
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	CloseSession(ctx context.Context, sessionID string) error
	Execute(ctx context.Context, sessionID string, task Task) (*Result, error)
}
