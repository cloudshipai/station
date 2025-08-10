package services

import (
	"fmt"
	"strings"

	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
)

// ResponseProcessor handles parsing and processing of AI model responses
type ResponseProcessor struct{}

// NewResponseProcessor creates a new response processor
func NewResponseProcessor() *ResponseProcessor {
	return &ResponseProcessor{}
}

// ExtractToolCallsFromResponse extracts actual tool calls from GenKit response messages
func (rp *ResponseProcessor) ExtractToolCallsFromResponse(response *ai.ModelResponse, modelName string) []interface{} {
	var toolCalls []interface{}
	
	// First check for tool requests in the response itself
	if response != nil {
		toolRequests := response.ToolRequests()
		for i, toolReq := range toolRequests {
			toolCall := map[string]interface{}{
				"step":           i + 1,
				"type":           "tool_call",
				"tool_name":      toolReq.Name,
				"tool_input":     toolReq.Input,
				"model_name":     modelName,
				"raw_tool_call":  toolReq,
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}
	
	// Fallback to checking request messages (old method)
	if len(toolCalls) == 0 && response != nil && response.Request != nil && len(response.Request.Messages) > 0 {
		return rp.extractFromRequestMessages(response, modelName)
	}
	
	return toolCalls
}

// extractFromRequestMessages extracts tool calls from request messages (fallback)
func (rp *ResponseProcessor) extractFromRequestMessages(response *ai.ModelResponse, modelName string) []interface{} {
	var toolCalls []interface{}
	
	stepCounter := 0
	for _, msg := range response.Request.Messages {
		if msg.Role == ai.RoleModel && len(msg.Content) > 0 {
			stepCounter++
			
			for _, part := range msg.Content {
				if part.ToolRequest != nil {
					toolCall := map[string]interface{}{
						"step":           stepCounter,
						"type":           "tool_call",
						"tool_name":      part.ToolRequest.Name,
						"tool_input":     part.ToolRequest.Input,
						"model_name":     modelName,
						"raw_tool_call":  part.ToolRequest,
					}
					toolCalls = append(toolCalls, toolCall)
				}
			}
		}
	}
	
	return toolCalls
}

// BuildExecutionStepsFromCapturedCalls builds execution steps from middleware-captured tool calls
func (rp *ResponseProcessor) BuildExecutionStepsFromCapturedCalls(capturedToolCalls []map[string]interface{}, response *ai.ModelResponse, agent *models.Agent, modelName string) []interface{} {
	var steps []interface{}
	
	// Create step from captured tool calls
	if len(capturedToolCalls) > 0 {
		step := map[string]interface{}{
			"step":              1,
			"role":              "model",
			"agent_id":          agent.ID,
			"agent_name":        agent.Name,
			"model_name":        modelName,
			"tool_calls_count":  len(capturedToolCalls),
			"content":           fmt.Sprintf("Used %d tools", len(capturedToolCalls)),
			"content_length":    len(fmt.Sprintf("Used %d tools", len(capturedToolCalls))),
		}
		steps = append(steps, step)
	}
	
	// Add final response step
	if response != nil {
		responseText := response.Text()
		finalStep := map[string]interface{}{
			"step":              len(steps) + 1,
			"role":              "model",
			"agent_id":          agent.ID,
			"agent_name":        agent.Name,
			"model_name":        modelName,
			"tool_calls_count":  0,
			"content":           rp.TruncateContent(responseText, 2000),
			"content_length":    len(responseText),
		}
		steps = append(steps, finalStep)
	}
	
	return steps
}

// BuildExecutionStepsFromResponse builds execution steps from GenKit response for detailed logging
func (rp *ResponseProcessor) BuildExecutionStepsFromResponse(response *ai.ModelResponse, agent *models.Agent, modelName string, toolsAvailable int) []interface{} {
	var steps []interface{}
	
	if response.Request == nil || len(response.Request.Messages) == 0 {
		return steps
	}
	
	stepCounter := 0
	for _, msg := range response.Request.Messages {
		stepCounter++
		
		step := map[string]interface{}{
			"step":             stepCounter,
			"role":             string(msg.Role),
			"agent_id":         agent.ID,
			"agent_name":       agent.Name,
			"model_name":       modelName,
			"tools_available":  toolsAvailable,
		}
		
		if len(msg.Content) > 0 {
			// Extract and truncate message content for logging
			content := rp.ExtractMessageContent(msg)
			step["content"] = rp.TruncateContent(content, 2000)
			step["content_length"] = len(content)
		}
		
		// Count tool calls in this step
		toolCallCount := 0
		for _, part := range msg.Content {
			if part.ToolRequest != nil {
				toolCallCount++
			}
		}
		step["tool_calls_count"] = toolCallCount
		
		steps = append(steps, step)
	}
	
	return steps
}

// ExtractMessageContent safely extracts content from a message part
func (rp *ResponseProcessor) ExtractMessageContent(msg *ai.Message) string {
	var contentParts []string
	
	for _, part := range msg.Content {
		if part.Text != "" {
			contentParts = append(contentParts, part.Text)
		} else if part.ToolRequest != nil {
			toolCallStr := fmt.Sprintf("[TOOL_CALL: %s]", part.ToolRequest.Name)
			contentParts = append(contentParts, toolCallStr)
		} else if part.ToolResponse != nil {
			toolRespStr := fmt.Sprintf("[TOOL_RESPONSE: %s]", rp.TruncateContent(fmt.Sprintf("%v", part.ToolResponse.Output), 100))
			contentParts = append(contentParts, toolRespStr)
		}
	}
	
	return strings.Join(contentParts, " ")
}

// AddToolOutputsToCapturedCalls adds tool outputs to captured tool calls for complete logging
func (rp *ResponseProcessor) AddToolOutputsToCapturedCalls(capturedToolCalls []map[string]interface{}, response *ai.ModelResponse) {
	if response.Request == nil || len(response.Request.Messages) == 0 {
		return
	}
	
	toolCallIndex := 0
	for _, msg := range response.Request.Messages {
		if msg.Role == ai.RoleModel {
			for _, part := range msg.Content {
				if part.ToolRequest != nil {
					if toolCallIndex < len(capturedToolCalls) {
						// Try to find corresponding tool response in subsequent messages
						for _, laterMsg := range response.Request.Messages {
							if laterMsg.Role == ai.RoleTool {
								for _, laterPart := range laterMsg.Content {
									if laterPart.ToolResponse != nil {
										// Match by tool name or request structure
										if rp.isMatchingToolResponse(part.ToolRequest, laterPart.ToolResponse) {
											capturedToolCalls[toolCallIndex]["tool_output"] = laterPart.ToolResponse.Output
											capturedToolCalls[toolCallIndex]["tool_response_raw"] = laterPart.ToolResponse
											break
										}
									}
								}
							}
						}
					}
					toolCallIndex++
				}
			}
		}
	}
}

// TruncateContent truncates content to a maximum length for logging
func (rp *ResponseProcessor) TruncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "...[truncated]"
}

// isMatchingToolResponse determines if a tool response matches a tool request
func (rp *ResponseProcessor) isMatchingToolResponse(request *ai.ToolRequest, response *ai.ToolResponse) bool {
	// Simple heuristic: if names match or if this is the only unmatched response
	return request.Name == response.Name
}