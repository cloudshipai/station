package dotprompt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	
	"gopkg.in/yaml.v2"
	
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
func (e *GenKitExecutor) ExecuteAgentWithDotprompt(agent models.Agent, agentTools []*models.AgentToolWithDetails, genkitApp *genkit.Genkit, mcpTools []ai.ToolRef, task string) (*ExecutionResponse, error) {
	startTime := time.Now()
	fmt.Printf("DEBUG DOTPROMPT: Starting hybrid execution for agent %s\n", agent.Name)
	
	// 1. Construct complete dotprompt content from database agent data
	dotpromptContent := e.buildDotpromptFromAgent(agent, agentTools, "default")
	fmt.Printf("DEBUG DOTPROMPT: Built dotprompt content, length: %d\n", len(dotpromptContent))
	
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
	
	fmt.Printf("DEBUG DOTPROMPT: Dotprompt compiled successfully\n")
	
	// 3. Render the prompt with merged input data (default + custom schema)
	schemaHelper := schema.NewExportHelper()
	
	// For now, only use userInput. Custom input data can be added via call_agent variables parameter
	inputData, err := schemaHelper.GetMergedInputData(&agent, task, nil)
	if err != nil {
		// Fallback to basic userInput on schema error
		inputData = map[string]interface{}{
			"userInput": task,
		}
	}
	
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
	
	fmt.Printf("DEBUG DOTPROMPT: Rendered %d messages from dotprompt\n", len(renderedPrompt.Messages))
	
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
	
	// 5. Get model name with proper provider prefix
	baseModelName := renderedPrompt.Model
	if baseModelName == "" {
		baseModelName = "gemini-1.5-flash" // fallback
	}
	if cfg := e.getStationConfig(); cfg != nil && cfg.AIModel != "" {
		baseModelName = cfg.AIModel
	}
	modelName := fmt.Sprintf("googleai/%s", baseModelName)
	
	// 6. Extract tool names for GenKit (merge frontmatter tools + MCP tools)
	var toolNames []string
	for _, tool := range renderedPrompt.Tools {
		toolNames = append(toolNames, tool)
	}
	
	fmt.Printf("DEBUG DOTPROMPT: Using model %s with %d messages\n", modelName, len(genkitMessages))
	fmt.Printf("DEBUG DOTPROMPT: Frontmatter tools: %v (%d tools)\n", toolNames, len(toolNames))
	fmt.Printf("DEBUG DOTPROMPT: MCP tools received: %d tools\n", len(mcpTools))
	for i, tool := range mcpTools {
		fmt.Printf("DEBUG DOTPROMPT:   MCP Tool %d: %s\n", i+1, tool.Name())
	}
	
	// 7. Execute with GenKit's Generate for full multi-turn and tool support
	ctx := context.Background()
	
	// Build generate options (match traditional approach exactly)
	var generateOpts []ai.GenerateOption
	generateOpts = append(generateOpts, ai.WithModelName(modelName))  // Use same as traditional
	generateOpts = append(generateOpts, ai.WithMessages(genkitMessages...))
	generateOpts = append(generateOpts, ai.WithMaxTurns(25))
	
	// Add MCP tools if available (same as traditional)
	generateOpts = append(generateOpts, ai.WithTools(mcpTools...))
	fmt.Printf("DEBUG DOTPROMPT: ✅ Added %d MCP tools to GenKit Generate options\n", len(mcpTools))
	
	// Debug: Print actual tool names that GenKit will see
	for i, tool := range mcpTools {
		fmt.Printf("DEBUG DOTPROMPT: GenKit Tool %d: %s\n", i+1, tool.Name())
	}
	
	fmt.Printf("DEBUG DOTPROMPT: About to call genkit.Generate with %d options\n", len(generateOpts))
	response, err := genkit.Generate(ctx, genkitApp, generateOpts...)
	fmt.Printf("DEBUG DOTPROMPT: genkit.Generate completed, err=%v\n", err)
	if err != nil {
		fmt.Printf("DEBUG DOTPROMPT: Generate failed with error: %v\n", err)
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			Error:     fmt.Sprintf("Generate failed: %v", err),
		}, nil
	}
	
	if response == nil {
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			Error:     "Generate returned nil response",
		}, nil
	}
	
	fmt.Printf("DEBUG DOTPROMPT: Generated response, length: %d\n", len(response.Text()))
	
	// Helper function
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	
	// Extract tool calls and execution steps (same logic as traditional approach)
	var allToolCalls []interface{}
	var executionSteps []interface{}
	stepCounter := 1
	
	// Check if any tool calls were made in the response
	if response.Request != nil && response.Request.Messages != nil {
		fmt.Printf("DEBUG DOTPROMPT: Request contains %d input messages\n", len(response.Request.Messages))
		
		for i, msg := range response.Request.Messages {
			fmt.Printf("DEBUG DOTPROMPT:   Input Message %d: Role=%s, Parts=%d\n", i, msg.Role, len(msg.Content))
			
			// Extract tool requests, responses, and model thoughts
			var modelThoughts []string
			var toolRequestsInMessage []map[string]interface{}
			
			for _, part := range msg.Content {
				if part.IsToolRequest() && part.ToolRequest != nil {
					fmt.Printf("DEBUG DOTPROMPT:   Found tool request: %s\n", part.ToolRequest.Name)
					
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
					// Capture model's intermediate thoughts/reasoning
					modelThoughts = append(modelThoughts, part.Text)
					fmt.Printf("DEBUG DOTPROMPT:   Found model thought: %s\n", part.Text[:min(100, len(part.Text))])
				} else if part.IsToolResponse() && part.ToolResponse != nil {
					fmt.Printf("DEBUG DOTPROMPT:   Found tool response for: %s\n", part.ToolResponse.Name)
					
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
				for j := range toolRequestsInMessage {
					if j < len(allToolCalls) {
						allToolCalls[len(allToolCalls)-len(toolRequestsInMessage)+j].(map[string]interface{})["model_thoughts"] = strings.Join(modelThoughts, " ")
					}
				}
			}
		}
	}
	
	// Log response message information
	if response.Message != nil {
		fmt.Printf("DEBUG DOTPROMPT: Response message: Role=%s, Parts=%d\n", response.Message.Role, len(response.Message.Content))
		for j, part := range response.Message.Content {
			if part.IsToolRequest() {
				fmt.Printf("DEBUG DOTPROMPT:   Part %d: TOOL REQUEST - %s\n", j, part.ToolRequest.Name)
			} else {
				fmt.Printf("DEBUG DOTPROMPT:   Part %d: TEXT\n", j)
			}
		}
	} else {
		fmt.Printf("DEBUG DOTPROMPT: ⚠️  Response.Message is nil\n")
	}
	
	fmt.Printf("DEBUG DOTPROMPT: Extracted %d tool calls and %d execution steps\n", len(allToolCalls), len(executionSteps))
	
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
	
	return &ExecutionResponse{
		Success:        true,
		Response:       response.Text(),
		ToolCalls:      toolCallsArray,
		ExecutionSteps: executionStepsArray,
		Duration:       time.Since(startTime),
		ModelName:      modelName,
		StepsUsed:      len(allToolCalls), // Actual number of tool calls made
		ToolsUsed:      len(allToolCalls), // Actual number of tools used
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
	
	// Success! Add info about which approach was used
	result.Response = fmt.Sprintf("🚀 Dotprompt + GenKit Execution:\n%s", result.Response)
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

// isDotpromptContent checks if the prompt contains dotprompt frontmatter
func (e *GenKitExecutor) isDotpromptContent(prompt string) bool {
	// Check for YAML frontmatter markers
	return strings.HasPrefix(strings.TrimSpace(prompt), "---") && 
		   strings.Contains(prompt, "\n---\n")
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
	stationConfig, err := e.loadStationConfig()
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
func (e *GenKitExecutor) getActiveModel(config *DotpromptConfig) string {
	// Try to load Station config
	stationConfig, err := e.loadStationConfig()
	if err == nil && stationConfig.AIModel != "" {
		// Station config takes priority
		return stationConfig.AIModel
	}
	
	// Fallback to dotprompt config
	if config.Model != "" {
		return config.Model
	}
	
	// Ultimate fallback
	return "gemini-1.5-flash"
}

// loadStationConfig loads the Station configuration
func (e *GenKitExecutor) loadStationConfig() (*StationConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	
	configPath := filepath.Join(homeDir, ".config", "station", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	
	var config StationConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}

// StationConfig represents the Station configuration structure
type StationConfig struct {
	AIModel   string `yaml:"ai_model"`
	AIProvider string `yaml:"ai_provider"`
}

// isModelSupported checks if a model is supported (for testing)
func (e *GenKitExecutor) isModelSupported(config *DotpromptConfig) bool {
	supportedModels := map[string]bool{
		"gemini-2.0-flash-exp": true,
		"gpt-4":                true,
		"gpt-3.5-turbo":        true,
	}
	
	return supportedModels[config.Model]
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
	if cfg := e.getStationConfig(); cfg != nil && cfg.AIModel != "" {
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

// getStationConfig loads the Station configuration for model info
func (e *GenKitExecutor) getStationConfig() *StationConfig {
	config, err := e.loadStationConfig()
	if err != nil {
		return nil
	}
	return config
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