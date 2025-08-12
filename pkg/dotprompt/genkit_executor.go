package dotprompt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	
	"github.com/google/dotprompt/go/dotprompt"
	"gopkg.in/yaml.v2"
	
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/models"
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

// ExecuteAgentWithDatabaseConfig executes an agent using hybrid approach: dotprompt rendering + real multi-step execution
func (e *GenKitExecutor) ExecuteAgentWithDatabaseConfig(agent models.Agent, agentTools []*models.AgentToolWithDetails, repos *repositories.Repositories, task string) (*ExecutionResponse, error) {
	startTime := time.Now()
	
	// 1. Get the active model from Station config (takes precedence)  
	modelName := e.getActiveModelFromConfig()
	
	// 2. Check if agent.Prompt contains dotprompt content and render accordingly
	var finalPrompt string
	var promptSource string
	var err error
	
	if e.isDotpromptContent(agent.Prompt) {
		// Use dotprompt rendering for rich templates
		finalPrompt, err = e.RenderDotpromptContent(agent.Prompt, task, agent.Name)
		if err != nil {
			return nil, fmt.Errorf("dotprompt rendering failed: %w", err)
		}
		promptSource = "dotprompt (frontmatter + template)"
	} else {
		// Simple text prompt - use as is (variables already handled by execution engine)
		finalPrompt = agent.Prompt
		promptSource = "simple text prompt"
	}
	
	// 3. Create execution agent with rendered prompt (in-memory only)
	executionAgent := agent
	executionAgent.Prompt = finalPrompt
	
	// 4. Execute using the real execution engine directly (like AgentService does)
	agentService := services.NewAgentService(repos)
	creator := services.NewIntelligentAgentCreator(repos, agentService)
	
	ctx := context.Background()
	result, err := creator.ExecuteAgentViaStdioMCP(ctx, &executionAgent, task, 0)
	if err != nil {
		return &ExecutionResponse{
			Success:   false,
			Response:  "",
			Duration:  time.Since(startTime),
			ModelName: modelName,
			Error:     err.Error(),
		}, nil
	}
	
	// 5. Convert execution result to our response format
	response := &ExecutionResponse{
		Success:   result.Success,
		Response:  result.Response,
		Duration:  result.Duration,
		ModelName: result.ModelName,
		StepsUsed: result.StepsUsed,
		ToolsUsed: result.ToolsUsed,
		Error:     result.Error,
	}
	
	// 6. Add hybrid execution info to response
	hybridInfo := fmt.Sprintf(`
🔄 Hybrid Execution Summary:
- Prompt Source: %s
- Model (Station config): %s
- Tools: %d available (%v)
- Execution: Real multi-step with tool calling

--- Agent Response ---
%s`, 
		promptSource, modelName, len(agentTools), e.extractToolNames(agentTools), result.Response)
	
	response.Response = hybridInfo
	
	return response, nil
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

// RenderDotpromptContent renders a dotprompt template with the given variables
func (e *GenKitExecutor) RenderDotpromptContent(dotpromptContent, task, agentName string) (string, error) {
	// 1. Create dotprompt instance
	dp := dotprompt.NewDotprompt(nil) // Use default options
	
	// 2. Prepare data for rendering
	data := &dotprompt.DataArgument{
		Input: map[string]any{
			"TASK":       task,
			"AGENT_NAME": agentName,
			"ENVIRONMENT": "default", // TODO: get from agent config
		},
		Context: map[string]any{
			"agent_name": agentName,
		},
	}
	
	// 3. Render the template  
	rendered, err := dp.Render(dotpromptContent, data, nil)
	if err != nil {
		return "", fmt.Errorf("failed to render dotprompt: %w", err)
	}
	
	// 4. Convert messages to text (for now, until we implement full ai.Generate)
	var renderedText strings.Builder
	for i, msg := range rendered.Messages {
		if i > 0 {
			renderedText.WriteString("\n\n")
		}
		renderedText.WriteString(fmt.Sprintf("[%s]: ", msg.Role))
		for _, part := range msg.Content {
			if textPart, ok := part.(*dotprompt.TextPart); ok {
				renderedText.WriteString(textPart.Text)
			}
		}
	}
	
	return renderedText.String(), nil
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