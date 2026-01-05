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
		Long:  "List, show, run, create, update, and delete AI agents",
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

	agentCreateCmd = &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new agent",
		Long: `Create a new AI agent with the specified configuration.

REQUIRED FLAGS:
  --prompt, -p       The system prompt for the agent
  --description, -d  A description of what the agent does

OPTIONAL FLAGS:
  --environment, -e       Environment to create the agent in (default: "default")
  --max-steps             Maximum execution steps (default: 5)
  --tools                 Comma-separated list of tool names to assign
  --input-schema          JSON schema for agent input variables
  --output-schema         JSON schema for structured output
  --output-schema-preset  Predefined schema preset (e.g., 'finops')

CLOUDSHIP INTEGRATION:
  --app                   CloudShip app classification for data ingestion
  --app-type              CloudShip app_type classification for data ingestion  
  --memory-topic          CloudShip memory topic key for context injection
  --memory-max-tokens     Max tokens for memory context (default: 2000)

ADVANCED OPTIONS:
  --sandbox               Sandbox config JSON for isolated execution
  --coding                Coding config JSON for OpenCode integration
  --notify                Enable notifications for this agent

EXAMPLES:
  # Create a simple agent
  stn agent create my-agent \
    --description "A helpful assistant" \
    --prompt "You are a helpful assistant that answers questions."

  # Create an agent with tools and memory
  stn agent create github-helper \
    --description "GitHub automation agent" \
    --prompt "You help with GitHub tasks." \
    --tools "__github_create_issue,__github_list_repos" \
    --max-steps 10 \
    --memory-topic "github-context"

  # Create with sandbox isolation
  stn agent create code-runner \
    --description "Runs Python code safely" \
    --prompt "Execute Python code in a sandbox." \
    --sandbox '{"enabled":true,"image":"python:3.11"}'

  # Create with structured output for CloudShip
  stn agent create cost-analyzer \
    --description "Analyzes cloud costs" \
    --prompt "Analyze cloud spending patterns." \
    --output-schema-preset finops \
    --app cloudship \
    --app-type cost_report`,
		Args: cobra.ExactArgs(1),
		RunE: runAgentCreate,
	}

	agentUpdateCmd = &cobra.Command{
		Use:   "update <name>",
		Short: "Update an existing agent",
		Long: `Update an existing AI agent's configuration.

All flags are optional - only provided values will be updated.

BASIC FLAGS:
  --environment, -e       Environment the agent is in (default: "default")
  --prompt, -p            Update the system prompt
  --description, -d       Update the description
  --max-steps             Update maximum execution steps
  --tools                 Update tool assignments (comma-separated)
  --output-schema         Update JSON schema for structured output
  --output-schema-preset  Update predefined schema preset

CLOUDSHIP INTEGRATION:
  --app                   Update CloudShip app classification
  --app-type              Update CloudShip app_type classification
  --memory-topic          Update CloudShip memory topic key
  --memory-max-tokens     Update max tokens for memory context

ADVANCED OPTIONS:
  --sandbox               Update sandbox config JSON
  --coding                Update coding config JSON
  --notify                Update notification setting

EXAMPLES:
  # Update the prompt
  stn agent update my-agent --prompt "New system prompt here."

  # Update max steps and tools
  stn agent update my-agent --max-steps 15 --tools "__slack_send_message"

  # Enable memory integration
  stn agent update my-agent --memory-topic "project-context" --memory-max-tokens 4000

  # Enable sandbox execution
  stn agent update my-agent --sandbox '{"enabled":true,"image":"node:20"}'

  # Enable notifications
  stn agent update my-agent --notify`,
		Args: cobra.ExactArgs(1),
		RunE: runAgentUpdate,
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

// runAgentCreate creates a new agent
func runAgentCreate(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(nil, telemetryService)
	return agentHandler.RunAgentCreate(cmd, args)
}

// runAgentUpdate updates an existing agent
func runAgentUpdate(cmd *cobra.Command, args []string) error {
	agentHandler := agent.NewAgentHandler(nil, telemetryService)
	return agentHandler.RunAgentUpdate(cmd, args)
}
