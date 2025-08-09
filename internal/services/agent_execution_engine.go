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

	// Filter to only include tools assigned to this agent and ensure clean tool names
	var tools []ai.ToolRef
	for _, assignedTool := range assignedTools {
		for _, mcpTool := range allTools {
			// Match by tool name - try multiple methods to get tool name  
			var toolName string
			if named, ok := mcpTool.(interface{ Name() string }); ok {
				toolName = named.Name()
			} else if stringer, ok := mcpTool.(interface{ String() string }); ok {
				toolName = stringer.String()
			} else {
				// Fallback: use the type name
				toolName = fmt.Sprintf("%T", mcpTool)
				logging.Debug("Tool has no Name() method, using type name: %s", toolName)
			}
			
			if toolName == assignedTool.ToolName {
				logging.Debug("Including assigned tool: %s", toolName)
				tools = append(tools, mcpTool) // Tool implements ToolRef interface
				break
			}
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
	
	// Debug: Log tool names and types
	for i, tool := range tools {
		if named, ok := tool.(interface{ Name() string }); ok {
			logging.Info("DEBUG: Tool %d: %s", i+1, named.Name())
		}
	}
	
	prompt := fmt.Sprintf("%s\n\nUser Task: %s", systemPrompt, task)
	logging.Info("DEBUG: About to call genkit.Generate with %d tools", len(tools))
	logging.Info("DEBUG: Prompt length: %d characters", len(prompt))
	
	// Create properly formatted model name with provider prefix
	var modelName string
	switch strings.ToLower(cfg.AIProvider) {
	case "gemini", "googlegenai":
		modelName = fmt.Sprintf("googleai/%s", cfg.AIModel)
	case "openai":
		modelName = fmt.Sprintf("station-openai/%s", cfg.AIModel)
	default:
		modelName = fmt.Sprintf("%s/%s", cfg.AIProvider, cfg.AIModel)
	}
	logging.Info("DEBUG: Model name: %s", modelName)
	
	// Station's custom OpenAI plugin handles tool calling properly, no middleware needed
	logging.Info("DEBUG: Using Station's custom plugin architecture - no middleware required")
	
	var generateOptions []ai.GenerateOption
	generateOptions = append(generateOptions, ai.WithModelName(modelName))
	generateOptions = append(generateOptions, ai.WithPrompt(prompt))
	
	generateOptions = append(generateOptions, ai.WithTools(tools...))
	
	
	response, err := genkit.Generate(ctx, genkitApp, generateOptions...)
	
	// Debug: Log the response details regardless of error
	logging.Info("DEBUG: GenKit Generate returned, error: %v", err)
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
	
	// Debug: Detailed GenKit Response Object Analysis
	logging.Info("DEBUG: === GenKit Response Object Details ===")
	logging.Info("DEBUG: Response Type: %T", response)
	logging.Info("DEBUG: Response Text Length: %d characters", len(responseText))
	
	// Examine response request details
	if response.Request != nil {
		logging.Info("DEBUG: Request Messages Count: %d", len(response.Request.Messages))
		for i, msg := range response.Request.Messages {
			logging.Info("DEBUG: Message %d - Role: %s, Content Parts: %d", i, msg.Role, len(msg.Content))
		}
	}
	
	// Examine response message details
	if response.Message != nil {
		logging.Info("DEBUG: Response Message Role: %s", response.Message.Role)
		logging.Info("DEBUG: Response Message Content Parts: %d", len(response.Message.Content))
		for i, part := range response.Message.Content {
			if part.IsText() {
				logging.Info("DEBUG: Content Part %d (Text): %.200s...", i, part.Text)
			} else if part.IsToolRequest() {
				logging.Info("DEBUG: Content Part %d (Tool Request): Name=%s, Ref=%s", 
					i, part.ToolRequest.Name, part.ToolRequest.Ref)
			} else if part.IsToolResponse() {
				logging.Info("DEBUG: Content Part %d (Tool Response): Name=%s, Ref=%s", 
					i, part.ToolResponse.Name, part.ToolResponse.Ref)
			}
		}
	}
	
	// Usage information
	if response.Usage != nil {
		logging.Info("DEBUG: Token Usage - Input: %d, Output: %d, Total: %d", 
			response.Usage.InputTokens, response.Usage.OutputTokens, response.Usage.TotalTokens)
	}
	
	// Tool request analysis
	toolRequests := response.ToolRequests()
	logging.Info("DEBUG: Tool Requests in Response: %d", len(toolRequests))
	for i, req := range toolRequests {
		logging.Info("DEBUG: Tool Request %d: Name=%s, Ref=%s, Input=%v", i+1, req.Name, req.Ref, req.Input)
	}
	
	// Debug: Check if response has any tool-related content
	logging.Info("DEBUG: Response length: %d characters", len(responseText))

	// Extract tool calls directly from GenKit response object (no middleware)
	logging.Info("DEBUG: Extracting tool calls directly from GenKit response object")
	var toolCalls []interface{}
	var steps []interface{}
	
	// Direct extraction from response.ToolRequests() (reuse existing variable)
	// toolRequests already declared above, so reuse it
	for i, toolReq := range toolRequests {
		toolCall := map[string]interface{}{
			"step":           i + 1,
			"type":           "tool_call",
			"tool_name":      toolReq.Name,
			"tool_input":     toolReq.Input,
			"model_name":     cfg.AIModel,
			"tool_call_id":   toolReq.Ref, // Station's fixed plugin preserves proper IDs
		}
		toolCalls = append(toolCalls, toolCall)
	}
	
	// Build simple execution steps from response data
	if len(toolCalls) > 0 {
		step := map[string]interface{}{
			"step":              1,
			"type":              "tool_execution",
			"agent_id":          agent.ID,
			"agent_name":        agent.Name,
			"model_name":        cfg.AIModel,
			"tool_calls_count":  len(toolCalls),
			"tools_used":        len(toolCalls),
		}
		steps = append(steps, step)
	}
	
	// Add final response step
	if responseText != "" {
		finalStep := map[string]interface{}{
			"step":              len(steps) + 1,
			"type":              "final_response",
			"agent_id":          agent.ID,
			"agent_name":        agent.Name,
			"model_name":        cfg.AIModel,
			"content_length":    len(responseText),
		}
		steps = append(steps, finalStep)
	}

	// Convert to database-compatible types
	toolCallsJSON := &models.JSONArray{}
	if len(toolCalls) > 0 {
		*toolCallsJSON = models.JSONArray(toolCalls)
	}
	
	executionStepsJSON := &models.JSONArray{}
	if len(steps) > 0 {
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

	// DEBUG: Log complete AgentExecutionResult data  
	logging.Info("DEBUG: === AgentExecutionResult Complete Data ===")
	logging.Info("DEBUG: Success: %v", result.Success)
	logging.Info("DEBUG: Response length: %d chars", len(result.Response))
	logging.Info("DEBUG: Duration: %v", result.Duration)
	logging.Info("DEBUG: ModelName: %s", result.ModelName)
	logging.Info("DEBUG: StepsUsed: %d", result.StepsUsed)
	logging.Info("DEBUG: ToolsUsed: %d", result.ToolsUsed)
	if result.TokenUsage != nil {
		logging.Info("DEBUG: TokenUsage: %+v", result.TokenUsage)
	} else {
		logging.Info("DEBUG: TokenUsage: nil")
	}
	if result.ToolCalls != nil {
		logging.Info("DEBUG: ToolCalls: %d items", len(*result.ToolCalls))
	} else {
		logging.Info("DEBUG: ToolCalls: nil")
	}
	if result.ExecutionSteps != nil {
		logging.Info("DEBUG: ExecutionSteps: %d items", len(*result.ExecutionSteps))
	} else {
		logging.Info("DEBUG: ExecutionSteps: nil")
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