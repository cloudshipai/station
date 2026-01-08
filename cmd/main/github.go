package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
)

// GitHub command group
var githubCmd = &cobra.Command{
	Use:   "github",
	Short: "GitHub integration commands",
	Long:  "Commands for integrating Station with GitHub Actions and other GitHub features.",
}

// GitHub init subcommand
var githubInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize GitHub Actions workflow for running Station agents",
	Long: `Create a GitHub Actions workflow file that runs Station agents in your CI/CD pipeline.

This command helps you set up the cloudshipai/station-action in your repository.
It will guide you through selecting an agent and configuring the workflow trigger.

The workflow can run agents from:
  • CloudShip Bundle ID (recommended) - No Station files in your repo
  • Bundle URL - Download from any URL
  • Local environment - Store agents in your repo

Examples:
  stn github init                           # Interactive setup
  stn github init --bundle-id <id>          # Use specific bundle
  stn github init --agent "Code Reviewer"   # Pre-select agent
  stn github init --trigger pr              # Trigger on pull requests`,
	RunE: runGitHubInit,
}

func init() {
	// Add flags to github init
	githubInitCmd.Flags().String("bundle-id", "", "CloudShip bundle ID to use")
	githubInitCmd.Flags().String("agent", "", "Agent name to run")
	githubInitCmd.Flags().String("trigger", "", "Workflow trigger: push, pr, schedule, manual")
	githubInitCmd.Flags().String("task", "", "Default task for the agent")
	githubInitCmd.Flags().Bool("yes", false, "Use defaults without prompting")

	// Add subcommands to github
	githubCmd.AddCommand(githubInitCmd)
}

func runGitHubInit(cmd *cobra.Command, args []string) error {
	bundleID, _ := cmd.Flags().GetString("bundle-id")
	agentName, _ := cmd.Flags().GetString("agent")
	trigger, _ := cmd.Flags().GetString("trigger")
	task, _ := cmd.Flags().GetString("task")
	useDefaults, _ := cmd.Flags().GetBool("yes")

	fmt.Println("🚀 Station GitHub Actions Setup")
	fmt.Println("================================")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Step 1: Determine agent source
	if bundleID == "" && !useDefaults {
		fmt.Println("How would you like to provide agents?")
		fmt.Println("  1. CloudShip Bundle ID (recommended - keeps repo clean)")
		fmt.Println("  2. Local environment (agents stored in repo)")
		fmt.Println()
		fmt.Print("Choice [1]: ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		if choice == "" || choice == "1" {
			fmt.Print("Enter your CloudShip Bundle ID: ")
			bundleID, _ = reader.ReadString('\n')
			bundleID = strings.TrimSpace(bundleID)

			if bundleID == "" {
				fmt.Println()
				fmt.Println("💡 To get a Bundle ID:")
				fmt.Println("   1. Run 'stn bundle share default' to upload your environment")
				fmt.Println("   2. Copy the Bundle ID from the output")
				fmt.Println()
				return fmt.Errorf("bundle ID is required for CloudShip mode")
			}
		}
	}

	// Step 2: Get agent name
	if agentName == "" && !useDefaults {
		// Try to list agents from local environment
		agents := listLocalAgents()

		if len(agents) > 0 && bundleID == "" {
			fmt.Println()
			fmt.Println("Available agents in your environment:")
			for i, agent := range agents {
				fmt.Printf("  %d. %s\n", i+1, agent)
			}
			fmt.Println()
		}

		fmt.Print("Enter agent name to run: ")
		agentName, _ = reader.ReadString('\n')
		agentName = strings.TrimSpace(agentName)

		if agentName == "" {
			return fmt.Errorf("agent name is required")
		}
	}

	// Step 3: Get trigger type
	if trigger == "" && !useDefaults {
		fmt.Println()
		fmt.Println("When should this workflow run?")
		fmt.Println("  1. On push to main branch")
		fmt.Println("  2. On pull requests")
		fmt.Println("  3. On a schedule (e.g., daily)")
		fmt.Println("  4. Manual trigger only")
		fmt.Println()
		fmt.Print("Choice [4]: ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			trigger = "push"
		case "2":
			trigger = "pr"
		case "3":
			trigger = "schedule"
		default:
			trigger = "manual"
		}
	}
	if trigger == "" {
		trigger = "manual"
	}

	// Step 4: Get default task
	if task == "" && !useDefaults {
		fmt.Println()
		fmt.Print("Enter default task for the agent (or press Enter to skip): ")
		task, _ = reader.ReadString('\n')
		task = strings.TrimSpace(task)

		if task == "" {
			task = "Analyze the codebase and provide insights."
		}
	}
	if task == "" {
		task = "Analyze the codebase and provide insights."
	}

	// Generate workflow file
	workflow := generateWorkflow(bundleID, agentName, trigger, task)

	// Create .github/workflows directory
	workflowDir := ".github/workflows"
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		return fmt.Errorf("failed to create workflow directory: %w", err)
	}

	// Write workflow file
	workflowPath := filepath.Join(workflowDir, "station-agent.yml")
	if err := os.WriteFile(workflowPath, []byte(workflow), 0644); err != nil {
		return fmt.Errorf("failed to write workflow file: %w", err)
	}

	fmt.Println()
	fmt.Println("✅ GitHub Actions workflow created!")
	fmt.Println()
	fmt.Printf("📄 File: %s\n", workflowPath)
	fmt.Println()
	fmt.Println("🔐 Required GitHub Secrets:")
	fmt.Println("   OPENAI_API_KEY      - Your OpenAI API key")
	if bundleID != "" {
		fmt.Println("   CLOUDSHIP_API_KEY   - Your CloudShip API key")
	}
	fmt.Println()
	fmt.Println("Set secrets at: https://github.com/<owner>/<repo>/settings/secrets/actions")
	fmt.Println()
	fmt.Println("Or use GitHub CLI:")
	fmt.Println("   gh secret set OPENAI_API_KEY")
	if bundleID != "" {
		fmt.Println("   gh secret set CLOUDSHIP_API_KEY")
	}
	fmt.Println()
	fmt.Println("📚 Documentation: https://docs.cloudshipai.com/station/github-actions")

	return nil
}

func listLocalAgents() []string {
	var agents []string

	// Try to load config and list agents
	cfg, err := config.Load()
	if err != nil {
		return agents
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return agents
	}
	defer database.Close()

	repos := repositories.New(database)

	// Get default environment
	env, err := repos.Environments.GetByName("default")
	if err != nil {
		return agents
	}

	// List agents in that environment
	agentList, err := repos.Agents.ListByEnvironment(env.ID)
	if err != nil {
		return agents
	}

	for _, agent := range agentList {
		agents = append(agents, agent.Name)
	}

	return agents
}

func generateWorkflow(bundleID, agentName, trigger, task string) string {
	var triggerYAML string
	switch trigger {
	case "push":
		triggerYAML = `on:
  push:
    branches: [main]
  workflow_dispatch:`
	case "pr":
		triggerYAML = `on:
  pull_request:
    types: [opened, synchronize]
  workflow_dispatch:`
	case "schedule":
		triggerYAML = `on:
  schedule:
    - cron: '0 9 * * 1'  # Every Monday at 9 AM UTC
  workflow_dispatch:`
	default:
		triggerYAML = `on:
  workflow_dispatch:
    inputs:
      task:
        description: 'Task for the agent'
        required: false
        default: '` + escapeYAML(task) + `'`
	}

	var actionConfig string
	if bundleID != "" {
		actionConfig = fmt.Sprintf(`        with:
          agent: '%s'
          task: |
            ${{ github.event.inputs.task || '%s' }}
          bundle-id: '%s'
          cloudship-api-key: ${{ secrets.CLOUDSHIP_API_KEY }}
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}`,
			escapeYAML(agentName),
			escapeYAML(task),
			bundleID,
		)
	} else {
		actionConfig = fmt.Sprintf(`        with:
          agent: '%s'
          task: |
            ${{ github.event.inputs.task || '%s' }}
          environment: 'default'
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}`,
			escapeYAML(agentName),
			escapeYAML(task),
		)
	}

	workflow := fmt.Sprintf(`# Station AI Agent Workflow
# Generated by: stn github init
# Documentation: https://docs.cloudshipai.com/station/github-actions

name: Run Station Agent

%s

jobs:
  run-agent:
    runs-on: ubuntu-latest
    
    steps:
      - uses: actions/checkout@v4

      - name: Run Station Agent
        uses: cloudshipai/station-action@main
%s
`, triggerYAML, actionConfig)

	return workflow
}

func escapeYAML(s string) string {
	// Escape single quotes by doubling them
	return strings.ReplaceAll(s, "'", "''")
}
