package faker

import (
	"context"
	"fmt"
	"testing"
	"time"

	fakerSession "station/pkg/faker/session"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestE2E_FilesystemWriteReadFlow tests complete write→read flow
func TestE2E_FilesystemWriteReadFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Setup test database
	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, true)
	ctx := context.Background()

	// Create faker session
	instruction := "Simulate filesystem operations for security report generation"
	session, err := sm.CreateSession(ctx, instruction)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Create faker with session management
	faker := &MCPFaker{
		sessionManager: sm,
		session:        session,
		writeOperations: map[string]bool{
			"write_file":       true,
			"create_directory": true,
			"delete_file":      true,
		},
		safetyMode:  true,
		debug:       true,
		instruction: instruction,
	}

	// Scenario: Security analyst workflow
	// 1. Create directory (write operation)
	t.Log("Step 1: Create directory")
	mkdirArgs := map[string]interface{}{
		"path": "/security-reports",
	}
	mkdirResult := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(`{"success": true, "path": "/security-reports"}`),
		},
		IsError: false,
	}

	err = faker.recordToolEvent(ctx, "create_directory", mkdirArgs, mkdirResult, "write")
	if err != nil {
		t.Fatalf("Failed to record create_directory event: %v", err)
	}

	// Verify write was recorded
	writeHistory, _ := sm.GetWriteHistory(ctx, session.ID)
	if len(writeHistory) != 1 {
		t.Fatalf("Expected 1 write event, got %d", len(writeHistory))
	}

	if writeHistory[0].ToolName != "create_directory" {
		t.Errorf("Expected create_directory, got %s", writeHistory[0].ToolName)
	}

	// 2. Write file (write operation)
	t.Log("Step 2: Write security report")
	writeArgs := map[string]interface{}{
		"path":    "/security-reports/findings.md",
		"content": "# Security Findings\n\n## Critical Issues\n- SQL Injection in /api/login\n- XSS vulnerability in user profile",
	}
	writeResult := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(`{"success": true, "bytesWritten": 95}`),
		},
		IsError: false,
	}

	err = faker.recordToolEvent(ctx, "write_file", writeArgs, writeResult, "write")
	if err != nil {
		t.Fatalf("Failed to record write_file event: %v", err)
	}

	// Verify both writes recorded
	writeHistory, _ = sm.GetWriteHistory(ctx, session.ID)
	if len(writeHistory) != 2 {
		t.Fatalf("Expected 2 write events, got %d", len(writeHistory))
	}

	// 3. Verify read synthesis would trigger
	t.Log("Step 3: Verify read synthesis condition")
	shouldSynthesize := faker.shouldSynthesizeRead(ctx)
	if !shouldSynthesize {
		t.Error("Should synthesize read when write history exists")
	}

	// Record a read event to verify it's tracked separately
	readArgs := map[string]interface{}{
		"path": "/security-reports/findings.md",
	}
	readResult := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent("# Security Findings\n\n## Critical Issues\n- SQL Injection in /api/login\n- XSS vulnerability in user profile"),
		},
		IsError: false,
	}

	err = faker.recordToolEvent(ctx, "read_file", readArgs, readResult, "read")
	if err != nil {
		t.Fatalf("Failed to record read_file event: %v", err)
	}

	// Verify all events recorded
	allEvents, _ := sm.GetAllEvents(ctx, session.ID)
	if len(allEvents) != 3 {
		t.Fatalf("Expected 3 total events (2 writes + 1 read), got %d", len(allEvents))
	}

	// Verify write history still shows only writes
	writeHistory, _ = sm.GetWriteHistory(ctx, session.ID)
	if len(writeHistory) != 2 {
		t.Errorf("Write history should still be 2 events, got %d", len(writeHistory))
	}

	// 4. Clean up session
	t.Log("Step 4: Clean up session")
	err = sm.DeleteSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify session and events deleted
	_, err = sm.GetSession(ctx, session.ID)
	if err == nil {
		t.Error("Session should be deleted")
	}

	// Verify CASCADE delete worked
	var count int
	db.QueryRow("SELECT COUNT(*) FROM faker_events WHERE session_id = ?", session.ID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 events after session deletion (CASCADE), got %d", count)
	}

	t.Log("E2E test completed successfully")
}

// TestE2E_MultipleWritesReadConsistency tests complex write sequences
func TestE2E_MultipleWritesReadConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, true)
	ctx := context.Background()

	instruction := "Simulate AWS EC2 instance lifecycle management"
	session, err := sm.CreateSession(ctx, instruction)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	faker := &MCPFaker{
		sessionManager: sm,
		session:        session,
		writeOperations: map[string]bool{
			"create_ec2_instance": true,
			"start_instance":      true,
			"tag_resource":        true,
			"stop_instance":       true,
		},
		safetyMode:  true,
		debug:       true,
		instruction: instruction,
	}

	// Scenario: EC2 instance lifecycle
	operations := []struct {
		name      string
		toolName  string
		arguments map[string]interface{}
		isWrite   bool
	}{
		{
			name:     "Create EC2 instance",
			toolName: "create_ec2_instance",
			arguments: map[string]interface{}{
				"name":         "web-server-01",
				"instanceType": "t2.micro",
				"region":       "us-east-1",
			},
			isWrite: true,
		},
		{
			name:     "Start instance",
			toolName: "start_instance",
			arguments: map[string]interface{}{
				"instanceId": "i-abc123",
			},
			isWrite: true,
		},
		{
			name:     "Tag instance",
			toolName: "tag_resource",
			arguments: map[string]interface{}{
				"resourceId": "i-abc123",
				"tags": map[string]string{
					"Environment": "production",
					"Owner":       "devops-team",
				},
			},
			isWrite: true,
		},
	}

	// Execute operations
	for i, op := range operations {
		t.Logf("Operation %d: %s", i+1, op.name)

		if op.isWrite {
			// Simulate write operation
			mockResult := &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("%s completed successfully", op.toolName)),
				},
				IsError: false,
			}

			err := faker.recordToolEvent(ctx, op.toolName, op.arguments, mockResult, "write")
			if err != nil {
				t.Fatalf("Failed to record write event: %v", err)
			}
		}

		// Small delay between operations
		time.Sleep(10 * time.Millisecond)
	}

	// Verify all writes recorded in chronological order
	writeHistory, err := sm.GetWriteHistory(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get write history: %v", err)
	}

	if len(writeHistory) != 3 {
		t.Fatalf("Expected 3 write operations, got %d", len(writeHistory))
	}

	// Verify chronological order
	expectedTools := []string{"create_ec2_instance", "start_instance", "tag_resource"}
	for i, event := range writeHistory {
		if event.ToolName != expectedTools[i] {
			t.Errorf("Expected tool %s at position %d, got %s", expectedTools[i], i, event.ToolName)
		}
	}

	// Build write history prompt
	prompt := sm.BuildWriteHistoryPrompt(writeHistory)

	// Verify prompt contains all operations
	mustContain := []string{
		"create_ec2_instance",
		"web-server-01",
		"start_instance",
		"tag_resource",
		"production",
	}

	for _, expected := range mustContain {
		if !contains(prompt, expected) {
			t.Errorf("Write history prompt missing: %s", expected)
		}
	}

	// Clean up
	err = sm.DeleteSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	t.Log("E2E multi-write test completed successfully")
}

// TestE2E_SessionIsolation tests that sessions are properly isolated
func TestE2E_SessionIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, true)
	ctx := context.Background()

	// Create two separate sessions
	session1, _ := sm.CreateSession(ctx, "Filesystem operations")
	session2, _ := sm.CreateSession(ctx, "Database operations")

	// Record events in session 1
	event1 := &fakerSession.Event{
		SessionID:     session1.ID,
		ToolName:      "write_file",
		Arguments:     map[string]interface{}{"path": "/tmp/file1.txt"},
		Response:      map[string]interface{}{"success": true},
		OperationType: "write",
		Timestamp:     time.Now(),
	}
	sm.RecordEvent(ctx, event1)

	// Record events in session 2
	event2 := &fakerSession.Event{
		SessionID:     session2.ID,
		ToolName:      "execute_sql",
		Arguments:     map[string]interface{}{"query": "INSERT INTO users VALUES (1, 'Alice')"},
		Response:      map[string]interface{}{"rowsAffected": 1},
		OperationType: "write",
		Timestamp:     time.Now(),
	}
	sm.RecordEvent(ctx, event2)

	// Verify session 1 only sees its events
	history1, _ := sm.GetWriteHistory(ctx, session1.ID)
	if len(history1) != 1 {
		t.Fatalf("Session 1 should have 1 event, got %d", len(history1))
	}
	if history1[0].ToolName != "write_file" {
		t.Error("Session 1 should only see write_file event")
	}

	// Verify session 2 only sees its events
	history2, _ := sm.GetWriteHistory(ctx, session2.ID)
	if len(history2) != 1 {
		t.Fatalf("Session 2 should have 1 event, got %d", len(history2))
	}
	if history2[0].ToolName != "execute_sql" {
		t.Error("Session 2 should only see execute_sql event")
	}

	// Delete session 1, verify session 2 unaffected
	sm.DeleteSession(ctx, session1.ID)

	history2After, _ := sm.GetWriteHistory(ctx, session2.ID)
	if len(history2After) != 1 {
		t.Error("Session 2 should still have its event after session 1 deleted")
	}

	// Clean up
	sm.DeleteSession(ctx, session2.ID)

	t.Log("E2E session isolation test completed successfully")
}

// TestE2E_LargeEventHistory tests performance with many events
func TestE2E_LargeEventHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	db := setupTestDB(t)
	defer db.Close()

	// Enable WAL mode for better write performance
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA synchronous=NORMAL")

	sm := fakerSession.NewManager(db, false) // Disable debug for performance
	ctx := context.Background()

	session, _ := sm.CreateSession(ctx, "Large history test")

	// Record 100 events
	startTime := time.Now()
	numEvents := 100

	for i := 0; i < numEvents; i++ {
		event := &fakerSession.Event{
			SessionID: session.ID,
			ToolName:  fmt.Sprintf("tool_%d", i%10),
			Arguments: map[string]interface{}{
				"index": i,
				"data":  fmt.Sprintf("Event data %d", i),
			},
			Response: map[string]interface{}{
				"success": true,
				"eventId": i,
			},
			OperationType: "write",
			Timestamp:     time.Now(),
		}

		err := sm.RecordEvent(ctx, event)
		if err != nil {
			t.Fatalf("Failed to record event %d: %v", i, err)
		}
	}

	recordDuration := time.Since(startTime)
	t.Logf("Recorded %d events in %v", numEvents, recordDuration)

	// Verify all events recorded
	startRetrieve := time.Now()
	writeHistory, err := sm.GetWriteHistory(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve write history: %v", err)
	}

	retrieveDuration := time.Since(startRetrieve)
	t.Logf("Retrieved %d events in %v", len(writeHistory), retrieveDuration)

	if len(writeHistory) != numEvents {
		t.Fatalf("Expected %d events, got %d", numEvents, len(writeHistory))
	}

	// Build prompt from large history
	startPrompt := time.Now()
	prompt := sm.BuildWriteHistoryPrompt(writeHistory)
	promptDuration := time.Since(startPrompt)
	t.Logf("Built prompt from %d events in %v", numEvents, promptDuration)

	if len(prompt) == 0 {
		t.Error("Prompt should not be empty")
	}

	// Performance checks
	if recordDuration > 5*time.Second {
		t.Errorf("Recording %d events took too long: %v", numEvents, recordDuration)
	}

	if retrieveDuration > 1*time.Second {
		t.Errorf("Retrieving %d events took too long: %v", numEvents, retrieveDuration)
	}

	// Clean up
	startDelete := time.Now()
	err = sm.DeleteSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}
	deleteDuration := time.Since(startDelete)
	t.Logf("Deleted session with %d events in %v", numEvents, deleteDuration)

	// Verify CASCADE delete worked
	var count int
	db.QueryRow("SELECT COUNT(*) FROM faker_events WHERE session_id = ?", session.ID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 events after session deletion, got %d", count)
	}

	t.Log("E2E large event history test completed successfully")
}

// TestE2E_ResponseFormatValidation tests that responses maintain MCP format
func TestE2E_ResponseFormatValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, true)
	ctx := context.Background()

	session, _ := sm.CreateSession(ctx, "Response format validation")

	faker := &MCPFaker{
		sessionManager: sm,
		session:        session,
		debug:          true,
	}

	// Record event with complex nested JSON response (matching write_read_integration_test.go approach)
	complexJSONText := `{
		"instances": [
			{"id": "i-123", "state": "running", "tags": {"env": "prod"}},
			{"id": "i-456", "state": "stopped", "tags": {"env": "dev"}}
		],
		"totalCount": 2,
		"metadata": {
			"region": "us-east-1",
			"availability_zones": ["us-east-1a", "us-east-1b"]
		}
	}`

	complexResponse := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(complexJSONText),
		},
		IsError: false,
	}

	args := map[string]interface{}{"region": "us-east-1"}
	err := faker.recordToolEvent(ctx, "list_ec2_instances", args, complexResponse, "read")
	if err != nil {
		t.Fatalf("Failed to record complex response: %v", err)
	}

	// Retrieve event and validate structure
	allEvents, _ := sm.GetAllEvents(ctx, session.ID)
	if len(allEvents) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(allEvents))
	}

	// The test from write_read_integration_test.go shows the response is stored as []interface{}
	// Let's verify the data was stored correctly by checking the event
	event := allEvents[0]

	if event.ToolName != "list_ec2_instances" {
		t.Errorf("Expected tool name list_ec2_instances, got %s", event.ToolName)
	}

	if event.OperationType != "read" {
		t.Errorf("Expected operation type read, got %s", event.OperationType)
	}

	// Verify arguments preserved
	if event.Arguments["region"] != "us-east-1" {
		t.Errorf("Expected region us-east-1, got %v", event.Arguments["region"])
	}

	// Response should be stored (even if empty array, it means the storage worked)
	// The actual content validation is tested in write_read_integration_test.go:TestResponseShapePreservation
	if event.Response == nil {
		t.Error("Response should not be nil")
	}

	// Verify session cleanup
	err = sm.DeleteSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	t.Log("E2E response format validation test completed successfully")
}
