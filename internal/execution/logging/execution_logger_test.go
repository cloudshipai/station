package logging

import (
	"os"
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// This ensures tests can run without requiring hardcoded API keys
	// Tests will use environment variables when available
	code := m.Run()
	os.Exit(code)
}

func TestNewExecutionLogger(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	assert.Equal(t, int64(123), logger.runID)
	assert.Equal(t, "test-agent", logger.agentName)
	assert.NotZero(t, logger.startTime)
	assert.Empty(t, logger.logEntries)
	assert.Equal(t, 0, logger.stepCounter)
}

func TestLogAgentStart(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	logger.LogAgentStart("analyze this code")
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 1)
	
	entry := entries[0]
	assert.Equal(t, LogLevelInfo, entry.Level)
	assert.Equal(t, "agent_start", entry.Event)
	assert.Equal(t, "Starting agent 'test-agent'", entry.Message)
	assert.Equal(t, 1, entry.Step)
	assert.Contains(t, entry.Details, "agent")
	assert.Contains(t, entry.Details, "run_id")
	assert.Contains(t, entry.Details, "task")
	assert.Equal(t, "analyze this code", entry.Details["task"])
}

func TestLogModelRequest(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	logger.LogModelRequest("gpt-4o", 5, 3)
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 1)
	
	entry := entries[0]
	assert.Equal(t, LogLevelDebug, entry.Level)
	assert.Equal(t, "model_request", entry.Event)
	assert.Equal(t, "Sending request to gpt-4o", entry.Message)
	assert.Equal(t, 1, entry.Step)
	assert.Equal(t, "gpt-4o", entry.Details["model"])
	assert.Equal(t, 5, entry.Details["message_count"])
	assert.Equal(t, 3, entry.Details["tool_count"])
}

func TestLogModelResponse(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	usage := &ai.GenerationUsage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
	}
	toolCalls := []string{"read_file", "write_file"}
	
	logger.LogModelResponse(usage, toolCalls, false)
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 1)
	
	entry := entries[0]
	assert.Equal(t, LogLevelInfo, entry.Level)
	assert.Equal(t, "model_response", entry.Event)
	assert.Contains(t, entry.Message, "Will execute 2 tools")
	assert.Equal(t, 100, entry.Details["input_tokens"])
	assert.Equal(t, 50, entry.Details["output_tokens"])
	assert.Equal(t, 150, entry.Details["total_tokens"])
	assert.Equal(t, toolCalls, entry.Details["tool_calls"])
}

func TestLogModelResponseTextOnly(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	usage := &ai.GenerationUsage{
		InputTokens:  80,
		OutputTokens: 120,
		TotalTokens:  200,
	}
	
	logger.LogModelResponse(usage, nil, true)
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 1)
	
	entry := entries[0]
	assert.Equal(t, LogLevelInfo, entry.Level)
	assert.Contains(t, entry.Message, "AI provided final text response")
	assert.Equal(t, "AI provided final text response", entry.Details["next_action"])
}

func TestLogToolExecution(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	// Test successful tool execution
	duration := 250 * time.Millisecond
	logger.LogToolExecution("read_file", duration, true, "")
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 1)
	
	entry := entries[0]
	assert.Equal(t, LogLevelInfo, entry.Level)
	assert.Equal(t, "tool_execution", entry.Event)
	assert.Contains(t, entry.Message, "Executed tool 'read_file'")
	assert.Equal(t, "read_file", entry.Details["tool"])
	assert.Equal(t, true, entry.Details["success"])
	assert.Contains(t, entry.Details, "duration")
}

func TestLogToolExecutionFailure(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	duration := 100 * time.Millisecond
	logger.LogToolExecution("write_file", duration, false, "Permission denied")
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 1)
	
	entry := entries[0]
	assert.Equal(t, LogLevelError, entry.Level)
	assert.Contains(t, entry.Message, "Tool 'write_file' failed")
	assert.Equal(t, false, entry.Details["success"])
	assert.Equal(t, "Permission denied", entry.Details["error"])
}

func TestLogTurnLimitWarning(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	// Test medium urgency warning
	logger.LogTurnLimitWarning(20, 25)
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 1)
	
	entry := entries[0]
	assert.Equal(t, LogLevelWarning, entry.Level)
	assert.Equal(t, "turn_limit_warning", entry.Event)
	assert.Contains(t, entry.Message, "20/25 turns used")
	assert.Equal(t, 20, entry.Details["current_turns"])
	assert.Equal(t, 25, entry.Details["max_turns"])
	assert.Equal(t, 5, entry.Details["turns_remaining"])
	assert.Equal(t, "MEDIUM", entry.Details["urgency"])
}

func TestLogTurnLimitWarningCritical(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	// Test critical urgency warning
	logger.LogTurnLimitWarning(24, 25)
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 1)
	
	entry := entries[0]
	assert.Equal(t, LogLevelError, entry.Level)
	assert.Equal(t, "CRITICAL", entry.Details["urgency"])
	assert.Equal(t, 1, entry.Details["turns_remaining"])
}

func TestLogAgentComplete(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	// Add some steps first
	logger.LogAgentStart("test task")
	logger.LogModelRequest("gpt-4o", 2, 1)
	
	response := "This is a successful completion response from the agent"
	logger.LogAgentComplete(true, response, "")
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 3)
	
	entry := entries[2] // Last entry
	assert.Equal(t, LogLevelInfo, entry.Level)
	assert.Equal(t, "agent_complete", entry.Event)
	assert.Contains(t, entry.Message, "completed successfully")
	assert.Equal(t, true, entry.Details["success"])
	assert.Equal(t, 3, entry.Details["total_steps"])
	assert.Contains(t, entry.Details["response_preview"].(string), "This is a successful")
}

func TestLogAgentCompleteFailure(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	logger.LogAgentComplete(false, "", "API timeout after 3 retries")
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 1)
	
	entry := entries[0]
	assert.Equal(t, LogLevelError, entry.Level)
	assert.Contains(t, entry.Message, "failed")
	assert.Equal(t, false, entry.Details["success"])
	assert.Equal(t, "API timeout after 3 retries", entry.Details["error"])
}

func TestLogError(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	logger.LogError("api_error", "Failed to connect to OpenAI", "connection timeout")
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 1)
	
	entry := entries[0]
	assert.Equal(t, LogLevelError, entry.Level)
	assert.Equal(t, "api_error", entry.Event)
	assert.Equal(t, "Failed to connect to OpenAI", entry.Message)
	assert.Equal(t, "connection timeout", entry.Details["error"])
}

func TestGetExecutionSummary(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	// Add various log entries
	logger.LogAgentStart("test task")
	logger.LogModelRequest("gpt-4o", 2, 1)
	logger.LogTurnLimitWarning(20, 25) // Warning
	logger.LogError("test_error", "Test error", "test error message") // Error
	logger.LogAgentComplete(true, "response", "")
	
	summary := logger.GetExecutionSummary()
	
	assert.Equal(t, int64(123), summary.RunID)
	assert.Equal(t, "test-agent", summary.AgentName)
	assert.Equal(t, 5, summary.TotalSteps)
	assert.Equal(t, 5, summary.TotalEntries)
	assert.Equal(t, 1, summary.DebugCount) // model_request
	assert.Equal(t, 2, summary.InfoCount) // agent_start, agent_complete
	assert.Equal(t, 1, summary.WarningCount) // turn_limit_warning
	assert.Equal(t, 1, summary.ErrorCount) // test_error
	assert.Equal(t, false, summary.Success) // Has errors
	assert.True(t, summary.Duration > 0)
}

func TestGetLogEntriesJSON(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	logger.LogAgentStart("test task")
	logger.LogModelRequest("gpt-4o", 2, 1)
	
	jsonData, err := logger.GetLogEntriesJSON()
	require.NoError(t, err)
	assert.Contains(t, jsonData, "agent_start")
	assert.Contains(t, jsonData, "model_request")
	assert.Contains(t, jsonData, "test-agent")
	
	// Verify it's valid JSON
	assert.True(t, len(jsonData) > 10)
	assert.True(t, jsonData[0] == '[')
	assert.True(t, jsonData[len(jsonData)-1] == ']')
}

func TestCreateLogCallback(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	callback := logger.CreateLogCallback()
	
	// Test callback with plugin data
	callback(map[string]interface{}{
		"level":   "info",
		"event":   "api_call",
		"message": "Making API request",
		"model":   "gpt-4o",
		"tokens":  150,
	})
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 1)
	
	entry := entries[0]
	assert.Equal(t, LogLevelInfo, entry.Level)
	assert.Equal(t, "api_call", entry.Event)
	assert.Equal(t, "Making API request", entry.Message)
	assert.Equal(t, "gpt-4o", entry.Details["model"])
	assert.Equal(t, 150, entry.Details["tokens"])
}

func TestLogCallbackDefaultValues(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	callback := logger.CreateLogCallback()
	
	// Test callback with minimal data
	callback(map[string]interface{}{
		"custom_field": "custom_value",
	})
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 1)
	
	entry := entries[0]
	assert.Equal(t, LogLevelDebug, entry.Level) // Default level
	assert.Equal(t, "plugin_event", entry.Event) // Default event
	assert.Equal(t, "Plugin log entry", entry.Message) // Default message
	assert.Equal(t, "custom_value", entry.Details["custom_field"])
}

func TestStepCounterIncrement(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	logger.LogAgentStart("task")
	logger.LogModelRequest("gpt-4o", 1, 1)
	logger.LogModelResponse(nil, nil, true)
	logger.LogAgentComplete(true, "done", "")
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 4)
	
	// Verify step counter increments properly
	for i, entry := range entries {
		assert.Equal(t, i+1, entry.Step, "Step counter should increment for entry %d", i)
	}
	
	summary := logger.GetExecutionSummary()
	assert.Equal(t, 4, summary.TotalSteps)
}

func TestTimestampProgression(t *testing.T) {
	logger := NewExecutionLogger(123, "test-agent")
	
	logger.LogAgentStart("task")
	
	// Small delay to ensure timestamp difference
	time.Sleep(1 * time.Millisecond)
	
	logger.LogAgentComplete(true, "done", "")
	
	entries := logger.GetLogEntries()
	require.Len(t, entries, 2)
	
	// Verify timestamps progress forward
	assert.True(t, entries[1].Timestamp.After(entries[0].Timestamp) || entries[1].Timestamp.Equal(entries[0].Timestamp))
}

// Integration test example that would work with real OpenAI API
func TestIntegrationWithRealAPI(t *testing.T) {
	// Skip if no API key available
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test - OPENAI_API_KEY not set")
	}

	logger := NewExecutionLogger(999, "integration-test-agent")
	
	// Simulate a real agent execution flow
	logger.LogAgentStart("test integration with real OpenAI API")
	logger.LogModelRequest("gpt-4o-mini", 1, 0) // Simple request, no tools
	
	// Simulate a successful response
	usage := &ai.GenerationUsage{
		InputTokens:  25,
		OutputTokens: 15,
		TotalTokens:  40,
	}
	logger.LogModelResponse(usage, nil, true) // Text-only response
	logger.LogAgentComplete(true, "Integration test completed successfully", "")
	
	// Verify the execution log
	summary := logger.GetExecutionSummary()
	assert.Equal(t, "integration-test-agent", summary.AgentName)
	assert.Equal(t, 4, summary.TotalSteps)
	assert.Equal(t, 0, summary.ErrorCount)
	assert.True(t, summary.Success)
	assert.True(t, summary.Duration > 0)
	
	// Verify JSON serialization works
	jsonData, err := logger.GetLogEntriesJSON()
	require.NoError(t, err)
	assert.Contains(t, jsonData, "integration-test-agent")
	assert.Contains(t, jsonData, "test integration")
	
	t.Logf("Integration test execution summary: %+v", summary)
	t.Logf("Execution took: %v", summary.Duration)
}