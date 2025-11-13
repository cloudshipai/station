package services

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"dagger.io/dagger"
	"station/internal/logging"
)

// JaegerService manages Jaeger all-in-one as a background service using Dagger
type JaegerService struct {
	client    *dagger.Client
	container *dagger.Container
	service   *dagger.Service
	dataDir   string
	uiPort    int
	otlpPort  int
	isRunning bool
}

// JaegerConfig holds Jaeger service configuration
type JaegerConfig struct {
	UIPort   int    // Default: 16686
	OTLPPort int    // Default: 4318
	DataDir  string // Default: ~/.local/share/station/jaeger-data
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
	if cfg.DataDir == "" {
		homeDir, _ := os.UserHomeDir()
		cfg.DataDir = filepath.Join(homeDir, ".local", "share", "station", "jaeger-data")
	}

	return &JaegerService{
		dataDir:  cfg.DataDir,
		uiPort:   cfg.UIPort,
		otlpPort: cfg.OTLPPort,
	}
}

// Start launches Jaeger as a Dagger service
func (j *JaegerService) Start(ctx context.Context) error {
	// Check if Jaeger already running
	if j.isAlreadyRunning() {
		logging.Info("🔍 Jaeger already running on port %d - reusing existing instance", j.uiPort)
		j.isRunning = true
		return nil
	}

	logging.Info("🔍 Launching Jaeger (background service)...")

	// Initialize Dagger client
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return fmt.Errorf("failed to create Dagger client: %w", err)
	}
	j.client = client

	// Create a named cache volume for Jaeger data persistence
	jaegerVolume := client.CacheVolume("jaeger-badger-data")

	// Create Jaeger container with Badger storage for persistence
	// Note: Running as root to avoid permission issues with cache volume
	container := client.Container().
		From("jaegertracing/all-in-one:latest").
		WithUser("root").
		WithEnvVariable("COLLECTOR_OTLP_ENABLED", "true").
		WithEnvVariable("SPAN_STORAGE_TYPE", "badger").
		WithEnvVariable("BADGER_EPHEMERAL", "false").
		WithEnvVariable("BADGER_DIRECTORY_VALUE", "/badger/data").
		WithEnvVariable("BADGER_DIRECTORY_KEY", "/badger/key").
		WithMountedCache("/badger", jaegerVolume).
		WithExposedPort(j.uiPort).   // Jaeger UI
		WithExposedPort(14268).      // Jaeger collector HTTP
		WithExposedPort(j.otlpPort). // OTLP HTTP
		WithExposedPort(4317)        // OTLP gRPC

	// Convert to service
	service := container.AsService()

	j.container = container
	j.service = service

	// Start the service with a background context
	// This creates long-lived tunnel processes that forward ports to localhost
	logging.Info("   Starting Jaeger container and port forwarding...")

	// Channel to signal when Up() completes or times out
	upDone := make(chan error, 1)

	go func() {
		// Up() with port forwarding - this blocks while tunnels are active
		err := service.Up(ctx, dagger.ServiceUpOpts{Ports: []dagger.PortForward{
			{Frontend: j.uiPort, Backend: j.uiPort},
			{Frontend: 14268, Backend: 14268},
			{Frontend: j.otlpPort, Backend: j.otlpPort},
			{Frontend: 4317, Backend: 4317},
		}})
		upDone <- err
	}()

	// Wait for Up() to complete with a generous timeout (2 minutes for first start)
	select {
	case err := <-upDone:
		if err != nil {
			return fmt.Errorf("Jaeger service.Up() failed: %w", err)
		}
		logging.Info("   ✅ Jaeger port forwarding established")
	case <-time.After(120 * time.Second):
		logging.Info("   ⚠️  Jaeger startup taking longer than expected, continuing in background...")
	}

	// Give Jaeger process itself time to start
	time.Sleep(5 * time.Second)

	// Check if Jaeger is ready (with timeout)
	readyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := j.waitForReady(readyCtx); err != nil {
		logging.Info("⚠️  Jaeger service started but connectivity check timed out")
		logging.Info("   💡 Jaeger is running - UI may take a few moments to become available")
	}

	j.isRunning = true

	logging.Info("   ✅ Jaeger UI: http://localhost:%d", j.uiPort)
	logging.Info("   ✅ OTLP endpoint: http://localhost:%d", j.otlpPort)
	logging.Info("   ✅ Traces persisted in Docker volume: jaeger-badger-data")

	return nil
}

// Stop gracefully shuts down Jaeger
func (j *JaegerService) Stop(ctx context.Context) error {
	if !j.isRunning {
		return nil
	}

	logging.Info("🧹 Stopping Jaeger service...")

	// Stop service
	if j.service != nil {
		_, err := j.service.Stop(ctx)
		if err != nil {
			logging.Error("Failed to stop Jaeger service: %v", err)
		}
	}

	// Close Dagger client
	if j.client != nil {
		if err := j.client.Close(); err != nil {
			logging.Error("Failed to close Dagger client: %v", err)
		}
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

// isAlreadyRunning checks if Jaeger is already running by trying to connect to UI
func (j *JaegerService) isAlreadyRunning() bool {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d", j.uiPort))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// waitForReady waits for Jaeger UI to be accessible
func (j *JaegerService) waitForReady(ctx context.Context) error {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("Jaeger did not start within 30 seconds")
		case <-ticker.C:
			if j.isAlreadyRunning() {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
