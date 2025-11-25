package services

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"station/internal/logging"
)

// JaegerService manages Jaeger all-in-one as a Docker container
type JaegerService struct {
	containerName string
	volumeName    string
	uiPort        int
	otlpPort      int
	isRunning     bool
}

// JaegerConfig holds Jaeger service configuration
type JaegerConfig struct {
	UIPort   int // Default: 16686
	OTLPPort int // Default: 4318
}

// NewJaegerService creates a new Jaeger service manager
func NewJaegerService(cfg *JaegerConfig) *JaegerService {
	// Set defaults
	if cfg.UIPort == 0 {
		cfg.UIPort = 16686
	}
	if cfg.OTLPPort == 0 {
		cfg.OTLPPort = 4318
	}

	return &JaegerService{
		containerName: "station-jaeger",
		volumeName:    "jaeger-badger-data",
		uiPort:        cfg.UIPort,
		otlpPort:      cfg.OTLPPort,
	}
}

// Start launches Jaeger as a Docker container
func (j *JaegerService) Start(ctx context.Context) error {
	// Check if Jaeger already running
	if j.isAlreadyRunning() {
		logging.Info("🔍 Jaeger already running on port %d - reusing existing instance", j.uiPort)
		j.isRunning = true
		return nil
	}

	logging.Info("🔍 Launching Jaeger (Docker container)...")

	// Create volume if it doesn't exist
	createVolumeCmd := exec.Command("docker", "volume", "create", j.volumeName)
	if output, err := createVolumeCmd.CombinedOutput(); err != nil {
		// Volume might already exist, which is fine
		if !strings.Contains(string(output), "already exists") {
			logging.Error("Failed to create Jaeger volume: %v\n%s", err, string(output))
		}
	}

	// Fix volume permissions - Jaeger runs as UID 10001
	// Run a temporary container to set ownership on the volume
	logging.Info("   Setting volume permissions for Jaeger user (UID 10001)...")
	chownCmd := exec.Command("docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/badger", j.volumeName),
		"busybox", "chown", "-R", "10001:10001", "/badger")
	if output, err := chownCmd.CombinedOutput(); err != nil {
		logging.Error("Failed to set volume permissions: %v\n%s", err, string(output))
	}

	// Stop and remove old container if it exists
	stopCmd := exec.Command("docker", "stop", j.containerName)
	stopCmd.Run() // Ignore error if container doesn't exist

	rmCmd := exec.Command("docker", "rm", j.containerName)
	rmCmd.Run() // Ignore error if container doesn't exist

	// Start Jaeger container with persistent volume
	dockerArgs := []string{
		"run",
		"-d",
		"--name", j.containerName,
		"-e", "COLLECTOR_OTLP_ENABLED=true",
		"-e", "SPAN_STORAGE_TYPE=badger",
		"-e", "BADGER_EPHEMERAL=false",
		"-e", "BADGER_DIRECTORY_VALUE=/badger/data",
		"-e", "BADGER_DIRECTORY_KEY=/badger/key",
		"-v", fmt.Sprintf("%s:/badger", j.volumeName),
		"-p", fmt.Sprintf("%d:16686", j.uiPort), // Jaeger UI
		"-p", fmt.Sprintf("%d:4318", j.otlpPort), // OTLP HTTP
		"-p", "4317:4317", // OTLP gRPC
		"-p", "14268:14268", // Jaeger collector HTTP
		"jaegertracing/all-in-one:latest",
	}

	startCmd := exec.Command("docker", dockerArgs...)
	output, err := startCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start Jaeger container: %w\n%s", err, string(output))
	}

	logging.Info("   ✅ Jaeger container started: %s", j.containerName)

	// Wait for Jaeger to be accessible
	logging.Info("   Waiting for Jaeger to be ready...")

	deadline := time.Now().Add(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		<-ticker.C
		if j.isAlreadyRunning() {
			logging.Info("   ✅ Jaeger UI ready: http://localhost:%d", j.uiPort)
			logging.Info("   ✅ OTLP endpoint: http://localhost:%d", j.otlpPort)
			logging.Info("   ✅ Traces persisted in volume: %s", j.volumeName)
			j.isRunning = true
			return nil
		}
	}

	// If we timeout, still mark as running
	logging.Info("   ⚠️  Jaeger startup taking longer than expected")
	logging.Info("   💡 Jaeger will continue starting in background")
	logging.Info("   💡 UI will be available at http://localhost:%d", j.uiPort)
	j.isRunning = true

	return nil
}

// Stop gracefully shuts down Jaeger
func (j *JaegerService) Stop(ctx context.Context) error {
	if !j.isRunning {
		return nil
	}

	logging.Info("🧹 Stopping Jaeger container...")

	// Stop container
	stopCmd := exec.Command("docker", "stop", j.containerName)
	if output, err := stopCmd.CombinedOutput(); err != nil {
		logging.Error("Failed to stop Jaeger container: %v\n%s", err, string(output))
	}

	// Remove container
	rmCmd := exec.Command("docker", "rm", j.containerName)
	if output, err := rmCmd.CombinedOutput(); err != nil {
		logging.Error("Failed to remove Jaeger container: %v\n%s", err, string(output))
	}

	j.isRunning = false
	logging.Info("   ✅ Jaeger stopped")

	return nil
}

// IsRunning returns whether Jaeger is currently running
func (j *JaegerService) IsRunning() bool {
	return j.isRunning
}

// GetOTLPEndpoint returns the OTLP HTTP endpoint URL
func (j *JaegerService) GetOTLPEndpoint() string {
	return fmt.Sprintf("http://localhost:%d", j.otlpPort)
}

// GetUIURL returns the Jaeger UI URL
func (j *JaegerService) GetUIURL() string {
	return fmt.Sprintf("http://localhost:%d", j.uiPort)
}

// isAlreadyRunning checks if Jaeger is already running and healthy
func (j *JaegerService) isAlreadyRunning() bool {
	// First check if container exists and is running
	checkContainerCmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", j.containerName)
	output, err := checkContainerCmd.Output()
	if err != nil || string(output) != "true\n" {
		return false
	}

	// Then verify UI is actually responding (not just port exposed)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d", j.uiPort))
	if err != nil {
		// Container running but UI not responding - might be hung
		logging.Info("   ⚠️  Jaeger container exists but UI not responding - will restart")
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}
