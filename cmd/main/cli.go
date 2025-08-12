package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"station/cmd/main/handlers"
	"station/cmd/main/handlers/mcp"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
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
	tuiModel := tui.NewModel(database, nil, nil)
	
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
	// Check if interactive mode is requested
	interactive, _ := cmd.Flags().GetBool("interactive")
	
	if interactive {
		return runMCPAddInteractive(cmd, args)
	}
	
	return runMCPAddFlags(cmd, args)
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
	
	// Use bundle path from args
	bundlePath := args[0]
	
	// If name not provided, use directory name
	if name == "" {
		name = filepath.Base(bundlePath)
	}
	
	// Show banner
	styles := getCLIStyles(themeManager)
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

// runDevelop implements the "stn develop" command
func runDevelop(cmd *cobra.Command, args []string) error {
	// Get command flags
	environment, _ := cmd.Flags().GetString("env")
	port, _ := cmd.Flags().GetInt("port")
	aiModel, _ := cmd.Flags().GetString("ai-model")
	aiProvider, _ := cmd.Flags().GetString("ai-provider")
	verbose, _ := cmd.Flags().GetBool("verbose")

	// Show banner
	styles := getCLIStyles(themeManager)
	banner := styles.Banner.Render("🧪 Station Development Playground")
	fmt.Println(banner)

	fmt.Printf("🌍 Environment: %s\n", environment)
	fmt.Printf("🚀 Starting development server on port %d...\n", port)
	fmt.Printf("🤖 AI Provider: %s, Model: %s\n", aiProvider, aiModel)
	fmt.Printf("🔧 Verbose: %v\n", verbose)
	
	ctx := context.Background()
	
	// Initialize database and services
	databasePath := viper.GetString("database_url")
	if databasePath == "" {
		configDir := getWorkspacePath()
		databasePath = filepath.Join(configDir, "station.db")
	}
	
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()
	
	repos := repositories.New(database)
	
	// Get environment ID
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found: %w", environment, err)
	}
	
	fmt.Printf("📁 Loading agents and MCP configs from environment: %s (ID: %d)\n", env.Name, env.ID)
	
	// Initialize Station's GenKit provider
	genkitProvider := services.NewGenKitProvider()
	genkitApp, err := genkitProvider.GetApp(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize GenKit: %w", err)
	}
	
	// Load MCP tools
	mcpManager := services.NewMCPConnectionManager(repos, genkitApp)
	mcpTools, mcpClients, err := mcpManager.GetEnvironmentMCPTools(ctx, env.ID)
	if err != nil {
		return fmt.Errorf("failed to load MCP tools: %w", err)
	}
	defer mcpManager.CleanupConnections(mcpClients)
	
	fmt.Printf("🔧 Loaded %d MCP tools from %d servers\n", len(mcpTools), len(mcpClients))
	
	// Load agent prompts from the environment
	workspacePath := getWorkspacePath()
	agentsDir := filepath.Join(workspacePath, "environments", environment, "agents")
	
	promptCount, err := loadAgentPrompts(ctx, genkitApp, agentsDir, environment)
	if err != nil {
		fmt.Printf("⚠️  Warning: failed to load agent prompts: %v\n", err)
	} else {
		fmt.Printf("🤖 Loaded %d agent prompts\n", promptCount)
	}
	
	// Define MCP tools in GenKit
	for _, tool := range mcpTools {
		// MCP tools are already registered in GenKit by the MCP plugin
		fmt.Printf("   ✅ MCP Tool: %s\n", tool.Name())
	}
	
	fmt.Println()
	fmt.Println("🎉 Station Development Playground is ready!")
	fmt.Printf("📖 To start the Genkit developer UI, run:\n")
	fmt.Printf("   genkit start -o -- stn develop --env %s --port %d\n", environment, port)
	fmt.Println()
	fmt.Println("🧪 This will start the interactive testing UI at http://localhost:4000")
	fmt.Println("🔧 All your agents and MCP tools will be available for testing")
	fmt.Println()
	fmt.Println("For now, Station development playground setup is complete.")
	fmt.Println("Your agents and tools are loaded in Genkit and ready to use.")
	
	// Keep the process alive to maintain MCP connections
	fmt.Println()
	fmt.Println("Press Ctrl+C to exit and cleanup MCP connections...")
	
	// Block indefinitely until interrupted
	select {}
	
	return nil
}