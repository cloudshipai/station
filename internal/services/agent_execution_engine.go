package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/models"
	dotprompt "station/pkg/dotprompt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/plugins/mcp"
	googledotprompt "github.com/google/dotprompt/go/dotprompt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	telemetryService   *TelemetryService // For creating spans
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

	// Create telemetry span if telemetry service is available
	var span trace.Span
	if aee.telemetryService != nil {
		ctx, span = aee.telemetryService.StartSpan(ctx, "agent_execution_engine.execute",
			trace.WithAttributes(
				attribute.String("agent.name", agent.Name),
				attribute.Int64("agent.id", agent.ID),
				attribute.Int64("run.id", runID),
				attribute.Int("user_variables.count", len(userVariables)),
			),
		)
		defer span.End()
	}

	// Log execution start
	err := aee.repos.AgentRuns.AppendDebugLog(ctx, runID, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     "info",
		"message":   fmt.Sprintf("Starting execution for agent '%s'", agent.Name),
		"details": map[string]interface{}{
			"agent_id": agent.ID,
			"task":     task,
		},
	})
	if err != nil {
		logging.Debug("Failed to log execution start: %v", err)
	}

	// All agents now use unified dotprompt execution system
		
		// Note: MCP cleanup will happen after dotprompt execution completes
		// Do NOT defer cleanup here as it would disconnect connections while LLM is still using tools
		
		// Get agent tools for the new dotprompt system
		agentTools, err := aee.repos.AgentTools.ListAgentTools(agent.ID)
		if err != nil {
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to get agent tools")
			}
			return nil, fmt.Errorf("failed to get agent tools for dotprompt execution: %w", err)
		}
		
		if span != nil {
			span.SetAttributes(attribute.Int("agent.tools_count", len(agentTools)))
		}
		
		// Get GenKit app for dotprompt execution
		genkitApp, err := aee.genkitProvider.GetApp(ctx)
		if err != nil {
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to get genkit app")
			}
			return nil, fmt.Errorf("failed to get genkit app for dotprompt execution: %w", err)
		}
		
		// Update MCP connection manager with GenKit app (same as traditional)
		aee.mcpConnManager.genkitApp = genkitApp
		
		// Initialize server pool if pooling is enabled (same as traditional)
		if err := aee.mcpConnManager.InitializeServerPool(ctx); err != nil {
			logging.Info("Warning: Failed to initialize MCP server pool for dotprompt: %v", err)
		}
		
		// Load MCP tools for dotprompt execution (reuse the same logic as traditional execution)
		var mcpLoadSpan trace.Span
		if span != nil {
			ctx, mcpLoadSpan = aee.telemetryService.StartSpan(ctx, "mcp.load_tools",
				trace.WithAttributes(
					attribute.Int64("environment.id", agent.EnvironmentID),
				),
			)
			defer mcpLoadSpan.End()
		}
		
		logging.Info("🔥 AGENT-ENGINE: About to call GetEnvironmentMCPTools for env %d", agent.EnvironmentID)
		allMCPTools, mcpClients, err := aee.mcpConnManager.GetEnvironmentMCPTools(ctx, agent.EnvironmentID)
		logging.Info("🔥 AGENT-ENGINE: GetEnvironmentMCPTools RETURNED - %d tools, %d clients, err=%v", len(allMCPTools), len(mcpClients), err != nil)
		if err != nil {
			if mcpLoadSpan != nil {
				mcpLoadSpan.RecordError(err)
				mcpLoadSpan.SetStatus(codes.Error, "failed to load MCP tools")
			}
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to get environment MCP tools")
			}
			return nil, fmt.Errorf("failed to get environment MCP tools for dotprompt execution: %w", err)
		}
		
		if mcpLoadSpan != nil {
			mcpLoadSpan.SetAttributes(
				attribute.Int("mcp.tools_loaded", len(allMCPTools)),
				attribute.Int("mcp.clients_connected", len(mcpClients)),
			)
		}
		
		// Store clients for cleanup after execution
		aee.activeMCPClients = mcpClients
		
		// Filter to only include tools assigned to this agent (same filtering logic as traditional)
		logging.Info("🔥 AGENT-ENGINE: About to filter tools - %d assigned tools from %d available MCP tools", len(agentTools), len(allMCPTools))
		logging.Debug("Filtering %d assigned tools from %d available MCP tools", len(agentTools), len(allMCPTools))
		var mcpTools []ai.ToolRef
		logging.Info("🔥 TOOL-FILTER: Starting tool filtering loop with %d assigned tools", len(agentTools))
		for i, assignedTool := range agentTools {
			logging.Info("🔥 TOOL-FILTER: Processing assigned tool %d/%d: %s", i+1, len(agentTools), assignedTool.ToolName)
			for j, mcpTool := range allMCPTools {
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
				
				if j < 5 || strings.Contains(toolName, "opencode") { // Log first 5 tools and any opencode tools
					logging.Info("🔥 TOOL-FILTER: Checking MCP tool %d: %s vs assigned %s", j, toolName, assignedTool.ToolName)
				}
				
				if toolName == assignedTool.ToolName {
					logging.Info("🔥 TOOL-FILTER: MATCHED! Adding tool %s", toolName)
					mcpTools = append(mcpTools, mcpTool)
					break
				}
			}
			logging.Info("🔥 TOOL-FILTER: Completed processing assigned tool %s", assignedTool.ToolName)
		}
		logging.Info("🔥 TOOL-FILTER: Tool filtering loop completed - found %d matching tools", len(mcpTools))
		
		logging.Debug("Dotprompt execution using %d tools (filtered from %d available)", len(mcpTools), len(allMCPTools))
		log.Printf("🔥 MCP-SETUP: MCP tools loaded - %d tools available, %d filtered", len(allMCPTools), len(mcpTools))
		
		// Add filtered tools count to span
		if span != nil {
			span.SetAttributes(attribute.Int("agent.filtered_tools_count", len(mcpTools)))
		}
		
		// Use our new dotprompt + genkit execution system with progressive logging
		log.Printf("🔥 MCP-SETUP: Creating dotprompt executor")
		executor := dotprompt.NewGenKitExecutor()
		
		// Create a logging callback for real-time progress updates
		logCallback := func(logEntry map[string]interface{}) {
			// Only store user-relevant logs in database for UI display
			if aee.shouldShowInLiveExecution(logEntry) {
				err := aee.repos.AgentRuns.AppendDebugLog(ctx, runID, logEntry)
				if err != nil {
					logging.Debug("Failed to append debug log: %v", err)
				}
			}
		}
		
		// Set the logging callback on the OpenAI plugin for detailed API call logging
		aee.genkitProvider.SetOpenAILogCallback(logCallback)
		
		log.Printf("🔥 AGENT-ENGINE: About to call dotprompt executor - agent: %s", agent.Name)
		
		// Create execution span
		var execSpan trace.Span
		if span != nil {
			ctx, execSpan = aee.telemetryService.StartSpan(ctx, "dotprompt.execute",
				trace.WithAttributes(
					attribute.String("task.preview", func() string {
						if len(task) > 200 { return task[:200] + "..." }
						return task
					}()),
				),
			)
			defer execSpan.End()
		}
		
		response, err := executor.ExecuteAgentWithDatabaseConfigAndLogging(*agent, agentTools, genkitApp, mcpTools, task, logCallback)
		log.Printf("🔥 AGENT-ENGINE: Dotprompt executor returned - response: %v, err: %v", response != nil, err)
		
		// Clean up MCP connections after execution is complete
		aee.mcpConnManager.CleanupConnections(aee.activeMCPClients)
		aee.activeMCPClients = nil
		
		if err != nil {
			// Record error in spans
			if execSpan != nil {
				execSpan.RecordError(err)
				execSpan.SetStatus(codes.Error, "dotprompt execution failed")
			}
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "agent execution failed")
			}
			
			// Log the execution failure for debugging
			if logCallback != nil {
				logCallback(map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
					"level":     "error",
					"message":   "Agent execution failed",
					"details": map[string]interface{}{
						"error":    err.Error(),
						"duration": time.Since(startTime).String(),
					},
				})
			}
			return nil, fmt.Errorf("dotprompt execution failed: %w", err)
		}
		
		// Add success metrics to spans
		duration := time.Since(startTime)
		if execSpan != nil {
			execSpan.SetAttributes(
				attribute.Bool("execution.success", response.Success),
				attribute.String("execution.model", response.ModelName),
				attribute.Float64("execution.duration_seconds", duration.Seconds()),
				attribute.Int("execution.steps_used", response.StepsUsed),
				attribute.Int("execution.tools_used", response.ToolsUsed),
			)
		}
		if span != nil {
			span.SetAttributes(
				attribute.Bool("execution.success", response.Success),
				attribute.String("execution.model", response.ModelName),
				attribute.Float64("execution.duration_seconds", duration.Seconds()),
				attribute.Int("execution.steps_used", response.StepsUsed),
				attribute.Int("execution.tools_used", response.ToolsUsed),
			)
		}

		// Convert ExecutionResponse to AgentExecutionResult  
		return &AgentExecutionResult{
			Success:        response.Success,
			Response:       response.Response,
			Duration:       duration,
			ModelName:      response.ModelName,
			StepsUsed:      response.StepsUsed,
			StepsTaken:     int64(response.StepsUsed), // Map StepsUsed to StepsTaken for database
			ToolsUsed:      response.ToolsUsed,
			Error:          response.Error,
			TokenUsage:     response.TokenUsage,           // ✅ Pass through token usage from dotprompt
			ToolCalls:      response.ToolCalls,           // ✅ Pass through tool calls
			ExecutionSteps: response.ExecutionSteps,     // ✅ Pass through execution steps
		}, nil
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

// shouldShowInLiveExecution filters out GenKit framework noise from live execution logs
// while keeping user-relevant information visible
func (aee *AgentExecutionEngine) shouldShowInLiveExecution(logEntry map[string]interface{}) bool {
	message, ok := logEntry["message"].(string)
	if !ok {
		return false
	}
	
	// Framework noise to filter out from live logs
	frameworkNoise := []string{
		"Context usage updated",
		"Turn 1/25 completed",
		"Turn 2/25 completed", 
		"Turn 3/25 completed",
		"Turn 4/25 completed",
		"Turn 5/25 completed",
		"Batch tool execution starting",
		"Batch tool execution completed", 
		"Enhanced generation starting",
		"Enhanced generation completed",
		"Station GenKit generation completed: success",
		"Starting Station-enhanced GenKit generation",
		// Additional GenKit/Station internal noise
		"🔧 STATION-GENERATE: Processing generation request with 4 options",
		"🔧 STATION-MIDDLEWARE: Request has 4 tools",
		"Starting Station-enhanced GenKit generation",
		"Turn 0: Model responded",
		"Turn 1: Model responded",
	}
	
	// Filter out turn completion messages (Turn X/Y completed)
	if strings.Contains(message, "Turn ") && strings.Contains(message, " completed") {
		return false
	}
	
	// Filter out turn messages with patterns
	if strings.Contains(message, "Turn ") && (strings.Contains(message, "Sending request to model") || strings.Contains(message, "Model requested") || strings.Contains(message, "Model responded")) {
		return false
	}
	
	// Filter out debug messages starting with emojis (application logic)
	if strings.HasPrefix(message, "🔧 ") || strings.HasPrefix(message, "🔥 ") || strings.HasPrefix(message, "📊 ") || strings.HasPrefix(message, "⚡ ") {
		return false
	}
	
	// Filter out specific framework noise
	for _, noise := range frameworkNoise {
		if message == noise {
			return false
		}
	}
	
	// Keep user-relevant logs
	return true
}
