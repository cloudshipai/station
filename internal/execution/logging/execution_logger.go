// Package logging provides centralized execution logging for Station agents
// This layer handles all user-visible execution tracking and debug logging
package logging

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/firebase/genkit/go/ai"
)

// ExecutionLogger provides centralized logging for agent execution
// This is the single source of truth for user-visible execution tracking
type ExecutionLogger struct {
	runID       int64
	agentName   string
	startTime   time.Time
	logEntries  []LogEntry
	stepCounter int
}

// LogEntry represents a single log entry for user visibility
type LogEntry struct {
	Timestamp time.Time                `json:"timestamp"`
	Level     LogLevel                 `json:"level"`
	Step      int                      `json:"step"`
	Event     string                   `json:"event"`
	Message   string                   `json:"message"`
	Details   map[string]interface{}   `json:"details,omitempty"`
	Error     string                   `json:"error,omitempty"`
}

// LogLevel defines the severity of log entries
type LogLevel string

const (
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
)

// NewExecutionLogger creates a new execution logger for an agent run
func NewExecutionLogger(runID int64, agentName string) *ExecutionLogger {
	return &ExecutionLogger{
		runID:       runID,
		agentName:   agentName,
		startTime:   time.Now(),
		logEntries:  make([]LogEntry, 0),
		stepCounter: 0,
	}
}

// LogAgentStart logs the beginning of agent execution
func (l *ExecutionLogger) LogAgentStart(task string) {
	l.stepCounter++
	l.addLogEntry(LogLevelInfo, "agent_start", fmt.Sprintf("Starting agent '%s'", l.agentName), map[string]interface{}{
		"agent":    l.agentName,
		"run_id":   l.runID,
		"task":     task,
		"duration": time.Since(l.startTime).String(),
	})
}

// LogModelRequest logs details about a model API request
func (l *ExecutionLogger) LogModelRequest(modelName string, messageCount, toolCount int) {
	l.stepCounter++
	l.addLogEntry(LogLevelDebug, "model_request", fmt.Sprintf("Sending request to %s", modelName), map[string]interface{}{
		"model":         modelName,
		"message_count": messageCount,
		"tool_count":    toolCount,
		"duration":      time.Since(l.startTime).String(),
	})
}

// LogModelResponse logs the response from the model
func (l *ExecutionLogger) LogModelResponse(usage *ai.GenerationUsage, toolCalls []string, hasText bool) {
	l.stepCounter++
	
	var nextAction string
	if len(toolCalls) > 0 {
		nextAction = fmt.Sprintf("Will execute %d tools: %v", len(toolCalls), toolCalls)
	} else if hasText {
		nextAction = "AI provided final text response"
	} else {
		nextAction = "No response content"
	}

	details := map[string]interface{}{
		"next_action": nextAction,
		"duration":   time.Since(l.startTime).String(),
	}

	if usage != nil {
		details["input_tokens"] = usage.InputTokens
		details["output_tokens"] = usage.OutputTokens  
		details["total_tokens"] = usage.TotalTokens
	}

	if len(toolCalls) > 0 {
		details["tool_calls"] = toolCalls
	}

	l.addLogEntry(LogLevelInfo, "model_response", fmt.Sprintf("Model responded. %s", nextAction), details)
}

// LogToolExecution logs the execution of a tool
func (l *ExecutionLogger) LogToolExecution(toolName string, duration time.Duration, success bool, errorMsg string) {
	l.stepCounter++
	
	level := LogLevelInfo
	message := fmt.Sprintf("Executed tool '%s' in %v", toolName, duration)
	
	details := map[string]interface{}{
		"tool":        toolName,
		"duration":    duration.String(),
		"success":     success,
		"total_time":  time.Since(l.startTime).String(),
	}

	if !success {
		level = LogLevelError
		message = fmt.Sprintf("Tool '%s' failed after %v", toolName, duration)
		details["error"] = errorMsg
	}

	l.addLogEntry(level, "tool_execution", message, details)
}

// LogTurnLimitWarning logs when approaching turn limits
func (l *ExecutionLogger) LogTurnLimitWarning(currentTurns, maxTurns int) {
	l.stepCounter++
	turnsRemaining := maxTurns - currentTurns
	
	level := LogLevelWarning
	if turnsRemaining <= 1 {
		level = LogLevelError
	}

	l.addLogEntry(level, "turn_limit_warning", fmt.Sprintf("Turn limit warning: %d/%d turns used", currentTurns, maxTurns), map[string]interface{}{
		"current_turns":    currentTurns,
		"max_turns":        maxTurns,
		"turns_remaining":  turnsRemaining,
		"urgency":          func() string {
			if turnsRemaining <= 1 { return "CRITICAL" }
			if turnsRemaining <= 3 { return "HIGH" }
			return "MEDIUM"
		}(),
		"duration": time.Since(l.startTime).String(),
	})
}

// LogAgentComplete logs the completion of agent execution
func (l *ExecutionLogger) LogAgentComplete(success bool, response string, errorMsg string) {
	l.stepCounter++
	
	level := LogLevelInfo
	message := fmt.Sprintf("Agent '%s' completed successfully", l.agentName)
	
	details := map[string]interface{}{
		"agent":       l.agentName,
		"success":     success,
		"total_steps": l.stepCounter,
		"duration":    time.Since(l.startTime).String(),
	}

	if response != "" {
		details["response_preview"] = func() string {
			if len(response) > 200 {
				return response[:200] + "..."
			}
			return response
		}()
	}

	if !success {
		level = LogLevelError
		message = fmt.Sprintf("Agent '%s' failed", l.agentName)
		if errorMsg != "" {
			details["error"] = errorMsg
		}
	}

	l.addLogEntry(level, "agent_complete", message, details)
}

// LogError logs an error during execution
func (l *ExecutionLogger) LogError(event, message, errorMsg string) {
	l.stepCounter++
	l.addLogEntry(LogLevelError, event, message, map[string]interface{}{
		"error":    errorMsg,
		"duration": time.Since(l.startTime).String(),
	})
}

// GetLogEntries returns all log entries for this execution
func (l *ExecutionLogger) GetLogEntries() []LogEntry {
	return l.logEntries
}

// GetLogEntriesJSON returns log entries as JSON for database storage
func (l *ExecutionLogger) GetLogEntriesJSON() (string, error) {
	data, err := json.Marshal(l.logEntries)
	if err != nil {
		return "", fmt.Errorf("failed to marshal log entries: %w", err)
	}
	return string(data), nil
}

// GetExecutionSummary returns a summary of the execution for user display
func (l *ExecutionLogger) GetExecutionSummary() ExecutionSummary {
	totalDuration := time.Since(l.startTime)
	
	// Count entries by level
	var debugCount, infoCount, warningCount, errorCount int
	for _, entry := range l.logEntries {
		switch entry.Level {
		case LogLevelDebug:
			debugCount++
		case LogLevelInfo:
			infoCount++
		case LogLevelWarning:
			warningCount++
		case LogLevelError:
			errorCount++
		}
	}

	return ExecutionSummary{
		RunID:         l.runID,
		AgentName:     l.agentName,
		StartTime:     l.startTime,
		Duration:      totalDuration,
		TotalSteps:    l.stepCounter,
		TotalEntries:  len(l.logEntries),
		DebugCount:    debugCount,
		InfoCount:     infoCount,
		WarningCount:  warningCount,
		ErrorCount:    errorCount,
		Success:       errorCount == 0,
	}
}

// ExecutionSummary provides a high-level summary of execution
type ExecutionSummary struct {
	RunID         int64         `json:"run_id"`
	AgentName     string        `json:"agent_name"`
	StartTime     time.Time     `json:"start_time"`
	Duration      time.Duration `json:"duration"`
	TotalSteps    int           `json:"total_steps"`
	TotalEntries  int           `json:"total_entries"`
	DebugCount    int           `json:"debug_count"`
	InfoCount     int           `json:"info_count"`
	WarningCount  int           `json:"warning_count"`
	ErrorCount    int           `json:"error_count"`
	Success       bool          `json:"success"`
}

// addLogEntry is an internal helper to add log entries
func (l *ExecutionLogger) addLogEntry(level LogLevel, event, message string, details map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Step:      l.stepCounter,
		Event:     event,
		Message:   message,
		Details:   details,
	}
	
	l.logEntries = append(l.logEntries, entry)
}

// CreateLogCallback creates a callback function for real-time logging
// This integrates with our OpenAI plugin's LogCallback functionality
func (l *ExecutionLogger) CreateLogCallback() func(map[string]interface{}) {
	return func(logData map[string]interface{}) {
		level := LogLevelDebug
		if levelStr, ok := logData["level"].(string); ok {
			level = LogLevel(levelStr)
		}

		event := "plugin_event"
		if eventStr, ok := logData["event"].(string); ok {
			event = eventStr
		}

		message := "Plugin log entry"
		if msgStr, ok := logData["message"].(string); ok {
			message = msgStr
		}

		// Extract details, removing redundant fields
		details := make(map[string]interface{})
		for k, v := range logData {
			if k != "level" && k != "event" && k != "message" && k != "timestamp" {
				details[k] = v
			}
		}

		l.stepCounter++
		l.addLogEntry(level, event, message, details)
	}
}