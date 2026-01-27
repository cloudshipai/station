package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"station/internal/db"

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

	// Lock for SQLite write operation
	db.SQLiteWriteMutex.Lock()
	defer db.SQLiteWriteMutex.Unlock()

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
		fmt.Fprintf(os.Stderr, "[FAKER SESSION] Created session %s: %s\n", session.ID, instruction)
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

	// Lock for SQLite write operation
	db.SQLiteWriteMutex.Lock()
	defer db.SQLiteWriteMutex.Unlock()

	result, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[FAKER SESSION] Deleted session %s\n", sessionID)
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

	// Lock for SQLite write operations (INSERT + UPDATE)
	db.SQLiteWriteMutex.Lock()
	defer db.SQLiteWriteMutex.Unlock()

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
		fmt.Fprintf(os.Stderr, "[FAKER SESSION] Recorded %s event: %s (%s)\n",
			event.OperationType, event.ToolName, event.SessionID)
	}

	// Update session timestamp (still under mutex lock)
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

// listSessions retrieves all sessions with aggregated stats, ordered by creation time (newest first)
func (s *store) listSessions(ctx context.Context) ([]*SessionListItem, error) {
	query := `
		SELECT
			fs.id,
			fs.instruction,
			fs.created_at,
			fs.updated_at,
			COUNT(fe.id) as tool_calls,
			COALESCE(MAX(fe.timestamp), fs.created_at) as last_event_time
		FROM faker_sessions fs
		LEFT JOIN faker_events fe ON fe.session_id = fs.id
		GROUP BY fs.id
		ORDER BY fs.created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*SessionListItem
	for rows.Next() {
		session := &SessionListItem{}
		var lastEventTimeStr string
		err := rows.Scan(
			&session.ID,
			&session.Instruction,
			&session.CreatedAt,
			&session.UpdatedAt,
			&session.ToolCallCount,
			&lastEventTimeStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session row: %w", err)
		}

		// Parse last event time and calculate duration
		lastEventTime, err := time.Parse("2006-01-02 15:04:05-07:00", lastEventTimeStr)
		if err != nil {
			// Try alternative format
			lastEventTime, err = time.Parse("2006-01-02 15:04:05", lastEventTimeStr)
			if err != nil {
				// If parsing fails, use session creation time
				lastEventTime = session.CreatedAt
			}
		}
		session.Duration = lastEventTime.Sub(session.CreatedAt)

		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return sessions, nil
}

// getSessionCount returns the total number of sessions
func (s *store) getSessionCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM faker_sessions").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get session count: %w", err)
	}
	return count, nil
}

// clearAllSessions deletes all sessions and returns the count deleted
func (s *store) clearAllSessions(ctx context.Context, debug bool) (int, error) {
	// Lock for SQLite write operation
	db.SQLiteWriteMutex.Lock()
	defer db.SQLiteWriteMutex.Unlock()

	result, err := s.db.ExecContext(ctx, "DELETE FROM faker_sessions")
	if err != nil {
		return 0, fmt.Errorf("failed to clear sessions: %w", err)
	}

	rows, _ := result.RowsAffected()
	if debug {
		fmt.Fprintf(os.Stderr, "[FAKER SESSION] Cleared %d sessions\n", rows)
	}

	return int(rows), nil
}

// getSessionDetails retrieves complete session information with all tool calls and stats
func (s *store) getSessionDetails(ctx context.Context, sessionID string) (*SessionDetails, error) {
	// Get session
	session := &Session{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, instruction, created_at, updated_at
		FROM faker_sessions WHERE id = ?
	`, sessionID).Scan(&session.ID, &session.Instruction, &session.CreatedAt, &session.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Get all tool calls
	rows, err := s.db.QueryContext(ctx, `
		SELECT tool_name, arguments, response, operation_type, timestamp
		FROM faker_events
		WHERE session_id = ?
		ORDER BY timestamp ASC
	`, sessionID)

	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	toolCalls := []*ToolCall{}
	uniqueTools := make(map[string]bool)
	readCount := 0
	writeCount := 0

	for rows.Next() {
		tc := &ToolCall{}
		var argsJSON, responseJSON string

		err := rows.Scan(&tc.ToolName, &argsJSON, &responseJSON, &tc.OperationType, &tc.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tool call: %w", err)
		}

		// Parse JSON arguments
		if err := json.Unmarshal([]byte(argsJSON), &tc.Arguments); err != nil {
			return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
		}

		// Parse JSON response
		if err := json.Unmarshal([]byte(responseJSON), &tc.Response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		toolCalls = append(toolCalls, tc)
		uniqueTools[tc.ToolName] = true

		if tc.OperationType == "read" {
			readCount++
		} else if tc.OperationType == "write" {
			writeCount++
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Calculate duration
	var duration time.Duration
	if len(toolCalls) > 0 {
		duration = toolCalls[len(toolCalls)-1].Timestamp.Sub(toolCalls[0].Timestamp)
	}

	// Build stats
	stats := &SessionStats{
		TotalToolCalls: len(toolCalls),
		ReadCalls:      readCount,
		WriteCalls:     writeCount,
		UniqueTools:    len(uniqueTools),
		Duration:       duration,
	}

	return &SessionDetails{
		Session:   session,
		ToolCalls: toolCalls,
		Stats:     stats,
	}, nil
}

// getMetrics retrieves aggregated metrics across all sessions
func (s *store) getMetrics(ctx context.Context) (*SessionMetrics, error) {
	metrics := &SessionMetrics{}

	// Count total sessions
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM faker_sessions`).Scan(&metrics.TotalSessions)
	if err != nil {
		return nil, fmt.Errorf("failed to count sessions: %w", err)
	}

	// Count sessions in last 24 hours
	last24h := time.Now().Add(-24 * time.Hour)
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM faker_sessions WHERE created_at > ?
	`, last24h).Scan(&metrics.SessionsLast24h)
	if err != nil {
		return nil, fmt.Errorf("failed to count 24h sessions: %w", err)
	}

	// Count sessions in last 7 days
	last7d := time.Now().Add(-7 * 24 * time.Hour)
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM faker_sessions WHERE created_at > ?
	`, last7d).Scan(&metrics.SessionsLast7d)
	if err != nil {
		return nil, fmt.Errorf("failed to count 7d sessions: %w", err)
	}

	// Count total tool calls
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM faker_events`).Scan(&metrics.TotalToolCalls)
	if err != nil {
		return nil, fmt.Errorf("failed to count tool calls: %w", err)
	}

	// Calculate average calls per session
	if metrics.TotalSessions > 0 {
		metrics.AvgCallsPerSession = float64(metrics.TotalToolCalls) / float64(metrics.TotalSessions)
	}

	// Get top 10 most called tools
	toolRows, err := s.db.QueryContext(ctx, `
		SELECT tool_name, COUNT(*) as count
		FROM faker_events
		GROUP BY tool_name
		ORDER BY count DESC
		LIMIT 10
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query top tools: %w", err)
	}
	defer toolRows.Close()

	topTools := []ToolUsage{}
	for toolRows.Next() {
		var tool ToolUsage
		if err := toolRows.Scan(&tool.ToolName, &tool.Count); err != nil {
			return nil, fmt.Errorf("failed to scan tool usage: %w", err)
		}
		if metrics.TotalToolCalls > 0 {
			tool.Percent = (float64(tool.Count) / float64(metrics.TotalToolCalls)) * 100
		}
		topTools = append(topTools, tool)
	}
	metrics.TopTools = topTools

	// Get 5 most recent sessions
	recentSessions, err := s.listSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list recent sessions: %w", err)
	}
	if len(recentSessions) > 5 {
		metrics.RecentSessions = recentSessions[:5]
	} else {
		metrics.RecentSessions = recentSessions
	}

	return metrics, nil
}

// exportReplayableSessionJSON exports a session in replayable JSON format
func (s *store) exportReplayableSessionJSON(ctx context.Context, sessionID string) ([]byte, error) {
	// Get session details
	details, err := s.getSessionDetails(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session details: %w", err)
	}

	if details.Session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Build replayable tool calls with sequence and timing
	replayableCalls := make([]ReplayableToolCall, len(details.ToolCalls))
	var startTime time.Time
	if len(details.ToolCalls) > 0 {
		startTime = details.ToolCalls[0].Timestamp
	}

	for i, call := range details.ToolCalls {
		elapsedMs := call.Timestamp.Sub(startTime).Milliseconds()
		replayableCalls[i] = ReplayableToolCall{
			Sequence:      i + 1,
			ToolName:      call.ToolName,
			Arguments:     call.Arguments,
			Response:      call.Response,
			OperationType: call.OperationType,
			TimestampUTC:  call.Timestamp,
			ElapsedMs:     elapsedMs,
		}
	}

	replayable := &ReplayableSession{
		SessionID:   details.Session.ID,
		Instruction: details.Session.Instruction,
		CreatedAt:   details.Session.CreatedAt,
		ToolCalls:   replayableCalls,
		Stats:       details.Stats,
	}

	return json.MarshalIndent(replayable, "", "  ")
}
