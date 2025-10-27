package services

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"station/internal/lighthouse"
	"station/internal/lighthouse/proto"
	"station/internal/logging"
)

// RegistrationState represents the current registration status
type RegistrationState int

const (
	RegistrationStateUnregistered RegistrationState = iota
	RegistrationStateRegistered
	RegistrationStateRejectedLimit // Rejected due to 1:1 limit - should not retry
)

// ManagementChannelService handles the bidirectional ManagementChannel streaming
type ManagementChannelService struct {
	lighthouseClient   *lighthouse.LighthouseClient
	managementHandler  *ManagementHandlerService
	registrationKey    string
	connectionCtx      context.Context
	connectionCancel   context.CancelFunc
	currentStream      proto.LighthouseService_ManagementChannelClient
	registrationState  RegistrationState
}

// NewManagementChannelService creates a new management channel service
func NewManagementChannelService(
	lighthouseClient *lighthouse.LighthouseClient,
	managementHandler *ManagementHandlerService,
	registrationKey string,
) *ManagementChannelService {
	return &ManagementChannelService{
		lighthouseClient:   lighthouseClient,
		managementHandler:  managementHandler,
		registrationKey:    registrationKey,
		registrationState:  RegistrationStateUnregistered,
	}
}

// Start begins the management channel streaming connection
func (mcs *ManagementChannelService) Start(ctx context.Context) error {
	logging.Info("Starting ManagementChannel bidirectional streaming")

	// Create long-lived context for connection management that survives MCP server lifecycle
	// This ensures the management channel continues retrying even after stdio/server modes complete
	mcs.connectionCtx, mcs.connectionCancel = context.WithCancel(context.Background())

	// Start connection maintenance loop
	go mcs.maintainConnection()

	return nil
}

// Stop gracefully shuts down the management channel
func (mcs *ManagementChannelService) Stop() error {
	logging.Info("Stopping ManagementChannel streaming")

	if mcs.connectionCancel != nil {
		mcs.connectionCancel()
	}

	return nil
}

// maintainConnection handles connection lifecycle and reconnection with proper state management
func (mcs *ManagementChannelService) maintainConnection() {
	retryDelay := 1 * time.Second
	maxRetryDelay := 30 * time.Second
	baseDelay := 1 * time.Second
	rejectedStateCheckInterval := 30 * time.Second // Check if other station disconnected

	for {
		select {
		case <-mcs.connectionCtx.Done():
			logging.Info("Management channel connection context cancelled")
			return
		default:
			// If we're in rejected state, don't try to establish connection - just wait
			if mcs.registrationState == RegistrationStateRejectedLimit {
				logging.Debug("Registration state is REJECTED_LIMIT, waiting %v before checking if other station disconnected", rejectedStateCheckInterval)
				time.Sleep(rejectedStateCheckInterval)

				// Reset state to try again - maybe the other station disconnected
				mcs.registrationState = RegistrationStateUnregistered
				logging.Info("Resetting registration state to UNREGISTERED to check if other station disconnected")
				continue
			}

			// Check if we already have an active connection
			if mcs.currentStream != nil {
				logging.Debug("Management channel stream already active, waiting...")
				time.Sleep(5 * time.Second) // Check every 5 seconds if connection is still alive
				continue
			}

			if err := mcs.establishConnection(); err != nil {
				// Check if this is a registration rejection (1:1 limit reached)
				isRegistrationRejected := strings.Contains(err.Error(), "already has") &&
										 strings.Contains(err.Error(), "online stations")

				if isRegistrationRejected {
					// Set state to rejected and stop trying
					mcs.registrationState = RegistrationStateRejectedLimit
					logging.Error("Management channel registration rejected (1:1 limit): %v. Entering REJECTED_LIMIT state - will check again in %v", err, rejectedStateCheckInterval)
					continue // Will hit the rejected state check above
				} else {
					// Normal exponential backoff for connection failures
					jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
					actualDelay := retryDelay + jitter
					logging.Error("Management channel connection failed: %v, retrying in %v", err, actualDelay)

					time.Sleep(actualDelay)

					// Exponential backoff: multiply by 1.5 with cap at maxRetryDelay
					retryDelay = time.Duration(float64(retryDelay) * 1.5)
					if retryDelay > maxRetryDelay {
						retryDelay = maxRetryDelay
					}
				}
				continue
			}

			// Successful connection - reset retry delay and set registered state
			retryDelay = baseDelay
			mcs.registrationState = RegistrationStateRegistered

			// Update global lighthouse status for API status endpoint
			lighthouse.SetConnected(true, "cloudship")
			lighthouse.SetRegistered(true, mcs.registrationKey)

			logging.Info("Successfully registered with CloudShip management channel")
		}
	}
}

// establishConnection establishes the ManagementChannel stream
func (mcs *ManagementChannelService) establishConnection() error {
	logging.Info("Establishing ManagementChannel stream with CloudShip")

	// 🚨 CRITICAL FIX: Ensure RegisterStation completed BEFORE opening ManagementChannel
	// This fixes the "station not found in ConnectionManager" error when containers restart

	// First ensure the underlying lighthouse client is connected
	if !mcs.lighthouseClient.IsConnected() {
		logging.Info("LighthouseClient not connected, attempting reconnection...")
		if err := mcs.lighthouseClient.Reconnect(); err != nil {
			logging.Error("Failed to reconnect lighthouse client: %v", err)
			return fmt.Errorf("failed to reconnect lighthouse client: %w", err)
		}
		logging.Info("Successfully reconnected LighthouseClient")
	} else {
		logging.Debug("LighthouseClient already connected, proceeding with registration check")
	}

	// ✅ REQUIRED: Verify RegisterStation completed successfully before opening stream
	// CloudShip requirement: Station MUST call RegisterStation gRPC before ManagementChannel
	if !mcs.lighthouseClient.IsRegistered() {
		logging.Info("Station not registered, attempting registration before opening ManagementChannel...")
		// Registration is handled in NewLighthouseClient, but might have failed
		// In server mode with container recreation, we need to re-register
		if err := mcs.lighthouseClient.Reconnect(); err != nil {
			logging.Error("Failed to register station: %v", err)
			return fmt.Errorf("station registration required before ManagementChannel: %w", err)
		}

		// Verify registration succeeded
		if !mcs.lighthouseClient.IsRegistered() {
			return fmt.Errorf("station registration failed - cannot open ManagementChannel without successful RegisterStation call")
		}
		logging.Info("✅ Station successfully registered, proceeding with ManagementChannel")
	} else {
		logging.Debug("✅ Station already registered, proceeding with ManagementChannel stream creation")
	}

	// Create independent stream context that doesn't get canceled during normal operations
	// This prevents "context canceled" errors that break the stream
	streamCtx := context.Background()

	// Create the management channel stream with independent context
	stream, err := mcs.lighthouseClient.ManagementChannel(streamCtx)
	if err != nil {
		return fmt.Errorf("failed to create management channel stream: %w", err)
	}

	logging.Info("Successfully established ManagementChannel stream")

	// Store stream reference for SendRun method
	mcs.currentStream = stream

	// Send station registration message first
	registrationMsg := &proto.ManagementMessage{
		RequestId:       fmt.Sprintf("registration_%d", time.Now().Unix()),
		RegistrationKey: mcs.registrationKey,
		IsResponse:      false,
		Success:         true,
		Message: &proto.ManagementMessage_StationRegistration{
			StationRegistration: &proto.StationRegistrationMessage{
				RegistrationKey: mcs.registrationKey,
			},
		},
	}

	if err := stream.Send(registrationMsg); err != nil {
		return fmt.Errorf("failed to send station registration: %w", err)
	}

	logging.Info("Sent station registration to CloudShip")

	// Start goroutine for receiving messages from CloudShip
	// This goroutine will run independently and handle connection errors
	go func() {
		if err := mcs.receiveMessages(stream); err != nil {
			logging.Error("Management channel receive error: %v", err)
			// Clear stream reference on error
			mcs.currentStream = nil
			// Update global lighthouse status to disconnected
			lighthouse.SetConnected(false, "")
			lighthouse.SetRegistered(false, "")
			// Connection will be retried by maintainConnection loop
		}
	}()

	// Connection established successfully - return immediately
	// receiveMessages goroutine will handle the ongoing connection
	return nil
}

// receiveMessages handles incoming messages from CloudShip
func (mcs *ManagementChannelService) receiveMessages(stream proto.LighthouseService_ManagementChannelClient) error {
	for {
		// Removed connectionCtx monitoring since stream has its own context.Background()
		// This prevents unnecessary stream restarts due to context cancellation

		// Use non-blocking receive with timeout
		done := make(chan error, 1)
		go func() {
			msg, err := stream.Recv()
			if err != nil {
				done <- err
				return
			}

			logging.Debug("Received management message: request_id=%s, is_response=%v", msg.RequestId, msg.IsResponse)

			// Only process requests (not responses)
			if !msg.IsResponse {
				go mcs.processRequest(stream, msg)
			}
			done <- nil
		}()

		select {
		case err := <-done:
			if err != nil {
				// Log different types of errors for better debugging
				if err == io.EOF {
					logging.Info("Management channel stream closed by CloudShip")
				} else {
					logging.Error("Failed to receive message from CloudShip: %v", err)
				}
				return fmt.Errorf("stream receive error: %w", err)
			}
		case <-time.After(60 * time.Second):
			// Management channel connection health check
			// No separate heartbeat needed - gRPC keepalive handles connection health
			// Longer timeout prevents "too_many_pings" issue while still detecting disconnections
			logging.Debug("Management channel receive timeout - connection appears healthy")
			continue
		}
	}
}

// processRequest processes a management request and sends response
func (mcs *ManagementChannelService) processRequest(stream proto.LighthouseService_ManagementChannelClient, req *proto.ManagementMessage) {
	logging.Info("Processing management request: %s (request_id: %s)", mcs.getRequestType(req), req.RequestId)

	// Process the request using the management handler
	resp, err := mcs.managementHandler.ProcessManagementRequest(context.Background(), req)
	if err != nil {
		logging.Error("Failed to process management request: %v", err)

		// Send error response
		errorResp := &proto.ManagementMessage{
			RequestId:       req.RequestId,
			RegistrationKey: mcs.registrationKey,
			IsResponse:      true,
			Success:         false,
			Message: &proto.ManagementMessage_Error{
				Error: &proto.ManagementError{
					Code:    proto.ErrorCode_UNKNOWN_ERROR,
					Message: err.Error(),
				},
			},
		}

		if sendErr := stream.Send(errorResp); sendErr != nil {
			logging.Error("Failed to send error response: %v", sendErr)
		}
		return
	}

	// Send the response
	if err := stream.Send(resp); err != nil {
		logging.Error("Failed to send management response: %v", err)
		return
	}

	logging.Info("Successfully sent management response: %s (request_id: %s)", mcs.getRequestType(req), req.RequestId)
}

// SendRun sends a completed agent run via ManagementChannel
func (mcs *ManagementChannelService) SendRun(agentRunData *proto.LighthouseAgentRunData, tags map[string]string) error {
	// Create SendRun request message
	sendRunMsg := &proto.ManagementMessage{
		RequestId:       fmt.Sprintf("sendrun_%d", time.Now().Unix()),
		RegistrationKey: mcs.registrationKey,
		IsResponse:      false,
		Success:         true,
		Message: &proto.ManagementMessage_SendRunRequest{
			SendRunRequest: &proto.SendRunRequest{
				RegistrationKey: mcs.registrationKey,
				Environment:     "cloudship",
				Mode:            proto.DeploymentMode_DEPLOYMENT_MODE_SERVE,
				Source:          proto.RunSource_RUN_SOURCE_UI_TRIGGERED,
				RunData:         agentRunData,
				Labels:          tags,
			},
		},
	}

	// We need access to the current stream to send the message
	// For now, we'll store the current stream reference
	if mcs.currentStream == nil {
		return fmt.Errorf("no active management channel stream")
	}

	if err := mcs.currentStream.Send(sendRunMsg); err != nil {
		logging.Error("Failed to send SendRun message: %v", err)
		return fmt.Errorf("failed to send SendRun message: %w", err)
	}

	logging.Debug("Successfully sent SendRun message for run %s", agentRunData.RunId)
	return nil
}

// SendStatusUpdate sends a status update message via ManagementChannel
func (mcs *ManagementChannelService) SendStatusUpdate(statusMsg *proto.ManagementMessage) error {
	if mcs.currentStream == nil {
		return fmt.Errorf("no active management channel stream")
	}
	
	if err := mcs.currentStream.Send(statusMsg); err != nil {
		return fmt.Errorf("failed to send status update: %w", err)
	}
	
	return nil
}

// getRequestType returns a human-readable request type for logging
func (mcs *ManagementChannelService) getRequestType(msg *proto.ManagementMessage) string {
	switch msg.Message.(type) {
	case *proto.ManagementMessage_ListAgentsRequest:
		return "ListAgents"
	case *proto.ManagementMessage_ListToolsRequest:
		return "ListTools"
	case *proto.ManagementMessage_GetEnvironmentsRequest:
		return "GetEnvironments"
	case *proto.ManagementMessage_ExecuteAgentRequest:
		return "ExecuteAgent"
	case *proto.ManagementMessage_StationRegistration:
		return "StationRegistration"
	case *proto.ManagementMessage_SendRunRequest:
		return "SendRun"
	case *proto.ManagementMessage_GetAgentDetailsRequest:
		return "GetAgentDetails"
	case *proto.ManagementMessage_UpdateAgentPromptRequest:
		return "UpdateAgentPrompt"
	default:
		return "Unknown"
	}
}
