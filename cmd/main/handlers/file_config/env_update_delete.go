package file_config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// updateEnvironmentCommand updates a file-based MCP environment (rename directory)
func (h *FileConfigHandler) updateEnvironmentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a file-based MCP environment (rename directory)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envName := args[0]
			newName, _ := cmd.Flags().GetString("name")
			
			if newName == "" {
				return fmt.Errorf("--name flag is required for updating environment")
			}
			
			// Check if source environment exists
			oldPath := fmt.Sprintf("./config/environments/%s", envName)
			if _, err := os.Stat(oldPath); err != nil {
				return fmt.Errorf("environment '%s' not found at %s", envName, oldPath)
			}
			
			// Check if target doesn't exist
			newPath := fmt.Sprintf("./config/environments/%s", newName)
			if _, err := os.Stat(newPath); err == nil {
				return fmt.Errorf("environment '%s' already exists at %s", newName, newPath)
			}
			
			// Rename directory
			if err := os.Rename(oldPath, newPath); err != nil {
				return fmt.Errorf("failed to rename environment directory: %w", err)
			}
			
			fmt.Printf("✅ Renamed environment: %s → %s\n", envName, newName)
			fmt.Printf("📁 New path: %s\n", newPath)
			
			return nil
		},
	}
	
	cmd.Flags().String("name", "", "New name for the environment (required)")
	
	return cmd
}

// deleteEnvironmentCommand deletes a file-based MCP environment
func (h *FileConfigHandler) deleteEnvironmentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a file-based MCP environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envName := args[0]
			force, _ := cmd.Flags().GetBool("force")
			
			// Check if environment exists
			configDir := fmt.Sprintf("./config/environments/%s", envName)
			if _, err := os.Stat(configDir); err != nil {
				return fmt.Errorf("environment '%s' not found at %s", envName, configDir)
			}
			
			// Prevent deletion of default environment unless forced
			if envName == "default" && !force {
				return fmt.Errorf("cannot delete default environment without --force flag")
			}
			
			// Confirm deletion
			if !force {
				fmt.Printf("⚠️  This will permanently delete the environment '%s' and all its configuration files.\n", envName)
				fmt.Printf("Are you sure you want to continue? (y/N): ")
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("Deletion cancelled")
					return nil
				}
			}
			
			// Remove directory and all contents
			if err := os.RemoveAll(configDir); err != nil {
				return fmt.Errorf("failed to remove environment directory: %w", err)
			}
			
			fmt.Printf("✅ Deleted file-based environment: %s\n", envName)
			fmt.Printf("🗑️  Removed directory: %s\n", configDir)
			
			return nil
		},
	}
	
	cmd.Flags().Bool("force", false, "Force deletion without confirmation (required for default environment)")
	
	return cmd
}