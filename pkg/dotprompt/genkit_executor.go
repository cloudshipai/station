package dotprompt

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"station/internal/logging"
	"station/internal/services"
	"station/pkg/models"
)

// GenkitExecutor handles dotprompt-based agent execution using GenKit Generate
type GenkitExecutor struct {
	genkitProvider *services.GenKitProvider
	mcpManager     *services.MCPConnectionManager
}

// NewGenkitExecutor creates a new GenKit-based dotprompt executor
func NewGenkitExecutor(genkitProvider *services.GenKitProvider, mcpManager *services.MCPConnectionManager) *GenkitExecutor {
	return &GenkitExecutor{
		genkitProvider: genkitProvider,
		mcpManager:     mcpManager,
	}
}

// ExecuteAgentWithDotpromptTemplate executes using dotprompt template rendering approach
func (e *GenkitExecutor) ExecuteAgentWithDotpromptTemplate(ctx context.Context, config DotpromptConfig, template string, request ExecutionRequest, toolMappings []ToolMapping) (*ExecutionResponse, error) {
	startTime := time.Now()
	
	logging.Info("Starting dotprompt template execution for agent: %s", config.Metadata.Name)

	// Get GenKit app instance
	genkitApp, err := e.genkitProvider.GetApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get GenKit app: %w", err)
	}

	// Render template manually using our dotprompt template system
	renderedPrompt, err := e.RenderTemplate(template, request)
	if err != nil {
		return nil, fmt.Errorf("failed to render dotprompt template: %w", err)
	}

	// Resolve tools
	tools, clients, err := e.resolveMCPTools(ctx, toolMappings)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve MCP tools: %w", err)
	}
	defer e.cleanupMCPClients(clients)

	// Build generate options using dotprompt-rendered template
	var genOptions []ai.GenerateOption
	genOptions = append(genOptions, ai.WithModelName(config.Model))
	genOptions = append(genOptions, ai.WithPrompt(renderedPrompt))
	
	if len(tools) > 0 {
		genOptions = append(genOptions, ai.WithTools(tools...))
	}

	// Add generation config from dotprompt config
	if config.Config.Temperature != nil || config.Config.MaxTokens != nil {
		genConfig := &ai.GenerationCommonConfig{}
		if config.Config.Temperature != nil {
			genConfig.Temperature = float64(*config.Config.Temperature)
		}
		if config.Config.MaxTokens != nil {
			genConfig.MaxOutputTokens = *config.Config.MaxTokens
		}
		genOptions = append(genOptions, ai.WithConfig(genConfig))
	}

	// Execute with GenKit Generate
	logging.Info("Executing GenKit Generate with dotprompt template and %d tools", len(tools))
	response, err := genkit.Generate(ctx, genkitApp, genOptions...)
	if err != nil {
		return &ExecutionResponse{
			Success:   false,
			Error:     fmt.Sprintf("Dotprompt template execution failed: %v", err),
			Duration:  time.Since(startTime),
			ModelName: config.Model,
		}, nil
	}

	// Process the response
	return e.processGenerateResponse(response, config, time.Since(startTime))
}

// ExecuteAgentWithGenerate executes an agent using traditional GenKit Generate for comparison
func (e *GenkitExecutor) ExecuteAgentWithGenerate(ctx context.Context, config DotpromptConfig, template string, request ExecutionRequest, toolMappings []ToolMapping) (*ExecutionResponse, error) {
	startTime := time.Now()
	
	logging.Info("Starting GenKit Generate execution for agent: %s", config.Metadata.Name)

	// Get GenKit app instance
	genkitApp, err := e.genkitProvider.GetApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get GenKit app: %w", err)
	}

	// Render template manually
	renderedPrompt, err := e.RenderTemplate(template, request)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	// Resolve tools
	tools, clients, err := e.resolveMCPTools(ctx, toolMappings)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve MCP tools: %w", err)
	}
	defer e.cleanupMCPClients(clients)

	// Build generate options
	var genOptions []ai.GenerateOption
	genOptions = append(genOptions, ai.WithModelName(config.Model))
	genOptions = append(genOptions, ai.WithPrompt(renderedPrompt))
	
	if len(tools) > 0 {
		genOptions = append(genOptions, ai.WithTools(tools...))
	}

	// Add generation config
	if request.Config != nil {
		// Convert our config to GenKit config
		if request.Config.Temperature != nil || request.Config.MaxTokens != nil {
			genConfig := &ai.GenerationCommonConfig{}
			if request.Config.Temperature != nil {
				genConfig.Temperature = float64(*request.Config.Temperature)
			}
			if request.Config.MaxTokens != nil {
				genConfig.MaxOutputTokens = *request.Config.MaxTokens
			}
			genOptions = append(genOptions, ai.WithConfig(genConfig))
		}
	}

	// Execute with GenKit Generate
	logging.Info("Executing GenKit Generate with %d tools", len(tools))
	response, err := genkit.Generate(ctx, genkitApp, genOptions...)
	if err != nil {
		return &ExecutionResponse{
			Success:   false,
			Error:     fmt.Sprintf("GenKit Generate failed: %v", err),
			Duration:  time.Since(startTime),
			ModelName: config.Model,
		}, nil
	}

	// Process the response
	return e.processGenerateResponse(response, config, time.Since(startTime))
}

// Helper methods

func (e *GenkitExecutor) resolveMCPTools(ctx context.Context, toolMappings []ToolMapping) ([]ai.ToolRef, []*interface{}, error) {
	if e.mcpManager == nil {
		logging.Debug("No MCP manager available - executing without tools")
		return []ai.ToolRef{}, []*interface{}{}, nil
	}

	// This would integrate with Station's MCP system to get actual tools
	// For now, return empty - needs full integration with MCPConnectionManager
	logging.Debug("Resolving %d tool mappings", len(toolMappings))
	
	// TODO: Implement actual tool resolution
	// This would:
	// 1. Connect to MCP servers based on toolMappings
	// 2. Get tool implementations
	// 3. Convert to ai.ToolRef instances
	
	return []ai.ToolRef{}, []*interface{}{}, nil
}

func (e *GenkitExecutor) cleanupMCPClients(clients []*interface{}) {
	// Cleanup MCP connections
	if e.mcpManager != nil && len(clients) > 0 {
		logging.Debug("Cleaning up %d MCP connections", len(clients))
		// e.mcpManager.CleanupConnections(clients)
	}
}

// RenderTemplate renders a dotprompt template with handlebars-style syntax
func (e *GenkitExecutor) RenderTemplate(template string, request ExecutionRequest) (string, error) {
	// Simple template rendering (handlebars-style)
	rendered := template
	
	// Replace {{task}} with actual task
	rendered = strings.ReplaceAll(rendered, "{{task}}", request.Task)
	
	// Handle system section
	rendered = strings.ReplaceAll(rendered, "{{#system}}", "")
	rendered = strings.ReplaceAll(rendered, "{{/system}}", "")
	
	// Handle context section
	if request.Context != nil {
		rendered = strings.ReplaceAll(rendered, "{{#if context}}", "")
		rendered = strings.ReplaceAll(rendered, "{{/if}}", "")
		rendered = strings.ReplaceAll(rendered, "{{toJson context}}", fmt.Sprintf("%+v", request.Context))
	} else {
		// Remove context section
		rendered = e.removeConditionalBlock(rendered, "{{#if context}}", "{{/if}}")
	}
	
	// Handle parameters section
	if request.Parameters != nil {
		rendered = strings.ReplaceAll(rendered, "{{#if parameters}}", "")
		rendered = strings.ReplaceAll(rendered, "{{/if}}", "")
		rendered = strings.ReplaceAll(rendered, "{{toJson parameters}}", fmt.Sprintf("%+v", request.Parameters))
	} else {
		// Remove parameters section
		rendered = e.removeConditionalBlock(rendered, "{{#if parameters}}", "{{/if}}")
	}
	
	return rendered, nil
}

func (e *GenkitExecutor) removeConditionalBlock(text, start, end string) string {
	for {
		startIdx := strings.Index(text, start)
		if startIdx == -1 {
			break
		}
		
		endIdx := strings.Index(text[startIdx:], end)
		if endIdx == -1 {
			break
		}
		
		// Remove the entire block
		text = text[:startIdx] + text[startIdx+endIdx+len(end):]
	}
	return text
}


func (e *GenkitExecutor) processGenerateResponse(response *ai.ModelResponse, config DotpromptConfig, duration time.Duration) (*ExecutionResponse, error) {
	logging.Info("Processing GenKit Generate response")
	
	responseText := response.Text()
	
	// Extract tool calls directly from GenKit response object
	var toolCalls []interface{}
	var steps []interface{}
	
	toolRequests := response.ToolRequests()
	for i, toolReq := range toolRequests {
		toolCall := map[string]interface{}{
			"step":         i + 1,
			"type":         "tool_call",
			"tool_name":    toolReq.Name,
			"tool_input":   toolReq.Input,
			"model_name":   config.Model,
			"tool_call_id": toolReq.Ref,
		}
		toolCalls = append(toolCalls, toolCall)
	}
	
	// Build execution steps
	if len(toolCalls) > 0 {
		step := map[string]interface{}{
			"step":             1,
			"type":             "tool_execution",
			"agent_id":         config.Metadata.AgentID,
			"agent_name":       config.Metadata.Name,
			"model_name":       config.Model,
			"tool_calls_count": len(toolCalls),
			"tools_used":       len(toolCalls),
		}
		steps = append(steps, step)
	}
	
	// Add final response step
	if responseText != "" {
		finalStep := map[string]interface{}{
			"step":           len(steps) + 1,
			"type":           "final_response",
			"agent_id":       config.Metadata.AgentID,
			"agent_name":     config.Metadata.Name,
			"model_name":     config.Model,
			"content_length": len(responseText),
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

	result := &ExecutionResponse{
		Success:        true,
		Response:       responseText,
		ToolCalls:      toolCallsJSON,
		ExecutionSteps: executionStepsJSON,
		Duration:       duration,
		ModelName:      config.Model,
		StepsUsed:      len(steps),
		ToolsUsed:      len(toolCalls),
		RawResponse:    response,
	}

	// Extract token usage
	if response.Usage != nil {
		result.TokenUsage = map[string]interface{}{
			"input_tokens":  response.Usage.InputTokens,
			"output_tokens": response.Usage.OutputTokens,
			"total_tokens":  response.Usage.TotalTokens,
		}
	}

	logging.Info("GenKit Generate execution completed: steps=%d, tools=%d, duration=%v", 
		result.StepsUsed, result.ToolsUsed, result.Duration)

	return result, nil
}

func buildInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"task": map[string]interface{}{
			"type":        "string",
			"description": "The task to execute",
		},
		"context": map[string]interface{}{
			"type":                 "object",
			"description":         "Additional context for task execution",
			"additionalProperties": true,
		},
		"parameters": map[string]interface{}{
			"type":                 "object",
			"description":         "Task-specific parameters",
			"additionalProperties": true,
		},
	}
}

func buildOutputSchema(config DotpromptConfig) map[string]interface{} {
	// Use the output schema from config, or build a default one
	if config.Output.Schema != nil {
		return config.Output.Schema
	}
	
	return map[string]interface{}{
		"result": map[string]interface{}{
			"type":        "object",
			"description": "Task execution result",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the task completed successfully",
				},
				"summary": map[string]interface{}{
					"type":        "string",
					"description": "Summary of what was accomplished",
				},
				"data": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": true,
					"description":         "Task-specific result data",
				},
			},
			"required": []string{"success", "summary"},
		},
	}
}