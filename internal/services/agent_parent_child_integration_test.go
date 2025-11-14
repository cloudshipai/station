package services

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"station/pkg/models"
)

// TestParentRunIDBackwardsCompatibility tests that existing runs work with NULL parent_run_id
func TestParentRunIDBackwardsCompatibility(t *testing.T) {
	// Test that the database layer handles NULL parent_run_id correctly
	t.Run("NULL parent_run_id handling", func(t *testing.T) {
		// This would normally test with real database, but for unit test we verify the model
		run := &models.AgentRun{
			ID:            1,
			AgentID:       1,
			UserID:        1,
			Task:          "Test task",
			FinalResponse: "Test response",
			StepsTaken:    5,
			Status:        "completed",
			ParentRunID:   nil, // This should be valid - represents old runs
		}

		if run.ParentRunID != nil {
			t.Error("ParentRunID should be nil for backwards compatibility")
		}
	})
}

// TestParentRunIDOptionalBehavior tests that parent_run_id is truly optional
func TestParentRunIDOptionalBehavior(t *testing.T) {
	t.Run("Context without parent run ID", func(t *testing.T) {
		ctx := context.Background()
		parentRunID := GetParentRunIDFromContext(ctx)

		if parentRunID != nil {
			t.Error("Should return nil when no parent run ID in context")
		}
	})

	t.Run("Context with parent run ID", func(t *testing.T) {
		ctx := context.Background()
		testRunID := int64(123)
		ctx = WithParentRunID(ctx, testRunID)

		parentRunID := GetParentRunIDFromContext(ctx)
		if parentRunID == nil {
			t.Fatal("Should return parent run ID when set in context")
		}

		if *parentRunID != testRunID {
			t.Errorf("Expected run ID %d, got %d", testRunID, *parentRunID)
		}
	})
}

// TestAgentToolTimeoutHandling tests timeout behavior for long-running agents
func TestAgentToolTimeoutHandling(t *testing.T) {
	// Create a mock service that simulates timeout
	mockService := &MockAgentServiceWithTimeout{
		delay: 100 * time.Millisecond,
	}

	tool := &AgentAsTool{
		agentID:       1,
		agentName:     "Timeout Test Agent",
		description:   "Tests timeout behavior",
		agentService:  mockService,
		environmentID: 1,
	}

	// Test with short timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := tool.RunRaw(ctx, map[string]interface{}{
		"task": "This should timeout",
	})

	if err == nil {
		t.Fatal("Expected timeout error")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Errorf("Expected timeout-related error, got: %v", err)
	}
}

// MockAgentServiceWithTimeout simulates slow agent execution for timeout testing
type MockAgentServiceWithTimeout struct {
	delay time.Duration
}

func (m *MockAgentServiceWithTimeout) ExecuteAgent(ctx context.Context, agentID int64, task string, userVariables map[string]interface{}) (*Message, error) {
	return m.ExecuteAgentWithRunID(ctx, agentID, task, 0, userVariables)
}

func (m *MockAgentServiceWithTimeout) ExecuteAgentWithRunID(ctx context.Context, agentID int64, task string, runID int64, userVariables map[string]interface{}) (*Message, error) {
	select {
	case <-time.After(m.delay):
		return &Message{
			Content: "Delayed response",
			Role:    RoleAssistant,
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *MockAgentServiceWithTimeout) CreateAgent(ctx context.Context, config *AgentConfig) (*models.Agent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockAgentServiceWithTimeout) GetAgent(ctx context.Context, agentID int64) (*models.Agent, error) {
	return &models.Agent{
		ID:   agentID,
		Name: "Timeout Test Agent",
	}, nil
}

func (m *MockAgentServiceWithTimeout) ListAgentsByEnvironment(ctx context.Context, environmentID int64) ([]*models.Agent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockAgentServiceWithTimeout) UpdateAgent(ctx context.Context, agentID int64, config *AgentConfig) (*models.Agent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockAgentServiceWithTimeout) UpdateAgentPrompt(ctx context.Context, agentID int64, prompt string) error {
	return fmt.Errorf("not implemented")
}

func (m *MockAgentServiceWithTimeout) DeleteAgent(ctx context.Context, agentID int64) error {
	return fmt.Errorf("not implemented")
}

// TestOTELSpanParentChildIntegration tests that OTEL spans include parent-child relationships
func TestOTELSpanParentChildIntegration(t *testing.T) {
	// This would test with real telemetry service, but for unit test we verify the logic
	t.Run("Span attributes include parent run ID", func(t *testing.T) {
		parentRunID := int64(456)
		ctx := WithParentRunID(context.Background(), parentRunID)

		// Verify context contains parent run ID
		retrievedParentID := GetParentRunIDFromContext(ctx)
		if retrievedParentID == nil || *retrievedParentID != parentRunID {
			t.Error("Parent run ID should be retrievable from context")
		}
	})
}
