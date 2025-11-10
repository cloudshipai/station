package faker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// SessionListItem represents a session in list view
type SessionListItem struct {
	ID            string
	Instruction   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Duration      time.Duration
	ToolCallCount int
}

// SessionDetails represents complete session information with all tool calls
type SessionDetails struct {
	Session   *FakerSession
	ToolCalls []*ToolCall
	Stats     *SessionStats
}

// ToolCall represents a single tool invocation
type ToolCall struct {
	ToolName      string
	Timestamp     time.Time
	Arguments     map[string]interface{}
	Response      interface{}
	OperationType string // "read" or "write"
}

// SessionStats represents statistics for a session
type SessionStats struct {
	TotalToolCalls int
	ReadCalls      int
	WriteCalls     int
	UniqueTools    int
	Duration       time.Duration
}

// SessionMetrics represents aggregated metrics across sessions
type SessionMetrics struct {
	TotalSessions      int
	SessionsLast24h    int
	SessionsLast7d     int
	TotalToolCalls     int
	AvgCallsPerSession float64
	TopTools           []ToolUsage
	RecentSessions     []*SessionListItem
}

// ToolUsage represents tool usage statistics
type ToolUsage struct {
	ToolName string
	Count    int
	Percent  float64
}

// SessionService provides high-level session operations
type SessionService struct {
	db *sql.DB
}

// NewSessionService creates a new session service
func NewSessionService(db *sql.DB) *SessionService {
	return &SessionService{db: db}
}

// ListSessions returns all sessions with basic info
func (s *SessionService) ListSessions(ctx context.Context) ([]*SessionListItem, error) {
	query := `
		SELECT
			fs.id,
			fs.instruction,
			fs.created_at,
			fs.updated_at,
			COUNT(fe.id) as tool_calls
		FROM faker_sessions fs
		LEFT JOIN faker_events fe ON fe.session_id = fs.id
		GROUP BY fs.id
		ORDER BY fs.created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*SessionListItem
	for rows.Next() {
		sess := &SessionListItem{}
		err := rows.Scan(
			&sess.ID,
			&sess.Instruction,
			&sess.CreatedAt,
			&sess.UpdatedAt,
			&sess.ToolCallCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		sess.Duration = sess.UpdatedAt.Sub(sess.CreatedAt)
		sessions = append(sessions, sess)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return sessions, nil
}

// GetSessionDetails returns complete session information
func (s *SessionService) GetSessionDetails(ctx context.Context, sessionID string) (*SessionDetails, error) {
	// Get session
	session := &FakerSession{}
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

		// Deserialize JSON
		if err := json.Unmarshal([]byte(argsJSON), &tc.Arguments); err != nil {
			// If unmarshal fails, keep as string
			tc.Arguments = map[string]interface{}{"_raw": argsJSON}
		}

		if err := json.Unmarshal([]byte(responseJSON), &tc.Response); err != nil {
			// If unmarshal fails, keep as string
			tc.Response = responseJSON
		}

		toolCalls = append(toolCalls, tc)
		uniqueTools[tc.ToolName] = true

		if tc.OperationType == "read" {
			readCount++
		} else {
			writeCount++
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	// Calculate stats
	stats := &SessionStats{
		TotalToolCalls: len(toolCalls),
		ReadCalls:      readCount,
		WriteCalls:     writeCount,
		UniqueTools:    len(uniqueTools),
		Duration:       session.UpdatedAt.Sub(session.CreatedAt),
	}

	return &SessionDetails{
		Session:   session,
		ToolCalls: toolCalls,
		Stats:     stats,
	}, nil
}

// DeleteSession deletes a session and all its events
func (s *SessionService) DeleteSession(ctx context.Context, sessionID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM faker_sessions WHERE id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return nil
}

// ClearAllSessions deletes all sessions
func (s *SessionService) ClearAllSessions(ctx context.Context) (int, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM faker_sessions`)
	if err != nil {
		return 0, fmt.Errorf("failed to clear sessions: %w", err)
	}

	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// GetMetrics returns aggregated metrics across all sessions
func (s *SessionService) GetMetrics(ctx context.Context) (*SessionMetrics, error) {
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
		tu := ToolUsage{}
		if err := toolRows.Scan(&tu.ToolName, &tu.Count); err != nil {
			return nil, fmt.Errorf("failed to scan tool usage: %w", err)
		}
		if metrics.TotalToolCalls > 0 {
			tu.Percent = float64(tu.Count) / float64(metrics.TotalToolCalls) * 100
		}
		topTools = append(topTools, tu)
	}
	metrics.TopTools = topTools

	// Get 5 most recent sessions
	recentRows, err := s.db.QueryContext(ctx, `
		SELECT
			fs.id,
			fs.instruction,
			fs.created_at,
			fs.updated_at,
			COUNT(fe.id) as tool_calls
		FROM faker_sessions fs
		LEFT JOIN faker_events fe ON fe.session_id = fs.id
		GROUP BY fs.id
		ORDER BY fs.created_at DESC
		LIMIT 5
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent sessions: %w", err)
	}
	defer recentRows.Close()

	recentSessions := []*SessionListItem{}
	for recentRows.Next() {
		sess := &SessionListItem{}
		err := recentRows.Scan(&sess.ID, &sess.Instruction, &sess.CreatedAt, &sess.UpdatedAt, &sess.ToolCallCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recent session: %w", err)
		}
		sess.Duration = sess.UpdatedAt.Sub(sess.CreatedAt)
		recentSessions = append(recentSessions, sess)
	}
	metrics.RecentSessions = recentSessions

	return metrics, nil
}

// GetToolUsageStats returns tool usage statistics
func (s *SessionService) GetToolUsageStats(ctx context.Context, limit int) ([]ToolUsage, error) {
	if limit <= 0 {
		limit = 10
	}

	// Get total tool calls for percentage calculation
	var totalCalls int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM faker_events`).Scan(&totalCalls)
	if err != nil {
		return nil, fmt.Errorf("failed to count total calls: %w", err)
	}

	query := `
		SELECT tool_name, COUNT(*) as count
		FROM faker_events
		GROUP BY tool_name
		ORDER BY count DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query tool usage: %w", err)
	}
	defer rows.Close()

	var toolUsage []ToolUsage
	for rows.Next() {
		tu := ToolUsage{}
		if err := rows.Scan(&tu.ToolName, &tu.Count); err != nil {
			return nil, fmt.Errorf("failed to scan tool usage: %w", err)
		}
		if totalCalls > 0 {
			tu.Percent = float64(tu.Count) / float64(totalCalls) * 100
		}
		toolUsage = append(toolUsage, tu)
	}

	return toolUsage, nil
}

// GetSessionsByTimeRange returns sessions within a time range
func (s *SessionService) GetSessionsByTimeRange(ctx context.Context, start, end time.Time) ([]*SessionListItem, error) {
	query := `
		SELECT
			fs.id,
			fs.instruction,
			fs.created_at,
			fs.updated_at,
			COUNT(fe.id) as tool_calls
		FROM faker_sessions fs
		LEFT JOIN faker_events fe ON fe.session_id = fs.id
		WHERE fs.created_at BETWEEN ? AND ?
		GROUP BY fs.id
		ORDER BY fs.created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions by time range: %w", err)
	}
	defer rows.Close()

	var sessions []*SessionListItem
	for rows.Next() {
		sess := &SessionListItem{}
		err := rows.Scan(&sess.ID, &sess.Instruction, &sess.CreatedAt, &sess.UpdatedAt, &sess.ToolCallCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sess.Duration = sess.UpdatedAt.Sub(sess.CreatedAt)
		sessions = append(sessions, sess)
	}

	return sessions, nil
}

// GetToolCallTimeline returns chronological timeline of tool calls
func (s *SessionService) GetToolCallTimeline(ctx context.Context, sessionID string) ([]map[string]interface{}, error) {
	query := `
		SELECT tool_name, operation_type, timestamp
		FROM faker_events
		WHERE session_id = ?
		ORDER BY timestamp ASC
	`

	rows, err := s.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}
	defer rows.Close()

	timeline := []map[string]interface{}{}
	for rows.Next() {
		var toolName, opType string
		var timestamp time.Time

		if err := rows.Scan(&toolName, &opType, &timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan timeline entry: %w", err)
		}

		timeline = append(timeline, map[string]interface{}{
			"tool_name":      toolName,
			"operation_type": opType,
			"timestamp":      timestamp.Format("15:04:05"),
			"elapsed":        timestamp.Sub(time.Time{}).String(),
		})
	}

	return timeline, nil
}

// ExportSessionJSON exports session details as JSON
func (s *SessionService) ExportSessionJSON(ctx context.Context, sessionID string) ([]byte, error) {
	details, err := s.GetSessionDetails(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(details, "", "  ")
}

// GetSessionCount returns total number of sessions
func (s *SessionService) GetSessionCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM faker_sessions`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions: %w", err)
	}
	return count, nil
}

// GroupSessionsByDate groups sessions by date
func (s *SessionService) GroupSessionsByDate(ctx context.Context, days int) (map[string]int, error) {
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

	query := `
		SELECT DATE(created_at) as date, COUNT(*) as count
		FROM faker_sessions
		WHERE created_at > ?
		GROUP BY DATE(created_at)
		ORDER BY date DESC
	`

	rows, err := s.db.QueryContext(ctx, query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to group sessions by date: %w", err)
	}
	defer rows.Close()

	grouped := make(map[string]int)
	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			return nil, fmt.Errorf("failed to scan date group: %w", err)
		}
		grouped[date] = count
	}

	return grouped, nil
}

// GetAverageDuration calculates average session duration
func (s *SessionService) GetAverageDuration(ctx context.Context) (time.Duration, error) {
	query := `
		SELECT AVG(CAST((julianday(updated_at) - julianday(created_at)) * 86400000 AS INTEGER)) as avg_duration_ms
		FROM faker_sessions
	`

	var avgMs sql.NullInt64
	err := s.db.QueryRowContext(ctx, query).Scan(&avgMs)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate average duration: %w", err)
	}

	if !avgMs.Valid {
		return 0, nil
	}

	return time.Duration(avgMs.Int64) * time.Millisecond, nil
}

// SearchSessions searches sessions by instruction content
func (s *SessionService) SearchSessions(ctx context.Context, searchTerm string) ([]*SessionListItem, error) {
	query := `
		SELECT
			fs.id,
			fs.instruction,
			fs.created_at,
			fs.updated_at,
			COUNT(fe.id) as tool_calls
		FROM faker_sessions fs
		LEFT JOIN faker_events fe ON fe.session_id = fs.id
		WHERE fs.instruction LIKE ?
		GROUP BY fs.id
		ORDER BY fs.created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, "%"+searchTerm+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to search sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*SessionListItem
	for rows.Next() {
		sess := &SessionListItem{}
		err := rows.Scan(&sess.ID, &sess.Instruction, &sess.CreatedAt, &sess.UpdatedAt, &sess.ToolCallCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sess.Duration = sess.UpdatedAt.Sub(sess.CreatedAt)
		sessions = append(sessions, sess)
	}

	return sessions, nil
}

// SortBy defines sort criteria
type SortBy string

const (
	SortByNewest    SortBy = "newest"
	SortByOldest    SortBy = "oldest"
	SortByDuration  SortBy = "duration"
	SortByToolCalls SortBy = "tool_calls"
)

// SortSessions sorts sessions by criteria
func SortSessions(sessions []*SessionListItem, sortBy SortBy) {
	switch sortBy {
	case SortByNewest:
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
		})
	case SortByOldest:
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].CreatedAt.Before(sessions[j].CreatedAt)
		})
	case SortByDuration:
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].Duration > sessions[j].Duration
		})
	case SortByToolCalls:
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].ToolCallCount > sessions[j].ToolCallCount
		})
	}
}

// ReplaySession returns a session with all tool calls that can be replayed
// This allows debugging, testing, and sharing faker scenarios
type ReplayableSession struct {
	SessionID   string               `json:"session_id"`
	Instruction string               `json:"instruction"`
	CreatedAt   time.Time            `json:"created_at"`
	ToolCalls   []ReplayableToolCall `json:"tool_calls"`
	Stats       *SessionStats        `json:"stats"`
}

// ReplayableToolCall represents a tool call that can be replayed
type ReplayableToolCall struct {
	Sequence      int                    `json:"sequence"`
	ToolName      string                 `json:"tool_name"`
	Arguments     map[string]interface{} `json:"arguments"`
	Response      interface{}            `json:"response"`
	OperationType string                 `json:"operation_type"`
	Timestamp     time.Time              `json:"timestamp"`
	ElapsedMs     int64                  `json:"elapsed_ms"`
}

// GetReplayableSession retrieves a session with all details needed for replay
func (s *SessionService) GetReplayableSession(ctx context.Context, sessionID string) (*ReplayableSession, error) {
	// Get session details
	details, err := s.GetSessionDetails(ctx, sessionID)
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
			Timestamp:     call.Timestamp,
			ElapsedMs:     elapsedMs,
		}
	}

	return &ReplayableSession{
		SessionID:   details.Session.ID,
		Instruction: details.Session.Instruction,
		CreatedAt:   details.Session.CreatedAt,
		ToolCalls:   replayableCalls,
		Stats:       details.Stats,
	}, nil
}

// ExportReplayableSessionJSON exports a session in replayable JSON format
func (s *SessionService) ExportReplayableSessionJSON(ctx context.Context, sessionID string) ([]byte, error) {
	session, err := s.GetReplayableSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(session, "", "  ")
}
