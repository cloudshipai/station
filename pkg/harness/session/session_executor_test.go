package session

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestHistoryStore_SaveLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "history-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewHistoryStore(tmpDir)
	sessionID := "test-session-1"

	// Create session directory
	os.MkdirAll(store.historyPath(sessionID)[:len(store.historyPath(sessionID))-len("/.history.json")], 0755)

	// Test empty load (should return empty history)
	history, err := store.Load(sessionID)
	if err != nil {
		t.Fatalf("Failed to load empty history: %v", err)
	}
	if len(history.Messages) != 0 {
		t.Errorf("Expected empty messages, got %d", len(history.Messages))
	}

	// Add some messages
	history.Messages = []StoredMessage{
		{Role: "user", Content: "Hello", Timestamp: time.Now()},
		{Role: "assistant", Content: "Hi there!", Timestamp: time.Now()},
	}
	history.TotalTokens = 100

	// Save
	if err := store.Save(sessionID, history); err != nil {
		t.Fatalf("Failed to save history: %v", err)
	}

	// Load again and verify
	loaded, err := store.Load(sessionID)
	if err != nil {
		t.Fatalf("Failed to load history: %v", err)
	}

	if len(loaded.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(loaded.Messages))
	}

	if loaded.Messages[0].Content != "Hello" {
		t.Errorf("Expected first message 'Hello', got '%s'", loaded.Messages[0].Content)
	}

	if loaded.TotalTokens != 100 {
		t.Errorf("Expected 100 tokens, got %d", loaded.TotalTokens)
	}
}

func TestHistoryStore_Append(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "history-append-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewHistoryStore(tmpDir)
	sessionID := "test-session-2"

	// Append first message
	err = store.Append(sessionID, []StoredMessage{
		{Role: "user", Content: "First message", Timestamp: time.Now()},
	})
	if err != nil {
		t.Fatalf("Failed to append first message: %v", err)
	}

	// Append second message
	err = store.Append(sessionID, []StoredMessage{
		{Role: "assistant", Content: "Response", Timestamp: time.Now()},
	})
	if err != nil {
		t.Fatalf("Failed to append second message: %v", err)
	}

	// Load and verify
	history, err := store.Load(sessionID)
	if err != nil {
		t.Fatalf("Failed to load history: %v", err)
	}

	if len(history.Messages) != 2 {
		t.Errorf("Expected 2 messages after append, got %d", len(history.Messages))
	}

	if history.Messages[0].Content != "First message" {
		t.Errorf("Expected first message 'First message', got '%s'", history.Messages[0].Content)
	}
}

func TestHistoryStore_Clear(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "history-clear-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewHistoryStore(tmpDir)
	sessionID := "test-session-3"

	// Create some history
	history := &SessionHistory{
		SessionID: sessionID,
		Messages: []StoredMessage{
			{Role: "user", Content: "Message", Timestamp: time.Now()},
		},
	}
	store.Save(sessionID, history)

	// Clear it
	err = store.Clear(sessionID)
	if err != nil {
		t.Fatalf("Failed to clear history: %v", err)
	}

	// Load again - should be empty
	loaded, err := store.Load(sessionID)
	if err != nil {
		t.Fatalf("Failed to load after clear: %v", err)
	}

	if len(loaded.Messages) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(loaded.Messages))
	}
}

func TestSessionManager_Integration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	manager := NewManager(tmpDir)
	historyStore := NewHistoryStore(tmpDir)

	// Create session
	session, err := manager.Create(ctx, "integration-test", "", "")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Acquire lock
	runID := "test-run-1"
	if err := manager.AcquireLock(ctx, session.ID, runID); err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Add messages
	err = historyStore.Append(session.ID, []StoredMessage{
		{Role: "user", Content: "First input", Timestamp: time.Now()},
		{Role: "assistant", Content: "First response", Timestamp: time.Now()},
	})
	if err != nil {
		t.Fatalf("Failed to append messages: %v", err)
	}

	// Simulate "closing" the REPL (release lock)
	if err := manager.ReleaseLock(ctx, session.ID, runID); err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}

	// Simulate "reopening" the REPL (resume session)
	resumedSession, err := manager.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if resumedSession.TotalRuns != 1 {
		t.Errorf("Expected TotalRuns=1, got %d", resumedSession.TotalRuns)
	}

	// Load history from resumed session
	history, err := historyStore.Load(session.ID)
	if err != nil {
		t.Fatalf("Failed to load history: %v", err)
	}

	if len(history.Messages) != 2 {
		t.Errorf("Expected 2 messages after resume, got %d", len(history.Messages))
	}

	// Add more messages in new "session"
	runID2 := "test-run-2"
	if err := manager.AcquireLock(ctx, session.ID, runID2); err != nil {
		t.Fatalf("Failed to acquire lock for run 2: %v", err)
	}

	err = historyStore.Append(session.ID, []StoredMessage{
		{Role: "user", Content: "Second input", Timestamp: time.Now()},
		{Role: "assistant", Content: "Second response", Timestamp: time.Now()},
	})
	if err != nil {
		t.Fatalf("Failed to append more messages: %v", err)
	}

	// Verify all 4 messages are present
	history, _ = historyStore.Load(session.ID)
	if len(history.Messages) != 4 {
		t.Errorf("Expected 4 messages total, got %d", len(history.Messages))
	}

	t.Logf("Session integration test passed - history persists across REPL sessions")
}
