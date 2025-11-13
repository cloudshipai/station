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

	// Create Jaeger container with in-memory storage
	// Note: Using BADGER_EPHEMERAL=true to avoid permission issues with persistent volumes
	// Traces will be lost on restart, but this is acceptable for development
	container := client.Container().
		From("jaegertracing/all-in-one:latest").
		WithEnvVariable("COLLECTOR_OTLP_ENABLED", "true").
		WithEnvVariable("SPAN_STORAGE_TYPE", "badger").
		WithEnvVariable("BADGER_EPHEMERAL", "true").
		WithExposedPort(j.uiPort).   // Jaeger UI
		WithExposedPort(14268).      // Jaeger collector HTTP
		WithExposedPort(j.otlpPort). // OTLP HTTP
		WithExposedPort(4317)        // OTLP gRPC

	// Convert to service
	service := container.AsService()

	j.container = container
	j.service = service

	// Start the service completely async - don't block Station startup
	// This creates long-lived tunnel processes that forward ports to localhost
	logging.Info("   Starting Jaeger container in background...")

	go func() {
		// Up() with port forwarding - this blocks until service stops
		err := service.Up(ctx, dagger.ServiceUpOpts{Ports: []dagger.PortForward{
			{Frontend: j.uiPort, Backend: j.uiPort},
			{Frontend: 14268, Backend: 14268},
			{Frontend: j.otlpPort, Backend: j.otlpPort},
			{Frontend: 4317, Backend: 4317},
		}})
		if err != nil {
			logging.Error("Jaeger service.Up() error: %v", err)
			j.isRunning = false
			return
		}
		logging.Info("   ✅ Jaeger port forwarding established")
	}()

	// Launch background health checker that polls until Jaeger is ready
	go func() {
		// Give container time to start before checking
		time.Sleep(3 * time.Second)

		// Poll for up to 2 minutes
		deadline := time.Now().Add(120 * time.Second)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for time.Now().Before(deadline) {
			<-ticker.C
			if j.isAlreadyRunning() {
				logging.Info("   ✅ Jaeger UI ready: http://localhost:%d", j.uiPort)
				logging.Info("   ✅ OTLP endpoint: http://localhost:%d", j.otlpPort)
				logging.Info("   ✅ Traces persisted in Docker volume: jaeger-badger-data")
				return
			}
		}

		// If we get here, Jaeger didn't start in time
		logging.Info("   ⚠️  Jaeger startup taking longer than expected")
		logging.Info("   💡 Traces will be buffered until Jaeger becomes available")
	}()

	j.isRunning = true
	logging.Info("   Jaeger starting in background (UI at http://localhost:%d)", j.uiPort)

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
