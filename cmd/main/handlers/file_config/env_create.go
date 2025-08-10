package file_config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"station/internal/config"
)

// createEnvironmentCommand creates a new file-based MCP environment
func (h *FileConfigHandler) createEnvironmentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new file-based MCP environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envName := args[0]
			createSamples, _ := cmd.Flags().GetBool("init-samples")
			
			// Use proper config directory
			stationConfig := config.GetStationConfigDir()
			configDir := filepath.Join(stationConfig, "environments", envName)
			
			// Check if environment already exists
			if _, err := os.Stat(configDir); err == nil {
				return fmt.Errorf("environment '%s' already exists at %s", envName, configDir)
			}
			
			// Create environment directory
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return fmt.Errorf("failed to create environment directory: %w", err)
			}
			
			// Create variables directory if it doesn't exist
			varsDir := filepath.Join(stationConfig, "vars")
			if err := os.MkdirAll(varsDir, 0755); err != nil {
				return fmt.Errorf("failed to create variables directory: %w", err)
			}
			
			fmt.Printf("✅ Created file-based environment: %s\n", envName)
			fmt.Printf("📁 Directory: %s\n", configDir)
			
			// Optionally create sample configs
			if createSamples {
				if err := h.createSampleConfigs(envName, configDir); err != nil {
					fmt.Printf("⚠️  Failed to create sample configs: %v\n", err)
				} else {
					fmt.Printf("📄 Created sample configuration files\n")
				}
			}
			
			fmt.Printf("\n📖 Next steps:\n")
			fmt.Printf("   • Run 'stn mcp init %s' to create template configurations\n", envName)
			fmt.Printf("   • Run 'stn mcp create <config-name> %s' to create configs from templates\n", envName)
			
			return nil
		},
	}
	
	cmd.Flags().Bool("init-samples", false, "Create sample configuration files")
	
	return cmd
}