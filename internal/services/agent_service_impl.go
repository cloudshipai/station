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

// AgentService implements AgentServiceInterface using AgentExecutionEngine directly
type AgentService struct {
	repos           *repositories.Repositories
	executionEngine *AgentExecutionEngine
}

// NewAgentService creates a new agent service
func NewAgentService(repos *repositories.Repositories) *AgentService {
	service := &AgentService{
		repos: repos,
	}
	// Create execution engine with self-reference
	service.executionEngine = NewAgentExecutionEngine(repos, service)
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
	
	// Execute using AgentExecutionEngine directly with stdio MCP and user variables
	result, err := s.executionEngine.ExecuteAgentViaStdioMCPWithVariables(ctx, agent, task, 0, userVariables) // Run ID 0 for MCP calls
	
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

// ExecuteAgentWithRunID executes an agent with proper run ID for logging - used by ExecutionQueueService
func (s *AgentService) ExecuteAgentWithRunID(ctx context.Context, agentID int64, task string, runID int64, userVariables map[string]interface{}) (*Message, error) {
	// Default to empty variables if nil provided
	if userVariables == nil {
		userVariables = make(map[string]interface{})
	}
	
	// Get the agent details
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent %d: %w", agentID, err)
	}

	log.Printf("DEBUG AgentService: About to execute agent %d (%s) with run ID %d and %d variables", agent.ID, agent.Name, runID, len(userVariables))
	
	// Execute using AgentExecutionEngine directly with stdio MCP and user variables - PASS THE REAL RUN ID
	result, err := s.executionEngine.ExecuteAgentViaStdioMCPWithVariables(ctx, agent, task, runID, userVariables) // Use real run ID!
	
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
		nil, // input_schema - not set in basic config
		config.CronSchedule,
		config.ScheduleEnabled,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Assign tools if provided
	if len(config.AssignedTools) > 0 {
		assignedCount := s.assignToolsToAgent(agent.ID, config.AssignedTools, config.EnvironmentID)
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
		nil, // input_schema - not set in basic config
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
		assignedCount := s.assignToolsToAgent(agentID, config.AssignedTools, config.EnvironmentID)
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
	return s.executionEngine.TestStdioMCPConnection(ctx)
}

// assignToolsToAgent assigns tools to an agent and returns the count of tools assigned
func (s *AgentService) assignToolsToAgent(agentID int64, toolNames []string, environmentID int64) int {
	assignedCount := 0
	
	for _, toolName := range toolNames {
		// Try to find the tool in the MCP tools table
		tool, err := s.repos.MCPTools.FindByNameInEnvironment(environmentID, toolName)
		if err != nil {
			// Tool doesn't exist in MCP tools table, create it
			log.Printf("Creating new MCP tool entry for: %s", toolName)
			mcpTool := &models.MCPTool{
				Name: toolName,
				Description: fmt.Sprintf("Auto-discovered tool: %s", toolName),
			}
			toolID, err := s.repos.MCPTools.Create(mcpTool)
			if err != nil {
				log.Printf("Failed to create MCP tool %s: %v", toolName, err)
				continue
			}
			tool = &models.MCPTool{ID: toolID, Name: toolName}
		}
		
		// Assign the tool to the agent
		_, err = s.repos.AgentTools.AddAgentTool(agentID, tool.ID)
		if err != nil {
			log.Printf("Failed to assign tool %s to agent %d: %v", toolName, agentID, err)
			continue
		}
		
		assignedCount++
		log.Printf("Assigned tool '%s' (ID: %d) to agent %d", toolName, tool.ID, agentID)
	}
	
	return assignedCount
}

// GetExecutionEngine returns the execution engine for direct access (used by CLI)
func (s *AgentService) GetExecutionEngine() *AgentExecutionEngine {
	return s.executionEngine
}