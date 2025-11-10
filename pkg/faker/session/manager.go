package session

import (
	"context"
	"database/sql"
)

// Manager handles session operations
type Manager interface {
	CreateSession(ctx context.Context, instruction string) (*Session, error)
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	RecordEvent(ctx context.Context, event *Event) error
	GetWriteHistory(ctx context.Context, sessionID string) ([]*Event, error)
	GetAllEvents(ctx context.Context, sessionID string) ([]*Event, error)
	BuildWriteHistoryPrompt(events []*Event) string

	// CLI support methods
	ListSessions(ctx context.Context) ([]*SessionListItem, error)
	GetSessionCount(ctx context.Context) (int, error)
	ClearAllSessions(ctx context.Context) (int, error)
	GetSessionDetails(ctx context.Context, sessionID string) (*SessionDetails, error)
	GetMetrics(ctx context.Context) (*SessionMetrics, error)
	ExportReplayableSessionJSON(ctx context.Context, sessionID string) ([]byte, error)
}

// manager implements Manager interface
type manager struct {
	store *store
	debug bool
}

// NewManager creates a new session manager
func NewManager(db *sql.DB, debug bool) Manager {
	return &manager{
		store: &store{db: db},
		debug: debug,
	}
}

// CreateSession creates a new faker session
func (m *manager) CreateSession(ctx context.Context, instruction string) (*Session, error) {
	return m.store.createSession(ctx, instruction, m.debug)
}

// GetSession retrieves a session by ID
func (m *manager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	return m.store.getSession(ctx, sessionID)
}

// DeleteSession deletes a session and all its events (CASCADE)
func (m *manager) DeleteSession(ctx context.Context, sessionID string) error {
	return m.store.deleteSession(ctx, sessionID, m.debug)
}

// RecordEvent records a tool call event in the session
func (m *manager) RecordEvent(ctx context.Context, event *Event) error {
	return m.store.recordEvent(ctx, event, m.debug)
}

// GetWriteHistory retrieves all write events for a session in chronological order
func (m *manager) GetWriteHistory(ctx context.Context, sessionID string) ([]*Event, error) {
	opType := OperationWrite
	return m.store.getEvents(ctx, sessionID, &opType)
}

// GetAllEvents retrieves all events for a session (both read and write)
func (m *manager) GetAllEvents(ctx context.Context, sessionID string) ([]*Event, error) {
	return m.store.getEvents(ctx, sessionID, nil)
}

// BuildWriteHistoryPrompt formats write history for AI prompts
func (m *manager) BuildWriteHistoryPrompt(events []*Event) string {
	builder := NewHistoryBuilder(events)
	return builder.BuildWriteHistoryPrompt()
}

// ListSessions retrieves all sessions with aggregated stats
func (m *manager) ListSessions(ctx context.Context) ([]*SessionListItem, error) {
	return m.store.listSessions(ctx)
}

// GetSessionCount returns the total number of sessions
func (m *manager) GetSessionCount(ctx context.Context) (int, error) {
	return m.store.getSessionCount(ctx)
}

// ClearAllSessions deletes all sessions and returns the count deleted
func (m *manager) ClearAllSessions(ctx context.Context) (int, error) {
	return m.store.clearAllSessions(ctx, m.debug)
}

// GetSessionDetails retrieves complete session information with all tool calls and stats
func (m *manager) GetSessionDetails(ctx context.Context, sessionID string) (*SessionDetails, error) {
	return m.store.getSessionDetails(ctx, sessionID)
}

// GetMetrics retrieves aggregated metrics across all sessions
func (m *manager) GetMetrics(ctx context.Context) (*SessionMetrics, error) {
	return m.store.getMetrics(ctx)
}

// ExportReplayableSessionJSON exports a session in replayable JSON format
func (m *manager) ExportReplayableSessionJSON(ctx context.Context, sessionID string) ([]byte, error) {
	return m.store.exportReplayableSessionJSON(ctx, sessionID)
}
