package main

import (
	"github.com/spf13/cobra"
	"station/cmd/main/handlers/agent"
)

// Agent command definitions
var (
	agentCmd = &cobra.Command{
		Use:   "agent",
		Short: "Manage agents",
		Long:  "List, show, run, and delete AI agents",
	}

	agentListCmd = &cobra.Command{
		Use:   "list",
		Short: "List agents",
		Long:  "List all available agents, optionally filtered by environment",
		RunE:  runAgentList,
	}

	agentShowCmd = &cobra.Command{
		Use:   "show <name>",
		Short: "Show agent details",
		Long:  "Show detailed information about an agent by name",
		Args:  cobra.ExactArgs(1),
		RunE:  runAgentShow,
	}

	agentRunCmd = &cobra.Command{
		Use:   "run <name> <task>",
		Short: "Run an agent using dotprompt",
		Long:  "Execute an agent by name using dotprompt methodology with custom frontmatter support",
		Args:  cobra.ExactArgs(2),
		RunE:  runAgentRun,
	}

	agentDeleteCmd = &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an agent",
		Long:  "Delete an agent and all associated data by name",
		Args:  cobra.ExactArgs(1),
		RunE:  runAgentDelete,
	}


	agentExportCmd = &cobra.Command{
		Use:   "export <name> [environment]",
		Short: "Export agent to dotprompt format",
		Long: `Export an agent configuration to dotprompt format under environments/<env>/agents/.
Creates a <agent-name>.prompt file with YAML frontmatter containing:
- Model configuration and parameters
- Tool dependencies and MCP server mappings
- Custom metadata and execution settings

This enables GitOps-ready agent management and cross-environment deployment.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: runAgentExport,
	}

	agentImportCmd = &cobra.Command{
		Use:   "import [environment]",
		Short: "Import agents from file-based configs",
		Long: `Import all agent configurations from environments/<env>/agents/ directory.
Scans for agent JSON files and creates agents if they don't already exist.
Skips existing agents to prevent duplicates.

This enables GitOps-ready agent deployment from version-controlled configs.`,
		RunE: runAgentImport,
	}

)

// runAgentList lists all agents
func runAgentList(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(themeManager)
	return agentHandler.RunAgentList(cmd, args)
}

// runAgentShow shows agent details
func runAgentShow(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(themeManager)
	return agentHandler.RunAgentShow(cmd, args)
}

// runAgentRun runs an agent
func runAgentRun(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(themeManager)
	return agentHandler.RunAgentRun(cmd, args)
}

// runAgentDelete deletes an agent
func runAgentDelete(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(themeManager)
	return agentHandler.RunAgentDelete(cmd, args)
}


// runAgentExport exports an agent to file-based config
func runAgentExport(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(themeManager)
	return agentHandler.RunAgentExport(cmd, args)
}

// runAgentImport imports agents from file-based configs
func runAgentImport(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(themeManager)
	return agentHandler.RunAgentImport(cmd, args)
}

