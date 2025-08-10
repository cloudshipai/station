package file_config

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// validateCommand validates file-based configs
func (h *FileConfigHandler) validateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [config-name] [environment-name]",
		Short: "Validate file-based MCP configurations",
		Long:  "Validate template syntax, variable resolution, and config structure.",
		Args:  cobra.MaximumNArgs(2),
		RunE:  h.validateConfigs,
	}

	cmd.Flags().Bool("check-vars", true, "Check for missing variables")
	cmd.Flags().Bool("dry-run", false, "Perform dry-run rendering")
	
	return cmd
}

// validateConfigs handles the validate command
func (h *FileConfigHandler) validateConfigs(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
	envName := "default"
	if len(args) > 1 {
		envName = args[1]
	}
	
	// Validate file-based environment exists
	if err := h.validateEnvironmentExists(envName); err != nil {
		return err
	}
	
	// Get or create environment ID for database operations
	envID, err := h.getOrCreateEnvironmentID(envName)
	if err != nil {
		return fmt.Errorf("failed to get environment ID: %w", err)
	}

	// Validate specific config or all configs
	if len(args) > 0 && args[0] != "" {
		return h.validateSingleConfig(ctx, envID, args[0], cmd)
	}

	// Validate all configs
	configs, err := h.fileConfigService.ListFileConfigs(ctx, envID)
	if err != nil {
		return fmt.Errorf("failed to list configs: %w", err)
	}

	fmt.Printf("Validating %d file-based configurations...\n", len(configs))
	
	valid := 0
	for _, config := range configs {
		fmt.Printf("  %s... ", config.Name)
		err := h.validateSingleConfig(ctx, envID, config.Name, cmd)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("✅\n")
			valid++
		}
	}

	fmt.Printf("\n%d/%d configurations are valid\n", valid, len(configs))
	return nil
}

// validateSingleConfig validates a single configuration
func (h *FileConfigHandler) validateSingleConfig(ctx context.Context, envID int64, configName string, cmd *cobra.Command) error {
	// Load and try to render the config
	_, err := h.fileConfigService.LoadAndRenderConfig(ctx, envID, configName)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	
	return nil
}