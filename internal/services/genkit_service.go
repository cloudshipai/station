package services

import (
	"context"
	"fmt"
	"log"
	"strings"
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
	environmentRepo      *repositories.EnvironmentRepo
	mcpConfigService     *MCPConfigService
	webhookService       *WebhookService
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
	environmentRepo *repositories.EnvironmentRepo,
	mcpConfigService *MCPConfigService,
	webhookService *WebhookService,
) *GenkitService {
	return &GenkitService{
		genkitApp:            genkitApp,
		openaiPlugin:         openaiPlugin,
		agentRepo:            agentRepo,
		agentRunRepo:         agentRunRepo,
		mcpConfigRepo:        mcpConfigRepo,
		agentToolRepo:        agentToolRepo,
		agentEnvironmentRepo: agentEnvironmentRepo,
		environmentRepo:      environmentRepo,
		mcpConfigService:     mcpConfigService,
		webhookService:       webhookService,
	}
}

// GetGenkitApp returns the underlying Genkit application instance
func (s *GenkitService) GetGenkitApp() *genkit.Genkit {
	return s.genkitApp
}

// InitializeMCP sets up MCP manager with configured servers for ALL environments
// This enables cross-environment agent tool access with environment namespacing
func (s *GenkitService) InitializeMCP(ctx context.Context) error {
	// Get all environments that have MCP configs
	allConfigs, err := s.mcpConfigRepo.GetAllLatestConfigs()
	if err != nil {
		return fmt.Errorf("failed to get all MCP configs: %w", err)
	}

	// Convert Station MCP configs to Genkit MCP server configs
	var mcpServers []mcp.MCPServerConfig
	for _, config := range allConfigs {
		// Decrypt the config to get server details
		decryptedData, err := s.mcpConfigService.DecryptConfigWithKeyID(config.ConfigJSON, config.EncryptionKeyID)
		if err != nil {
			log.Printf("Failed to decrypt MCP config %d: %v", config.ID, err)
			continue
		}
		
		// Get environment name for prefixing
		env, err := s.environmentRepo.GetByID(config.EnvironmentID)
		if err != nil {
			log.Printf("Failed to get environment %d for config %d: %v", config.EnvironmentID, config.ID, err)
			continue
		}
		
		// Convert each server in the config
		for serverName, serverConfig := range decryptedData.Servers {
			// Convert environment map to slice format
			var envVars []string
			for key, value := range serverConfig.Env {
				envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
			}
			
			mcpServers = append(mcpServers, mcp.MCPServerConfig{
				Name: fmt.Sprintf("%d_%s", env.ID, serverName), // Environment ID-prefixed server name
				Config: mcp.MCPClientOptions{
					Name:    fmt.Sprintf("%d_%s", env.ID, serverName),
					Version: "1.0.0",
					Stdio: &mcp.StdioConfig{
						Command: serverConfig.Command,
						Args:    serverConfig.Args,
						Env:     envVars,
					},
				},
			})
		}
	}
	
	manager, err := mcp.NewMCPManager(mcp.MCPManagerOptions{
		Name:       "station-cross-environment",
		Version:    "1.0.0",
		MCPServers: mcpServers,
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

	// Get tools assigned to this agent across all environments it has access to
	var tools []ai.Tool
	if s.mcpManager != nil {
		// Get all available MCP tools
		allMcpTools, err := s.mcpManager.GetActiveTools(ctx, s.genkitApp)
		if err != nil {
			log.Printf("Failed to get MCP tools: %v", err)
		} else {
			// Filter tools based on agent's cross-environment tool assignments
			assignedTools, err := s.agentToolRepo.List(agentID)
			if err != nil {
				log.Printf("Failed to get agent tool assignments: %v", err)
			} else {
				// Create a map of assigned tool names for quick lookup
				assignedToolNames := make(map[string]bool)
				log.Printf("DEBUG: Agent %d assigned tools:", agentID)
				for _, assignedTool := range assignedTools {
					assignedToolNames[assignedTool.ToolName] = true
					log.Printf("  - '%s' (env: %d)", assignedTool.ToolName, assignedTool.EnvironmentID)
				}
				
				// Log available MCP tools for comparison
				log.Printf("DEBUG: Available MCP tools from manager:")
				for _, mcpTool := range allMcpTools {
					log.Printf("  - '%s'", mcpTool.Name())
				}
				
				// Filter MCP tools to only include those assigned to the agent
				for _, mcpTool := range allMcpTools {
					toolName := mcpTool.Name()
					
					// Check if this MCP tool matches any assigned tool
					// MCP tools are prefixed with environment_server_, but database stores bare names
					matched := false
					var matchedAssignedTool string
					
					for assignedTool := range assignedToolNames {
						// Check if the MCP tool name ends with the assigned tool name
						// The assigned tool name already includes server prefixes (e.g., "aws___read_documentation")
						// So we need to match either:
						// 1. Exact match: toolName == assignedTool  
						// 2. Suffix match: toolName ends with "_" + assignedTool (for environment-prefixed tools)
						if toolName == assignedTool || strings.HasSuffix(toolName, "_"+assignedTool) {
							matched = true
							matchedAssignedTool = assignedTool
							break
						}
					}
					
					if matched {
						tools = append(tools, mcpTool)
						log.Printf("DEBUG: Matched MCP tool '%s' to assigned tool '%s' for agent %d", toolName, matchedAssignedTool, agentID)
					}
				}
				
				log.Printf("Agent %d has %d assigned tools, filtered to %d available MCP tools", agentID, len(assignedTools), len(tools))
			}
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
	
	// Send webhook notifications for successful completions if webhook service is available
	if s.webhookService != nil && status == "completed" {
		go s.sendWebhookNotification(agentID, run)
	}
	
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

// ReinitializeMCP reinitializes the MCP manager with current configs from database
// This should be called after MCP config changes (add/delete/update)
func (s *GenkitService) ReinitializeMCP(ctx context.Context) error {
	log.Printf("Reinitializing MCP manager with current configs from database")
	
	// Close existing MCP manager if it exists
	// Note: The Genkit MCP manager doesn't expose a direct Close method
	// so we'll replace it with a new instance
	s.mcpManager = nil
	
	// Reinitialize with current configs
	return s.InitializeMCP(ctx)
}

// Close cleans up the Genkit service with timeout
func (s *GenkitService) Close(ctx context.Context) error {
	log.Printf("Closing cross-environment Genkit service")
	
	// Create timeout context if none provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
	}
	
	// Force cleanup of MCP manager
	if s.mcpManager != nil {
		log.Printf("Cleaning up MCP manager connections")
		s.mcpManager = nil
	}
	
	// Cleanup any pending operations with timeout
	done := make(chan struct{})
	go func() {
		// Perform any cleanup operations here
		time.Sleep(100 * time.Millisecond) // Small delay for cleanup
		close(done)
	}()
	
	select {
	case <-done:
		log.Printf("Genkit service cleanup completed")
	case <-ctx.Done():
		log.Printf("Genkit service cleanup timeout - forcing close")
	}
	
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

// sendWebhookNotification sends webhook notifications for completed agent runs
func (s *GenkitService) sendWebhookNotification(agentID int64, agentRun *models.AgentRun) {
	// Get the agent details
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		log.Printf("Failed to get agent %d for webhook notification: %v", agentID, err)
		return
	}
	
	// Send webhook notification
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := s.webhookService.NotifyAgentRunCompleted(ctx, agentRun, agent); err != nil {
		log.Printf("Failed to send webhook notification for agent run %d: %v", agentRun.ID, err)
	} else {
		log.Printf("Webhook notifications sent for agent run %d", agentRun.ID)
	}
}