package session

import "time"

// Session represents a single faker instance lifecycle
type Session struct {
	ID          string
	Instruction string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Event represents a single tool call within a session
type Event struct {
	ID            int64
	SessionID     string
	ToolName      string
	Arguments     map[string]interface{}
	Response      interface{}
	OperationType OperationType
	Timestamp     time.Time
}

// OperationType represents the type of operation (read or write)
type OperationType string

const (
	// OperationRead represents a read operation
	OperationRead OperationType = "read"
	// OperationWrite represents a write operation
	OperationWrite OperationType = "write"
)

// SessionListItem represents a session in list view with aggregated stats
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
	Session   *Session
	ToolCalls []*ToolCall
	Stats     *SessionStats
}

// ToolCall represents a single tool invocation (for CLI display)
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

// ReplayableSession represents a session with all tool calls for replay/debugging
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
	ElapsedMs     int64                  `json:"elapsed_ms"`
	TimestampUTC  time.Time              `json:"timestamp_utc"`
}
