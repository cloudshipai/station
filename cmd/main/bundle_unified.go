package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"station/cmd/main/handlers/common"
	"station/internal/services"
)

// Unified bundle command with create and install subcommands
var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Bundle management commands",
	Long: `Create and install Station bundles.
	
Subcommands:
  create   Create a bundle from an environment
  install  Install a bundle from URL or file path`,
}

// Bundle create subcommand
var bundleCreateCmd = &cobra.Command{
	Use:   "create <environment>",
	Short: "Create a bundle from an environment",
	Long: `Create a deployable bundle (.tar.gz) from an environment.
This uses the same bundling logic as the API and creates bundles
that are compatible with the bundle API installation endpoints.

Examples:
  stn bundle create default              # Bundle the default environment
  stn bundle create production           # Bundle the production environment  
  stn bundle create default --output my-bundle.tar.gz  # Custom output path`,
	Args: cobra.ExactArgs(1),
	RunE: runBundleCreate,
}

// Bundle install subcommand
var bundleInstallCmd = &cobra.Command{
	Use:   "install <bundle-source> <environment-name>",
	Short: "Install a bundle from URL or file path",
	Long: `Install a bundle from a remote URL or local file path.
This uses the same installation logic as the Station UI.

Examples:
  stn bundle install https://github.com/cloudshipai/registry/releases/download/v1.0.0/devops-security-bundle.tar.gz security
  stn bundle install ./my-bundle.tar.gz production
  stn bundle install /path/to/bundle.tar.gz development`,
	Args: cobra.ExactArgs(2),
	RunE: runBundleInstall,
}

func init() {
	// Add flags to create subcommand
	bundleCreateCmd.Flags().String("output", "", "Output path for bundle (defaults to <environment>.tar.gz)")
	bundleCreateCmd.Flags().Bool("local", true, "Save bundle locally (always true for CLI)")
	
	// Add subcommands to main bundle command
	bundleCmd.AddCommand(bundleCreateCmd)
	bundleCmd.AddCommand(bundleInstallCmd)
}

func runBundleCreate(cmd *cobra.Command, args []string) error {
	environmentName := args[0]
	outputPath, _ := cmd.Flags().GetString("output")

	// Get Station config root
	configRoot, err := common.GetStationConfigRoot()
	if err != nil {
		return fmt.Errorf("failed to get station config root: %w", err)
	}

	// Environment directory path
	envPath := filepath.Join(configRoot, "environments", environmentName)
	
	// Check if environment directory exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("environment '%s' not found at %s", environmentName, envPath)
	}

	// Set default output path if not provided
	if outputPath == "" {
		outputPath = fmt.Sprintf("%s.tar.gz", environmentName)
	}

	fmt.Printf("🗂️  Bundling environment: %s\n", environmentName)
	fmt.Printf("📂 Source path: %s\n", envPath)

	// Create bundle service
	bundleService := services.NewBundleService()
	
	// Validate environment
	if err := bundleService.ValidateEnvironment(envPath); err != nil {
		return fmt.Errorf("environment validation failed: %w", err)
	}

	// Get bundle info for preview
	bundleInfo, err := bundleService.GetBundleInfo(envPath)
	if err != nil {
		return fmt.Errorf("failed to analyze environment: %w", err)
	}

	fmt.Printf("📋 Found:\n")
	fmt.Printf("   🤖 %d agent(s): %v\n", len(bundleInfo.AgentFiles), bundleInfo.AgentFiles)
	fmt.Printf("   🔧 %d MCP config(s): %v\n", len(bundleInfo.MCPConfigs), bundleInfo.MCPConfigs)
	if len(bundleInfo.OtherFiles) > 0 {
		fmt.Printf("   📄 %d other file(s): %v\n", len(bundleInfo.OtherFiles), bundleInfo.OtherFiles)
	}

	// Create tar.gz bundle using the same logic as the API
	tarData, err := bundleService.CreateBundle(envPath)
	if err != nil {
		return fmt.Errorf("failed to create bundle: %w", err)
	}

	// Save to file
	if err := os.WriteFile(outputPath, tarData, 0644); err != nil {
		return fmt.Errorf("failed to save bundle: %w", err)
	}

	fmt.Printf("✅ Bundle created: %s\n", outputPath)
	fmt.Printf("📊 Size: %d bytes\n", len(tarData))
	fmt.Printf("\n🚀 Install with:\n")
	fmt.Printf("   stn bundle install %s <environment-name>\n", outputPath)
	fmt.Printf("   Or use the Station UI Bundle installation\n")

	return nil
}

func runBundleInstall(cmd *cobra.Command, args []string) error {
	bundleSource := args[0]
	environmentName := args[1]

	fmt.Printf("📦 Installing bundle from: %s\n", bundleSource)
	fmt.Printf("🎯 Target environment: %s\n", environmentName)

	// Determine source type (URL or file path)
	var sourceType string
	if bundleSource[:4] == "http" {
		sourceType = "url"
	} else {
		sourceType = "file"
		// Check if local file exists
		if _, err := os.Stat(bundleSource); os.IsNotExist(err) {
			return fmt.Errorf("bundle file not found: %s", bundleSource)
		}
	}

	// Create request payload
	payload := map[string]interface{}{
		"bundle_location":  bundleSource,
		"environment_name": environmentName,
		"source":          sourceType,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to create request payload: %w", err)
	}

	// Make API request to Station server
	apiURL := "http://localhost:8585/api/v1/bundles/install"
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to connect to Station API at %s: %w\nMake sure Station server is running with 'stn serve'", apiURL, err)
	}
	defer resp.Body.Close()

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	// Check if installation was successful
	if success, ok := result["success"].(bool); !ok || !success {
		errorMsg, _ := result["error"].(string)
		message, _ := result["message"].(string)
		if errorMsg != "" {
			return fmt.Errorf("bundle installation failed: %s", errorMsg)
		}
		if message != "" {
			return fmt.Errorf("bundle installation failed: %s", message)
		}
		return fmt.Errorf("bundle installation failed: unknown error")
	}

	fmt.Printf("✅ Bundle installed successfully!\n")
	fmt.Printf("🎯 Environment '%s' is ready to use\n", environmentName)
	fmt.Printf("\n🔧 Next steps:\n")
	fmt.Printf("   stn sync                     # Sync MCP tools\n")
	fmt.Printf("   stn agent list --env %s     # List available agents\n", environmentName)
	fmt.Printf("   open http://localhost:8585   # View in Station UI\n")

	return nil
}