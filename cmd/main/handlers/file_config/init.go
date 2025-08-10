package file_config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// initCommand initializes file config structure
func (h *FileConfigHandler) initCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [environment-name]",
		Short: "Initialize file-based configuration structure",
		Long:  "Create the directory structure and sample files for file-based configurations.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  h.initStructure,
	}

	cmd.Flags().String("config-dir", "./config", "Configuration directory")
	cmd.Flags().Bool("create-sample", true, "Create sample configuration files")
	
	return cmd
}

// initStructure handles the init command
func (h *FileConfigHandler) initStructure(cmd *cobra.Command, args []string) error {
	envName := "default"
	if len(args) > 0 {
		envName = args[0]
	}

	configDir, _ := cmd.Flags().GetString("config-dir")
	createSample, _ := cmd.Flags().GetBool("create-sample")

	fmt.Printf("Initializing file-based config structure for environment '%s'...\n", envName)

	// Create directory structure
	envDir := filepath.Join(configDir, "environments", envName)
	err := os.MkdirAll(envDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	fmt.Printf("✅ Created directory: %s\n", envDir)

	// Create variables directory
	varsDir := filepath.Join(configDir, "vars")
	err = os.MkdirAll(varsDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create variables directory: %w", err)
	}

	fmt.Printf("✅ Created directory: %s\n", varsDir)

	if createSample {
		// Create sample config
		samplePath := filepath.Join(envDir, "github.json")
		sampleContent := h.generateSampleTemplate("github")
		err = os.WriteFile(samplePath, []byte(sampleContent), 0644)
		if err != nil {
			fmt.Printf("⚠️  Failed to create sample config: %v\n", err)
		} else {
			fmt.Printf("✅ Created sample config: %s\n", samplePath)
		}

		// Create sample variables
		varsPath := filepath.Join(envDir, "variables.yml")
		varsContent := `# Global variables for environment: ` + envName + `
github_token: "your-github-token-here"
github_org: "your-org"
`
		err = os.WriteFile(varsPath, []byte(varsContent), 0644)
		if err != nil {
			fmt.Printf("⚠️  Failed to create sample variables: %v\n", err)
		} else {
			fmt.Printf("✅ Created sample variables: %s\n", varsPath)
		}

		// Create sample template-specific variables
		templateVarsPath := filepath.Join(envDir, "github.vars.yml")
		templateVarsContent := `# Template-specific variables for github config
# These override global variables when rendering the github template
github_token: "github-specific-token"
github_repo: "specific-repo"
`
		err = os.WriteFile(templateVarsPath, []byte(templateVarsContent), 0644)
		if err != nil {
			fmt.Printf("⚠️  Failed to create template variables: %v\n", err)
		} else {
			fmt.Printf("✅ Created template-specific variables: %s\n", templateVarsPath)
		}

		fmt.Printf("\n📖 Next steps:\n")
		fmt.Printf("   1. Edit %s with your template\n", samplePath)
		fmt.Printf("   2. Update variables in %s\n", varsPath)
		fmt.Printf("   3. Run 'stn mcp discover github %s' to test\n", envName)
	}

	return nil
}

// createSampleConfigs creates sample configuration files for an environment
func (h *FileConfigHandler) createSampleConfigs(envName, configDir string) error {
	// Create sample github config
	sampleConfig := h.generateSampleTemplate("github")
	samplePath := filepath.Join(configDir, "github.json")
	if err := os.WriteFile(samplePath, []byte(sampleConfig), 0644); err != nil {
		return fmt.Errorf("failed to create sample config: %w", err)
	}
	
	// Create sample variables file
	varsContent := fmt.Sprintf(`# Global variables for environment: %s
github_token: "your-github-token-here"
github_org: "your-org"
`, envName)
	varsPath := filepath.Join(configDir, "variables.yml")
	if err := os.WriteFile(varsPath, []byte(varsContent), 0644); err != nil {
		return fmt.Errorf("failed to create variables file: %w", err)
	}
	
	return nil
}