package telemetry

import (
	"context"
	"fmt"
	"log"
	"time"

	"station/internal/db/repositories"
	"station/pkg/models"

	"github.com/firebase/genkit/go/core/tracing"
)

// GenkitTelemetryClient captures comprehensive Genkit execution details for agent runs
type GenkitTelemetryClient struct {
	repos     *repositories.Repositories
	runID     int64
	agentID   int64
	agentName string
}

// NewGenkitTelemetryClient creates a new telemetry client for capturing agent run details
func NewGenkitTelemetryClient(repos *repositories.Repositories, runID, agentID int64, agentName string) *GenkitTelemetryClient {
	return &GenkitTelemetryClient{
		repos:     repos,
		runID:     runID,
		agentID:   agentID,
		agentName: agentName,
	}
}

// Save captures comprehensive trace data and stores it in the agent run record
func (c *GenkitTelemetryClient) Save(ctx context.Context, trace *tracing.Data) error {
	if trace == nil {
		return fmt.Errorf("trace data is nil")
	}

	log.Printf("📊 Capturing telemetry for run %d: trace %s with %d spans", c.runID, trace.TraceID, len(trace.Spans))

	// Extract comprehensive execution data
	executionData := c.extractExecutionData(trace)

	// Store the captured data in the agent run record
	return c.updateAgentRunWithTelemetry(ctx, executionData)
}

// ExecutionData represents comprehensive execution details captured from Genkit traces
type ExecutionData struct {
	TraceID         string                   `json:"trace_id"`
	TotalDuration   int64                    `json:"total_duration_ms"`
	StepsTaken      int64                    `json:"steps_taken"`
	ToolCalls       []ToolCallData          `json:"tool_calls"`
	ExecutionSteps  []ExecutionStepData     `json:"execution_steps"`
	TokenUsage      *TokenUsageData         `json:"token_usage,omitempty"`
	ModelInfo       *ModelInfoData          `json:"model_info,omitempty"`
	ErrorInfo       *ErrorInfoData          `json:"error_info,omitempty"`
	PerformanceData *PerformanceData        `json:"performance_data,omitempty"`
}

// ToolCallData represents detailed tool call information
type ToolCallData struct {
	ToolName     string                 `json:"tool_name"`
	StartTime    int64                  `json:"start_time_ms"`
	EndTime      int64                  `json:"end_time_ms"`
	Duration     int64                  `json:"duration_ms"`
	Input        map[string]interface{} `json:"input,omitempty"`
	Output       interface{}            `json:"output,omitempty"`
	Status       string                 `json:"status"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	SpanID       string                 `json:"span_id"`
}

// ExecutionStepData represents detailed execution step information
type ExecutionStepData struct {
	StepName      string                 `json:"step_name"`
	StepType      string                 `json:"step_type"`
	StartTime     int64                  `json:"start_time_ms"`
	EndTime       int64                  `json:"end_time_ms"`
	Duration      int64                  `json:"duration_ms"`
	Status        string                 `json:"status"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Input         interface{}            `json:"input,omitempty"`
	Output        interface{}            `json:"output,omitempty"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	SpanID        string                 `json:"span_id"`
}

// TokenUsageData represents comprehensive token usage information
type TokenUsageData struct {
	InputTokens          int                `json:"input_tokens"`
	OutputTokens         int                `json:"output_tokens"`
	TotalTokens          int                `json:"total_tokens"`
	CachedContentTokens  int                `json:"cached_content_tokens,omitempty"`
	ThoughtsTokens       int                `json:"thoughts_tokens,omitempty"`
	InputCharacters      int                `json:"input_characters,omitempty"`
	OutputCharacters     int                `json:"output_characters,omitempty"`
	InputImages          int                `json:"input_images,omitempty"`
	OutputImages         int                `json:"output_images,omitempty"`
	InputAudioFiles      float64            `json:"input_audio_files,omitempty"`
	OutputAudioFiles     float64            `json:"output_audio_files,omitempty"`
	InputVideos          float64            `json:"input_videos,omitempty"`
	OutputVideos         float64            `json:"output_videos,omitempty"`
	Custom               map[string]float64 `json:"custom,omitempty"`
}

// ModelInfoData represents model configuration and information
type ModelInfoData struct {
	ModelName    string                 `json:"model_name"`
	Provider     string                 `json:"provider"`
	Temperature  float64                `json:"temperature,omitempty"`
	MaxTokens    int                    `json:"max_tokens,omitempty"`
	TopP         float64                `json:"top_p,omitempty"`
	TopK         int                    `json:"top_k,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
}

// ErrorInfoData represents error information
type ErrorInfoData struct {
	ErrorType    string `json:"error_type"`
	ErrorMessage string `json:"error_message"`
	ErrorCode    string `json:"error_code,omitempty"`
	FailedSpanID string `json:"failed_span_id,omitempty"`
}

// PerformanceData represents performance metrics
type PerformanceData struct {
	TotalExecutionTime    int64   `json:"total_execution_time_ms"`
	ModelInferenceTime    int64   `json:"model_inference_time_ms,omitempty"`
	ToolExecutionTime     int64   `json:"tool_execution_time_ms,omitempty"`
	PromptProcessingTime  int64   `json:"prompt_processing_time_ms,omitempty"`
	ResponseGenerationTime int64  `json:"response_generation_time_ms,omitempty"`
	TokensPerSecond       float64 `json:"tokens_per_second,omitempty"`
	AgentLoopMetrics      interface{} `json:"agent_loop_metrics,omitempty"`
}

// extractExecutionData extracts comprehensive execution data from Genkit trace
func (c *GenkitTelemetryClient) extractExecutionData(trace *tracing.Data) *ExecutionData {
	data := &ExecutionData{
		TraceID:        trace.TraceID,
		TotalDuration:  int64(trace.EndTime - trace.StartTime),
		ToolCalls:      []ToolCallData{},
		ExecutionSteps: []ExecutionStepData{},
	}

	var totalTokenUsage *TokenUsageData
	var modelInfo *ModelInfoData
	var errorInfo *ErrorInfoData
	performanceData := &PerformanceData{
		TotalExecutionTime: data.TotalDuration,
	}
	
	// Enhanced metrics for agent loop analysis
	var agentLoopMetrics = struct {
		ConversationTurns      int
		ToolHeavyOperations    int
		TextOnlyResponses      int
		MixedResponses         int
		DominantStrategy       string
		ConversationPhases     map[string]int
		ToolTypesUsed          map[string]int
		TokenEfficiency        float64
	}{
		ConversationPhases: make(map[string]int),
		ToolTypesUsed:      make(map[string]int),
	}

	stepCount := int64(0)
	for _, span := range trace.Spans {
		if span == nil {
			continue
		}

		stepCount++
		startTime := int64(span.StartTime)
		endTime := int64(span.EndTime)
		duration := endTime - startTime

		// Extract tool calls
		if c.isToolCall(span) {
			toolCall := c.extractToolCall(span, startTime, endTime, duration)
			data.ToolCalls = append(data.ToolCalls, toolCall)
			performanceData.ToolExecutionTime += duration
		}

		// Extract execution steps
		executionStep := c.extractExecutionStep(span, startTime, endTime, duration)
		data.ExecutionSteps = append(data.ExecutionSteps, executionStep)

		// Extract token usage from span attributes
		if usage := c.extractTokenUsage(span); usage != nil {
			if totalTokenUsage == nil {
				totalTokenUsage = usage
			} else {
				// Aggregate token usage
				totalTokenUsage.InputTokens += usage.InputTokens
				totalTokenUsage.OutputTokens += usage.OutputTokens
				totalTokenUsage.TotalTokens += usage.TotalTokens
				totalTokenUsage.CachedContentTokens += usage.CachedContentTokens
				totalTokenUsage.ThoughtsTokens += usage.ThoughtsTokens
			}
		}

		// Extract model information
		if model := c.extractModelInfo(span); model != nil {
			modelInfo = model
		}

		// Extract error information (Code 0 is OK, non-zero indicates error)
		if span.Status.Code != 0 {
			errorInfo = &ErrorInfoData{
				ErrorType:    fmt.Sprintf("code_%d", span.Status.Code),
				ErrorMessage: span.Status.Description,
				FailedSpanID: span.SpanID,
			}
		}

		// Update performance metrics
		if c.isModelInference(span) {
			performanceData.ModelInferenceTime += duration
		}
		
		// Extract enhanced agent loop metrics from span attributes
		if span.Attributes != nil {
			// Conversation analysis
			if turns, ok := span.Attributes["conversation.turn"].(float64); ok {
				agentLoopMetrics.ConversationTurns = int(turns)
			}
			
			// Agent behavior patterns
			if behavior, ok := span.Attributes["agent.behavior"].(string); ok {
				switch behavior {
				case "tool_only":
					agentLoopMetrics.ToolHeavyOperations++
				case "text_only":
					agentLoopMetrics.TextOnlyResponses++
				case "tool_and_text":
					agentLoopMetrics.MixedResponses++
				}
			}
			
			// Agent strategy
			if strategy, ok := span.Attributes["agent.strategy"].(string); ok {
				agentLoopMetrics.DominantStrategy = strategy
			}
			
			// Conversation phase tracking
			if phase, ok := span.Attributes["conversation.phase"].(string); ok {
				agentLoopMetrics.ConversationPhases[phase]++
			}
			
			// Tool type analysis
			toolTypes := []string{"read", "write", "search", "analysis", "system"}
			for _, toolType := range toolTypes {
				if count, ok := span.Attributes[fmt.Sprintf("response.tools.%s_operations", toolType)].(float64); ok {
					agentLoopMetrics.ToolTypesUsed[toolType] += int(count)
				}
			}
			
			// Token efficiency
			if efficiency, ok := span.Attributes["response.tokens.efficiency"].(float64); ok {
				agentLoopMetrics.TokenEfficiency = efficiency
			}
		}
	}

	data.StepsTaken = stepCount
	data.TokenUsage = totalTokenUsage
	data.ModelInfo = modelInfo
	data.ErrorInfo = errorInfo
	data.PerformanceData = performanceData

	// Calculate tokens per second if we have the data
	if totalTokenUsage != nil && data.TotalDuration > 0 {
		tokensPerMs := float64(totalTokenUsage.OutputTokens) / float64(data.TotalDuration)
		data.PerformanceData.TokensPerSecond = tokensPerMs * 1000 // Convert to tokens per second
	}

	// Store agent loop metrics in performance data for later use
	data.PerformanceData.AgentLoopMetrics = agentLoopMetrics

	return data
}

// isToolCall determines if a span represents a tool call
func (c *GenkitTelemetryClient) isToolCall(span *tracing.SpanData) bool {
	if span.Attributes == nil {
		return false
	}
	
	// Check for MCP tool indicators
	if spanType, ok := span.Attributes["genkit:type"].(string); ok {
		if spanType == "tool" || spanType == "mcp" {
			return true
		}
	}
	
	// Check for tool name patterns
	if toolName, ok := span.Attributes["genkit:name"].(string); ok {
		// MCP tools often have underscores in their names
		return toolName != "" && (contains(toolName, "tool") || contains(toolName, "mcp") || contains(toolName, "_"))
	}
	
	return false
}

// isModelInference determines if a span represents model inference
func (c *GenkitTelemetryClient) isModelInference(span *tracing.SpanData) bool {
	if span.Attributes == nil {
		return false
	}
	
	if spanType, ok := span.Attributes["genkit:type"].(string); ok {
		return spanType == "generate" || spanType == "model" || spanType == "llm"
	}
	
	return contains(span.DisplayName, "generate") || contains(span.DisplayName, "llm")
}

// extractToolCall extracts detailed tool call information from a span
func (c *GenkitTelemetryClient) extractToolCall(span *tracing.SpanData, startTime, endTime, duration int64) ToolCallData {
	toolCall := ToolCallData{
		ToolName:  span.DisplayName,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  duration,
		Status:    fmt.Sprintf("code_%d", span.Status.Code),
		SpanID:    span.SpanID,
	}

	if span.Status.Description != "" {
		toolCall.ErrorMessage = span.Status.Description
	}

	// Extract tool name from attributes if available
	if span.Attributes != nil {
		if name, ok := span.Attributes["genkit:name"].(string); ok && name != "" {
			toolCall.ToolName = name
		}
		
		// Extract input parameters
		if input, ok := span.Attributes["genkit:input"]; ok {
			if inputMap, ok := input.(map[string]interface{}); ok {
				toolCall.Input = inputMap
			}
		}
		
		// Extract output
		if output, ok := span.Attributes["genkit:output"]; ok {
			toolCall.Output = output
		}
	}

	return toolCall
}

// extractExecutionStep extracts execution step information from a span
func (c *GenkitTelemetryClient) extractExecutionStep(span *tracing.SpanData, startTime, endTime, duration int64) ExecutionStepData {
	step := ExecutionStepData{
		StepName:  span.DisplayName,
		StepType:  "execution",
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  duration,
		Status:    fmt.Sprintf("code_%d", span.Status.Code),
		SpanID:    span.SpanID,
		Metadata:  make(map[string]interface{}),
	}

	if span.Status.Description != "" {
		step.ErrorMessage = span.Status.Description
	}

	// Extract metadata from attributes
	if span.Attributes != nil {
		for key, value := range span.Attributes {
			step.Metadata[key] = value
			
			// Extract specific fields
			switch key {
			case "genkit:type":
				if stepType, ok := value.(string); ok {
					step.StepType = stepType
				}
			case "genkit:input":
				step.Input = value
			case "genkit:output":
				step.Output = value
			}
		}
	}

	return step
}

// extractTokenUsage extracts token usage information from a span
func (c *GenkitTelemetryClient) extractTokenUsage(span *tracing.SpanData) *TokenUsageData {
	if span.Attributes == nil {
		return nil
	}

	usage := &TokenUsageData{}
	found := false

	// Look for usage information in attributes
	if usageAttr, ok := span.Attributes["genkit:usage"]; ok {
		if usageMap, ok := usageAttr.(map[string]interface{}); ok {
			found = true
			if inputTokens, ok := usageMap["inputTokens"].(float64); ok {
				usage.InputTokens = int(inputTokens)
			}
			if outputTokens, ok := usageMap["outputTokens"].(float64); ok {
				usage.OutputTokens = int(outputTokens)
			}
			if totalTokens, ok := usageMap["totalTokens"].(float64); ok {
				usage.TotalTokens = int(totalTokens)
			}
			if cachedTokens, ok := usageMap["cachedContentTokens"].(float64); ok {
				usage.CachedContentTokens = int(cachedTokens)
			}
			if thoughtsTokens, ok := usageMap["thoughtsTokens"].(float64); ok {
				usage.ThoughtsTokens = int(thoughtsTokens)
			}
		}
	}

	if !found {
		return nil
	}
	return usage
}

// extractModelInfo extracts model information from a span
func (c *GenkitTelemetryClient) extractModelInfo(span *tracing.SpanData) *ModelInfoData {
	if span.Attributes == nil {
		return nil
	}

	model := &ModelInfoData{
		Configuration: make(map[string]interface{}),
	}
	found := false

	// Extract model information from attributes
	if modelName, ok := span.Attributes["genkit:model"].(string); ok {
		model.ModelName = modelName
		found = true
	}
	
	if provider, ok := span.Attributes["genkit:provider"].(string); ok {
		model.Provider = provider
		found = true
	}

	if temp, ok := span.Attributes["genkit:temperature"].(float64); ok {
		model.Temperature = temp
		found = true
	}

	if maxTokens, ok := span.Attributes["genkit:maxTokens"].(float64); ok {
		model.MaxTokens = int(maxTokens)
		found = true
	}

	// Copy all genkit configuration to model configuration
	for key, value := range span.Attributes {
		if contains(key, "genkit:") {
			model.Configuration[key] = value
			found = true
		}
	}

	if !found {
		return nil
	}
	return model
}

// updateAgentRunWithTelemetry updates the agent run record with captured telemetry data
func (c *GenkitTelemetryClient) updateAgentRunWithTelemetry(ctx context.Context, data *ExecutionData) error {
	// Convert execution data to interface arrays for JSONArray
	toolCallsArray := make([]interface{}, len(data.ToolCalls))
	for i, toolCall := range data.ToolCalls {
		toolCallsArray[i] = toolCall
	}

	executionStepsArray := make([]interface{}, len(data.ExecutionSteps))
	for i, step := range data.ExecutionSteps {
		executionStepsArray[i] = step
	}

	// Create comprehensive metadata with enhanced agent loop insights
	metadata := map[string]interface{}{
		"trace_id":         data.TraceID,
		"total_duration_ms": data.TotalDuration,
		"telemetry_captured_at": time.Now().Unix(),
		"agent_name":       c.agentName,
	}
	
	// Add agent loop metrics if available
	if data.PerformanceData != nil && data.PerformanceData.AgentLoopMetrics != nil {
		metadata["agent_loop_metrics"] = data.PerformanceData.AgentLoopMetrics
	}

	if data.TokenUsage != nil {
		metadata["token_usage"] = data.TokenUsage
	}
	if data.ModelInfo != nil {
		metadata["model_info"] = data.ModelInfo
	}  
	if data.ErrorInfo != nil {
		metadata["error_info"] = data.ErrorInfo
	}
	if data.PerformanceData != nil {
		metadata["performance_data"] = data.PerformanceData
	}

	// Update the agent run with comprehensive telemetry data
	toolCallsJSONArray := (*models.JSONArray)(&toolCallsArray)
	executionStepsJSONArray := (*models.JSONArray)(&executionStepsArray)
	
	status := "completed"
	if data.ErrorInfo != nil {
		status = "failed"
	}
	
	completedAt := time.Now()
	
	err := c.repos.AgentRuns.UpdateCompletion(
		ctx,
		c.runID,
		"", // final_response will be set separately
		data.StepsTaken,
		toolCallsJSONArray,
		executionStepsJSONArray,
		status,
		&completedAt,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update agent run with telemetry: %w", err)
	}

	log.Printf("📊 Successfully captured comprehensive telemetry for run %d: %d steps, %d tool calls, %d execution steps", 
		c.runID, data.StepsTaken, len(data.ToolCalls), len(data.ExecutionSteps))

	return nil
}

// Helper function for string contains check
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}