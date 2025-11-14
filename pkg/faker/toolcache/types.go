package toolcache

import "time"

// CachedTool represents a tool schema cached in the database
type CachedTool struct {
	ID         int64
	FakerID    string
	ToolName   string
	ToolSchema string // JSON-encoded mcp.Tool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
