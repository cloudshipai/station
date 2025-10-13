package mcp

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

//go:embed prompts/*.md
var promptsFS embed.FS

// Prompts and Discovery Handlers
// Handles prompt operations and tool discovery: list prompts, get prompt, discover tools, list MCP configs

func (s *Server) handleListPrompts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	category := request.GetString("category", "")

	// Read all .md files from embedded prompts directory
	entries, err := promptsFS.ReadDir("prompts")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read prompts directory: %v", err)), nil
	}

	var prompts []map[string]interface{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			// Remove .md extension for prompt name
			promptName := strings.TrimSuffix(entry.Name(), ".md")

			// Simple category detection based on filename
			promptCategory := "general"
			if strings.Contains(promptName, "ci-cd") || strings.Contains(promptName, "cicd") {
				promptCategory = "cicd"
			} else if strings.Contains(promptName, "station-mcp") {
				promptCategory = "guide"
			}

			// Skip if category filter doesn't match
			if category != "" && promptCategory != category {
				continue
			}

			prompts = append(prompts, map[string]interface{}{
				"name":        promptName,
				"category":    promptCategory,
				"description": fmt.Sprintf("Guidance for %s", promptName),
			})
		}
	}

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

	// Try to read the prompt file with .md extension
	promptPath := filepath.Join("prompts", name+".md")
	content, err := promptsFS.ReadFile(promptPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Prompt '%s' not found", name)), nil
	}

	return mcp.NewToolResultText(string(content)), nil
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