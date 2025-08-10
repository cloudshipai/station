package file_config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// listEnvironmentsCommand lists file-based MCP environments
func (h *FileConfigHandler) listEnvironmentsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List file-based MCP environments",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// List environments from proper config directory
			configHome := os.Getenv("XDG_CONFIG_HOME")
			if configHome == "" {
				configHome = filepath.Join(os.Getenv("HOME"), ".config")
			}
			configDir := filepath.Join(configHome, "station", "environments")
			
			// Check if config directory exists
			if _, err := os.Stat(configDir); os.IsNotExist(err) {
				fmt.Printf("No file-based environments found.\n")
				fmt.Printf("Run 'stn init' to initialize the default environment.\n")
				return nil
			}
			
			// Read environment directories
			entries, err := os.ReadDir(configDir)
			if err != nil {
				return fmt.Errorf("failed to read environments directory: %w", err)
			}
			
			fmt.Printf("File-based MCP Environments:\n")
			fmt.Printf("═══════════════════════════\n\n")
			
			envs := []string{}
			for _, entry := range entries {
				if entry.IsDir() {
					envs = append(envs, entry.Name())
				}
			}
			
			if len(envs) == 0 {
				fmt.Printf("No environments found. Create one with 'stn mcp env create <name>'\n")
				return nil
			}
			
			for _, envName := range envs {
				envPath := filepath.Join(configDir, envName)
				info, err := os.Stat(envPath)
				if err != nil {
					continue
				}
				
				fmt.Printf("• %s", envName)
				fmt.Printf(" [Created: %s]\n", info.ModTime().Format("Jan 2, 2006 15:04"))
				
				// List config files
				if files, err := os.ReadDir(envPath); err == nil {
					configFiles := []string{}
					for _, file := range files {
						if !file.IsDir() && (strings.HasSuffix(file.Name(), ".json") || strings.HasSuffix(file.Name(), ".yaml")) {
							configFiles = append(configFiles, file.Name())
						}
					}
					if len(configFiles) > 0 {
						fmt.Printf("  📄 Configs: %s\n", strings.Join(configFiles, ", "))
					}
				}
				fmt.Printf("  📁 Path: %s\n", filepath.Join(configDir, envName))
			}
			
			return nil
		},
	}
}