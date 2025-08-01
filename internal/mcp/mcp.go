package mcp

// This file contains only the minimum imports needed for the split files
// The actual functionality has been moved to separate focused files:
// - server.go: Server struct, NewServer, Start, Shutdown, auth methods
// - tool_discovery.go: ToolDiscoveryService 
// - resources.go: All resource handlers
// - tools.go: All tool handlers
// - tool_suggestion.go: Agent configuration suggestion logic
// - prompts.go: Enhanced prompts and specialized agent creation guides