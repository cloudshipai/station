package services

import (
	"context"

	"station/pkg/models"
)

// AgentServiceInterface defines the interface that GenkitService implements
// This ensures compatibility with ExecutionQueueService and MCP server handlers
type AgentServiceInterface interface {
	// Core agent execution - used by ExecutionQueueService and MCP server
	ExecuteAgent(ctx context.Context, agentID int64, task string) (*Message, error)
	
	// Agent management - used by MCP server
	CreateAgent(ctx context.Context, config *AgentConfig) (*models.Agent, error)
	GetAgent(ctx context.Context, agentID int64) (*models.Agent, error)
	ListAgentsByEnvironment(ctx context.Context, environmentID int64) ([]*models.Agent, error)
	UpdateAgent(ctx context.Context, agentID int64, config *AgentConfig) (*models.Agent, error)
	DeleteAgent(ctx context.Context, agentID int64) error
}

// AgentConfig represents the configuration for creating/updating an agent
// This struct is used by GenkitService for agent management
type AgentConfig struct {
	EnvironmentID int64    `json:"environment_id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Prompt        string   `json:"prompt"`        // System prompt (matches db field name)
	AssignedTools []string `json:"assigned_tools"` // List of tool names to assign to this agent
	MaxSteps      int64    `json:"max_steps"`     // Maximum steps for agent execution (matches db field type)
	CreatedBy     int64    `json:"created_by"`    // User ID who created the agent
	ModelProvider string   `json:"model_provider,omitempty"` // Model provider (e.g., "openai", "anthropic")
	ModelID       string   `json:"model_id,omitempty"`       // Specific model ID (e.g., "gpt-4o", "claude-3-5-sonnet-20241022")
	CronSchedule  *string  `json:"cron_schedule,omitempty"`  // Cron schedule for automated runs
	ScheduleEnabled bool   `json:"schedule_enabled"`         // Whether scheduled runs are enabled
}