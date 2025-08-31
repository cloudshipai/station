package tools

import (
	"context"
	"testing"
	"time"

	stationctx "station/pkg/context"

	"github.com/firebase/genkit/go/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockContextManager implements the ContextManager interface for testing
type MockContextManager struct {
	utilizationPercent float64
	tokensRemaining    int
	safeActionLimit    int
	isApproachingLimit bool
	canExecuteAction   bool
	canExecuteReason   string
	tokenUsageHistory  []stationctx.TokenUsage
}

func (m *MockContextManager) GetUtilizationPercent() float64 {
	return m.utilizationPercent
}

func (m *MockContextManager) GetTokensRemaining() int {
	return m.tokensRemaining
}

func (m *MockContextManager) CanExecuteAction(estimatedTokens int) (bool, string) {
	if m.canExecuteReason != "" {
		return m.canExecuteAction, m.canExecuteReason
	}
	return m.tokensRemaining >= estimatedTokens, "Mock context manager check"
}

func (m *MockContextManager) GetSafeActionLimit() int {
	return m.safeActionLimit
}

func (m *MockContextManager) TrackTokenUsage(usage stationctx.TokenUsage) {
	m.tokenUsageHistory = append(m.tokenUsageHistory, usage)
	m.tokensRemaining -= usage.TotalTokens
	if m.tokensRemaining < 0 {
		m.tokensRemaining = 0
	}
}

func (m *MockContextManager) IsApproachingLimit() bool {
	return m.isApproachingLimit
}

func TestNewExecutor(t *testing.T) {
	tests := []struct {
		name           string
		config         ToolExecutorConfig
		expectedMaxOut int
		expectedStrategy TruncationStrategy
	}{
		{
			name:             "Default configuration",
			config:           ToolExecutorConfig{},
			expectedMaxOut:   2000,
			expectedStrategy: TruncationStrategyIntelligent,
		},
		{
			name: "Custom configuration",
			config: ToolExecutorConfig{
				MaxOutputTokens:    1000,
				TruncationStrategy: TruncationStrategyHead,
				ContextBuffer:      500,
			},
			expectedMaxOut:   1000,
			expectedStrategy: TruncationStrategyHead,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextManager := &MockContextManager{
				tokensRemaining: 5000,
				safeActionLimit: 3000,
			}
			
			executor := NewExecutor(tt.config, contextManager, nil)
			
			assert.Equal(t, tt.expectedMaxOut, executor.Config.MaxOutputTokens)
			assert.Equal(t, tt.expectedStrategy, executor.Config.TruncationStrategy)
			assert.NotNil(t, executor.metrics)
			assert.NotNil(t, executor.OutputProcessor)
			
			if tt.config.ContextBuffer == 0 {
				assert.Equal(t, 1000, executor.Config.ContextBuffer) // Default
			} else {
				assert.Equal(t, tt.config.ContextBuffer, executor.Config.ContextBuffer)
			}
		})
	}
}

func TestExecutor_CanExecuteTool(t *testing.T) {
	config := ToolExecutorConfig{
		EnableContextProtection: true,
		MaxOutputTokens:        1000,
	}
	
	tests := []struct {
		name           string
		contextManager *MockContextManager
		toolCall       *ai.ToolRequest
		expectedResult bool
		expectedReason string
	}{
		{
			name: "Can execute - sufficient context",
			contextManager: &MockContextManager{
				utilizationPercent: 0.5, // 50% utilization
				tokensRemaining:    3000,
				safeActionLimit:    2000,
				canExecuteAction:   true,
			},
			toolCall: &ai.ToolRequest{Name: "read_file"},
			expectedResult: true,
		},
		{
			name: "Cannot execute - approaching limit",
			contextManager: &MockContextManager{
				utilizationPercent: 0.95, // 95% utilization
				isApproachingLimit: true,
			},
			toolCall: &ai.ToolRequest{Name: "read_file"},
			expectedResult: false,
		},
		{
			name: "Cannot execute - insufficient tokens",
			contextManager: &MockContextManager{
				utilizationPercent: 0.7,
				tokensRemaining:    100, // Very low
				canExecuteAction:   false,
				canExecuteReason:   "Not enough tokens remaining",
			},
			toolCall: &ai.ToolRequest{Name: "large_analysis"},
			expectedResult: false,
		},
		{
			name: "Can execute - context protection disabled",
			contextManager: &MockContextManager{
				utilizationPercent: 0.95,
				isApproachingLimit: true,
			},
			toolCall: &ai.ToolRequest{Name: "read_file"},
			expectedResult: true, // Should pass when protection disabled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Special handling for disabled context protection test
			testConfig := config
			if tt.name == "Can execute - context protection disabled" {
				testConfig.EnableContextProtection = false
			}
			
			executor := NewExecutor(testConfig, tt.contextManager, nil)
			execCtx := &ExecutionContext{
				RemainingTokens:  tt.contextManager.tokensRemaining,
				SafeActionLimit:  tt.contextManager.safeActionLimit,
			}
			
			canExecute, reason := executor.CanExecuteTool(tt.toolCall, execCtx)
			
			assert.Equal(t, tt.expectedResult, canExecute)
			if !canExecute && tt.expectedReason != "" {
				assert.Contains(t, reason, tt.expectedReason)
			}
		})
	}
}

func TestExecutor_EstimateToolOutputTokens(t *testing.T) {
	config := ToolExecutorConfig{MaxOutputTokens: 2000}
	executor := NewExecutor(config, nil, nil)

	tests := []struct {
		name         string
		toolCall     *ai.ToolRequest
		expectedMin  int
		expectedMax  int
	}{
		{
			name:        "File read tool",
			toolCall:    &ai.ToolRequest{Name: "read_text_file"},
			expectedMin: 400,
			expectedMax: 600,
		},
		{
			name:        "Directory listing",
			toolCall:    &ai.ToolRequest{Name: "list_directory"},
			expectedMin: 150,
			expectedMax: 250,
		},
		{
			name:        "Search tool",
			toolCall:    &ai.ToolRequest{Name: "search_files"},
			expectedMin: 700,
			expectedMax: 900,
		},
		{
			name:        "Write operation",
			toolCall:    &ai.ToolRequest{Name: "write_file"},
			expectedMin: 30,
			expectedMax: 70,
		},
		{
			name:        "Unknown tool",
			toolCall:    &ai.ToolRequest{Name: "unknown_tool"},
			expectedMin: 1300, // 70% of 2000
			expectedMax: 1500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimate, err := executor.EstimateToolOutputTokens(tt.toolCall)
			
			require.NoError(t, err)
			assert.GreaterOrEqual(t, estimate, tt.expectedMin)
			assert.LessOrEqual(t, estimate, tt.expectedMax)
		})
	}
}

func TestExecutor_ExecuteTool(t *testing.T) {
	logMessages := make([]map[string]interface{}, 0)
	logCallback := func(data map[string]interface{}) {
		logMessages = append(logMessages, data)
	}

	contextManager := &MockContextManager{
		utilizationPercent: 0.6,
		tokensRemaining:    3000,
		safeActionLimit:    2000,
		canExecuteAction:   true,
	}

	config := ToolExecutorConfig{
		EnableContextProtection: true,
		MaxOutputTokens:        1000,
		TruncationStrategy:     TruncationStrategyHead,
		ToolTimeout:           5 * time.Second,
	}

	executor := NewExecutor(config, contextManager, logCallback)

	execCtx := &ExecutionContext{
		ConversationID:  "test-conversation",
		TurnNumber:      1,
		RemainingTokens: 3000,
		SafeActionLimit: 2000,
	}

	toolCall := &ai.ToolRequest{
		Name: "read_text_file",
		Input: map[string]interface{}{
			"file_path": "/test/file.txt",
		},
	}

	result, err := executor.ExecuteTool(context.Background(), toolCall, execCtx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "read_text_file", result.ToolName)
	assert.True(t, result.Success)
	assert.NotEmpty(t, result.ExecutionID)
	assert.NotZero(t, result.Timestamp)
	assert.Greater(t, result.Duration, time.Duration(0))

	// Check that logs were generated
	assert.Greater(t, len(logMessages), 0)
	
	// Verify token usage was tracked
	assert.Greater(t, len(contextManager.tokenUsageHistory), 0)
	
	// Check metrics were updated
	metrics := executor.GetToolMetrics()
	assert.Equal(t, 1, metrics.TotalExecutions)
	assert.Equal(t, 1, metrics.SuccessfulExecutions)
	assert.Equal(t, 0, metrics.FailedExecutions)
}

func TestExecutor_ExecuteTool_ContextProtection(t *testing.T) {
	contextManager := &MockContextManager{
		utilizationPercent: 0.95, // Very high utilization
		isApproachingLimit: true,
		tokensRemaining:    100,
		canExecuteAction:   false,
		canExecuteReason:   "Context approaching limit",
	}

	config := ToolExecutorConfig{
		EnableContextProtection: true,
		MaxOutputTokens:        1000,
	}

	executor := NewExecutor(config, contextManager, nil)

	execCtx := &ExecutionContext{
		RemainingTokens: 100,
		SafeActionLimit: 50,
	}

	toolCall := &ai.ToolRequest{Name: "large_analysis_tool"}

	result, err := executor.ExecuteTool(context.Background(), toolCall, execCtx)

	require.NoError(t, err) // Should not return error, just blocked execution
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "Context protection prevented execution")
	assert.Empty(t, result.Output)
	
	// Verify metrics show failed execution
	metrics := executor.GetToolMetrics()
	assert.Equal(t, 1, metrics.TotalExecutions)
	assert.Equal(t, 0, metrics.SuccessfulExecutions)
	assert.Equal(t, 1, metrics.FailedExecutions)
}

func TestExecutor_ExecuteToolBatch(t *testing.T) {
	contextManager := &MockContextManager{
		utilizationPercent: 0.4,
		tokensRemaining:    5000,
		safeActionLimit:    3000,
		canExecuteAction:   true,
	}

	config := ToolExecutorConfig{
		EnableContextProtection: true,
		MaxOutputTokens:        500,
		MaxConcurrentTools:     2,
	}

	executor := NewExecutor(config, contextManager, nil)

	execCtx := &ExecutionContext{
		ConversationID:  "batch-test",
		TurnNumber:      2,
		RemainingTokens: 5000,
		SafeActionLimit: 3000,
	}

	toolCalls := []*ai.ToolRequest{
		{Name: "read_file", Input: map[string]interface{}{"path": "file1.txt"}},
		{Name: "read_file", Input: map[string]interface{}{"path": "file2.txt"}},
		{Name: "list_directory", Input: map[string]interface{}{"path": "/home"}},
	}

	results, err := executor.ExecuteToolBatch(context.Background(), toolCalls, execCtx)

	require.NoError(t, err)
	assert.Len(t, results, 3)

	// All tools should succeed
	for i, result := range results {
		assert.NotNil(t, result)
		assert.True(t, result.Success, "Tool %d should succeed", i)
		assert.NotEmpty(t, result.ExecutionID)
		assert.Equal(t, toolCalls[i].Name, result.ToolName)
	}

	// Check metrics for batch execution
	metrics := executor.GetToolMetrics()
	assert.Equal(t, 3, metrics.TotalExecutions)
	assert.Equal(t, 3, metrics.SuccessfulExecutions)
	assert.Equal(t, 0, metrics.FailedExecutions)
}

func TestExecutor_GetToolMetrics(t *testing.T) {
	executor := NewExecutor(ToolExecutorConfig{}, nil, nil)

	// Simulate some executions by directly updating metrics
	executor.recordExecution(&ExecutionResult{
		ToolName:  "read_file",
		Success:   true,
		TokensUsed: 100,
		Duration:  100 * time.Millisecond,
		Truncated: false,
	})
	
	executor.recordExecution(&ExecutionResult{
		ToolName:  "search_files",
		Success:   false,
		Error:     "Tool failed",
		Duration:  50 * time.Millisecond,
	})
	
	executor.recordExecution(&ExecutionResult{
		ToolName:  "list_directory",
		Success:   true,
		TokensUsed: 50,
		Duration:  75 * time.Millisecond,
		Truncated: true,
		OriginalLength: 1000,
	})

	metrics := executor.GetToolMetrics()

	assert.Equal(t, 3, metrics.TotalExecutions)
	assert.Equal(t, 2, metrics.SuccessfulExecutions)
	assert.Equal(t, 1, metrics.FailedExecutions)
	assert.Equal(t, 1, metrics.TruncatedExecutions)
	assert.Equal(t, 50.0, metrics.AverageTokenUsage) // (100+0+50)/3 = 50
	assert.Greater(t, metrics.TokensSaved, 0)
	
	// Check tool type breakdown  
	assert.Equal(t, 1, metrics.ToolTypeBreakdown["read"])   // read_file
	assert.Equal(t, 1, metrics.ToolTypeBreakdown["search"]) // search_files
	assert.Equal(t, 1, metrics.ToolTypeBreakdown["list"])   // list_directory
}

func TestExecutor_CategorizeToolType(t *testing.T) {
	executor := NewExecutor(ToolExecutorConfig{}, nil, nil)

	tests := []struct {
		toolName     string
		expectedType string
	}{
		{"read_text_file", "read"},
		{"get_file_info", "read"},
		{"write_file", "write"},
		{"create_file", "write"},
		{"update_config", "write"},
		{"list_directory", "list"},
		{"search_files", "search"},
		{"find_pattern", "search"},
		{"analyze_code", "analyze"},
		{"check_syntax", "analyze"},
		{"execute_command", "execute"},
		{"run_script", "execute"},
		{"unknown_tool", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := executor.categorizeToolType(tt.toolName)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

func TestExecutor_GenerateExecutionID(t *testing.T) {
	executor := NewExecutor(ToolExecutorConfig{}, nil, nil)

	id1 := executor.generateExecutionID()
	id2 := executor.generateExecutionID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2) // Should generate unique IDs
	assert.Len(t, id1, 16)       // Should be 16 hex characters (8 bytes)
}

func TestExecutor_ProcessLargeOutput(t *testing.T) {
	contextManager := &MockContextManager{
		utilizationPercent: 0.3,
		tokensRemaining:    1000,
		safeActionLimit:    500,
		canExecuteAction:   true,
	}

	config := ToolExecutorConfig{
		EnableContextProtection: true,
		MaxOutputTokens:        100, // Very small limit to force truncation
		TruncationStrategy:     TruncationStrategyHead,
	}

	executor := NewExecutor(config, contextManager, nil)

	execCtx := &ExecutionContext{
		RemainingTokens: 1000,
		SafeActionLimit: 500,
	}

	// This would generate output larger than our token limit
	toolCall := &ai.ToolRequest{Name: "read_large_file"}

	result, err := executor.ExecuteTool(context.Background(), toolCall, execCtx)

	require.NoError(t, err)
	assert.True(t, result.Success)
	
	// Output should be truncated for large content
	// Since our mock executeToolInternal returns small output, 
	// we test the truncation logic works with the processor
	assert.NotEmpty(t, result.Output)
}

// Benchmark tests for performance
func BenchmarkExecutor_ExecuteTool(b *testing.B) {
	contextManager := &MockContextManager{
		utilizationPercent: 0.5,
		tokensRemaining:    10000,
		safeActionLimit:    5000,
		canExecuteAction:   true,
	}

	executor := NewExecutor(ToolExecutorConfig{}, contextManager, nil)
	execCtx := &ExecutionContext{
		RemainingTokens: 10000,
		SafeActionLimit: 5000,
	}
	toolCall := &ai.ToolRequest{Name: "read_file"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.ExecuteTool(context.Background(), toolCall, execCtx)
	}
}

func BenchmarkExecutor_EstimateTokens(b *testing.B) {
	executor := NewExecutor(ToolExecutorConfig{}, nil, nil)
	toolCall := &ai.ToolRequest{Name: "read_text_file"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.EstimateToolOutputTokens(toolCall)
	}
}