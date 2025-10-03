package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"station/cmd/main/handlers/common"
	"station/internal/config"
	"station/internal/services"
)

// Unified bundle command with create, install, and share subcommands
var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Bundle management commands",
	Long: `Create, install, and share Station bundles.

Subcommands:
  create   Create a bundle from an environment
  install  Install a bundle from URL or file path
  share    Upload a bundle to CloudShip`,
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

// Bundle share subcommand
var bundleShareCmd = &cobra.Command{
	Use:   "share <bundle-path-or-environment>",
	Short: "Upload a bundle to CloudShip",
	Long: `Upload a bundle (.tar.gz) to your CloudShip account.

This command uploads bundles to CloudShip's public bundle API, making them
accessible to your organization. Requires CloudShip to be configured with
a valid registration key.

If you provide an environment name, it will first create the bundle and then upload it.
If you provide a .tar.gz file path, it will upload that file directly.

Examples:
  stn bundle share default                    # Create and upload default environment
  stn bundle share ./my-bundle.tar.gz         # Upload existing bundle file
  stn bundle share production --api-url https://api.cloudshipai.com`,
	Args: cobra.ExactArgs(1),
	RunE: runBundleShare,
}

func init() {
	// Add flags to create subcommand
	bundleCreateCmd.Flags().String("output", "", "Output path for bundle (defaults to <environment>.tar.gz)")
	bundleCreateCmd.Flags().Bool("local", true, "Save bundle locally (always true for CLI)")

	// Add flags to share subcommand
	bundleShareCmd.Flags().String("api-url", "https://api.cloudshipai.com", "CloudShip API URL")
	bundleShareCmd.Flags().Bool("keep-local", false, "Keep the local bundle file after upload (only for environment uploads)")

	// Add subcommands to main bundle command
	bundleCmd.AddCommand(bundleCreateCmd)
	bundleCmd.AddCommand(bundleInstallCmd)
	bundleCmd.AddCommand(bundleShareCmd)
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

	// Use BundleService to install bundle directly (no server dependency)
	bundleService := services.NewBundleService()
	result, err := bundleService.InstallBundle(bundleSource, environmentName)
	if err != nil || !result.Success {
		errorMsg := result.Error
		if errorMsg == "" && err != nil {
			errorMsg = err.Error()
		}
		return fmt.Errorf("bundle installation failed: %s", errorMsg)
	}

	fmt.Printf("✅ Bundle installed successfully!\n")
	fmt.Printf("🎯 Environment '%s' is ready to use\n", result.EnvironmentName)
	fmt.Printf("📊 Installed: %d agents, %d MCP configs\n", result.InstalledAgents, result.InstalledMCPs)
	fmt.Printf("\n🔧 Next steps:\n")
	fmt.Printf("   stn sync %s                  # Sync MCP tools\n", result.EnvironmentName)
	fmt.Printf("   stn agent list --env %s     # List available agents\n", result.EnvironmentName)
	fmt.Printf("   open http://localhost:8585   # View in Station UI\n")

	return nil
}

// CloudShip upload response
type CloudShipUploadResponse struct {
	BundleID     string `json:"bundle_id"`
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	Organization string `json:"organization"`
	UploadedAt   string `json:"uploaded_at"`
	DownloadURL  string `json:"download_url"`
}

func runBundleShare(cmd *cobra.Command, args []string) error {
	source := args[0]
	apiURL, _ := cmd.Flags().GetString("api-url")
	keepLocal, _ := cmd.Flags().GetBool("keep-local")

	// Load Station config to get CloudShip registration key
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	// Check if CloudShip is configured
	if !cfg.CloudShip.Enabled || cfg.CloudShip.RegistrationKey == "" {
		return fmt.Errorf("CloudShip is not configured. Please set cloudship.enabled=true and cloudship.registration_key in your config")
	}

	registrationKey := cfg.CloudShip.RegistrationKey

	// Determine if source is a file or environment
	var bundlePath string
	var isTemporary bool

	if strings.HasSuffix(source, ".tar.gz") {
		// Source is already a bundle file
		if _, err := os.Stat(source); os.IsNotExist(err) {
			return fmt.Errorf("bundle file not found: %s", source)
		}
		bundlePath = source
		isTemporary = false
		fmt.Printf("📦 Using existing bundle: %s\n", bundlePath)
	} else {
		// Source is an environment name - create bundle first
		configRoot, err := common.GetStationConfigRoot()
		if err != nil {
			return fmt.Errorf("failed to get station config root: %w", err)
		}

		envPath := filepath.Join(configRoot, "environments", source)
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			return fmt.Errorf("environment '%s' not found at %s", source, envPath)
		}

		fmt.Printf("🗂️  Creating bundle from environment: %s\n", source)

		bundleService := services.NewBundleService()
		tarData, err := bundleService.CreateBundle(envPath)
		if err != nil {
			return fmt.Errorf("failed to create bundle: %w", err)
		}

		// Save to temporary file
		bundlePath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.tar.gz", source))
		if err := os.WriteFile(bundlePath, tarData, 0644); err != nil {
			return fmt.Errorf("failed to save bundle: %w", err)
		}

		isTemporary = !keepLocal
		fmt.Printf("✅ Bundle created: %s (%d bytes)\n", bundlePath, len(tarData))
	}

	// Clean up temporary file if needed
	if isTemporary {
		defer func() {
			if err := os.Remove(bundlePath); err != nil {
				fmt.Printf("⚠️  Warning: Failed to remove temporary bundle: %v\n", err)
			}
		}()
	}

	// Upload to CloudShip
	fmt.Printf("☁️  Uploading to CloudShip...\n")

	response, err := uploadBundleToCloudShip(apiURL, registrationKey, bundlePath)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	fmt.Printf("✅ Bundle uploaded successfully!\n")
	fmt.Printf("📦 Bundle ID: %s\n", response.BundleID)
	fmt.Printf("🏢 Organization: %s\n", response.Organization)
	fmt.Printf("📊 Size: %d bytes\n", response.Size)
	fmt.Printf("📅 Uploaded: %s\n", response.UploadedAt)
	fmt.Printf("\n🔗 Download URL: %s%s\n", apiURL, response.DownloadURL)
	fmt.Printf("\n🚀 Install on another station:\n")
	fmt.Printf("   stn bundle install %s%s <environment-name>\n", apiURL, response.DownloadURL)

	return nil
}

func uploadBundleToCloudShip(apiURL, registrationKey, bundlePath string) (*CloudShipUploadResponse, error) {
	// Open the bundle file
	file, err := os.Open(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open bundle: %w", err)
	}
	defer file.Close()

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("bundle", filepath.Base(bundlePath))
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}

	writer.Close()

	// Create HTTP request
	uploadURL := fmt.Sprintf("%s/api/public/bundles/upload", strings.TrimSuffix(apiURL, "/"))
	req, err := http.NewRequest("POST", uploadURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Registration-Key", registrationKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("upload failed (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var uploadResp CloudShipUploadResponse
	if err := json.Unmarshal(bodyBytes, &uploadResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &uploadResp, nil
}