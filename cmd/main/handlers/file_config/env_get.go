package file_config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// getEnvironmentCommand gets file-based MCP environment details
func (h *FileConfigHandler) getEnvironmentCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Get file-based MCP environment details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envName := args[0]
			
			// Check file config directory
			configDir := fmt.Sprintf("./config/environments/%s", envName)
			stat, err := os.Stat(configDir)
			if err != nil {
				return fmt.Errorf("environment '%s' not found at %s", envName, configDir)
			}
			
			fmt.Printf("File-based Environment Details:\n")
			fmt.Printf("══════════════════════════════\n\n")
			fmt.Printf("Name: %s\n", envName)
			fmt.Printf("Directory: %s\n", configDir)
			fmt.Printf("Created: %s\n", stat.ModTime().Format("Jan 2, 2006 15:04"))
			
			// List config files
			if files, err := os.ReadDir(configDir); err == nil {
				configFiles := []string{}
				varFiles := []string{}
				otherFiles := []string{}
				
				for _, file := range files {
					if file.IsDir() {
						continue
					}
					
					fileName := file.Name()
					if strings.HasSuffix(fileName, ".json") || strings.HasSuffix(fileName, ".yaml") {
						configFiles = append(configFiles, fileName)
					} else if strings.HasSuffix(fileName, ".vars.yml") || strings.HasSuffix(fileName, ".env") {
						varFiles = append(varFiles, fileName)
					} else {
						otherFiles = append(otherFiles, fileName)
					}
				}
				
				if len(configFiles) > 0 {
					fmt.Printf("\nConfiguration Templates:\n")
					for _, file := range configFiles {
						fmt.Printf("  📄 %s\n", file)
					}
				}
				
				if len(varFiles) > 0 {
					fmt.Printf("\nVariable Files:\n")
					for _, file := range varFiles {
						fmt.Printf("  🔧 %s\n", file)
					}
				}
				
				if len(otherFiles) > 0 {
					fmt.Printf("\nOther Files:\n")
					for _, file := range otherFiles {
						fmt.Printf("  📋 %s\n", file)
					}
				}
				
				if len(configFiles) == 0 && len(varFiles) == 0 && len(otherFiles) == 0 {
					fmt.Printf("\nNo configuration files found.\n")
					fmt.Printf("Run 'stn mcp init %s' to create sample configurations.\n", envName)
				}
			}
			
			return nil
		},
	}
}