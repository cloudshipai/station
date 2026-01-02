package coding

import (
	"context"
)

// NATSClient defines the interface for NATS coding operations.
// This interface enables testing NATSBackend with mocks.
type NATSClient interface {
	// IsConnected returns true if the NATS connection is active.
	IsConnected() bool

	// Close closes the NATS connection.
	Close() error

	// ExecuteTask publishes a task and returns a TaskExecution for tracking.
	ExecuteTask(ctx context.Context, task *CodingTask) (*TaskExecution, error)

	// GetSession retrieves session state from KV.
	GetSession(ctx context.Context, name string) (*SessionState, error)

	// SaveSession stores session state in KV.
	SaveSession(ctx context.Context, state *SessionState) error

	// DeleteSession removes session state from KV.
	DeleteSession(ctx context.Context, name string) error

	// GetState retrieves arbitrary state from KV.
	GetState(ctx context.Context, key string) ([]byte, error)

	// SetState stores arbitrary state in KV.
	SetState(ctx context.Context, key string, value []byte) error

	// DeleteState removes arbitrary state from KV.
	DeleteState(ctx context.Context, key string) error
}

// Verify NATSCodingClient implements NATSClient interface
var _ NATSClient = (*NATSCodingClient)(nil)
