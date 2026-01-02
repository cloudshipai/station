package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"station/internal/services"
	"station/internal/version"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type UpMetadata struct {
	SourceType    string    `json:"source_type"`
	SourceName    string    `json:"source_name"`
	SourcePath    string    `json:"source_path"`
	VariablesFile string    `json:"variables_file"`
	ConfiguredAt  time.Time `json:"configured_at"`
	StnVersion    string    `json:"stn_version"`
}

const upMetadataFile = ".stn-up-metadata.json"

func readVariablesYml(envPath string) (map[string]string, error) {
	variablesPath := filepath.Join(envPath, "variables.yml")
	data, err := os.ReadFile(variablesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("failed to read variables.yml: %w", err)
	}

	var rawVars map[string]interface{}
	if err := yaml.Unmarshal(data, &rawVars); err != nil {
		return nil, fmt.Errorf("failed to parse variables.yml: %w", err)
	}

	vars := make(map[string]string)
	for key, value := range rawVars {
		vars[key] = fmt.Sprintf("%v", value)
	}
	return vars, nil
}

func mergeVariablesWithEnv(variables map[string]string) map[string]string {
	merged := make(map[string]string)
	for key, defaultValue := range variables {
		if envValue := os.Getenv(key); envValue != "" {
			merged[key] = envValue
		} else {
			merged[key] = defaultValue
		}
	}
	return merged
}

func writeUpMetadata(imageName string, metadata UpMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	metadataPath := fmt.Sprintf("/home/station/.config/station/%s", upMetadataFile)
	writeCmd := dockerCommand("run", "--rm",
		"-v", "station-config:/home/station/.config/station",
		imageName,
		"sh", "-c", fmt.Sprintf("echo '%s' > %s", string(data), metadataPath))
	return writeCmd.Run()
}

func readUpMetadata(imageName string) (*UpMetadata, error) {
	metadataPath := fmt.Sprintf("/home/station/.config/station/%s", upMetadataFile)
	readCmd := dockerCommand("run", "--rm",
		"-v", "station-config:/home/station/.config/station",
		imageName,
		"cat", metadataPath)

	output, err := readCmd.Output()
	if err != nil {
		return nil, err
	}

	var metadata UpMetadata
	if err := json.Unmarshal(output, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func bundleEnvironmentOnTheFly(envName string) (string, string, error) {
	envPath := filepath.Join(os.Getenv("HOME"), ".config", "station", "environments", envName)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("environment not found: %s\nExpected at: %s", envName, envPath)
	}

	bundleService := services.NewBundleService()
	tarData, err := bundleService.CreateBundle(envPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create bundle: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "stn-up-bundle-*.tar.gz")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := tmpFile.Write(tarData); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", "", fmt.Errorf("failed to write bundle: %w", err)
	}
	tmpFile.Close()

	return tmpFile.Name(), envPath, nil
}

// dockerCommand creates an exec.Command for docker with proper environment inheritance
func dockerCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("docker", args...)
	cmd.Env = os.Environ() // Inherit environment to preserve Docker context
	return cmd
}

var upCmd = &cobra.Command{
	Use:   "up [environment]",
	Short: "Start Station server in a Docker container",
	Long: `Start a containerized Station server with isolated configuration and workspace access.

This command:
- Builds or uses existing Station runtime container
- Stores all configuration in Docker volume (isolated from host)
- Mounts your workspace directory with correct file permissions
- Exposes Dynamic Agent MCP port (8587) by default
- Use --dev to also expose UI (8585) and MCP (8586) ports

Port Modes:
  Default:  Only 8587 (Dynamic Agent MCP) - for production/CloudShip
  --dev:    8585 (UI) + 8586 (MCP) + 8587 - for local development

Examples:
  # Production mode (only 8587 exposed)
  stn up default
  stn up my-environment

  # Development mode (all ports exposed)
  stn up default --dev
  stn up my-environment --dev

  # Start with a CloudShip bundle
  stn up --bundle e26b414a-f076-4135-927f-810bc1dc892a -e AWS_KEY=$AWS_KEY
  
  # Reset and reconfigure
  stn down --remove-volume
  stn up new-environment
`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUp,
}

func init() {
	upCmd.Flags().StringP("workspace", "w", "", "Workspace directory to mount (default: current directory)")
	upCmd.Flags().BoolP("detach", "d", true, "Run container in background")
	upCmd.Flags().Bool("upgrade", false, "Rebuild container image before starting")
	upCmd.Flags().StringSlice("env", []string{}, "Additional environment variables to pass through")
	upCmd.Flags().Bool("dev", false, "Development mode: expose UI (8585) and MCP (8586) ports. Default only exposes Dynamic Agent MCP (8587)")

	// Init flags for first-time setup
	upCmd.Flags().String("provider", "", "AI provider for initialization (openai, gemini, anthropic, custom). Defaults to openai")
	upCmd.Flags().String("model", "", "AI model to use (e.g., gpt-5-mini, gemini-2.0-flash-exp). Defaults based on provider")
	upCmd.Flags().String("api-key", "", "API key for AI provider (alternative to environment variables)")
	upCmd.Flags().String("base-url", "", "Custom base URL for OpenAI-compatible endpoints")
	upCmd.Flags().Bool("ship", false, "Bootstrap with ship CLI MCP integration for filesystem access")
	upCmd.Flags().BoolP("yes", "y", false, "Use defaults without interactive prompts")
	// Removed --jaeger flag - Jaeger should be run separately via docker-compose or standalone container
	// Station exports traces to OTEL endpoint configured in config.yaml (otel_endpoint)

	// Bundle flag for instant sandbox
	upCmd.Flags().String("bundle", "", "CloudShip bundle ID or URL to install and run")

	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	checkDockerCmd := dockerCommand("info")
	checkDockerCmd.Stdout = nil
	checkDockerCmd.Stderr = nil
	if err := checkDockerCmd.Run(); err != nil {
		return fmt.Errorf("Docker daemon is not running. Please start Docker Desktop and try again")
	}

	if containerID := getRunningStationContainer(); containerID != "" {
		fmt.Printf("✅ Station server already running (container: %s)\n", containerID[:12])
		workspace, _ := cmd.Flags().GetString("workspace")
		if workspace != "" {
			fmt.Printf("ℹ️  To change workspace, run 'stn down' first, then 'stn up --workspace %s'\n", workspace)
		}
		return nil
	}

	imageName := "station-server:latest"
	bundleSource, _ := cmd.Flags().GetString("bundle")
	originalBundleFlag := bundleSource
	envArg := ""
	if len(args) > 0 {
		envArg = args[0]
	}

	if envArg != "" && bundleSource != "" {
		return fmt.Errorf("cannot specify both environment argument and --bundle flag")
	}

	checkVolumeCmd := dockerCommand("volume", "inspect", "station-config")
	checkVolumeCmd.Stdout = nil
	checkVolumeCmd.Stderr = nil
	volumeExists := checkVolumeCmd.Run() == nil

	var existingMetadata *UpMetadata
	if volumeExists {
		if !dockerImageExists(imageName) {
			if err := buildRuntimeContainer(); err != nil {
				return fmt.Errorf("failed to build container: %w", err)
			}
		}
		existingMetadata, _ = readUpMetadata(imageName)
	}

	if envArg == "" && bundleSource == "" {
		if existingMetadata != nil {
			fmt.Printf("🔄 Restarting previously configured Station (%s: %s)\n",
				existingMetadata.SourceType, existingMetadata.SourceName)
		} else if !volumeExists {
			return fmt.Errorf("no configuration found\n\nUsage:\n  stn up <environment>           Bundle and serve local environment\n  stn up --bundle <id>           Download and serve CloudShip bundle\n\nExample:\n  stn up default")
		}
	}

	if (envArg != "" || bundleSource != "") && existingMetadata != nil {
		newSource := envArg
		if bundleSource != "" {
			newSource = bundleSource
		}
		if existingMetadata.SourceName != newSource {
			return fmt.Errorf("already configured with %s: %s\n\nTo reconfigure, run:\n  stn down --remove-volume\n  stn up %s",
				existingMetadata.SourceType, existingMetadata.SourceName, newSource)
		}
	}

	var envVariables map[string]string
	var envPath string

	if envArg != "" {
		fmt.Printf("📦 Bundling environment: %s\n", envArg)
		bundlePath, path, err := bundleEnvironmentOnTheFly(envArg)
		if err != nil {
			return err
		}
		defer os.Remove(bundlePath)
		bundleSource = bundlePath
		envPath = path

		vars, err := readVariablesYml(envPath)
		if err != nil {
			return fmt.Errorf("failed to read variables.yml: %w", err)
		}
		envVariables = mergeVariablesWithEnv(vars)
		if len(envVariables) > 0 {
			fmt.Printf("📋 Loaded %d variables from variables.yml\n", len(envVariables))
		}
	}

	workspace, _ := cmd.Flags().GetString("workspace")
	if workspace == "" {
		var err error
		workspace, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return fmt.Errorf("failed to resolve workspace path: %w", err)
	}

	if _, err := os.Stat(absWorkspace); os.IsNotExist(err) {
		return fmt.Errorf("workspace directory does not exist: %s", absWorkspace)
	}

	fmt.Printf("📁 Workspace: %s\n", absWorkspace)

	needsInit := false
	if !volumeExists {
		fmt.Printf("📦 Creating Station data volume (first run)...\n")
		createVolumeCmd := dockerCommand("volume", "create", "station-config")
		if err := createVolumeCmd.Run(); err != nil {
			return fmt.Errorf("failed to create station-config volume: %w", err)
		}
		fmt.Printf("✅ Created Station data volume\n")
		needsInit = true
	} else {
		checkConfigCmd := dockerCommand("run", "--rm",
			"-v", "station-config:/home/station/.config/station",
			imageName,
			"test", "-f", "/home/station/.config/station/config.yaml")
		checkConfigCmd.Stdout = nil
		checkConfigCmd.Stderr = nil
		configExists := checkConfigCmd.Run() == nil

		if !configExists {
			fmt.Printf("📦 Station volume exists but is empty, initializing...\n")
			needsInit = true
		}
	}

	if needsInit {
		fmt.Printf("💡 Note: All configuration stored in Docker volume (isolated from host)\n")
		fmt.Printf("💡 Manage settings via UI at http://localhost:8585/settings\n")
	}

	// Build or check for image
	upgrade, _ := cmd.Flags().GetBool("upgrade")

	if upgrade || !dockerImageExists(imageName) {
		if upgrade {
			fmt.Printf("🔨 Upgrading Station runtime container...\n")
		} else {
			fmt.Printf("🔨 Building Station runtime container (first run)...\n")
		}
		if err := buildRuntimeContainer(); err != nil {
			return fmt.Errorf("failed to build container: %w", err)
		}
	}

	// Get current user info for proper file permissions
	uid := os.Getuid()
	gid := os.Getgid()

	// Initialize the container volume if needed
	if needsInit {
		// Check if host has existing Station config we can use
		hostConfigDir := filepath.Join(os.Getenv("HOME"), ".config", "station")
		hostConfigFile := filepath.Join(hostConfigDir, "config.yaml")
		hostConfigExists := false
		if _, err := os.Stat(hostConfigFile); err == nil {
			hostConfigExists = true
		}

		// If --bundle is passed and host config exists, copy host config to container
		// This allows users to run `stn init` once and reuse config for all containers
		if bundleSource != "" && hostConfigExists {
			fmt.Printf("📋 Found local Station config at %s\n", hostConfigDir)
			fmt.Printf("🔧 Importing host configuration to container...\n")

			// Read host config to extract provider and model settings
			hostConfig, err := readConfigFile(hostConfigFile)
			if err != nil {
				log.Printf("Warning: Failed to read host config: %v (will use defaults)", err)
			} else {
				// Extract provider and model from host config for init
				if p, ok := hostConfig["ai_provider"].(string); ok && p != "" {
					cmd.Flags().Set("provider", p)
				}
				if m, ok := hostConfig["ai_model"].(string); ok && m != "" {
					cmd.Flags().Set("model", m)
				}
				fmt.Printf("   Using provider: %s, model: %s from host config\n",
					hostConfig["ai_provider"], hostConfig["ai_model"])
			}

			// Copy config to container and fix paths + localhost URLs for Docker networking
			// - database_url: point to container path
			// - workspace: point to container path
			// - localhost URLs: rewrite to host.docker.internal for Docker networking
			fmt.Printf("   Copying config and fixing paths for container...\n")

			// Build sed command to fix paths (nats:// URLs stay 127.0.0.1 for embedded NATS)
			sedCmd := `cp /host-config/config.yaml /home/station/.config/station/config.yaml && \
				sed -i 's|database_url:.*|database_url: /home/station/.config/station/station.db|' /home/station/.config/station/config.yaml && \
				sed -i 's|workspace:.*|workspace: /home/station/.config/station|' /home/station/.config/station/config.yaml && \
				sed -i 's|localhost:|host.docker.internal:|g' /home/station/.config/station/config.yaml && \
				sed -i 's|127\.0\.0\.1:|host.docker.internal:|g' /home/station/.config/station/config.yaml && \
				sed -i 's|nats://host.docker.internal:|nats://127.0.0.1:|g' /home/station/.config/station/config.yaml`

			// Check if STATION_LOCAL_MODE is set - check both env var and --env flags
			localModeOverride := os.Getenv("STATION_LOCAL_MODE")
			// Also check --env flags for STATION_LOCAL_MODE
			envVars, _ := cmd.Flags().GetStringSlice("env")
			for _, envVar := range envVars {
				if strings.HasPrefix(envVar, "STATION_LOCAL_MODE=") {
					localModeOverride = strings.TrimPrefix(envVar, "STATION_LOCAL_MODE=")
					break
				}
			}
			if localModeOverride != "" {
				sedCmd += fmt.Sprintf(` && sed -i 's|local_mode:.*|local_mode: %s|' /home/station/.config/station/config.yaml`, localModeOverride)
			}

			copyArgs := []string{
				"run", "--rm",
				"-v", "station-config:/home/station/.config/station",
				"-v", fmt.Sprintf("%s:/host-config:ro", hostConfigDir),
				imageName,
				"sh", "-c", sedCmd,
			}

			copyCmd := dockerCommand(copyArgs...)
			copyCmd.Stderr = os.Stderr // Show any errors
			if err := copyCmd.Run(); err != nil {
				log.Printf("Warning: Failed to copy host config: %v (will use defaults)", err)
			} else {
				fmt.Printf("   ✅ Config copied with localhost → host.docker.internal\n")
			}
			// Still need to run init to create database schema, but settings come from host config
		}
	}

	if needsInit {
		fmt.Printf("🔧 Initializing Station in container volume...\n")

		// Get init parameters from flags or defaults
		provider, _ := cmd.Flags().GetString("provider")
		model, _ := cmd.Flags().GetString("model")
		apiKey, _ := cmd.Flags().GetString("api-key")
		baseURL, _ := cmd.Flags().GetString("base-url")
		ship, _ := cmd.Flags().GetBool("ship")
		useDefaults, _ := cmd.Flags().GetBool("yes")

		// If --bundle is passed, implicitly use defaults (no interactive prompts)
		if bundleSource != "" {
			useDefaults = true
		}

		// Default to openai if not specified
		if provider == "" {
			provider = "openai"
		}

		// Run init in a temporary container
		// Note: Running as root for volume initialization simplicity
		initArgs := []string{
			"run", "--rm",
		}

		// If --api-key flag is provided, use it as STN_AI_API_KEY
		if apiKey != "" {
			initArgs = append(initArgs, "-e", fmt.Sprintf("STN_AI_API_KEY=%s", apiKey))
		}

		// Pass through AI provider env vars for init (if not already set via flag)
		if apiKey == "" {
			aiEnvVars := []string{
				"OPENAI_API_KEY",
				"ANTHROPIC_API_KEY",
				"GOOGLE_API_KEY",
				"GEMINI_API_KEY",
				"STN_AI_API_KEY",
				"AI_API_KEY",
			}
			for _, envVar := range aiEnvVars {
				if value := os.Getenv(envVar); value != "" {
					initArgs = append(initArgs, "-e", fmt.Sprintf("%s=%s", envVar, value))
				}
			}
		}

		initArgs = append(initArgs,
			"-v", "station-config:/home/station/.config/station",
			"-e", "HOME=/home/station",
			"-e", "STATION_CONFIG_DIR=/home/station/.config/station",
			imageName,
			"stn", "init",
			"--provider", provider,
		)

		// Add optional init flags
		if model != "" {
			initArgs = append(initArgs, "--model", model)
		}
		if baseURL != "" {
			initArgs = append(initArgs, "--base-url", baseURL)
		}
		if ship {
			initArgs = append(initArgs, "--ship")
		}
		if useDefaults {
			initArgs = append(initArgs, "--yes")
		}

		initCmd := dockerCommand(initArgs...)
		initCmd.Stdout = os.Stdout
		initCmd.Stderr = os.Stderr

		if err := initCmd.Run(); err != nil {
			return fmt.Errorf("failed to initialize Station in container: %w", err)
		}

		fmt.Printf("✅ Station initialized in container volume\n")
	}

	// Install bundle if specified
	if bundleSource != "" {
		fmt.Printf("📦 Installing bundle: %s\n", bundleSource)

		// If it's a UUID, download from CloudShip first
		var bundlePath string
		if isUUID(bundleSource) {
			fmt.Printf("   Downloading from CloudShip...\n")
			downloadedPath, err := downloadBundleFromCloudShip(bundleSource)
			if err != nil {
				return fmt.Errorf("failed to download bundle from CloudShip: %w\n\nMake sure you're authenticated with 'stn auth login'", err)
			}
			defer os.Remove(downloadedPath) // Clean up temp file
			bundlePath = downloadedPath
		} else if strings.HasPrefix(bundleSource, "http://") || strings.HasPrefix(bundleSource, "https://") {
			// Download from URL to temp file
			fmt.Printf("   Downloading from URL...\n")
			resp, err := http.Get(bundleSource)
			if err != nil {
				return fmt.Errorf("failed to download bundle: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				return fmt.Errorf("failed to download bundle: HTTP %d", resp.StatusCode)
			}

			tmpFile, err := os.CreateTemp("", "station-bundle-*.tar.gz")
			if err != nil {
				return fmt.Errorf("failed to create temp file: %w", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := io.Copy(tmpFile, resp.Body); err != nil {
				tmpFile.Close()
				return fmt.Errorf("failed to save bundle: %w", err)
			}
			tmpFile.Close()
			bundlePath = tmpFile.Name()
		} else {
			// Assume it's a local file path
			if _, err := os.Stat(bundleSource); os.IsNotExist(err) {
				return fmt.Errorf("bundle file not found: %s", bundleSource)
			}
			bundlePath = bundleSource
		}

		// Install bundle into the default environment so it becomes the primary sandbox
		fmt.Printf("   Installing bundle into default environment...\n")

		// Copy bundle to container volume via docker cp to a temp location,
		// then install with stn bundle install
		installArgs := []string{
			"run", "--rm",
			"-v", "station-config:/home/station/.config/station",
			"-e", "HOME=/home/station",
			"-e", "STATION_CONFIG_DIR=/home/station/.config/station",
		}

		// Mount the bundle file into the container
		bundleMount := fmt.Sprintf("%s:/tmp/bundle.tar.gz:ro", bundlePath)
		installArgs = append(installArgs, "-v", bundleMount)

		// Pass through AI provider env vars for bundle sync
		aiEnvVars := []string{
			"OPENAI_API_KEY",
			"ANTHROPIC_API_KEY",
			"GOOGLE_API_KEY",
			"GEMINI_API_KEY",
			"STN_AI_API_KEY",
		}
		for _, envVar := range aiEnvVars {
			if value := os.Getenv(envVar); value != "" {
				installArgs = append(installArgs, "-e", fmt.Sprintf("%s=%s", envVar, value))
			}
		}

		// Install bundle into "default" environment - this makes the bundle the primary sandbox
		installArgs = append(installArgs, imageName, "stn", "bundle", "install", "/tmp/bundle.tar.gz", "default", "--force")

		installCmd := dockerCommand(installArgs...)
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr

		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("failed to install bundle: %w", err)
		}

		// Sync the default environment to register MCP tools
		fmt.Printf("   Syncing default environment...\n")
		syncArgs := []string{
			"run", "--rm",
			"-v", "station-config:/home/station/.config/station",
			"-e", "HOME=/home/station",
			"-e", "STATION_CONFIG_DIR=/home/station/.config/station",
		}

		// Pass through AI provider env vars for sync
		for _, envVar := range aiEnvVars {
			if value := os.Getenv(envVar); value != "" {
				syncArgs = append(syncArgs, "-e", fmt.Sprintf("%s=%s", envVar, value))
			}
		}

		syncArgs = append(syncArgs, imageName, "stn", "sync", "default")

		syncCmd := dockerCommand(syncArgs...)
		syncCmd.Stdout = os.Stdout
		syncCmd.Stderr = os.Stderr

		if err := syncCmd.Run(); err != nil {
			// Sync might fail if MCP servers aren't available yet, but that's okay
			fmt.Printf("⚠️  Bundle sync had issues (MCP servers may start when container runs)\n")
		}

		fmt.Printf("✅ Bundle installed to default environment!\n")

		metadata := UpMetadata{
			ConfiguredAt: time.Now(),
			StnVersion:   version.Version,
		}
		if envArg != "" {
			metadata.SourceType = "environment"
			metadata.SourceName = envArg
			metadata.SourcePath = envPath
			metadata.VariablesFile = filepath.Join(envPath, "variables.yml")
		} else {
			metadata.SourceType = "bundle"
			metadata.SourceName = originalBundleFlag
			metadata.SourcePath = ""
		}
		if err := writeUpMetadata(imageName, metadata); err != nil {
			log.Printf("Warning: Failed to save metadata: %v", err)
		}
	}

	// Prepare Docker run command
	dockerArgs := []string{"run", "--name", "station-server"}

	devMode, _ := cmd.Flags().GetBool("dev")
	detach, _ := cmd.Flags().GetBool("detach")
	if detach {
		dockerArgs = append(dockerArgs, "-d")
	} else {
		dockerArgs = append(dockerArgs, "-it")
	}

	// Add restart policy
	dockerArgs = append(dockerArgs, "--restart", "unless-stopped")

	// User mapping to prevent root-owned files in workspace
	if runtime.GOOS == "linux" {
		// On Linux, map to host user so workspace files maintain correct ownership
		dockerArgs = append(dockerArgs, "--user", fmt.Sprintf("%d:%d", uid, gid))

		// Get docker group GID for Docker-in-Docker support
		if stat, err := os.Stat("/var/run/docker.sock"); err == nil {
			dockerGID := getDockerGroupID(stat)
			if dockerGID > 0 {
				// Add supplementary docker group
				dockerArgs = append(dockerArgs, "--group-add", fmt.Sprintf("%d", dockerGID))
			}
		}

		// Add host.docker.internal mapping for Linux (needed for CloudShip/Lighthouse connectivity)
		dockerArgs = append(dockerArgs, "--add-host", "host.docker.internal:host-gateway")

		// With the new user setup, we don't need to fix permissions manually
		// The entrypoint handles it automatically
	}
	// macOS and Windows: Docker Desktop handles permissions automatically

	// Volume mounts
	dockerArgs = append(dockerArgs,
		"-v", fmt.Sprintf("%s:/workspace:rw", absWorkspace),
		"-v", "station-config:/home/station/.config/station:rw", // All Station data in volume (including config.yaml)
	)

	// Pass the host workspace path so container knows the mapping
	dockerArgs = append(dockerArgs,
		"-e", fmt.Sprintf("HOST_WORKSPACE=%s", absWorkspace),
	)

	// Docker socket mount for Dagger (if exists)
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		dockerArgs = append(dockerArgs, "-v", "/var/run/docker.sock:/var/run/docker.sock")
	}

	// Named volume for cache (persists across container restarts)
	dockerArgs = append(dockerArgs, "-v", "station-cache:/home/station/.cache")

	// Port mappings - default only exposes Dynamic Agent MCP (8587) and embedded NATS (4222)
	dockerArgs = append(dockerArgs, "-p", "8587:8587", "-p", "4222:4222")
	if devMode {
		dockerArgs = append(dockerArgs, "-p", "8585:8585", "-p", "8586:8586")
	}

	// Note: Jaeger UI port (16686) not mapped - run Jaeger separately if needed
	// Station exports traces to otel_endpoint configured in config.yaml

	// Environment variables
	if err := addEnvironmentVariables(&dockerArgs, cmd); err != nil {
		log.Printf("Warning: Some environment variables may not be set: %v", err)
	}

	// Add variables from variables.yml (for stn up <environment>)
	if envVariables != nil {
		for key, value := range envVariables {
			dockerArgs = append(dockerArgs, "-e", fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Ensure STATION_CONFIG_DIR is set for proper paths
	dockerArgs = append(dockerArgs, "-e", "STATION_CONFIG_DIR=/home/station/.config/station")

	// Enable dev mode to start API server (management UI)
	dockerArgs = append(dockerArgs, "-e", "STN_DEV_MODE=true")

	// Set working directory
	dockerArgs = append(dockerArgs, "-w", "/workspace")

	dockerArgs = append(dockerArgs, imageName, "stn", "serve", "--database", "/home/station/.config/station/station.db", "--mcp-port", "8586")

	// Run the container
	fmt.Printf("🐳 Starting container...\n")
	dockerCmd := dockerCommand(dockerArgs...)
	dockerCmd.Stdout = os.Stdout
	dockerCmd.Stderr = os.Stderr

	if err := dockerCmd.Run(); err != nil {
		// Clean up failed container to avoid "name already in use" errors
		cleanupCmd := dockerCommand("rm", "-f", "station-server")
		_ = cleanupCmd.Run() // Ignore errors, container might not exist
		return fmt.Errorf("failed to start container: %w", err)
	}

	if devMode {
		if err := updateMCPConfig(absWorkspace); err != nil {
			log.Printf("Warning: Failed to update .mcp.json: %v", err)
			fmt.Printf("⚠️  Please manually add Station to your .mcp.json:\n")
			fmt.Printf(`  "station": {
    "url": "http://localhost:8586/mcp",
    "transport": "http"
  }` + "\n")
		}
	}

	fmt.Printf("\n✅ Station server started successfully!\n")
	fmt.Printf("🔗 Dynamic Agent MCP: http://localhost:8587/mcp\n")
	if devMode {
		fmt.Printf("🔗 MCP: http://localhost:8586/mcp\n")
		fmt.Printf("🔗 UI:  http://localhost:8585\n")
	}
	fmt.Printf("📁 Workspace: %s\n", absWorkspace)

	if detach {
		if devMode {
			fmt.Printf("\n💡 Configuration: Managed via UI at http://localhost:8585/settings\n")
		}
		fmt.Printf("💡 Run 'stn logs' to see container output\n")
		fmt.Printf("💡 Run 'stn down' to stop (data preserved in volume)\n")
		fmt.Printf("💡 Run 'stn down --remove-volume' to delete all data\n")
	}

	return nil
}

func getRunningStationContainer() string {
	cmd := dockerCommand("ps", "-q", "-f", "name=station-server")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func readConfigFile(path string) (map[string]interface{}, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}

func dockerImageExists(imageName string) bool {
	cmd := dockerCommand("images", "-q", imageName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

func buildRuntimeContainer() error {
	_, dockerfileErr := os.Stat("Dockerfile")
	hasLocalDockerfile := dockerfileErr == nil

	if hasLocalDockerfile {
		fmt.Printf("🔨 Building Station container from local Dockerfile...\n")
		buildCmd := dockerCommand("build",
			"--build-arg", "INSTALL_SHIP=true",
			"-t", "station-server:latest",
			".")
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr

		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("failed to build image: %w", err)
		}

		fmt.Printf("✅ Successfully built Station container from local source\n")
		return nil
	}

	fmt.Printf("📥 Pulling Station container from registry...\n")
	pullCmd := dockerCommand("pull", "ghcr.io/cloudshipai/station:latest")
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr

	if pullErr := pullCmd.Run(); pullErr != nil {
		return fmt.Errorf("failed to pull image from registry: %w", pullErr)
	}

	tagCmd := dockerCommand("tag", "ghcr.io/cloudshipai/station:latest", "station-server:latest")
	if tagErr := tagCmd.Run(); tagErr != nil {
		return fmt.Errorf("failed to tag pulled image: %w", tagErr)
	}
	fmt.Printf("✅ Successfully pulled Station container\n")
	return nil
}

func addUserMapping(dockerArgs *[]string) error {
	// Cross-platform user mapping strategy
	switch runtime.GOOS {
	case "linux":
		// On Linux, use actual UID/GID plus docker group for socket access
		uid := os.Getuid()
		gid := os.Getgid()

		// Get docker socket group ID for Docker-in-Docker support
		dockerGID := gid
		if stat, err := os.Stat("/var/run/docker.sock"); err == nil {
			if dgid := getDockerGroupID(stat); dgid > 0 {
				dockerGID = dgid
			}
		}

		// Set user with supplementary docker group
		*dockerArgs = append(*dockerArgs, "--user", fmt.Sprintf("%d:%d", uid, dockerGID))

		// Mount passwd and group for user resolution (if they exist)
		if _, err := os.Stat("/etc/passwd"); err == nil {
			*dockerArgs = append(*dockerArgs, "-v", "/etc/passwd:/etc/passwd:ro")
		}
		if _, err := os.Stat("/etc/group"); err == nil {
			*dockerArgs = append(*dockerArgs, "-v", "/etc/group:/etc/group:ro")
		}

	case "darwin":
		// macOS doesn't have /etc/passwd in the same way
		// Docker Desktop for Mac handles file permissions differently
		// Files created in mounted volumes automatically get correct ownership
		// So we don't need to set --user on macOS
		log.Printf("Running on macOS - Docker Desktop handles file permissions automatically")

	case "windows":
		// Windows with Docker Desktop also handles permissions automatically
		log.Printf("Running on Windows - Docker Desktop handles file permissions automatically")

	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	return nil
}

func addEnvironmentVariables(dockerArgs *[]string, cmd *cobra.Command) error {
	// Note: config.yaml is now in Docker volume, not mounted from host
	// All configuration is managed through the volume

	// Check if --api-key flag was provided
	apiKey, _ := cmd.Flags().GetString("api-key")
	if apiKey != "" {
		// If --api-key flag is set, use it as STN_AI_API_KEY
		*dockerArgs = append(*dockerArgs, "-e", fmt.Sprintf("STN_AI_API_KEY=%s", apiKey))
	}

	// Essential AI provider keys to pass through (if not already set via flag)
	aiKeys := []string{
		"OPENAI_API_KEY",
		"ANTHROPIC_API_KEY",
		"GOOGLE_API_KEY",
		"GEMINI_API_KEY",
		"AI_API_KEY",
		"STN_AI_API_KEY",
		"STN_AI_BASE_URL",
		"STN_AI_PROVIDER",
		"STN_AI_MODEL",
	}

	// AWS credentials for cost explorer and other tools
	awsKeys := []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_REGION",
		"AWS_DEFAULT_REGION",
		"AWS_PROFILE",
	}

	// CloudShip/Lighthouse integration (for registration and remote control)
	cloudshipKeys := []string{
		"STN_CLOUDSHIP_ENABLED",
		"STN_CLOUDSHIP_KEY",
		"STN_CLOUDSHIP_ENDPOINT",
		"STN_CLOUDSHIP_STATION_ID",
		"STN_CLOUDSHIP_NAME", // v2: user-defined station name for CloudShip
		"STN_CLOUDSHIP_BASE_URL",
		"CLOUDSHIP_API_KEY",
		"CLOUDSHIPAI_REGISTRATION_KEY",
	}

	// CloudShip OAuth settings (for MCP authentication via CloudShip)
	oauthKeys := []string{
		"STN_CLOUDSHIP_OAUTH_ENABLED",
		"STN_CLOUDSHIP_OAUTH_CLIENT_ID",
		"STN_CLOUDSHIP_OAUTH_AUTH_URL",
		"STN_CLOUDSHIP_OAUTH_TOKEN_URL",
		"STN_CLOUDSHIP_OAUTH_INTROSPECT_URL",
		"STN_CLOUDSHIP_OAUTH_REDIRECT_URI",
		"STN_CLOUDSHIP_OAUTH_SCOPES",
	}

	// Station mode control
	modeKeys := []string{
		"STATION_LOCAL_MODE", // Set to "false" to enable OAuth authentication
	}

	// OpenTelemetry for distributed tracing
	otelKeys := []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_SERVICE_NAME",
		"OTEL_TRACES_EXPORTER",
	}

	// Other tool-specific keys
	otherKeys := []string{
		"GITHUB_TOKEN",
		"GITLAB_TOKEN",
		"SLACK_BOT_TOKEN",
		"SLACK_APP_TOKEN",
	}

	// Combine all keys
	allKeys := append(aiKeys, awsKeys...)
	allKeys = append(allKeys, cloudshipKeys...)
	allKeys = append(allKeys, oauthKeys...)
	allKeys = append(allKeys, modeKeys...)
	allKeys = append(allKeys, otelKeys...)
	allKeys = append(allKeys, otherKeys...)

	// Pass through environment variables that exist
	for _, key := range allKeys {
		if value := os.Getenv(key); value != "" {
			*dockerArgs = append(*dockerArgs, "-e", fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Add any additional env vars specified via --env flag
	envVars, _ := cmd.Flags().GetStringSlice("env")
	for _, envVar := range envVars {
		*dockerArgs = append(*dockerArgs, "-e", envVar)
	}

	// Set HOME appropriately for the container
	*dockerArgs = append(*dockerArgs, "-e", "HOME=/home/station")

	// Set PATH to include common tool locations
	*dockerArgs = append(*dockerArgs, "-e", "PATH=/home/station/.local/bin:/home/station/.cargo/bin:/usr/local/bin:/usr/bin:/bin")

	return nil
}

func updateMCPConfig(workspace string) error {
	mcpPath := filepath.Join(workspace, ".mcp.json")

	var config map[string]interface{}

	// Read existing config if it exists
	if data, err := ioutil.ReadFile(mcpPath); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			log.Printf("Warning: Failed to parse existing .mcp.json: %v", err)
			config = make(map[string]interface{})
		}
	} else {
		config = make(map[string]interface{})
	}

	// Ensure mcpServers exists
	if _, ok := config["mcpServers"]; !ok {
		config["mcpServers"] = make(map[string]interface{})
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		return fmt.Errorf(".mcp.json has invalid format for mcpServers")
	}

	// Add or update Station server configuration (HTTP transport)
	mcpServers["station"] = map[string]interface{}{
		"type": "http",
		"url":  "http://localhost:8586/mcp",
	}

	// Write back the updated config
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := ioutil.WriteFile(mcpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write .mcp.json: %w", err)
	}

	fmt.Printf("✅ Updated .mcp.json with Station server configuration\n")
	return nil
}
