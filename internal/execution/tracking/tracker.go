// Package tracking provides progressive execution tracking for Station's agent execution
package tracking

import (
	"fmt"
	"time"

	"github.com/firebase/genkit/go/ai"
)

// Tracker provides real-time logging and metrics collection for AI generation
type Tracker struct {
	logCallback func(map[string]interface{})
	startTime   time.Time
	turnCount   int
}

// NewTracker creates a new progress tracker
func NewTracker(logCallback func(map[string]interface{})) *Tracker {
	return &Tracker{
		logCallback: logCallback,
		startTime:   time.Now(),
		turnCount:   0,
	}
}

// LogModelRequest logs details about a model request
func (t *Tracker) LogModelRequest(req *ai.ModelRequest) {
	if t.logCallback == nil || req == nil {
		return
	}
	
	t.turnCount++
	
	// Analyze request complexity
	messageCount := len(req.Messages)
	toolCount := len(req.Tools)
	
	// Estimate request size
	totalChars := 0
	userMessages := 0
	systemMessages := 0
	toolMessages := 0
	
	for _, msg := range req.Messages {
		if msg == nil {
			continue
		}
		
		switch msg.Role {
		case ai.RoleUser:
			userMessages++
		case ai.RoleSystem:
			systemMessages++
		case ai.RoleTool:
			toolMessages++
		}
		
		for _, part := range msg.Content {
			if part != nil && part.IsText() {
				totalChars += len(part.Text)
			}
		}
	}
	
	t.logCallback(map[string]interface{}{
		"event":     "model_request",
		"level":     "debug",
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   fmt.Sprintf("Turn %d: Sending request to model", t.turnCount),
		"details": map[string]interface{}{
			"turn":             t.turnCount,
			"message_count":    messageCount,
			"available_tools":  toolCount,
			"user_messages":    userMessages,
			"system_messages":  systemMessages,
			"tool_messages":    toolMessages,
			"estimated_chars":  totalChars,
			"estimated_tokens": totalChars / 4, // Rough estimation
		},
	})
}

// LogModelResponse logs details about a model response
func (t *Tracker) LogModelResponse(resp *ai.ModelResponse, err error) {
	if t.logCallback == nil {
		return
	}
	
	if err != nil {
		t.logError("model_response_error", err)
		return
	}
	
	if resp == nil {
		t.logCallback(map[string]interface{}{
			"event":     "model_response",
			"level":     "warn",
			"timestamp": time.Now().Format(time.RFC3339),
			"message":   "Received nil model response",
			"turn":      t.turnCount,
		})
		return
	}
	
	// Analyze response
	toolRequests := 0
	responseLength := 0
	
	if resp.Message != nil {
		for _, part := range resp.Message.Content {
			if part == nil {
				continue
			}
			
			if part.IsToolRequest() {
				toolRequests++
			} else if part.IsText() {
				responseLength += len(part.Text)
			}
		}
	}
	
	level := "info"
	message := fmt.Sprintf("Turn %d: Model responded", t.turnCount)
	
	if toolRequests > 0 {
		message = fmt.Sprintf("Turn %d: Model requested %d tool calls", t.turnCount, toolRequests)
		level = "debug"
	}
	
	t.logCallback(map[string]interface{}{
		"event":     "model_response",
		"level":     level,
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   message,
		"details": map[string]interface{}{
			"turn":              t.turnCount,
			"tool_requests":     toolRequests,
			"response_length":   responseLength,
			"finish_reason":     resp.FinishReason,
			"has_finish_message": resp.FinishMessage != "",
		},
	})
}

// LogToolCall logs when a tool is called
func (t *Tracker) LogToolCall(toolName string, input interface{}) {
	if t.logCallback == nil {
		return
	}
	
	// Analyze input size
	inputSize := 0
	inputType := "unknown"
	
	if input != nil {
		inputType = fmt.Sprintf("%T", input)
		switch v := input.(type) {
		case string:
			inputSize = len(v)
		case []byte:
			inputSize = len(v)
		case map[string]interface{}:
			inputSize = len(fmt.Sprintf("%+v", v)) // Rough estimate
		default:
			inputSize = 100 // Conservative estimate
		}
	}
	
	t.logCallback(map[string]interface{}{
		"event":     "tool_call",
		"level":     "info",
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   fmt.Sprintf("Executing tool: %s", toolName),
		"details": map[string]interface{}{
			"tool_name":   toolName,
			"input_type":  inputType,
			"input_size":  inputSize,
			"turn":        t.turnCount,
		},
	})
}

// LogToolResult logs the result of a tool execution
func (t *Tracker) LogToolResult(toolName string, output interface{}, err error, duration time.Duration) {
	if t.logCallback == nil {
		return
	}
	
	if err != nil {
		t.logCallback(map[string]interface{}{
			"event":     "tool_error",
			"level":     "error",
			"timestamp": time.Now().Format(time.RFC3339),
			"message":   fmt.Sprintf("Tool %s failed: %v", toolName, err),
			"details": map[string]interface{}{
				"tool_name": toolName,
				"error":     err.Error(),
				"duration":  duration.String(),
				"turn":      t.turnCount,
			},
		})
		return
	}
	
	// Analyze output
	outputSize := 0
	outputType := "nil"
	
	if output != nil {
		outputType = fmt.Sprintf("%T", output)
		switch v := output.(type) {
		case string:
			outputSize = len(v)
		case []byte:
			outputSize = len(v)
		default:
			outputSize = len(fmt.Sprintf("%+v", v))
		}
	}
	
	level := "info"
	if outputSize > 10000 { // Large output
		level = "warn"
	}
	
	t.logCallback(map[string]interface{}{
		"event":     "tool_result",
		"level":     level,
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   fmt.Sprintf("Tool %s completed in %v", toolName, duration),
		"details": map[string]interface{}{
			"tool_name":    toolName,
			"output_type":  outputType,
			"output_size":  outputSize,
			"duration":     duration.String(),
			"duration_ms":  duration.Milliseconds(),
			"turn":         t.turnCount,
			"large_output": outputSize > 10000,
		},
	})
}

// LogContextProtection logs when context protection is applied
func (t *Tracker) LogContextProtection(action string, details map[string]interface{}) {
	if t.logCallback == nil {
		return
	}
	
	t.logCallback(map[string]interface{}{
		"event":     "context_protection",
		"level":     "info",
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   fmt.Sprintf("Context protection applied: %s", action),
		"details":   details,
	})
}

// LogConversationSummary logs a summary of the entire conversation
func (t *Tracker) LogConversationSummary(totalTurns int, success bool, duration time.Duration) {
	if t.logCallback == nil {
		return
	}
	
	level := "info"
	status := "completed"
	if !success {
		level = "error"
		status = "failed"
	}
	
	t.logCallback(map[string]interface{}{
		"event":     "conversation_summary",
		"level":     level,
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   fmt.Sprintf("Conversation %s after %d turns in %v", status, totalTurns, duration),
		"details": map[string]interface{}{
			"total_turns":     totalTurns,
			"success":         success,
			"duration":        duration.String(),
			"duration_ms":     duration.Milliseconds(),
			"avg_turn_time":   duration.Milliseconds() / int64(max(totalTurns, 1)),
		},
	})
}

// logError is a helper to log errors with consistent format
func (t *Tracker) logError(event string, err error) {
	if t.logCallback == nil || err == nil {
		return
	}
	
	t.logCallback(map[string]interface{}{
		"event":     event,
		"level":     "error",
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   fmt.Sprintf("Error occurred: %v", err),
		"details": map[string]interface{}{
			"error": err.Error(),
			"turn":  t.turnCount,
		},
	})
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}