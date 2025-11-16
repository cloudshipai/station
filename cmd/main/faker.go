package main

import (
	"fmt"
	"os"
	"strings"

	"station/internal/config"
	"station/internal/db"
	"station/pkg/faker"
	"station/pkg/faker/toolcache"

	"github.com/spf13/cobra"
)

var (
	fakerCommand       string
	fakerArgs          string
	fakerEnvVars       []string
	fakerCacheDir      string
	fakerDebug         bool
	fakerPassthrough   bool
	fakerAIEnabled     bool
	fakerAIModel       string
	fakerAIInstruction string
	fakerAITemplate    string
	fakerStandalone    bool
	fakerID            string
)

var fakerCmd = &cobra.Command{
	Use:   "faker",
	Short: "Run MCP faker proxy that enriches real MCP responses with realistic mock data",
	Long: `Run an MCP faker proxy server that intercepts and enriches responses from real MCP servers.

The faker proxy:
  1. Connects to a real MCP server (AWS, Datadog, Stripe, etc.)
  2. Proxies all tool calls to the real server
  3. Intercepts responses and analyzes their structure
  4. Enriches responses with realistic variable data using entropy helpers
  5. Returns schema-correct mock data to agents for evaluation testing

This allows agents to test against real API structures without needing production data or credentials.

Examples:
  # Faker proxy with NPM-based MCP server
  stn faker \
    --command "npx" \
    --args "-y,@aws-sdk/mcp-server-aws-cost-explorer" \
    --env AWS_REGION=us-east-1 \
    --env AWS_ACCESS_KEY_ID=test

  # Faker proxy with binary MCP server
  stn faker \
    --command "datadog-mcp" \
    --args "--api-key,fake,--app-key,fake"

  # Faker proxy with Station mock (for testing)
  stn faker \
    --command "stn" \
    --args "mock,aws-guardduty" \
    --passthrough

  # AI-powered faker with custom instructions
  stn faker \
    --command "npx" \
    --args "-y,@aws-sdk/mcp-server-aws-cost-explorer" \
    --ai-enabled \
    --ai-model "gemini-1.5-flash" \
    --ai-instruction "Generate realistic AWS cost data with proper spending patterns, service breakdowns, and budget alerts"

  # AI-powered faker with predefined template
  stn faker \
    --command "npx" \
    --args "-y,@datadog/mcp-server-datadog" \
    --ai-enabled \
    --ai-template "monitoring-high-alert"

  # View faker session history
  stn faker sessions list
  stn faker sessions view <session-id>
  stn faker metrics

Usage in template.json:
  {
    "mcpServers": {
      "aws-cost-faker": {
        "command": "stn",
        "args": [
          "faker",
          "--command", "npx",
          "--args", "-y,@aws-sdk/mcp-server-aws-cost-explorer",
          "--env", "AWS_REGION=us-east-1",
          "--ai-enabled",
          "--ai-instruction", "Generate realistic AWS cost data with proper spending patterns"
        ]
      }
    }
  }`,
	RunE: runFaker,
}

func init() {
	fakerCmd.Flags().StringVar(&fakerCommand, "command", "", "Command to execute target MCP server")
	fakerCmd.Flags().StringVar(&fakerArgs, "args", "", "Comma-separated args for target (e.g., '-y,@aws/mcp')")
	fakerCmd.Flags().StringSliceVar(&fakerEnvVars, "env", []string{}, "Environment variables (repeatable, format: KEY=VALUE)")
	fakerCmd.Flags().StringVar(&fakerCacheDir, "cache-dir", "", "Directory for schema cache (default: ~/.cache/station/faker)")
	fakerCmd.Flags().BoolVar(&fakerDebug, "debug", false, "Enable debug logging")
	fakerCmd.Flags().BoolVar(&fakerPassthrough, "passthrough", false, "Disable enrichment (pure proxy mode)")

	// AI enrichment flags
	fakerCmd.Flags().BoolVar(&fakerAIEnabled, "ai-enabled", false, "Enable AI-powered enrichment using Station's configured AI provider")
	fakerCmd.Flags().StringVar(&fakerAIModel, "ai-model", "", "AI model for enrichment (overrides Station's configured model)")
	fakerCmd.Flags().StringVar(&fakerAIInstruction, "ai-instruction", "", "Custom instruction for AI data generation (e.g., 'Generate high-alert monitoring data')")
	fakerCmd.Flags().StringVar(&fakerAITemplate, "ai-template", "", "Predefined instruction template (use 'list' to see available templates)")

	// Standalone mode flags
	fakerCmd.Flags().BoolVar(&fakerStandalone, "standalone", false, "Run in standalone mode (no target MCP server, AI-generated tools only)")
	fakerCmd.Flags().StringVar(&fakerID, "faker-id", "", "Unique identifier for this faker instance (required for standalone mode, e.g., 'aws-cloudwatch-faker')")

	// Note: --command is required for the proxy mode, but not for subcommands like sessions/metrics or standalone mode
}

func runFaker(cmd *cobra.Command, args []string) error {
	// Standalone mode validation
	if fakerStandalone {
		if fakerID == "" {
			return fmt.Errorf("--faker-id is required when using --standalone mode")
		}
		if fakerAIInstruction == "" {
			return fmt.Errorf("--ai-instruction is required when using --standalone mode")
		}
		// In standalone mode, --command is not required
	} else {
		// Check if command flag is provided (required for proxy mode)
		if fakerCommand == "" {
			return fmt.Errorf("--command flag is required when running faker proxy\nUse 'stn faker sessions' or 'stn faker metrics' for session management")
		}
	}

	// Template listing temporarily removed in refactor
	if fakerAITemplate == "list" {
		fmt.Println("Template listing not yet implemented in new faker")
		return nil
	}

	// Parse environment variables
	envVars := make(map[string]string)
	for _, envStr := range fakerEnvVars {
		parts := strings.SplitN(envStr, "=", 2)
		if len(parts) == 2 {
			envVars[parts[0]] = parts[1]
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Invalid env var format '%s', expected KEY=VALUE\n", envStr)
		}
	}

	// Parse args string into slice
	var targetArgs []string
	if fakerArgs != "" {
		targetArgs = strings.Split(fakerArgs, ",")
	}

	// Parse environment variables
	envVars = make(map[string]string)
	for _, envStr := range fakerEnvVars {
		parts := strings.SplitN(envStr, "=", 2)
		if len(parts) == 2 {
			envVars[parts[0]] = parts[1]
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Invalid env var format '%s', expected KEY=VALUE\n", envStr)
		}
	}

	// Build instruction
	instruction := fakerAIInstruction

	// Create and run MCP faker server based on mode
	var f *faker.MCPFaker
	var err error

	if fakerStandalone {
		// Standalone mode - generate tools via AI, no target MCP server

		// Open database for tool cache
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		database, err := db.New(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Run database migrations to ensure faker_tool_cache table exists
		if err := database.Migrate(); err != nil {
			return fmt.Errorf("failed to run database migrations: %w", err)
		}

		// Create tool cache
		toolCache := toolcache.NewCache(database.Conn())

		// Generate deterministic config hash from faker configuration
		// This ensures same config always uses same cache key
		aiModel := fakerAIModel
		if aiModel == "" {
			// Use Station's configured model as default
			aiModel = cfg.AIModel
		}
		configHash := toolcache.GenerateConfigHash(fakerID, instruction, aiModel)

		if fakerDebug {
			fmt.Fprintf(os.Stderr, "[FAKER CLI] Creating standalone faker\n")
			fmt.Fprintf(os.Stderr, "[FAKER CLI]   User ID: %s\n", fakerID)
			fmt.Fprintf(os.Stderr, "[FAKER CLI]   Config Hash: %s\n", configHash)
			fmt.Fprintf(os.Stderr, "[FAKER CLI]   AI Model: %s\n", aiModel)
		}

		// Create standalone faker with config hash
		f, err = faker.NewStandaloneFaker(configHash, instruction, toolCache, fakerDebug)
		if err != nil {
			return fmt.Errorf("failed to create standalone faker: %w", err)
		}
	} else {
		// Proxy mode - connect to real MCP server
		f, err = faker.NewMCPFaker(fakerCommand, targetArgs, envVars, instruction, fakerDebug)
		if err != nil {
			return fmt.Errorf("failed to create faker: %w", err)
		}
	}

	if err := f.Serve(); err != nil {
		return fmt.Errorf("faker failed: %w", err)
	}

	return nil
}
