package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"station/internal/config"
	"station/internal/services"
)

func (s *Server) handleSyncEnvironment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	environmentName, err := request.RequireString("environment_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_name' parameter: %v", err)), nil
	}

	browser := request.GetBool("browser", false)
	dryRun := request.GetBool("dry_run", false)
	validate := request.GetBool("validate", false)

	if browser {
		return s.handleSyncWithBrowser(ctx, environmentName)
	}

	return s.handleSyncDirect(ctx, environmentName, dryRun, validate)
}

func (s *Server) handleSyncWithBrowser(ctx context.Context, environmentName string) (*mcp.CallToolResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load config: %v", err)), nil
	}

	browserSync := services.NewBrowserSyncService(cfg.APIPort)
	result, err := browserSync.SyncWithBrowser(ctx, environmentName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Browser sync failed: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":     true,
		"environment": environmentName,
		"mode":        "browser",
		"message":     "Sync completed via browser input. Variables have been securely configured.",
		"result":      result,
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleSyncDirect(ctx context.Context, environmentName string, dryRun, validate bool) (*mcp.CallToolResult, error) {
	syncService := services.NewDeclarativeSync(s.repos, s.config)

	syncOptions := services.SyncOptions{
		DryRun:      dryRun,
		Validate:    validate,
		Interactive: false,
		Verbose:     false,
	}

	result, err := syncService.SyncEnvironment(ctx, environmentName, syncOptions)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Sync failed: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":     true,
		"environment": environmentName,
		"mode":        "direct",
		"message":     "Sync completed successfully",
		"result": map[string]interface{}{
			"agents_processed":      result.AgentsProcessed,
			"agents_synced":         result.AgentsSynced,
			"agents_skipped":        result.AgentsSkipped,
			"mcp_servers_processed": result.MCPServersProcessed,
			"mcp_servers_connected": result.MCPServersConnected,
			"validation_errors":     result.ValidationErrors,
		},
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}
