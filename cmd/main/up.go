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

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start Station server in a Docker container",
	Long: `Start a containerized Station server with isolated configuration and workspace access.

This command:
- Builds or uses existing Station runtime container
- Stores all configuration in Docker volume (isolated from host)
- Mounts your workspace directory with correct file permissions
- Automatically configures .mcp.json for Claude integration
- Exposes ports for MCP (3000), Dynamic MCP (3001), UI (8585)

Key Features:
- Config management via UI (restart container to apply changes)
- Workspace files maintain your user ownership (no root permission issues)
- Data persists across container restarts in Docker volume

Examples:
  # Basic usage
  stn up                                    # Start with current directory as workspace
  stn up --workspace ~/code                 # Use specific directory as workspace

  # First-time setup with configuration
  stn up --provider openai --ship           # Init with OpenAI and ship CLI
  stn up --provider gemini --model gemini-2.0-flash-exp --yes
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

	// Init flags for first-time setup
	upCmd.Flags().String("provider", "", "AI provider for initialization (openai, gemini, custom)")
	upCmd.Flags().String("model", "", "AI model to use (e.g., gpt-4o-mini, gemini-2.0-flash-exp)")
	upCmd.Flags().String("base-url", "", "Custom base URL for OpenAI-compatible endpoints")
	upCmd.Flags().Bool("ship", false, "Bootstrap with ship CLI MCP integration for filesystem access")
	upCmd.Flags().BoolP("yes", "y", false, "Use defaults without interactive prompts")

	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
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

	// Get config path early for initialization
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "station")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("Station config not found at %s. Run 'stn init' first", configPath)
	}

	// Image name for all operations
	imageName := "station-server:latest"

	// Check if station-config volume exists
	checkVolumeCmd := exec.Command("docker", "volume", "inspect", "station-config")
	checkVolumeCmd.Stdout = nil
	checkVolumeCmd.Stderr = nil
	volumeExists := checkVolumeCmd.Run() == nil

	needsInit := false
	if !volumeExists {
		fmt.Printf("📦 Creating Station data volume (first run)...\n")
		createVolumeCmd := exec.Command("docker", "volume", "create", "station-config")
		if err := createVolumeCmd.Run(); err != nil {
			return fmt.Errorf("failed to create station-config volume: %w", err)
		}
		fmt.Printf("✅ Created Station data volume\n")
		needsInit = true
	} else {
		// Volume exists, check if it contains config
		checkConfigCmd := exec.Command("docker", "run", "--rm",
			"-v", "station-config:/root/.config/station",
			imageName,
			"test", "-f", "/root/.config/station/config.yaml")
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
		baseURL, _ := cmd.Flags().GetString("base-url")
		ship, _ := cmd.Flags().GetBool("ship")
		useDefaults, _ := cmd.Flags().GetBool("yes")

		// Default to openai if not specified
		if provider == "" {
			provider = "openai"
			// Try to read from host config if it exists
			if cfg, err := readConfigFile(configPath + "/config.yaml"); err == nil {
				if p, ok := cfg["ai_provider"].(string); ok {
					provider = p
				}
			}
		}

		// Run init in a temporary container
		// Note: Running as root for volume initialization simplicity
		initArgs := []string{
			"run", "--rm",
		}

		// Pass through AI provider env vars for init
		aiEnvVars := []string{
			"OPENAI_API_KEY",
			"ANTHROPIC_API_KEY",
			"GOOGLE_API_KEY",
			"GEMINI_API_KEY",
		}
		for _, envVar := range aiEnvVars {
			if value := os.Getenv(envVar); value != "" {
				initArgs = append(initArgs, "-e", fmt.Sprintf("%s=%s", envVar, value))
			}
		}

		initArgs = append(initArgs,
			"-v", "station-config:/root/.config/station",
			"-e", "HOME=/root",
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

		initCmd := exec.Command("docker", initArgs...)
		initCmd.Stdout = os.Stdout
		initCmd.Stderr = os.Stderr

		if err := initCmd.Run(); err != nil {
			return fmt.Errorf("failed to initialize Station in container: %w", err)
		}

		fmt.Printf("✅ Station initialized in container volume\n")
	}

	// Prepare Docker run command
	dockerArgs := []string{"run", "--name", "station-server"}

	// Add detach flag if requested
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

		// Fix volume permissions: chown the volume to match host user
		// This is necessary because init runs as root but serve runs as user
		fmt.Printf("🔧 Fixing volume permissions for user %d:%d...\n", uid, gid)
		chownArgs := []string{
			"run", "--rm",
			"-v", "station-config:/root/.config/station",
			imageName,
			"sh", "-c", fmt.Sprintf("chown -R %d:%d /root/.config/station", uid, gid),
		}
		chownCmd := exec.Command("docker", chownArgs...)
		if err := chownCmd.Run(); err != nil {
			log.Printf("Warning: Could not fix volume permissions: %v", err)
		}
	}
	// macOS and Windows: Docker Desktop handles permissions automatically

	// Volume mounts
	dockerArgs = append(dockerArgs,
		"-v", fmt.Sprintf("%s:/workspace:rw", absWorkspace),
		"-v", "station-config:/root/.config/station:rw",  // All Station data in volume (including config.yaml)
	)

	// Pass the host workspace path so container knows the mapping
	dockerArgs = append(dockerArgs,
		"-e", fmt.Sprintf("HOST_WORKSPACE=%s", absWorkspace),
	)

	// Docker socket mount for Dagger (if exists)
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		dockerArgs = append(dockerArgs, "-v", "/var/run/docker.sock:/var/run/docker.sock")
	}

	// Named volume for Dagger cache (persists across container restarts)
	dockerArgs = append(dockerArgs, "-v", "station-dagger-cache:/root/.cache")

	// Port mappings
	dockerArgs = append(dockerArgs,
		"-p", "3000:3000",  // MCP
		"-p", "3001:3001",  // Dynamic Agent MCP
		"-p", "3002:3002",  // MCP Agents
		"-p", "8585:8585",  // UI/API
	)

	// Environment variables
	if err := addEnvironmentVariables(&dockerArgs, cmd); err != nil {
		log.Printf("Warning: Some environment variables may not be set: %v", err)
	}

	// Set working directory
	dockerArgs = append(dockerArgs, "-w", "/workspace")

	// Add image and command with database path override
	dockerArgs = append(dockerArgs, imageName, "stn", "serve", "--database", "/root/.config/station/station.db")

	// Run the container
	fmt.Printf("🐳 Starting container...\n")
	dockerCmd := exec.Command("docker", dockerArgs...)
	dockerCmd.Stdout = os.Stdout
	dockerCmd.Stderr = os.Stderr

	if err := dockerCmd.Run(); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Update .mcp.json
	if err := updateMCPConfig(absWorkspace); err != nil {
		log.Printf("Warning: Failed to update .mcp.json: %v", err)
		fmt.Printf("⚠️  Please manually add Station to your .mcp.json:\n")
		fmt.Printf(`  "station": {
    "url": "http://localhost:3000/sse",
    "transport": "sse"
  }`+"\n")
	}

	fmt.Printf("\n✅ Station server started successfully!\n")
	fmt.Printf("🔗 MCP: http://localhost:3000/mcp\n")
	fmt.Printf("🔗 Dynamic Agent MCP: http://localhost:3001/mcp\n")
	fmt.Printf("🔗 UI:  http://localhost:8585\n")
	fmt.Printf("📁 Workspace: %s\n", absWorkspace)

	if detach {
		fmt.Printf("\n💡 Configuration: Managed via UI at http://localhost:8585/settings\n")
		fmt.Printf("💡 Run 'stn logs' to see container output\n")
		fmt.Printf("💡 Run 'stn down' to stop (data preserved in volume)\n")
		fmt.Printf("💡 Run 'stn down --remove-volume' to delete all data\n")
	}

	return nil
}

func getRunningStationContainer() string {
	cmd := exec.Command("docker", "ps", "-q", "-f", "name=station-server")
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
	cmd := exec.Command("docker", "images", "-q", imageName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

func buildRuntimeContainer() error {
	fmt.Printf("Building Station container (this may take a few minutes)...\n")

	// Build the Docker image
	buildCmd := exec.Command("docker", "build",
		"--build-arg", "INSTALL_SHIP=true",
		"-t", "station-server:latest",
		".")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		// Fallback to pulling pre-built image if available
		fmt.Printf("Build failed, attempting to pull pre-built image...\n")
		pullCmd := exec.Command("docker", "pull", "ghcr.io/cloudshipai/station:latest")
		if pullErr := pullCmd.Run(); pullErr != nil {
			return fmt.Errorf("failed to build or pull image: build error: %w, pull error: %v", err, pullErr)
		}

		// Tag the pulled image for local use
		tagCmd := exec.Command("docker", "tag", "ghcr.io/cloudshipai/station:latest", "station-server:latest")
		return tagCmd.Run()
	}

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

	// Essential AI provider keys to pass through
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
	*dockerArgs = append(*dockerArgs, "-e", "HOME=/root")

	// Set PATH to include common tool locations
	*dockerArgs = append(*dockerArgs, "-e", "PATH=/root/.local/bin:/usr/local/bin:/usr/bin:/bin")

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
		"url":  "http://localhost:3000/mcp",
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

