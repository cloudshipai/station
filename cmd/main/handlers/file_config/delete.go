package file_config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"station/internal/db/repositories"
)

// deleteCommand deletes a file-based config
func (h *FileConfigHandler) deleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <config-name-or-id> [environment-name]",
		Short: "Delete a file-based MCP configuration",
		Long:  "Delete a file-based MCP configuration by name or ID and clean up associated files.",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  h.deleteConfig,
	}

	cmd.Flags().Bool("keep-files", false, "Keep template and variable files")
	cmd.Flags().Bool("force", false, "Force deletion without confirmation")
	
	return cmd
}

// deleteConfig handles the delete command
func (h *FileConfigHandler) deleteConfig(cmd *cobra.Command, args []string) error {
	configNameOrID := args[0]
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

	// Find config (try by name first, then by ID)
	var fileConfig *repositories.FileConfigRecord
	if configByName, err := h.repos.FileMCPConfigs.GetByEnvironmentAndName(envID, configNameOrID); err == nil {
		fileConfig = configByName
	} else {
		// Try parsing as ID
		if id, parseErr := strconv.ParseInt(configNameOrID, 10, 64); parseErr == nil {
			if configByID, err := h.repos.FileMCPConfigs.GetByID(id); err == nil {
				fileConfig = configByID
			}
		}
	}

	if fileConfig == nil {
		return fmt.Errorf("config '%s' not found", configNameOrID)
	}

	force, _ := cmd.Flags().GetBool("force")
	if !force {
		fmt.Printf("Are you sure you want to delete config '%s' (ID: %d) from environment '%s'? (y/N): ", fileConfig.ConfigName, fileConfig.ID, envName)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Deletion cancelled")
			return nil
		}
	}

	// Delete associated tools first (since they reference the file config)
	err = h.repos.MCPTools.DeleteByFileConfigID(fileConfig.ID)
	if err != nil {
		return fmt.Errorf("failed to delete associated tools: %w", err)
	}

	// Delete database record
	err = h.repos.FileMCPConfigs.Delete(fileConfig.ID)
	if err != nil {
		return fmt.Errorf("failed to delete config record: %w", err)
	}

	fmt.Printf("✅ Deleted config '%s' (ID: %d) and associated tools from database\n", fileConfig.ConfigName, fileConfig.ID)

	// Delete files if not keeping them
	keepFiles, _ := cmd.Flags().GetBool("keep-files")
	if !keepFiles {
		// Build actual file paths using XDG config directory structure
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			configHome = filepath.Join(os.Getenv("HOME"), ".config")
		}
		envDir := filepath.Join(configHome, "station", "environments", envName)
		
		// Delete template file
		templatePath := filepath.Join(envDir, fileConfig.ConfigName+".json")
		if err := os.Remove(templatePath); err == nil {
			fmt.Printf("   Deleted template: %s\n", templatePath)
		}
		
		// Note: Global variables.yml is shared across configs, so we don't delete it
		// Only template-specific variable files would be deleted if they existed
		templateVarsPath := filepath.Join(envDir, fileConfig.ConfigName+".vars.yml")
		if err := os.Remove(templateVarsPath); err == nil {
			fmt.Printf("   Deleted template variables: %s\n", templateVarsPath)
		}
	}

	return nil
}