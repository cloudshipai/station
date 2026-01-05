package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// OpenCodeDockerCompose is the default docker-compose configuration for OpenCode
const OpenCodeDockerCompose = `# Station OpenCode Sandbox
# AI coding assistant with Station NATS plugin pre-installed
#
# Usage:
#   stn opencode up    - Start OpenCode
#   stn opencode down  - Stop OpenCode
#
# The container mounts your host OpenCode config and auth for seamless authentication.
# If you've authenticated with 'opencode auth', it will just work.

services:
  opencode:
    image: ghcr.io/cloudshipai/opencode-station:latest
    container_name: station-opencode
    restart: unless-stopped
    entrypoint: ["opencode", "serve", "--hostname", "0.0.0.0", "--port", "4096"]
    ports:
      - "${OPENCODE_PORT:-4099}:4096"     # OpenCode API (default 4099 to avoid conflict with native)
    volumes:
      # Mount host auth for seamless Anthropic OAuth authentication
      - ${HOME}/.local/share/opencode/auth.json:/root/.local/share/opencode/auth.json:ro
      # Note: We don't mount opencode.json since config schemas differ between versions.
      # Use environment variables (OPENCODE_*) to configure instead.
      # Persistent workspace storage
      - station-opencode-workspaces:/workspaces
    environment:
      # Optional: Connect to NATS for Station orchestration
      - NATS_URL=${NATS_URL:-}
      # Auto-approve tool executions (sandbox mode)
      - OPENCODE_AUTO_APPROVE=true
    extra_hosts:
      # Allow container to reach host services (NATS, etc.)
      - "host.docker.internal:host-gateway"
    healthcheck:
      test: ["CMD", "curl", "-sf", "http://localhost:4096/health"]
      interval: 30s
      timeout: 10s
      start_period: 10s
      retries: 3

volumes:
  station-opencode-workspaces:
`

// GetOpenCodeComposePath returns the path to the opencode docker-compose file
func GetOpenCodeComposePath() (string, error) {
	configDir := os.Getenv("STATION_CONFIG_DIR")
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir = filepath.Join(homeDir, ".config", "station")
	}
	return filepath.Join(configDir, "docker-compose.opencode.yml"), nil
}

// EnsureOpenCodeComposeFile creates the docker-compose file if it doesn't exist
func EnsureOpenCodeComposeFile() (string, error) {
	composePath, err := GetOpenCodeComposePath()
	if err != nil {
		return "", err
	}

	// Check if file exists
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		// Create the file
		if err := os.MkdirAll(filepath.Dir(composePath), 0755); err != nil {
			return "", fmt.Errorf("failed to create config directory: %w", err)
		}
		if err := os.WriteFile(composePath, []byte(OpenCodeDockerCompose), 0644); err != nil {
			return "", fmt.Errorf("failed to write docker-compose file: %w", err)
		}
	}

	return composePath, nil
}

// NewOpenCodeCmd creates the opencode command group
func NewOpenCodeCmd() *cobra.Command {
	opencodeCmd := &cobra.Command{
		Use:   "opencode",
		Short: "Manage OpenCode AI coding sandbox",
		Long: `Manage the OpenCode AI coding assistant container for Station.

OpenCode provides a sandboxed AI coding environment with:
  - Full filesystem access within /workspaces
  - Git operations (clone, commit, push)
  - Code execution and testing
  - Station NATS plugin for orchestration

Your host OpenCode authentication is automatically mounted,
so if you've run 'opencode auth', it will just work.

Examples:
  stn opencode up      # Start OpenCode sandbox
  stn opencode down    # Stop OpenCode
  stn opencode status  # Check if OpenCode is running
  stn opencode logs    # View OpenCode logs
  stn opencode clean   # Remove all data and start fresh`,
	}

	opencodeCmd.AddCommand(newOpenCodeUpCmd())
	opencodeCmd.AddCommand(newOpenCodeDownCmd())
	opencodeCmd.AddCommand(newOpenCodeStatusCmd())
	opencodeCmd.AddCommand(newOpenCodeCleanCmd())
	opencodeCmd.AddCommand(newOpenCodeLogsCmd())

	return opencodeCmd
}

func newOpenCodeUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Start OpenCode AI coding sandbox",
		Long: `Start the OpenCode AI coding assistant using Docker Compose.

OpenCode will be available at:
  - API: http://localhost:4099 (or OPENCODE_PORT if set)

The container mounts your host OpenCode config and authentication,
so Anthropic OAuth tokens are automatically available.

Workspaces are persisted in the 'station-opencode-workspaces' Docker volume.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if host auth.json exists
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			authPath := filepath.Join(homeDir, ".local", "share", "opencode", "auth.json")
			if _, err := os.Stat(authPath); os.IsNotExist(err) {
				fmt.Printf("⚠️  Warning: No OpenCode auth found at %s\n", authPath)
				fmt.Printf("   Run 'opencode auth' first to authenticate with Anthropic.\n")
				fmt.Printf("   Or set ANTHROPIC_API_KEY environment variable.\n\n")
			}

			composePath, err := EnsureOpenCodeComposeFile()
			if err != nil {
				return fmt.Errorf("failed to ensure docker-compose file: %w", err)
			}

			fmt.Printf("🚀 Starting OpenCode sandbox...\n")
			fmt.Printf("   Config: %s\n", composePath)

			// Run docker compose up -d
			dockerCmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d")
			dockerCmd.Stdout = os.Stdout
			dockerCmd.Stderr = os.Stderr

			if err := dockerCmd.Run(); err != nil {
				return fmt.Errorf("failed to start OpenCode: %w", err)
			}

			port := os.Getenv("OPENCODE_PORT")
			if port == "" {
				port = "4099"
			}

			fmt.Printf("\n✅ OpenCode started!\n")
			fmt.Printf("   🔗 API: http://localhost:%s\n", port)
			fmt.Printf("   📁 Workspaces: /workspaces (inside container)\n")
			fmt.Printf("\n💡 Usage:\n")
			fmt.Printf("   Configure Station coding backend:\n")
			fmt.Printf("     coding:\n")
			fmt.Printf("       backend: opencode\n")
			fmt.Printf("       opencode:\n")
			fmt.Printf("         url: http://localhost:%s\n", port)
			fmt.Printf("\n   Or for NATS-based orchestration:\n")
			fmt.Printf("     Start NATS first, then: NATS_URL=nats://localhost:4222 stn opencode up\n")

			return nil
		},
	}
}

func newOpenCodeDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop OpenCode sandbox",
		Long:  `Stop the OpenCode container. Workspace data is preserved in the Docker volume.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			composePath, err := GetOpenCodeComposePath()
			if err != nil {
				return err
			}

			// Check if compose file exists
			if _, err := os.Stat(composePath); os.IsNotExist(err) {
				fmt.Printf("ℹ️  OpenCode not configured (no docker-compose file)\n")
				return nil
			}

			fmt.Printf("🛑 Stopping OpenCode...\n")

			dockerCmd := exec.Command("docker", "compose", "-f", composePath, "down")
			dockerCmd.Stdout = os.Stdout
			dockerCmd.Stderr = os.Stderr

			if err := dockerCmd.Run(); err != nil {
				return fmt.Errorf("failed to stop OpenCode: %w", err)
			}

			fmt.Printf("✅ OpenCode stopped\n")
			fmt.Printf("💡 Workspace data preserved in 'station-opencode-workspaces' volume\n")
			fmt.Printf("   Run 'stn opencode clean' to remove all data\n")

			return nil
		},
	}
}

func newOpenCodeStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check OpenCode status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if container is running
			checkCmd := exec.Command("docker", "ps", "--filter", "name=station-opencode", "--format", "{{.Status}}")
			output, err := checkCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to check OpenCode status: %w", err)
			}

			port := os.Getenv("OPENCODE_PORT")
			if port == "" {
				port = "4099"
			}

			status := strings.TrimSpace(string(output))
			if status == "" {
				fmt.Printf("⚫ OpenCode is not running\n")
				fmt.Printf("   Run 'stn opencode up' to start\n")
			} else {
				fmt.Printf("🟢 OpenCode is running\n")
				fmt.Printf("   Status: %s\n", status)
				fmt.Printf("   🔗 API: http://localhost:%s\n", port)

				healthCmd := exec.Command("curl", "-sf", fmt.Sprintf("http://localhost:%s/health", port))
				if healthErr := healthCmd.Run(); healthErr == nil {
					fmt.Printf("   ❤️  Health: OK\n")
				} else {
					fmt.Printf("   ⚠️  Health: Not responding (may still be starting)\n")
				}
			}

			return nil
		},
	}
}

func newOpenCodeCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Remove all OpenCode data and start fresh",
		Long:  `Stops OpenCode and removes the workspace volume. All coding data will be lost.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			composePath, err := GetOpenCodeComposePath()
			if err != nil {
				return err
			}

			fmt.Printf("🧹 Cleaning OpenCode data...\n")

			// Stop and remove containers
			if _, err := os.Stat(composePath); err == nil {
				fmt.Printf("   🛑 Stopping OpenCode...\n")
				downCmd := exec.Command("docker", "compose", "-f", composePath, "down", "-v")
				downCmd.Stdout = os.Stdout
				downCmd.Stderr = os.Stderr
				_ = downCmd.Run() // Ignore errors if not running
			}

			// Also try to remove the volume directly (in case compose file doesn't exist)
			fmt.Printf("   🗑️  Removing workspace volume...\n")
			rmVolCmd := exec.Command("docker", "volume", "rm", "station-opencode-workspaces")
			output, err := rmVolCmd.CombinedOutput()
			if err != nil {
				if strings.Contains(string(output), "No such volume") {
					fmt.Printf("   ℹ️  Volume already removed\n")
				} else if strings.Contains(string(output), "volume is in use") {
					return fmt.Errorf("volume is in use - stop OpenCode first")
				}
				// Ignore other errors
			} else {
				fmt.Printf("   ✅ Removed workspace volume\n")
			}

			fmt.Printf("\n✨ OpenCode cleaned!\n")
			fmt.Printf("   Run 'stn opencode up' to start fresh\n")

			return nil
		},
	}
}

func newOpenCodeLogsCmd() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View OpenCode logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerArgs := []string{"logs", "station-opencode"}
			if follow {
				dockerArgs = append(dockerArgs, "-f")
			}

			dockerCmd := exec.Command("docker", dockerArgs...)
			dockerCmd.Stdout = os.Stdout
			dockerCmd.Stderr = os.Stderr

			return dockerCmd.Run()
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")

	return cmd
}
