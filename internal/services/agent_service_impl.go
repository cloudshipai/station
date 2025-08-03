package services

import (
	"context"
	"fmt"

	"station/internal/db/repositories"
	"station/pkg/models"
)

// AgentService implements AgentServiceInterface using IntelligentAgentCreator
type AgentService struct {
	repos   *repositories.Repositories
	creator *IntelligentAgentCreator
}

// NewAgentService creates a new agent service
func NewAgentService(repos *repositories.Repositories) *AgentService {
	creator := NewIntelligentAgentCreator(repos, nil) // Self-reference will be set later
	service := &AgentService{
		repos:   repos,
		creator: creator,
	}
	// Set the self-reference for the creator
	creator.agentService = service
	return service
}

// ExecuteAgent executes an agent with a specific task
func (s *AgentService) ExecuteAgent(ctx context.Context, agentID int64, task string) (*Message, error) {
	// Get the agent details
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent %d: %w", agentID, err)
	}

	// Execute using IntelligentAgentCreator with stdio MCP
	result, err := s.creator.ExecuteAgentViaStdioMCP(ctx, agent, task, 0) // Run ID 0 for MCP calls
	if err != nil {
		return nil, fmt.Errorf("failed to execute agent via stdio MCP: %w", err)
	}

	// Convert result to Message format
	extra := map[string]interface{}{
		"agent_id":     agent.ID,
		"agent_name":   agent.Name,
		"steps_taken":  result.StepsTaken,
		"tool_calls":   result.ToolCalls,
		"execution_steps": result.ExecutionSteps,
	}

	return &Message{
		Content: result.Response,
		Role:    RoleAssistant,
		Extra:   extra,
	}, nil
}

// CreateAgent creates a new agent
func (s *AgentService) CreateAgent(ctx context.Context, config *AgentConfig) (*models.Agent, error) {
	// Create agent using repository
	agent, err := s.repos.Agents.Create(
		config.Name,
		config.Description,
		config.Prompt,
		config.MaxSteps,
		config.EnvironmentID,
		config.CreatedBy,
		config.CronSchedule,
		config.ScheduleEnabled,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Assign tools if provided
	if len(config.AssignedTools) > 0 {
		assignedCount := s.creator.assignToolsToAgent(agent.ID, config.AssignedTools, config.EnvironmentID)
		if assignedCount == 0 {
			return nil, fmt.Errorf("failed to assign any tools to agent")
		}
	}

	return agent, nil
}

// GetAgent retrieves an agent by ID
func (s *AgentService) GetAgent(ctx context.Context, agentID int64) (*models.Agent, error) {
	return s.repos.Agents.GetByID(agentID)
}

// ListAgentsByEnvironment lists agents in a specific environment
func (s *AgentService) ListAgentsByEnvironment(ctx context.Context, environmentID int64) ([]*models.Agent, error) {
	return s.repos.Agents.ListByEnvironment(environmentID)
}

// UpdateAgent updates an agent's configuration
func (s *AgentService) UpdateAgent(ctx context.Context, agentID int64, config *AgentConfig) (*models.Agent, error) {
	// Update agent using repository
	err := s.repos.Agents.Update(
		agentID,
		config.Name,
		config.Description,
		config.Prompt,
		config.MaxSteps,
		config.CronSchedule,
		config.ScheduleEnabled,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	// Update tool assignments if provided
	if len(config.AssignedTools) > 0 {
		// Clear existing assignments
		if err := s.repos.AgentTools.Clear(agentID); err != nil {
			return nil, fmt.Errorf("failed to clear existing tool assignments: %w", err)
		}
		
		// Assign new tools
		assignedCount := s.creator.assignToolsToAgent(agentID, config.AssignedTools, config.EnvironmentID)
		if assignedCount == 0 {
			return nil, fmt.Errorf("failed to assign any tools to agent")
		}
	}

	return s.repos.Agents.GetByID(agentID)
}

// DeleteAgent deletes an agent
func (s *AgentService) DeleteAgent(ctx context.Context, agentID int64) error {
	// Clear tool assignments first
	if err := s.repos.AgentTools.Clear(agentID); err != nil {
		return fmt.Errorf("failed to clear tool assignments: %w", err)
	}
	
	// Delete the agent
	return s.repos.Agents.Delete(agentID)
}

// InitializeMCP initializes MCP for the agent service
func (s *AgentService) InitializeMCP(ctx context.Context) error {
	// Test the stdio MCP connection
	return s.creator.TestStdioMCPConnection(ctx)
}