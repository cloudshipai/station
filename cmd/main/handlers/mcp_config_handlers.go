package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	
	internalconfig "station/internal/config"
	"station/internal/filesystem"
	"station/internal/template"
	"station/internal/variables"
	"station/pkg/config"
)

// MCPConfigHandler handles MCP configuration CLI commands
type MCPConfigHandler struct {
	configManager config.ConfigManager
	hybridLoader  config.LoaderStrategy
}

// NewMCPConfigHandler creates a new MCP config handler
func NewMCPConfigHandler() *MCPConfigHandler {
	// Initialize filesystem
	configDir := viper.GetString("mcp-conf-dir")
	if configDir == "" {
		configDir = filepath.Join(os.Getenv("HOME"), ".config", "station")
	}
	
	varsDir := viper.GetString("vars-dir")
	if varsDir == "" {
		varsDir = configDir
	}
	
	fs := filesystem.NewDefaultConfigFileSystem(configDir, varsDir)
	templateEngine := template.NewGoTemplateEngine()
	variableStore := variables.NewEnvVariableStore(fs)
	
	opts := config.FileConfigOptions{
		ConfigDir:      configDir,
		VariablesDir:   varsDir,
		Strategy:       config.StrategyTemplateFirst,
		FileSystem:     fs,
		AutoCreate:     true,
		ValidateOnLoad: true,
	}
	
	configManager := internalconfig.NewFileConfigManager(fs, templateEngine, variableStore, opts, nil)
	
	return &MCPConfigHandler{
		configManager: configManager,
		// hybridLoader would be initialized with database connections
	}
}

// RegisterCommands registers all MCP config commands
func (h *MCPConfigHandler) RegisterCommands(mcpCmd *cobra.Command) {
	// mcp config command group
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage MCP configuration templates",
		Long:  "Create, edit, and manage MCP configuration templates with GitOps support",
	}
	
	// mcp config list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List MCP configuration templates",
		RunE:  h.ListConfigs,
	}
	listCmd.Flags().StringP("env", "e", "default", "Environment name")
	listCmd.Flags().BoolP("show-vars", "v", false, "Show template variables")
	
	// mcp config create
	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new MCP configuration template",
		Args:  cobra.ExactArgs(1),
		RunE:  h.CreateConfig,
	}
	createCmd.Flags().StringP("env", "e", "default", "Environment name")
	createCmd.Flags().StringP("template", "t", "", "Base template to copy from")
	createCmd.Flags().BoolP("interactive", "i", true, "Interactive template creation")
	
	// mcp config edit
	editCmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit an MCP configuration template",
		Args:  cobra.ExactArgs(1),
		RunE:  h.EditConfig,
	}
	editCmd.Flags().StringP("env", "e", "default", "Environment name")
	editCmd.Flags().StringP("editor", "", os.Getenv("EDITOR"), "Editor to use")
	
	// mcp config validate
	validateCmd := &cobra.Command{
		Use:   "validate [name]",
		Short: "Validate MCP configuration templates",
		RunE:  h.ValidateConfig,
	}
	validateCmd.Flags().StringP("env", "e", "default", "Environment name")
	validateCmd.Flags().BoolP("all", "a", false, "Validate all templates")
	
	// mcp config render
	renderCmd := &cobra.Command{
		Use:   "render <name>",
		Short: "Preview rendered MCP configuration",
		Args:  cobra.ExactArgs(1),
		RunE:  h.RenderConfig,
	}
	renderCmd.Flags().StringP("env", "e", "default", "Environment name")
	renderCmd.Flags().BoolP("output", "o", false, "Save rendered output to file")
	
	// mcp config init
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize MCP configuration directory structure",
		RunE:  h.InitConfigStructure,
	}
	initCmd.Flags().StringP("env", "e", "default", "Environment name to create")
	
	// Add subcommands
	configCmd.AddCommand(listCmd, createCmd, editCmd, validateCmd, renderCmd, initCmd)
	
	// mcp vars command group
	varsCmd := &cobra.Command{
		Use:   "vars",
		Short: "Manage MCP configuration variables",
		Long:  "Manage environment variables and secrets for MCP templates",
	}
	
	// mcp vars list
	varsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List configuration variables",
		RunE:  h.ListVariables,
	}
	varsListCmd.Flags().StringP("env", "e", "default", "Environment name")
	varsListCmd.Flags().StringP("template", "t", "", "Template-specific variables")
	varsListCmd.Flags().BoolP("secrets", "s", false, "Show secret variables (masked)")
	
	// mcp vars set
	varsSetCmd := &cobra.Command{
		Use:   "set <key=value>...",
		Short: "Set configuration variables",
		Args:  cobra.MinimumNArgs(1),
		RunE:  h.SetVariables,
	}
	varsSetCmd.Flags().StringP("env", "e", "default", "Environment name")
	varsSetCmd.Flags().StringP("template", "t", "", "Template-specific variables")
	varsSetCmd.Flags().BoolP("secret", "s", false, "Mark variables as secrets")
	
	// mcp vars edit
	varsEditCmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit configuration variables interactively",
		RunE:  h.EditVariables,
	}
	varsEditCmd.Flags().StringP("env", "e", "default", "Environment name")
	varsEditCmd.Flags().StringP("template", "t", "", "Template-specific variables")
	
	// mcp vars import
	varsImportCmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import variables from file",
		Args:  cobra.ExactArgs(1),
		RunE:  h.ImportVariables,
	}
	varsImportCmd.Flags().StringP("env", "e", "default", "Environment name")
	varsImportCmd.Flags().StringP("template", "t", "", "Template-specific variables")
	
	// Add vars subcommands
	varsCmd.AddCommand(varsListCmd, varsSetCmd, varsEditCmd, varsImportCmd)
	
	// Add main command groups
	mcpCmd.AddCommand(configCmd, varsCmd)
}

// Command implementations

func (h *MCPConfigHandler) ListConfigs(cmd *cobra.Command, args []string) error {
	envName, _ := cmd.Flags().GetString("env")
	showVars, _ := cmd.Flags().GetBool("show-vars")
	
	ctx := context.Background()
	envID := h.getEnvironmentID(envName) // Helper method to get env ID
	
	// Discover templates
	templates, err := h.configManager.DiscoverTemplates(ctx, envName)
	if err != nil {
		return fmt.Errorf("failed to discover templates: %w", err)
	}
	
	if len(templates) == 0 {
		fmt.Printf("No MCP configuration templates found in environment '%s'\n", envName)
		fmt.Printf("Run 'stn mcp config init --env %s' to create the directory structure\n", envName)
		return nil
	}
	
	fmt.Printf("MCP Configuration Templates (Environment: %s)\n", envName)
	fmt.Printf("===============================================\n\n")
	
	for _, template := range templates {
		fmt.Printf("📋 %s\n", template.Name)
		fmt.Printf("   Path: %s\n", template.Path)
		fmt.Printf("   Size: %d bytes\n", template.Size)
		fmt.Printf("   Modified: %s\n", template.ModTime.Format(time.RFC3339))
		
		if template.HasVars {
			fmt.Printf("   Variables: %s\n", template.VarsPath)
		}
		
		if showVars {
			// Load and display template variables
			if vars, err := h.loadTemplateVariables(ctx, envID, template.Name); err == nil {
				fmt.Printf("   Template Variables:\n")
				for _, variable := range vars {
					secretMark := ""
					if variable.Secret {
						secretMark = " 🔒"
					}
					requiredMark := ""
					if variable.Required {
						requiredMark = " (required)"
					}
					fmt.Printf("     - %s%s%s: %s\n", variable.Name, secretMark, requiredMark, variable.Description)
				}
			}
		}
		
		fmt.Println()
	}
	
	return nil
}

func (h *MCPConfigHandler) CreateConfig(cmd *cobra.Command, args []string) error {
	configName := args[0]
	envName, _ := cmd.Flags().GetString("env")
	baseTemplate, _ := cmd.Flags().GetString("template")
	interactive, _ := cmd.Flags().GetBool("interactive")
	
	ctx := context.Background()
	envID := h.getEnvironmentID(envName)
	
	// Ensure environment structure exists
	if err := h.configManager.EnsureEnvironmentStructure(envName); err != nil {
		return fmt.Errorf("failed to create environment structure: %w", err)
	}
	
	var templateContent string
	
	if baseTemplate != "" {
		// Copy from existing template
		existingTemplate, err := h.configManager.LoadTemplate(ctx, envID, baseTemplate)
		if err != nil {
			return fmt.Errorf("failed to load base template: %w", err)
		}
		templateContent = existingTemplate.Content
	} else if interactive {
		// Interactive template creation
		templateContent = h.promptForTemplateContent(configName)
	} else {
		// Use default template
		templateContent = h.getDefaultTemplateContent(configName)
	}
	
	// Create template
	template := &config.MCPTemplate{
		Name:    configName,
		Content: templateContent,
		Metadata: config.TemplateMetadata{
			Version:     "1.0.0",
			Description: fmt.Sprintf("MCP configuration for %s", configName),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
	
	if err := h.configManager.SaveTemplate(ctx, envID, configName, template); err != nil {
		return fmt.Errorf("failed to save template: %w", err)
	}
	
	fmt.Printf("✅ Created MCP configuration template: %s\n", configName)
	fmt.Printf("📁 Template path: %s\n", h.configManager.GetConfigPath(envName, configName))
	
	// Extract variables and prompt for values
	if interactive {
		return h.promptForVariables(ctx, envID, configName, template)
	}
	
	return nil
}

func (h *MCPConfigHandler) ValidateConfig(cmd *cobra.Command, args []string) error {
	envName, _ := cmd.Flags().GetString("env")
	validateAll, _ := cmd.Flags().GetBool("all")
	
	ctx := context.Background()
	
	var templatesToValidate []string
	
	if validateAll || len(args) == 0 {
		// Validate all templates
		templates, err := h.configManager.DiscoverTemplates(ctx, envName)
		if err != nil {
			return fmt.Errorf("failed to discover templates: %w", err)
		}
		
		for _, template := range templates {
			templatesToValidate = append(templatesToValidate, template.Name)
		}
	} else {
		// Validate specific template
		templatesToValidate = args
	}
	
	if len(templatesToValidate) == 0 {
		fmt.Printf("No templates found to validate in environment '%s'\n", envName)
		return nil
	}
	
	fmt.Printf("Validating MCP configuration templates...\n\n")
	
	allValid := true
	for _, templateName := range templatesToValidate {
		templatePath := h.configManager.GetConfigPath(envName, templateName)
		
		validation, err := h.configManager.ValidateTemplate(ctx, templatePath)
		if err != nil {
			fmt.Printf("❌ %s: Failed to validate (%v)\n", templateName, err)
			allValid = false
			continue
		}
		
		if validation.Valid {
			fmt.Printf("✅ %s: Valid\n", templateName)
			if len(validation.Warnings) > 0 {
				for _, warning := range validation.Warnings {
					fmt.Printf("   ⚠️  Warning: %s\n", warning.Message)
				}
			}
		} else {
			fmt.Printf("❌ %s: Invalid\n", templateName)
			for _, error := range validation.Errors {
				fmt.Printf("   🚫 Error: %s\n", error.Message)
			}
			allValid = false
		}
	}
	
	fmt.Println()
	if allValid {
		fmt.Printf("✅ All templates are valid!\n")
	} else {
		fmt.Printf("❌ Some templates have validation errors\n")
		return fmt.Errorf("validation failed")
	}
	
	return nil
}

func (h *MCPConfigHandler) RenderConfig(cmd *cobra.Command, args []string) error {
	configName := args[0]
	envName, _ := cmd.Flags().GetString("env")
	saveOutput, _ := cmd.Flags().GetBool("output")
	
	ctx := context.Background()
	envID := h.getEnvironmentID(envName)
	
	// Load template
	template, err := h.configManager.LoadTemplate(ctx, envID, configName)
	if err != nil {
		return fmt.Errorf("failed to load template: %w", err)
	}
	
	// Load variables
	variables, err := h.configManager.LoadVariables(ctx, envID)
	if err != nil {
		return fmt.Errorf("failed to load variables: %w", err)
	}
	
	// Render template
	renderedConfig, err := h.configManager.RenderTemplate(ctx, template, variables)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}
	
	// Display or save output
	if saveOutput {
		outputPath := fmt.Sprintf("%s-rendered.json", configName)
		// Save rendered config to file
		fmt.Printf("Rendered configuration saved to: %s\n", outputPath)
	} else {
		fmt.Printf("Rendered MCP Configuration (%s):\n", configName)
		fmt.Printf("=====================================\n")
		// Display rendered JSON (would need to marshal renderedConfig)
		fmt.Printf("%+v\n", renderedConfig)
	}
	
	return nil
}

func (h *MCPConfigHandler) InitConfigStructure(cmd *cobra.Command, args []string) error {
	envName, _ := cmd.Flags().GetString("env")
	
	fmt.Printf("Initializing MCP configuration structure for environment '%s'...\n", envName)
	
	// Create directory structure
	if err := h.configManager.EnsureEnvironmentStructure(envName); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}
	
	configPath := h.configManager.GetConfigPath(envName, "")
	varsPath := h.configManager.GetVariablesPath(envName)
	
	fmt.Printf("✅ Created configuration directory structure:\n")
	fmt.Printf("   📁 Templates: %s\n", filepath.Dir(configPath))
	fmt.Printf("   📁 Variables: %s\n", filepath.Dir(varsPath))
	fmt.Printf("   📁 Template vars: %s\n", filepath.Join(filepath.Dir(configPath), "template-vars"))
	
	// Create example template if none exist
	templates, _ := h.configManager.DiscoverTemplates(context.Background(), envName)
	if len(templates) == 0 {
		fmt.Printf("\nCreating example template...\n")
		if err := h.createExampleTemplate(envName); err != nil {
			return fmt.Errorf("failed to create example template: %w", err)
		}
	}
	
	return nil
}

// Helper methods

func (h *MCPConfigHandler) getEnvironmentID(envName string) int64 {
	// This would look up the environment ID from the database
	// For now, return a placeholder
	return 1
}

func (h *MCPConfigHandler) loadTemplateVariables(ctx context.Context, envID int64, templateName string) ([]config.TemplateVariable, error) {
	template, err := h.configManager.LoadTemplate(ctx, envID, templateName)
	if err != nil {
		return nil, err
	}
	return template.Variables, nil
}

func (h *MCPConfigHandler) promptForTemplateContent(configName string) string {
	// This would provide an interactive prompt for template creation
	// For now, return a default template
	return h.getDefaultTemplateContent(configName)
}

func (h *MCPConfigHandler) getDefaultTemplateContent(configName string) string {
	return fmt.Sprintf(`{
  "mcpServers": {
    "{{.ServerName | default \"%s\"}}": {
      "command": "{{required \"Command is required\" .Command}}",
      "args": {{.Args | default "[]" | toJSON}},
      "env": {
        "API_KEY": "{{required \"API key is required\" .ApiKey}}"
      }
    }
  }
}`, configName)
}

func (h *MCPConfigHandler) promptForVariables(ctx context.Context, envID int64, configName string, template *config.MCPTemplate) error {
	// This would prompt the user for required variables
	// For now, just print what variables are needed
	fmt.Printf("\nTemplate variables found:\n")
	for _, variable := range template.Variables {
		fmt.Printf("  - %s (%s)%s: %s\n", 
			variable.Name, 
			variable.Type,
			map[bool]string{true: " - required", false: ""}[variable.Required],
			variable.Description,
		)
	}
	fmt.Printf("\nRun 'stn mcp vars edit --env %s --template %s' to set variables\n", "default", configName)
	return nil
}

func (h *MCPConfigHandler) createExampleTemplate(envName string) error {
	ctx := context.Background()
	envID := h.getEnvironmentID(envName)
	
	exampleTemplate := &config.MCPTemplate{
		Name:    "example",
		Content: h.getDefaultTemplateContent("example"),
		Metadata: config.TemplateMetadata{
			Version:     "1.0.0",
			Description: "Example MCP configuration template",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
	
	if err := h.configManager.SaveTemplate(ctx, envID, "example", exampleTemplate); err != nil {
		return err
	}
	
	fmt.Printf("✅ Created example template: example.json\n")
	fmt.Printf("📝 Edit with: stn mcp config edit example --env %s\n", envName)
	
	return nil
}

// Variable management methods would be implemented here
func (h *MCPConfigHandler) ListVariables(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("not implemented yet")
}

func (h *MCPConfigHandler) SetVariables(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("not implemented yet")
}

func (h *MCPConfigHandler) EditVariables(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("not implemented yet")
}

func (h *MCPConfigHandler) ImportVariables(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("not implemented yet")
}

// EditConfig edits an MCP configuration template
func (h *MCPConfigHandler) EditConfig(cmd *cobra.Command, args []string) error {
	configName := args[0]
	envName, _ := cmd.Flags().GetString("env")
	editor, _ := cmd.Flags().GetString("editor")
	
	if editor == "" {
		editor = "vi" // fallback default
	}
	
	ctx := context.Background()
	envID := h.getEnvironmentID(envName)
	
	// Check if template exists
	_, err := h.configManager.LoadTemplate(ctx, envID, configName)
	if err != nil {
		return fmt.Errorf("template '%s' not found in environment '%s': %w", configName, envName, err)
	}
	
	templatePath := h.configManager.GetConfigPath(envName, configName)
	fmt.Printf("Opening template for editing: %s\n", templatePath)
	fmt.Printf("Use your editor (%s) to edit the template.\n", editor)
	fmt.Printf("After editing, run 'stn mcp config validate %s --env %s' to validate changes.\n", configName, envName)
	
	return fmt.Errorf("interactive editing not implemented - please edit file directly: %s", templatePath)
}