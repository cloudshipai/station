package adapter

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPServerConfig holds connection configuration for a real MCP server
type MCPServerConfig struct {
	ID          string            `json:"id"`          // Unique server identifier
	Name        string            `json:"name"`        // Human-readable name
	Type        string            `json:"type"`        // Transport type: "stdio", "http", "sse"
	Command     string            `json:"command"`     // For stdio: command to execute
	Args        []string          `json:"args"`        // For stdio: command arguments  
	URL         string            `json:"url"`         // For http/sse: server URL
	Environment map[string]string `json:"environment"` // Environment variables
	Timeout     int               `json:"timeout"`     // Connection timeout in seconds
}

// ToolMapping represents a tool and its source server
type ToolMapping struct {
	ServerID    string    `json:"server_id"`    // ID of the source MCP server
	ToolName    string    `json:"tool_name"`    // Original tool name
	Schema      mcp.Tool  `json:"schema"`       // Original tool schema from source
	Description string    `json:"description"`  // Tool description
}

// AgentSession represents an agent's tool session
type AgentSession struct {
	AgentID       int64    `json:"agent_id"`       // Agent identifier
	SelectedTools []string `json:"selected_tools"` // Tool names this agent can access
	Environment   string   `json:"environment"`    // Agent's environment context
}

// ProxyServerConfig holds configuration for the proxy server
type ProxyServerConfig struct {
	Name        string `json:"name"`         // Proxy server name
	Version     string `json:"version"`      // Proxy server version
	Description string `json:"description"`  // Proxy server description
}