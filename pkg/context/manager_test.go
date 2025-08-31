package context

import (
	"strings"
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name          string
		modelName     string
		expectedLimit int
	}{
		{
			name:          "GPT-4o model",
			modelName:     "gpt-4o",
			expectedLimit: 128000,
		},
		{
			name:          "GPT-4 model",
			modelName:     "gpt-4",
			expectedLimit: 8192,
		},
		{
			name:          "GPT-3.5-turbo model", 
			modelName:     "gpt-3.5-turbo",
			expectedLimit: 16385,
		},
		{
			name:          "Claude-3.5-sonnet model",
			modelName:     "claude-3-5-sonnet",
			expectedLimit: 200000,
		},
		{
			name:          "Gemini-1.5 model",
			modelName:     "gemini-1.5-pro",
			expectedLimit: 1000000,
		},
		{
			name:          "Unknown model",
			modelName:     "unknown-model",
			expectedLimit: 4096,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(tt.modelName, nil)
			
			assert.Equal(t, tt.expectedLimit, manager.ModelContextLimit)
			assert.Equal(t, tt.modelName, manager.ModelName)
			assert.Equal(t, 0.9, manager.ThresholdPercent)
			assert.Equal(t, 4000, manager.SummarizationBuffer)
			assert.Equal(t, 0, manager.CurrentTokens)
		})
	}
}

func TestManager_TrackTokenUsage(t *testing.T) {
	logCalled := false
	logCallback := func(data map[string]interface{}) {
		logCalled = true
		assert.Equal(t, "Context usage updated", data["message"])
		details, ok := data["details"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 1500, details["total_tokens"])
		assert.Equal(t, 1000, details["input_tokens"])
		assert.Equal(t, 500, details["output_tokens"])
	}
	
	manager := NewManager("gpt-4o", logCallback)
	
	usage := TokenUsage{
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
	}
	
	manager.TrackTokenUsage(usage)
	
	assert.Equal(t, 1000, manager.InputTokens)
	assert.Equal(t, 500, manager.OutputTokens)
	assert.Equal(t, 1500, manager.CurrentTokens)
	assert.True(t, logCalled)
}

func TestManager_GetUtilizationPercent(t *testing.T) {
	manager := NewManager("gpt-4", nil) // 8192 context limit
	
	// Test empty context
	assert.Equal(t, 0.0, manager.GetUtilizationPercent())
	
	// Test 50% utilization
	manager.CurrentTokens = 4096
	assert.InDelta(t, 0.5, manager.GetUtilizationPercent(), 0.001)
	
	// Test 90% utilization
	manager.CurrentTokens = 7372 // 90% of 8192
	assert.InDelta(t, 0.9, manager.GetUtilizationPercent(), 0.001)
	
	// Test over 100% (edge case)
	manager.CurrentTokens = 10000
	assert.Greater(t, manager.GetUtilizationPercent(), 1.0)
}

func TestManager_ShouldSummarize(t *testing.T) {
	manager := NewManager("gpt-4", nil) // 8192 context limit
	
	// Below threshold - should not summarize
	manager.CurrentTokens = 7000 // ~85%
	should, reason := manager.ShouldSummarize()
	assert.False(t, should)
	assert.Contains(t, reason, "below threshold")
	
	// At threshold - should summarize
	manager.CurrentTokens = 7373 // Just over 90% of 8192 (7372.8)
	should, reason = manager.ShouldSummarize()
	assert.True(t, should)
	assert.Contains(t, reason, "threshold")
	
	// Above threshold - should summarize
	manager.CurrentTokens = 7500 // ~91%
	should, reason = manager.ShouldSummarize()
	assert.True(t, should)
	assert.Contains(t, reason, "threshold")
}

func TestManager_CanExecuteAction(t *testing.T) {
	manager := NewManager("gpt-4", nil) // 8192 limit, 4000 buffer
	manager.CurrentTokens = 3000
	
	// Small action - should succeed
	canExecute, reason := manager.CanExecuteAction(500)
	assert.True(t, canExecute)
	assert.Contains(t, reason, "safely")
	
	// Large action that would overflow - should fail
	canExecute, reason = manager.CanExecuteAction(2000) // 3000 + 2000 + 4000 > 8192
	assert.False(t, canExecute)
	assert.Contains(t, reason, "exceed")
	
	// Edge case - exactly at limit
	// availableSpace := 8192 - 3000 - 4000 = 1192 remaining
	canExecute, reason = manager.CanExecuteAction(1192)
	assert.True(t, canExecute)
	
	canExecute, reason = manager.CanExecuteAction(1193)
	assert.False(t, canExecute)
}

func TestManager_GetTokensRemaining(t *testing.T) {
	manager := NewManager("gpt-4", nil) // 8192 context limit
	
	// Empty context
	assert.Equal(t, 8192, manager.GetTokensRemaining())
	
	// Partial usage
	manager.CurrentTokens = 3000
	assert.Equal(t, 5192, manager.GetTokensRemaining())
	
	// Over limit (edge case)
	manager.CurrentTokens = 10000
	assert.Equal(t, 0, manager.GetTokensRemaining())
}

func TestManager_GetSafeActionLimit(t *testing.T) {
	manager := NewManager("gpt-4", nil) // 8192 limit, 4000 buffer
	manager.CurrentTokens = 2000
	
	// Normal case: 8192 - 2000 - 4000 = 2192
	assert.Equal(t, 2192, manager.GetSafeActionLimit())
	
	// Near limit case
	manager.CurrentTokens = 6000 // 8192 - 6000 - 4000 = -1808
	assert.Equal(t, 0, manager.GetSafeActionLimit())
}

func TestManager_IsApproachingLimit(t *testing.T) {
	manager := NewManager("gpt-4", nil) // 8192 context limit
	
	// Below threshold
	manager.CurrentTokens = 7000 // ~85%
	assert.False(t, manager.IsApproachingLimit())
	
	// At threshold
	manager.CurrentTokens = 7373 // Just over 90% of 8192 (7372.8)
	assert.True(t, manager.IsApproachingLimit())
	
	// Above threshold
	manager.CurrentTokens = 7500 // ~91%
	assert.True(t, manager.IsApproachingLimit())
}

func TestManager_GetContextStatus(t *testing.T) {
	manager := NewManager("gpt-4", nil) // 8192 limit
	
	// Test low utilization
	manager.CurrentTokens = 2000 // ~24%
	status := manager.GetContextStatus()
	
	assert.True(t, status.CanExecuteAction)
	assert.False(t, status.ShouldSummarize)
	assert.InDelta(t, 0.24, status.UtilizationPercent, 0.01)
	assert.Equal(t, 6192, status.TokensRemaining)
	assert.Contains(t, status.RecommendedAction, "Normal operation")
	
	// Test high utilization
	manager.CurrentTokens = 7500 // ~91%
	status = manager.GetContextStatus()
	
	assert.False(t, status.CanExecuteAction) // 692 remaining < 4000 buffer
	assert.True(t, status.ShouldSummarize)
	assert.InDelta(t, 0.91, status.UtilizationPercent, 0.01)
	assert.Equal(t, 692, status.TokensRemaining)
	assert.Contains(t, status.RecommendedAction, "Summarize")
}

func TestManager_EstimateTokensInMessages(t *testing.T) {
	manager := NewManager("gpt-4", nil)
	
	// Test empty messages
	messages := []*ai.Message{}
	tokens := manager.EstimateTokensInMessages(messages)
	assert.Equal(t, 0, tokens)
	
	// Test single text message
	messages = []*ai.Message{
		{
			Role: ai.RoleUser,
			Content: []*ai.Part{
				ai.NewTextPart("Hello, this is a test message with about twenty words to estimate token count."),
			},
		},
	}
	tokens = manager.EstimateTokensInMessages(messages)
	assert.Greater(t, tokens, 0)
	assert.Less(t, tokens, 100) // Should be reasonable estimate
	
	// Test message with tool request and response
	messages = []*ai.Message{
		{
			Role: ai.RoleUser,
			Content: []*ai.Part{
				ai.NewTextPart("Please read the file."),
			},
		},
		{
			Role: ai.RoleModel,
			Content: []*ai.Part{
				ai.NewToolRequestPart(&ai.ToolRequest{
					Name: "read_text_file",
					Input: map[string]interface{}{
						"file_path": "/path/to/file.txt",
					},
				}),
			},
		},
		{
			Role: ai.RoleTool,
			Content: []*ai.Part{
				ai.NewToolResponsePart(&ai.ToolResponse{
					Name:   "read_text_file",
					Output: strings.Repeat("This is file content. ", 50), // ~200 words
				}),
			},
		},
	}
	
	tokens = manager.EstimateTokensInMessages(messages)
	assert.Greater(t, tokens, 50)  // Should account for all content
	assert.Less(t, tokens, 400)    // But not be excessive (increased for tool content)
}

func TestManager_LogContextWarning(t *testing.T) {
	var loggedData map[string]interface{}
	logCallback := func(data map[string]interface{}) {
		loggedData = data
	}
	
	manager := NewManager("gpt-4", logCallback) // 8192 limit
	
	// Test no warning needed
	manager.CurrentTokens = 2000 // ~24%
	manager.LogContextWarning()
	assert.Nil(t, loggedData) // No log should be generated
	
	// Test medium utilization warning
	manager.CurrentTokens = 6000 // ~73%
	manager.LogContextWarning()
	require.NotNil(t, loggedData)
	assert.Equal(t, "warning", loggedData["level"])
	assert.Contains(t, loggedData["message"], "approaching limit")
	
	// Reset for next test
	loggedData = nil
	
	// Test high utilization warning (needs summarization)
	manager.CurrentTokens = 7500 // ~91%
	manager.LogContextWarning()
	require.NotNil(t, loggedData)
	assert.Equal(t, "warning", loggedData["level"])
	assert.Contains(t, loggedData["message"], "THRESHOLD EXCEEDED")
}

func TestManager_Reset(t *testing.T) {
	logCalled := false
	logCallback := func(data map[string]interface{}) {
		logCalled = true
		assert.Equal(t, "Context manager reset for new conversation", data["message"])
	}
	
	manager := NewManager("gpt-4", logCallback)
	
	// Set some state
	manager.CurrentTokens = 5000
	manager.InputTokens = 3000
	manager.OutputTokens = 2000
	oldTime := manager.LastMeasurement
	
	// Wait a moment to ensure time difference
	time.Sleep(1 * time.Millisecond)
	
	// Reset
	manager.Reset()
	
	assert.Equal(t, 0, manager.CurrentTokens)
	assert.Equal(t, 0, manager.InputTokens)
	assert.Equal(t, 0, manager.OutputTokens)
	assert.True(t, manager.LastMeasurement.After(oldTime))
	assert.True(t, logCalled)
}

func TestManager_UpdateModel(t *testing.T) {
	logCalled := false
	logCallback := func(data map[string]interface{}) {
		logCalled = true
		assert.Equal(t, "Context manager model updated", data["message"])
	}
	
	manager := NewManager("gpt-4", logCallback)
	originalLimit := manager.ModelContextLimit
	
	// Update to different model
	manager.UpdateModel("gpt-4o")
	
	assert.Equal(t, "gpt-4o", manager.ModelName)
	assert.Equal(t, 128000, manager.ModelContextLimit)
	assert.NotEqual(t, originalLimit, manager.ModelContextLimit)
	assert.True(t, logCalled)
}

// Benchmark test for token estimation performance
func BenchmarkManager_EstimateTokensInMessages(b *testing.B) {
	manager := NewManager("gpt-4", nil)
	
	// Create a reasonably large conversation
	messages := make([]*ai.Message, 20)
	for i := 0; i < 20; i++ {
		messages[i] = &ai.Message{
			Role: ai.RoleUser,
			Content: []*ai.Part{
				ai.NewTextPart(strings.Repeat("This is a test message with some content. ", 10)),
			},
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.EstimateTokensInMessages(messages)
	}
}