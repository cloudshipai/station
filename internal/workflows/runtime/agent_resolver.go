package runtime

import (
	"context"
)

// AgentResolver defines the interface for resolving agents by ID or name.
// This allows decoupling agent resolution from execution for easier testing.
type AgentResolver interface {
	GetAgentByID(ctx context.Context, id int64) (AgentInfo, error)
	GetAgentByNameAndEnvironment(ctx context.Context, name string, environmentID int64) (AgentInfo, error)
	GetAgentByNameGlobal(ctx context.Context, name string) (AgentInfo, error)
	GetEnvironmentIDByName(ctx context.Context, name string) (int64, error)
}
