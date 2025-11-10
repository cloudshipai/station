package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// store handles database operations for sessions
type store struct {
	db *sql.DB
}

// createSession inserts a new session into the database
func (s *store) createSession(ctx context.Context, instruction string, debug bool) (*Session, error) {
	session := &Session{
		ID:          uuid.New().String(),
		Instruction: instruction,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	query := `
		INSERT INTO faker_sessions (id, instruction, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		session.ID,
		session.Instruction,
		session.CreatedAt,
		session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	if debug {
		fmt.Printf("[FAKER SESSION] Created session %s: %s\n", session.ID, instruction)
	}

	return session, nil
}

// getSession retrieves a session by ID
func (s *store) getSession(ctx context.Context, sessionID string) (*Session, error) {
	query := `
		SELECT id, instruction, created_at, updated_at
		FROM faker_sessions
		WHERE id = ?
	`

	session := &Session{}
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
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

// deleteSession deletes a session and all its events (CASCADE)
func (s *store) deleteSession(ctx context.Context, sessionID string, debug bool) error {
	query := `DELETE FROM faker_sessions WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if debug {
		fmt.Printf("[FAKER SESSION] Deleted session %s\n", sessionID)
	}

	return nil
}

// recordEvent inserts a tool call event into the database
func (s *store) recordEvent(ctx context.Context, event *Event, debug bool) error {
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

	result, err := s.db.ExecContext(ctx, query,
		event.SessionID,
		event.ToolName,
		string(argsJSON),
		string(responseJSON),
		string(event.OperationType),
		event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to record event: %w", err)
	}

	id, _ := result.LastInsertId()
	event.ID = id

	if debug {
		fmt.Printf("[FAKER SESSION] Recorded %s event: %s (%s)\n",
			event.OperationType, event.ToolName, event.SessionID)
	}

	// Update session timestamp
	_, err = s.db.ExecContext(ctx,
		`UPDATE faker_sessions SET updated_at = ? WHERE id = ?`,
		time.Now(), event.SessionID)
	if err != nil {
		return fmt.Errorf("failed to update session timestamp: %w", err)
	}

	return nil
}

// getEvents retrieves events for a session with optional operation type filter
func (s *store) getEvents(ctx context.Context, sessionID string, opType *OperationType) ([]*Event, error) {
	var query string
	var args []interface{}

	if opType != nil {
		query = `
			SELECT id, session_id, tool_name, arguments, response, operation_type, timestamp
			FROM faker_events
			WHERE session_id = ? AND operation_type = ?
			ORDER BY timestamp ASC
		`
		args = []interface{}{sessionID, string(*opType)}
	} else {
		query = `
			SELECT id, session_id, tool_name, arguments, response, operation_type, timestamp
			FROM faker_events
			WHERE session_id = ?
			ORDER BY timestamp ASC
		`
		args = []interface{}{sessionID}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		event := &Event{}
		var argsJSON, responseJSON, opTypeStr string

		err := rows.Scan(
			&event.ID,
			&event.SessionID,
			&event.ToolName,
			&argsJSON,
			&responseJSON,
			&opTypeStr,
			&event.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		event.OperationType = OperationType(opTypeStr)

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
