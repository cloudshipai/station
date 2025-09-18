package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// Prompts and Discovery Handlers
// Handles prompt operations and tool discovery: list prompts, get prompt, discover tools, list MCP configs

func (s *Server) handleListPrompts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	category := request.GetString("category", "")

	// For now, return empty list as prompts are not implemented in file-based system
	var prompts []map[string]interface{}

	response := map[string]interface{}{
		"success": true,
		"prompts": prompts,
		"count":   len(prompts),
	}

	if category != "" {
		response["category"] = category
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleGetPrompt(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'name' parameter: %v", err)), nil
	}

	// For now, return not found as prompts are not implemented in file-based system
	return mcp.NewToolResultError(fmt.Sprintf("Prompt '%s' not found", name)), nil
}


func (s *Server) handleListMCPConfigs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Return empty configs for now since file-based system doesn't use database configs
	allConfigs := []map[string]interface{}{}

	response := map[string]interface{}{
		"success": true,
		"configs": allConfigs,
		"count":   len(allConfigs),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}