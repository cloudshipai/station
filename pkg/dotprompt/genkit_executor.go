package dotprompt

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"station/internal/config"
	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"gopkg.in/yaml.v2"
)

// ToolCallTracker monitors tool usage to prevent obsessive calling loops
type ToolCallTracker struct {
	TotalCalls          int
	ConsecutiveSameTool map[string]int
	LastToolUsed        string
	MaxToolCalls        int
	MaxConsecutive      int
	LogCallback         func(map[string]interface{})
	ToolFailures        int  // Track number of tool failures
	HasToolFailures     bool // Track if any tool failures occurred
}

// DotPromptMetadata represents the metadata section in dotprompt frontmatter
type DotPromptMetadata struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
	App         string   `yaml:"app"`      // CloudShip data ingestion app classification (optional)
	AppType     string   `yaml:"app_type"` // CloudShip data ingestion app_type classification (optional)
}

// DotPromptConfig represents the YAML frontmatter in a .prompt file
type DotPromptConfig struct {
	Metadata map[string]interface{} `yaml:"metadata"`
	Model    string                 `yaml:"model"`
	MaxSteps int                    `yaml:"max_steps"`
	Tools    []string               `yaml:"tools"`
	Input    map[string]interface{} `yaml:"input"`
	Output   map[string]interface{} `yaml:"output"`
}

// GenKitExecutor handles agent execution using GenKit's dotprompt.Execute()
type GenKitExecutor struct {
	logCallback func(map[string]interface{})
}

// NewGenKitExecutor creates a new GenKit-based dotprompt executor
func NewGenKitExecutor() *GenKitExecutor {
	return &GenKitExecutor{}
}

// extractDotPromptMetadata extracts app/app_type metadata from dotprompt file if it exists
func (e *GenKitExecutor) extractDotPromptMetadata(promptPath string) (app, appType string) {
	content, err := os.ReadFile(promptPath)
	if err != nil {
		return "", "" // Gracefully handle file read errors
	}

	// Parse the frontmatter (similar to existing dotprompt parsers)
	parts := strings.Split(string(content), "---")
	if len(parts) < 3 {
		return "", "" // No frontmatter found
	}

	// Extract YAML frontmatter (parts[1])
	yamlContent := strings.TrimSpace(parts[1])
	if yamlContent == "" {
		return "", ""
	}

	var config DotPromptConfig
	if err := yaml.Unmarshal([]byte(yamlContent), &config); err != nil {
		return "", "" // Gracefully handle YAML parse errors
	}

	// Extract app and app_type from metadata if they exist
	if config.Metadata != nil {
		if appVal, exists := config.Metadata["app"]; exists {
			if appStr, ok := appVal.(string); ok {
				app = appStr
			}
		}
		if appTypeVal, exists := config.Metadata["app_type"]; exists {
			if appTypeStr, ok := appTypeVal.(string); ok {
				appType = appTypeStr
			}
		}
	}

	// Validate app_type against the 5 standard CloudShip types
	if appType != "" {
		validAppTypes := map[string]bool{
			"investigations": true,
			"opportunities":  true,
			"projections":    true,
			"inventory":      true,
			"events":         true,
		}

		if !validAppTypes[appType] {
			// Invalid app_type - return empty to skip data ingestion
			return "", ""
		}
	}

	return app, appType
}

// ExecuteAgent executes an agent using dotprompt.Execute() with registered MCP tools
func (e *GenKitExecutor) ExecuteAgent(ctx context.Context, agent models.Agent, agentTools []*models.AgentToolWithDetails, genkitApp *genkit.Genkit, mcpTools []ai.ToolRef, task string, logCallback func(map[string]interface{}), environmentName string, userVariables map[string]interface{}) (*ExecutionResponse, error) {
	startTime := time.Now()
	e.logCallback = logCallback

	// Get agent's dotprompt file path using provided environment name
	promptPath, err := e.getAgentPromptPath(agent, environmentName)
	if err != nil {
		promptPathError := fmt.Errorf("failed to get agent prompt path: %w", err)
		return &ExecutionResponse{
			Success:  false,
			Response: "",
			Duration: time.Since(startTime),
			Error:    promptPathError.Error(),
		}, promptPathError
	}

	// Extract metadata from dotprompt file (app/app_type for data ingestion)
	app, appType := e.extractDotPromptMetadata(promptPath)
	if app != "" || appType != "" {
		if e.logCallback != nil {
			e.logCallback(map[string]interface{}{
				"event":    "metadata_extracted",
				"app":      app,
				"app_type": appType,
			})
		}
	}

	// CRITICAL: Register filtered MCP tools BEFORE loading the prompt
	// The prompt's frontmatter may reference tools in its `tools:` section
	// and GenKit requires those tools to already be registered actions
	// Use panic recovery to skip already-registered tools (happens in child agent executions)
	registeredCount := 0
	skippedCount := 0
	for _, toolRef := range mcpTools {
		if tool, ok := toolRef.(ai.Tool); ok {
			// Wrap in anonymous function to use defer/recover for already-registered tools
			func() {
				defer func() {
					if r := recover(); r != nil {
						// Tool already registered - this is normal for child agent executions
						// where parent already registered the tools
						skippedCount++
						fmt.Printf("DEBUG: Tool %s already registered (skipped)\n", tool.Name())
					}
				}()
				fmt.Printf("DEBUG: Registering tool: %s (type: %T)\n", tool.Name(), tool)
				genkit.RegisterAction(genkitApp, tool)
				registeredCount++
			}()
		}
	}
	fmt.Printf("DEBUG: Registered %d tools, skipped %d already-registered tools for agent %s\n", registeredCount, skippedCount, agent.Name)

	// Load or lookup dotprompt file AFTER registering tools
	// For child agent executions, the prompt is already registered by the parent
	// Try to lookup first, then load if not found (avoids re-registration panic)
	agentPrompt := genkit.LookupPrompt(genkitApp, agent.Name)

	if agentPrompt == nil {
		// Prompt not registered yet - load it (this is the first execution)
		fmt.Printf("DEBUG: Loading prompt for %s (first execution)\n", agent.Name)
		agentPrompt = genkit.LoadPrompt(genkitApp, promptPath, "")

		if agentPrompt == nil {
			promptLoadError := fmt.Errorf("failed to load prompt from: %s", promptPath)
			return &ExecutionResponse{
				Success:  false,
				Response: "",
				Duration: time.Since(startTime),
				Error:    promptLoadError.Error(),
			}, promptLoadError
		}
	} else {
		// Prompt already registered - reuse it (child agent execution)
		fmt.Printf("DEBUG: Reusing already-registered prompt for %s (child agent)\n", agent.Name)
	}

	// Get model configuration
	modelName := e.getModelName()

	// Execute with dotprompt.Execute() - use 5-minute timeout for tool execution
	execCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	maxTurns := int(agent.MaxSteps)
	if maxTurns <= 0 {
		maxTurns = 25
	}

	// Merge userVariables with userInput for dotprompt template rendering
	// This allows agents with input schemas to receive structured parameters
	inputMap := map[string]any{"userInput": task}
	if userVariables != nil {
		for k, v := range userVariables {
			inputMap[k] = v
		}
	}

	// Note: We intentionally do NOT create our own root span here because GenKit v1.0.1
	// creates its own trace context that ignores our parent span. Creating a span here
	// results in duplicate traces (our empty root + GenKit's generate traces).
	// Instead, the agent_execution_engine creates a proper span with full metadata.

	// Pass tools to Execute - this is CRITICAL for function calling to work
	// Tools must be explicitly passed via ai.WithTools()
	resp, err := agentPrompt.Execute(execCtx,
		ai.WithInput(inputMap),
		ai.WithMaxTurns(maxTurns),
		ai.WithModelName(modelName),
		ai.WithTools(mcpTools...))

	if err != nil {
		execError := fmt.Errorf("dotprompt.Execute() failed: %w", err)
		return &ExecutionResponse{
			Success:  false,
			Response: "",
			Duration: time.Since(startTime),
			Error:    execError.Error(),
		}, execError
	}

	// Extract response data (use working pattern from existing code)
	finalResponse := resp.Text()

	// Extract token usage
	tokenUsage := make(map[string]interface{})
	if resp.Usage != nil {
		tokenUsage["input_tokens"] = resp.Usage.InputTokens
		tokenUsage["output_tokens"] = resp.Usage.OutputTokens
		tokenUsage["total_tokens"] = resp.Usage.TotalTokens
		if resp.Usage.CachedContentTokens > 0 {
			tokenUsage["cached_tokens"] = resp.Usage.CachedContentTokens
		}
	}

	// Extract tool calls AND tool responses from conversation history
	// We need both for complete benchmark evaluation (request params + response output)
	history := resp.History()
	var toolCallsArray *models.JSONArray
	var toolCallsInterface models.JSONArray

	// First pass: collect all tool requests
	toolRequestMap := make(map[string]map[string]interface{}) // Map by ref ID for matching responses

	for _, msg := range history {
		if msg.Content != nil && len(msg.Content) > 0 {
			for _, part := range msg.Content {
				if part.IsToolRequest() {
					toolReq := part.ToolRequest
					toolCall := map[string]interface{}{
						"tool_name":    toolReq.Name,
						"parameters":   toolReq.Input,
						"tool_call_id": toolReq.Ref,
					}
					toolRequestMap[toolReq.Ref] = toolCall
					toolCallsInterface = append(toolCallsInterface, toolCall)
				}
			}
		}
	}

	// Second pass: match tool responses to requests and add output
	for _, msg := range history {
		if msg.Content != nil && len(msg.Content) > 0 {
			for _, part := range msg.Content {
				if part.IsToolResponse() {
					toolResp := part.ToolResponse
					// Find the matching tool call and add the output
					if toolCall, exists := toolRequestMap[toolResp.Ref]; exists {
						toolCall["output"] = toolResp.Output
					}
				}
			}
		}
	}

	// Set tool calls array if we have any
	if len(toolCallsInterface) > 0 {
		toolCallsArray = &toolCallsInterface
	}

	return &ExecutionResponse{
		Success:    true,
		Response:   finalResponse,
		ToolCalls:  toolCallsArray,
		Duration:   time.Since(startTime),
		ModelName:  modelName,
		StepsUsed:  len(toolCallsInterface),
		ToolsUsed:  len(toolCallsInterface),
		TokenUsage: tokenUsage,
		App:        app,     // CloudShip data ingestion app classification
		AppType:    appType, // CloudShip data ingestion app_type classification
	}, nil
}

// getAgentPromptPath returns dotprompt file path for an agent using provided environment name
func (e *GenKitExecutor) getAgentPromptPath(agent models.Agent, environmentName string) (string, error) {
	// Use centralized path resolution for container compatibility
	promptPath := config.GetAgentPromptPath(environmentName, agent.Name)

	// Check if file exists
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		return "", fmt.Errorf("dotprompt file not found: %s", promptPath)
	}

	return promptPath, nil
}

// getModelName builds the model name with provider prefix
func (e *GenKitExecutor) getModelName() string {
	cfg, err := config.Load()
	if err != nil {
		return "openai/gpt-5-mini" // fallback
	}

	baseModel := cfg.AIModel
	if baseModel == "" {
		baseModel = "gpt-5-mini"
	}

	switch cfg.AIProvider {
	case "gemini", "googlegenai":
		return fmt.Sprintf("googleai/%s", baseModel)
	case "openai":
		return fmt.Sprintf("openai/%s", baseModel)
	default:
		return fmt.Sprintf("%s/%s", cfg.AIProvider, baseModel)
	}
}
