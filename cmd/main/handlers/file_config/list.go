package file_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"station/pkg/config"
)

// listCommand lists all file-based configs
func (h *FileConfigHandler) listCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [environment-name]",
		Short: "List file-based MCP configurations",
		Long:  "List all file-based MCP configurations for an environment.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  h.listConfigs,
	}

	cmd.Flags().String("format", "table", "Output format: table, json, yaml")
	cmd.Flags().Bool("include-db", false, "Include database configs in output")
	
	return cmd
}

// listConfigs handles the list command
func (h *FileConfigHandler) listConfigs(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
	// Get environment
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

	// Legacy database configs no longer supported - file-based configs only
	includeDB, _ := cmd.Flags().GetBool("include-db")
	if includeDB {
		fmt.Println("Warning: --include-db flag ignored - database configs no longer supported")
	}

	// Get only file configs
	fileConfigs, err := h.fileConfigService.ListFileConfigs(ctx, envID)
	if err != nil {
		return fmt.Errorf("failed to list file configs: %w", err)
	}

	// Display file configs directly
	return h.displayFileConfigs(cmd, fileConfigs)
}

// displayFileConfigs displays file configs in the requested format
func (h *FileConfigHandler) displayFileConfigs(cmd *cobra.Command, configs []config.ConfigInfo) error {
	outputFormat, _ := cmd.Flags().GetString("output")
	
	switch outputFormat {
	case "json":
		data, err := json.MarshalIndent(configs, "", "  ")
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", data)
		
	case "table":
		if len(configs) == 0 {
			fmt.Printf("No configurations found.\n")
			return nil
		}
		
		fmt.Printf("%-20s %-10s %-15s %s\n", "NAME", "TYPE", "SOURCE", "PATH")
		fmt.Printf("%-20s %-10s %-15s %s\n", "────", "────", "──────", "────")
		
		for _, config := range configs {
			fmt.Printf("%-20s %-10s %-15s %s\n", 
				config.Name, "file", "file", config.Path)
		}
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
	
	return nil
}