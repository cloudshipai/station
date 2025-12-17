package faker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	fakerSession "station/pkg/faker/session"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestWriteInterception verifies that write operations are properly intercepted and recorded
func TestWriteInterception(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, false)
	ctx := context.Background()

	session, _ := sm.CreateSession(ctx, "Test write interception")

	// Create a minimal faker with session management
	faker := &MCPFaker{
		sessionManager: sm,
		session:        session,
		writeOperations: map[string]bool{
			"create_file": true,
			"write_file":  true,
		},
		safetyMode: true,
		debug:      false,
	}

	// Simulate write operation
	toolName := "create_file"
	args := map[string]interface{}{
		"path":    "/tmp/security_report.txt",
		"content": "SQL injection found in login endpoint",
	}

	mockResult := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent("File created successfully"),
		},
		IsError: false,
	}

	// Record the write event
	err := faker.recordToolEvent(ctx, toolName, args, mockResult, "write")
	if err != nil {
		t.Fatalf("Failed to record write event: %v", err)
	}

	// Verify write was recorded
	writeHistory, _ := sm.GetWriteHistory(ctx, session.ID)
	if len(writeHistory) != 1 {
		t.Fatalf("Expected 1 write event, got %d", len(writeHistory))
	}

	if writeHistory[0].ToolName != "create_file" {
		t.Errorf("Expected tool name create_file, got %s", writeHistory[0].ToolName)
	}

	if writeHistory[0].Arguments["path"] != "/tmp/security_report.txt" {
		t.Errorf("Write event arguments not preserved correctly")
	}
}

// TestReadSynthesisWithWriteHistory tests read synthesis based on accumulated writes
func TestReadSynthesisWithWriteHistory(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, false)
	ctx := context.Background()

	instruction := "Simulate filesystem with security reports"
	session, _ := sm.CreateSession(ctx, instruction)

	// Record write operation: create file
	writeEvent := &fakerSession.Event{
		SessionID: session.ID,
		ToolName:  "write_file",
		Arguments: map[string]interface{}{
			"path":    "/security/findings.json",
			"content": `{"vulnerabilities": [{"type": "SQLi", "severity": "high"}]}`,
		},
		Response: map[string]interface{}{
			"success": true,
		},
		OperationType: "write",
		Timestamp:     time.Now(),
	}
	sm.RecordEvent(ctx, writeEvent)

	// Now simulate read operation - should see the written file
	writeHistory, _ := sm.GetWriteHistory(ctx, session.ID)

	if len(writeHistory) != 1 {
		t.Fatalf("Expected 1 write in history, got %d", len(writeHistory))
	}

	// Build prompt that would be sent to AI for read synthesis
	prompt := sm.BuildWriteHistoryPrompt(writeHistory)

	// Verify prompt contains write operation details
	if !contains(prompt, "write_file") {
		t.Error("Prompt should contain write_file tool name")
	}

	if !contains(prompt, "/security/findings.json") {
		t.Error("Prompt should contain file path")
	}

	if !contains(prompt, "vulnerabilities") {
		t.Error("Prompt should contain file content")
	}
}

// TestMultipleWritesBeforeRead simulates realistic write→write→read scenario
func TestMultipleWritesBeforeRead(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, false)
	ctx := context.Background()

	session, _ := sm.CreateSession(ctx, "Simulate AWS EC2 management")

	// Write 1: Create instance
	createEvent := &fakerSession.Event{
		SessionID: session.ID,
		ToolName:  "create_ec2_instance",
		Arguments: map[string]interface{}{
			"name":         "web-server-01",
			"instanceType": "t2.micro",
			"region":       "us-east-1",
		},
		Response: map[string]interface{}{
			"instanceId": "i-abc123",
			"state":      "pending",
		},
		OperationType: "write",
		Timestamp:     time.Now(),
	}
	sm.RecordEvent(ctx, createEvent)

	// Write 2: Start instance
	startEvent := &fakerSession.Event{
		SessionID: session.ID,
		ToolName:  "start_instance",
		Arguments: map[string]interface{}{
			"instanceId": "i-abc123",
		},
		Response: map[string]interface{}{
			"success": true,
			"state":   "running",
		},
		OperationType: "write",
		Timestamp:     time.Now().Add(1 * time.Second),
	}
	sm.RecordEvent(ctx, startEvent)

	// Write 3: Tag instance
	tagEvent := &fakerSession.Event{
		SessionID: session.ID,
		ToolName:  "tag_resource",
		Arguments: map[string]interface{}{
			"resourceId": "i-abc123",
			"tags": map[string]string{
				"Environment": "production",
				"Owner":       "devops-team",
			},
		},
		Response: map[string]interface{}{
			"success": true,
		},
		OperationType: "write",
		Timestamp:     time.Now().Add(2 * time.Second),
	}
	sm.RecordEvent(ctx, tagEvent)

	// Verify write history contains all 3 operations in chronological order
	writeHistory, _ := sm.GetWriteHistory(ctx, session.ID)

	if len(writeHistory) != 3 {
		t.Fatalf("Expected 3 write events, got %d", len(writeHistory))
	}

	// Verify order
	expectedTools := []string{"create_ec2_instance", "start_instance", "tag_resource"}
	for i, event := range writeHistory {
		if event.ToolName != expectedTools[i] {
			t.Errorf("Expected tool %s at position %d, got %s", expectedTools[i], i, event.ToolName)
		}
	}

	// Build prompt for list_instances read operation
	prompt := sm.BuildWriteHistoryPrompt(writeHistory)

	// Verify prompt contains all write operations
	if !contains(prompt, "create_ec2_instance") {
		t.Error("Prompt should mention instance creation")
	}

	if !contains(prompt, "web-server-01") {
		t.Error("Prompt should contain instance name")
	}

	if !contains(prompt, "running") {
		t.Error("Prompt should reflect instance state after start")
	}

	if !contains(prompt, "production") {
		t.Error("Prompt should contain tags")
	}
}

// TestShouldSynthesizeRead verifies logic for when to synthesize reads
func TestShouldSynthesizeRead(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, false)
	ctx := context.Background()

	session, _ := sm.CreateSession(ctx, "Test synthesis decision")

	faker := &MCPFaker{
		sessionManager: sm,
		session:        session,
		debug:          false,
	}

	// Initially, no write history - should NOT synthesize
	shouldSynthesize := faker.shouldSynthesizeRead(ctx)
	if shouldSynthesize {
		t.Error("Should not synthesize read when no write history exists")
	}

	// Record a write operation
	writeEvent := &fakerSession.Event{
		SessionID:     session.ID,
		ToolName:      "create_resource",
		Arguments:     map[string]interface{}{"name": "test"},
		Response:      map[string]interface{}{"success": true},
		OperationType: "write",
		Timestamp:     time.Now(),
	}
	sm.RecordEvent(ctx, writeEvent)

	// Now with write history - SHOULD synthesize
	shouldSynthesize = faker.shouldSynthesizeRead(ctx)
	if !shouldSynthesize {
		t.Error("Should synthesize read when write history exists")
	}
}

// TestReadEventRecording verifies that synthesized reads are also recorded
func TestReadEventRecording(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, false)
	ctx := context.Background()

	session, _ := sm.CreateSession(ctx, "Test read recording")

	faker := &MCPFaker{
		sessionManager: sm,
		session:        session,
		debug:          false,
	}

	// Record a write
	writeArgs := map[string]interface{}{"path": "/data/file.txt"}
	writeResult := &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent("Write successful")},
	}
	faker.recordToolEvent(ctx, "write_file", writeArgs, writeResult, "write")

	// Record a read
	readArgs := map[string]interface{}{"path": "/data/file.txt"}
	readResult := &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent("File contents here")},
	}
	faker.recordToolEvent(ctx, "read_file", readArgs, readResult, "read")

	// Verify both events in history
	allEvents, _ := sm.GetAllEvents(ctx, session.ID)
	if len(allEvents) != 2 {
		t.Fatalf("Expected 2 total events, got %d", len(allEvents))
	}

	if allEvents[0].OperationType != "write" {
		t.Error("First event should be write")
	}

	if allEvents[1].OperationType != "read" {
		t.Error("Second event should be read")
	}
}

// TestFilesystemWriteReadConsistency simulates realistic filesystem scenario
func TestFilesystemWriteReadConsistency(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, false)
	ctx := context.Background()

	instruction := `Simulate filesystem operations where:
	- Files written should appear in directory listings
	- File contents should match what was written
	- Directories created should exist in parent listings`

	session, _ := sm.CreateSession(ctx, instruction)

	// Scenario: Security analyst creating report

	// Step 1: Create directory
	mkdirEvent := &fakerSession.Event{
		SessionID: session.ID,
		ToolName:  "create_directory",
		Arguments: map[string]interface{}{
			"path": "/reports/2025-11-08",
		},
		Response: map[string]interface{}{
			"success": true,
		},
		OperationType: "write",
		Timestamp:     time.Now(),
	}
	sm.RecordEvent(ctx, mkdirEvent)

	// Step 2: Write initial report
	writeEvent := &fakerSession.Event{
		SessionID: session.ID,
		ToolName:  "write_file",
		Arguments: map[string]interface{}{
			"path":    "/reports/2025-11-08/findings.md",
			"content": "# Security Findings\n\n## Critical Issues\n- SQL Injection in /api/login",
		},
		Response: map[string]interface{}{
			"success":      true,
			"bytesWritten": 65,
		},
		OperationType: "write",
		Timestamp:     time.Now().Add(1 * time.Second),
	}
	sm.RecordEvent(ctx, writeEvent)

	// Step 3: Append to report
	appendEvent := &fakerSession.Event{
		SessionID: session.ID,
		ToolName:  "append_file",
		Arguments: map[string]interface{}{
			"path":    "/reports/2025-11-08/findings.md",
			"content": "\n- XSS vulnerability in user profile page",
		},
		Response: map[string]interface{}{
			"success": true,
		},
		OperationType: "write",
		Timestamp:     time.Now().Add(2 * time.Second),
	}
	sm.RecordEvent(ctx, appendEvent)

	// Now if agent reads the file, it should see accumulated content
	writeHistory, _ := sm.GetWriteHistory(ctx, session.ID)

	if len(writeHistory) != 3 {
		t.Fatalf("Expected 3 write operations, got %d", len(writeHistory))
	}

	// Build prompt for read_file operation
	prompt := sm.BuildWriteHistoryPrompt(writeHistory)

	// Prompt should contain context about all write operations
	mustContain := []string{
		"create_directory",
		"/reports/2025-11-08",
		"write_file",
		"findings.md",
		"SQL Injection",
		"append_file",
		"XSS vulnerability",
	}

	for _, expected := range mustContain {
		if !contains(prompt, expected) {
			t.Errorf("Expected prompt to contain %q for read synthesis context", expected)
		}
	}
}

// TestDatabaseWriteReadConsistency simulates database operations
func TestDatabaseWriteReadConsistency(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, false)
	ctx := context.Background()

	session, _ := sm.CreateSession(ctx, "Simulate database operations")

	// Insert record
	insertEvent := &fakerSession.Event{
		SessionID: session.ID,
		ToolName:  "execute_sql",
		Arguments: map[string]interface{}{
			"query": "INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com')",
		},
		Response: map[string]interface{}{
			"rowsAffected": 1,
			"insertId":     42,
		},
		OperationType: "write",
		Timestamp:     time.Now(),
	}
	sm.RecordEvent(ctx, insertEvent)

	// Update record
	updateEvent := &fakerSession.Event{
		SessionID: session.ID,
		ToolName:  "execute_sql",
		Arguments: map[string]interface{}{
			"query": "UPDATE users SET status = 'verified' WHERE id = 42",
		},
		Response: map[string]interface{}{
			"rowsAffected": 1,
		},
		OperationType: "write",
		Timestamp:     time.Now().Add(1 * time.Second),
	}
	sm.RecordEvent(ctx, updateEvent)

	writeHistory, _ := sm.GetWriteHistory(ctx, session.ID)
	prompt := sm.BuildWriteHistoryPrompt(writeHistory)

	// When agent queries "SELECT * FROM users WHERE id = 42",
	// synthesized response should reflect both INSERT and UPDATE
	if !contains(prompt, "INSERT INTO users") {
		t.Error("Prompt should contain insert operation")
	}

	if !contains(prompt, "Alice") {
		t.Error("Prompt should contain inserted name")
	}

	if !contains(prompt, "verified") {
		t.Error("Prompt should contain updated status")
	}
}

// TestResponseShapePreservation verifies MCP response format is maintained
func TestResponseShapePreservation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, false)
	ctx := context.Background()

	session, _ := sm.CreateSession(ctx, "Test response shapes")

	faker := &MCPFaker{
		sessionManager: sm,
		session:        session,
		debug:          false,
	}

	// Record event with complex response shape
	complexResponse := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(`{
				"instances": [
					{"id": "i-123", "state": "running", "tags": {"env": "prod"}},
					{"id": "i-456", "state": "stopped", "tags": {"env": "dev"}}
				],
				"totalCount": 2
			}`),
		},
		IsError: false,
	}

	args := map[string]interface{}{"region": "us-east-1"}
	err := faker.recordToolEvent(ctx, "list_instances", args, complexResponse, "read")
	if err != nil {
		t.Fatalf("Failed to record complex response: %v", err)
	}

	// Retrieve and verify response structure preserved
	allEvents, _ := sm.GetAllEvents(ctx, session.ID)
	if len(allEvents) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(allEvents))
	}

	// Response is stored as a string (text content joined together)
	responseStr, ok := allEvents[0].Response.(string)
	if !ok {
		t.Fatalf("Response should be stored as string, got %T", allEvents[0].Response)
	}

	if responseStr == "" {
		t.Fatal("Response string should not be empty")
	}

	// Verify JSON structure is preserved in the string
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(responseStr), &parsed); err != nil {
		t.Fatalf("Response text should be valid JSON: %v", err)
	}

	if parsed["totalCount"] != float64(2) {
		t.Error("JSON structure not preserved correctly")
	}
}
