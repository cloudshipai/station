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
type GenkitService struct {
	genkitApp       *genkit.Genkit
	openaiPlugin    *oai.OpenAI
	mcpManager      *mcp.MCPManager
	agentRepo       *repositories.AgentRepo
	agentRunRepo    *repositories.AgentRunRepo
	mcpConfigRepo   *repositories.MCPConfigRepo
	environmentID   int64
}

// NewGenkitService creates a new Genkit service
func NewGenkitService(
	genkitApp *genkit.Genkit,
	openaiPlugin *oai.OpenAI,
	agentRepo *repositories.AgentRepo,
	agentRunRepo *repositories.AgentRunRepo,
	mcpConfigRepo *repositories.MCPConfigRepo,
	environmentID int64,
) *GenkitService {
	return &GenkitService{
		genkitApp:     genkitApp,
		openaiPlugin:  openaiPlugin,
		agentRepo:     agentRepo,
		agentRunRepo:  agentRunRepo,
		mcpConfigRepo: mcpConfigRepo,
		environmentID: environmentID,
	}
}

// InitializeMCP sets up MCP manager with configured servers for the environment
func (s *GenkitService) InitializeMCP(ctx context.Context) error {
	// For now, we'll initialize without MCP configs to keep it simple
	// This can be extended later to load actual MCP configs from the database
	
	manager, err := mcp.NewMCPManager(mcp.MCPManagerOptions{
		Name:       fmt.Sprintf("station-env-%d", s.environmentID),
		Version:    "1.0.0",
		MCPServers: []mcp.MCPServerConfig{}, // Empty for now
	})
	if err != nil {
		return fmt.Errorf("failed to create MCP manager: %w", err)
	}

	s.mcpManager = manager
	log.Printf("Initialized Genkit MCP manager for environment %d", s.environmentID)
	return nil
}

// ExecuteAgent executes an agent using Genkit AI generation
func (s *GenkitService) ExecuteAgent(ctx context.Context, agentID, userID int64, task string) (*models.AgentRun, error) {
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
	log.Printf("Closing Genkit service for environment %d", s.environmentID)
	return nil
}