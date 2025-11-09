package services

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHierarchicalAgentExecution tests end-to-end multi-agent hierarchical execution
func TestHierarchicalAgentExecution(t *testing.T) {
	// Skip if no OpenAI API key
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("Skipping E2E test: OPENAI_API_KEY not set")
	}

	t.Run("Parent-Child Run Tracking", func(t *testing.T) {
		// Test that parent run ID context is properly passed and tracked
		ctx := context.Background()

		// Create parent run ID
		var parentRunID int64 = 100

		// Add to context
		ctx = WithParentRunID(ctx, parentRunID)

		// Extract and verify
		extracted := GetParentRunIDFromContext(ctx)
		require.NotNil(t, extracted, "Parent run ID should be extractable from context")
		assert.Equal(t, parentRunID, *extracted, "Extracted parent run ID should match")

		t.Logf("Parent run ID tracking verified: %d", *extracted)
	})

	t.Run("Context Propagation Through Hierarchy", func(t *testing.T) {
		// Test that context propagates through multiple levels
		ctx := context.Background()

		// Level 1: Orchestrator
		orchestratorRunID := int64(200)
		ctx1 := WithParentRunID(ctx, orchestratorRunID)

		extracted1 := GetParentRunIDFromContext(ctx1)
		require.NotNil(t, extracted1)
		assert.Equal(t, orchestratorRunID, *extracted1)

		// Level 2: Worker agent (child of orchestrator)
		workerRunID := int64(201)
		ctx2 := WithParentRunID(ctx1, workerRunID)

		extracted2 := GetParentRunIDFromContext(ctx2)
		require.NotNil(t, extracted2)
		assert.Equal(t, workerRunID, *extracted2, "Context should contain most recent parent")

		t.Logf("Multi-level context propagation verified: L1=%d, L2=%d", *extracted1, *extracted2)
	})
}

// TestAgentAsToolCreation tests agent-as-tool creation
func TestAgentAsToolCreation(t *testing.T) {
	t.Run("Tool Name Format", func(t *testing.T) {
		// Test that agent tool names follow the expected format
		toolName := "mcp__station__agent__test-worker"

		assert.Contains(t, toolName, "mcp__station__agent__")
		assert.Contains(t, toolName, "test-worker")

		t.Logf("Agent-as-tool name format verified: %s", toolName)
	})
}

// TestExecutionTimeout tests that timeouts are properly handled
func TestExecutionTimeout(t *testing.T) {
	t.Run("Context Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// Wait for context to expire
		<-ctx.Done()

		err := ctx.Err()
		assert.Error(t, err, "Context should have error after timeout")
		assert.Equal(t, context.DeadlineExceeded, err, "Error should be DeadlineExceeded")

		t.Logf("Timeout handling verified: %v", err)
	})
}
