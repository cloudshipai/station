package dotprompt

import (
	"context"
	"fmt"
	"strings"
	"time"
	
	"station/internal/config"
	"station/internal/logging"
	"station/pkg/models"
	"station/pkg/schema"
	
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/google/dotprompt/go/dotprompt"
)

// GenKitExecutor handles dotprompt-based agent execution using GenKit Generate
type GenKitExecutor struct{}

// NewGenKitExecutor creates a new GenKit-based dotprompt executor
func NewGenKitExecutor() *GenKitExecutor {
	return &GenKitExecutor{}
}

// ExecuteAgentWithDotpromptTemplate is deprecated - use ExecuteAgentWithDatabaseConfig instead
// This method is kept for backward compatibility with existing tests only
func (e *GenKitExecutor) ExecuteAgentWithDotpromptTemplate(extractor *RuntimeExtraction, request *ExecutionRequest) (*ExecutionResponse, error) {
	// Redirect to the real implementation
	return nil, fmt.Errorf("ExecuteAgentWithDotpromptTemplate is deprecated - use ExecuteAgentWithDatabaseConfig instead")
}

// ExecuteAgentWithDotprompt executes an agent using hybrid approach: dotprompt direct + GenKit Generate
func (e *GenKitExecutor) ExecuteAgentWithDotprompt(agent models.Agent, agentTools []*models.AgentToolWithDetails, genkitApp *genkit.Genkit, mcpTools []ai.ToolRef, task string, userVariables ...map[string]interface{}) (*ExecutionResponse, error) {
	startTime := time.Now()
	logging.Debug("Starting unified dotprompt execution for agent %s", agent.Name)
	
	// Handle optional userVariables parameter
	var customData map[string]interface{}
	logging.Debug("DEBUG GenKitExecutor: userVariables length = %d", len(userVariables))
	if len(userVariables) > 0 {
		customData = userVariables[0]
		logging.Debug("DEBUG GenKitExecutor: Received %d user variables: %+v", len(customData), customData)
	} else {
		logging.Debug("DEBUG GenKitExecutor: No user variables provided")
	}
	
	// 1. Use agent prompt directly if it contains multi-role syntax
	var dotpromptContent string
	if e.isDotpromptContent(agent.Prompt) {
		// Agent already has dotprompt format (either frontmatter or multi-role)
		dotpromptContent = agent.Prompt
		logging.Debug("Using agent prompt directly, length: %d", len(dotpromptContent))
	} else {
		// Legacy agent, build frontmatter
		dotpromptContent = e.buildDotpromptFromAgent(agent, agentTools, "default")
		logging.Debug("Built dotprompt content, length: %d", len(dotpromptContent))
	}
	
	// 2. Use dotprompt library directly for multi-role rendering (bypasses GenKit constraint)
	dp := dotprompt.NewDotprompt(nil)
	promptFunc, err := dp.Compile(dotpromptContent, nil)
	if err != nil {
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			Error:     fmt.Sprintf("failed to compile dotprompt: %v", err),
		}, nil
	}
	
	logging.Debug("Dotprompt compiled successfully")
	
	// 3. Render the prompt with merged input data (default + custom schema)
	schemaHelper := schema.NewExportHelper()
	
	// Pass user variables to schema system for agents with JSON Schema, or just userInput for simple agents
	inputData, err := schemaHelper.GetMergedInputData(&agent, task, customData)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare input data for agent %s: %w", agent.Name, err)
	}
	logging.Debug("Input data prepared with %d fields: %+v", len(inputData), inputData)
	
	data := &dotprompt.DataArgument{
		Input: inputData,
	}
	
	renderedPrompt, err := promptFunc(data, nil)
	if err != nil {
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			Error:     fmt.Sprintf("failed to render dotprompt: %v", err),
		}, nil
	}
	
	// Process rendered prompt messages
	logging.Debug("Rendered %d messages from dotprompt", len(renderedPrompt.Messages))
	
	// 4. Convert dotprompt messages to GenKit messages
	genkitMessages, err := e.convertDotpromptToGenkitMessages(renderedPrompt.Messages)
	if err != nil {
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			Error:     fmt.Sprintf("failed to convert messages: %v", err),
		}, nil
	}
	logging.Debug("Converted to %d GenKit messages", len(genkitMessages))
	
	// 5. Get model name with proper provider prefix (same logic as agent_execution_engine.go)
	baseModelName := renderedPrompt.Model
	if baseModelName == "" {
		baseModelName = "gemini-1.5-flash" // fallback
	}
	
	// Load configuration 
	cfg, err := config.Load()
	if err != nil {
		logging.Debug("Failed to load config: %v, using defaults", err)
		cfg = &config.Config{
			AIProvider: "openai",
			AIModel:    baseModelName,
		}
	}
	
	// Override model from config if available
	if cfg.AIModel != "" {
		baseModelName = cfg.AIModel
	}
	
	// Use the same provider resolution logic as agent_execution_engine.go:237-256
	var modelName string
	switch strings.ToLower(cfg.AIProvider) {
	case "gemini", "googlegenai":
		modelName = fmt.Sprintf("googleai/%s", baseModelName)
	case "openai":
		modelName = fmt.Sprintf("openai/%s", baseModelName)
	default:
		modelName = fmt.Sprintf("%s/%s", cfg.AIProvider, baseModelName)
	}
	
	logging.Debug("Using model %s (provider: %s, config: %s)", modelName, cfg.AIProvider, baseModelName)
	
	// 6. Extract tool names for GenKit (merge frontmatter tools + MCP tools)
	var toolNames []string
	for _, tool := range renderedPrompt.Tools {
		toolNames = append(toolNames, tool)
	}
	
	logging.Debug("Using model %s with %d messages and %d MCP tools", modelName, len(genkitMessages), len(mcpTools))
	
	// 7. Execute with GenKit's Generate for full multi-turn and tool support
	ctx := context.Background()
	
	// Build generate options (match traditional approach exactly)
	var generateOpts []ai.GenerateOption
	generateOpts = append(generateOpts, ai.WithModelName(modelName))  // Use same as traditional
	generateOpts = append(generateOpts, ai.WithMessages(genkitMessages...))
	generateOpts = append(generateOpts, ai.WithMaxTurns(25))
	
	// Add MCP tools if available (same as traditional)
	generateOpts = append(generateOpts, ai.WithTools(mcpTools...))
	
	// Execute AI generation with configured options
	response, err := genkit.Generate(ctx, genkitApp, generateOpts...)
	if err != nil {
		// Generation failed - return error response
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			Error:     fmt.Sprintf("Generate failed: %v", err),
		}, nil
	}
	
	if response == nil {
		// Generation returned nil response - return error
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			Error:     "Generate returned nil response",
		}, nil
	}
	
	responseText := response.Text()
	
	// Generation successful - extract response text
	
	// Extract tool calls and execution steps with detailed debugging
	var allToolCalls []interface{}
	var executionSteps []interface{}
	stepCounter := 1
	
	logging.Debug("=== TOOL CALLS & EXECUTION STEPS EXTRACTION ===")
	
	// Check if any tool calls were made in the response
	if response.Request != nil && response.Request.Messages != nil {
		logging.Debug("Processing %d messages from response.Request", len(response.Request.Messages))
		
		for msgIdx, msg := range response.Request.Messages {
			logging.Debug("  Message[%d]: Role=%s, ContentParts=%d", msgIdx, msg.Role, len(msg.Content))
			
			// Extract tool requests, responses, and model thoughts
			var modelThoughts []string
			var toolRequestsInMessage []map[string]interface{}
			
			for partIdx, part := range msg.Content {
				logging.Debug("    Part[%d]: IsToolRequest=%t, IsText=%t, IsToolResponse=%t", 
					partIdx, part.IsToolRequest(), part.IsText(), part.IsToolResponse())
				
				if part.IsToolRequest() && part.ToolRequest != nil {
					logging.Debug("      ToolRequest: Name=%s, Ref=%s", part.ToolRequest.Name, part.ToolRequest.Ref)
					logging.Debug("      ToolRequest Input: %+v", part.ToolRequest.Input)
					
					// Add tool call to array
					toolCall := map[string]interface{}{
						"step":           stepCounter,
						"tool_name":      part.ToolRequest.Name,
						"tool_input":     part.ToolRequest.Input,
						"tool_call_id":   part.ToolRequest.Ref,
						"message_role":   string(msg.Role),
					}
					allToolCalls = append(allToolCalls, toolCall)
					toolRequestsInMessage = append(toolRequestsInMessage, toolCall)
					
					// Add execution step
					executionStep := map[string]interface{}{
						"step":      stepCounter,
						"type":      "tool_call",
						"tool_name": part.ToolRequest.Name,
						"input":     part.ToolRequest.Input,
						"timestamp": time.Now().Format(time.RFC3339),
					}
					executionSteps = append(executionSteps, executionStep)
					stepCounter++
					
				} else if part.IsText() && part.Text != "" {
					logging.Debug("      Text content: %q", part.Text)
					// Capture model's intermediate thoughts/reasoning
					modelThoughts = append(modelThoughts, part.Text)
					
				} else if part.IsToolResponse() && part.ToolResponse != nil {
					logging.Debug("      ToolResponse: Name=%s", part.ToolResponse.Name)
					logging.Debug("      ToolResponse Output: %+v", part.ToolResponse.Output)
					
					// Add tool response as execution step
					executionStep := map[string]interface{}{
						"step":      stepCounter,
						"type":      "tool_response", 
						"tool_name": part.ToolResponse.Name,
						"output":    part.ToolResponse.Output,
						"timestamp": time.Now().Format(time.RFC3339),
					}
					executionSteps = append(executionSteps, executionStep)
					stepCounter++
				}
			}
			
			// If there were model thoughts alongside tool requests, add them to tool calls
			if len(modelThoughts) > 0 && len(toolRequestsInMessage) > 0 {
				logging.Debug("    Adding model thoughts to %d tool requests", len(toolRequestsInMessage))
				for j := range toolRequestsInMessage {
					if j < len(allToolCalls) {
						allToolCalls[len(allToolCalls)-len(toolRequestsInMessage)+j].(map[string]interface{})["model_thoughts"] = strings.Join(modelThoughts, " ")
					}
				}
			} else if len(modelThoughts) > 0 {
				logging.Debug("    Found model thoughts without tool requests: %d thoughts", len(modelThoughts))
				// Add model thoughts as execution step even if no tool calls
				executionStep := map[string]interface{}{
					"step":      stepCounter,
					"type":      "model_reasoning",
					"content":   strings.Join(modelThoughts, " "),
					"timestamp": time.Now().Format(time.RFC3339),
				}
				executionSteps = append(executionSteps, executionStep)
				stepCounter++
			}
		}
	} else {
		logging.Debug("No messages found in response.Request")
	}
	
	// Note: Some AI providers put tool calls in different response fields,
	// but this GenKit version uses response.Request.Messages
	
	logging.Debug("EXTRACTION SUMMARY: %d tool calls, %d execution steps", len(allToolCalls), len(executionSteps))
	
	// Log detailed summary of extracted data
	if len(allToolCalls) > 0 {
		logging.Debug("Tool calls extracted:")
		for i, toolCall := range allToolCalls {
			if tc, ok := toolCall.(map[string]interface{}); ok {
				logging.Debug("  [%d] %s (step %v)", i, tc["tool_name"], tc["step"])
			}
		}
	}
	
	if len(executionSteps) > 0 {
		logging.Debug("Execution steps extracted:")
		for i, step := range executionSteps {
			if s, ok := step.(map[string]interface{}); ok {
				logging.Debug("  [%d] %s (step %v)", i, s["type"], s["step"])
			}
		}
	}
	
	// Convert to JSONArray format
	var toolCallsArray *models.JSONArray
	var executionStepsArray *models.JSONArray
	
	if len(allToolCalls) > 0 {
		jsonArray := models.JSONArray(allToolCalls)
		toolCallsArray = &jsonArray
	}
	
	if len(executionSteps) > 0 {
		jsonArray := models.JSONArray(executionSteps)
		executionStepsArray = &jsonArray
	}
	
	// Debug: Dissect the GenKit response object to understand its structure
	tokenUsage := make(map[string]interface{})
	if response != nil {
		logging.Debug("GenKit Response Object Structure Analysis:")
		logging.Debug("  response != nil: true")
		logging.Debug("  response.Text(): %q", response.Text())
		
		// Check Usage field
		if response.Usage != nil {
			logging.Debug("  response.Usage != nil: true")
			logging.Debug("  response.Usage.InputTokens: %d", response.Usage.InputTokens)
			logging.Debug("  response.Usage.OutputTokens: %d", response.Usage.OutputTokens)
			tokenUsage["input_tokens"] = response.Usage.InputTokens
			tokenUsage["output_tokens"] = response.Usage.OutputTokens
			tokenUsage["total_tokens"] = response.Usage.InputTokens + response.Usage.OutputTokens
		} else {
			logging.Debug("  response.Usage: nil")
		}
		
		// Check Request field (sometimes token usage is here)
		if response.Request != nil {
			logging.Debug("  response.Request != nil: true")
			if response.Request.Messages != nil {
				logging.Debug("  response.Request.Messages: %d messages", len(response.Request.Messages))
			}
		} else {
			logging.Debug("  response.Request: nil")
		}
		
		// Check if there are other response fields we should examine
		// (Candidates field not available in this GenKit version)
		
		// Try to access other potential fields using reflection-like approach
		logging.Debug("  Full response type: %T", response)
		
		// Log token usage summary
		if len(tokenUsage) > 0 {
			logging.Debug("  Extracted token usage: %+v", tokenUsage)
		} else {
			logging.Debug("  No token usage extracted")
		}
	} else {
		logging.Debug("GenKit response is nil")
	}

	return &ExecutionResponse{
		Success:        true,
		Response:       responseText, // Use the responseText variable we created
		ToolCalls:      toolCallsArray,
		ExecutionSteps: executionStepsArray,
		Duration:       time.Since(startTime),
		ModelName:      modelName,
		StepsUsed:      len(executionSteps), // Actual number of execution steps
		ToolsUsed:      len(allToolCalls),   // Actual number of tools used
		TokenUsage:     tokenUsage,          // Add extracted token usage
		Error:          "",
	}, nil
}

// ExecuteAgentWithDatabaseConfig executes an agent using the dotprompt + genkit approach
func (e *GenKitExecutor) ExecuteAgentWithDatabaseConfig(agent models.Agent, agentTools []*models.AgentToolWithDetails, genkitApp *genkit.Genkit, mcpTools []ai.ToolRef, task string) (*ExecutionResponse, error) {
	// Execute using the dotprompt + genkit approach
	result, err := e.ExecuteAgentWithDotprompt(agent, agentTools, genkitApp, mcpTools, task)
	if err != nil {
		return &ExecutionResponse{
			Success:  false,
			Response: "",
			Duration: 0,
			Error:    fmt.Sprintf("❌ Dotprompt + GenKit execution failed: %v\n\nThis agent requires the new dotprompt + genkit execution system. Please check that:\n- Agent configuration is valid\n- GenKit provider is properly initialized\n- All required tools are available", err),
		}, nil
	}
	
	// Success! Return clean response without execution engine prefix
	return result, nil
}

// extractToolNames extracts tool names from agent tools for display
func (e *GenKitExecutor) extractToolNames(agentTools []*models.AgentToolWithDetails) []string {
	var toolNames []string
	for _, tool := range agentTools {
		// AgentToolWithDetails has ToolName field from the join
		toolNames = append(toolNames, tool.ToolName)
	}
	return toolNames
}

// isDotpromptContent checks if the prompt contains dotprompt frontmatter or multi-role syntax
func (e *GenKitExecutor) isDotpromptContent(prompt string) bool {
	trimmed := strings.TrimSpace(prompt)
	
	// Check for YAML frontmatter markers
	hasFrontmatter := strings.HasPrefix(trimmed, "---") && 
		   strings.Contains(prompt, "\n---\n")
		   
	// Check for multi-role dotprompt syntax
	hasMultiRole := strings.Contains(prompt, "{{role \"") || strings.Contains(prompt, "{{role '")
	
	return hasFrontmatter || hasMultiRole
}

// getPromptSource returns a description of the prompt source type
func (e *GenKitExecutor) getPromptSource(prompt string) string {
	if e.isDotpromptContent(prompt) {
		return "dotprompt (frontmatter + template)"
	}
	return "simple text prompt"
}

// RenderDotpromptContent renders a dotprompt template with the given variables using the new multi-role system
func (e *GenKitExecutor) RenderDotpromptContent(dotpromptContent, task, agentName string) (string, error) {
	// 1. Create renderer
	renderer := NewRenderer()
	
	// 2. Prepare render context with automatic variables
	context := &RenderContext{
		UserInput:   task,
		AgentName:   agentName,
		Environment: "default", // TODO: get from agent config
		UserVariables: make(map[string]interface{}),
	}
	
	// 3. Render the template using our multi-role system
	parsed, err := renderer.Render(dotpromptContent, context)
	if err != nil {
		return "", fmt.Errorf("failed to render dotprompt: %w", err)
	}
	
	// 4. Convert to Genkit-compatible format
	renderedText, err := renderer.RenderToGenkit(parsed)
	if err != nil {
		return "", fmt.Errorf("failed to convert to Genkit format: %w", err)
	}
	
	return renderedText, nil
}

// getActiveModelFromConfig gets the model from Station config (without dotprompt fallback)
func (e *GenKitExecutor) getActiveModelFromConfig() string {
	// Try to load Station config
	stationConfig, err := config.Load()
	if err == nil && stationConfig.AIModel != "" {
		return stationConfig.AIModel
	}
	
	// Fallback if Station config not available
	return "gemini-1.5-flash"
}

// ExecuteAgentWithGenerate provides OpenAI compatibility - deprecated in favor of ExecuteAgentWithDatabaseConfig
// This method is kept for backward compatibility with existing tests only
func (e *GenKitExecutor) ExecuteAgentWithGenerate(extractor *RuntimeExtraction, request *ExecutionRequest) (*ExecutionResponse, error) {
	return nil, fmt.Errorf("ExecuteAgentWithGenerate is deprecated - use ExecuteAgentWithDatabaseConfig for real execution")
}

// renderTemplate performs basic template variable substitution
func (e *GenKitExecutor) renderTemplate(template string, variables map[string]interface{}) (string, error) {
	rendered := template
	
	// Basic variable substitution
	for key, value := range variables {
		placeholder := "{{" + key + "}}"
		valueStr := fmt.Sprintf("%v", value)
		rendered = strings.ReplaceAll(rendered, placeholder, valueStr)
	}
	
	return rendered, nil
}

// getActiveModel determines the active model using Station config priority
func (e *GenKitExecutor) getActiveModel(dpConfig *DotpromptConfig) string {
	// Try to load Station config
	stationConfig, err := config.Load()
	if err == nil && stationConfig.AIModel != "" {
		// Station config takes priority
		return stationConfig.AIModel
	}
	
	// Fallback to dotprompt config
	if dpConfig.Model != "" {
		return dpConfig.Model
	}
	
	// Ultimate fallback
	return "gemini-1.5-flash"
}


// isModelSupported checks if a model is supported (for testing)
func (e *GenKitExecutor) isModelSupported(dpConfig *DotpromptConfig) bool {
	supportedModels := map[string]bool{
		"gemini-2.0-flash-exp": true,
		"gpt-4":                true,
		"gpt-3.5-turbo":        true,
	}
	
	return supportedModels[dpConfig.Model]
}

// buildDotpromptFromAgent constructs complete dotprompt content from database agent data
// This reuses the same logic as our export functions
func (e *GenKitExecutor) buildDotpromptFromAgent(agent models.Agent, agentTools []*models.AgentToolWithDetails, environment string) string {
	// Check if agent prompt already contains dotprompt frontmatter
	if strings.HasPrefix(strings.TrimSpace(agent.Prompt), "---") {
		// Agent prompt is already a complete dotprompt, use as-is
		return agent.Prompt
	}

	// Agent prompt is simple text, wrap it with frontmatter
	var content strings.Builder

	// Get configured model from Station config, fallback to default
	modelName := "gemini-1.5-flash" // default fallback
	if cfg, _ := config.Load(); cfg != nil && cfg.AIModel != "" {
		modelName = cfg.AIModel
	}

	// YAML frontmatter with multi-role support (same as export logic)
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("model: \"%s\"\n", modelName))
	content.WriteString("config:\n")
	content.WriteString("  temperature: 0.3\n")
	content.WriteString("  max_tokens: 2000\n")
	content.WriteString("  maxTurns: 25\n")
	
	// Input schema with merged custom and default variables
	schemaHelper := schema.NewExportHelper()
	inputSchemaSection, err := schemaHelper.GenerateInputSchemaSection(&agent)
	if err != nil {
		// Fallback to default if custom schema is invalid
		content.WriteString("input:\n")
		content.WriteString("  schema:\n")
		content.WriteString("    userInput: string\n")
	} else {
		content.WriteString(inputSchemaSection)
	}
	
	content.WriteString("metadata:\n")
	content.WriteString(fmt.Sprintf("  name: \"%s\"\n", agent.Name))
	if agent.Description != "" {
		content.WriteString(fmt.Sprintf("  description: \"%s\"\n", agent.Description))
	}
	content.WriteString("  version: \"1.0.0\"\n")

	// Tools section
	if len(agentTools) > 0 {
		content.WriteString("tools:\n")
		for _, tool := range agentTools {
			content.WriteString(fmt.Sprintf("  - \"%s\"\n", tool.ToolName))
		}
	}

	// Station metadata
	content.WriteString("station:\n")
	content.WriteString("  execution_metadata:\n")
	if agent.MaxSteps > 0 {
		content.WriteString(fmt.Sprintf("    max_steps: %d\n", agent.MaxSteps))
	}
	content.WriteString(fmt.Sprintf("    environment: \"%s\"\n", environment))
	content.WriteString(fmt.Sprintf("    agent_id: %d\n", agent.ID))
	content.WriteString("---\n\n")

	// Multi-role template using dotprompt {{role}} directives
	content.WriteString("{{role \"system\"}}\n")
	content.WriteString(agent.Prompt)
	content.WriteString("\n\n{{role \"user\"}}\n")
	content.WriteString("{{userInput}}")
	content.WriteString("\n")

	return content.String()
}


// extractPromptContent extracts just the prompt content from a dotprompt file (removes frontmatter)
func (e *GenKitExecutor) extractPromptContent(dotpromptContent string) (string, error) {
	// Split by frontmatter markers
	parts := strings.Split(dotpromptContent, "---")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid dotprompt format: missing frontmatter markers")
	}
	
	// Return everything after the second "---"
	content := strings.Join(parts[2:], "---")
	return strings.TrimSpace(content), nil
}

// isMultiRolePrompt checks if a prompt already contains role directives
func (e *GenKitExecutor) isMultiRolePrompt(prompt string) bool {
	return strings.Contains(prompt, "{{role \"") || strings.Contains(prompt, "{{role '")
}

// convertDotpromptToGenkitMessages converts dotprompt messages to GenKit messages
func (e *GenKitExecutor) convertDotpromptToGenkitMessages(dotpromptMessages []dotprompt.Message) ([]*ai.Message, error) {
	var genkitMessages []*ai.Message
	
	for _, dpMsg := range dotpromptMessages {
		// Convert role
		var role ai.Role
		switch dpMsg.Role {
		case dotprompt.RoleUser:
			role = ai.RoleUser
		case dotprompt.RoleModel:
			role = ai.RoleModel
		case dotprompt.RoleSystem:
			role = ai.RoleSystem
		case dotprompt.RoleTool:
			role = ai.RoleTool
		default:
			role = ai.RoleUser // fallback
		}
		
		// Convert content parts
		var parts []*ai.Part
		for _, dpPart := range dpMsg.Content {
			switch part := dpPart.(type) {
			case *dotprompt.TextPart:
				if part.Text != "" {
					parts = append(parts, ai.NewTextPart(part.Text))
				}
			case *dotprompt.MediaPart:
				// Convert media part if needed
				if part.Media.URL != "" {
					parts = append(parts, ai.NewMediaPart(part.Media.ContentType, part.Media.URL))
				}
			}
		}
		
		// Skip empty messages
		if len(parts) == 0 {
			continue
		}
		
		// Create GenKit message
		genkitMsg := &ai.Message{
			Role:    role,
			Content: parts,
		}
		
		genkitMessages = append(genkitMessages, genkitMsg)
	}
	
	return genkitMessages, nil
}