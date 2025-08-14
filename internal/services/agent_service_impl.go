package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

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

// ExecuteAgent executes an agent with a specific task and optional user variables
func (s *AgentService) ExecuteAgent(ctx context.Context, agentID int64, task string, userVariables map[string]interface{}) (*Message, error) {
	// Default to empty variables if nil provided
	if userVariables == nil {
		userVariables = make(map[string]interface{})
	}
	
	// Get the agent details
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent %d: %w", agentID, err)
	}

	log.Printf("DEBUG AgentService: About to execute agent %d (%s) with %d variables", agent.ID, agent.Name, len(userVariables))
	
	// CRITICAL FIX: Create fresh execution context like CLI does
	// This prevents shared state issues that cause STDIO pipe transport failures
	log.Printf("DEBUG AgentService: Creating fresh execution context to avoid shared state issues")
	freshCreator := NewIntelligentAgentCreator(s.repos, nil) // Fresh creator instance
	
	// Execute using fresh IntelligentAgentCreator with stdio MCP and user variables
	result, err := freshCreator.ExecuteAgentViaStdioMCP(ctx, agent, task, 0, userVariables) // Run ID 0 for MCP calls
	
	log.Printf("DEBUG AgentService: Fresh execution context ExecuteAgentViaStdioMCP returned for agent %d, error: %v", agent.ID, err)
	if err != nil {
		return nil, fmt.Errorf("failed to execute agent via stdio MCP: %w", err)
	}

	// Convert result to Message format with proper types for execution queue
	extra := map[string]interface{}{
		"agent_id":     agent.ID,
		"agent_name":   agent.Name,
		"steps_taken":  result.StepsTaken,
	}
	
	// Include variables in response for tracking if provided
	if len(userVariables) > 0 {
		extra["user_variables"] = userVariables
	}
	
	// Add tool calls and execution steps directly (they're already *models.JSONArray)
	if result.ToolCalls != nil {
		extra["tool_calls"] = result.ToolCalls
	}
	
	if result.ExecutionSteps != nil {
		extra["execution_steps"] = result.ExecutionSteps
	}
	
	// Preserve rich GenKit response object data from Station's OpenAI plugin
	if result.TokenUsage != nil {
		extra["token_usage"] = result.TokenUsage
	}
	
	if result.Duration > 0 {
		extra["duration"] = result.Duration.Seconds()
	}
	
	if result.ModelName != "" {
		extra["model_name"] = result.ModelName
	}
	
	if result.ToolsUsed > 0 {
		extra["tools_used"] = result.ToolsUsed
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
	// Get agent details before deletion for file cleanup
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent %d for deletion: %w", agentID, err)
	}

	// Get environment name for file path construction
	environment, err := s.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		return fmt.Errorf("failed to get environment %d for agent %d: %w", agent.EnvironmentID, agentID, err)
	}

	// Clear tool assignments first
	if err := s.repos.AgentTools.Clear(agentID); err != nil {
		return fmt.Errorf("failed to clear tool assignments: %w", err)
	}
	
	// Delete the agent from database
	if err := s.repos.Agents.Delete(agentID); err != nil {
		return fmt.Errorf("failed to delete agent from database: %w", err)
	}

	// Clean up .prompt file from filesystem
	if err := s.deleteAgentPromptFile(agent.Name, environment.Name); err != nil {
		log.Printf("Warning: Failed to delete .prompt file for agent %s: %v", agent.Name, err)
		// Don't fail the entire operation if file cleanup fails
	}

	return nil
}

// deleteAgentPromptFile removes the .prompt file for an agent from the filesystem
func (s *AgentService) deleteAgentPromptFile(agentName, environmentName string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	promptFilePath := filepath.Join(homeDir, ".config", "station", "environments", environmentName, "agents", agentName+".prompt")
	
	// Check if file exists before attempting deletion
	if _, err := os.Stat(promptFilePath); os.IsNotExist(err) {
		return nil // Not an error if file doesn't exist
	}

	// Remove the .prompt file
	if err := os.Remove(promptFilePath); err != nil {
		return fmt.Errorf("failed to delete .prompt file %s: %w", promptFilePath, err)
	}

	log.Printf("Successfully deleted .prompt file: %s", promptFilePath)
	return nil
}

// InitializeMCP initializes MCP for the agent service
func (s *AgentService) InitializeMCP(ctx context.Context) error {
	// Test the stdio MCP connection
	return s.creator.TestStdioMCPConnection(ctx)
}