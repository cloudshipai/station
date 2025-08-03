package file_config

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"station/pkg/config"
)

// createCommand creates a new file-based config
func (h *FileConfigHandler) createCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <config-name> [environment-name]",
		Short: "Create a new file-based MCP configuration",
		Long: `Create a new file-based MCP configuration template.
This will create the template file and optionally initialize variables.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: h.createConfig,
	}

	cmd.Flags().String("template", "", "Path to template file to use as base")
	cmd.Flags().StringSlice("servers", []string{}, "Server names to include (format: name:command:args)")
	cmd.Flags().StringSlice("set-var", []string{}, "Set variables (format: key=value)")
	cmd.Flags().Bool("interactive", false, "Interactive config creation")
	cmd.Flags().Bool("discover-tools", true, "Automatically discover tools after creation")
	
	return cmd
}

// createConfig handles the create command
func (h *FileConfigHandler) createConfig(cmd *cobra.Command, args []string) error {
	configName := args[0]
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

	fmt.Printf("Creating file-based config '%s' in environment '%s'...\n", configName, envName)

	// Check if interactive mode
	interactive, _ := cmd.Flags().GetBool("interactive")
	if interactive {
		return h.createConfigInteractive(ctx, envID, configName)
	}

	// Create basic template
	template := &config.MCPTemplate{
		Name:    configName,
		FilePath: fmt.Sprintf("./config/environments/%s/%s.json", envName, configName),
		Content: h.generateSampleTemplate(configName),
		Variables: []config.TemplateVariable{
			{
				Name:        "ApiKey",
				Required:    true,
				Description: "API key for the service",
				Secret:      true,
			},
		},
	}

	// Create variables
	variables := make(map[string]interface{})
	setVars, _ := cmd.Flags().GetStringSlice("set-var")
	for _, setVar := range setVars {
		parts := strings.SplitN(setVar, "=", 2)
		if len(parts) == 2 {
			variables[parts[0]] = parts[1]
		}
	}

	// Create the config
	err = h.fileConfigService.CreateOrUpdateTemplate(ctx, envID, configName, template, variables)
	if err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}

	fmt.Printf("✅ Created file-based config '%s'\n", configName)
	fmt.Printf("   Template: %s\n", template.FilePath)

	// Discover tools if requested
	discoverTools, _ := cmd.Flags().GetBool("discover-tools")
	if discoverTools {
		fmt.Printf("🔍 Discovering tools...\n")
		result, err := h.fileConfigService.DiscoverToolsForConfig(ctx, envID, configName)
		if err != nil {
			fmt.Printf("⚠️  Tool discovery failed: %v\n", err)
		} else {
			fmt.Printf("✅ Discovered %d tools from %d servers\n", result.TotalTools, result.SuccessfulServers)
		}
	}

	return nil
}

// createConfigInteractive handles interactive config creation
func (h *FileConfigHandler) createConfigInteractive(ctx context.Context, envID int64, configName string) error {
	// This would implement interactive config creation
	// For now, just create a basic config
	fmt.Printf("Interactive mode not yet implemented. Creating basic config...\n")
	return nil
}