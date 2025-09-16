package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// BundleEnvironmentRequest represents the input for creating a bundle from an environment
type BundleEnvironmentRequest struct {
	EnvironmentName string `json:"environmentName"`
	OutputPath      string `json:"outputPath,omitempty"`
}

// Shared helper functions and types
// All handler functions have been moved to specific modules:
// - agent_handlers.go: Agent CRUD operations
// - execution_handlers.go: Agent execution and runs
// - tool_handlers.go: Tool management
// - environment_handlers.go: Environment management
// - export_handlers.go: Agent export operations
// - prompts_handlers.go: Prompts and discovery operations

// Helper function for converting lighthouse runs (used by execution_handlers.go)
// This function is shared across multiple modules and remains here

func (s *Server) handleSuggestAgentConfig(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("Agent configuration suggestion not implemented"), nil
}