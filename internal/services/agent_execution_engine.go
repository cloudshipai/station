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
	default:
		modelName = fmt.Sprintf("%s/%s", cfg.AIProvider, cfg.AIModel)
	}
	logging.Info("DEBUG: Model name: %s", modelName)
	
	// Provider-aware tool call tracking middleware  
	var capturedToolCalls []map[string]interface{}
	// Define providers that need middleware for tool call extraction
	// These providers use OpenAI-compatible format where GenKit's multi-turn orchestration hides tool calls
	openAICompatibleProviders := []string{
		"openai", "anthropic", "groq", "deepseek", "together",
		"fireworks", "perplexity", "mistral", "cohere",
		"huggingface", "replicate", "anyscale", "local",
	}
	
	// Check if current provider needs middleware
	// NOTE: Gemini and other native providers are excluded - they work beautifully with native GenKit extraction
	needsMiddleware := false
	for _, provider := range openAICompatibleProviders {
		if strings.HasPrefix(strings.ToLower(cfg.AIProvider), provider) {
			needsMiddleware = true
			break
		}
	}
	
	logging.Info("DEBUG: Provider needs middleware: %v", needsMiddleware)
	
	var generateOptions []ai.GenerateOption
	generateOptions = append(generateOptions, ai.WithModelName(modelName))
	generateOptions = append(generateOptions, ai.WithPrompt(prompt))
	
	generateOptions = append(generateOptions, ai.WithTools(tools...))
	
	// Add middleware for OpenAI-compatible providers to capture tool calls
	if needsMiddleware {
		toolTrackingMiddleware := func(next ai.ModelFunc) ai.ModelFunc {
			return func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
				logging.Info("DEBUG: Middleware intercepting model call")
				resp, err := next(ctx, req, cb)
				if err != nil {
					logging.Info("DEBUG: Middleware - model call failed: %v", err)
					return resp, err
				}
				
				// Capture tool calls from response
				if resp != nil && resp.Message != nil {
					logging.Info("DEBUG: Middleware checking response for tool calls")
					for _, part := range resp.Message.Content {
						if part.IsToolRequest() {
							toolReq := part.ToolRequest
							logging.Info("DEBUG: Middleware captured tool call: %s", toolReq.Name)
							capturedToolCalls = append(capturedToolCalls, map[string]interface{}{
								"tool_name": toolReq.Name,
								"input":     toolReq.Input,
								"step":      len(capturedToolCalls) + 1,
								"type":      "tool_call",
							})
						}
					}
					logging.Info("DEBUG: Middleware captured %d tool calls total", len(capturedToolCalls))
				}
				
				return resp, err
			}
		}
		generateOptions = append(generateOptions, ai.WithMiddleware(toolTrackingMiddleware))
		logging.Info("DEBUG: Added tool tracking middleware for provider: %s", cfg.AIProvider)
	}
	
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
	
	// Debug: Detailed tool request analysis
	toolRequests := response.ToolRequests()
	logging.Info("DEBUG: Tool Requests in Response: %d", len(toolRequests))
	for i, req := range toolRequests {
		logging.Info("DEBUG: Tool Request %d: Name=%s, Input=%v", i+1, req.Name, req.Input)
	}
	
	// Debug: Check if response has any tool-related content
	logging.Info("DEBUG: Response length: %d characters", len(responseText))

	// Extract tool calls using provider-aware approach
	var toolCalls interface{}
	var steps []interface{}
	
	if needsMiddleware && len(capturedToolCalls) > 0 {
		// Use middleware-captured tool calls for OpenAI-compatible providers
		logging.Info("DEBUG: Using middleware-captured tool calls (%d captured)", len(capturedToolCalls))
		var toolCallsInterface []interface{}
		for _, toolCall := range capturedToolCalls {
			toolCallsInterface = append(toolCallsInterface, toolCall)
		}
		toolCalls = toolCallsInterface
		
		// Build execution steps from captured tool calls
		steps = aee.responseProcessor.BuildExecutionStepsFromCapturedCalls(capturedToolCalls, response, agent, cfg.AIModel)
	} else {
		// Use native GenKit tool call extraction for Gemini and providers without middleware
		logging.Info("DEBUG: Using native GenKit tool call extraction")
		toolCalls = aee.responseProcessor.ExtractToolCallsFromResponse(response, cfg.AIModel)
		steps = aee.responseProcessor.BuildExecutionStepsFromResponse(response, agent, cfg.AIModel, len(tools))
	}

	// Convert to database-compatible types
	toolCallsJSON := &models.JSONArray{}
	if toolCalls != nil {
		if toolCallsSlice, ok := toolCalls.([]interface{}); ok {
			*toolCallsJSON = models.JSONArray(toolCallsSlice)
		}
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
		ToolsUsed:      func() int {
			if toolCallsSlice, ok := toolCalls.([]interface{}); ok {
				return len(toolCallsSlice)
			}
			return 0
		}(),
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