package mocks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MockServer provides the base framework for creating mock MCP servers
// These servers return realistic fake data for demonstration purposes
type MockServer struct {
	name        string
	version     string
	description string
	tools       []mcp.Tool
	handlers    map[string]ToolHandler
}

// ToolHandler is a function that handles tool execution
type ToolHandler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)

// NewMockServer creates a new mock MCP server
func NewMockServer(name, version, description string) *MockServer {
	return &MockServer{
		name:        name,
		version:     version,
		description: description,
		tools:       []mcp.Tool{},
		handlers:    make(map[string]ToolHandler),
	}
}

// RegisterTool registers a tool with its handler
func (m *MockServer) RegisterTool(tool mcp.Tool, handler ToolHandler) {
	m.tools = append(m.tools, tool)
	m.handlers[tool.Name] = handler
}

// Serve starts the mock MCP server in stdio mode
func (m *MockServer) Serve() error {
	// Create MCP server
	mcpServer := server.NewMCPServer(
		m.name,
		m.version,
		server.WithToolCapabilities(true),
	)

	// Register all tools
	for _, tool := range m.tools {
		toolCopy := tool // Create a copy for closure
		handler := m.handlers[tool.Name]

		mcpServer.AddTool(toolCopy, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return handler(ctx, request)
		})
	}

	// Start stdio server
	if err := server.ServeStdio(mcpServer); err != nil {
		return fmt.Errorf("failed to serve mock MCP server: %w", err)
	}

	return nil
}

// Helper functions for common response patterns

// SuccessResult creates a successful tool result with JSON content
func SuccessResult(data interface{}) (*mcp.CallToolResult, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// ErrorResult creates an error tool result
func ErrorResult(errMsg string) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(errMsg), nil
}
