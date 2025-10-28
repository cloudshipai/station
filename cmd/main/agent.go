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
)

// runAgentList lists all agents
func runAgentList(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(nil, telemetryService)
	return agentHandler.RunAgentList(cmd, args)
}

// runAgentShow shows agent details
func runAgentShow(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(nil, telemetryService)
	return agentHandler.RunAgentShow(cmd, args)
}

// runAgentRun runs an agent
func runAgentRun(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(nil, telemetryService)
	return agentHandler.RunAgentRun(cmd, args)
}

// runAgentDelete deletes an agent
func runAgentDelete(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(nil, telemetryService)
	return agentHandler.RunAgentDelete(cmd, args)
}
