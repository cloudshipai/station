package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/afero"
	
	internalconfig "station/internal/config"
	"station/internal/filesystem"
	"station/internal/template"
	"station/internal/variables"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/config"
)

// FileConfigHandler handles file-based MCP configuration CLI commands
type FileConfigHandler struct {
	// hybridConfigService removed - using file-based configs only
	fileConfigService   *services.FileConfigService
	repos               *repositories.Repositories
}

// NewFileConfigHandler creates a new file config handler
func NewFileConfigHandler() *FileConfigHandler {
	// Get database path directly from config file or use fallback
	databasePath := getConfigDatabasePath()
	
	database, err := db.New(databasePath)
	if err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		os.Exit(1)
	}

	repos := repositories.New(database)
	
	// Initialize file config components
	fs := afero.NewOsFs()
	fileSystem := filesystem.NewConfigFileSystem(fs, "./config", "./config/vars")
	templateEngine := template.NewGoTemplateEngine()
	variableStore := variables.NewEnvVariableStore(fs)
	
	// Create file config options
	fileConfigOptions := config.FileConfigOptions{
		ConfigDir:       "./config",
		VariablesDir:    "./config/vars",
		Strategy:        config.StrategyTemplateFirst,
		AutoCreate:      true,
		BackupOnChange:  false,
		ValidateOnLoad:  true,
	}
	
	// Create file config manager
	fileConfigManager := internalconfig.NewFileConfigManager(
		fileSystem,
		templateEngine,
		variableStore,
		fileConfigOptions,
		repos.Environments,
	)
	
	// Initialize services (updated for file-based configs only)
	toolDiscoveryService := services.NewToolDiscoveryService(repos)
	fileConfigService := services.NewFileConfigService(fileConfigManager, toolDiscoveryService, repos)
	
	return &FileConfigHandler{
		fileConfigService:   fileConfigService,
		repos:               repos,
	}
}

// getConfigDatabasePath gets the database path from config or returns fallback
func getConfigDatabasePath() string {
	// Try to read from the expected config file location
	configPath := os.ExpandEnv("$HOME/.config/station/config.yaml")
	if data, err := os.ReadFile(configPath); err == nil {
		// Simple parsing to extract database_url
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "database_url:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}
	
	// Fallback to local database
	return "station.db"
}


// RegisterCommands registers all file config commands (deprecated - use RegisterMCPCommands)
func (h *FileConfigHandler) RegisterCommands(rootCmd *cobra.Command) {
	// Create the file-config command group
	fileConfigCmd := &cobra.Command{
		Use:   "file-config",
		Short: "Manage file-based MCP configurations",
		Long: `File-based configuration management for MCP servers.
Supports GitOps workflows with template-specific variable resolution.`,
	}

	// Add subcommands
	fileConfigCmd.AddCommand(h.listCommand())
	fileConfigCmd.AddCommand(h.createCommand())
	fileConfigCmd.AddCommand(h.updateCommand())
	fileConfigCmd.AddCommand(h.deleteCommand())
	fileConfigCmd.AddCommand(h.validateCommand())
	fileConfigCmd.AddCommand(h.discoverCommand())
	fileConfigCmd.AddCommand(h.statusCommand())
	fileConfigCmd.AddCommand(h.initCommand())
	fileConfigCmd.AddCommand(h.variablesCommand())
	
	rootCmd.AddCommand(fileConfigCmd)
}

// RegisterMCPCommands integrates file-based commands into the mcp command structure
func (h *FileConfigHandler) RegisterMCPCommands(mcpCmd *cobra.Command) {
	// Add file-based subcommands to existing mcp command with single-word names
	createCmd := h.createCommand()
	createCmd.Use = "create <config-name> [environment-name]"
	createCmd.Short = "Create a new MCP configuration from template"
	mcpCmd.AddCommand(createCmd)
	
	mcpCmd.AddCommand(h.updateCommand()) 
	mcpCmd.AddCommand(h.validateCommand())
	
	// Rename discover to single word
	discoverCmd := h.discoverCommand()
	discoverCmd.Use = "discover <config-name> [environment-name]"
	discoverCmd.Short = "Discover and register tools for an MCP configuration"
	mcpCmd.AddCommand(discoverCmd)
	
	// Commented out to avoid conflict with new sync/status commands
	// mcpCmd.AddCommand(h.statusCommand())
	mcpCmd.AddCommand(h.initCommand())
	mcpCmd.AddCommand(h.variablesCommand())
	
	// Add environment management under MCP
	mcpCmd.AddCommand(h.environmentsCommand())
	
	// Update the existing list command to support both file and database configs
	// We'll modify the existing mcp list to include file configs by default
}

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

// updateCommand updates an existing file-based config
func (h *FileConfigHandler) updateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <config-name> [environment-name]",
		Short: "Update a file-based MCP configuration",
		Long:  "Update an existing file-based MCP configuration template or variables.",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  h.updateConfig,
	}

	cmd.Flags().String("template", "", "Path to new template file")
	cmd.Flags().StringSlice("set-var", []string{}, "Set variables (format: key=value)")
	cmd.Flags().Bool("discover-tools", true, "Rediscover tools after update")
	
	return cmd
}

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

// validateCommand validates file-based configs
func (h *FileConfigHandler) validateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [config-name] [environment-name]",
		Short: "Validate file-based MCP configurations",
		Long:  "Validate template syntax, variable resolution, and config structure.",
		Args:  cobra.MaximumNArgs(2),
		RunE:  h.validateConfigs,
	}

	cmd.Flags().Bool("check-vars", true, "Check for missing variables")
	cmd.Flags().Bool("dry-run", false, "Perform dry-run rendering")
	
	return cmd
}

// discoverCommand discovers tools for file configs
func (h *FileConfigHandler) discoverCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover <config-name> [environment-name]",
		Short: "Discover tools for a file-based configuration",
		Long:  "Load, render, and discover MCP tools for a file-based configuration.",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  h.discoverTools,
	}

	cmd.Flags().Bool("verbose", false, "Verbose output during discovery")
	cmd.Flags().Int("timeout", 30, "Discovery timeout in seconds")
	
	return cmd
}

// statusCommand shows file config status
func (h *FileConfigHandler) statusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [environment-name]",
		Short: "Show file-based configuration status",
		Long:  "Show the status of file-based configurations including change detection.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  h.showStatus,
	}

	cmd.Flags().Bool("check-changes", true, "Check for file changes")
	cmd.Flags().Bool("tool-counts", true, "Include tool counts")
	
	return cmd
}

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

// variablesCommand manages variables
func (h *FileConfigHandler) variablesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variables",
		Short: "Manage configuration variables",
		Long:  "Manage global and template-specific variables for file-based configurations.",
	}

	// Add variables subcommands
	cmd.AddCommand(h.listVariablesCommand())
	cmd.AddCommand(h.setVariableCommand())
	cmd.AddCommand(h.getVariableCommand())
	cmd.AddCommand(h.deleteVariableCommand())
	
	return cmd
}

// Command implementations

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

func (h *FileConfigHandler) updateConfig(cmd *cobra.Command, args []string) error {
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

	fmt.Printf("Updating file-based config '%s' in environment '%s'...\n", configName, envName)

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

func (h *FileConfigHandler) validateConfigs(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
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

	// Validate specific config or all configs
	if len(args) > 0 && args[0] != "" {
		return h.validateSingleConfig(ctx, envID, args[0], cmd)
	}

	// Validate all configs
	configs, err := h.fileConfigService.ListFileConfigs(ctx, envID)
	if err != nil {
		return fmt.Errorf("failed to list configs: %w", err)
	}

	fmt.Printf("Validating %d file-based configurations...\n", len(configs))
	
	valid := 0
	for _, config := range configs {
		fmt.Printf("  %s... ", config.Name)
		err := h.validateSingleConfig(ctx, envID, config.Name, cmd)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Printf("✅\n")
			valid++
		}
	}

	fmt.Printf("\n%d/%d configurations are valid\n", valid, len(configs))
	return nil
}

func (h *FileConfigHandler) discoverTools(cmd *cobra.Command, args []string) error {
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

	verbose, _ := cmd.Flags().GetBool("verbose")
	
	fmt.Printf("🔍 Discovering tools for config '%s' in environment '%s'...\n", configName, envName)
	
	if verbose {
		fmt.Printf("   1. Loading template...\n")
		fmt.Printf("   2. Resolving variables...\n")
		fmt.Printf("   3. Rendering configuration...\n")
		fmt.Printf("   4. Connecting to MCP servers...\n")
		fmt.Printf("   5. Discovering tools...\n")
	}

	result, err := h.fileConfigService.DiscoverToolsForConfig(ctx, envID, configName)
	if err != nil {
		return fmt.Errorf("tool discovery failed: %w", err)
	}

	// Display results
	if result.Success {
		fmt.Printf("✅ Tool discovery completed successfully!\n")
		fmt.Printf("   Servers processed: %d/%d\n", result.SuccessfulServers, result.TotalServers)
		fmt.Printf("   Tools discovered: %d\n", result.TotalTools)
		fmt.Printf("   Duration: %v\n", result.CompletedAt.Sub(result.StartedAt))
	} else {
		fmt.Printf("⚠️  Tool discovery completed with issues\n")
		fmt.Printf("   Successful servers: %d/%d\n", result.SuccessfulServers, result.TotalServers)
		fmt.Printf("   Tools discovered: %d\n", result.TotalTools)
		fmt.Printf("   Errors: %d\n", len(result.Errors))
	}

	if len(result.Errors) > 0 && verbose {
		fmt.Printf("\nErrors encountered:\n")
		for _, err := range result.Errors {
			fmt.Printf("   - %s: %s\n", err.ServerName, err.Message)
		}
	}

	return nil
}

func (h *FileConfigHandler) showStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
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

	fmt.Printf("File-based Configuration Status - Environment: %s\n", envName)
	fmt.Printf("═══════════════════════════════════════════════════\n\n")

	// Get file configs
	configs, err := h.fileConfigService.ListFileConfigs(ctx, envID)
	if err != nil {
		return fmt.Errorf("failed to list configs: %w", err)
	}

	if len(configs) == 0 {
		fmt.Printf("No file-based configurations found.\n")
		fmt.Printf("Run 'stn file-config init' to set up the structure.\n")
		return nil
	}

	// Display each config status
	for i, config := range configs {
		if i > 0 {
			fmt.Printf("\n")
		}
		
		fmt.Printf("📄 %s\n", config.Name)
		fmt.Printf("   Type: %s\n", config.Type)
		fmt.Printf("   Path: %s\n", config.Path)
		
		if config.Metadata != nil {
			if lastLoaded, ok := config.Metadata["last_loaded"]; ok {
				fmt.Printf("   Last loaded: %s\n", lastLoaded)
			}
			if templateHash, ok := config.Metadata["template_hash"]; ok {
				fmt.Printf("   Template hash: %s\n", templateHash[:12]+"...")
			}
		}

		// Show tool count if requested
		toolCounts, _ := cmd.Flags().GetBool("tool-counts")
		if toolCounts {
			// This would require getting the file config record and counting tools
			fmt.Printf("   Tools: (counting...)\n")
		}
	}

	return nil
}

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

// Helper methods

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

// displayConfigs - legacy function for ConfigSummary (removed)
/*
func (h *FileConfigHandler) displayConfigs(cmd *cobra.Command, configs []services.ConfigSummary) error {
	format, _ := cmd.Flags().GetString("format")
	
	switch format {
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
		
		fmt.Printf("%-20s %-10s %-10s %-15s %s\n", "NAME", "TYPE", "VERSION", "SOURCE", "PATH")
		fmt.Printf("%-20s %-10s %-10s %-15s %s\n", "────", "────", "───────", "──────", "────")
		
		for _, config := range configs {
			version := "-"
			if config.Version > 0 {
				version = fmt.Sprintf("v%d", config.Version)
			}
			
			path := config.Path
			if path == "" {
				path = "-"
			}
			
			fmt.Printf("%-20s %-10s %-10s %-15s %s\n", 
				config.Name, 
				config.Type, 
				version,
				config.Source,
				path,
			)
		}
		
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
	
	return nil
}
*/

func (h *FileConfigHandler) generateSampleTemplate(configName string) string {
	template := `{
  "name": "%s",
  "servers": {
    "%s-server": {
      "command": "node",
      "args": ["/usr/local/lib/node_modules/@modelcontextprotocol/server-github/dist/index.js"],
      "env": {
        "GITHUB_TOKEN": "{{.GithubToken}}",
        "GITHUB_REPO": "{{.GithubRepo}}"
      }
    }
  }
}`
	return fmt.Sprintf(template, configName, configName)
}

func (h *FileConfigHandler) createConfigInteractive(ctx context.Context, envID int64, configName string) error {
	// This would implement interactive config creation
	// For now, just create a basic config
	fmt.Printf("Interactive mode not yet implemented. Creating basic config...\n")
	return nil
}

func (h *FileConfigHandler) validateSingleConfig(ctx context.Context, envID int64, configName string, cmd *cobra.Command) error {
	// Load and try to render the config
	_, err := h.fileConfigService.LoadAndRenderConfig(ctx, envID, configName)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	
	return nil
}

// Variable management subcommands

func (h *FileConfigHandler) listVariablesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list [environment-name]",
		Short: "List configuration variables",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Variable listing not yet implemented\n")
			return nil
		},
	}
}

func (h *FileConfigHandler) setVariableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value> [environment-name]",
		Short: "Set a configuration variable",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Variable setting not yet implemented\n")
			return nil
		},
	}
}

func (h *FileConfigHandler) getVariableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key> [environment-name]",
		Short: "Get a configuration variable",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Variable getting not yet implemented\n")
			return nil
		},
	}
}

func (h *FileConfigHandler) deleteVariableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <key> [environment-name]",
		Short: "Delete a configuration variable",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Variable deletion not yet implemented\n")
			return nil
		},
	}
}

// environmentsCommand manages MCP environments
func (h *FileConfigHandler) environmentsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "environments",
		Short: "Manage MCP environments",
		Long:  "Create, list, update, and delete environments for MCP configurations.",
		Aliases: []string{"env"},
	}

	// Add environment subcommands
	cmd.AddCommand(h.listEnvironmentsCommand())
	cmd.AddCommand(h.createEnvironmentCommand())
	cmd.AddCommand(h.updateEnvironmentCommand())
	cmd.AddCommand(h.deleteEnvironmentCommand())
	cmd.AddCommand(h.getEnvironmentCommand())
	
	return cmd
}

// Environment management subcommands

func (h *FileConfigHandler) listEnvironmentsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List file-based MCP environments",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// List environments from file system
			configDir := "./config/environments"
			
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
				fmt.Printf("  📁 Path: %s\n", envPath)
			}
			
			return nil
		},
	}
}

func (h *FileConfigHandler) createEnvironmentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new file-based MCP environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envName := args[0]
			createSamples, _ := cmd.Flags().GetBool("init-samples")
			
			// Create file structure
			configDir := fmt.Sprintf("./config/environments/%s", envName)
			
			// Check if environment already exists
			if _, err := os.Stat(configDir); err == nil {
				return fmt.Errorf("environment '%s' already exists at %s", envName, configDir)
			}
			
			// Create environment directory
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return fmt.Errorf("failed to create environment directory: %w", err)
			}
			
			// Create variables directory if it doesn't exist
			varsDir := "./config/vars"
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

// getOrCreateEnvironmentID gets environment ID from database, creating if needed
func (h *FileConfigHandler) getOrCreateEnvironmentID(envName string) (int64, error) {
	// Try to get existing environment
	env, err := h.repos.Environments.GetByName(envName)
	if err == nil {
		return env.ID, nil
	}
	
	// Environment doesn't exist, create it
	description := fmt.Sprintf("Auto-created environment for file-based config: %s", envName)
	env, err = h.repos.Environments.Create(envName, &description, 1) // Default user ID 1
	if err != nil {
		return 0, fmt.Errorf("failed to create environment: %w", err)
	}
	
	return env.ID, nil
}

// validateEnvironmentExists checks if file-based environment directory exists
func (h *FileConfigHandler) validateEnvironmentExists(envName string) error {
	configDir := fmt.Sprintf("./config/environments/%s", envName)
	if _, err := os.Stat(configDir); err != nil {
		return fmt.Errorf("environment '%s' not found at %s", envName, configDir)
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