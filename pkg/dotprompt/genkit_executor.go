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
func (e *GenKitExecutor) ExecuteAgent(agent models.Agent, agentTools []*models.AgentToolWithDetails, genkitApp *genkit.Genkit, mcpTools []ai.ToolRef, task string, logCallback func(map[string]interface{}), environmentName string, userVariables map[string]interface{}) (*ExecutionResponse, error) {
	startTime := time.Now()
	e.logCallback = logCallback

	// Get agent's dotprompt file path using provided environment name
	promptPath, err := e.getAgentPromptPath(agent, environmentName)
	if err != nil {
		return &ExecutionResponse{
			Success:  false,
			Response: "",
			Duration: time.Since(startTime),
			Error:    fmt.Sprintf("failed to get agent prompt path: %v", err),
		}, nil
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

	// Load dotprompt file
	agentPrompt := genkit.LoadPrompt(genkitApp, promptPath, "")
	if agentPrompt == nil {
		return &ExecutionResponse{
			Success:  false,
			Response: "",
			Duration: time.Since(startTime),
			Error:    fmt.Sprintf("failed to load prompt from: %s", promptPath),
		}, nil
	}

	// Register filtered MCP tools so dotprompt.Execute() can find them
	for _, toolRef := range mcpTools {
		if tool, ok := toolRef.(ai.Tool); ok {
			genkit.RegisterAction(genkitApp, tool)
		}
	}

	// Get model configuration
	modelName := e.getModelName()

	// Execute with dotprompt.Execute()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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

	resp, err := agentPrompt.Execute(ctx,
		ai.WithInput(inputMap),
		ai.WithMaxTurns(maxTurns),
		ai.WithModelName(modelName))

	if err != nil {
		return &ExecutionResponse{
			Success:  false,
			Response: "",
			Duration: time.Since(startTime),
			Error:    fmt.Sprintf("dotprompt.Execute() failed: %v", err),
		}, nil
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

	// Extract tool calls
	toolRequests := resp.ToolRequests()
	var toolCallsArray *models.JSONArray
	if len(toolRequests) > 0 {
		toolCallsInterface := make(models.JSONArray, len(toolRequests))
		for i, req := range toolRequests {
			toolCallsInterface[i] = map[string]interface{}{
				"tool_name":    req.Name,
				"parameters":   req.Input,
				"tool_call_id": req.Ref,
			}
		}
		toolCallsArray = &toolCallsInterface
	}

	return &ExecutionResponse{
		Success:    true,
		Response:   finalResponse,
		ToolCalls:  toolCallsArray,
		Duration:   time.Since(startTime),
		ModelName:  modelName,
		StepsUsed:  len(resp.ToolRequests()),
		ToolsUsed:  len(resp.ToolRequests()),
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
		return "openai/gpt-4o-mini" // fallback
	}

	baseModel := cfg.AIModel
	if baseModel == "" {
		baseModel = "gpt-4o-mini"
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
