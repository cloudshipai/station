package faker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FakerSession represents a single faker instance lifecycle
type FakerSession struct {
	ID          string
	Instruction string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// FakerEvent represents a single tool call within a session
type FakerEvent struct {
	ID            int64
	SessionID     string
	ToolName      string
	Arguments     map[string]interface{}
	Response      interface{}
	OperationType string // "read" or "write"
	Timestamp     time.Time
}

// SessionManager handles faker session persistence
type SessionManager struct {
	db    *sql.DB
	debug bool
}

// NewSessionManager creates a new session manager
func NewSessionManager(db *sql.DB, debug bool) *SessionManager {
	return &SessionManager{
		db:    db,
		debug: debug,
	}
}

// CreateSession creates a new faker session
func (sm *SessionManager) CreateSession(ctx context.Context, instruction string) (*FakerSession, error) {
	session := &FakerSession{
		ID:          uuid.New().String(),
		Instruction: instruction,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	query := `
		INSERT INTO faker_sessions (id, instruction, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`

	_, err := sm.db.ExecContext(ctx, query,
		session.ID,
		session.Instruction,
		session.CreatedAt,
		session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	if sm.debug {
		fmt.Printf("[FAKER SESSION] Created session %s: %s\n", session.ID, instruction)
	}

	return session, nil
}

// RecordEvent records a tool call event in the session
func (sm *SessionManager) RecordEvent(ctx context.Context, event *FakerEvent) error {
	// Serialize arguments and response to JSON
	argsJSON, err := json.Marshal(event.Arguments)
	if err != nil {
		return fmt.Errorf("failed to marshal arguments: %w", err)
	}

	responseJSON, err := json.Marshal(event.Response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	query := `
		INSERT INTO faker_events (session_id, tool_name, arguments, response, operation_type, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := sm.db.ExecContext(ctx, query,
		event.SessionID,
		event.ToolName,
		string(argsJSON),
		string(responseJSON),
		event.OperationType,
		event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to record event: %w", err)
	}

	id, _ := result.LastInsertId()
	event.ID = id

	if sm.debug {
		fmt.Printf("[FAKER SESSION] Recorded %s event: %s (%s)\n",
			event.OperationType, event.ToolName, event.SessionID)
	}

	// Update session timestamp
	_, err = sm.db.ExecContext(ctx,
		`UPDATE faker_sessions SET updated_at = ? WHERE id = ?`,
		time.Now(), event.SessionID)
	if err != nil {
		return fmt.Errorf("failed to update session timestamp: %w", err)
	}

	return nil
}

// GetWriteHistory retrieves all write events for a session in chronological order
func (sm *SessionManager) GetWriteHistory(ctx context.Context, sessionID string) ([]*FakerEvent, error) {
	query := `
		SELECT id, session_id, tool_name, arguments, response, operation_type, timestamp
		FROM faker_events
		WHERE session_id = ? AND operation_type = 'write'
		ORDER BY timestamp ASC
	`

	rows, err := sm.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query write history: %w", err)
	}
	defer rows.Close()

	var events []*FakerEvent
	for rows.Next() {
		event := &FakerEvent{}
		var argsJSON, responseJSON string

		err := rows.Scan(
			&event.ID,
			&event.SessionID,
			&event.ToolName,
			&argsJSON,
			&responseJSON,
			&event.OperationType,
			&event.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		// Deserialize JSON
		if err := json.Unmarshal([]byte(argsJSON), &event.Arguments); err != nil {
			return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
		}

		if err := json.Unmarshal([]byte(responseJSON), &event.Response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return events, nil
}

// GetAllEvents retrieves all events for a session (both read and write)
func (sm *SessionManager) GetAllEvents(ctx context.Context, sessionID string) ([]*FakerEvent, error) {
	query := `
		SELECT id, session_id, tool_name, arguments, response, operation_type, timestamp
		FROM faker_events
		WHERE session_id = ?
		ORDER BY timestamp ASC
	`

	rows, err := sm.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []*FakerEvent
	for rows.Next() {
		event := &FakerEvent{}
		var argsJSON, responseJSON string

		err := rows.Scan(
			&event.ID,
			&event.SessionID,
			&event.ToolName,
			&argsJSON,
			&responseJSON,
			&event.OperationType,
			&event.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		// Deserialize JSON
		if err := json.Unmarshal([]byte(argsJSON), &event.Arguments); err != nil {
			return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
		}

		if err := json.Unmarshal([]byte(responseJSON), &event.Response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return events, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(ctx context.Context, sessionID string) (*FakerSession, error) {
	query := `
		SELECT id, instruction, created_at, updated_at
		FROM faker_sessions
		WHERE id = ?
	`

	session := &FakerSession{}
	err := sm.db.QueryRowContext(ctx, query, sessionID).Scan(
		&session.ID,
		&session.Instruction,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// DeleteSession deletes a session and all its events (CASCADE)
func (sm *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM faker_sessions WHERE id = ?`

	result, err := sm.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if sm.debug {
		fmt.Printf("[FAKER SESSION] Deleted session %s\n", sessionID)
	}

	return nil
}

// BuildWriteHistoryPrompt formats write history for AI prompts
func (sm *SessionManager) BuildWriteHistoryPrompt(events []*FakerEvent) string {
	if len(events) == 0 {
		return "No previous write operations."
	}

	var prompt string
	prompt += "Previous Write Operations (in chronological order):\n\n"

	for i, event := range events {
		argsJSON, _ := json.MarshalIndent(event.Arguments, "", "  ")
		responseJSON, _ := json.MarshalIndent(event.Response, "", "  ")

		prompt += fmt.Sprintf("%d. [%s] %s\n",
			i+1,
			event.Timestamp.Format("2006-01-02 15:04:05"),
			event.ToolName,
		)
		prompt += fmt.Sprintf("   Arguments: %s\n", string(argsJSON))
		prompt += fmt.Sprintf("   Response: %s\n\n", string(responseJSON))
	}

	return prompt
}
