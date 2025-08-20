package mcp

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
)

// syncCmd implements the declarative MCP sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync file-based configurations to database",
	Long: `Declaratively synchronize all file-based agent and MCP configurations to the database.
This command scans .prompt files and MCP configurations, validates dependencies, and updates the database accordingly.`,
	Example: `  stn mcp sync                    # Sync all environments (interactive)
  stn mcp sync --env production   # Sync specific environment  
  stn mcp sync --dry-run          # Show what would change
  stn mcp sync --validate         # Validate configurations only
  stn mcp sync --no-interactive   # Skip prompting for missing variables
  stn mcp sync --confirm          # Auto-confirm agent deletions`,
	RunE: runMCPSync,
}

// syncFlags holds command-line flags for sync
type syncFlags struct {
	Environment string
	DryRun      bool
	Validate    bool
	Verbose     bool
	Interactive bool
	Confirm     bool
}

func init() {
	// Add sync command flags
	syncCmd.Flags().String("env", "", "Environment to sync (default: all environments)")
	syncCmd.Flags().Bool("dry-run", false, "Show what would be changed without applying changes")
	syncCmd.Flags().Bool("validate", false, "Only validate configurations without syncing")
	syncCmd.Flags().BoolP("verbose", "v", false, "Verbose output showing all operations")
	syncCmd.Flags().BoolP("interactive", "i", true, "Prompt for missing variables (default: true)")
	syncCmd.Flags().Bool("confirm", false, "Skip interactive prompts for agent deletion (auto-confirm)")
	
	// Bind flags to viper
	viper.BindPFlag("mcp.sync.environment", syncCmd.Flags().Lookup("env"))
	viper.BindPFlag("mcp.sync.dry_run", syncCmd.Flags().Lookup("dry-run"))
	viper.BindPFlag("mcp.sync.validate", syncCmd.Flags().Lookup("validate"))
	viper.BindPFlag("mcp.sync.verbose", syncCmd.Flags().Lookup("verbose"))
	viper.BindPFlag("mcp.sync.interactive", syncCmd.Flags().Lookup("interactive"))
	viper.BindPFlag("mcp.sync.confirm", syncCmd.Flags().Lookup("confirm"))
}

func runMCPSync(cmd *cobra.Command, args []string) error {
	// Parse command flags
	flags := syncFlags{
		Environment: viper.GetString("mcp.sync.environment"),
		DryRun:      viper.GetBool("mcp.sync.dry_run"),
		Validate:    viper.GetBool("mcp.sync.validate"),
		Verbose:     viper.GetBool("mcp.sync.verbose"),
		Interactive: viper.GetBool("mcp.sync.interactive"),
		Confirm:     viper.GetBool("mcp.sync.confirm"),
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)

	// Create sync service
	syncer := services.NewDeclarativeSync(repos, cfg)

	// Determine environments to sync
	environments, err := determineEnvironments(repos, flags.Environment)
	if err != nil {
		return err
	}

	fmt.Printf("🔄 Starting MCP sync...\n")
	if flags.DryRun {
		fmt.Printf("📋 DRY RUN MODE - no changes will be applied\n")
	}
	if flags.Validate {
		fmt.Printf("✅ VALIDATION MODE - checking configurations only\n")
	}
	fmt.Printf("\n")

	// Process each environment
	var totalErrors int
	var totalAgents int
	var totalSynced int

	for _, envName := range environments {
		fmt.Printf("🌍 Processing environment: %s\n", envName)
		
		result, err := syncer.SyncEnvironment(context.Background(), envName, services.SyncOptions{
			DryRun:      flags.DryRun,
			Validate:    flags.Validate,
			Verbose:     flags.Verbose,
			Interactive: flags.Interactive,
			Confirm:     flags.Confirm,
		})
		
		if err != nil {
			fmt.Printf("❌ Failed to sync environment %s: %v\n", envName, err)
			totalErrors++
			continue
		}

		// Display results
		displaySyncResults(result, flags.Verbose)
		
		totalAgents += result.AgentsProcessed
		totalSynced += result.AgentsSynced
		totalErrors += result.ValidationErrors
		
		fmt.Printf("\n")
	}

	// Display summary
	fmt.Printf("📊 Sync Summary:\n")
	fmt.Printf("  🌍 Environments: %d processed\n", len(environments))
	fmt.Printf("  🤖 Agents: %d processed, %d synced\n", totalAgents, totalSynced)
	
	if totalErrors > 0 {
		fmt.Printf("  ❌ Errors: %d validation errors\n", totalErrors)
		return fmt.Errorf("sync completed with %d errors", totalErrors)
	} else {
		fmt.Printf("  ✅ All configurations synced successfully\n")
	}

	return nil
}

func determineEnvironments(repos *repositories.Repositories, envFlag string) ([]string, error) {
	if envFlag != "" {
		// Validate specific environment exists
		env, err := repos.Environments.GetByName(envFlag)
		if err != nil {
			return nil, fmt.Errorf("environment '%s' not found: %w", envFlag, err)
		}
		return []string{env.Name}, nil
	}

	// Get all environments
	environments, err := repos.Environments.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}

	envNames := make([]string, len(environments))
	for i, env := range environments {
		envNames[i] = env.Name
	}

	return envNames, nil
}

func displaySyncResults(result *services.SyncResult, verbose bool) {
	// Show agent processing results
	if result.AgentsProcessed > 0 {
		fmt.Printf("  🤖 Agents:\n")
		fmt.Printf("    📁 Found: %d .prompt files\n", result.AgentsProcessed)
		fmt.Printf("    ✅ Synced: %d\n", result.AgentsSynced)
		fmt.Printf("    ⏭️  Skipped: %d (no changes)\n", result.AgentsSkipped)
		
		if result.ValidationErrors > 0 {
			fmt.Printf("    ❌ Errors: %d validation failures\n", result.ValidationErrors)
		}
	}

	// Show MCP server results
	if result.MCPServersProcessed > 0 {
		fmt.Printf("  🔧 MCP Servers:\n")
		fmt.Printf("    📋 Processed: %d configurations\n", result.MCPServersProcessed)
		fmt.Printf("    🔗 Connected: %d servers\n", result.MCPServersConnected)
	}

	// Show detailed operations if verbose
	if verbose {
		if len(result.Operations) > 0 {
			fmt.Printf("  📝 Operations:\n")
			for _, op := range result.Operations {
				fmt.Printf("    %s %s\n", getOperationIcon(op.Type), op.Description)
			}
		}

		if len(result.ValidationMessages) > 0 {
			fmt.Printf("  ⚠️  Validation Issues:\n")
			for _, errMsg := range result.ValidationMessages {
				fmt.Printf("    • %s\n", errMsg)
			}
		}
	}
}

func getOperationIcon(opType services.SyncOperationType) string {
	switch opType {
	case services.OpTypeCreate:
		return "➕"
	case services.OpTypeUpdate:
		return "📝"
	case services.OpTypeDelete:
		return "🗑️"
	case services.OpTypeSkip:
		return "⏭️"
	case services.OpTypeValidate:
		return "✅"
	case services.OpTypeError:
		return "❌"
	default:
		return "📋"
	}
}

// Add sync command to MCP command group
func AddSyncCommand(mcpCmd *cobra.Command) {
	mcpCmd.AddCommand(syncCmd)
}