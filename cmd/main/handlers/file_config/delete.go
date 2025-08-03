package file_config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// deleteCommand deletes a file-based config
func (h *FileConfigHandler) deleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <config-name> [environment-name]",
		Short: "Delete a file-based MCP configuration",
		Long:  "Delete a file-based MCP configuration and optionally clean up associated files.",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  h.deleteConfig,
	}

	cmd.Flags().Bool("keep-files", false, "Keep template and variable files")
	cmd.Flags().Bool("force", false, "Force deletion without confirmation")
	
	return cmd
}

// deleteConfig handles the delete command
func (h *FileConfigHandler) deleteConfig(cmd *cobra.Command, args []string) error {
	configName := args[0]
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

	force, _ := cmd.Flags().GetBool("force")
	if !force {
		fmt.Printf("Are you sure you want to delete config '%s' from environment '%s'? (y/N): ", configName, envName)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Deletion cancelled")
			return nil
		}
	}

	// Get file config record to find associated files
	fileConfig, err := h.repos.FileMCPConfigs.GetByEnvironmentAndName(envID, configName)
	if err != nil {
		return fmt.Errorf("config '%s' not found: %w", configName, err)
	}

	// Delete database record (this will cascade to tools)
	err = h.repos.FileMCPConfigs.Delete(fileConfig.ID)
	if err != nil {
		return fmt.Errorf("failed to delete config record: %w", err)
	}

	fmt.Printf("✅ Deleted config '%s' from database\n", configName)

	// Delete files if not keeping them
	keepFiles, _ := cmd.Flags().GetBool("keep-files")
	if !keepFiles {
		if fileConfig.TemplatePath != "" {
			os.Remove(fileConfig.TemplatePath)
			fmt.Printf("   Deleted template: %s\n", fileConfig.TemplatePath)
		}
		if fileConfig.VariablesPath != "" {
			os.Remove(fileConfig.VariablesPath)
			fmt.Printf("   Deleted variables: %s\n", fileConfig.VariablesPath)
		}
		if fileConfig.TemplateSpecificVarsPath != "" {
			os.Remove(fileConfig.TemplateSpecificVarsPath)
			fmt.Printf("   Deleted template-specific variables: %s\n", fileConfig.TemplateSpecificVarsPath)
		}
	}

	return nil
}