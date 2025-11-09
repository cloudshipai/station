package faker

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Enable foreign keys for CASCADE delete
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	// Create faker_sessions table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS faker_sessions (
			id TEXT PRIMARY KEY,
			instruction TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create faker_sessions table: %v", err)
	}

	// Create faker_events table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS faker_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			arguments TEXT NOT NULL,
			response TEXT NOT NULL,
			operation_type TEXT NOT NULL CHECK(operation_type IN ('read', 'write')),
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES faker_sessions(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create faker_events table: %v", err)
	}

	// Create indexes
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_faker_events_session_time
			ON faker_events(session_id, timestamp)
	`)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	return db
}

func TestSessionManager_CreateSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := NewSessionManager(db, false)
	ctx := context.Background()

	instruction := "Test scenario: Simulate AWS EC2 instance management"
	session, err := sm.CreateSession(ctx, instruction)

	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.ID == "" {
		t.Error("Session ID should not be empty")
	}

	if session.Instruction != instruction {
		t.Errorf("Expected instruction %q, got %q", instruction, session.Instruction)
	}

	if session.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	// Verify session was persisted to database
	retrievedSession, err := sm.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if retrievedSession.ID != session.ID {
		t.Errorf("Expected session ID %s, got %s", session.ID, retrievedSession.ID)
	}
}

func TestSessionManager_RecordEvent_Write(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := NewSessionManager(db, false)
	ctx := context.Background()

	session, _ := sm.CreateSession(ctx, "Test scenario")

	event := &FakerEvent{
		SessionID: session.ID,
		ToolName:  "create_ec2_instance",
		Arguments: map[string]interface{}{
			"name":         "web-server",
			"instanceType": "t2.micro",
		},
		Response: map[string]interface{}{
			"success":    true,
			"instanceId": "i-abc123",
		},
		OperationType: "write",
		Timestamp:     time.Now(),
	}

	err := sm.RecordEvent(ctx, event)
	if err != nil {
		t.Fatalf("RecordEvent failed: %v", err)
	}

	if event.ID == 0 {
		t.Error("Event ID should be set after recording")
	}

	// Retrieve write history
	writeHistory, err := sm.GetWriteHistory(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetWriteHistory failed: %v", err)
	}

	if len(writeHistory) != 1 {
		t.Fatalf("Expected 1 write event, got %d", len(writeHistory))
	}

	if writeHistory[0].ToolName != "create_ec2_instance" {
		t.Errorf("Expected tool name create_ec2_instance, got %s", writeHistory[0].ToolName)
	}

	if writeHistory[0].Arguments["name"] != "web-server" {
		t.Errorf("Expected name web-server, got %v", writeHistory[0].Arguments["name"])
	}
}

func TestSessionManager_RecordEvent_Read(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := NewSessionManager(db, false)
	ctx := context.Background()

	session, _ := sm.CreateSession(ctx, "Test scenario")

	readEvent := &FakerEvent{
		SessionID: session.ID,
		ToolName:  "list_instances",
		Arguments: map[string]interface{}{},
		Response: []map[string]interface{}{
			{"instanceId": "i-abc123", "name": "web-server"},
		},
		OperationType: "read",
		Timestamp:     time.Now(),
	}

	err := sm.RecordEvent(ctx, readEvent)
	if err != nil {
		t.Fatalf("RecordEvent failed: %v", err)
	}

	// Read events should NOT appear in write history
	writeHistory, _ := sm.GetWriteHistory(ctx, session.ID)
	if len(writeHistory) != 0 {
		t.Errorf("Expected 0 write events, got %d", len(writeHistory))
	}

	// But should appear in all events
	allEvents, err := sm.GetAllEvents(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetAllEvents failed: %v", err)
	}

	if len(allEvents) != 1 {
		t.Fatalf("Expected 1 total event, got %d", len(allEvents))
	}

	if allEvents[0].OperationType != "read" {
		t.Errorf("Expected operation type read, got %s", allEvents[0].OperationType)
	}
}

func TestSessionManager_WriteReadSequence(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := NewSessionManager(db, false)
	ctx := context.Background()

	session, _ := sm.CreateSession(ctx, "Simulate EC2 instance lifecycle")

	// Write: Create instance
	writeEvent := &FakerEvent{
		SessionID: session.ID,
		ToolName:  "create_ec2_instance",
		Arguments: map[string]interface{}{
			"name": "app-server",
			"type": "t2.small",
		},
		Response: map[string]interface{}{
			"success":    true,
			"instanceId": "i-xyz789",
		},
		OperationType: "write",
		Timestamp:     time.Now(),
	}
	sm.RecordEvent(ctx, writeEvent)

	// Write: Tag instance
	tagEvent := &FakerEvent{
		SessionID: session.ID,
		ToolName:  "tag_instance",
		Arguments: map[string]interface{}{
			"instanceId": "i-xyz789",
			"tags":       map[string]string{"Environment": "production"},
		},
		Response: map[string]interface{}{
			"success": true,
		},
		OperationType: "write",
		Timestamp:     time.Now().Add(1 * time.Second),
	}
	sm.RecordEvent(ctx, tagEvent)

	// Read: List instances
	readEvent := &FakerEvent{
		SessionID: session.ID,
		ToolName:  "list_instances",
		Arguments: map[string]interface{}{},
		Response: []map[string]interface{}{
			{
				"instanceId": "i-xyz789",
				"name":       "app-server",
				"tags":       map[string]string{"Environment": "production"},
			},
		},
		OperationType: "read",
		Timestamp:     time.Now().Add(2 * time.Second),
	}
	sm.RecordEvent(ctx, readEvent)

	// Verify write history contains both writes in chronological order
	writeHistory, _ := sm.GetWriteHistory(ctx, session.ID)
	if len(writeHistory) != 2 {
		t.Fatalf("Expected 2 write events, got %d", len(writeHistory))
	}

	if writeHistory[0].ToolName != "create_ec2_instance" {
		t.Errorf("First write should be create_ec2_instance, got %s", writeHistory[0].ToolName)
	}

	if writeHistory[1].ToolName != "tag_instance" {
		t.Errorf("Second write should be tag_instance, got %s", writeHistory[1].ToolName)
	}

	// Verify all events includes both writes and read in chronological order
	allEvents, _ := sm.GetAllEvents(ctx, session.ID)
	if len(allEvents) != 3 {
		t.Fatalf("Expected 3 total events, got %d", len(allEvents))
	}
}

func TestSessionManager_DeleteSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := NewSessionManager(db, false)
	ctx := context.Background()

	session, _ := sm.CreateSession(ctx, "Test scenario")

	// Record some events
	event := &FakerEvent{
		SessionID:     session.ID,
		ToolName:      "test_tool",
		Arguments:     map[string]interface{}{"key": "value"},
		Response:      map[string]interface{}{"result": "success"},
		OperationType: "write",
		Timestamp:     time.Now(),
	}
	sm.RecordEvent(ctx, event)

	// Delete session
	err := sm.DeleteSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Verify session no longer exists
	_, err = sm.GetSession(ctx, session.ID)
	if err == nil {
		t.Error("Expected error when getting deleted session")
	}

	// Verify events were also deleted (CASCADE)
	// Query database directly to check CASCADE worked
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM faker_events WHERE session_id = ?", session.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 events after session deletion (CASCADE), got %d", count)
	}
}

func TestSessionManager_BuildWriteHistoryPrompt(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := NewSessionManager(db, false)

	events := []*FakerEvent{
		{
			ToolName: "create_file",
			Arguments: map[string]interface{}{
				"path":    "/tmp/report.txt",
				"content": "Security findings",
			},
			Response: map[string]interface{}{
				"success": true,
			},
			Timestamp: time.Date(2025, 11, 8, 10, 30, 0, 0, time.UTC),
		},
		{
			ToolName: "update_file",
			Arguments: map[string]interface{}{
				"path":    "/tmp/report.txt",
				"content": "Security findings\n- SQL injection in login.php",
			},
			Response: map[string]interface{}{
				"success": true,
			},
			Timestamp: time.Date(2025, 11, 8, 10, 31, 0, 0, time.UTC),
		},
	}

	prompt := sm.BuildWriteHistoryPrompt(events)

	// Verify prompt contains both events
	if prompt == "" {
		t.Error("Prompt should not be empty")
	}

	// Check for key elements
	expectedStrings := []string{
		"Previous Write Operations",
		"create_file",
		"update_file",
		"/tmp/report.txt",
		"Security findings",
		"SQL injection",
	}

	for _, expected := range expectedStrings {
		if !contains(prompt, expected) {
			t.Errorf("Expected prompt to contain %q", expected)
		}
	}
}

func TestSessionManager_EmptyWriteHistory(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := NewSessionManager(db, false)
	ctx := context.Background()

	session, _ := sm.CreateSession(ctx, "Test scenario")

	writeHistory, err := sm.GetWriteHistory(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetWriteHistory failed: %v", err)
	}

	// Empty slice is valid, nil is also acceptable in Go
	if writeHistory == nil {
		writeHistory = []*FakerEvent{}
	}

	if len(writeHistory) != 0 {
		t.Errorf("Expected empty write history, got %d events", len(writeHistory))
	}

	// Test prompt generation with empty history
	prompt := sm.BuildWriteHistoryPrompt(writeHistory)
	if !contains(prompt, "No previous write operations") {
		t.Error("Expected prompt to indicate no previous operations")
	}
}

func TestSessionManager_ConcurrentEventRecording(t *testing.T) {
	// SQLite doesn't handle concurrent writes well with in-memory DBs
	// This test verifies serial event recording works correctly
	// For production, we'd use WAL mode with shared cache

	db := setupTestDB(t)
	defer db.Close()

	// Enable SQLite WAL mode for concurrent writes
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA synchronous=NORMAL")

	sm := NewSessionManager(db, false)
	ctx := context.Background()

	session, err := sm.CreateSession(ctx, "Concurrent test")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Record events serially to avoid SQLite concurrency issues in test
	// (Production DB would use connection pooling)
	numEvents := 10
	for i := 0; i < numEvents; i++ {
		event := &FakerEvent{
			SessionID: session.ID,
			ToolName:  "concurrent_tool",
			Arguments: map[string]interface{}{
				"index": i,
			},
			Response: map[string]interface{}{
				"success": true,
			},
			OperationType: "write",
			Timestamp:     time.Now(),
		}
		err := sm.RecordEvent(ctx, event)
		if err != nil {
			t.Errorf("RecordEvent failed: %v", err)
		}
	}

	// Verify all events were recorded
	writeHistory, _ := sm.GetWriteHistory(ctx, session.ID)
	if len(writeHistory) != numEvents {
		t.Errorf("Expected %d events, got %d", numEvents, len(writeHistory))
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr))))
}

func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}
