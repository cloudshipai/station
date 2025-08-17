package services

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/models"
	dotprompt "station/pkg/dotprompt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
	googledotprompt "github.com/google/dotprompt/go/dotprompt"
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
	mcpConnManager := NewMCPConnectionManager(repos, nil)
	
	// Check environment variable for connection pooling
	if os.Getenv("STATION_MCP_POOLING") == "true" {
		mcpConnManager.EnableConnectionPooling()
		logging.Info("🏊 MCP connection pooling enabled via STATION_MCP_POOLING environment variable")
	}
	
	return &AgentExecutionEngine{
		repos:             repos,
		agentService:      agentService,
		genkitProvider:    NewGenKitProvider(),
		mcpConnManager:    mcpConnManager,
		telemetryManager:  NewTelemetryManager(),
	}
}

// ExecuteAgentViaStdioMCP executes an agent using self-bootstrapping stdio MCP architecture
func (aee *AgentExecutionEngine) ExecuteAgentViaStdioMCP(ctx context.Context, agent *models.Agent, task string, runID int64) (*AgentExecutionResult, error) {
	// Default to empty user variables for backward compatibility
	return aee.ExecuteAgentViaStdioMCPWithVariables(ctx, agent, task, runID, map[string]interface{}{})
}

// ExecuteAgentViaStdioMCPWithVariables executes an agent with user-defined variables for dotprompt rendering
func (aee *AgentExecutionEngine) ExecuteAgentViaStdioMCPWithVariables(ctx context.Context, agent *models.Agent, task string, runID int64, userVariables map[string]interface{}) (*AgentExecutionResult, error) {
	startTime := time.Now()
	logging.Info("Starting unified dotprompt execution for agent '%s'", agent.Name)

	// All agents now use unified dotprompt execution system
		
		// Setup cleanup of MCP connections when dotprompt execution completes
		defer func() {
			aee.mcpConnManager.CleanupConnections(aee.activeMCPClients)
			// Clear the slice for next execution
			aee.activeMCPClients = nil
		}()
		
		// Get agent tools for the new dotprompt system
		agentTools, err := aee.repos.AgentTools.ListAgentTools(agent.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get agent tools for dotprompt execution: %w", err)
		}
		
		// Get GenKit app for dotprompt execution
		genkitApp, err := aee.genkitProvider.GetApp(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get genkit app for dotprompt execution: %w", err)
		}
		
		// Update MCP connection manager with GenKit app (same as traditional)
		aee.mcpConnManager.genkitApp = genkitApp
		
		// Initialize server pool if pooling is enabled (same as traditional)
		if err := aee.mcpConnManager.InitializeServerPool(ctx); err != nil {
			logging.Info("Warning: Failed to initialize MCP server pool for dotprompt: %v", err)
		}
		
		// Load MCP tools for dotprompt execution (reuse the same logic as traditional execution)
		allMCPTools, mcpClients, err := aee.mcpConnManager.GetEnvironmentMCPTools(ctx, agent.EnvironmentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get environment MCP tools for dotprompt execution: %w", err)
		}
		
		// Store clients for cleanup after execution
		aee.activeMCPClients = mcpClients
		
		// Filter to only include tools assigned to this agent (same filtering logic as traditional)
		logging.Debug("Filtering %d assigned tools from %d available MCP tools", len(agentTools), len(allMCPTools))
		var mcpTools []ai.ToolRef
		for _, assignedTool := range agentTools {
			for _, mcpTool := range allMCPTools {
				// Match by tool name - same method as traditional execution
				var toolName string
				if named, ok := mcpTool.(interface{ Name() string }); ok {
					toolName = named.Name()
				} else if stringer, ok := mcpTool.(interface{ String() string }); ok {
					toolName = stringer.String()
				} else {
					// Fallback: use the type name
					toolName = fmt.Sprintf("%T", mcpTool)
				}
				
				if toolName == assignedTool.ToolName {
					mcpTools = append(mcpTools, mcpTool)
					break
				}
			}
		}
		
		logging.Debug("Dotprompt execution using %d tools (filtered from %d available)", len(mcpTools), len(allMCPTools))
		
		// Use our new dotprompt + genkit execution system
		executor := dotprompt.NewGenKitExecutor()
		response, err := executor.ExecuteAgentWithDatabaseConfig(*agent, agentTools, genkitApp, mcpTools, task)
		if err != nil {
			return nil, fmt.Errorf("dotprompt execution failed: %w", err)
		}
		
		// Convert ExecutionResponse to AgentExecutionResult  
		return &AgentExecutionResult{
			Success:        response.Success,
			Response:       response.Response,
			Duration:       time.Since(startTime), // Use our own timing
			ModelName:      response.ModelName,
			StepsUsed:      response.StepsUsed,
			ToolsUsed:      response.ToolsUsed,
			Error:          response.Error,
			TokenUsage:     make(map[string]interface{}), // TODO: Add token usage from dotprompt system
			ToolCalls:      response.ToolCalls,           // ✅ Pass through tool calls
			ExecutionSteps: response.ExecutionSteps,     // ✅ Pass through execution steps
		}, nil
}

// ExecuteAgentWithMessages executes an agent using pre-rendered ai.Message objects
// This allows dotprompt multi-role templates while reusing all tool loading and config logic
func (aee *AgentExecutionEngine) ExecuteAgentWithMessages(ctx context.Context, agent *models.Agent, messages []*ai.Message, userID int64) (*AgentExecutionResult, error) {
	// Reuse all the existing setup logic but replace the prompt/system message handling
	logging.Info("=== Executing Agent with Messages (Dotprompt Multi-Role) ===")
	logging.Info("Agent: %s (ID: %d, Environment: %d)", agent.Name, agent.ID, agent.EnvironmentID)
	logging.Info("Messages: %d", len(messages))
	
	// Get tools assigned to this specific agent (same as existing method)
	assignedTools, err := aee.repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assigned tools for agent %d: %w", agent.ID, err)
	}

	logging.Debug("Agent has %d assigned tools for execution", len(assignedTools))

	// Get available tools and reuse clients for execution (same as existing)
	allTools, executionClients, err := aee.mcpConnManager.GetEnvironmentMCPTools(ctx, agent.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment tools for environment %d: %w", agent.EnvironmentID, err)
	}

	// Store active clients for reuse (same as existing)
	aee.activeMCPClients = executionClients

	// Filter tools to only include those assigned to the agent (same as existing)
	var tools []ai.ToolRef
	for _, assignedTool := range assignedTools {
		for _, mcpTool := range allTools {
			var toolName string
			if named, ok := mcpTool.(interface{ Name() string }); ok {
				toolName = named.Name()
			} else if stringer, ok := mcpTool.(interface{ String() string }); ok {
				toolName = stringer.String()
			} else {
				toolName = fmt.Sprintf("%T", mcpTool)
				logging.Debug("Tool has no Name() method, using type name: %s", toolName)
			}
			
			if toolName == assignedTool.ToolName {
				logging.Debug("Including assigned tool: %s", toolName)
				tools = append(tools, mcpTool)
				break
			}
		}
	}

	logging.Info("Agent execution using %d tools (filtered from %d available)", len(tools), len(allTools))

	// Load configuration (same as existing)
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Get GenKit app (same as existing)
	genkitApp, err := aee.genkitProvider.GetApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get GenKit app: %w", err)
	}

	// Create model name (same as existing)
	var modelName string
	switch strings.ToLower(cfg.AIProvider) {
	case "gemini", "googlegenai":
		modelName = fmt.Sprintf("googleai/%s", cfg.AIModel)
	case "openai":
		modelName = fmt.Sprintf("openai/%s", cfg.AIModel)
	default:
		modelName = fmt.Sprintf("%s/%s", cfg.AIProvider, cfg.AIModel)
	}
	logging.Debug("Using model: %s", modelName)

	// Build generate options with messages instead of system+prompt
	var generateOptions []ai.GenerateOption
	generateOptions = append(generateOptions, ai.WithModelName(modelName))
	generateOptions = append(generateOptions, ai.WithMessages(messages...)) // KEY DIFFERENCE
	generateOptions = append(generateOptions, ai.WithTools(tools...))
	generateOptions = append(generateOptions, ai.WithMaxTurns(25))

	logging.Debug("Calling genkit.Generate with %d messages and %d tools", len(messages), len(tools))

	// Execute with GenKit (same as existing)
	genCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	response, err := genkit.Generate(genCtx, genkitApp, generateOptions...)
	if err != nil {
		return nil, fmt.Errorf("GenKit Generate failed: %w", err)
	}

	logging.Debug("GenKit Generate completed successfully")

	// Return result (same format as existing)
	result := &AgentExecutionResult{
		Success:   true,
		Response:  response.Text(),
		Duration:  time.Since(time.Now()), // Will be overridden by caller
		ModelName: modelName,
		StepsUsed: 1, // TODO: Extract from response if available
		ToolsUsed: len(tools),
	}

	return result, nil
}

// GetGenkitProvider returns the genkit provider for external access
func (aee *AgentExecutionEngine) GetGenkitProvider() *GenKitProvider {
	return aee.genkitProvider
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

// RenderAgentPromptWithDotprompt renders agent prompt with dotprompt if it contains frontmatter
func (aee *AgentExecutionEngine) RenderAgentPromptWithDotprompt(agentPrompt string, userVariables map[string]interface{}) (string, error) {
	// Check if this is a dotprompt with YAML frontmatter
	if !aee.isDotpromptContent(agentPrompt) {
		// Not a dotprompt, return as-is
		return agentPrompt, nil
	}

	logging.Debug("Agent prompt is dotprompt format, rendering with %d variables", len(userVariables))
	
	// Do inline dotprompt rendering to avoid import cycle
	renderedPrompt, err := aee.renderDotpromptInline(agentPrompt, userVariables)
	if err != nil {
		return "", fmt.Errorf("failed to render dotprompt: %w", err)
	}

	logging.Debug("Dotprompt rendering successful, result length: %d characters", len(renderedPrompt))
	return renderedPrompt, nil
}

// isDotpromptContent checks if the prompt contains dotprompt frontmatter or multi-role syntax
func (aee *AgentExecutionEngine) isDotpromptContent(prompt string) bool {
	trimmed := strings.TrimSpace(prompt)
	
	// Check for YAML frontmatter markers
	hasFrontmatter := strings.HasPrefix(trimmed, "---") && 
		   strings.Contains(prompt, "\n---\n")
		   
	// Check for multi-role dotprompt syntax
	hasMultiRole := strings.Contains(prompt, "{{role \"") || strings.Contains(prompt, "{{role '")
	
	return hasFrontmatter || hasMultiRole
}

// renderDotpromptInline renders dotprompt content inline to avoid import cycles
func (aee *AgentExecutionEngine) renderDotpromptInline(dotpromptContent string, userVariables map[string]interface{}) (string, error) {
	// 1. Create dotprompt instance
	dp := googledotprompt.NewDotprompt(nil) // Use default options
	
	// 2. Prepare data for rendering with user-defined variables only
	data := &googledotprompt.DataArgument{
		Input:   userVariables, // User-defined variables like {{my_folder}}, {{my_var}}
		Context: map[string]any{}, // Keep context empty unless needed
	}
	
	// 3. Render the template  
	rendered, err := dp.Render(dotpromptContent, data, nil)
	if err != nil {
		return "", fmt.Errorf("failed to render dotprompt: %w", err)
	}
	
	// 4. Convert messages to text (extract just the content, no role prefixes)
	var renderedText strings.Builder
	for i, msg := range rendered.Messages {
		if i > 0 {
			renderedText.WriteString("\n\n")
		}
		// Don't include role prefix - just the content
		for _, part := range msg.Content {
			if textPart, ok := part.(*googledotprompt.TextPart); ok {
				renderedText.WriteString(textPart.Text)
			}
		}
	}
	
	return renderedText.String(), nil
}

// AgentSchema represents the input/output schema for an agent
type AgentSchema struct {
	AgentID      int64                  `json:"agent_id"`
	AgentName    string                 `json:"agent_name"`
	HasSchema    bool                   `json:"has_schema"`
	InputSchema  map[string]interface{} `json:"input_schema,omitempty"`
	OutputSchema map[string]interface{} `json:"output_schema,omitempty"`
	Variables    []string               `json:"variables,omitempty"` // Available template variables
}

// GetAgentSchema extracts schema information from agent's dotprompt content using GenKit's parser
func (aee *AgentExecutionEngine) GetAgentSchema(agent *models.Agent) (*AgentSchema, error) {
	schema := &AgentSchema{
		AgentID:   agent.ID,
		AgentName: agent.Name,
		HasSchema: false,
		Variables: []string{},
	}
	
	if !aee.isDotpromptContent(agent.Prompt) {
		// Simple text prompt - no schema
		return schema, nil
	}
	
	// Use GenKit's dotprompt parser to properly parse the template
	parsedPrompt, err := googledotprompt.ParseDocument(agent.Prompt)
	if err != nil {
		return schema, fmt.Errorf("failed to parse dotprompt document: %w", err)
	}
	
	schema.HasSchema = true
	
	// Extract input schema from parsed metadata
	if parsedPrompt.Input.Schema != nil {
		// Schema is of type 'any', so we need to properly handle it
		if schemaMap, ok := parsedPrompt.Input.Schema.(map[string]interface{}); ok {
			schema.InputSchema = schemaMap
			
			// Extract variable names from the input schema
			for varName := range schemaMap {
				schema.Variables = append(schema.Variables, varName)
			}
		} else {
			// Store the raw schema even if it's not a map
			if schemaAny, ok := parsedPrompt.Input.Schema.(interface{}); ok {
				// Try to convert to map[string]interface{} for JSON serialization
				schema.InputSchema = map[string]interface{}{"schema": schemaAny}
			}
		}
	}
	
	// Extract output schema from parsed metadata
	if parsedPrompt.Output.Schema != nil {
		if schemaMap, ok := parsedPrompt.Output.Schema.(map[string]interface{}); ok {
			schema.OutputSchema = schemaMap
		} else {
			// Store the raw schema even if it's not a map
			if schemaAny, ok := parsedPrompt.Output.Schema.(interface{}); ok {
				schema.OutputSchema = map[string]interface{}{"schema": schemaAny}
			}
		}
	}
	
	// Also extract variables from template content as fallback
	if len(schema.Variables) == 0 {
		variables := aee.extractTemplateVariables(agent.Prompt)
		schema.Variables = variables
	}
	
	return schema, nil
}

// extractTemplateVariables finds all {{variable}} patterns in the template content as fallback
func (aee *AgentExecutionEngine) extractTemplateVariables(dotpromptContent string) []string {
	// Extract template content (after frontmatter)
	parts := strings.SplitN(strings.TrimSpace(dotpromptContent), "\n---\n", 2)
	templateContent := parts[len(parts)-1] // Use last part (template content)
	
	// Find all {{variable}} patterns
	var variables []string
	variableMap := make(map[string]bool) // Use map to deduplicate
	
	// Simple regex to find {{variable}} patterns
	start := 0
	for {
		openIndex := strings.Index(templateContent[start:], "{{")
		if openIndex == -1 {
			break
		}
		openIndex += start
		
		closeIndex := strings.Index(templateContent[openIndex:], "}}")
		if closeIndex == -1 {
			break
		}
		closeIndex += openIndex
		
		// Extract variable name
		varContent := strings.TrimSpace(templateContent[openIndex+2 : closeIndex])
		
		// Handle simple variable names (no complex handlebars logic)
		if varContent != "" && !strings.Contains(varContent, " ") && !strings.Contains(varContent, "#") {
			variableMap[varContent] = true
		}
		
		start = closeIndex + 2
	}
	
	// Convert map to slice
	for variable := range variableMap {
		variables = append(variables, variable)
	}
	
	return variables
}
