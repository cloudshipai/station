package file_config

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// discoverCommand discovers tools for file configs
func (h *FileConfigHandler) discoverCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "discover <config-name-or-id> [environment-name]",
		Short:      "[DEPRECATED] Discover tools for a file-based configuration",
		Long:       "[DEPRECATED] This command is deprecated. Use 'stn mcp sync <environment>' instead for automatic tool discovery and template management.",
		Args:       cobra.RangeArgs(1, 2),
		RunE:       h.discoverTools,
		Deprecated: "Use 'stn mcp sync <environment>' instead, which automatically discovers tools and handles template bundles.",
	}

	cmd.Flags().Bool("verbose", false, "Verbose output during discovery")
	cmd.Flags().Int("timeout", 30, "Discovery timeout in seconds")
	
	return cmd
}

// discoverTools handles the discover command
func (h *FileConfigHandler) discoverTools(cmd *cobra.Command, args []string) error {
	configNameOrID := args[0]
	envName := "default"
	if len(args) > 1 {
		envName = args[1]
	}

	ctx := context.Background()
	
	// Validate file-based environment exists
	if err := h.validateEnvironmentExists(envName); err != nil {
		return err
	}
	
	// Get or create environment ID for database operations
	envID, err := h.getOrCreateEnvironmentID(envName)
	if err != nil {
		return fmt.Errorf("failed to get environment ID: %w", err)
	}

	// Find config (try by name first, then by ID)
	var configName string
	if configByName, err := h.repos.FileMCPConfigs.GetByEnvironmentAndName(envID, configNameOrID); err == nil {
		configName = configByName.ConfigName
	} else {
		// Try parsing as ID
		if id, parseErr := strconv.ParseInt(configNameOrID, 10, 64); parseErr == nil {
			if configByID, err := h.repos.FileMCPConfigs.GetByID(id); err == nil {
				configName = configByID.ConfigName
			}
		}
	}

	if configName == "" {
		return fmt.Errorf("config '%s' not found", configNameOrID)
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	
	fmt.Printf("🔍 Discovering tools for config '%s' in environment '%s'...\n", configName, envName)
	
	if verbose {
		fmt.Printf("   1. Loading template...\n")
		fmt.Printf("   2. Resolving variables...\n")
		fmt.Printf("   3. Rendering configuration...\n")
		fmt.Printf("   4. Connecting to MCP servers...\n")
		fmt.Printf("   5. Discovering tools...\n")
	}

	result, err := h.fileConfigService.DiscoverToolsForConfig(ctx, envID, configName)
	if err != nil {
		return fmt.Errorf("tool discovery failed: %w", err)
	}

	// Display results
	if result.Success {
		fmt.Printf("✅ Tool discovery completed successfully!\n")
		fmt.Printf("   Servers processed: %d/%d\n", result.SuccessfulServers, result.TotalServers)
		fmt.Printf("   Tools discovered: %d\n", result.TotalTools)
		fmt.Printf("   Duration: %v\n", result.CompletedAt.Sub(result.StartedAt))
	} else {
		fmt.Printf("⚠️  Tool discovery completed with issues\n")
		fmt.Printf("   Successful servers: %d/%d\n", result.SuccessfulServers, result.TotalServers)
		fmt.Printf("   Tools discovered: %d\n", result.TotalTools)
		fmt.Printf("   Errors: %d\n", len(result.Errors))
	}

	if len(result.Errors) > 0 && verbose {
		fmt.Printf("\nErrors encountered:\n")
		for _, err := range result.Errors {
			fmt.Printf("   - %s: %s\n", err.ServerName, err.Message)
		}
	}

	return nil
}