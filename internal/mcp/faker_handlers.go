package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"station/internal/config"
	"station/internal/services"
)

// Faker Management Handlers
// Handles faker operations: create standalone faker with custom tools

func (s *Server) handleFakerCreateStandalone(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Required parameters
	environmentName, err := request.RequireString("environment_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_name' parameter: %v", err)), nil
	}

	fakerName, err := request.RequireString("faker_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'faker_name' parameter: %v", err)), nil
	}

	description, err := request.RequireString("description")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'description' parameter: %v", err)), nil
	}

	goal, err := request.RequireString("goal")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'goal' parameter: %v", err)), nil
	}

	// Optional parameters
	toolsJSON := request.GetString("tools", "")
	autoSync := request.GetBool("auto_sync", true)
	debug := request.GetBool("debug", false)

	// Get environment directory using centralized path resolution (respects workspace config)
	envDir := config.GetEnvironmentDir(environmentName)

	// Check environment exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return mcp.NewToolResultError(fmt.Sprintf("Environment '%s' does not exist at %s", environmentName, envDir)), nil
	}

	// Parse tools if provided
	var tools []ToolDefinition
	if toolsJSON != "" {
		if err := json.Unmarshal([]byte(toolsJSON), &tools); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to parse tools JSON: %v", err)), nil
		}
	}

	// Extract tool names for FAKER_TOOL_NAMES env var
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	// Build faker args using --standalone mode (matching example faker files)
	fakerArgs := []string{
		"faker",
		"--standalone",
		"--faker-id", fakerName,
		"--ai-model", "gpt-4o-mini", // Uses Station's global AI model
		"--ai-instruction", goal,
	}
	if debug {
		fakerArgs = append(fakerArgs, "--debug")
	}

	// Create faker MCP server template file (e.g., datadog.json)
	// This matches the format in ~/.config/station/environments/default/*.json
	fakerTemplate := map[string]interface{}{
		"name":        fakerName,
		"description": fmt.Sprintf("MCP server configuration for %s", fakerName),
		"mcpServers": map[string]interface{}{
			fakerName: map[string]interface{}{
				"name":        fakerName,
				"description": description,
				"command":     "stn",
				"args":        fakerArgs,
				"env": map[string]string{
					"FAKER_TOOL_NAMES": strings.Join(toolNames, ","),
				},
			},
		},
	}

	// Write faker template file (e.g., ~/.config/station/environments/default/datadog.json)
	fakerTemplatePath := filepath.Join(envDir, fmt.Sprintf("%s.json", fakerName))
	fakerTemplateData, err := json.MarshalIndent(fakerTemplate, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal faker template: %v", err)), nil
	}

	if err := os.WriteFile(fakerTemplatePath, fakerTemplateData, 0644); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to write faker template file: %v", err)), nil
	}

	// Auto-sync if requested
	var syncResult *services.SyncResult
	if autoSync {
		syncService := services.NewDeclarativeSync(s.repos, s.config)
		syncOptions := services.SyncOptions{
			DryRun:      false,
			Validate:    true,
			Interactive: false,
			Verbose:     false,
			Confirm:     false,
		}

		syncResult, err = syncService.SyncEnvironment(ctx, environmentName, syncOptions)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Faker created but sync failed: %v", err)), nil
		}
	}

	// Build response
	response := map[string]interface{}{
		"success":          true,
		"message":          fmt.Sprintf("Created standalone faker '%s' in environment '%s'", fakerName, environmentName),
		"environment":      environmentName,
		"faker_name":       fakerName,
		"description":      description,
		"goal":             goal,
		"tools_count":      len(tools),
		"template_path":    fakerTemplatePath,
		"template_updated": true,
		"auto_synced":      autoSync,
		"note":             "AI model will use Station's global AI configuration (STN_AI_PROVIDER and STN_AI_MODEL)",
	}

	if syncResult != nil {
		response["sync_result"] = map[string]interface{}{
			"agents_processed":  syncResult.AgentsProcessed,
			"servers_processed": syncResult.MCPServersProcessed,
			"servers_connected": syncResult.MCPServersConnected,
			"validation_errors": syncResult.ValidationErrors,
		}
	}

	nextSteps := []string{
		fmt.Sprintf("Faker template saved to %s", fakerTemplatePath),
	}

	if autoSync {
		nextSteps = append(nextSteps, "✅ Environment synced - faker is now active and ready to use")
	} else {
		nextSteps = append(nextSteps, fmt.Sprintf("Run 'stn sync %s' to activate the faker and discover its tools", environmentName))
	}

	response["next_steps"] = nextSteps

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}
