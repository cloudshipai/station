package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const codeModeCLIGuidance = `Station CLI is the recommended way to manage agents and workflows when using AI coding assistants.

Common commands:

  Agent Management:
    stn agent list                    # List all agents
    stn agent run <name> "<task>"     # Execute an agent
    stn runs list                     # List recent runs
    stn runs inspect <id>             # View run details

  Configuration:
    stn config show                   # Show current config
    stn config --browser              # Edit config in browser
    stn sync                          # Sync environment changes

  Workflows:
    stn workflow list                 # List workflows
    stn workflow run <id>             # Start a workflow

  Status:
    stn status                        # Check Station status
    stn auth status                   # Check authentication

For full documentation: https://docs.cloudshipai.com/station`

func ServeCodeMode() error {
	mcpServer := server.NewMCPServer(
		"Station Code Mode",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	tool := mcp.NewTool("station_cli",
		mcp.WithDescription("Get guidance on using the Station CLI. Station agents and workflows should be managed via the stn command-line tool."),
		mcp.WithString("question", mcp.Description("Optional: specific question about Station CLI usage")),
	)

	mcpServer.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		question := request.GetString("question", "")

		response := codeModeCLIGuidance
		if question != "" {
			response = "For: " + question + "\n\n" + codeModeCLIGuidance
		}

		return mcp.NewToolResultText(response), nil
	})

	return server.ServeStdio(mcpServer)
}
