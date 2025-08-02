package main

import (
	"github.com/spf13/cobra"
	"station/cmd/main/handlers"
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
		Use:   "show <id>",
		Short: "Show agent details",
		Long:  "Show detailed information about an agent by ID",
		Args:  cobra.ExactArgs(1),
		RunE:  runAgentShow,
	}

	agentRunCmd = &cobra.Command{
		Use:   "run <id> <task>",
		Short: "Run an agent",
		Long:  "Execute an agent with the specified task",
		Args:  cobra.ExactArgs(2),
		RunE:  runAgentRun,
	}

	agentDeleteCmd = &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an agent",
		Long:  "Delete an agent and all associated data",
		Args:  cobra.ExactArgs(1),
		RunE:  runAgentDelete,
	}

	agentCreateCmd = &cobra.Command{
		Use:   "create <name> <description>",
		Short: "Create a new intelligent agent",
		Long: `Create a new AI agent using intelligent analysis to determine:
- Optimal environment and tool assignments
- System prompt based on requirements
- Maximum steps and configuration

The agent will be created using Station's self-bootstrapping architecture, 
where our own MCP server analyzes requirements to create optimized agents.`,
		Args: cobra.ExactArgs(2),
		RunE: runAgentCreate,
	}
)

// runAgentList lists all agents
func runAgentList(cmd *cobra.Command, args []string) error {
	agentHandler := handlers.NewAgentHandler(themeManager)
	return agentHandler.RunAgentList(cmd, args)
}

// runAgentShow shows agent details
func runAgentShow(cmd *cobra.Command, args []string) error {
	agentHandler := handlers.NewAgentHandler(themeManager)
	return agentHandler.RunAgentShow(cmd, args)
}

// runAgentRun runs an agent
func runAgentRun(cmd *cobra.Command, args []string) error {
	agentHandler := handlers.NewAgentHandler(themeManager)
	return agentHandler.RunAgentRun(cmd, args)
}

// runAgentDelete deletes an agent
func runAgentDelete(cmd *cobra.Command, args []string) error {
	agentHandler := handlers.NewAgentHandler(themeManager)
	return agentHandler.RunAgentDelete(cmd, args)
}

// runAgentCreate creates a new intelligent agent
func runAgentCreate(cmd *cobra.Command, args []string) error {
	agentHandler := handlers.NewAgentHandler(themeManager)
	return agentHandler.RunAgentCreate(cmd, args)
}