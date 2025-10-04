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
	"syscall"

	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start Station server in a Docker container",
	Long: `Start a containerized Station server that mounts your local configuration and workspace.

This command:
- Builds or uses existing Station runtime container
- Mounts your local Station configuration (~/.config/station)
- Mounts your current or specified workspace directory
- Preserves file permissions using your user ID
- Automatically configures .mcp.json for Claude integration
- Exposes ports for API (3000), SSH (2222), MCP (3001), and UI (8585)

Examples:
  stn up                     # Start with current directory as workspace
  stn up --workspace ~/      # Use home directory as workspace
  stn up --workspace /project # Use specific directory as workspace
  stn up --detach           # Run in background
`,
	RunE: runUp,
}

func init() {
	upCmd.Flags().StringP("workspace", "w", "", "Workspace directory to mount (default: current directory)")
	upCmd.Flags().BoolP("detach", "d", true, "Run container in background")
	upCmd.Flags().Bool("upgrade", false, "Rebuild container image before starting")
	upCmd.Flags().StringSlice("env", []string{}, "Additional environment variables to pass through")
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

	fmt.Printf("🚀 Starting Station server...\n")
	fmt.Printf("📁 Workspace: %s\n", absWorkspace)

	// Build or check for image
	imageName := "station-runtime:latest"
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

	// User mapping for file permissions (cross-platform)
	if err := addUserMapping(&dockerArgs); err != nil {
		log.Printf("Warning: Could not set user mapping: %v", err)
		// Continue anyway - some platforms don't support this
	}

	// Volume mounts
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "station")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("Station config not found at %s. Run 'stn init' first", configPath)
	}

	// Core volume mounts
	dockerArgs = append(dockerArgs,
		"-v", fmt.Sprintf("%s:/workspace:rw", absWorkspace),
		"-v", fmt.Sprintf("%s:/root/.config/station:rw", configPath),
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

	if detach {
		fmt.Printf("\n💡 Run 'stn logs' to see container output\n")
		fmt.Printf("💡 Run 'stn down' to stop the server\n")
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

func dockerImageExists(imageName string) bool {
	cmd := exec.Command("docker", "images", "-q", imageName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

func buildRuntimeContainer() error {
	fmt.Printf("Building runtime container (this may take a few minutes)...\n")

	// Use the new runtime build command
	buildCmd := exec.Command("stn", "build", "runtime")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		// Fallback to pulling pre-built image if available
		fmt.Printf("Build failed, attempting to pull pre-built image...\n")
		pullCmd := exec.Command("docker", "pull", "ghcr.io/cloudshipai/station-runtime:latest")
		if pullErr := pullCmd.Run(); pullErr != nil {
			return fmt.Errorf("failed to build or pull image: build error: %w, pull error: %v", err, pullErr)
		}

		// Tag the pulled image for local use
		tagCmd := exec.Command("docker", "tag", "ghcr.io/cloudshipai/station-runtime:latest", "station-runtime:latest")
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
			if sysStat, ok := stat.Sys().(*syscall.Stat_t); ok {
				dockerGID = int(sysStat.Gid)
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
	// Read encryption key from local config file
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "station", "config.yaml")
	if data, err := ioutil.ReadFile(configPath); err == nil {
		// Simple key extraction from YAML (encryption_key: value)
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "encryption_key:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					encryptionKey := strings.TrimSpace(parts[1])
					*dockerArgs = append(*dockerArgs, "-e", fmt.Sprintf("STATION_ENCRYPTION_KEY=%s", encryptionKey))
					break
				}
			}
		}
	}

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

	// Override config path to use container-local path (not host-specific)
	*dockerArgs = append(*dockerArgs, "-e", "STATION_CONFIG=/root/.config/station/config.yaml")

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