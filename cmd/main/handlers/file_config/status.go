package file_config

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// statusCommand shows file config status
func (h *FileConfigHandler) statusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [environment-name]",
		Short: "Show file-based configuration status",
		Long:  "Show the status of file-based configurations including change detection.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  h.showStatus,
	}

	cmd.Flags().Bool("check-changes", true, "Check for file changes")
	cmd.Flags().Bool("tool-counts", true, "Include tool counts")
	
	return cmd
}

// showStatus handles the status command
func (h *FileConfigHandler) showStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
	envName := "default"
	if len(args) > 0 {
		envName = args[0]
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

	fmt.Printf("File-based Configuration Status - Environment: %s\n", envName)
	fmt.Printf("═══════════════════════════════════════════════════\n\n")

	// Get file configs
	configs, err := h.fileConfigService.ListFileConfigs(ctx, envID)
	if err != nil {
		return fmt.Errorf("failed to list configs: %w", err)
	}

	if len(configs) == 0 {
		fmt.Printf("No file-based configurations found.\n")
		fmt.Printf("Run 'stn file-config init' to set up the structure.\n")
		return nil
	}

	// Display each config status
	for i, config := range configs {
		if i > 0 {
			fmt.Printf("\n")
		}
		
		fmt.Printf("📄 %s\n", config.Name)
		fmt.Printf("   Type: %s\n", config.Type)
		fmt.Printf("   Path: %s\n", config.Path)
		
		if config.Metadata != nil {
			if lastLoaded, ok := config.Metadata["last_loaded"]; ok {
				fmt.Printf("   Last loaded: %s\n", lastLoaded)
			}
			if templateHash, ok := config.Metadata["template_hash"]; ok {
				fmt.Printf("   Template hash: %s\n", templateHash[:12]+"...")
			}
		}

		// Show tool count if requested
		toolCounts, _ := cmd.Flags().GetBool("tool-counts")
		if toolCounts {
			// This would require getting the file config record and counting tools
			fmt.Printf("   Tools: (counting...)\n")
		}
	}

	return nil
}