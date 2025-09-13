package lighthouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"runtime"
	"station/internal/config"
	"station/internal/lighthouse/proto"
	"station/internal/logging"
	"station/pkg/types"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// DeploymentMode represents how Station is currently running
type DeploymentMode int

const (
	ModeUnknown DeploymentMode = iota
	ModeStdio                  // stn stdio - local development
	ModeServe                  // stn serve - team/production server
	ModeCLI                    // all other commands - CI/CD & ephemeral
)

// LighthouseConfig holds configuration for connecting to CloudShip Lighthouse
type LighthouseConfig struct {
	// Core connection settings
	Endpoint        string `yaml:"endpoint"`        // lighthouse.cloudship.ai:443
	RegistrationKey string `yaml:"registration_key"` // CloudShip registration key
	StationID       string `yaml:"station_id"`      // Generated station ID
	TLS             bool   `yaml:"tls"`             // Enable TLS (default: true)
	
	// Optional settings
	Environment     string        `yaml:"environment"`      // Environment name (default: "default")
	ConnectTimeout  time.Duration `yaml:"connect_timeout"`  // Connection timeout (default: 10s)
	RequestTimeout  time.Duration `yaml:"request_timeout"`  // Request timeout (default: 30s)
	KeepAlive       time.Duration `yaml:"keepalive"`        // Keep alive interval (default: 30s)
	
	// Mode-specific settings
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"` // serve mode heartbeat (default: 30s)
	BufferSize        int           `yaml:"buffer_size"`        // Local buffer size (default: 100)
}

// DefaultLighthouseConfig returns sensible defaults
func DefaultLighthouseConfig() *LighthouseConfig {
	return &LighthouseConfig{
		Endpoint:          "lighthouse.cloudship.ai:443",
		TLS:               true,
		Environment:       "default",
		ConnectTimeout:    10 * time.Second,
		RequestTimeout:    30 * time.Second,
		KeepAlive:         30 * time.Second,
		HeartbeatInterval: 30 * time.Second,
		BufferSize:        100,
	}
}

// LighthouseClient manages connection to CloudShip Lighthouse service
type LighthouseClient struct {
	config   *LighthouseConfig
	mode     DeploymentMode
	conn     *grpc.ClientConn
	client   proto.LighthouseServiceClient
	
	// State management
	registered   bool
	stationID    string
	mu           sync.RWMutex
	
	// Background tasks
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	
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
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 10 * time.Second
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 30 * time.Second
	}
	if config.KeepAlive == 0 {
		config.KeepAlive = 30 * time.Second
	}
	if config.BufferSize == 0 {
		config.BufferSize = 100
	}
	
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

// connect establishes gRPC connection to Lighthouse
func (lc *LighthouseClient) connect() error {
	var opts []grpc.DialOption
	
	// Configure TLS
	if lc.config.TLS {
		tlsConfig := &tls.Config{
			ServerName: strings.Split(lc.config.Endpoint, ":")[0],
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	
	// Configure keep-alive
	keepaliveParams := keepalive.ClientParameters{
		Time:                lc.config.KeepAlive,
		Timeout:             lc.config.ConnectTimeout,
		PermitWithoutStream: true,
	}
	opts = append(opts, grpc.WithKeepaliveParams(keepaliveParams))
	
	// Connect with timeout
	connectCtx, cancel := context.WithTimeout(lc.ctx, lc.config.ConnectTimeout)
	defer cancel()
	
	conn, err := grpc.DialContext(connectCtx, lc.config.Endpoint, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", lc.config.Endpoint, err)
	}
	
	lc.conn = conn
	lc.client = proto.NewLighthouseServiceClient(conn)
	
	logging.Info("Connected to CloudShip Lighthouse at %s", lc.config.Endpoint)
	return nil
}

// register registers this Station instance with CloudShip
func (lc *LighthouseClient) register() error {
	if lc.client == nil {
		return fmt.Errorf("not connected to Lighthouse")
	}
	
	// Generate station ID if not provided
	if lc.config.StationID == "" {
		lc.config.StationID = generateStationID()
	}
	
	req := &proto.RegisterStationRequest{
		StationId:       lc.config.StationID,
		RegistrationKey: lc.config.RegistrationKey,
		EnvironmentName: lc.config.Environment,
		Mode:            deploymentModeToProto(lc.mode),
		Capabilities:    lc.getStationCapabilities(),
		Metadata:        lc.getStationMetadata(),
	}
	
	ctx, cancel := context.WithTimeout(lc.ctx, lc.config.RequestTimeout)
	defer cancel()
	
	resp, err := lc.client.RegisterStation(ctx, req)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}
	
	if !resp.Success {
		return fmt.Errorf("registration rejected: %s", resp.ErrorMessage)
	}
	
	lc.mu.Lock()
	lc.registered = true
	lc.stationID = resp.StationId
	lc.mu.Unlock()
	
	logging.Info("Station registered with CloudShip (ID: %s, Mode: %s)", resp.StationId, lc.mode)
	return nil
}

// IsRegistered returns true if successfully registered with CloudShip
func (lc *LighthouseClient) IsRegistered() bool {
	if lc == nil {
		return false
	}
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return lc.registered
}

// IsConnected returns true if connected to Lighthouse
func (lc *LighthouseClient) IsConnected() bool {
	if lc == nil || lc.conn == nil {
		return false
	}
	return lc.conn.GetState().String() == "READY"
}

// GetMode returns the current deployment mode
func (lc *LighthouseClient) GetMode() DeploymentMode {
	if lc == nil {
		return ModeUnknown
	}
	return lc.mode
}

// SendRun sends agent run data to CloudShip (async, buffered)
func (lc *LighthouseClient) SendRun(runData *types.AgentRun, environment string, labels map[string]string) {
	if !lc.IsRegistered() {
		return // Graceful degradation - no cloud integration
	}
	
	req := &proto.SendRunRequest{
		RegistrationKey: lc.config.RegistrationKey,
		Environment:     environment,
		Mode:            convertDeploymentModeToProto(lc.mode),
		Source:          proto.RunSource_RUN_SOURCE_ANALYTICS,  // Station completed run data
		RunData:         convertAgentRunToProto(runData),
		Labels:          labels,
	}
	
	// For CLI mode, send synchronously to avoid context cancellation
	if lc.mode == ModeCLI {
		lc.sendRunSync(req)
	} else {
		// Non-blocking send to buffer for other modes
		select {
		case lc.runBuffer <- req:
			// Successfully buffered
		default:
			logging.Info("Lighthouse run buffer full, dropping run data (run_id: %s)", runData.ID)
		}
	}
}

// SendEphemeralSnapshot sends CLI mode rich context snapshot
func (lc *LighthouseClient) SendEphemeralSnapshot(runData *types.AgentRun, deploymentCtx *types.DeploymentContext, system *types.SystemSnapshot) error {
	if !lc.IsRegistered() || lc.mode != ModeCLI {
		return nil // Graceful degradation or wrong mode
	}
	
	environment := lc.config.Environment
	if environment == "" {
		environment = "default" // Use default if not specified
	}
	
	// Debug: Check for nil pointers before conversion
	logging.Debug("SendEphemeralSnapshot: deploymentCtx=%v, system=%v", deploymentCtx != nil, system != nil)
	if deploymentCtx != nil {
		logging.Debug("SendEphemeralSnapshot: deploymentCtx.CommandLine=%s, WorkingDir=%s, EnvVars=%d, Args=%d", 
			deploymentCtx.CommandLine, deploymentCtx.WorkingDirectory, len(deploymentCtx.EnvVars), len(deploymentCtx.Arguments))
		
		// Debug: Show actual environment variables being sent
		for key, value := range deploymentCtx.EnvVars {
			// Truncate long values like PATH
			displayValue := value
			if len(displayValue) > 100 {
				displayValue = displayValue[:100] + "..."
			}
			logging.Debug("SendEphemeralSnapshot: EnvVar %s=%s", key, displayValue)
		}
	}
	
	if system != nil {
		logging.Debug("SendEphemeralSnapshot: system - Agents=%d, MCPServers=%d, Variables=%d, Tools=%d", 
			len(system.Agents), len(system.MCPServers), len(system.Variables), len(system.AvailableTools))
	}
	
	req := &proto.EphemeralSnapshotRequest{
		RegistrationKey: lc.config.RegistrationKey,
		Environment:     environment,
		Source:          proto.RunSource_RUN_SOURCE_CLI_SNAPSHOT,  // CLI ephemeral snapshot
		Context:         convertDeploymentContextToProto(deploymentCtx),
		RunData:         convertAgentRunToProto(runData),
		System:          convertSystemSnapshotToProto(system),
	}
	
	logging.Debug("SendEphemeralSnapshot: req prepared - RegistrationKey=%s, Environment=%s, Source=%d", 
		req.RegistrationKey, req.Environment, req.Source)
	logging.Debug("SendEphemeralSnapshot: req.Context != nil: %v", req.Context != nil)
	logging.Debug("SendEphemeralSnapshot: req.RunData != nil: %v", req.RunData != nil)
	logging.Debug("SendEphemeralSnapshot: req.System != nil: %v", req.System != nil)
	
	if req.RunData != nil {
		logging.Debug("SendEphemeralSnapshot: RunData - ID=%s, AgentID=%s, Status=%d, ToolCalls=%d", 
			req.RunData.RunId, req.RunData.AgentId, req.RunData.Status, len(req.RunData.ToolCalls))
	}
	
	ctx, cancel := context.WithTimeout(lc.ctx, lc.config.RequestTimeout)
	defer cancel()
	
	resp, err := lc.client.SendEphemeralSnapshot(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send ephemeral snapshot: %w", err)
	}
	
	if !resp.Success {
		return fmt.Errorf("ephemeral snapshot rejected: %s", resp.Message)
	}
	
	logging.Debug("Sent ephemeral snapshot to CloudShip (ID: %s)", resp.SnapshotId)
	return nil
}

// SendSystemHealth sends system health metrics (serve mode primarily)
func (lc *LighthouseClient) SendSystemHealth(status proto.SystemStatus, metrics *types.SystemMetrics) {
	if !lc.IsRegistered() || lc.mode != ModeServe {
		return // Only for serve mode
	}
	
	req := &proto.SystemHealthRequest{
		RegistrationKey: lc.config.RegistrationKey,
		Environment:     lc.config.Environment,
		Status:          status,
		Metrics:         convertSystemMetricsToProto(metrics),
		Timestamp:       timestampNow(),
	}
	
	// Non-blocking send to buffer
	select {
	case lc.healthBuffer <- req:
		// Successfully buffered
	default:
		logging.Info("Lighthouse health buffer full, dropping health data")
	}
}

// Close gracefully shuts down the Lighthouse client
func (lc *LighthouseClient) Close() error {
	if lc == nil {
		return nil
	}
	
	logging.Info("Shutting down Lighthouse client...")
	
	// Cancel context to stop background workers (only if initialized)
	if lc.cancel != nil {
		lc.cancel()
	}
	
	// Wait for background workers to finish
	lc.wg.Wait()
	
	// Close gRPC connection
	if lc.conn != nil {
		if err := lc.conn.Close(); err != nil {
			logging.Error("Error closing Lighthouse connection: %v", err)
			return err
		}
	}
	
	logging.Info("Lighthouse client shutdown complete")
	return nil
}

// startBackgroundWorkers starts background goroutines for buffered operations
func (lc *LighthouseClient) startBackgroundWorkers() {
	// Run data worker
	lc.wg.Add(1)
	go func() {
		defer lc.wg.Done()
		for {
			select {
			case <-lc.ctx.Done():
				return
			case req := <-lc.runBuffer:
				lc.sendRunSync(req)
			}
		}
	}()
	
	// Health data worker (serve mode only)
	if lc.mode == ModeServe {
		lc.wg.Add(1)
		go func() {
			defer lc.wg.Done()
			for {
				select {
				case <-lc.ctx.Done():
					return
				case req := <-lc.healthBuffer:
					lc.sendHealthSync(req)
				}
			}
		}()
		
		// Heartbeat worker (serve mode only)
		lc.wg.Add(1)
		go func() {
			defer lc.wg.Done()
			ticker := time.NewTicker(lc.config.HeartbeatInterval)
			defer ticker.Stop()
			
			for {
				select {
				case <-lc.ctx.Done():
					return
				case <-ticker.C:
					lc.sendHeartbeat()
				}
			}
		}()
	}
}

// sendRunSync sends run data synchronously (internal)
func (lc *LighthouseClient) sendRunSync(req *proto.SendRunRequest) {
	if !lc.IsConnected() {
		logging.Debug("Not connected to Lighthouse, skipping run data")
		return
	}
	
	ctx, cancel := context.WithTimeout(lc.ctx, lc.config.RequestTimeout)
	defer cancel()
	
	resp, err := lc.client.SendRun(ctx, req)
	if err != nil {
		logging.Info("Failed to send run to CloudShip: %v", err)
		return
	}
	
	if !resp.Success {
		logging.Info("CloudShip rejected run: %s", resp.Message)
		return
	}
	
	logging.Debug("Sent run to CloudShip (run_id: %s)", resp.RunId)
}

// sendHealthSync sends health data synchronously (internal)
func (lc *LighthouseClient) sendHealthSync(req *proto.SystemHealthRequest) {
	if !lc.IsConnected() {
		return
	}
	
	ctx, cancel := context.WithTimeout(lc.ctx, lc.config.RequestTimeout)
	defer cancel()
	
	resp, err := lc.client.SendSystemHealth(ctx, req)
	if err != nil {
		logging.Debug("Failed to send health data to CloudShip: %v", err)
		return
	}
	
	if !resp.Success {
		logging.Info("CloudShip rejected health data: %s", resp.Message)
	}
}

// sendHeartbeat sends periodic heartbeat (serve mode only)
func (lc *LighthouseClient) sendHeartbeat() {
	if !lc.IsConnected() {
		return
	}
	
	req := &proto.HeartbeatRequest{
		RegistrationKey: lc.config.RegistrationKey,
		Environment:     lc.config.Environment,
		Status:          proto.SystemStatus_SYSTEM_STATUS_HEALTHY, // TODO: dynamic status
		Timestamp:       timestampNow(),
	}
	
	ctx, cancel := context.WithTimeout(lc.ctx, lc.config.RequestTimeout)
	defer cancel()
	
	resp, err := lc.client.Heartbeat(ctx, req)
	if err != nil {
		logging.Debug("Heartbeat failed: %v", err)
		return
	}
	
	if !resp.Success {
		logging.Info("CloudShip heartbeat rejected: %s", resp.Message)
	}
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
		TLS:             true, // Default to TLS for production
		Environment:     "default", // TODO: make configurable
	}
	
	// Apply defaults
	if lighthouseConfig.Endpoint == "" {
		lighthouseConfig.Endpoint = "lighthouse.cloudship.ai:443"
	}
	
	// Disable TLS for localhost testing
	if strings.Contains(lighthouseConfig.Endpoint, "localhost:") || strings.Contains(lighthouseConfig.Endpoint, "127.0.0.1:") {
		lighthouseConfig.TLS = false
	}
	
	return NewLighthouseClient(lighthouseConfig, mode)
}

// Helper functions

func deploymentModeToProto(mode DeploymentMode) proto.DeploymentMode {
	switch mode {
	case ModeStdio:
		return proto.DeploymentMode_DEPLOYMENT_MODE_STDIO
	case ModeServe:
		return proto.DeploymentMode_DEPLOYMENT_MODE_SERVE
	case ModeCLI:
		return proto.DeploymentMode_DEPLOYMENT_MODE_CLI
	default:
		return proto.DeploymentMode_DEPLOYMENT_MODE_UNSPECIFIED
	}
}

func (lc *LighthouseClient) getStationCapabilities() *proto.StationCapabilities {
	return &proto.StationCapabilities{
		CanExecuteAgents:      true,
		HasMcpServers:         true,
		SupportsBidirectional: lc.mode == ModeServe,
		Environments:          []string{lc.config.Environment},
		AgentCount:            0, // TODO: get actual count
		ToolCount:             0, // TODO: get actual count
	}
}

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

func generateStationID() string {
	// Generate a simple station ID - can be enhanced later
	hostname, _ := os.Hostname()
	return fmt.Sprintf("station-%s-%d", hostname, time.Now().Unix())
}

func (mode DeploymentMode) String() string {
	switch mode {
	case ModeStdio:
		return "stdio"
	case ModeServe:
		return "serve"
	case ModeCLI:
		return "cli"
	default:
		return "unknown"
	}
}

// convertDeploymentModeToProto converts Station deployment mode to proto enum
func convertDeploymentModeToProto(mode DeploymentMode) proto.DeploymentMode {
	switch mode {
	case ModeStdio:
		return proto.DeploymentMode_DEPLOYMENT_MODE_STDIO
	case ModeServe:
		return proto.DeploymentMode_DEPLOYMENT_MODE_SERVE
	case ModeCLI:
		return proto.DeploymentMode_DEPLOYMENT_MODE_CLI
	default:
		return proto.DeploymentMode_DEPLOYMENT_MODE_UNSPECIFIED
	}
}