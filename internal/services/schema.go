package services

// Message represents a message in the agent execution chain
// This provides a clean message schema for agent execution
type Message struct {
	Content string                 `json:"content"`
	Role    string                 `json:"role"`
	Extra   map[string]interface{} `json:"extra,omitempty"`
}

// Role constants for message types
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// For backward compatibility with existing code that expects schema.Assistant
var Assistant = RoleAssistant
