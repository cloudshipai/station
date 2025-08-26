package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"text/template/parse"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"station/cmd/main/handlers"
	"station/cmd/main/handlers/load"
	"station/cmd/main/handlers/mcp"
	"station/internal/db"
	"station/internal/tui"
	"station/pkg/bundle"
	bundlecli "station/pkg/bundle/cli"
)

// DotPromptConfig represents the YAML frontmatter in a .prompt file
type DotPromptConfig struct {
	Model       string                 `yaml:"model"`
	Config      map[string]interface{} `yaml:"config"`
	Metadata    map[string]interface{} `yaml:"metadata"`
	Tools       []string               `yaml:"tools"`
	Station     map[string]interface{} `yaml:"station"`
	Input       map[string]interface{} `yaml:"input"`
	Output      map[string]interface{} `yaml:"output"`
}

// parseDotPrompt parses a .prompt file with YAML frontmatter and prompt content
func parseDotPrompt(content string) (*DotPromptConfig, string, error) {
	// Split on the first occurrence of "---" after the initial "---"
	parts := strings.Split(content, "---")
	if len(parts) < 3 {
		// No frontmatter, treat entire content as prompt
		return &DotPromptConfig{}, content, nil
	}
	
	// The YAML frontmatter is parts[1], the prompt content starts from parts[2]
	yamlContent := strings.TrimSpace(parts[1])
	promptContent := strings.TrimSpace(strings.Join(parts[2:], "---"))
	
	var config DotPromptConfig
	if yamlContent != "" {
		err := yaml.Unmarshal([]byte(yamlContent), &config)
		if err != nil {
			return nil, "", fmt.Errorf("failed to parse YAML frontmatter: %w", err)
		}
	}
	
	return &config, promptContent, nil
}

// loadAgentPrompts loads all .prompt files from the specified agents directory
func loadAgentPrompts(ctx context.Context, genkitApp *genkit.Genkit, agentsDir, environment string) (int, error) {
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		return 0, fmt.Errorf("agents directory does not exist: %s", agentsDir)
	}
	
	promptCount := 0
	err := filepath.Walk(agentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Only process .prompt files
		if !strings.HasSuffix(info.Name(), ".prompt") {
			return nil
		}
		
		// Get the agent name from filename (without .prompt extension)
		agentName := strings.TrimSuffix(info.Name(), ".prompt")
		
		// Read the prompt file
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("   ⚠️  Warning: failed to read prompt file %s: %v\n", path, err)
			return nil // Continue with other files
		}
		
		// Parse the dotprompt format (YAML frontmatter + prompt content)
		promptConfig, promptContent, err := parseDotPrompt(string(content))
		if err != nil {
			fmt.Printf("   ⚠️  Warning: failed to parse prompt file %s: %v\n", path, err)
			return nil // Continue with other files
		}
		
		// Log what we parsed for debugging
		fmt.Printf("   📝 Parsed prompt: %s (model: %s, %d tools)\n", agentName, promptConfig.Model, len(promptConfig.Tools))
		
		// Build GenKit prompt options with parsed content
		promptOptions := []ai.PromptOption{
			ai.WithPrompt(promptContent),
		}
		
		// Log the model from frontmatter for debugging
		if promptConfig.Model != "" {
			fmt.Printf("   🎯 Prompt specifies model: %s\n", promptConfig.Model)
		}
		
		// Add input schema matching the prompt template variables
		inputType := struct {
			TASK        string `json:"TASK" jsonschema:"description=The specific task or instruction for the agent to perform"`
			ENVIRONMENT string `json:"ENVIRONMENT" jsonschema:"description=The environment context (e.g., dev, staging, production)"`
		}{
			TASK:        "Please analyze and explore the file structure within the allowed directories",
			ENVIRONMENT: environment,
		}
		
		if promptConfig.Input != nil && len(promptConfig.Input) > 0 {
			// TODO: Parse the frontmatter input schema properly
			// For now, use the default template variable schema
			promptOptions = append(promptOptions, ai.WithInputType(inputType))
		} else {
			// Default input matches Station prompt template variables ({{TASK}}, {{ENVIRONMENT}})
			promptOptions = append(promptOptions, ai.WithInputType(inputType))
		}
		
		// Add output schema if defined in frontmatter
		if promptConfig.Output != nil && len(promptConfig.Output) > 0 {
			// Create output type - for most Station agents, this should be a string response
			promptOptions = append(promptOptions, ai.WithOutputType(""))
		} else {
			// Default to string output to avoid schema errors
			promptOptions = append(promptOptions, ai.WithOutputType(""))
		}
		
		// Define the prompt in GenKit with proper configuration
		_, err = genkit.DefinePrompt(genkitApp, agentName, promptOptions...)
		if err != nil {
			fmt.Printf("   ⚠️  Warning: failed to define prompt %s: %v\n", agentName, err)
			return nil // Continue with other files
		}
		
		fmt.Printf("   ✅ Agent Prompt: %s\n", agentName)
		promptCount++
		return nil
	})
	
	return promptCount, err
}


// runMCPList implements the "station mcp list" command
func runMCPList(cmd *cobra.Command, args []string) error {
	mcpHandler := mcp.NewMCPHandler(themeManager)
	return mcpHandler.RunMCPList(cmd, args)
}

// runMCPTools implements the "station mcp tools" command
func runMCPTools(cmd *cobra.Command, args []string) error {
	mcpHandler := mcp.NewMCPHandler(themeManager)
	return mcpHandler.RunMCPTools(cmd, args)
}

// runMCPDelete implements the "station mcp delete" command
func runMCPDelete(cmd *cobra.Command, args []string) error {
	mcpHandler := mcp.NewMCPHandler(themeManager)
	return mcpHandler.RunMCPDelete(cmd, args)
}

// runUI implements the "station ui" command
func runUI(cmd *cobra.Command, args []string) error {
	// Disable all logging for clean TUI experience
	log.SetOutput(io.Discard)
	
	// Check if configuration exists
	databasePath := viper.GetString("database_url")
	if databasePath == "" {
		return fmt.Errorf("database path not configured. Please run 'stn init' first")
	}

	// Initialize database
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	// For the UI command, we'll create a minimal setup
	// The TUI can work with nil services for basic functionality
	
	// Create TUI model with minimal services (nil is acceptable for basic UI)
	tuiModel := tui.NewModel(database, nil)
	
	// Launch the TUI with same options as SSH
	program := tea.NewProgram(tuiModel, 
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	
	_, err = program.Run()
	return err
}

// runMCPAdd implements the "station mcp add" command
func runMCPAdd(cmd *cobra.Command, args []string) error {
	environment, _ := cmd.Flags().GetString("environment")
	endpoint, _ := cmd.Flags().GetString("endpoint")
	
	// Get config name from args or generate default
	var configName string
	if len(args) > 0 {
		configName = args[0]
	} else {
		configName = "new-mcp-config"
	}
	
	// Use the load handler's editor functionality
	loadHandler := load.NewLoadHandler(themeManager)
	return loadHandler.HandleMCPEditor(endpoint, environment, configName)
}

// runMCPAddFlags handles flag-based mode
func runMCPAddFlags(cmd *cobra.Command, args []string) error {
	// Get flags
	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	configID, _ := cmd.Flags().GetString("config-id")
	serverName, _ := cmd.Flags().GetString("server-name")
	command, _ := cmd.Flags().GetString("command")
	argsSlice, _ := cmd.Flags().GetStringSlice("args")
	envVars, _ := cmd.Flags().GetStringToString("env")

	// Validate required flags
	if configID == "" {
		return fmt.Errorf("--config-id is required")
	}
	if serverName == "" {
		return fmt.Errorf("--server-name is required")
	}
	if command == "" {
		return fmt.Errorf("--command is required")
	}

	// Show banner
	styles := getCLIStyles(themeManager)
	banner := styles.Banner.Render("🔧 Add MCP Server to Configuration")
	fmt.Println(banner)

	// Create spinner model with server configuration
	model := handlers.NewSpinnerModelWithServerConfig(
		fmt.Sprintf("Adding server '%s' to configuration '%s'...", serverName, configID),
		configID, serverName, command, argsSlice, envVars, environment, endpoint, themeManager)

	// Start the spinner
	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("failed to run spinner: %w", err)
	}

	// Check results
	final := finalModel.(handlers.SpinnerModel)
	if final.GetError() != nil {
		fmt.Println(getCLIStyles(themeManager).Error.Render("❌ Failed to add server: " + final.GetError().Error()))
		return final.GetError()
	}

	// Show success banner
	showSuccessBanner(fmt.Sprintf("Server '%s' successfully added to configuration!", serverName), themeManager)
	fmt.Printf("Result: %s\n", final.GetResult())

	return nil
}

// runMCPAddInteractive handles interactive mode with beautiful forms
func runMCPAddInteractive(cmd *cobra.Command, args []string) error {
	// Show retro banner
	retroBanner := getCLIStyles(themeManager).Banner.Render("🎛️  Interactive MCP Server Configuration")
	fmt.Println(retroBanner)
	fmt.Println(getCLIStyles(themeManager).Info.Render("Use arrow keys to navigate, Enter to select, Ctrl+C to exit"))
	fmt.Println()

	// Get basic flags that might be pre-set
	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	
	// Create the interactive form model
	formModel := handlers.NewMCPAddForm(endpoint, environment, themeManager)
	
	// Run the interactive form
	program := tea.NewProgram(formModel, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("failed to run interactive form: %w", err)
	}
	
	// Check if user cancelled
	final := finalModel.(*handlers.MCPAddFormModel)
	if final.IsCancelled() {
		fmt.Println(getCLIStyles(themeManager).Info.Render("Operation cancelled"))
		return nil
	}
	
	// Show completion banner with collected data
	showSuccessBanner("MCP Server Configuration Complete!", themeManager)
	fmt.Printf("Adding server: %s\n", getCLIStyles(themeManager).Success.Render(final.GetServerName()))
	fmt.Printf("To config: %s\n", getCLIStyles(themeManager).Success.Render(final.GetConfigID()))
	fmt.Printf("Command: %s %v\n", getCLIStyles(themeManager).Success.Render(final.GetCommand()), final.GetArgs())
	
	// Now execute the actual addition
	mcpHandler := mcp.NewMCPHandler(themeManager)
	result, err := mcpHandler.AddServerToConfig(final.GetConfigID(), final.GetServerName(), final.GetCommand(), final.GetArgs(), final.GetEnvVars(), final.GetEnvironment(), final.GetEndpoint())
	if err != nil {
		fmt.Println(getCLIStyles(themeManager).Error.Render("❌ Failed to add server: " + err.Error()))
		return err
	}
	
	fmt.Printf("Result: %s\n", result)
	return nil
}

// runMCPSync implements the "station mcp sync" command
func runMCPSync(cmd *cobra.Command, args []string) error {
	mcpHandler := mcp.NewMCPHandler(themeManager)
	return mcpHandler.RunMCPSync(cmd, args)
}

// runMCPStatus implements the "station mcp status" command
func runMCPStatus(cmd *cobra.Command, args []string) error {
	mcpHandler := mcp.NewMCPHandler(themeManager)
	return mcpHandler.RunMCPStatus(cmd, args)
}

// runTemplateCreate implements the "station template create" command
func runTemplateCreate(cmd *cobra.Command, args []string) error {
	// Get flags
	name, _ := cmd.Flags().GetString("name")
	author, _ := cmd.Flags().GetString("author")
	description, _ := cmd.Flags().GetString("description")
	envName, _ := cmd.Flags().GetString("env")
	
	// Use bundle path from args
	bundlePath := args[0]
	
	// If name not provided, use directory name
	if name == "" {
		name = filepath.Base(bundlePath)
	}
	
	// Show banner
	styles := getCLIStyles(themeManager)
	
	if envName != "" {
		// Enhanced mode: Create bundle from existing environment
		banner := styles.Banner.Render("📦 Create Template Bundle from Environment")
		fmt.Println(banner)
		fmt.Printf("🌍 Scanning environment: %s\n", envName)
		
		return createBundleFromEnvironment(bundlePath, envName, name, author, description)
	} else {
		// Original mode: Create empty bundle template
		banner := styles.Banner.Render("📦 Create Template Bundle")
		fmt.Println(banner)
		
		// Create bundle CLI
		bundleCLI := bundlecli.NewBundleCLI(nil)
		opts := bundle.CreateOptions{
			Name:        name,
			Author:      author,
			Description: description,
		}
		
		return bundleCLI.CreateBundle(bundlePath, opts)
	}
}

// createBundleFromEnvironment creates a bundle by scanning an existing environment
func createBundleFromEnvironment(bundlePath, envName, name, author, description string) error {
	// Get workspace path and environment directory
	workspacePath := getWorkspacePath()
	envDir := filepath.Join(workspacePath, "environments", envName)
	
	// Check if environment exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return fmt.Errorf("environment '%s' does not exist at %s", envName, envDir)
	}
	
	// Create bundle directory
	if err := os.MkdirAll(bundlePath, 0755); err != nil {
		return fmt.Errorf("failed to create bundle directory: %w", err)
	}
	
	fmt.Printf("📂 Scanning environment directory: %s\n", envDir)
	
	// Scan for MCP configurations
	mcpConfigs, err := scanMCPConfigs(envDir)
	if err != nil {
		return fmt.Errorf("failed to scan MCP configs: %w", err)
	}
	fmt.Printf("   📡 Found %d MCP configuration(s)\n", len(mcpConfigs))
	
	// Scan for agent prompts
	agents, err := scanAgentPrompts(envDir)
	if err != nil {
		return fmt.Errorf("failed to scan agent prompts: %w", err)
	}
	fmt.Printf("   🤖 Found %d agent prompt(s)\n", len(agents))
	
	// Scan for template variables
	variables, err := scanTemplateVariables(envDir, mcpConfigs, agents)
	if err != nil {
		return fmt.Errorf("failed to scan template variables: %w", err)
	}
	fmt.Printf("   📝 Found %d template variable(s)\n", len(variables))
	
	// Merge MCP configurations into single template
	mergedMCPConfig, err := mergeMCPConfigs(mcpConfigs)
	if err != nil {
		return fmt.Errorf("failed to merge MCP configs: %w", err)
	}
	
	// Create bundle structure
	if err := createEnhancedBundleStructure(bundlePath, name, author, description, envName, mergedMCPConfig, agents, variables); err != nil {
		return fmt.Errorf("failed to create bundle structure: %w", err)
	}
	
	fmt.Printf("✅ Bundle created successfully from environment '%s'\n", envName)
	fmt.Printf("📁 Bundle path: %s\n", bundlePath)
	fmt.Printf("📝 Next steps:\n")
	fmt.Printf("   1. Review the generated template.json and adjust as needed\n")
	fmt.Printf("   2. Update manifest.json with additional metadata\n")
	fmt.Printf("   3. Run 'stn template validate %s' to test your bundle\n", bundlePath)
	fmt.Printf("   4. Run 'stn template bundle %s' to package for distribution\n", bundlePath)
	
	return nil
}

// runTemplateValidate implements the "station template validate" command
func runTemplateValidate(cmd *cobra.Command, args []string) error {
	bundlePath := args[0]
	
	// Show banner
	styles := getCLIStyles(themeManager)
	banner := styles.Banner.Render("🔍 Validate Template Bundle")
	fmt.Println(banner)
	
	// Create bundle CLI and validate
	bundleCLI := bundlecli.NewBundleCLI(nil)
	summary, err := bundleCLI.ValidateBundle(bundlePath)
	if err != nil {
		return err
	}
	
	// Print validation results
	bundleCLI.PrintValidationSummary(summary)
	return nil
}

// runTemplateBundle implements the "station template bundle" command
func runTemplateBundle(cmd *cobra.Command, args []string) error {
	bundlePath := args[0]
	outputPath, _ := cmd.Flags().GetString("output")
	validateFirst, _ := cmd.Flags().GetBool("validate")
	
	// Show banner
	styles := getCLIStyles(themeManager)
	banner := styles.Banner.Render("📦 Package Template Bundle")
	fmt.Println(banner)
	
	// Create bundle CLI and package
	bundleCLI := bundlecli.NewBundleCLI(nil)
	summary, err := bundleCLI.PackageBundle(bundlePath, outputPath, validateFirst)
	if err != nil {
		return err
	}
	
	// Print packaging results
	bundleCLI.PrintPackageSummary(summary)
	return nil
}

// runTemplatePublish implements the "station template publish" command
func runTemplatePublish(cmd *cobra.Command, args []string) error {
	bundlePath := args[0]
	registry, _ := cmd.Flags().GetString("registry")
	skipValidation, _ := cmd.Flags().GetBool("skip-validation")
	
	// Show banner
	styles := getCLIStyles(themeManager)
	banner := styles.Banner.Render("📤 Publish Template Bundle")
	fmt.Println(banner)
	
	// TODO: Implement publishing logic
	fmt.Printf("Publishing %s to registry '%s'...\n", bundlePath, registry)
	if skipValidation {
		fmt.Println("⚠️  Skipping validation")
	}
	
	// For now, just package the bundle
	bundleCLI := bundlecli.NewBundleCLI(nil)
	summary, err := bundleCLI.PackageBundle(bundlePath, "", !skipValidation)
	if err != nil {
		return err
	}
	
	if !summary.Success {
		return fmt.Errorf("bundle packaging failed")
	}
	
	fmt.Printf("✅ Bundle packaged successfully: %s\n", summary.OutputPath)
	fmt.Printf("🚀 Publishing to registry '%s' (feature coming soon)\n", registry)
	
	return nil
}

// runTemplateInstall implements the "station template install" command  
func runTemplateInstall(cmd *cobra.Command, args []string) error {
	bundleRef := args[0]
	environmentName := "default" // Default environment
	if len(args) > 1 {
		environmentName = args[1]
	}
	
	registry, _ := cmd.Flags().GetString("registry")
	force, _ := cmd.Flags().GetBool("force")
	
	// Show banner
	styles := getCLIStyles(themeManager)
	banner := styles.Banner.Render("📥 Install Template Bundle")
	fmt.Println(banner)
	
	fmt.Printf("🎯 Installing '%s' into environment '%s'\n", bundleRef, environmentName)
	if registry != "" {
		fmt.Printf("📡 Registry: %s\n", registry)
	}
	if force {
		fmt.Printf("⚠️  Force reinstall mode enabled\n")
	}
	fmt.Println()
	
	// Call our installation logic
	if err := installTemplateBundle(bundleRef, environmentName, force); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}
	
	fmt.Printf("✅ Bundle '%s' installed successfully!\n", bundleRef)
	fmt.Printf("📋 Next steps:\n")
	fmt.Printf("   1. Run 'stn sync %s' to load MCP configs and agents\n", environmentName)
	fmt.Printf("   2. If prompted for variables, update ~/.config/station/environments/%s/variables.yml\n", environmentName)
	
	return nil
}

// installTemplateBundle installs a template bundle into the specified environment
func installTemplateBundle(bundleRef, environmentName string, force bool) error {
	// Determine if bundleRef is a local file or remote URL
	var bundlePath string
	if strings.HasPrefix(bundleRef, "http://") || strings.HasPrefix(bundleRef, "https://") {
		// Download remote bundle
		fmt.Printf("⬇️  Downloading bundle from remote URL...\n")
		tempFile, err := downloadBundle(bundleRef)
		if err != nil {
			return fmt.Errorf("failed to download bundle: %w", err)
		}
		defer os.Remove(tempFile)
		bundlePath = tempFile
	} else if strings.HasSuffix(bundleRef, ".tar.gz") && fileExists(bundleRef) {
		bundlePath = bundleRef
	} else {
		// TODO: Handle registry-based bundles by name in the future
		return fmt.Errorf("bundle not found. Please provide a local .tar.gz file or remote URL")
	}
	
	// Get Station config directory
	configDir := os.ExpandEnv("$HOME/.config/station")
	envDir := filepath.Join(configDir, "environments", environmentName)
	
	// Create environment directory if it doesn't exist
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("failed to create environment directory: %w", err)
	}
	
	// Extract bundle to temporary directory
	tempDir, err := os.MkdirTemp("", "bundle-install-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	
	fmt.Printf("📦 Extracting bundle...\n")
	if err := extractTarGz(bundlePath, tempDir); err != nil {
		return fmt.Errorf("failed to extract bundle: %w", err)
	}
	
	// Install MCP configuration
	fmt.Printf("⚙️  Installing MCP configuration...\n")
	templatePath := filepath.Join(tempDir, "template.json")
	if fileExists(templatePath) {
		manifestPath := filepath.Join(tempDir, "manifest.json")
		configName := "template"
		if fileExists(manifestPath) {
			// Try to get a better config name from manifest
			if manifest, err := loadManifestFile(manifestPath); err == nil && manifest.Name != "" {
				// Sanitize name for filename
				configName = strings.ToLower(strings.ReplaceAll(manifest.Name, " ", "-"))
				configName = strings.ReplaceAll(configName, "_", "-")
			}
		}
		
		destConfigPath := filepath.Join(envDir, configName+".json")
		if err := copyFile(templatePath, destConfigPath); err != nil {
			return fmt.Errorf("failed to install MCP config: %w", err)
		}
		fmt.Printf("   ✅ Installed MCP config: %s.json\n", configName)
	}
	
	// Install agents
	agentsDir := filepath.Join(tempDir, "agents")
	if dirExists(agentsDir) {
		fmt.Printf("🤖 Installing agents...\n")
		
		destAgentsDir := filepath.Join(envDir, "agents")
		if err := os.MkdirAll(destAgentsDir, 0755); err != nil {
			return fmt.Errorf("failed to create agents directory: %w", err)
		}
		
		// Copy all .prompt files
		entries, err := os.ReadDir(agentsDir)
		if err != nil {
			return fmt.Errorf("failed to read agents directory: %w", err)
		}
		
		agentCount := 0
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".prompt") {
				continue
			}
			
			srcPath := filepath.Join(agentsDir, entry.Name())
			destPath := filepath.Join(destAgentsDir, entry.Name())
			
			// Don't overwrite existing agents unless force is enabled
			if !force && fileExists(destPath) {
				fmt.Printf("   ⏭️  Skipping existing agent: %s (use --force to overwrite)\n", entry.Name())
				continue
			}
			
			if err := copyFile(srcPath, destPath); err != nil {
				return fmt.Errorf("failed to install agent %s: %w", entry.Name(), err)
			}
			fmt.Printf("   ✅ Installed agent: %s\n", entry.Name())
			agentCount++
		}
		
		if agentCount == 0 {
			fmt.Printf("   ℹ️  No new agents installed\n")
		}
	}
	
	// Install example variables (only if variables.yml doesn't exist)
	variablesPath := filepath.Join(envDir, "variables.yml")
	if !fileExists(variablesPath) {
		exampleVarsPath := filepath.Join(tempDir, "examples", "development.vars.yml")
		if fileExists(exampleVarsPath) {
			fmt.Printf("📝 Installing example variables...\n")
			if err := copyFile(exampleVarsPath, variablesPath); err != nil {
				return fmt.Errorf("failed to install example variables: %w", err)
			}
			fmt.Printf("   ✅ Created variables.yml from development example\n")
		}
	} else {
		fmt.Printf("📝 Preserving existing variables.yml\n")
	}
	
	return nil
}

// Helper functions
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()
	
	_, err = io.Copy(destFile, sourceFile)
	return err
}

func extractTarGz(src, dst string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()
	
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()
	
	tr := tar.NewReader(gzr)
	
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		
		target := filepath.Join(dst, header.Name)
		
		// Security: ensure target is within dst directory
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) {
			continue
		}
		
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			
			file, err := os.Create(target)
			if err != nil {
				return err
			}
			
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return err
			}
			file.Close()
		}
	}
	
	return nil
}

func loadManifestFile(path string) (*bundle.BundleManifest, error) {
	// Simple implementation - just try to extract name from JSON
	// For now, return error to use fallback
	return nil, fmt.Errorf("manifest parsing not implemented")
}

// runTemplateList implements the "station template list" command
func runTemplateList(cmd *cobra.Command, args []string) error {
	registry, _ := cmd.Flags().GetString("registry")
	search, _ := cmd.Flags().GetString("search")
	
	// Show banner
	styles := getCLIStyles(themeManager)
	banner := styles.Banner.Render("📋 Available Template Bundles")
	fmt.Println(banner)
	
	if registry != "" {
		fmt.Printf("Registry: %s\n", registry)
	}
	if search != "" {
		fmt.Printf("Search: %s\n", search)
	}
	
	// TODO: Implement registry listing
	fmt.Printf("🚀 Registry discovery (feature coming soon)\n")
	
	return nil
}

// runTemplateRegistryAdd implements the "station template registry add" command
func runTemplateRegistryAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	url := args[1]
	
	// Show banner
	styles := getCLIStyles(themeManager)
	banner := styles.Banner.Render("➕ Add Template Registry")
	fmt.Println(banner)
	
	fmt.Printf("Adding registry '%s' at %s\n", name, url)
	
	// TODO: Implement registry configuration
	fmt.Printf("🚀 Registry management (feature coming soon)\n")
	
	return nil
}

// runTemplateRegistryList implements the "station template registry list" command
func runTemplateRegistryList(cmd *cobra.Command, args []string) error {
	// Show banner
	styles := getCLIStyles(themeManager)
	banner := styles.Banner.Render("📋 Configured Registries")
	fmt.Println(banner)
	
	// TODO: Implement registry listing
	fmt.Printf("🚀 Registry management (feature coming soon)\n")
	
	return nil
}

// downloadBundle downloads a bundle from a remote URL and returns the path to the temp file
func downloadBundle(url string) (string, error) {
	// Create temporary file
	tempFile, err := os.CreateTemp("", "bundle-download-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Try downloading without authentication first
	resp, err := downloadWithAuth(url, "")
	if err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to download from %s: %w", url, err)
	}
	defer resp.Body.Close()

	// If we get a 404 and this is a GitHub URL, try with authentication
	if resp.StatusCode == http.StatusNotFound && strings.Contains(url, "github.com") {
		resp.Body.Close()
		
		// Look for GitHub token
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			token = os.Getenv("GH_TOKEN") // Alternative env var used by gh CLI
		}
		
		if token != "" {
			fmt.Printf("   🔐 Trying with GitHub authentication for private repo...\n")
			resp, err = downloadWithAuth(url, token)
			if err != nil {
				os.Remove(tempFile.Name())
				return "", fmt.Errorf("failed to download from %s with authentication: %w", url, err)
			}
			defer resp.Body.Close()
		}
	}

	if resp.StatusCode != http.StatusOK {
		os.Remove(tempFile.Name())
		if resp.StatusCode == http.StatusNotFound && strings.Contains(url, "github.com") {
			return "", fmt.Errorf("download failed with status %d: %s (hint: for private repos, set GITHUB_TOKEN environment variable)", resp.StatusCode, resp.Status)
		}
		return "", fmt.Errorf("download failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	// Copy response body to temp file
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write bundle to temp file: %w", err)
	}

	return tempFile.Name(), nil
}

// downloadWithAuth downloads from URL with optional GitHub token authentication
func downloadWithAuth(url, token string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	// Add GitHub token if provided
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	
	return client.Do(req)
}

// Environment scanning helper functions for enhanced bundle creation

// MCPConfigInfo holds information about an MCP configuration file
type MCPConfigInfo struct {
	Name     string
	FilePath string
	Config   map[string]interface{}
}

// AgentPromptInfo holds information about an agent prompt file  
type AgentPromptInfo struct {
	Name       string
	FilePath   string
	Config     *DotPromptConfig
	PromptText string
}

// TemplateVariable represents a template variable found in configs or prompts
type TemplateVariable struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Default     interface{}
}

// scanMCPConfigs scans the environment directory for MCP configuration files
func scanMCPConfigs(envDir string) ([]*MCPConfigInfo, error) {
	var configs []*MCPConfigInfo
	
	// Walk through environment directory looking for .json files
	err := filepath.Walk(envDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip non-JSON files and directories
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}
		
		// Skip files in agents subdirectory
		if strings.Contains(path, filepath.Join(envDir, "agents")) {
			return nil
		}
		
		// Read and parse the JSON file
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("   ⚠️  Warning: failed to read %s: %v\n", path, err)
			return nil // Continue with other files
		}
		
		var config map[string]interface{}
		if err := json.Unmarshal(content, &config); err != nil {
			fmt.Printf("   ⚠️  Warning: failed to parse JSON in %s: %v\n", path, err)
			return nil // Continue with other files
		}
		
		// Check if this looks like an MCP config (has mcpServers field)
		if _, hasMCPServers := config["mcpServers"]; hasMCPServers {
			name := strings.TrimSuffix(info.Name(), ".json")
			configs = append(configs, &MCPConfigInfo{
				Name:     name,
				FilePath: path,
				Config:   config,
			})
			fmt.Printf("   ✅ MCP Config: %s\n", name)
		}
		
		return nil
	})
	
	return configs, err
}

// scanAgentPrompts scans the environment directory for agent prompt files
func scanAgentPrompts(envDir string) ([]*AgentPromptInfo, error) {
	var agents []*AgentPromptInfo
	
	agentsDir := filepath.Join(envDir, "agents")
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		return agents, nil // No agents directory is fine
	}
	
	err := filepath.Walk(agentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Only process .prompt files
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".prompt") {
			return nil
		}
		
		// Read and parse the prompt file
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("   ⚠️  Warning: failed to read %s: %v\n", path, err)
			return nil // Continue with other files
		}
		
		// Parse the dotprompt format
		config, promptText, err := parseDotPrompt(string(content))
		if err != nil {
			fmt.Printf("   ⚠️  Warning: failed to parse prompt file %s: %v\n", path, err)
			return nil // Continue with other files
		}
		
		name := strings.TrimSuffix(info.Name(), ".prompt")
		agents = append(agents, &AgentPromptInfo{
			Name:       name,
			FilePath:   path,
			Config:     config,
			PromptText: promptText,
		})
		fmt.Printf("   ✅ Agent Prompt: %s\n", name)
		
		return nil
	})
	
	return agents, err
}

// scanTemplateVariables scans MCP configs, agent prompts, and variables.yml for template variables
func scanTemplateVariables(envDir string, mcpConfigs []*MCPConfigInfo, agents []*AgentPromptInfo) ([]*TemplateVariable, error) {
	variableMap := make(map[string]*TemplateVariable)
	
	// Scan MCP configurations for template variables using proper Go template parsing
	for _, mcpConfig := range mcpConfigs {
		content, err := json.Marshal(mcpConfig.Config)
		if err != nil {
			continue
		}
		
		variables := extractTemplateVariables(string(content))
		for _, varName := range variables {
			if _, exists := variableMap[varName]; !exists {
				variableMap[varName] = &TemplateVariable{
					Name:        varName,
					Type:        "string",
					Description: fmt.Sprintf("Variable found in MCP config: %s", mcpConfig.Name),
					Required:    true,
				}
			}
		}
	}
	
	// Scan agent prompts for template variables using proper Go template parsing
	for _, agent := range agents {
		variables := extractTemplateVariables(agent.PromptText)
		for _, varName := range variables {
			if _, exists := variableMap[varName]; !exists {
				variableMap[varName] = &TemplateVariable{
					Name:        varName,
					Type:        "string",
					Description: fmt.Sprintf("Variable found in agent prompt: %s", agent.Name),
					Required:    true,
				}
			}
		}
	}
	
	// Read existing variables.yml file to get more context
	variablesFile := filepath.Join(envDir, "variables.yml")
	if content, err := os.ReadFile(variablesFile); err == nil {
		var existingVars map[string]interface{}
		if err := yaml.Unmarshal(content, &existingVars); err == nil {
			for varName, value := range existingVars {
				upperVarName := strings.ToUpper(varName)
				if variable, exists := variableMap[upperVarName]; exists {
					// Update with actual value and inferred type
					variable.Default = value
					switch value.(type) {
					case int, int64, float64:
						variable.Type = "number"
					case bool:
						variable.Type = "boolean"
					default:
						variable.Type = "string"
					}
				} else {
					// Add variable that exists in variables.yml but wasn't found in templates
					variableMap[upperVarName] = &TemplateVariable{
						Name:        upperVarName,
						Type:        "string",
						Description: "Variable from existing configuration",
						Required:    false,
						Default:     value,
					}
				}
			}
			fmt.Printf("   📄 Loaded existing variables from variables.yml\n")
		}
	}
	
	// Convert map to slice
	var variables []*TemplateVariable
	for _, variable := range variableMap {
		variables = append(variables, variable)
	}
	
	return variables, nil
}

// extractTemplateVariables uses Go's template parser to properly extract variables from templates
func extractTemplateVariables(content string) []string {
	var variables []string
	variableSet := make(map[string]bool)
	
	// Create a template and parse the content
	tmpl, err := template.New("scan").Parse(content)
	if err != nil {
		// If parsing fails, template might have syntax errors or no variables
		return variables
	}
	
	// Create a visitor that captures variable accesses
	visitor := &templateVariableVisitor{
		variables: variableSet,
	}
	
	// Walk the parsed template tree to find variable references
	if tmpl.Tree != nil && tmpl.Tree.Root != nil {
		visitor.visitNode(tmpl.Tree.Root)
	}
	
	// Convert set to slice
	for varName := range variableSet {
		variables = append(variables, varName)
	}
	
	return variables
}

// templateVariableVisitor walks a Go template parse tree to find variable references
type templateVariableVisitor struct {
	variables map[string]bool
}

// visitNode recursively visits template nodes to find variable references
func (v *templateVariableVisitor) visitNode(node parse.Node) {
	if node == nil {
		return
	}
	
	switch n := node.(type) {
	case *parse.ListNode:
		if n != nil {
			for _, child := range n.Nodes {
				v.visitNode(child)
			}
		}
	case *parse.ActionNode:
		if n != nil && n.Pipe != nil {
			v.visitPipe(n.Pipe)
		}
	case *parse.IfNode:
		if n != nil {
			v.visitPipe(n.Pipe)
			v.visitNode(n.List)
			v.visitNode(n.ElseList)
		}
	case *parse.RangeNode:
		if n != nil {
			v.visitPipe(n.Pipe)
			v.visitNode(n.List)
			v.visitNode(n.ElseList)
		}
	case *parse.WithNode:
		if n != nil {
			v.visitPipe(n.Pipe)
			v.visitNode(n.List)
			v.visitNode(n.ElseList)
		}
	case *parse.TextNode:
		// Text nodes don't contain variables
	case *parse.CommentNode:
		// Comment nodes don't contain variables
	default:
		// Handle other node types if needed
	}
}

// visitPipe examines a template pipe for variable references
func (v *templateVariableVisitor) visitPipe(pipe *parse.PipeNode) {
	if pipe == nil {
		return
	}
	
	for _, cmd := range pipe.Cmds {
		if cmd != nil {
			for _, arg := range cmd.Args {
				v.visitArg(arg)
			}
		}
	}
}

// visitArg examines a template argument for variable references
func (v *templateVariableVisitor) visitArg(arg parse.Node) {
	if arg == nil {
		return
	}
	
	switch a := arg.(type) {
	case *parse.FieldNode:
		// Field access like .ROOT_PATH
		if len(a.Ident) > 0 && a.Ident[0] != "" {
			v.variables[a.Ident[0]] = true
		}
	case *parse.VariableNode:
		// Variable reference like $var
		if len(a.Ident) > 0 && a.Ident[0] != "" {
			v.variables[a.Ident[0]] = true
		}
	case *parse.ChainNode:
		// Chained field access
		v.visitArg(a.Node)
	case *parse.PipeNode:
		// Nested pipe
		v.visitPipe(a)
	default:
		// Other argument types (strings, numbers, etc.) don't contain variables
	}
}

// mergeMCPConfigs merges multiple MCP configurations into a single template config
func mergeMCPConfigs(configs []*MCPConfigInfo) (map[string]interface{}, error) {
	if len(configs) == 0 {
		return map[string]interface{}{
			"name":        "empty-template",
			"description": "Template bundle created from environment with no MCP configs",
			"mcpServers":  map[string]interface{}{},
		}, nil
	}
	
	// Start with the first config as base
	baseConfig := configs[0]
	result := make(map[string]interface{})
	
	// Copy base config
	for k, v := range baseConfig.Config {
		result[k] = v
	}
	
	// Ensure we have the required fields
	if result["name"] == nil {
		result["name"] = "merged-template"
	}
	if result["description"] == nil {
		result["description"] = "Template bundle created from environment MCP configurations"
	}
	
	// Merge all mcpServers sections
	allServers := make(map[string]interface{})
	
	for _, config := range configs {
		if mcpServers, ok := config.Config["mcpServers"].(map[string]interface{}); ok {
			for serverName, serverConfig := range mcpServers {
				// Use config name as prefix to avoid conflicts
				mergedServerName := serverName
				if len(configs) > 1 {
					mergedServerName = fmt.Sprintf("%s_%s", config.Name, serverName)
				}
				allServers[mergedServerName] = serverConfig
			}
		}
	}
	
	result["mcpServers"] = allServers
	return result, nil
}

// createEnhancedBundleStructure creates the bundle directory structure with scanned content
func createEnhancedBundleStructure(bundlePath, name, author, description, envName string, mcpConfig map[string]interface{}, agents []*AgentPromptInfo, variables []*TemplateVariable) error {
	// Create main bundle files
	
	// 1. Create template.json with merged MCP configuration
	templatePath := filepath.Join(bundlePath, "template.json")
	templateContent, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template config: %w", err)
	}
	if err := os.WriteFile(templatePath, templateContent, 0644); err != nil {
		return fmt.Errorf("failed to write template.json: %w", err)
	}
	
	// 2. Create manifest.json with bundle metadata and variable schema
	manifest := map[string]interface{}{
		"name":            name,
		"version":         "1.0.0",
		"description":     description,
		"author":          author,
		"source_env":      envName,
		"created_at":      time.Now().UTC().Format(time.RFC3339),
		"station_version": "0.2.7", // Current Station version
		"variables":       createVariableSchema(variables),
		"agents":          createAgentManifest(agents),
		"mcp_servers":     len(mcpConfig["mcpServers"].(map[string]interface{})),
	}
	
	if description == "" {
		manifest["description"] = fmt.Sprintf("Template bundle created from environment '%s'", envName)
	}
	
	manifestPath := filepath.Join(bundlePath, "manifest.json")
	manifestContent, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, manifestContent, 0644); err != nil {
		return fmt.Errorf("failed to write manifest.json: %w", err)
	}
	
	// 3. Create agents directory and copy agent prompt files
	if len(agents) > 0 {
		agentsDir := filepath.Join(bundlePath, "agents")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			return fmt.Errorf("failed to create agents directory: %w", err)
		}
		
		for _, agent := range agents {
			// Reconstruct the original .prompt file format
			var yamlContent []byte
			if agent.Config != nil {
				yamlContent, err = yaml.Marshal(agent.Config)
				if err != nil {
					return fmt.Errorf("failed to marshal agent config for %s: %w", agent.Name, err)
				}
			}
			
			var fullContent string
			if len(yamlContent) > 0 && string(yamlContent) != "{}\n" {
				fullContent = fmt.Sprintf("---\n%s---\n%s", string(yamlContent), agent.PromptText)
			} else {
				fullContent = agent.PromptText
			}
			
			agentPath := filepath.Join(agentsDir, agent.Name+".prompt")
			if err := os.WriteFile(agentPath, []byte(fullContent), 0644); err != nil {
				return fmt.Errorf("failed to write agent file %s: %w", agentPath, err)
			}
		}
	}
	
	// 4. Create examples directory with variable examples
	if len(variables) > 0 {
		examplesDir := filepath.Join(bundlePath, "examples")
		if err := os.MkdirAll(examplesDir, 0755); err != nil {
			return fmt.Errorf("failed to create examples directory: %w", err)
		}
		
		// Create development.vars.yml with example values
		exampleVars := make(map[string]interface{})
		for _, variable := range variables {
			if variable.Default != nil {
				// Use lowercase for YAML keys (matches Station convention)
				yamlKey := strings.ToLower(variable.Name)
				exampleVars[yamlKey] = variable.Default
			} else {
				// Provide example based on type
				yamlKey := strings.ToLower(variable.Name)
				switch variable.Type {
				case "number":
					exampleVars[yamlKey] = 8585
				case "boolean":
					exampleVars[yamlKey] = true
				default:
					exampleVars[yamlKey] = fmt.Sprintf("your_%s_value", strings.ToLower(variable.Name))
				}
			}
		}
		
		exampleVarsPath := filepath.Join(examplesDir, "development.vars.yml")
		exampleVarsContent, err := yaml.Marshal(exampleVars)
		if err != nil {
			return fmt.Errorf("failed to marshal example variables: %w", err)
		}
		if err := os.WriteFile(exampleVarsPath, exampleVarsContent, 0644); err != nil {
			return fmt.Errorf("failed to write example variables: %w", err)
		}
	}
	
	// 5. Create variables.schema.json for validation
	schemaPath := filepath.Join(bundlePath, "variables.schema.json")
	schema := createJSONSchema(variables)
	schemaContent, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal variables schema: %w", err)
	}
	if err := os.WriteFile(schemaPath, schemaContent, 0644); err != nil {
		return fmt.Errorf("failed to write variables schema: %w", err)
	}
	
	return nil
}

// createVariableSchema creates a simplified variable schema for the manifest
func createVariableSchema(variables []*TemplateVariable) []map[string]interface{} {
	var schema []map[string]interface{}
	
	for _, variable := range variables {
		varSchema := map[string]interface{}{
			"name":        variable.Name,
			"type":        variable.Type,
			"required":    variable.Required,
			"description": variable.Description,
		}
		if variable.Default != nil {
			varSchema["default"] = variable.Default
		}
		schema = append(schema, varSchema)
	}
	
	return schema
}

// createAgentManifest creates agent metadata for the manifest
func createAgentManifest(agents []*AgentPromptInfo) []map[string]interface{} {
	var agentList []map[string]interface{}
	
	for _, agent := range agents {
		agentInfo := map[string]interface{}{
			"name": agent.Name,
			"file": agent.Name + ".prompt",
		}
		
		if agent.Config != nil {
			if agent.Config.Model != "" {
				agentInfo["model"] = agent.Config.Model
			}
			if len(agent.Config.Tools) > 0 {
				agentInfo["tools"] = agent.Config.Tools
			}
		}
		
		agentList = append(agentList, agentInfo)
	}
	
	return agentList
}

// createJSONSchema creates a JSON Schema for template variables
func createJSONSchema(variables []*TemplateVariable) map[string]interface{} {
	schema := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"title":   "Template Variables Schema",
		"description": "JSON Schema for template bundle variables",
		"properties": make(map[string]interface{}),
		"required": []string{},
	}
	
	properties := schema["properties"].(map[string]interface{})
	var required []string
	
	for _, variable := range variables {
		propSchema := map[string]interface{}{
			"type":        variable.Type,
			"description": variable.Description,
		}
		
		if variable.Default != nil {
			propSchema["default"] = variable.Default
		}
		
		// Use lowercase for JSON schema (matches YAML convention)
		propName := strings.ToLower(variable.Name)
		properties[propName] = propSchema
		
		if variable.Required {
			required = append(required, propName)
		}
	}
	
	if len(required) > 0 {
		schema["required"] = required
	}
	
	return schema
}