package services

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"station/internal/db/repositories"
	"station/pkg/models"
)

// EinoAgentService manages Eino-based AI agents with MCP tool integration
type EinoAgentService struct {
	repos                *repositories.Repositories
	mcpClientService     *MCPClientService
	toolDiscoveryService *ToolDiscoveryService
	mcpToolsLoader       *MCPToolsLoader
	modelProvider        *ModelProviderService
}

// NewEinoAgentService creates a new Eino agent service
func NewEinoAgentService(
	repos *repositories.Repositories,
	mcpClientService *MCPClientService,
	toolDiscoveryService *ToolDiscoveryService,
) *EinoAgentService {
	return &EinoAgentService{
		repos:                repos,
		mcpClientService:     mcpClientService,
		toolDiscoveryService: toolDiscoveryService,
		mcpToolsLoader:       NewMCPToolsLoader(mcpClientService, toolDiscoveryService),
		modelProvider:        NewModelProviderService(),
	}
}

// AgentConfig represents the configuration for creating an agent
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
}

// CreateAgent creates a new AI agent with specified tools
func (s *EinoAgentService) CreateAgent(ctx context.Context, config *AgentConfig) (*models.Agent, error) {
	// Validate that the environment exists
	_, err := s.repos.Environments.GetByID(config.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment %d: %w", config.EnvironmentID, err)
	}

	// Validate that all assigned tools exist in the environment
	availableTools, err := s.toolDiscoveryService.GetToolsByEnvironment(config.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get available tools for environment %d: %w", config.EnvironmentID, err)
	}

	// Create a map of available tools for validation
	availableToolMap := make(map[string]bool)
	for _, tool := range availableTools {
		availableToolMap[tool.Name] = true
	}

	// Validate assigned tools
	for _, toolName := range config.AssignedTools {
		if !availableToolMap[toolName] {
			return nil, fmt.Errorf("tool %s is not available in environment %d", toolName, config.EnvironmentID)
		}
	}

	// Create the agent record in database using the repository's Create method
	agent, err := s.repos.Agents.Create(
		config.Name,
		config.Description,
		config.Prompt,
		config.MaxSteps,
		config.EnvironmentID,
		config.CreatedBy,
		nil,   // cronSchedule - not set through this service  
		false, // scheduleEnabled - not set through this service
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Create tool assignments via AgentTool relationships
	if len(config.AssignedTools) > 0 {
		// Get tool IDs by names
		toolIDs, err := s.getToolIDsByNames(config.EnvironmentID, config.AssignedTools)
		if err != nil {
			// Clean up created agent on error
			s.repos.Agents.Delete(agent.ID)
			return nil, fmt.Errorf("failed to get tool IDs: %w", err)
		}

		// Create AgentTool relationships
		for _, toolID := range toolIDs {
			_, err := s.repos.AgentTools.Add(agent.ID, toolID)
			if err != nil {
				// Clean up on error - delete agent and any created tool relationships
				s.repos.Agents.Delete(agent.ID)
				return nil, fmt.Errorf("failed to create agent tool relationship: %w", err)
			}
		}
	}

	return agent, nil
}

// ExecuteAgent runs an agent with the given input and returns the response
func (s *EinoAgentService) ExecuteAgent(ctx context.Context, agentID int64, input string) (*schema.Message, error) {
	// Get the agent configuration
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent %d: %w", agentID, err)
	}

	// Get the assigned tool names for this agent
	assignedToolNames, err := s.getAssignedToolNames(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assigned tools for agent %d: %w", agentID, err)
	}

	// Load the assigned MCP tools for this agent
	einoTools, err := s.mcpToolsLoader.LoadSpecificTools(agent.EnvironmentID, assignedToolNames)
	if err != nil {
		return nil, fmt.Errorf("failed to load tools for agent %d: %w", agentID, err)
	}

	// Create the chat model based on agent configuration or default
	chatModel, err := s.createChatModel(agent)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model for agent %d: %w", agentID, err)
	}

	// Configure the ReAct agent
	maxSteps := int(agent.MaxSteps) // Convert int64 to int
	if maxSteps <= 0 {
		maxSteps = 12 // default from Eino
	}

	// Convert InvokableTool slice to BaseTool slice (all InvokableTool implement BaseTool)
	baseTools := make([]tool.BaseTool, len(einoTools))
	for i, einoTool := range einoTools {
		baseTools[i] = einoTool
	}

	agentConfig := &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: baseTools,
		},
		MessageModifier: func(ctx context.Context, input []*schema.Message) []*schema.Message {
			// Add system prompt if provided
			if agent.Prompt != "" {
				systemMsg := &schema.Message{
					Role:    schema.System,
					Content: agent.Prompt,
				}
				return append([]*schema.Message{systemMsg}, input...)
			}
			return input
		},
		MaxStep: maxSteps,
	}

	// Create the ReAct agent
	reactAgent, err := react.NewAgent(ctx, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ReAct agent: %w", err)
	}

	// Prepare the input message
	inputMessages := []*schema.Message{
		{
			Role:    schema.User,
			Content: input,
		},
	}

	// Execute the agent
	result, err := reactAgent.Generate(ctx, inputMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to execute agent: %w", err)
	}

	return result, nil
}

// GetAgent retrieves an agent by ID
func (s *EinoAgentService) GetAgent(ctx context.Context, agentID int64) (*models.Agent, error) {
	return s.repos.Agents.GetByID(agentID)
}

// ListAgentsByEnvironment lists all agents in an environment
func (s *EinoAgentService) ListAgentsByEnvironment(ctx context.Context, environmentID int64) ([]*models.Agent, error) {
	return s.repos.Agents.ListByEnvironment(environmentID)
}

// UpdateAgent updates an existing agent
func (s *EinoAgentService) UpdateAgent(ctx context.Context, agentID int64, config *AgentConfig) (*models.Agent, error) {
	// Get existing agent
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent %d: %w", agentID, err)
	}

	// Validate assigned tools if they changed
	if len(config.AssignedTools) > 0 {
		availableTools, err := s.toolDiscoveryService.GetToolsByEnvironment(agent.EnvironmentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get available tools: %w", err)
		}

		availableToolMap := make(map[string]bool)
		for _, tool := range availableTools {
			availableToolMap[tool.Name] = true
		}

		for _, toolName := range config.AssignedTools {
			if !availableToolMap[toolName] {
				return nil, fmt.Errorf("tool %s is not available in environment %d", toolName, agent.EnvironmentID)
			}
		}
	}

	// Update agent fields
	if config.Name != "" {
		agent.Name = config.Name
	}
	if config.Description != "" {
		agent.Description = config.Description
	}
	if config.Prompt != "" {
		agent.Prompt = config.Prompt
	}
	if config.MaxSteps > 0 {
		agent.MaxSteps = config.MaxSteps
	}

	// Save updates (preserve existing schedule settings)
	if err := s.repos.Agents.Update(agent.ID, agent.Name, agent.Description, agent.Prompt, agent.MaxSteps, agent.CronSchedule, agent.ScheduleEnabled); err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	// Update tool assignments if provided
	if len(config.AssignedTools) > 0 {
		// Delete existing tool assignments - we'll implement a simple approach
		// by removing them one by one since we don't have a batch delete method
		existingToolNames, err := s.getAssignedToolNames(agentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing tool assignments: %w", err)
		}

		// Get existing tool IDs for removal
		if len(existingToolNames) > 0 {
			existingToolIDs, err := s.getToolIDsByNames(agent.EnvironmentID, existingToolNames)
			if err != nil {
				return nil, fmt.Errorf("failed to get existing tool IDs: %w", err)
			}

			// Remove existing assignments
			for _, toolID := range existingToolIDs {
				err := s.repos.AgentTools.Remove(agentID, toolID)
				if err != nil {
					return nil, fmt.Errorf("failed to remove agent tool assignment: %w", err)
				}
			}
		}

		// Get tool IDs by names for new assignments
		toolIDs, err := s.getToolIDsByNames(agent.EnvironmentID, config.AssignedTools)
		if err != nil {
			return nil, fmt.Errorf("failed to get tool IDs: %w", err)
		}

		// Create new AgentTool relationships
		for _, toolID := range toolIDs {
			_, err := s.repos.AgentTools.Add(agentID, toolID)
			if err != nil {
				return nil, fmt.Errorf("failed to create agent tool relationship: %w", err)
			}
		}
	}

	return agent, nil
}

// DeleteAgent deletes an agent
func (s *EinoAgentService) DeleteAgent(ctx context.Context, agentID int64) error {
	return s.repos.Agents.Delete(agentID)
}

// getToolIDsByNames converts tool names to tool IDs for a given environment
func (s *EinoAgentService) getToolIDsByNames(environmentID int64, toolNames []string) ([]int64, error) {
	tools, err := s.toolDiscoveryService.GetToolsByEnvironment(environmentID)
	if err != nil {
		return nil, err
	}

	toolMap := make(map[string]int64)
	for _, tool := range tools {
		toolMap[tool.Name] = tool.ID
	}

	toolIDs := make([]int64, 0, len(toolNames))
	for _, toolName := range toolNames {
		if toolID, exists := toolMap[toolName]; exists {
			toolIDs = append(toolIDs, toolID)
		} else {
			return nil, fmt.Errorf("tool %s not found in environment %d", toolName, environmentID)
		}
	}

	return toolIDs, nil
}

// getAssignedToolNames gets the tool names assigned to an agent
func (s *EinoAgentService) getAssignedToolNames(agentID int64) ([]string, error) {
	// Get agent tools with details (includes tool names)
	agentToolsWithDetails, err := s.repos.AgentTools.List(agentID)
	if err != nil {
		return nil, err
	}

	toolNames := make([]string, len(agentToolsWithDetails))
	for i, agentTool := range agentToolsWithDetails {
		toolNames[i] = agentTool.ToolName
	}

	return toolNames, nil
}

// createChatModel creates a chat model for the agent based on its configuration
func (s *EinoAgentService) createChatModel(agent *models.Agent) (model.ToolCallingChatModel, error) {
	// For now, agent model configuration is stored as simple fields
	// In a more advanced implementation, you could store JSON model config in a separate field
	
	// Try to use default model provider if no specific configuration
	chatModel, err := s.modelProvider.CreateDefaultChatModel()
	if err != nil {
		return nil, fmt.Errorf("failed to create default chat model: %w", err)
	}
	
	return chatModel, nil
}

// GetAvailableModels returns all available model providers and their models
func (s *EinoAgentService) GetAvailableModels() map[string]*ModelProvider {
	return s.modelProvider.GetProviders()
}

// CreateChatModelByConfig creates a chat model using specific provider and model configuration
func (s *EinoAgentService) CreateChatModelByConfig(providerName, modelID string) (model.ToolCallingChatModel, error) {
	return s.modelProvider.CreateChatModel(providerName, modelID)
}