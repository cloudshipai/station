package services

import (
	"context"
	"fmt"
	"io"
	"time"

	"station/internal/lighthouse"
	"station/internal/lighthouse/proto"
	"station/internal/logging"
)

// ManagementChannelService handles the bidirectional ManagementChannel streaming
type ManagementChannelService struct {
	lighthouseClient  *lighthouse.LighthouseClient
	managementHandler *ManagementHandlerService
	registrationKey   string
	connectionCtx     context.Context
	connectionCancel  context.CancelFunc
	currentStream     proto.LighthouseService_ManagementChannelClient
}

// NewManagementChannelService creates a new management channel service
func NewManagementChannelService(
	lighthouseClient *lighthouse.LighthouseClient,
	managementHandler *ManagementHandlerService,
	registrationKey string,
) *ManagementChannelService {
	return &ManagementChannelService{
		lighthouseClient:  lighthouseClient,
		managementHandler: managementHandler,
		registrationKey:   registrationKey,
	}
}

// Start begins the management channel streaming connection
func (mcs *ManagementChannelService) Start(ctx context.Context) error {
	logging.Info("Starting ManagementChannel bidirectional streaming")

	// Create cancellable context for connection management
	mcs.connectionCtx, mcs.connectionCancel = context.WithCancel(ctx)

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

// maintainConnection handles connection lifecycle and reconnection
func (mcs *ManagementChannelService) maintainConnection() {
	retryDelay := 5 * time.Second
	maxRetryDelay := 30 * time.Second

	for {
		select {
		case <-mcs.connectionCtx.Done():
			logging.Info("Management channel connection context cancelled")
			return
		default:
			if err := mcs.establishConnection(); err != nil {
				logging.Error("Management channel connection failed: %v, retrying in %v", err, retryDelay)

				// Exponential backoff with max delay
				time.Sleep(retryDelay)
				if retryDelay < maxRetryDelay {
					retryDelay = retryDelay * 2
				}
				continue
			}

			// Reset retry delay on successful connection
			retryDelay = 5 * time.Second
		}
	}
}

// establishConnection establishes the ManagementChannel stream
func (mcs *ManagementChannelService) establishConnection() error {
	logging.Info("Establishing ManagementChannel stream with CloudShip")

	// First ensure the underlying lighthouse client is connected
	if !mcs.lighthouseClient.IsConnected() {
		logging.Info("LighthouseClient not connected, attempting reconnection...")
		if err := mcs.lighthouseClient.Reconnect(); err != nil {
			logging.Error("Failed to reconnect lighthouse client: %v", err)
			return fmt.Errorf("failed to reconnect lighthouse client: %w", err)
		}
		logging.Info("Successfully reconnected LighthouseClient")
	} else {
		logging.Debug("LighthouseClient already connected, proceeding with stream creation")
	}

	// Create the management channel stream
	stream, err := mcs.lighthouseClient.ManagementChannel(mcs.connectionCtx)
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

	// Start goroutines for sending and receiving
	errChan := make(chan error, 2)

	// Goroutine for receiving messages from CloudShip
	go func() {
		errChan <- mcs.receiveMessages(stream)
	}()

	// Wait for error or context cancellation
	select {
	case <-mcs.connectionCtx.Done():
		// Clear stream reference on context cancellation
		mcs.currentStream = nil
		return mcs.connectionCtx.Err()
	case err := <-errChan:
		// Clear stream reference on error
		mcs.currentStream = nil
		if err != nil {
			return fmt.Errorf("management channel error: %w", err)
		}
		return nil
	}
}

// receiveMessages handles incoming messages from CloudShip
func (mcs *ManagementChannelService) receiveMessages(stream proto.LighthouseService_ManagementChannelClient) error {
	// Create heartbeat ticker to detect broken connections
	// Reduced frequency to avoid "too_many_pings" errors from CloudShip
	heartbeatTicker := time.NewTicker(5 * time.Minute)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-mcs.connectionCtx.Done():
			return mcs.connectionCtx.Err()

		case <-heartbeatTicker.C:
			// Send heartbeat (reuse registration message) to detect broken connections
			logging.Debug("Sending heartbeat to CloudShip")
			heartbeat := &proto.ManagementMessage{
				RequestId:       fmt.Sprintf("heartbeat_%d", time.Now().Unix()),
				RegistrationKey: mcs.registrationKey,
				IsResponse:      false,
				Success:         true,
				Message: &proto.ManagementMessage_StationRegistration{
					StationRegistration: &proto.StationRegistrationMessage{
						RegistrationKey: mcs.registrationKey,
					},
				},
			}

			if err := stream.Send(heartbeat); err != nil {
				logging.Error("Failed to send heartbeat: %v", err)
				return fmt.Errorf("heartbeat send error: %w", err)
			}

		default:
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
			case <-time.After(5 * time.Second):
				// Continue loop to check heartbeat and context with longer timeout
				continue
			}
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
	default:
		return "Unknown"
	}
}
