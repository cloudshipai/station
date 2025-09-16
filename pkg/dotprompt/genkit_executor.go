package dotprompt

import (
	"context"
	"fmt"
	"os"
	"time"
	
	"station/internal/config"
	"station/pkg/models"
	
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// ToolCallTracker monitors tool usage to prevent obsessive calling loops
type ToolCallTracker struct {
	TotalCalls          int
	ConsecutiveSameTool map[string]int
	LastToolUsed        string
	MaxToolCalls        int
	MaxConsecutive      int
	LogCallback         func(map[string]interface{})
	ToolFailures        int // Track number of tool failures
	HasToolFailures     bool // Track if any tool failures occurred
}

// GenKitExecutor handles agent execution using GenKit's dotprompt.Execute()
type GenKitExecutor struct {
	logCallback func(map[string]interface{})
}

// NewGenKitExecutor creates a new GenKit-based dotprompt executor
func NewGenKitExecutor() *GenKitExecutor {
	return &GenKitExecutor{}
}

// ExecuteAgent executes an agent using dotprompt.Execute() with registered MCP tools
func (e *GenKitExecutor) ExecuteAgent(agent models.Agent, agentTools []*models.AgentToolWithDetails, genkitApp *genkit.Genkit, mcpTools []ai.ToolRef, task string, logCallback func(map[string]interface{}), environmentName string) (*ExecutionResponse, error) {
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
	
	resp, err := agentPrompt.Execute(ctx,
		ai.WithInput(map[string]any{"userInput": task}),
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
	}, nil
}

// getAgentPromptPath returns dotprompt file path for an agent using provided environment name
func (e *GenKitExecutor) getAgentPromptPath(agent models.Agent, environmentName string) (string, error) {
	// Build dotprompt file path using provided environment name
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	
	promptPath := fmt.Sprintf("%s/.config/station/environments/%s/agents/%s.prompt", homeDir, environmentName, agent.Name)
	
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

