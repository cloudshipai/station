package dotprompt

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"station/internal/logging"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// ============================================================================
// GENKIT GENERATION AND TOOL CALLING CONTROL
// ============================================================================

// generateWithCustomTurnLimit implements custom turn limiting with final response capability
func (e *GenKitExecutor) generateWithCustomTurnLimit(ctx context.Context, genkitApp *genkit.Genkit, generateOpts []ai.GenerateOption, tracker *ToolCallTracker, maxToolCalls int, modelName string) (*ai.ModelResponse, error) {
	
	// Add detailed debug logging before GenKit Generate call
	if tracker.LogCallback != nil {
		tracker.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "debug",
			"message":   "ABOUT TO CALL genkit.Generate() - this is where conversations often hang",
			"details": map[string]interface{}{
				"model_name":    modelName,
				"genkit_app":    fmt.Sprintf("%T", genkitApp),
				"genkit_app_nil": genkitApp == nil,
				"context_done":  ctx.Err() != nil,
				"opts_count":    len(generateOpts),
				"max_tool_calls": maxToolCalls,
			},
		})
	}
	
	// First attempt: normal generation with tools
	logging.Debug("CRITICAL: About to call genkit.Generate with model=%s, opts=%d", modelName, len(generateOpts))
	
	// Extract and log OpenCode tool usage before Generate call
	e.logOpenCodeToolUsage(generateOpts, tracker, "BEFORE_GENERATE")
	
	// Add aggressive logging around GenKit Generate call
	fmt.Fprintf(os.Stderr, "🔥 CRITICAL: About to call genkit.Generate - checking for OpenCode tools in options\n")
	for i, opt := range generateOpts {
		optStr := fmt.Sprintf("%+v", opt)
		if strings.Contains(optStr, "opencode") || strings.Contains(optStr, "execute") {
			fmt.Fprintf(os.Stderr, "🔥 FOUND OpenCode-related option at index %d: %s\n", i, optStr)
		}
	}
	
	// THE CRITICAL CALL - This is where execution might hang with OpenCode tools
	startGenerate := time.Now()
	response, err := genkit.Generate(ctx, genkitApp, generateOpts...)
	generateDuration := time.Since(startGenerate)
	
	// Immediate post-generate logging
	fmt.Fprintf(os.Stderr, "🔥 GENKIT RETURNED: duration=%v, response=%v, error=%v\n", 
		generateDuration, response != nil, err != nil)
	
	// Extract and log OpenCode tool results after Generate call
	e.logOpenCodeToolResults(response, tracker, "AFTER_GENERATE", generateDuration)
	
	if err != nil {
		logging.Debug("GenKit Generate failed: %v", err)
		
		// Check if this is a tool-related error that should be handled gracefully
		if strings.Contains(err.Error(), "tool") && 
		   (strings.Contains(err.Error(), "failed") || strings.Contains(err.Error(), "error")) {
			// Mark tool failure in tracker
			if tracker != nil {
				tracker.ToolFailures++
				tracker.HasToolFailures = true
				if tracker.LogCallback != nil {
					tracker.LogCallback(map[string]interface{}{
						"timestamp": time.Now().Format(time.RFC3339),
						"level":     "warning",
						"message":   "Tool failure detected - marking execution as failed",
						"details": map[string]interface{}{
							"error": err.Error(),
							"tool_failures": tracker.ToolFailures,
						},
					})
				}
			}
			
			// Create a synthetic response that continues the conversation with tool error
			toolName := extractToolNameFromError(err.Error())
			logging.Debug("Creating synthetic response for tool failure: %s", toolName)
			
			syntheticResponse := &ai.ModelResponse{
				Message: ai.NewTextMessage(ai.RoleModel, fmt.Sprintf(
					"I encountered an error while trying to use the %s tool: %s\n\nLet me continue with the available information and suggest alternative approaches.", 
					toolName, err.Error())),
			}
			
			return syntheticResponse, nil
		}
		
		return nil, fmt.Errorf("generation failed: %w", err)
	}
	
	if response == nil {
		return nil, fmt.Errorf("genkit returned nil response with no error")
	}
	
	logging.Debug("GenKit Generate succeeded in %v", generateDuration)
	
	// Check for tool usage patterns and decide if we should force completion
	if response.Request != nil && response.Request.Messages != nil {
		if tracker != nil {
			// Log pattern analysis
			if tracker.LogCallback != nil {
				tracker.LogCallback(map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
					"level":     "debug",
					"message":   "Analyzing tool call patterns after generation",
					"details": map[string]interface{}{
						"total_calls":    tracker.TotalCalls,
						"messages_count": len(response.Request.Messages),
					},
				})
			}
			
			// Check if we should force completion due to tool calling patterns
			shouldComplete, reason := e.shouldForceCompletion(response.Request.Messages, tracker)
			if shouldComplete {
				logging.Debug("Forcing completion due to tool pattern: %s", reason)
				
				// Log pattern detection
				if tracker.LogCallback != nil {
					tracker.LogCallback(map[string]interface{}{
						"timestamp": time.Now().Format(time.RFC3339),
						"level":     "warning",
						"message":   fmt.Sprintf("Forcing completion: %s", reason),
					})
				}
				
				// Create a final response that synthesizes available information
				finalOpts := make([]ai.GenerateOption, len(generateOpts))
				copy(finalOpts, generateOpts)
				
				// Add a system message to force final response
				forceMessage := ai.NewTextMessage(ai.RoleSystem, 
					"Based on the information you've gathered, please provide a final comprehensive response. Do not use any more tools.")
				finalOpts = append(finalOpts, ai.WithMessages(forceMessage))
				
				finalResponse, finalErr := genkit.Generate(ctx, genkitApp, finalOpts...)
				if finalErr != nil {
					// If final generation fails, return the original response
					logging.Debug("Final generation failed, returning original: %v", finalErr)
					return response, nil
				}
				
				return finalResponse, nil
			}
		}
	}
	
	return response, nil
}

// generateWithToolLimits provides the original tool limiting approach
func (e *GenKitExecutor) generateWithToolLimits(ctx context.Context, genkitApp *genkit.Genkit, generateOpts []ai.GenerateOption, tracker *ToolCallTracker) (*ai.ModelResponse, error) {
	// This method is deprecated in favor of generateWithCustomTurnLimit
	return e.generateWithCustomTurnLimit(ctx, genkitApp, generateOpts, tracker, 25, "gpt-4")
}

// analyzeToolCallPatterns examines conversation messages to detect problematic tool usage
func (e *GenKitExecutor) analyzeToolCallPatterns(messages []*ai.Message, tracker *ToolCallTracker) string {
	if len(messages) < 4 {
		return "insufficient_data"
	}
	
	// Extract recent tool calls (last 6)
	recentTools := e.extractRecentToolCalls(messages, 6)
	if len(recentTools) < 3 {
		return "normal"
	}
	
	// Check for repetitive patterns
	if e.isRepetitivePattern(recentTools) {
		return "repetitive_loop"
	}
	
	// Check for information gathering dominance (>70% of calls are info gathering)
	infoGatheringCount := 0
	for _, tool := range recentTools {
		if e.isInformationGatheringTool(tool) {
			infoGatheringCount++
		}
	}
	
	if float64(infoGatheringCount)/float64(len(recentTools)) > 0.7 {
		return "information_gathering_loop"
	}
	
	return "normal"
}

// shouldForceCompletion determines if conversation should be forced to complete
func (e *GenKitExecutor) shouldForceCompletion(messages []*ai.Message, tracker *ToolCallTracker) (bool, string) {
	if len(messages) < 8 {
		return false, "" // Need sufficient conversation history
	}
	
	// Analyze recent tool call patterns
	pattern := e.analyzeToolCallPatterns(messages, tracker)
	
	if pattern == "repetitive_loop" {
		return true, "Detected repetitive tool calling pattern"
	}
	
	if pattern == "information_gathering_loop" {
		// Get recent tool calls to check for excessive information gathering
		recentToolCalls := e.extractRecentToolCalls(messages, 5)
		if e.isRepetitivePattern(recentToolCalls) {
			return true, "Detected excessive information gathering without synthesis"
		}
		
		// Also check for specific problematic tool patterns
		infoToolCount := 0
		for _, toolCall := range recentToolCalls {
			// Check for OpenCode tools or known problematic patterns
			toolNameLower := strings.ToLower(toolCall)
			if strings.Contains(toolNameLower, "opencode") {
				return true, "OpenCode tool causing execution hang"
			}
			
			// Count information gathering tools
			if e.isInformationGatheringTool(toolCall) {
				infoToolCount++
			}
		}
		
		// If most recent calls are just information gathering, force completion
		if float64(infoToolCount)/float64(len(recentToolCalls)) > 0.8 {
			return true, "Too much information gathering without synthesis"
		}
	}
	
	return false, ""
}

// extractRecentToolCalls gets tool names from recent messages
func (e *GenKitExecutor) extractRecentToolCalls(messages []*ai.Message, lookback int) []string {
	var tools []string
	count := 0
	
	// Work backwards through messages
	for i := len(messages) - 1; i >= 0 && count < lookback; i-- {
		msg := messages[i]
		if msg.Role == ai.RoleModel && len(msg.Content) > 0 {
			for _, part := range msg.Content {
				if part.IsToolRequest() {
					tools = append([]string{part.ToolRequest.Name}, tools...) // Prepend to maintain order
					count++
					if count >= lookback {
						break
					}
				}
			}
		}
	}
	
	return tools
}

// isRepetitivePattern detects if the same tools are being called repeatedly
func (e *GenKitExecutor) isRepetitivePattern(toolCalls []string) bool {
	if len(toolCalls) < 4 {
		return false
	}
	
	// Check for immediate repetition (A, A, A)
	consecutive := 1
	for i := 1; i < len(toolCalls); i++ {
		if toolCalls[i] == toolCalls[i-1] {
			consecutive++
			if consecutive >= 3 {
				return true // Found 3+ consecutive identical calls
			}
		} else {
			consecutive = 1
		}
	}
	
	// Check for alternating patterns (A, B, A, B)
	if len(toolCalls) >= 4 {
		pattern1 := toolCalls[0] + toolCalls[1]
		pattern2 := toolCalls[2] + toolCalls[3]
		if pattern1 == pattern2 {
			return true // Found alternating pattern
		}
	}
	
	return false
}

// isInformationGatheringTool determines if a tool is primarily for information gathering
func (e *GenKitExecutor) isInformationGatheringTool(toolName string) bool {
	infoTools := map[string]bool{
		"read":     true,
		"list":     true,
		"search":   true,
		"get":      true,
		"fetch":    true,
		"find":     true,
		"scan":     true,
		"check":    true,
		"analyze":  true,
		"inspect":  true,
		"show":     true,
		"view":     true,
		"cat":      true,
		"head":     true,
		"tail":     true,
		"grep":     true,
		"ls":       true,
		"dir":      true,
	}
	
	toolNameLower := strings.ToLower(toolName)
	for tool := range infoTools {
		if strings.Contains(toolNameLower, tool) {
			return true
		}
	}
	
	return infoTools[toolName]
}

// extractToolNameFromError attempts to extract tool name from error message
func extractToolNameFromError(errorMsg string) string {
	// Look for common patterns like "tool 'name'" or "function 'name'"
	patterns := []string{
		"tool '", "tool \"", "function '", "function \"", "calling '", "calling \"",
	}
	
	for _, pattern := range patterns {
		if idx := strings.Index(errorMsg, pattern); idx != -1 {
			start := idx + len(pattern)
			end := start
			for end < len(errorMsg) && errorMsg[end] != '\'' && errorMsg[end] != '"' && errorMsg[end] != ' ' {
				end++
			}
			if end > start {
				return errorMsg[start:end]
			}
		}
	}
	
	return "unknown_tool"
}