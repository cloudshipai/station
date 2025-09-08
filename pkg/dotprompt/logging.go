package dotprompt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
)

// ============================================================================
// OPENCODE LOGGING AND DEBUGGING UTILITIES
// ============================================================================

// logOpenCodeToolUsage logs OpenCode tool usage before Generate call
func (e *GenKitExecutor) logOpenCodeToolUsage(generateOpts []ai.GenerateOption, tracker *ToolCallTracker, phase string) {
	if tracker == nil || tracker.LogCallback == nil {
		return
	}

	// Look for OpenCode tools in the options
	hasOpenCodeTool := false
	toolNames := []string{}
	
	for _, opt := range generateOpts {
		optStr := fmt.Sprintf("%+v", opt)
		if strings.Contains(strings.ToLower(optStr), "opencode") || 
		   strings.Contains(strings.ToLower(optStr), "execute") ||
		   strings.Contains(strings.ToLower(optStr), "python") ||
		   strings.Contains(strings.ToLower(optStr), "code") {
			hasOpenCodeTool = true
			// Try to extract tool name from the option string
			if idx := strings.Index(optStr, "Name:"); idx != -1 {
				nameStart := idx + 5
				nameEnd := nameStart
				for nameEnd < len(optStr) && optStr[nameEnd] != ' ' && optStr[nameEnd] != '}' {
					nameEnd++
				}
				if nameEnd > nameStart {
					toolNames = append(toolNames, optStr[nameStart:nameEnd])
				}
			}
		}
	}
	
	if hasOpenCodeTool {
		tracker.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "debug",
			"message":   fmt.Sprintf("OpenCode tool detected in %s phase", phase),
			"details": map[string]interface{}{
				"phase":          phase,
				"has_opencode":   hasOpenCodeTool,
				"tool_names":     toolNames,
				"total_options":  len(generateOpts),
			},
		})
	}
}

// logOpenCodeToolResults logs OpenCode tool results after Generate call
func (e *GenKitExecutor) logOpenCodeToolResults(response *ai.ModelResponse, tracker *ToolCallTracker, phase string, duration time.Duration) {
	if tracker == nil || tracker.LogCallback == nil {
		return
	}
	
	hasOpenCodeResult := false
	toolCallCount := 0
	toolNames := []string{}
	
	if response != nil && response.Request != nil && response.Request.Messages != nil {
		for _, msg := range response.Request.Messages {
			if msg.Role == ai.RoleModel && len(msg.Content) > 0 {
				for _, part := range msg.Content {
					if part.IsToolRequest() {
						toolCallCount++
						toolName := part.ToolRequest.Name
						toolNames = append(toolNames, toolName)
						
						if strings.Contains(strings.ToLower(toolName), "opencode") ||
						   strings.Contains(strings.ToLower(toolName), "execute") ||
						   strings.Contains(strings.ToLower(toolName), "python") ||
						   strings.Contains(strings.ToLower(toolName), "code") {
							hasOpenCodeResult = true
						}
					}
				}
			}
		}
	}
	
	if hasOpenCodeResult || duration > 30*time.Second {
		tracker.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "debug",
			"message":   fmt.Sprintf("OpenCode tool execution completed in %s phase", phase),
			"details": map[string]interface{}{
				"phase":            phase,
				"duration_seconds": duration.Seconds(),
				"has_opencode":     hasOpenCodeResult,
				"tool_call_count":  toolCallCount,
				"tool_names":       toolNames,
				"slow_execution":   duration > 30*time.Second,
			},
		})
	}
}

// writeOpenCodeLogFile writes detailed OpenCode logging to a file for debugging
func (e *GenKitExecutor) writeOpenCodeLogFile(data map[string]interface{}) error {
	// Create logs directory if it doesn't exist
	logsDir := filepath.Join(os.TempDir(), "station_opencode_logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}
	
	// Create log filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("opencode_debug_%s.log", timestamp)
	logPath := filepath.Join(logsDir, filename)
	
	// Convert data to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal log data: %v", err)
	}
	
	// Add timestamp header
	logEntry := fmt.Sprintf("[%s] OpenCode Debug Log\n%s\n\n", 
		time.Now().Format(time.RFC3339), string(jsonData))
	
	// Write to file (append mode)
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer file.Close()
	
	if _, err := file.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to log file: %v", err)
	}
	
	return nil
}