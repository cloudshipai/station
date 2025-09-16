package mcp

import (
	"fmt"
	"time"

	"station/internal/services"
	"station/pkg/models"
	"station/pkg/types"
)

// convertToLighthouseRun converts MCP execution result to Lighthouse format
func convertToLighthouseRun(agent *models.Agent, task string, runID int64, result *services.AgentExecutionResult) *types.AgentRun {
	status := "completed"
	if !result.Success {
		status = "failed"
	}

	// Calculate times based on result duration
	completedAt := time.Now()
	startedAt := completedAt.Add(-result.Duration)

	return &types.AgentRun{
		ID:             fmt.Sprintf("run_%d", runID),
		AgentID:        fmt.Sprintf("agent_%d", agent.ID),
		AgentName:      agent.Name,
		Task:           task,
		Response:       result.Response,
		Status:         status,
		DurationMs:     result.Duration.Milliseconds(),
		ModelName:      result.ModelName,
		StartedAt:      startedAt,
		CompletedAt:    completedAt,
		ToolCalls:      convertMCPToolCalls(result.ToolCalls),
		ExecutionSteps: convertMCPExecutionSteps(result.ExecutionSteps),
		TokenUsage:     convertMCPTokenUsage(result.TokenUsage),
		OutputSchema: func() string {
			if agent.OutputSchema != nil {
				return *agent.OutputSchema
			}
			return ""
		}(),
		OutputSchemaPreset: func() string {
			if agent.OutputSchemaPreset != nil {
				return *agent.OutputSchemaPreset
			}
			return ""
		}(),
		Metadata: map[string]string{
			"source": "mcp",
			"mode":   "stdio",
		},
	}
}

// convertMCPToolCalls converts Station tool calls to Lighthouse format
func convertMCPToolCalls(toolCalls *models.JSONArray) []types.ToolCall {
	if toolCalls == nil {
		return nil
	}

	var lighthouseCalls []types.ToolCall
	for _, call := range *toolCalls {
		if callMap, ok := call.(map[string]interface{}); ok {
			toolCall := types.ToolCall{
				ToolName:   getStringFromMap(callMap, "tool_name"),
				Parameters: callMap["parameters"],
				Result:     getStringFromMap(callMap, "result"),
				DurationMs: int64(getIntFromMap(callMap, "duration_ms")),
				Success:    getBoolFromMap(callMap, "success"),
				Timestamp:  time.Now(),
			}
			lighthouseCalls = append(lighthouseCalls, toolCall)
		}
	}
	return lighthouseCalls
}

// convertMCPExecutionSteps converts execution steps to Lighthouse format
func convertMCPExecutionSteps(executionSteps *models.JSONArray) []types.ExecutionStep {
	if executionSteps == nil {
		return nil
	}

	var lighthouseSteps []types.ExecutionStep
	for _, step := range *executionSteps {
		if stepMap, ok := step.(map[string]interface{}); ok {
			step := types.ExecutionStep{
				StepNumber:  getIntFromMap(stepMap, "step_number"),
				Description: getStringFromMap(stepMap, "description"),
				Type:        getStringFromMap(stepMap, "type"),
				DurationMs:  int64(getIntFromMap(stepMap, "duration_ms")),
				Timestamp:   time.Now(),
			}
			lighthouseSteps = append(lighthouseSteps, step)
		}
	}
	return lighthouseSteps
}

// convertMCPTokenUsage converts token usage to Lighthouse format
func convertMCPTokenUsage(tokenUsage map[string]interface{}) *types.TokenUsage {
	if tokenUsage == nil {
		return nil
	}

	return &types.TokenUsage{
		PromptTokens:     getIntFromMap(tokenUsage, "input_tokens"),
		CompletionTokens: getIntFromMap(tokenUsage, "output_tokens"),
		TotalTokens:      getIntFromMap(tokenUsage, "total_tokens"),
		CostUSD:          getFloatFromMap(tokenUsage, "cost_usd"),
	}
}

// Helper functions for type conversion
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getIntFromMap(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

func getBoolFromMap(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func getFloatFromMap(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return 0.0
}