package memory

import "time"

// MemoryConfig configures the memory middleware from agent frontmatter
type MemoryConfig struct {
	Sources       []string `yaml:"sources,omitempty" json:"sources,omitempty"`
	WorkspacePath string   `yaml:"workspace_path,omitempty" json:"workspace_path,omitempty"`
	AutoFlush     bool     `yaml:"auto_flush,omitempty" json:"auto_flush,omitempty"`
}

// Backend interface for file operations
type Backend interface {
	ReadFile(path string) ([]byte, error)
}

// WritableBackend extends Backend with write capability
type WritableBackend interface {
	Backend
	WriteFile(path string, data []byte, perm int) error
	MkdirAll(path string, perm int) error
}

// MemoryEntry represents a single memory item
type MemoryEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
	Source    string    `json:"source"` // "user", "agent", "system"
	Kind      string    `json:"kind"`   // "fact", "preference", "note", "task"
}

// HeartbeatTokens defines special tokens for heartbeat responses
const (
	HeartbeatOKToken = "HEARTBEAT_OK"
	NoReplyToken     = "NO_REPLY"
)
