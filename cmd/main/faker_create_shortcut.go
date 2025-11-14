package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"

	"github.com/spf13/cobra"
)

var (
	fakerCreateShortcutEnv         string
	fakerCreateShortcutInstruction string
	fakerCreateShortcutTemplate    string
	fakerCreateShortcutModel       string
	fakerCreateShortcutNoSync      bool
)

var fakerCreateShortcutCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a standalone faker with AI-generated tools",
	Long: `Create a faker that generates tools via AI without connecting to a real MCP server.

This command creates a proper MCP server configuration in the environment's directory
and optionally syncs the environment to generate tools.

Examples:
  # Create faker using built-in template
  stn faker create aws-cost --template aws-finops --env production

  # Create faker with custom instruction
  stn faker create custom-api \
    --instruction "Generate tools for managing user accounts..." \
    --env development

  # Create without auto-syncing
  stn faker create stripe-api --template stripe-payments --env staging --no-sync

  # List available templates
  stn faker templates`,
	Args: cobra.ExactArgs(1),
	RunE: runFakerCreateShortcut,
}

func init() {
	fakerCreateShortcutCmd.Flags().StringVar(&fakerCreateShortcutEnv, "env", "", "Environment name (required)")
	fakerCreateShortcutCmd.Flags().StringVar(&fakerCreateShortcutInstruction, "instruction", "", "Custom AI instruction for tool generation")
	fakerCreateShortcutCmd.Flags().StringVar(&fakerCreateShortcutTemplate, "template", "", "Use built-in template (e.g., aws-finops, gcp-finops)")
	fakerCreateShortcutCmd.Flags().StringVar(&fakerCreateShortcutModel, "model", "", "AI model (defaults to global config)")
	fakerCreateShortcutCmd.Flags().BoolVar(&fakerCreateShortcutNoSync, "no-sync", false, "Don't automatically sync environment after creation")

	fakerCreateShortcutCmd.MarkFlagRequired("env")

	fakerCmd.AddCommand(fakerCreateShortcutCmd)
}

func runFakerCreateShortcut(cmd *cobra.Command, args []string) error {
	fakerName := args[0]

	// Load config for templates and defaults
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine instruction (from template or custom)
	var instruction string
	var aiModel string

	if fakerCreateShortcutTemplate != "" {
		// Use template
		if template, exists := cfg.FakerTemplates[fakerCreateShortcutTemplate]; exists {
			instruction = template.Instruction
			aiModel = template.Model
			fmt.Printf("Using template '%s' (%s)\n", fakerCreateShortcutTemplate, template.Name)
		} else {
			return fmt.Errorf("template '%s' not found. Run 'stn faker templates' to list available templates", fakerCreateShortcutTemplate)
		}
	} else if fakerCreateShortcutInstruction != "" {
		// Use custom instruction
		instruction = fakerCreateShortcutInstruction
	} else {
		return fmt.Errorf("either --template or --instruction must be provided")
	}

	// Determine model (flag > template > global config)
	if fakerCreateShortcutModel != "" {
		aiModel = fakerCreateShortcutModel
	} else if aiModel == "" {
		aiModel = cfg.AIModel
	}

	if aiModel == "" {
		aiModel = "gpt-4o-mini" // Final fallback
	}

	// Build MCP server config (proper format with args array)
	mcpServerConfig := map[string]interface{}{
		"command": "stn", // Use stn from PATH, not absolute path
		"args": []string{
			"faker",
			"--standalone",
			"--faker-id", fakerName,
			"--ai-model", aiModel,
			"--ai-instruction", instruction,
		},
	}

	// Create single-server template structure
	singleServerTemplate := map[string]interface{}{
		"name":        fakerName,
		"description": fmt.Sprintf("Faker configuration for %s", fakerName),
		"mcpServers": map[string]interface{}{
			fakerName: mcpServerConfig,
		},
	}

	// Get environment directory
	envDir := config.GetEnvironmentDir(fakerCreateShortcutEnv)

	// Check if environment exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return fmt.Errorf("environment '%s' does not exist at %s", fakerCreateShortcutEnv, envDir)
	}

	// Write config file
	configPath := filepath.Join(envDir, fmt.Sprintf("%s.json", fakerName))
	configData, err := json.MarshalIndent(singleServerTemplate, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✓ Created faker '%s' in environment '%s'\n", fakerName, fakerCreateShortcutEnv)
	fmt.Printf("  Config file: %s\n", configPath)
	fmt.Printf("  Model: %s\n", aiModel)

	// Auto-sync if not disabled
	if !fakerCreateShortcutNoSync {
		fmt.Printf("\n🔄 Syncing environment to generate tools (this may take 30-60 seconds)...\n")

		// Open database
		database, err := db.New(cfg.DatabaseURL)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to sync (database error): %v\n", err)
			fmt.Printf("Run manually: stn sync %s\n", fakerCreateShortcutEnv)
			return nil
		}
		defer database.Close()

		// Create repositories
		repos := repositories.New(database)

		// Run sync
		syncer := services.NewDeclarativeSync(repos, cfg)
		result, err := syncer.SyncEnvironment(cmd.Context(), fakerCreateShortcutEnv, services.SyncOptions{
			Interactive: false,
			Verbose:     false,
		})

		if err != nil {
			fmt.Printf("⚠️  Warning: Sync completed with errors: %v\n", err)
			fmt.Printf("Run manually: stn sync %s\n", fakerCreateShortcutEnv)
			return nil
		}

		fmt.Printf("\n✓ Sync completed successfully!\n")
		fmt.Printf("  MCP Servers: %d processed, %d connected\n", result.MCPServersProcessed, result.MCPServersConnected)
		fmt.Printf("  Agents: %d processed, %d synced\n", result.AgentsProcessed, result.AgentsSynced)

		if result.ValidationErrors > 0 {
			fmt.Printf("  ⚠️  Validation warnings: %d\n", result.ValidationErrors)
		}
	} else {
		fmt.Printf("\n💡 Sync skipped. Run: stn sync %s\n", fakerCreateShortcutEnv)
	}

	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  stn agent list --env %s  # View agents that can use these tools\n", fakerCreateShortcutEnv)

	return nil
}
