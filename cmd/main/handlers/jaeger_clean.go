package handlers

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"station/internal/logging"
)

// NewJaegerCleanCmd creates the clean-jaeger command
func NewJaegerCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean-jaeger",
		Short: "Clean Jaeger telemetry data",
		Long:  "Stops Jaeger container and removes its data volume to start fresh",
		RunE: func(cmd *cobra.Command, args []string) error {
			logging.Info("🧹 Cleaning Jaeger data...")

			containerName := "station-jaeger"
			volumeName := "jaeger-badger-data"

			// Stop Jaeger container if running
			logging.Info("   🛑 Stopping Jaeger container...")
			stopCmd := exec.Command("docker", "stop", containerName)
			if output, err := stopCmd.CombinedOutput(); err != nil {
				if strings.Contains(string(output), "No such container") {
					logging.Info("   ℹ️  Jaeger container not running")
				} else {
					logging.Error("   ⚠️  Failed to stop container: %v", err)
				}
			} else {
				logging.Info("   ✅ Stopped Jaeger container")
			}

			// Remove Jaeger container
			logging.Info("   🗑️  Removing Jaeger container...")
			rmCmd := exec.Command("docker", "rm", containerName)
			if output, err := rmCmd.CombinedOutput(); err != nil {
				if strings.Contains(string(output), "No such container") {
					logging.Info("   ℹ️  Jaeger container already removed")
				} else {
					logging.Error("   ⚠️  Failed to remove container: %v", err)
				}
			} else {
				logging.Info("   ✅ Removed Jaeger container")
			}

			// Remove Jaeger data volume
			logging.Info("   🗑️  Removing Jaeger data volume...")
			rmVolCmd := exec.Command("docker", "volume", "rm", volumeName)
			output, err := rmVolCmd.CombinedOutput()
			if err != nil {
				if strings.Contains(string(output), "no such volume") {
					logging.Info("   ℹ️  Jaeger volume already removed")
				} else if strings.Contains(string(output), "volume is in use") {
					logging.Error("   ❌ Volume is in use - please stop Station first")
					return fmt.Errorf("Jaeger volume is in use")
				} else {
					logging.Error("   ❌ Failed to remove volume: %v\n%s", err, string(output))
					return fmt.Errorf("failed to remove Jaeger volume: %w", err)
				}
			} else {
				logging.Info("   ✅ Removed Jaeger data volume: %s", volumeName)
			}

			logging.Info("✨ Jaeger cleaned!")
			logging.Info("   📝 Next time you run Station with Jaeger, it will start fresh")
			logging.Info("")
			logging.Info("   💡 Jaeger startup:")
			logging.Info("      • stn stdio → Enabled by default (use --jaeger=false to disable)")
			logging.Info("      • stn serve → Disabled by default (use --jaeger to enable)")

			return nil
		},
	}
}
