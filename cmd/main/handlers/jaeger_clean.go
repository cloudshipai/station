package handlers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"github.com/spf13/cobra"
	"station/internal/logging"
)

// NewJaegerCleanCmd creates the clean-jaeger command
func NewJaegerCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean-jaeger",
		Short: "Clean Jaeger telemetry data",
		Long:  "Removes Jaeger cache volume and local data directory to start fresh",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			logging.Info("🧹 Cleaning Jaeger data...")

			// Clean Dagger cache volume (this is where the actual data lives)
			client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
			if err != nil {
				logging.Error("Failed to connect to Dagger: %v", err)
			} else {
				defer client.Close()
			}

			// Try to remove the Docker volume directly
			logging.Info("   🗑️  Removing Dagger cache volume 'station-dagger-cache'...")
			
			// Check if volume exists
			checkCmd := exec.Command("docker", "volume", "inspect", "station-dagger-cache")
			if err := checkCmd.Run(); err == nil {
				// Volume exists, try to remove it
				rmCmd := exec.Command("docker", "volume", "rm", "station-dagger-cache")
				output, err := rmCmd.CombinedOutput()
				if err != nil {
					// If removal fails, it's likely because Jaeger is running
					if strings.Contains(string(output), "in use") || strings.Contains(string(output), "is in use") {
						logging.Info("   ⚠️  Volume is in use (Jaeger may be running)")
						logging.Info("   💡 Stop Jaeger first with: docker stop jaeger")
						logging.Info("   💡 Or restart OpenCode to disconnect Station MCP")
					} else {
						logging.Error("   ❌ Failed to remove volume: %v", err)
					}
				} else {
					logging.Info("   ✅ Removed Dagger cache volume")
				}
			} else {
				logging.Info("   ℹ️  No Dagger cache volume found")
			}

			// Clean local data directory (probably doesn't exist but check anyway)
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			dataDir := filepath.Join(homeDir, ".local", "share", "station", "jaeger-data")
			if _, err := os.Stat(dataDir); err == nil {
				if err := os.RemoveAll(dataDir); err != nil {
					return fmt.Errorf("failed to remove Jaeger data directory: %w", err)
				}
				logging.Info("   ✅ Removed local Jaeger data: %s", dataDir)
			} else {
				logging.Info("   ℹ️  No local Jaeger data directory found")
			}

			logging.Info("✨ Jaeger data cleaned!")
			logging.Info("   📝 Jaeger will start fresh on next `stn stdio` or `stn up`")

			return nil
		},
	}
}
