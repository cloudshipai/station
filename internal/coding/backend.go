package coding

import (
	"context"
)

// Backend defines the interface for coding backends like OpenCode.
// All operations (including git) are executed by the backend, not locally.
type Backend interface {
	// Ping checks if the backend is healthy.
	Ping(ctx context.Context) error

	// CreateSession creates a new coding session.
	// If opts.RepoURL is provided, the backend clones the repo into the workspace.
	CreateSession(ctx context.Context, opts SessionOptions) (*Session, error)

	// GetSession retrieves an existing session.
	GetSession(ctx context.Context, sessionID string) (*Session, error)

	// CloseSession closes and cleans up a session.
	CloseSession(ctx context.Context, sessionID string) error

	// Execute runs a coding task in the session.
	Execute(ctx context.Context, sessionID string, task Task) (*Result, error)

	// Git operations - executed by the backend, not locally

	// GitCommit commits changes in the session workspace.
	GitCommit(ctx context.Context, sessionID string, message string, addAll bool) (*GitCommitResult, error)

	// GitPush pushes commits to the remote repository.
	GitPush(ctx context.Context, sessionID string, remote, branch string, setUpstream bool) (*GitPushResult, error)

	// GitBranch creates and/or switches to a branch.
	GitBranch(ctx context.Context, sessionID string, branch string, create bool) (*GitBranchResult, error)
}

// SessionOptions configures how a coding session is created.
type SessionOptions struct {
	// WorkspacePath is an explicit workspace path (optional for remote backends).
	// If empty, the backend creates a workspace automatically.
	WorkspacePath string

	// Title is an optional session title for identification.
	Title string

	// RepoURL is the git repository URL to clone (optional).
	// If provided, the backend clones the repo into the workspace.
	RepoURL string

	// Branch is the git branch to checkout (optional, defaults to default branch).
	Branch string

	// GitCredentials contains authentication for git operations.
	GitCredentials *GitCredentials

	// ExistingSessionID allows continuing an existing OpenCode session.
	// If provided, CreateSession will attach to this session instead of creating a new one.
	// This enables session continuity across multiple coding tool invocations.
	ExistingSessionID string
}
