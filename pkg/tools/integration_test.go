package tools

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/openai/openai-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real OpenAI Integration Test - requires OPENAI_API_KEY
func TestExecutor_RealOpenAI_Integration(t *testing.T) {
	// Skip if no API key provided
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping real OpenAI test: OPENAI_API_KEY not set")
	}

	// Create real OpenAI client - constructor reads API key from environment
	client := openai.NewClient()
	
	// Mock context manager that simulates real usage
	contextManager := &MockContextManager{
		utilizationPercent: 0.3,  // 30% context usage
		tokensRemaining:    5000, // Plenty of space
		safeActionLimit:    3000,
		canExecuteAction:   true,
	}

	config := ToolExecutorConfig{
		EnableContextProtection: true,
		MaxOutputTokens:        500, // Limit output for cost control
		TruncationStrategy:     TruncationStrategyIntelligent,
		ToolTimeout:           30 * time.Second,
	}

	executor := NewExecutor(config, contextManager, nil)

	execCtx := &ExecutionContext{
		ConversationID:   "real-test-conversation",
		TurnNumber:       1,
		RemainingTokens:  5000,
		SafeActionLimit:  3000,
	}

	// Create real tool execution scenario
	toolCall := &ai.ToolRequest{
		Name: "analyze_code", // Simulate a code analysis tool
		Input: map[string]interface{}{
			"code": `
func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}
			`,
			"language": "go",
		},
	}

	t.Run("Real Tool Execution with Context Protection", func(t *testing.T) {
		// Execute tool with real-world simulation
		result, err := executor.ExecuteTool(context.Background(), toolCall, execCtx)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "analyze_code", result.ToolName)
		assert.True(t, result.Success)
		assert.NotEmpty(t, result.ExecutionID)
		assert.Greater(t, result.Duration, time.Duration(0))

		// Verify context protection worked
		assert.Greater(t, len(contextManager.tokenUsageHistory), 0)
		assert.Less(t, contextManager.tokensRemaining, 5000) // Tokens were consumed

		t.Logf("✅ Real tool execution completed:")
		t.Logf("  Tool: %s", result.ToolName)
		t.Logf("  Success: %v", result.Success)
		t.Logf("  Duration: %v", result.Duration)
		t.Logf("  Output Length: %d chars", len(result.Output))
		t.Logf("  Tokens Used: %d", result.TokensUsed)
		t.Logf("  Truncated: %v", result.Truncated)
	})

	t.Run("Context Overflow Protection", func(t *testing.T) {
		// Simulate context near limit
		highUtilizationManager := &MockContextManager{
			utilizationPercent: 0.95, // 95% utilization
			isApproachingLimit: true,
			tokensRemaining:    200,   // Very low
			safeActionLimit:    100,
			canExecuteAction:   false,
			canExecuteReason:   "Context approaching limit",
		}

		protectedExecutor := NewExecutor(config, highUtilizationManager, nil)

		protectedExecCtx := &ExecutionContext{
			RemainingTokens: 200,
			SafeActionLimit: 100,
		}

		result, err := protectedExecutor.ExecuteTool(context.Background(), toolCall, protectedExecCtx)

		require.NoError(t, err)
		assert.False(t, result.Success) // Should be blocked
		assert.Contains(t, result.Error, "Context protection prevented execution")

		t.Logf("✅ Context protection worked:")
		t.Logf("  Blocked execution due to: %s", result.Error)
		t.Logf("  Context utilization: 95%%")
		t.Logf("  Remaining tokens: 200")
	})

	t.Run("Real GPT-4o-mini Call", func(t *testing.T) {
		// Make actual API call to test token estimation accuracy
		ctx := context.Background()
		
		response, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model: "gpt-4o-mini",
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Write a short explanation of the Go fibonacci function above in exactly 2 sentences."),
			},
			MaxTokens: openai.Int(100), // Limit for cost control
		})

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Len(t, response.Choices, 1)

		actualTokens := int(response.Usage.TotalTokens)
		actualOutput := response.Choices[0].Message.Content

		// Test our token estimation accuracy
		estimatedTokens, err := executor.EstimateToolOutputTokens(&ai.ToolRequest{
			Name: "gpt_analysis",
		})
		require.NoError(t, err)

		t.Logf("✅ Real GPT-4o-mini API call:")
		t.Logf("  Input: 'Write a short explanation...'")
		t.Logf("  Actual tokens used: %d", actualTokens)
		t.Logf("  Our estimation: %d tokens", estimatedTokens)
		t.Logf("  Estimation accuracy: %.1f%%", float64(estimatedTokens)/float64(actualTokens)*100)
		t.Logf("  Response: %s", actualOutput)

		// Our estimation should be conservative (within reasonable bounds)
		// Being conservative (higher) is better for context protection
		assert.Greater(t, estimatedTokens, actualTokens/4)  // Not too low (at least 25% of actual)
		assert.Less(t, estimatedTokens, actualTokens*5)     // Not too high (at most 5x actual)
	})
}

// Benchmark real API performance
func BenchmarkExecutor_RealOpenAI(b *testing.B) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		b.Skip("Skipping real OpenAI benchmark: OPENAI_API_KEY not set")
	}

	contextManager := &MockContextManager{
		utilizationPercent: 0.5,
		tokensRemaining:    10000,
		safeActionLimit:    5000,
		canExecuteAction:   true,
	}

	executor := NewExecutor(ToolExecutorConfig{
		MaxOutputTokens: 100,
		ToolTimeout:    10 * time.Second,
	}, contextManager, nil)

	execCtx := &ExecutionContext{
		RemainingTokens: 10000,
		SafeActionLimit: 5000,
	}

	toolCall := &ai.ToolRequest{
		Name: "simple_analysis",
		Input: map[string]interface{}{
			"prompt": "Count to 3",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.ExecuteTool(context.Background(), toolCall, execCtx)
	}
}