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

	// Agent Template Bundle Commands
	agentBundleCmd = &cobra.Command{
		Use:   "bundle",
		Short: "Manage agent template bundles",
		Long: `Create, validate, and manage reusable agent template bundles.
Agent bundles enable packaging complete agent configurations with dependencies,
variables, and MCP tools for easy sharing and deployment across environments.`,
	}

	agentBundleCreateCmd = &cobra.Command{
		Use:   "create <path>",
		Short: "Create a new agent bundle",
		Long: `Create a new agent template bundle with scaffolding structure.
Generates manifest.json, agent.json, variables.schema.json, and tools.json
with proper validation and examples for quick development.`,
		Args: cobra.ExactArgs(1),
		RunE: runAgentBundleCreate,
	}

	agentBundleValidateCmd = &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate an agent bundle",
		Long: `Validate an agent template bundle for completeness and correctness.
Checks manifest structure, agent configuration, variable consistency,
tool mappings, and dependency requirements.`,
		Args: cobra.ExactArgs(1),
		RunE: runAgentBundleValidate,
	}

	agentBundleInstallCmd = &cobra.Command{
		Use:   "install <path> [environment]",
		Short: "Install an agent bundle",
		Long: `Install an agent template bundle into the specified environment.
Resolves dependencies, validates variables, installs MCP bundles,
and creates the agent instance with rendered configuration.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: runAgentBundleInstall,
	}

	agentBundleDuplicateCmd = &cobra.Command{
		Use:   "duplicate <agent-id> <target-environment>",
		Short: "Duplicate an agent across environments",
		Long: `Create a copy of an existing agent in a different environment.
Allows customization of name and variables for the new environment
while preserving the core agent configuration and dependencies.`,
		Args: cobra.ExactArgs(2),
		RunE: runAgentBundleDuplicate,
	}

	agentBundleExportCmd = &cobra.Command{
		Use:   "export <agent-id> <output-path>",
		Short: "Export an agent as a template bundle",
		Long: `Export an existing agent as a reusable template bundle.
Analyzes the agent configuration, dependencies, and variables to create
a complete bundle package for sharing and deployment.`,
		Args: cobra.ExactArgs(2),
		RunE: runAgentBundleExport,
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

// runAgentCreate creates a new intelligent agent
func runAgentCreate(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(themeManager)
	return agentHandler.RunAgentCreate(cmd, args)
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

// Agent Bundle Command Handlers

// runAgentBundleCreate creates a new agent bundle
func runAgentBundleCreate(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(themeManager)
	return agentHandler.RunAgentBundleCreate(cmd, args)
}

// runAgentBundleValidate validates an agent bundle
func runAgentBundleValidate(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(themeManager)
	return agentHandler.RunAgentBundleValidate(cmd, args)
}

// runAgentBundleInstall installs an agent bundle
func runAgentBundleInstall(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(themeManager)
	return agentHandler.RunAgentBundleInstall(cmd, args)
}

// runAgentBundleDuplicate duplicates an agent across environments
func runAgentBundleDuplicate(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(themeManager)
	return agentHandler.RunAgentBundleDuplicate(cmd, args)
}

// runAgentBundleExport exports an agent as a template bundle
func runAgentBundleExport(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(themeManager)
	return agentHandler.RunAgentBundleExport(cmd, args)
}