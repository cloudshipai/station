package session

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE faker_sessions (
			id TEXT PRIMARY KEY,
			instruction TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE faker_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT,
			tool_name TEXT,
			arguments TEXT,
			response TEXT,
			operation_type TEXT,
			timestamp DATETIME,
			FOREIGN KEY (session_id) REFERENCES faker_sessions(id) ON DELETE CASCADE
		)
	`)
	require.NoError(t, err)

	return db
}

func TestSessionManager_CreateSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewManager(db, false)
	ctx := context.Background()

	session, err := mgr.CreateSession(ctx, "test instruction")
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, "test instruction", session.Instruction)
	assert.False(t, session.CreatedAt.IsZero())
	assert.False(t, session.UpdatedAt.IsZero())
}

func TestSessionManager_GetSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewManager(db, false)
	ctx := context.Background()

	// Create a session
	created, err := mgr.CreateSession(ctx, "test instruction")
	require.NoError(t, err)

	// Retrieve it
	retrieved, err := mgr.GetSession(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.Instruction, retrieved.Instruction)
}

func TestSessionManager_RecordEvent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewManager(db, false)
	ctx := context.Background()

	// Create a session
	session, err := mgr.CreateSession(ctx, "test instruction")
	require.NoError(t, err)

	// Record a write event
	event := &Event{
		SessionID:     session.ID,
		ToolName:      "write_file",
		Arguments:     map[string]interface{}{"path": "/tmp/test.txt", "content": "hello"},
		Response:      map[string]interface{}{"success": true},
		OperationType: OperationWrite,
		Timestamp:     time.Now(),
	}

	err = mgr.RecordEvent(ctx, event)
	require.NoError(t, err)
	assert.NotZero(t, event.ID)
}

func TestSessionManager_GetWriteHistory(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewManager(db, false)
	ctx := context.Background()

	// Create a session
	session, err := mgr.CreateSession(ctx, "test instruction")
	require.NoError(t, err)

	// Record write events
	writeEvent1 := &Event{
		SessionID:     session.ID,
		ToolName:      "write_file",
		Arguments:     map[string]interface{}{"path": "/tmp/test1.txt"},
		Response:      map[string]interface{}{"success": true},
		OperationType: OperationWrite,
		Timestamp:     time.Now(),
	}
	err = mgr.RecordEvent(ctx, writeEvent1)
	require.NoError(t, err)

	// Record a read event (should not appear in write history)
	readEvent := &Event{
		SessionID:     session.ID,
		ToolName:      "read_file",
		Arguments:     map[string]interface{}{"path": "/tmp/test1.txt"},
		Response:      map[string]interface{}{"content": "hello"},
		OperationType: OperationRead,
		Timestamp:     time.Now(),
	}
	err = mgr.RecordEvent(ctx, readEvent)
	require.NoError(t, err)

	// Get write history
	writeHistory, err := mgr.GetWriteHistory(ctx, session.ID)
	require.NoError(t, err)
	assert.Len(t, writeHistory, 1)
	assert.Equal(t, "write_file", writeHistory[0].ToolName)
}

func TestSessionManager_GetAllEvents(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewManager(db, false)
	ctx := context.Background()

	// Create a session
	session, err := mgr.CreateSession(ctx, "test instruction")
	require.NoError(t, err)

	// Record multiple events
	events := []*Event{
		{
			SessionID:     session.ID,
			ToolName:      "write_file",
			Arguments:     map[string]interface{}{"path": "/tmp/test1.txt"},
			Response:      map[string]interface{}{"success": true},
			OperationType: OperationWrite,
			Timestamp:     time.Now(),
		},
		{
			SessionID:     session.ID,
			ToolName:      "read_file",
			Arguments:     map[string]interface{}{"path": "/tmp/test1.txt"},
			Response:      map[string]interface{}{"content": "hello"},
			OperationType: OperationRead,
			Timestamp:     time.Now(),
		},
	}

	for _, event := range events {
		err = mgr.RecordEvent(ctx, event)
		require.NoError(t, err)
	}

	// Get all events
	allEvents, err := mgr.GetAllEvents(ctx, session.ID)
	require.NoError(t, err)
	assert.Len(t, allEvents, 2)
}

func TestSessionManager_DeleteSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mgr := NewManager(db, false)
	ctx := context.Background()

	// Create a session
	session, err := mgr.CreateSession(ctx, "test instruction")
	require.NoError(t, err)

	// Delete it
	err = mgr.DeleteSession(ctx, session.ID)
	require.NoError(t, err)

	// Verify it's gone
	_, err = mgr.GetSession(ctx, session.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestHistoryBuilder_BuildWriteHistoryPrompt(t *testing.T) {
	events := []*Event{
		{
			ToolName:      "write_file",
			Arguments:     map[string]interface{}{"path": "/tmp/test.txt"},
			Response:      map[string]interface{}{"success": true},
			OperationType: OperationWrite,
			Timestamp:     time.Now(),
		},
	}

	builder := NewHistoryBuilder(events)
	prompt := builder.BuildWriteHistoryPrompt()

	assert.Contains(t, prompt, "Previous Write Operations")
	assert.Contains(t, prompt, "write_file")
	assert.Contains(t, prompt, "/tmp/test.txt")
}

func TestHistoryBuilder_BuildSummary(t *testing.T) {
	events := []*Event{
		{
			ToolName:      "write_file",
			OperationType: OperationWrite,
			Timestamp:     time.Now(),
		},
		{
			ToolName:      "read_file",
			OperationType: OperationRead,
			Timestamp:     time.Now(),
		},
		{
			ToolName:      "write_file",
			OperationType: OperationWrite,
			Timestamp:     time.Now(),
		},
	}

	builder := NewHistoryBuilder(events)
	summary := builder.BuildSummary()

	assert.Contains(t, summary, "Total operations: 3")
	assert.Contains(t, summary, "Write operations: 2")
	assert.Contains(t, summary, "Read operations: 1")
	assert.Contains(t, summary, "write_file: 2")
	assert.Contains(t, summary, "read_file: 1")
}
