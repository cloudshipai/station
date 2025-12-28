package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// JaegerDockerCompose is the default docker-compose configuration for Jaeger
const JaegerDockerCompose = `# Station Jaeger Configuration
# Provides OpenTelemetry tracing with in-memory storage
# 
# Usage:
#   stn jaeger up    - Start Jaeger
#   stn jaeger down  - Stop Jaeger

services:
  jaeger:
    image: jaegertracing/all-in-one:latest
    container_name: station-jaeger
    restart: unless-stopped
    ports:
      - "16686:16686"   # Jaeger UI
      - "4317:4317"     # OTLP gRPC
      - "4318:4318"     # OTLP HTTP
      - "14268:14268"   # Jaeger thrift HTTP
    environment:
      - SPAN_STORAGE_TYPE=memory
`

// GetJaegerComposePath returns the path to the jaeger docker-compose file
func GetJaegerComposePath() (string, error) {
	configDir := os.Getenv("STATION_CONFIG_DIR")
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir = filepath.Join(homeDir, ".config", "station")
	}
	return filepath.Join(configDir, "docker-compose.jaeger.yml"), nil
}

// EnsureJaegerComposeFile creates the docker-compose file if it doesn't exist
func EnsureJaegerComposeFile() (string, error) {
	composePath, err := GetJaegerComposePath()
	if err != nil {
		return "", err
	}

	// Check if file exists
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		// Create the file
		if err := os.MkdirAll(filepath.Dir(composePath), 0755); err != nil {
			return "", fmt.Errorf("failed to create config directory: %w", err)
		}
		if err := os.WriteFile(composePath, []byte(JaegerDockerCompose), 0644); err != nil {
			return "", fmt.Errorf("failed to write docker-compose file: %w", err)
		}
	}

	return composePath, nil
}

// NewJaegerCmd creates the jaeger command group
func NewJaegerCmd() *cobra.Command {
	jaegerCmd := &cobra.Command{
		Use:   "jaeger",
		Short: "Manage Jaeger telemetry service",
		Long: `Manage the Jaeger OpenTelemetry collector for distributed tracing.

Jaeger provides trace visualization and analysis for Station agent executions.
Data is persisted in a Docker volume across restarts.

Examples:
  stn jaeger up      # Start Jaeger
  stn jaeger down    # Stop Jaeger  
  stn jaeger status  # Check if Jaeger is running
  stn jaeger clean   # Remove all data and start fresh
  stn jaeger logs    # View Jaeger logs`,
	}

	jaegerCmd.AddCommand(newJaegerUpCmd())
	jaegerCmd.AddCommand(newJaegerDownCmd())
	jaegerCmd.AddCommand(newJaegerStatusCmd())
	jaegerCmd.AddCommand(newJaegerCleanCmd())
	jaegerCmd.AddCommand(newJaegerLogsCmd())

	return jaegerCmd
}

func newJaegerUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Start Jaeger telemetry service",
		Long: `Start the Jaeger OpenTelemetry collector using Docker Compose.

Jaeger will be available at:
  - UI: http://localhost:16686
  - OTLP HTTP: http://localhost:4318
  - OTLP gRPC: localhost:4317

Traces are persisted in the 'station-jaeger-data' Docker volume.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			composePath, err := EnsureJaegerComposeFile()
			if err != nil {
				return fmt.Errorf("failed to ensure docker-compose file: %w", err)
			}

			fmt.Printf("🚀 Starting Jaeger...\n")
			fmt.Printf("   Config: %s\n", composePath)

			// Run docker compose up -d
			dockerCmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d")
			dockerCmd.Stdout = os.Stdout
			dockerCmd.Stderr = os.Stderr

			if err := dockerCmd.Run(); err != nil {
				return fmt.Errorf("failed to start Jaeger: %w", err)
			}

			fmt.Printf("\n✅ Jaeger started!\n")
			fmt.Printf("   🔍 UI: http://localhost:16686\n")
			fmt.Printf("   📡 OTLP HTTP: http://localhost:4318\n")
			fmt.Printf("   📡 OTLP gRPC: localhost:4317\n")
			fmt.Printf("\n💡 Configure Station to use Jaeger:\n")
			fmt.Printf("   Set otel_endpoint: http://localhost:4318 in config.yaml\n")
			fmt.Printf("   Or for Docker: otel_endpoint: http://host.docker.internal:4318\n")

			return nil
		},
	}
}

func newJaegerDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop Jaeger telemetry service",
		Long:  `Stop the Jaeger container. Data is preserved in the Docker volume.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			composePath, err := GetJaegerComposePath()
			if err != nil {
				return err
			}

			// Check if compose file exists
			if _, err := os.Stat(composePath); os.IsNotExist(err) {
				fmt.Printf("ℹ️  Jaeger not configured (no docker-compose file)\n")
				return nil
			}

			fmt.Printf("🛑 Stopping Jaeger...\n")

			dockerCmd := exec.Command("docker", "compose", "-f", composePath, "down")
			dockerCmd.Stdout = os.Stdout
			dockerCmd.Stderr = os.Stderr

			if err := dockerCmd.Run(); err != nil {
				return fmt.Errorf("failed to stop Jaeger: %w", err)
			}

			fmt.Printf("✅ Jaeger stopped\n")
			fmt.Printf("💡 Data preserved in 'station-jaeger-data' volume\n")
			fmt.Printf("   Run 'stn jaeger clean' to remove all data\n")

			return nil
		},
	}
}

func newJaegerStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check Jaeger status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if container is running
			checkCmd := exec.Command("docker", "ps", "--filter", "name=station-jaeger", "--format", "{{.Status}}")
			output, err := checkCmd.Output()
			if err != nil {
				return fmt.Errorf("failed to check Jaeger status: %w", err)
			}

			status := strings.TrimSpace(string(output))
			if status == "" {
				fmt.Printf("⚫ Jaeger is not running\n")
				fmt.Printf("   Run 'stn jaeger up' to start\n")
			} else {
				fmt.Printf("🟢 Jaeger is running\n")
				fmt.Printf("   Status: %s\n", status)
				fmt.Printf("   🔍 UI: http://localhost:16686\n")
				fmt.Printf("   📡 OTLP: http://localhost:4318\n")
			}

			return nil
		},
	}
}

func newJaegerCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Remove all Jaeger data and start fresh",
		Long:  `Stops Jaeger and removes the data volume. All trace data will be lost.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			composePath, err := GetJaegerComposePath()
			if err != nil {
				return err
			}

			fmt.Printf("🧹 Cleaning Jaeger data...\n")

			// Stop and remove containers
			if _, err := os.Stat(composePath); err == nil {
				fmt.Printf("   🛑 Stopping Jaeger...\n")
				downCmd := exec.Command("docker", "compose", "-f", composePath, "down", "-v")
				downCmd.Stdout = os.Stdout
				downCmd.Stderr = os.Stderr
				_ = downCmd.Run() // Ignore errors if not running
			}

			// Also try to remove the volume directly (in case compose file doesn't exist)
			fmt.Printf("   🗑️  Removing data volume...\n")
			rmVolCmd := exec.Command("docker", "volume", "rm", "station-jaeger-data")
			output, err := rmVolCmd.CombinedOutput()
			if err != nil {
				if strings.Contains(string(output), "No such volume") {
					fmt.Printf("   ℹ️  Volume already removed\n")
				} else if strings.Contains(string(output), "volume is in use") {
					return fmt.Errorf("volume is in use - stop Jaeger first")
				}
				// Ignore other errors
			} else {
				fmt.Printf("   ✅ Removed data volume\n")
			}

			fmt.Printf("\n✨ Jaeger cleaned!\n")
			fmt.Printf("   Run 'stn jaeger up' to start fresh\n")

			return nil
		},
	}
}

func newJaegerLogsCmd() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View Jaeger logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			dockerArgs := []string{"logs", "station-jaeger"}
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
