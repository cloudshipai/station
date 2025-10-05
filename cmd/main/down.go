package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop Station server container",
	Long: `Stop and remove the Station server container.

This command:
- Stops the running Station container gracefully
- Removes the container
- Data (config, environments, agents) persists in Docker volume
- Workspace files are unchanged

Options:
- --remove-volume: Delete ALL Station data (config, environments, agents, bundles)
- --clean-mcp: Remove Station from .mcp.json
- --remove-image: Remove Docker image
- --force: Force kill if graceful stop fails

Examples:
  stn down                        # Stop server (data preserved)
  stn down --remove-volume        # Stop and delete all data
  stn down --clean-mcp            # Stop and remove from .mcp.json
  stn down --remove-image         # Stop and remove Docker image
`,
	RunE: runDown,
}

func init() {
	downCmd.Flags().Bool("clean-mcp", false, "Remove Station from .mcp.json")
	downCmd.Flags().Bool("remove-image", false, "Remove Docker image after stopping")
	downCmd.Flags().Bool("remove-volume", false, "Remove Station data volume (WARNING: deletes container's environments/agents/bundles)")
	downCmd.Flags().Bool("force", false, "Force stop (kill) if graceful stop fails")
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	containerID := getRunningStationContainer()
	if containerID == "" {
		fmt.Printf("ℹ️  Station server is not running\n")
		return nil
	}

	fmt.Printf("🛑 Stopping Station server...\n")

	// Try graceful stop first
	stopCmd := exec.Command("docker", "stop", "station-server")
	stopCmd.Stdout = os.Stdout
	stopCmd.Stderr = os.Stderr

	if err := stopCmd.Run(); err != nil {
		force, _ := cmd.Flags().GetBool("force")
		if force {
			fmt.Printf("⚠️  Graceful stop failed, forcing kill...\n")
			killCmd := exec.Command("docker", "kill", "station-server")
			if err := killCmd.Run(); err != nil {
				return fmt.Errorf("failed to kill container: %w", err)
			}
		} else {
			return fmt.Errorf("failed to stop container: %w (use --force to force kill)", err)
		}
	}

	// Remove the container
	fmt.Printf("🗑️  Removing container...\n")
	rmCmd := exec.Command("docker", "rm", "station-server")
	if err := rmCmd.Run(); err != nil {
		log.Printf("Warning: Failed to remove container: %v", err)
	}

	// Clean up .mcp.json if requested
	cleanMCP, _ := cmd.Flags().GetBool("clean-mcp")
	if cleanMCP {
		if err := removeMCPConfig(); err != nil {
			log.Printf("Warning: Failed to remove Station from .mcp.json: %v", err)
		} else {
			fmt.Printf("✅ Removed Station from .mcp.json\n")
		}
	}

	// Remove image if requested
	removeImage, _ := cmd.Flags().GetBool("remove-image")
	if removeImage {
		fmt.Printf("🗑️  Removing Docker image...\n")
		rmiCmd := exec.Command("docker", "rmi", "station-server:latest")
		if err := rmiCmd.Run(); err != nil {
			log.Printf("Warning: Failed to remove image: %v", err)
		} else {
			fmt.Printf("✅ Removed Docker image\n")
		}
	}

	// Remove volume if requested
	removeVolume, _ := cmd.Flags().GetBool("remove-volume")
	if removeVolume {
		fmt.Printf("🗑️  Removing Station data volume...\n")
		volRmCmd := exec.Command("docker", "volume", "rm", "station-config")
		if err := volRmCmd.Run(); err != nil {
			log.Printf("Warning: Failed to remove volume: %v", err)
		} else {
			fmt.Printf("✅ Removed Station data volume (environments/agents/bundles)\n")
		}
	}

	fmt.Printf("\n✅ Station server stopped successfully\n")
	if !removeVolume {
		fmt.Printf("💡 All data preserved in Docker volume:\n")
		fmt.Printf("   - Configuration (config.yaml)\n")
		fmt.Printf("   - Environments and agents\n")
		fmt.Printf("   - Bundles and database\n")
	} else {
		fmt.Printf("🗑️  All Station data has been deleted\n")
	}
	fmt.Printf("💡 Run 'stn up' to start Station again\n")

	return nil
}

func removeMCPConfig() error {
	// Try current directory first
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	mcpPath := filepath.Join(cwd, ".mcp.json")

	// If not in current dir, check common locations
	if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
		// Try home directory
		homeDir, _ := os.UserHomeDir()
		mcpPath = filepath.Join(homeDir, ".mcp.json")
		if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
			// No .mcp.json to clean up
			return nil
		}
	}

	// Read existing config
	data, err := ioutil.ReadFile(mcpPath)
	if err != nil {
		return fmt.Errorf("failed to read .mcp.json: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse .mcp.json: %w", err)
	}

	// Remove Station from mcpServers
	if mcpServers, ok := config["mcpServers"].(map[string]interface{}); ok {
		delete(mcpServers, "station")

		// Write back the updated config
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		if err := ioutil.WriteFile(mcpPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write .mcp.json: %w", err)
		}
	}

	return nil
}

// Additional commands for container management

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show Station server container logs",
	Long:  `Display logs from the Station server container.`,
	RunE:  runLogs,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Station server status",
	Long:  `Display the current status of the Station server container.`,
	RunE:  runStatus,
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart Station server container",
	Long:  `Restart the Station server container (equivalent to 'stn down' followed by 'stn up').`,
	RunE:  runRestart,
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	logsCmd.Flags().String("tail", "100", "Number of lines to show from the end of the logs")
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(restartCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	containerID := getRunningStationContainer()
	if containerID == "" {
		return fmt.Errorf("Station server is not running")
	}

	logArgs := []string{"logs"}

	follow, _ := cmd.Flags().GetBool("follow")
	if follow {
		logArgs = append(logArgs, "-f")
	}

	tail, _ := cmd.Flags().GetString("tail")
	if tail != "" {
		logArgs = append(logArgs, "--tail", tail)
	}

	logArgs = append(logArgs, "station-server")

	logCmd := exec.Command("docker", logArgs...)
	logCmd.Stdout = os.Stdout
	logCmd.Stderr = os.Stderr

	return logCmd.Run()
}

func runStatus(cmd *cobra.Command, args []string) error {
	containerID := getRunningStationContainer()
	if containerID == "" {
		fmt.Printf("❌ Station server is not running\n")
		return nil
	}

	fmt.Printf("✅ Station server is running\n")
	fmt.Printf("📦 Container ID: %s\n", containerID[:12])

	// Get more details about the container
	inspectCmd := exec.Command("docker", "inspect", "--format",
		`{{.State.Status}}|{{.State.StartedAt}}|{{range $p, $conf := .NetworkSettings.Ports}}{{$p}}->{{(index $conf 0).HostPort}} {{end}}`,
		"station-server")

	output, err := inspectCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) >= 2 {
		fmt.Printf("📊 Status: %s\n", parts[0])
		fmt.Printf("🕐 Started: %s\n", parts[1])
		if len(parts) >= 3 && parts[2] != "" {
			fmt.Printf("🔗 Ports: %s\n", parts[2])
		}
	}

	// Check if .mcp.json has Station configured
	cwd, _ := os.Getwd()
	mcpPath := filepath.Join(cwd, ".mcp.json")
	if data, err := ioutil.ReadFile(mcpPath); err == nil {
		var config map[string]interface{}
		if err := json.Unmarshal(data, &config); err == nil {
			if mcpServers, ok := config["mcpServers"].(map[string]interface{}); ok {
				if _, hasStation := mcpServers["station"]; hasStation {
					fmt.Printf("✅ Station configured in .mcp.json\n")
				} else {
					fmt.Printf("⚠️  Station not configured in .mcp.json (run 'stn up' to fix)\n")
				}
			}
		}
	}

	return nil
}

func runRestart(cmd *cobra.Command, args []string) error {
	fmt.Printf("🔄 Restarting Station server...\n")

	// Stop the container
	if err := runDown(&cobra.Command{}, args); err != nil {
		return fmt.Errorf("failed to stop: %w", err)
	}

	// Start it again
	if err := runUp(&cobra.Command{}, args); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}

	return nil
}