package services

import (
	"context"
	"fmt"
	"station/internal/lighthouse"
	"station/internal/lighthouse/proto"
	"station/internal/logging"
	"time"
)

// StreamingService handles bidirectional gRPC streaming with CloudShip
// This service manages the persistent connection for server mode remote control.
type StreamingService struct {
	lighthouseClient *lighthouse.LighthouseClient
	commandHandler   *CommandHandlerService
	metricsService   *MetricsService
	registrationKey  string
	environment      string
	connectionCtx    context.Context
	connectionCancel context.CancelFunc
}

// NewStreamingService creates a new bidirectional streaming service
func NewStreamingService(
	lighthouseClient *lighthouse.LighthouseClient,
	commandHandler *CommandHandlerService,
	metricsService *MetricsService,
	registrationKey string,
	environment string,
) *StreamingService {
	return &StreamingService{
		lighthouseClient: lighthouseClient,
		commandHandler:   commandHandler,
		metricsService:   metricsService,
		registrationKey:  registrationKey,
		environment:      environment,
	}
}

// Start begins the bidirectional streaming connection with CloudShip
func (ss *StreamingService) Start(ctx context.Context) error {
	logging.Info("Starting bidirectional streaming service for server mode")

	// Create cancellable context for connection management
	ss.connectionCtx, ss.connectionCancel = context.WithCancel(ctx)

	// Start connection maintenance loop
	go ss.maintainConnection()

	return nil
}

// Stop gracefully shuts down the bidirectional streaming connection
func (ss *StreamingService) Stop() error {
	logging.Info("Stopping bidirectional streaming service")

	if ss.connectionCancel != nil {
		ss.connectionCancel()
	}

	return nil
}

// maintainConnection manages the persistent connection to CloudShip
func (ss *StreamingService) maintainConnection() {
	for {
		select {
		case <-ss.connectionCtx.Done():
			logging.Info("Bidirectional streaming connection context cancelled")
			return
		default:
			if err := ss.establishConnection(); err != nil {
				logging.Error("Connection failed: %v, retrying in 30s", err)

				// Wait before retrying, but also check for context cancellation
				select {
				case <-ss.connectionCtx.Done():
					return
				case <-time.After(30 * time.Second):
					continue
				}
			}
		}
	}
}

// establishConnection creates and maintains a single bidirectional stream
func (ss *StreamingService) establishConnection() error {
	logging.Info("Establishing bidirectional connection to CloudShip")

	// Only reconnect if the lighthouse client is not connected or not registered
	if !ss.lighthouseClient.IsConnected() || !ss.lighthouseClient.IsRegistered() {
		logging.Info("Lighthouse client needs reconnection...")
		if err := ss.lighthouseClient.Reconnect(); err != nil {
			return fmt.Errorf("failed to reconnect lighthouse client: %v", err)
		}
		logging.Info("Successfully reconnected to CloudShip Lighthouse")
	} else {
		logging.Debug("Lighthouse client already connected and registered")
	}

	// Create the bidirectional stream
	stream, err := ss.lighthouseClient.Connect(ss.connectionCtx)
	if err != nil {
		return fmt.Errorf("failed to create Connect stream: %v", err)
	}

	// Start goroutines for sending and receiving
	sendDone := make(chan error, 1)
	receiveDone := make(chan error, 1)

	go ss.sendConnectionRequests(stream, sendDone)
	go ss.receiveCloudShipCommands(stream, receiveDone)

	// Wait for either goroutine to complete or context to be cancelled
	select {
	case <-ss.connectionCtx.Done():
		logging.Info("Connection context cancelled, closing stream")
		return ss.connectionCtx.Err()
	case err := <-sendDone:
		logging.Error("Send goroutine completed with error: %v", err)
		return err
	case err := <-receiveDone:
		logging.Error("Receive goroutine completed with error: %v", err)
		return err
	}
}

// sendConnectionRequests sends periodic connection requests with system metrics
func (ss *StreamingService) sendConnectionRequests(stream proto.LighthouseService_ConnectClient, done chan<- error) {
	ticker := time.NewTicker(60 * time.Second) // Send every 60 seconds
	defer ticker.Stop()

	// Send initial connection request immediately
	if err := ss.sendConnectionRequest(stream); err != nil {
		done <- fmt.Errorf("initial connection request failed: %v", err)
		return
	}

	for {
		select {
		case <-ss.connectionCtx.Done():
			done <- ss.connectionCtx.Err()
			return
		case <-ticker.C:
			if err := ss.sendConnectionRequest(stream); err != nil {
				done <- fmt.Errorf("periodic connection request failed: %v", err)
				return
			}
		}
	}
}

// sendConnectionRequest sends a single connection request with current metrics
func (ss *StreamingService) sendConnectionRequest(stream proto.LighthouseService_ConnectClient) error {
	metrics := ss.metricsService.GetCurrentMetrics()

	// Convert to proto format
	protoMetrics := &proto.SystemMetrics{
		CpuUsagePercent:    metrics.CPUUsagePercent,
		MemoryUsagePercent: metrics.MemoryUsagePercent,
		DiskUsageMb:        metrics.DiskUsageMB,
		UptimeSeconds:      metrics.UptimeSeconds,
		ActiveConnections:  int32(metrics.ActiveConnections),
		ActiveRuns:         int32(metrics.ActiveRuns),
		NetworkInBytes:     metrics.NetworkInBytes,
		NetworkOutBytes:    metrics.NetworkOutBytes,
		AdditionalMetrics:  metrics.AdditionalMetrics,
	}

	request := &proto.ConnectRequest{
		RegistrationKey: ss.registrationKey,
		Environment:     ss.environment,
		Metrics:         protoMetrics,
	}

	logging.Debug("Sending connection request with metrics: Memory=%.1f%%, ActiveRuns=%d, Uptime=%ds",
		metrics.MemoryUsagePercent, metrics.ActiveRuns, metrics.UptimeSeconds)

	return stream.Send(request)
}

// receiveCloudShipCommands handles incoming commands from CloudShip
func (ss *StreamingService) receiveCloudShipCommands(stream proto.LighthouseService_ConnectClient, done chan<- error) {
	for {
		select {
		case <-ss.connectionCtx.Done():
			done <- ss.connectionCtx.Err()
			return
		default:
			cmd, err := stream.Recv()
			if err != nil {
				done <- fmt.Errorf("receive error: %v", err)
				return
			}

			logging.Info("Received CloudShip command: %T", cmd.Command)

			// Process command in separate goroutine to avoid blocking the stream
			go func() {
				defer func() {
					if r := recover(); r != nil {
						logging.Error("Panic in command processing: %v", r)
					}
				}()

				if err := ss.commandHandler.ProcessCommand(context.Background(), cmd); err != nil {
					logging.Error("Failed to process command %T: %v", cmd.Command, err)
				}
			}()
		}
	}
}

// IsConnected returns whether the service has an active connection
func (ss *StreamingService) IsConnected() bool {
	return ss.connectionCtx != nil && ss.connectionCtx.Err() == nil
}
