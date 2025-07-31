package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
	oai "github.com/firebase/genkit/go/plugins/compat_oai/openai"

	"station/internal/db/repositories"
	"station/pkg/models"
)

// GenkitService provides Genkit-based AI agent execution integrated with Station
// Now supports cross-environment agents with tool namespacing
type GenkitService struct {
	genkitApp            *genkit.Genkit
	openaiPlugin         *oai.OpenAI
	mcpManager           *mcp.MCPManager
	agentRepo            *repositories.AgentRepo
	agentRunRepo         *repositories.AgentRunRepo
	mcpConfigRepo        *repositories.MCPConfigRepo
	agentToolRepo        *repositories.AgentToolRepo
	agentEnvironmentRepo *repositories.AgentEnvironmentRepo
	mcpConfigService     *MCPConfigService
}

// NewGenkitService creates a new Genkit service with cross-environment support
func NewGenkitService(
	genkitApp *genkit.Genkit,
	openaiPlugin *oai.OpenAI,
	agentRepo *repositories.AgentRepo,
	agentRunRepo *repositories.AgentRunRepo,
	mcpConfigRepo *repositories.MCPConfigRepo,
	agentToolRepo *repositories.AgentToolRepo,
	agentEnvironmentRepo *repositories.AgentEnvironmentRepo,
	mcpConfigService *MCPConfigService,
) *GenkitService {
	return &GenkitService{
		genkitApp:            genkitApp,
		openaiPlugin:         openaiPlugin,
		agentRepo:            agentRepo,
		agentRunRepo:         agentRunRepo,
		mcpConfigRepo:        mcpConfigRepo,
		agentToolRepo:        agentToolRepo,
		agentEnvironmentRepo: agentEnvironmentRepo,
		mcpConfigService:     mcpConfigService,
	}
}

// InitializeMCP sets up MCP manager with configured servers for ALL environments
// This enables cross-environment agent tool access with environment namespacing
func (s *GenkitService) InitializeMCP(ctx context.Context) error {
	// Get all environments that have MCP configs
	allConfigs, err := s.mcpConfigRepo.GetAllLatestConfigs()
	if err != nil {
		return fmt.Errorf("failed to get all MCP configs: %w", err)
	}

	// For now, initialize with empty config to keep it simple
	// This can be extended later to load actual cross-environment MCP configs
	manager, err := mcp.NewMCPManager(mcp.MCPManagerOptions{
		Name:       "station-cross-environment",
		Version:    "1.0.0",
		MCPServers: []mcp.MCPServerConfig{}, // Empty for now
	})
	if err != nil {
		return fmt.Errorf("failed to create MCP manager: %w", err)
	}

	s.mcpManager = manager
	log.Printf("Initialized cross-environment Genkit MCP manager with %d configs", len(allConfigs))
	return nil
}

// ExecuteAgent implements AgentServiceInterface for compatibility with ExecutionQueue and MCP server
// This wrapper converts our 3-parameter method to the 2-parameter interface expected by existing code
func (s *GenkitService) ExecuteAgent(ctx context.Context, agentID int64, task string) (*Message, error) {
	// Use a default userID of 0 for system executions (ExecutionQueue, MCP)
	const systemUserID = 0
	
	// Call our internal 3-parameter method
	agentRun, execErr := s.executeAgentInternal(ctx, agentID, systemUserID, task)
	if agentRun == nil {
		// If no run was created at all, return the error
		return nil, execErr
	}
	
	// Convert AgentRun to Message for interface compatibility
	message := &Message{
		Content: agentRun.FinalResponse,
		Role:    RoleAssistant,
		Extra: map[string]interface{}{
			"message_id":   fmt.Sprintf("run-%d", agentRun.ID),
			"agent_id":     agentRun.AgentID,
			"user_id":      agentRun.UserID,
			"status":       agentRun.Status,
			"steps_taken":  agentRun.StepsTaken,
			"started_at":   agentRun.StartedAt,
			"completed_at": agentRun.CompletedAt,
			"run_id":       agentRun.ID,
		},
	}
	
	// Return the message and preserve the execution error if there was one
	return message, execErr
}

// executeAgentInternal is our original ExecuteAgent method renamed for internal use
func (s *GenkitService) executeAgentInternal(ctx context.Context, agentID, userID int64, task string) (*models.AgentRun, error) {
	// Get agent from database
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Get model for the agent (defaulting to gpt-4o-mini)
	model := s.openaiPlugin.Model(s.genkitApp, "gpt-4o-mini")

	// Get available MCP tools (empty for now, can be extended)
	var tools []ai.Tool
	if s.mcpManager != nil {
		mcpTools, err := s.mcpManager.GetActiveTools(ctx, s.genkitApp)
		if err != nil {
			log.Printf("Failed to get MCP tools: %v", err)
		} else {
			tools = mcpTools
		}
	}

	// Convert tools to ToolRef
	var toolRefs []ai.ToolRef
	for _, tool := range tools {
		toolRefs = append(toolRefs, tool)
	}

	// Create messages with agent prompt and user task
	var messages []*ai.Message
	if agent.Prompt != "" {
		messages = append(messages, ai.NewSystemTextMessage(agent.Prompt))
	}
	messages = append(messages, ai.NewUserTextMessage(task))

	log.Printf("Executing agent %d with %d tools", agentID, len(tools))

	// Execute using Genkit
	startTime := time.Now()
	
	// Build generate options
	generateOptions := []ai.GenerateOption{
		ai.WithModel(model),
		ai.WithMessages(messages...),
	}
	
	// Only add tool options if we have tools
	if len(toolRefs) > 0 {
		generateOptions = append(generateOptions, 
			ai.WithTools(toolRefs...),
			ai.WithToolChoice(ai.ToolChoiceAuto),
		)
	}
	
	response, err := genkit.Generate(ctx, s.genkitApp, generateOptions...)

	// Create agent run record
	status := "failed"
	finalResponse := ""
	var completedAt *time.Time
	
	if err != nil {
		finalResponse = fmt.Sprintf("Error: %v", err)
		completedAt = &[]time.Time{time.Now()}[0]
	} else {
		status = "completed"
		finalResponse = response.Text()
		completedAt = &[]time.Time{time.Now()}[0]
	}

	// Create execution steps
	executionSteps := &models.JSONArray{
		models.ExecutionStep{
			StepNumber: 1,
			Action:     "generate_response",
			Response:   finalResponse,
			Timestamp:  startTime,
		},
	}

	// Create the agent run
	run, createErr := s.agentRunRepo.Create(
		agentID,
		userID,
		task,
		finalResponse,
		1, // stepsTaken
		&models.JSONArray{}, // toolCalls (empty for now)
		executionSteps,
		status,
		completedAt,
	)

	if createErr != nil {
		log.Printf("Failed to create agent run record: %v", createErr)
		// Return the original error if generation failed, otherwise the create error
		if err != nil {
			return nil, err
		}
		return nil, createErr
	}

	if err != nil {
		return run, err
	}

	log.Printf("Agent %d execution completed successfully", agentID)
	return run, nil
}

// GetAgentRun retrieves an agent run by ID
func (s *GenkitService) GetAgentRun(runID int64) (*models.AgentRun, error) {
	return s.agentRunRepo.GetByID(runID)
}

// GetAgentRunsForAgent gets recent runs for a specific agent
func (s *GenkitService) GetAgentRunsForAgent(agentID int64, limit int) ([]*models.AgentRun, error) {
	return s.agentRunRepo.ListByAgent(agentID)
}

// AddMCPServer dynamically adds an MCP server to the manager
func (s *GenkitService) AddMCPServer(ctx context.Context, name string, config mcp.MCPClientOptions) error {
	if s.mcpManager == nil {
		return fmt.Errorf("MCP manager not initialized")
	}

	return s.mcpManager.Connect(ctx, name, config)
}

// GetAvailableTools returns all available MCP tools
func (s *GenkitService) GetAvailableTools(ctx context.Context) ([]ai.Tool, error) {
	if s.mcpManager == nil {
		return []ai.Tool{}, nil
	}

	return s.mcpManager.GetActiveTools(ctx, s.genkitApp)
}

// Close cleans up the Genkit service
func (s *GenkitService) Close(ctx context.Context) error {
	// Note: Genkit MCP manager doesn't expose a direct Close method
	// This is a placeholder for cleanup logic
	log.Printf("Closing cross-environment Genkit service")
	return nil
}

// ====================================================================================
// AgentServiceInterface Implementation - Additional Methods for MCP Server Compatibility
// ====================================================================================

// CreateAgent implements AgentServiceInterface for MCP compatibility  
func (s *GenkitService) CreateAgent(ctx context.Context, config *AgentConfig) (*models.Agent, error) {
	// Create the agent record
	agent, err := s.agentRepo.Create(
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

	// TODO: Handle AssignedTools, ModelProvider, ModelID
	// For now, we'll ignore these fields since our current implementation
	// doesn't yet support per-agent model configuration
	
	return agent, nil
}

// GetAgent implements AgentServiceInterface for MCP server compatibility
func (s *GenkitService) GetAgent(ctx context.Context, agentID int64) (*models.Agent, error) {
	return s.agentRepo.GetByID(agentID)
}

// ListAgentsByEnvironment implements AgentServiceInterface for MCP server compatibility
func (s *GenkitService) ListAgentsByEnvironment(ctx context.Context, environmentID int64) ([]*models.Agent, error) {
	return s.agentRepo.ListByEnvironment(environmentID)
}

// UpdateAgent implements AgentServiceInterface for MCP server compatibility
func (s *GenkitService) UpdateAgent(ctx context.Context, agentID int64, config *AgentConfig) (*models.Agent, error) {
	// Update the agent record
	err := s.agentRepo.Update(
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

	// TODO: Handle AssignedTools, ModelProvider, ModelID updates
	// For now, we'll ignore these fields since our current implementation
	// doesn't yet support per-agent model configuration
	
	// Get the updated agent to return
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated agent: %w", err)
	}
	
	return agent, nil
}

// DeleteAgent implements AgentServiceInterface for MCP server compatibility
func (s *GenkitService) DeleteAgent(ctx context.Context, agentID int64) error {
	return s.agentRepo.Delete(agentID)
}

// ExecuteAgentWithUser is the new method that provides the full 3-parameter signature
// This should be used by new code that can provide the userID
func (s *GenkitService) ExecuteAgentWithUser(ctx context.Context, agentID, userID int64, task string) (*models.AgentRun, error) {
	return s.executeAgentInternal(ctx, agentID, userID, task)
}