package debug

import (
	"encoding/json"
	"log"
	"regexp"
	"strconv"
	"strings"
)

// LogEntry represents a structured debug log entry
type LogEntry struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details"`
	Timestamp string                 `json:"timestamp"`
}

// ExtractToolUsageFromDebugLogs parses debug logs to extract tool usage information
// This provides a simple way to get tool usage without complex runtime interception
func ExtractToolUsageFromDebugLogs(debugLogsJSON string) (int, []string) {
	if debugLogsJSON == "" {
		return 0, nil
	}

	// Parse the JSON array of log entries
	var logEntries []LogEntry
	if err := json.Unmarshal([]byte(debugLogsJSON), &logEntries); err != nil {
		log.Printf("Failed to parse debug logs JSON: %v", err)
		return 0, nil
	}

	toolCount := 0
	var toolNames []string

	// Look for tool usage patterns in the log messages
	toolCallPattern := regexp.MustCompile(`Model requested (\d+) tool calls`)
	
	for _, entry := range logEntries {
		// Check for "Model requested X tool calls" pattern
		if matches := toolCallPattern.FindStringSubmatch(entry.Message); len(matches) > 1 {
			if count, err := strconv.Atoi(matches[1]); err == nil {
				toolCount += count
			}
		}
		
		// Extract tool names from middleware messages
		if strings.Contains(entry.Message, "STATION-MIDDLEWARE: Request has") {
			if entry.Details != nil {
				if toolNamesRaw, exists := entry.Details["tool_names"]; exists {
					if toolNamesSlice, ok := toolNamesRaw.([]interface{}); ok {
						for _, toolNameRaw := range toolNamesSlice {
							if toolName, ok := toolNameRaw.(string); ok {
								// Only add unique tool names
								found := false
								for _, existing := range toolNames {
									if existing == toolName {
										found = true
										break
									}
								}
								if !found {
									toolNames = append(toolNames, toolName)
								}
							}
						}
					}
				}
			}
		}
	}

	return toolCount, toolNames
}

// ToolCall represents a single tool execution with input/output
type ToolCall struct {
	ToolName  string                 `json:"tool_name"`
	Input     map[string]interface{} `json:"input"`
	Output    string                 `json:"output"`
	Timestamp string                 `json:"timestamp"`
}

// ExtractToolCallsFromDebugLogs parses debug logs to extract detailed tool call information
func ExtractToolCallsFromDebugLogs(debugLogsJSON string) ([]ToolCall, error) {
	if debugLogsJSON == "" {
		return nil, nil
	}

	var logEntries []LogEntry
	if err := json.Unmarshal([]byte(debugLogsJSON), &logEntries); err != nil {
		return nil, err
	}

	var toolCalls []ToolCall
	toolCallMap := make(map[string]*ToolCall) // Track pending tool calls by tool name

	for _, entry := range logEntries {
		// Look for "Calling MCP tool" pattern
		if strings.Contains(entry.Message, "Calling MCP tool") {
			if entry.Details != nil {
				if toolName, exists := entry.Details["tool"]; exists {
					if toolNameStr, ok := toolName.(string); ok {
						toolCall := &ToolCall{
							ToolName:  toolNameStr,
							Input:     make(map[string]interface{}),
							Timestamp: entry.Timestamp,
						}
						
						// Extract args if available
						if args, exists := entry.Details["args"]; exists {
							if argsMap, ok := args.(map[string]interface{}); ok {
								toolCall.Input = argsMap
							}
						}
						
						toolCallMap[toolNameStr] = toolCall
					}
				}
			}
		}
		
		// Look for "MCP tool call succeeded" pattern
		if strings.Contains(entry.Message, "MCP tool call succeeded") {
			if entry.Details != nil {
				if toolName, exists := entry.Details["tool"]; exists {
					if toolNameStr, ok := toolName.(string); ok {
						if pendingCall, exists := toolCallMap[toolNameStr]; exists {
							// Set the output result
							if result, exists := entry.Details["result"]; exists {
								if resultStr, ok := result.(string); ok {
									pendingCall.Output = resultStr
								} else {
									// Handle non-string results by converting to JSON
									if resultBytes, err := json.Marshal(result); err == nil {
										pendingCall.Output = string(resultBytes)
									}
								}
							}
							
							// Move completed call to results and remove from pending
							toolCalls = append(toolCalls, *pendingCall)
							delete(toolCallMap, toolNameStr)
						}
					}
				}
			}
		}
	}

	return toolCalls, nil
}

// Simple validation helper
func IsValidToolCount(count int) bool {
	return count >= 0 && count <= 100 // Reasonable bounds for tool usage
}