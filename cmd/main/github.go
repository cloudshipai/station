package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

The workflow automatically detects your AI provider from Station config and generates
the correct environment variables. Supports:
  - API key authentication (OPENAI_API_KEY, ANTHROPIC_API_KEY, etc.)
  - OAuth authentication (STN_AI_OAUTH_TOKEN for Anthropic OAuth)

The workflow can run agents from:
  • CloudShip Bundle ID (recommended) - No Station files in your repo
  • Bundle URL - Download from any URL
  • Local environment - Store agents in your repo

Examples:
  stn github init                           # Interactive setup
  stn github init --bundle-id <id>          # Use specific bundle
  stn github init --agent "Code Reviewer"   # Pre-select agent
  stn github init --trigger pr              # Trigger on pull requests
  stn github init --provider anthropic      # Override detected provider`,
	RunE: runGitHubInit,
}

// AIProviderConfig holds detected AI provider configuration
type AIProviderConfig struct {
	Provider     string
	Model        string
	AuthType     string            // "api_key" or "oauth"
	EnvVarName   string            // Primary env var (e.g., OPENAI_API_KEY, ANTHROPIC_API_KEY)
	EnvVars      []string          // All env vars needed for this auth type
	EnvVarLabels map[string]string // Labels for each env var
}

type SecretsBackendConfig struct {
	Backend string
	Path    string
	Region  string
}

// RequiredSecrets holds all secrets needed for the workflow
type RequiredSecrets struct {
	AIProvider     AIProviderConfig
	CloudShipKey   bool
	MCPVariables   []string // Additional env vars from MCP configs
	SecretsBackend *SecretsBackendConfig
}

func init() {
	// Add flags to github init
	githubInitCmd.Flags().String("bundle-id", "", "CloudShip bundle ID to use")
	githubInitCmd.Flags().String("agent", "", "Agent name to run")
	githubInitCmd.Flags().String("trigger", "", "Workflow trigger: push, pr, schedule, manual")
	githubInitCmd.Flags().String("task", "", "Default task for the agent")
	githubInitCmd.Flags().String("provider", "", "Override detected AI provider (openai, anthropic, google)")
	githubInitCmd.Flags().Bool("yes", false, "Use defaults without prompting")

	// Add subcommands to github
	githubCmd.AddCommand(githubInitCmd)
}

func runGitHubInit(cmd *cobra.Command, args []string) error {
	bundleID, _ := cmd.Flags().GetString("bundle-id")
	agentName, _ := cmd.Flags().GetString("agent")
	trigger, _ := cmd.Flags().GetString("trigger")
	task, _ := cmd.Flags().GetString("task")
	providerOverride, _ := cmd.Flags().GetString("provider")
	useDefaults, _ := cmd.Flags().GetBool("yes")

	fmt.Println("🚀 Station GitHub Actions Setup")
	fmt.Println("================================")
	fmt.Println()

	// Detect AI provider from config
	secrets := detectRequiredSecrets(bundleID, providerOverride)

	fmt.Printf("🤖 Detected AI Provider: %s (%s)\n", secrets.AIProvider.Provider, secrets.AIProvider.Model)

	if secrets.SecretsBackend != nil {
		fmt.Printf("🔐 Secrets Backend: %s (%s)\n", secrets.SecretsBackend.Backend, secrets.SecretsBackend.Path)
		fmt.Println("   ✨ No API keys needed - secrets fetched at runtime!")
	} else if secrets.AIProvider.AuthType == "oauth" {
		fmt.Printf("🔐 Auth Type: OAuth (Anthropic Console)\n")
		fmt.Printf("🔑 Required Secrets:\n")
		for _, envVar := range secrets.AIProvider.EnvVars {
			label := secrets.AIProvider.EnvVarLabels[envVar]
			fmt.Printf("   • %s - %s\n", envVar, label)
		}
	} else {
		fmt.Printf("🔑 API Key Variable: %s\n", secrets.AIProvider.EnvVarName)
	}

	if len(secrets.MCPVariables) > 0 && secrets.SecretsBackend == nil {
		fmt.Printf("🔧 MCP Variables: %s\n", strings.Join(secrets.MCPVariables, ", "))
	}
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
			secrets.CloudShipKey = true
		}
	}

	// Update CloudShipKey if bundle-id was provided via flag
	if bundleID != "" {
		secrets.CloudShipKey = true
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
	workflow := generateWorkflowWithSecrets(bundleID, agentName, trigger, task, secrets)

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

	if secrets.SecretsBackend != nil {
		fmt.Println("🔐 Secrets Backend Mode")
		fmt.Printf("   Backend: %s\n", secrets.SecretsBackend.Backend)
		fmt.Printf("   Path: %s\n", secrets.SecretsBackend.Path)
		fmt.Println()
		fmt.Println("Required GitHub Secrets:")
		if secrets.SecretsBackend.Backend == "aws-secretsmanager" || secrets.SecretsBackend.Backend == "aws-ssm" {
			fmt.Println("   AWS_ROLE_ARN                     - IAM role ARN for OIDC")
		}
		if secrets.CloudShipKey {
			fmt.Println("   CLOUDSHIP_API_KEY                - Your CloudShip API key")
		}
		fmt.Println()
		fmt.Println("Setup AWS OIDC (one-time):")
		fmt.Println("   1. Create IAM OIDC provider for token.actions.githubusercontent.com")
		fmt.Println("   2. Create IAM role with trust policy for your GitHub repo")
		fmt.Println("   3. Grant role access to your secrets in " + secrets.SecretsBackend.Backend)
		fmt.Println()
		fmt.Println("Set the role ARN as a secret:")
		fmt.Println("   gh secret set AWS_ROLE_ARN")
	} else {
		fmt.Println("🔐 Required GitHub Secrets:")

		if secrets.AIProvider.AuthType == "oauth" {
			for _, envVar := range secrets.AIProvider.EnvVars {
				label := secrets.AIProvider.EnvVarLabels[envVar]
				fmt.Printf("   %-30s - %s\n", envVar, label)
			}
		} else if secrets.AIProvider.EnvVarName != "" {
			fmt.Printf("   %-30s - Your %s API key\n", secrets.AIProvider.EnvVarName, secrets.AIProvider.Provider)
		}

		if secrets.CloudShipKey {
			fmt.Println("   CLOUDSHIP_API_KEY                - Your CloudShip API key")
		}
		for _, mcpVar := range secrets.MCPVariables {
			fmt.Printf("   %-30s - Required by MCP tools\n", mcpVar)
		}
		fmt.Println()
		fmt.Println("Set secrets at: https://github.com/<owner>/<repo>/settings/secrets/actions")
		fmt.Println()
		fmt.Println("Or use GitHub CLI:")

		if secrets.AIProvider.AuthType == "oauth" {
			for _, envVar := range secrets.AIProvider.EnvVars {
				fmt.Printf("   gh secret set %s\n", envVar)
			}
		} else if secrets.AIProvider.EnvVarName != "" {
			fmt.Printf("   gh secret set %s\n", secrets.AIProvider.EnvVarName)
		}

		if secrets.CloudShipKey {
			fmt.Println("   gh secret set CLOUDSHIP_API_KEY")
		}
		for _, mcpVar := range secrets.MCPVariables {
			fmt.Printf("   gh secret set %s\n", mcpVar)
		}
	}
	fmt.Println()
	fmt.Println("📚 Documentation: https://docs.cloudshipai.com/station/github-actions")

	return nil
}

// detectRequiredSecrets detects all required secrets from Station config
func detectRequiredSecrets(bundleID string, providerOverride string) RequiredSecrets {
	secrets := RequiredSecrets{
		AIProvider: AIProviderConfig{
			Provider:     "OpenAI",
			Model:        "gpt-4o",
			AuthType:     "api_key",
			EnvVarName:   "OPENAI_API_KEY",
			EnvVars:      []string{"OPENAI_API_KEY"},
			EnvVarLabels: map[string]string{"OPENAI_API_KEY": "Your OpenAI API key"},
		},
		CloudShipKey: bundleID != "",
		MCPVariables: []string{},
	}

	// Try to load Station config
	cfg, err := config.Load()
	if err != nil {
		return secrets
	}

	// Determine provider (override takes precedence)
	provider := strings.ToLower(cfg.AIProvider)
	if providerOverride != "" {
		provider = strings.ToLower(providerOverride)
	}

	secrets.AIProvider.Model = cfg.AIModel

	authType := cfg.AIAuthType
	if authType == "oauth" && provider == "anthropic" {
		secrets.AIProvider.Provider = "Anthropic"
		secrets.AIProvider.AuthType = "oauth"
		secrets.AIProvider.EnvVarName = "STN_AI_OAUTH_TOKEN" // Primary
		secrets.AIProvider.EnvVars = []string{
			"STN_AI_AUTH_TYPE",
			"STN_AI_OAUTH_TOKEN",
			"STN_AI_OAUTH_REFRESH_TOKEN",
			"STN_AI_OAUTH_EXPIRES_AT",
			"STN_AI_PROVIDER",
			"STN_AI_MODEL",
		}
		secrets.AIProvider.EnvVarLabels = map[string]string{
			"STN_AI_AUTH_TYPE":           "Set to 'oauth'",
			"STN_AI_OAUTH_TOKEN":         "Anthropic OAuth access token",
			"STN_AI_OAUTH_REFRESH_TOKEN": "Anthropic OAuth refresh token",
			"STN_AI_OAUTH_EXPIRES_AT":    "Token expiry timestamp (milliseconds)",
			"STN_AI_PROVIDER":            "Set to 'anthropic'",
			"STN_AI_MODEL":               fmt.Sprintf("Set to '%s' or your preferred model", cfg.AIModel),
		}
	} else {
		// API key authentication
		switch provider {
		case "openai":
			secrets.AIProvider.Provider = "OpenAI"
			secrets.AIProvider.EnvVarName = "OPENAI_API_KEY"
			secrets.AIProvider.EnvVars = []string{"OPENAI_API_KEY"}
			secrets.AIProvider.EnvVarLabels = map[string]string{"OPENAI_API_KEY": "Your OpenAI API key"}
		case "anthropic":
			secrets.AIProvider.Provider = "Anthropic"
			secrets.AIProvider.EnvVarName = "ANTHROPIC_API_KEY"
			secrets.AIProvider.EnvVars = []string{"ANTHROPIC_API_KEY"}
			secrets.AIProvider.EnvVarLabels = map[string]string{"ANTHROPIC_API_KEY": "Your Anthropic API key"}
		case "google", "gemini":
			secrets.AIProvider.Provider = "Google"
			secrets.AIProvider.EnvVarName = "GOOGLE_API_KEY"
			secrets.AIProvider.EnvVars = []string{"GOOGLE_API_KEY"}
			secrets.AIProvider.EnvVarLabels = map[string]string{"GOOGLE_API_KEY": "Your Google AI API key"}
		case "ollama":
			secrets.AIProvider.Provider = "Ollama"
			secrets.AIProvider.EnvVarName = "" // Ollama doesn't need an API key
			secrets.AIProvider.EnvVars = []string{}
			secrets.AIProvider.EnvVarLabels = map[string]string{}
		default:
			// Keep defaults
		}
	}

	// If not using bundle-id, scan local environment for MCP variables
	if bundleID == "" {
		secrets.MCPVariables = scanMCPEnvVariables(cfg)
	}

	if cfg.Secrets.Backend != "" {
		secrets.SecretsBackend = &SecretsBackendConfig{
			Backend: cfg.Secrets.Backend,
			Path:    cfg.Secrets.Path,
			Region:  cfg.Secrets.Region,
		}
	}

	return secrets
}

func scanMCPEnvVariables(cfg *config.Config) []string {
	var vars []string
	seen := make(map[string]bool)

	envDir := config.GetEnvironmentDir("default")

	entries, err := os.ReadDir(envDir)
	if err != nil {
		return vars
	}

	// Pattern to match environment variable references in MCP configs
	// Matches: $VAR, ${VAR}, {{.VAR}}
	envVarPattern := regexp.MustCompile(`(?:\$\{?([A-Z][A-Z0-9_]*)\}?|\{\{\s*\.([A-Z][A-Z0-9_]*)\s*\}\})`)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(envDir, entry.Name()))
		if err != nil {
			continue
		}

		matches := envVarPattern.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			// Get the variable name from either capture group
			varName := match[1]
			if varName == "" {
				varName = match[2]
			}

			if varName != "" && !seen[varName] {
				// Skip common internal variables
				if isInternalVariable(varName) {
					continue
				}
				seen[varName] = true
				vars = append(vars, varName)
			}
		}
	}

	return vars
}

// isInternalVariable checks if a variable is an internal Station variable
func isInternalVariable(varName string) bool {
	internalVars := map[string]bool{
		"HOME":                       true,
		"PATH":                       true,
		"USER":                       true,
		"SHELL":                      true,
		"TERM":                       true,
		"PWD":                        true,
		"STATION_CONFIG_DIR":         true,
		"STATION_ENCRYPTION_KEY":     true,
		"STN_AI_API_KEY":             true,
		"STN_AI_PROVIDER":            true,
		"STN_AI_MODEL":               true,
		"STN_AI_AUTH_TYPE":           true,
		"STN_AI_OAUTH_TOKEN":         true,
		"STN_AI_OAUTH_REFRESH_TOKEN": true,
		"STN_AI_OAUTH_EXPIRES_AT":    true,
		"OPENAI_API_KEY":             true,
		"ANTHROPIC_API_KEY":          true,
		"GOOGLE_API_KEY":             true,
	}
	return internalVars[varName]
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

func generateWorkflowWithSecrets(bundleID, agentName, trigger, task string, secrets RequiredSecrets) string {
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

	if secrets.SecretsBackend != nil {
		return generateWorkflowWithSecretsBackend(bundleID, agentName, trigger, task, secrets, triggerYAML)
	}

	var envLines []string

	if secrets.AIProvider.AuthType == "oauth" {
		for _, envVar := range secrets.AIProvider.EnvVars {
			envLines = append(envLines, fmt.Sprintf("          %s: ${{ secrets.%s }}",
				envVar, envVar))
		}
	} else if secrets.AIProvider.EnvVarName != "" {
		envLines = append(envLines, fmt.Sprintf("          %s: ${{ secrets.%s }}",
			secrets.AIProvider.EnvVarName, secrets.AIProvider.EnvVarName))
	}

	for _, mcpVar := range secrets.MCPVariables {
		envLines = append(envLines, fmt.Sprintf("          %s: ${{ secrets.%s }}", mcpVar, mcpVar))
	}
	envSection := strings.Join(envLines, "\n")

	providerInput := ""
	if secrets.AIProvider.AuthType == "oauth" {
		providerInput = fmt.Sprintf("\n          provider: 'anthropic'")
	} else if secrets.AIProvider.Provider != "OpenAI" {
		providerInput = fmt.Sprintf("\n          provider: '%s'", strings.ToLower(secrets.AIProvider.Provider))
	}

	var actionConfig string
	if bundleID != "" {
		actionConfig = fmt.Sprintf(`        with:
          agent: '%s'
          task: |
            ${{ github.event.inputs.task || '%s' }}
          bundle-id: '%s'
          cloudship-api-key: ${{ secrets.CLOUDSHIP_API_KEY }}%s
        env:
%s`,
			escapeYAML(agentName),
			escapeYAML(task),
			bundleID,
			providerInput,
			envSection,
		)
	} else {
		actionConfig = fmt.Sprintf(`        with:
          agent: '%s'
          task: |
            ${{ github.event.inputs.task || '%s' }}
          environment: 'default'%s
        env:
%s`,
			escapeYAML(agentName),
			escapeYAML(task),
			providerInput,
			envSection,
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

func generateWorkflowWithSecretsBackend(bundleID, agentName, trigger, task string, secrets RequiredSecrets, triggerYAML string) string {
	sb := secrets.SecretsBackend

	providerInput := ""
	if secrets.AIProvider.Provider != "OpenAI" {
		providerInput = fmt.Sprintf("\n          provider: '%s'", strings.ToLower(secrets.AIProvider.Provider))
	}

	secretsInputs := fmt.Sprintf(`
          secrets-backend: '%s'
          secrets-path: '%s'`, sb.Backend, sb.Path)
	if sb.Region != "" {
		secretsInputs += fmt.Sprintf("\n          secrets-region: '%s'", sb.Region)
	}

	var actionConfig string
	if bundleID != "" {
		actionConfig = fmt.Sprintf(`        with:
          agent: '%s'
          task: |
            ${{ github.event.inputs.task || '%s' }}
          bundle-id: '%s'
          cloudship-api-key: ${{ secrets.CLOUDSHIP_API_KEY }}%s%s`,
			escapeYAML(agentName),
			escapeYAML(task),
			bundleID,
			providerInput,
			secretsInputs,
		)
	} else {
		actionConfig = fmt.Sprintf(`        with:
          agent: '%s'
          task: |
            ${{ github.event.inputs.task || '%s' }}
          environment: 'default'%s%s`,
			escapeYAML(agentName),
			escapeYAML(task),
			providerInput,
			secretsInputs,
		)
	}

	var awsStep string
	if sb.Backend == "aws-secretsmanager" || sb.Backend == "aws-ssm" {
		region := sb.Region
		if region == "" {
			region = "us-east-1"
		}
		awsStep = fmt.Sprintf(`
      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_ROLE_ARN }}
          aws-region: %s
`, region)
	}

	workflow := fmt.Sprintf(`# Station AI Agent Workflow
# Generated by: stn github init
# Documentation: https://docs.cloudshipai.com/station/github-actions
#
# This workflow uses a secrets backend to fetch secrets at runtime.
# No API keys stored in GitHub Secrets!

name: Run Station Agent

%s

jobs:
  run-agent:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    
    steps:
      - uses: actions/checkout@v4
%s
      - name: Run Station Agent
        uses: cloudshipai/station-action@main
%s
`, triggerYAML, awsStep, actionConfig)

	return workflow
}

func escapeYAML(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
