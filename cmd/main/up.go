package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// dockerCommand creates an exec.Command for docker with proper environment inheritance
func dockerCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("docker", args...)
	cmd.Env = os.Environ() // Inherit environment to preserve Docker context
	return cmd
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start Station server in a Docker container",
	Long: `Start a containerized Station server with isolated configuration and workspace access.

This command:
- Builds or uses existing Station runtime container
- Stores all configuration in Docker volume (isolated from host)
- Mounts your workspace directory with correct file permissions
- Automatically configures .mcp.json for Claude integration
- Exposes ports for MCP (3000), Dynamic MCP (3030), UI (8585)

Key Features:
- Config management via UI (restart container to apply changes)
- Workspace files maintain your user ownership (no root permission issues)
- Data persists across container restarts in Docker volume

Examples:
  # Basic usage
  stn up                                    # Start with current directory as workspace
  stn up --workspace ~/code                 # Use specific directory as workspace

  # First-time setup with configuration (uses environment variables)
  stn up --provider openai --ship           # Init with OpenAI (requires OPENAI_API_KEY env var)
  stn up --provider gemini --model gemini-2.0-flash-exp --yes

  # Pass API key via flag (no environment variable needed)
  stn up --provider openai --api-key sk-xxx... --yes
  stn up --provider gemini --api-key xxx... --model gemini-2.0-flash-exp --yes

  # Custom provider (Ollama, Anthropic, etc.)
  stn up --provider custom --base-url http://localhost:11434/v1 --model llama2

  # Advanced options
  stn up --upgrade                          # Rebuild container image first
  stn up --env CUSTOM_VAR=value            # Pass additional env vars
`,
	RunE: runUp,
}

func init() {
	upCmd.Flags().StringP("workspace", "w", "", "Workspace directory to mount (default: current directory)")
	upCmd.Flags().BoolP("detach", "d", true, "Run container in background")
	upCmd.Flags().Bool("upgrade", false, "Rebuild container image before starting")
	upCmd.Flags().StringSlice("env", []string{}, "Additional environment variables to pass through")
	upCmd.Flags().Bool("develop", false, "Enable Genkit Developer UI mode (exposes port 4033 for reflection API)")
	upCmd.Flags().String("environment", "default", "Station environment to use in develop mode (e.g., default, production, security)")

	// Init flags for first-time setup
	upCmd.Flags().String("provider", "", "AI provider for initialization (openai, gemini, custom)")
	upCmd.Flags().String("model", "", "AI model to use (e.g., gpt-4o-mini, gemini-2.0-flash-exp)")
	upCmd.Flags().String("api-key", "", "API key for AI provider (alternative to environment variables)")
	upCmd.Flags().String("base-url", "", "Custom base URL for OpenAI-compatible endpoints")
	upCmd.Flags().Bool("ship", false, "Bootstrap with ship CLI MCP integration for filesystem access")
	upCmd.Flags().BoolP("yes", "y", false, "Use defaults without interactive prompts")
	upCmd.Flags().Bool("jaeger", true, "Auto-launch Jaeger for distributed tracing (default: true)")

	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	// Check if Docker daemon is running
	checkDockerCmd := dockerCommand("info")
	checkDockerCmd.Stdout = nil
	checkDockerCmd.Stderr = nil
	if err := checkDockerCmd.Run(); err != nil {
		return fmt.Errorf("Docker daemon is not running. Please start Docker Desktop and try again")
	}

	// Check if container is already running
	if containerID := getRunningStationContainer(); containerID != "" {
		fmt.Printf("✅ Station server already running (container: %s)\n", containerID[:12])

		// Update workspace mount if requested
		workspace, _ := cmd.Flags().GetString("workspace")
		if workspace != "" {
			fmt.Printf("ℹ️  To change workspace, run 'stn down' first, then 'stn up --workspace %s'\n", workspace)
		}
		return nil
	}

	// Determine workspace directory
	workspace, _ := cmd.Flags().GetString("workspace")
	if workspace == "" {
		var err error
		workspace, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Resolve to absolute path
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return fmt.Errorf("failed to resolve workspace path: %w", err)
	}

	// Check if workspace exists
	if _, err := os.Stat(absWorkspace); os.IsNotExist(err) {
		return fmt.Errorf("workspace directory does not exist: %s", absWorkspace)
	}

	fmt.Printf("📁 Workspace: %s\n", absWorkspace)

	// Image name for all operations
	imageName := "station-server:latest"

	// Check if station-config volume exists
	checkVolumeCmd := dockerCommand("volume", "inspect", "station-config")
	checkVolumeCmd.Stdout = nil
	checkVolumeCmd.Stderr = nil
	volumeExists := checkVolumeCmd.Run() == nil

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
		// Volume exists, check if it contains config
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
		fmt.Printf("🔧 Initializing Station in container volume...\n")

		// Get init parameters from flags or defaults
		provider, _ := cmd.Flags().GetString("provider")
		model, _ := cmd.Flags().GetString("model")
		apiKey, _ := cmd.Flags().GetString("api-key")
		baseURL, _ := cmd.Flags().GetString("base-url")
		ship, _ := cmd.Flags().GetBool("ship")
		useDefaults, _ := cmd.Flags().GetBool("yes")

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

	// Prepare Docker run command
	dockerArgs := []string{"run", "--name", "station-server"}

	// Don't expose port 4033 - genkit start manages the reflection API
	developMode, _ := cmd.Flags().GetBool("develop")

	// Add detach flag if requested (but not in develop mode - needs to stay foreground)
	detach, _ := cmd.Flags().GetBool("detach")
	if developMode {
		// Force foreground mode for genkit start compatibility
		// Use -i (not -it) since genkit start doesn't provide a TTY
		dockerArgs = append(dockerArgs, "-i")
		detach = false // Override detach flag
	} else if detach {
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

	// Port mappings
	dockerArgs = append(dockerArgs,
		"-p", "8586:8586", // MCP
		"-p", "3030:3030", // Dynamic Agent MCP
		"-p", "3002:3002", // MCP Agents
		"-p", "8585:8585", // UI/API
	)

	// Add Genkit Developer UI port if --develop flag is set
	if developMode {
		dockerArgs = append(dockerArgs, "-p", "4000:4000") // Genkit Developer UI
	}

	// Add Jaeger UI port if --jaeger flag is set (default: true)
	enableJaegerPort, _ := cmd.Flags().GetBool("jaeger")
	if enableJaegerPort {
		dockerArgs = append(dockerArgs, "-p", "16686:16686") // Jaeger UI
	}

	// Environment variables
	if err := addEnvironmentVariables(&dockerArgs, cmd); err != nil {
		log.Printf("Warning: Some environment variables may not be set: %v", err)
	}

	// Enable Genkit Developer UI mode if --develop flag is set
	if developMode {
		dockerArgs = append(dockerArgs, "-e", "GENKIT_ENV=dev")
	}

	// Ensure STATION_CONFIG_DIR is set for proper paths
	dockerArgs = append(dockerArgs, "-e", "STATION_CONFIG_DIR=/home/station/.config/station")

	// Enable dev mode to start API server (management UI)
	dockerArgs = append(dockerArgs, "-e", "STN_DEV_MODE=true")

	// Set working directory
	dockerArgs = append(dockerArgs, "-w", "/workspace")

	// Add image and command - use 'genkit start' in develop mode to enable Developer UI
	if developMode {
		environment, _ := cmd.Flags().GetString("environment")
		dockerArgs = append(dockerArgs, imageName, "genkit", "start", "--non-interactive", "--", "stn", "develop", "--env", environment)
	} else {
		// Check if Jaeger should be enabled (default: true)
		enableJaeger, _ := cmd.Flags().GetBool("jaeger")
		if enableJaeger {
			dockerArgs = append(dockerArgs, imageName, "stn", "serve", "--database", "/home/station/.config/station/station.db", "--mcp-port", "8586", "--jaeger")
		} else {
			dockerArgs = append(dockerArgs, imageName, "stn", "serve", "--database", "/home/station/.config/station/station.db", "--mcp-port", "8586")
		}
	}

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

	// Update .mcp.json
	if err := updateMCPConfig(absWorkspace); err != nil {
		log.Printf("Warning: Failed to update .mcp.json: %v", err)
		fmt.Printf("⚠️  Please manually add Station to your .mcp.json:\n")
		fmt.Printf(`  "station": {
    "url": "http://localhost:8586/mcp",
    "transport": "http"
  }` + "\n")
	}

	fmt.Printf("\n✅ Station server started successfully!\n")
	fmt.Printf("🔗 MCP: http://localhost:8586/mcp\n")
	fmt.Printf("🔗 Dynamic Agent MCP: http://localhost:3030/mcp\n")
	fmt.Printf("🔗 UI:  http://localhost:8585\n")
	fmt.Printf("📁 Workspace: %s\n", absWorkspace)

	// Show Jaeger URL if enabled
	jaegerEnabled, _ := cmd.Flags().GetBool("jaeger")
	if jaegerEnabled {
		fmt.Printf("🔍 Jaeger UI: http://localhost:16686 (tracing enabled)\n")
	}

	if developMode {
		environment, _ := cmd.Flags().GetString("environment")
		fmt.Printf("\n🧪 Genkit Developer UI Mode Enabled!\n")
		fmt.Printf("📖 Container is running: genkit start -- stn develop --env %s\n", environment)
		fmt.Printf("🔗 Genkit Developer UI: http://localhost:4000\n")
		fmt.Printf("🔗 Station UI: http://localhost:8585\n")
		fmt.Printf("💡 All agents and MCP tools from '%s' environment available\n", environment)
	}

	if detach {
		fmt.Printf("\n💡 Configuration: Managed via UI at http://localhost:8585/settings\n")
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
	// Check if Dockerfile exists (development mode)
	_, dockerfileErr := os.Stat("Dockerfile")
	hasDockerfile := dockerfileErr == nil

	// Try pulling pre-built image first (production/normal use)
	fmt.Printf("📥 Pulling Station container from registry...\n")
	pullCmd := dockerCommand("pull", "ghcr.io/cloudshipai/station:latest")
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr

	pullErr := pullCmd.Run()
	if pullErr == nil {
		// Successfully pulled, tag for local use
		tagCmd := dockerCommand("tag", "ghcr.io/cloudshipai/station:latest", "station-server:latest")
		if tagErr := tagCmd.Run(); tagErr != nil {
			return fmt.Errorf("failed to tag pulled image: %w", tagErr)
		}
		fmt.Printf("✅ Successfully pulled Station container\n")
		return nil
	}

	// Pull failed, try building if Dockerfile exists (development mode)
	if !hasDockerfile {
		return fmt.Errorf("failed to pull image and no Dockerfile found for local build: %w", pullErr)
	}

	fmt.Printf("⚠️  Pull failed, building from Dockerfile (development mode)...\n")
	buildCmd := dockerCommand("build",
		"--build-arg", "INSTALL_SHIP=true",
		"-t", "station-server:latest",
		".")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}

	fmt.Printf("✅ Successfully built Station container\n")
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

	// CloudShip integration
	cloudshipKeys := []string{
		"STN_CLOUDSHIPAI_KEY",
		"STN_CLOUDSHIPAI_ENDPOINT",
		"CLOUDSHIP_API_KEY",
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
