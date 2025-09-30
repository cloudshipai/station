package lighthouse

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"station/internal/config"
	"station/internal/lighthouse/proto"
	"station/internal/logging"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
)

// LighthouseClient manages connection to CloudShip Lighthouse service
type LighthouseClient struct {
	config *LighthouseConfig
	mode   DeploymentMode
	conn   *grpc.ClientConn
	client proto.LighthouseServiceClient

	// State management
	registered     bool
	stationID      string
	organizationID string // Cached organization ID from registration key
	mu             sync.RWMutex

	// Background tasks
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Buffering for reliability
	runBuffer    chan *proto.SendRunRequest
	healthBuffer chan *proto.SystemHealthRequest
}

// NewLighthouseClient creates a new Lighthouse client
func NewLighthouseClient(config *LighthouseConfig, mode DeploymentMode) (*LighthouseClient, error) {
	if config == nil {
		config = DefaultLighthouseConfig()
	}

	// Apply defaults for zero values
	config.ApplyDefaults()

	// Validate required configuration
	if config.RegistrationKey == "" {
		return &LighthouseClient{
			config:     config,
			mode:       mode,
			registered: false,
		}, nil // Not an error - just unregistered mode
	}

	if config.Endpoint == "" {
		return nil, fmt.Errorf("lighthouse endpoint is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &LighthouseClient{
		config:       config,
		mode:         mode,
		ctx:          ctx,
		cancel:       cancel,
		runBuffer:    make(chan *proto.SendRunRequest, config.BufferSize),
		healthBuffer: make(chan *proto.SystemHealthRequest, 10),
	}

	// Attempt connection
	if err := client.connect(); err != nil {
		logging.Info("Failed to connect to Lighthouse: %v (continuing without cloud integration)", err)
		return client, nil // Not fatal - graceful degradation
	}

	// Attempt registration
	if err := client.register(); err != nil {
		logging.Info("Failed to register with CloudShip: %v (continuing without cloud integration)", err)
		return client, nil // Not fatal - graceful degradation
	}

	// Start background workers for buffered operations
	client.startBackgroundWorkers()

	return client, nil
}

// GetMode returns the current deployment mode
func (lc *LighthouseClient) GetMode() DeploymentMode {
	if lc == nil {
		return ModeUnknown
	}
	return lc.mode
}

// generateStationID generates a simple station ID
func generateStationID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("station-%s-%d", hostname, time.Now().Unix())
}

// getStationCapabilities returns station capabilities for registration
func (lc *LighthouseClient) getStationCapabilities() *proto.StationCapabilities {
	return &proto.StationCapabilities{
		CanExecuteAgents:      true,
		HasMcpServers:         true,
		SupportsBidirectional: lc.mode == ModeServe || lc.mode == ModeStdio,
		Environments:          []string{lc.config.Environment},
		AgentCount:            0, // TODO: get actual count
		ToolCount:             0, // TODO: get actual count
	}
}

// getStationMetadata returns station metadata for registration
func (lc *LighthouseClient) getStationMetadata() map[string]string {
	hostname, _ := os.Hostname()
	return map[string]string{
		"station_version": "v0.11.0", // TODO: get from version package
		"os":              runtime.GOOS,
		"arch":            runtime.GOARCH,
		"go_version":      runtime.Version(),
		"hostname":        hostname,
		"mode":            lc.mode.String(),
	}
}

// InitializeLighthouseFromConfig creates a Lighthouse client from Station config
func InitializeLighthouseFromConfig(cfg *config.Config, mode DeploymentMode) (*LighthouseClient, error) {
	// Check if CloudShip integration is enabled
	if !cfg.CloudShip.Enabled || cfg.CloudShip.RegistrationKey == "" {
		return nil, nil // No CloudShip integration
	}

	lighthouseConfig := &LighthouseConfig{
		Endpoint:        cfg.CloudShip.Endpoint,
		RegistrationKey: cfg.CloudShip.RegistrationKey,
		StationID:       cfg.CloudShip.StationID,
		TLS:             true,      // Default to TLS for production
		Environment:     "default", // TODO: make configurable
	}

	// Apply defaults
	if lighthouseConfig.Endpoint == "" {
		lighthouseConfig.Endpoint = "lighthouse.cloudshipai.com:443"
	}

	// Disable TLS for localhost testing and port 50051 (standard insecure gRPC port)
	if strings.Contains(lighthouseConfig.Endpoint, "localhost:") ||
	   strings.Contains(lighthouseConfig.Endpoint, "127.0.0.1:") ||
	   strings.Contains(lighthouseConfig.Endpoint, ":50051") {
		lighthouseConfig.TLS = false
	}

	return NewLighthouseClient(lighthouseConfig, mode)
}

// DetectModeFromCommand detects deployment mode based on command line arguments
func DetectModeFromCommand() DeploymentMode {
	args := os.Args
	if len(args) < 2 {
		return ModeCLI
	}

	switch args[1] {
	case "stdio":
		return ModeStdio
	case "serve":
		return ModeServe
	default:
		return ModeCLI
	}
}
