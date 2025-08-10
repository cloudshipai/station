package file_config

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// updateCommand updates an existing file-based config
func (h *FileConfigHandler) updateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <config-name-or-id> [environment-name]",
		Short: "Update a file-based MCP configuration",
		Long:  "Update an existing file-based MCP configuration template or variables by name or ID.",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  h.updateConfig,
	}

	cmd.Flags().String("template", "", "Path to new template file")
	cmd.Flags().StringSlice("set-var", []string{}, "Set variables (format: key=value)")
	cmd.Flags().Bool("discover-tools", true, "Rediscover tools after update")
	
	return cmd
}

// updateConfig handles the update command
func (h *FileConfigHandler) updateConfig(cmd *cobra.Command, args []string) error {
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

	fmt.Printf("Updating file-based config '%s' in environment '%s'...\n", configName, envName)

	// Handle template file update
	templatePath, _ := cmd.Flags().GetString("template")
	if templatePath != "" {
		// Read new template content
		templateContent, err := ioutil.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template file: %w", err)
		}

		// Update the template file directly
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			configHome = filepath.Join(os.Getenv("HOME"), ".config")
		}
		envDir := filepath.Join(configHome, "station", "environments", envName)
		targetPath := filepath.Join(envDir, configName+".json")

		err = ioutil.WriteFile(targetPath, templateContent, 0644)
		if err != nil {
			return fmt.Errorf("failed to update template file: %w", err)
		}

		fmt.Printf("✅ Updated template for config '%s' from %s\n", configName, templatePath)
	}

	// Handle variable updates
	setVars, _ := cmd.Flags().GetStringSlice("set-var")
	if len(setVars) > 0 {
		variables := make(map[string]interface{})
		for _, setVar := range setVars {
			parts := strings.SplitN(setVar, "=", 2)
			if len(parts) == 2 {
				variables[parts[0]] = parts[1]
			}
		}

		err = h.fileConfigService.UpdateTemplateVariables(ctx, envID, configName, variables)
		if err != nil {
			return fmt.Errorf("failed to update variables: %w", err)
		}

		fmt.Printf("✅ Updated variables for config '%s'\n", configName)
	}

	// Rediscover tools if requested
	discoverTools, _ := cmd.Flags().GetBool("discover-tools")
	if discoverTools {
		fmt.Printf("🔍 Rediscovering tools...\n")
		result, err := h.fileConfigService.DiscoverToolsForConfig(ctx, envID, configName)
		if err != nil {
			fmt.Printf("⚠️  Tool discovery failed: %v\n", err)
		} else {
			fmt.Printf("✅ Rediscovered %d tools from %d servers\n", result.TotalTools, result.SuccessfulServers)
		}
	}

	return nil
}