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
