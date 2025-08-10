package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"station/cmd/main/handlers"
	"station/cmd/main/handlers/mcp"
	"station/internal/db"
	"station/internal/tui"
	"station/pkg/bundle"
	bundlecli "station/pkg/bundle/cli"
)

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
	// Determine if bundleRef is a local file or registry reference
	var bundlePath string
	if strings.HasSuffix(bundleRef, ".tar.gz") && fileExists(bundleRef) {
		bundlePath = bundleRef
	} else {
		// TODO: Handle registry-based bundles in the future
		return fmt.Errorf("registry-based bundles not yet supported. Please provide a local .tar.gz file")
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