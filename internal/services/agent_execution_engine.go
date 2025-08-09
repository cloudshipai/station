package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
)

// AgentExecutionResult contains the result of an agent execution
type AgentExecutionResult struct {
	Success        bool                     `json:"success"`
	Response       string                   `json:"response"`
	ToolCalls      *models.JSONArray        `json:"tool_calls"`
	Steps          []interface{}            `json:"steps"`
	ExecutionSteps *models.JSONArray        `json:"execution_steps"` // For database storage
	Duration       time.Duration            `json:"duration"`
	TokenUsage     map[string]interface{}   `json:"token_usage,omitempty"`
	ModelName      string                   `json:"model_name"`
	StepsUsed      int                      `json:"steps_used"`
	StepsTaken     int64                    `json:"steps_taken"` // For database storage
	ToolsUsed      int                      `json:"tools_used"`
	Error          string                   `json:"error,omitempty"`
}

// AgentExecutionEngine handles the execution of agents using GenKit and MCP
type AgentExecutionEngine struct {
	repos              *repositories.Repositories
	agentService       AgentServiceInterface
	genkitProvider     *GenKitProvider
	mcpConnManager     *MCPConnectionManager
	responseProcessor  *ResponseProcessor
	telemetryManager   *TelemetryManager
	activeMCPClients   []*mcp.GenkitMCPClient // Store active connections for cleanup after execution
}

// NewAgentExecutionEngine creates a new agent execution engine
func NewAgentExecutionEngine(repos *repositories.Repositories, agentService AgentServiceInterface) *AgentExecutionEngine {
	return &AgentExecutionEngine{
		repos:             repos,
		agentService:      agentService,
		genkitProvider:    NewGenKitProvider(),
		mcpConnManager:    NewMCPConnectionManager(repos, nil),
		responseProcessor: NewResponseProcessor(),
		telemetryManager:  NewTelemetryManager(),
	}
}

// ExecuteAgentViaStdioMCP executes an agent using self-bootstrapping stdio MCP architecture
func (aee *AgentExecutionEngine) ExecuteAgentViaStdioMCP(ctx context.Context, agent *models.Agent, task string, runID int64) (*AgentExecutionResult, error) {
	startTime := time.Now()
	logging.Info("Starting stdio MCP agent execution for agent '%s'", agent.Name)

	// Setup cleanup of MCP connections when execution completes
	defer func() {
		aee.mcpConnManager.CleanupConnections(aee.activeMCPClients)
		// Clear the slice for next execution
		aee.activeMCPClients = nil
	}()

	// Initialize Genkit + MCP if not already done
	genkitApp, err := aee.genkitProvider.GetApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Genkit for agent execution: %w", err)
	}
	
	// Update MCP connection manager with GenKit app
	aee.mcpConnManager.genkitApp = genkitApp

	// Let Genkit handle tracing automatically - no need for custom span wrapping
	// Get tools assigned to this specific agent
	assignedTools, err := aee.repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assigned tools for agent %d: %w", agent.ID, err)
	}

	logging.Debug("Agent has %d assigned tools for execution", len(assignedTools))

	// Get MCP tools from agent's environment using connection manager
	allTools, clients, err := aee.mcpConnManager.GetEnvironmentMCPTools(ctx, agent.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment MCP tools for agent %d: %w", agent.ID, err)
	}
	// Store clients for cleanup
	aee.activeMCPClients = clients

	// TESTING: Use all available tools instead of filtering by assigned tools
	// This allows agents to access all environment tools for GitHub MCP functionality
	// Deduplicate tools to prevent GenKit errors
	var tools []ai.ToolRef
	toolNames := make(map[string]bool)
	for _, mcpTool := range allTools {
		var toolName string
		if named, ok := mcpTool.(interface{ Name() string }); ok {
			toolName = named.Name()
		} else if stringer, ok := mcpTool.(interface{ String() string }); ok {
			toolName = stringer.String()
		} else {
			toolName = fmt.Sprintf("%T", mcpTool)
		}
		
		// Only add if not already seen
		if !toolNames[toolName] {
			toolNames[toolName] = true
			logging.Debug("Including environment tool: %s", toolName)
			tools = append(tools, mcpTool)
		} else {
			logging.Debug("Skipping duplicate tool: %s", toolName)
		}
	}

	logging.Info("Agent execution using %d tools (filtered from %d available in environment)", len(tools), len(allTools))
	if len(tools) == 0 {
		logging.Debug("WARNING: No tools available for agent execution - this may indicate a configuration issue")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Build the system prompt with agent-specific instructions
	systemPrompt := fmt.Sprintf(`%s

You are executing on behalf of agent '%s' (ID: %d) in environment ID: %d.
Available tools: %d
Max steps allowed: %d

IMPORTANT: You MUST use the available tools to complete tasks. Always start by using tools to gather information or perform actions.

Available tools for this task:
%s

Guidelines:
- ALWAYS use available tools when they can help complete the task
- Be methodical and thorough in your approach
- Use tools to gather information and perform actions
- Provide clear explanations of what you're doing and why
- If you encounter errors, try alternative approaches
- Summarize your findings and actions at the end`,
		agent.Prompt,
		agent.Name,
		agent.ID,
		agent.EnvironmentID,
		len(tools),
		agent.MaxSteps,
		func() string {
			var toolNames []string
			for _, tool := range tools {
				toolNames = append(toolNames, "- "+tool.Name())
			}
			if len(toolNames) == 0 {
				return "- No tools available"
			}
			return strings.Join(toolNames, "\n")
		}())

	// Execute with GenKit
	logging.Info("Executing agent with GenKit...")
	logging.Info("System prompt: %s", systemPrompt)
	logging.Info("User task: %s", task)
	logging.Info("Model: %s", cfg.AIModel)
	logging.Info("Number of tools: %d", len(tools))
	
	prompt := fmt.Sprintf("%s\n\nUser Task: %s", systemPrompt, task)
	// Create properly formatted model name with provider prefix
	modelName := fmt.Sprintf("%s/%s", cfg.AIProvider, cfg.AIModel)
	response, err := genkit.Generate(ctx, genkitApp,
		ai.WithModelName(modelName),
		ai.WithPrompt(prompt),
		ai.WithTools(tools...), // Re-enable tools
	)
	if err != nil {
		logging.Info("GenKit Generate error: %v", err)
		return &AgentExecutionResult{
			Success:   false,
			Error:     err.Error(),
			Duration:  time.Since(startTime),
			ModelName: cfg.AIModel,
		}, nil
	}
	
	logging.Info("GenKit Generate completed successfully")

	// Process the response
	responseText := response.Text()
	logging.Info("AI Response Text: %s", responseText)
	logging.Info("Tool Requests in Response: %d", len(response.ToolRequests()))

	// Extract tool calls and execution steps for detailed logging
	toolCalls := aee.responseProcessor.ExtractToolCallsFromResponse(response, cfg.AIModel)
	steps := aee.responseProcessor.BuildExecutionStepsFromResponse(response, agent, cfg.AIModel, len(tools))
	
	// Add tool outputs to captured calls for complete audit trail
	if len(toolCalls) > 0 {
		if capturedCalls, ok := toolCalls[0].([]map[string]interface{}); ok {
			aee.responseProcessor.AddToolOutputsToCapturedCalls(capturedCalls, response)
		}
	}

	// Convert to database-compatible types
	toolCallsJSON := &models.JSONArray{}
	if toolCalls != nil {
		*toolCallsJSON = models.JSONArray(toolCalls)
	}
	
	executionStepsJSON := &models.JSONArray{}
	if steps != nil {
		*executionStepsJSON = models.JSONArray(steps)
	}
	
	result := &AgentExecutionResult{
		Success:        true,
		Response:       responseText,
		ToolCalls:      toolCallsJSON,
		Steps:          steps,
		ExecutionSteps: executionStepsJSON,
		Duration:       time.Since(startTime),
		ModelName:      cfg.AIModel,
		StepsUsed:      len(steps),
		StepsTaken:     int64(len(steps)),
		ToolsUsed:      len(toolCalls),
	}

	// Extract token usage if available
	if response.Usage != nil {
		result.TokenUsage = map[string]interface{}{
			"input_tokens":  response.Usage.InputTokens,
			"output_tokens": response.Usage.OutputTokens,
			"total_tokens":  response.Usage.TotalTokens,
		}
	}

	logging.Info("Agent execution completed successfully in %v (steps: %d, tools: %d)", 
		result.Duration, result.StepsUsed, result.ToolsUsed)

	return result, nil
}

// TestStdioMCPConnection tests the MCP connection for debugging
func (aee *AgentExecutionEngine) TestStdioMCPConnection(ctx context.Context) error {
	logging.Info("Testing stdio MCP connection...")

	genkitApp, err := aee.genkitProvider.GetApp(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize Genkit for MCP test: %w", err)
	}
	
	// Update MCP connection manager
	aee.mcpConnManager.genkitApp = genkitApp

	// Test getting tools from default environment (ID: 1)
	tools, clients, err := aee.mcpConnManager.GetEnvironmentMCPTools(ctx, 1)
	if err != nil {
		return fmt.Errorf("failed to get MCP tools: %w", err)
	}
	
	// Cleanup connections
	defer aee.mcpConnManager.CleanupConnections(clients)

	logging.Info("✅ MCP connection test successful - discovered %d tools", len(tools))
	
	for i, tool := range tools {
		if named, ok := tool.(interface{ Name() string }); ok {
			logging.Info("  Tool %d: %s", i+1, named.Name())
		} else {
			logging.Info("  Tool %d: %T (no Name method)", i+1, tool)
		}
	}

	return nil
}