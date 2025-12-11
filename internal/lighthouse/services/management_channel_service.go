package services

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"time"

	"station/internal/lighthouse"
	"station/internal/lighthouse/proto"
	"station/internal/logging"
)

// Helper functions for runtime info (at package level to avoid method receiver issues)
func getHostnameHelper() (string, error) {
	return os.Hostname()
}

func getRuntimeOS() string {
	return runtime.GOOS
}

func getRuntimeArch() string {
	return runtime.GOARCH
}

func getRuntimeVersion() string {
	return runtime.Version()
}

// RegistrationState represents the current registration status
type RegistrationState int

const (
	RegistrationStateUnregistered RegistrationState = iota
	RegistrationStateRegistered
	RegistrationStateRejectedLimit // Rejected due to 1:1 limit - should not retry
)

// ManagementChannelService handles the bidirectional ManagementChannel streaming
type ManagementChannelService struct {
	lighthouseClient  *lighthouse.LighthouseClient
	managementHandler *ManagementHandlerService
	registrationKey   string
	stationName       string   // v2: user-defined unique station name
	stationTags       []string // v2: user-defined tags for filtering
	connectionCtx     context.Context
	connectionCancel  context.CancelFunc
	currentStream     proto.LighthouseService_ManagementChannelClient
	registrationState RegistrationState
	memoryClient      *lighthouse.MemoryClient                   // For CloudShip memory integration
	useV2Auth         bool                                       // True if using v2 StationAuth flow
	stationID         string                                     // Station ID returned by AuthResult (v2)
	orgID             string                                     // Organization ID returned by AuthResult (v2) for telemetry
	heartbeatInterval time.Duration                              // Heartbeat interval from AuthResult (v2)
	onAuthSuccess     func(stationID, stationName, orgID string) // Callback when auth succeeds
}

// ManagementChannelConfig holds configuration for the management channel service
type ManagementChannelConfig struct {
	RegistrationKey string
	StationName     string   // v2: required for v2 auth flow
	StationTags     []string // v2: optional user-defined tags
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
		registrationState: RegistrationStateUnregistered,
		memoryClient:      lighthouse.NewMemoryClient(2 * time.Second), // 2 second timeout per PRD
		useV2Auth:         false,                                       // Default to v1 for backward compatibility
	}
}

// NewManagementChannelServiceV2 creates a new management channel service with v2 auth support
func NewManagementChannelServiceV2(
	lighthouseClient *lighthouse.LighthouseClient,
	managementHandler *ManagementHandlerService,
	config ManagementChannelConfig,
) *ManagementChannelService {
	// Use v2 auth if station name is provided
	useV2 := config.StationName != ""

	return &ManagementChannelService{
		lighthouseClient:  lighthouseClient,
		managementHandler: managementHandler,
		registrationKey:   config.RegistrationKey,
		stationName:       config.StationName,
		stationTags:       config.StationTags,
		registrationState: RegistrationStateUnregistered,
		memoryClient:      lighthouse.NewMemoryClient(2 * time.Second),
		useV2Auth:         useV2,
		heartbeatInterval: 60 * time.Second, // Default, will be updated by AuthResult
	}
}

// GetMemoryClient returns the memory client for CloudShip memory integration
func (mcs *ManagementChannelService) GetMemoryClient() *lighthouse.MemoryClient {
	return mcs.memoryClient
}

// GetStationID returns the station ID assigned by CloudShip after authentication
func (mcs *ManagementChannelService) GetStationID() string {
	return mcs.stationID
}

// GetOrgID returns the organization ID from CloudShip for telemetry filtering
func (mcs *ManagementChannelService) GetOrgID() string {
	return mcs.orgID
}

// GetStationName returns the station name
func (mcs *ManagementChannelService) GetStationName() string {
	return mcs.stationName
}

// SetOnAuthSuccess sets a callback to be invoked when CloudShip auth succeeds
// This is used to update TelemetryService with station/org info for trace filtering
func (mcs *ManagementChannelService) SetOnAuthSuccess(callback func(stationID, stationName, orgID string)) {
	mcs.onAuthSuccess = callback
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

	// First ensure the underlying lighthouse client is connected (gRPC connection only)
	if !mcs.lighthouseClient.IsConnected() {
		logging.Info("LighthouseClient not connected, attempting connection...")
		if err := mcs.lighthouseClient.ConnectOnly(); err != nil {
			logging.Error("Failed to connect lighthouse client: %v", err)
			return fmt.Errorf("failed to connect lighthouse client: %w", err)
		}
		logging.Info("Successfully connected LighthouseClient")
	} else {
		logging.Debug("LighthouseClient already connected")
	}

	// V2 AUTH FLOW: Skip RegisterStation entirely - use StationAuth on ManagementChannel
	if mcs.useV2Auth {
		logging.Info("🚀 Using v2 auth flow: station_name=%s tags=%v", mcs.stationName, mcs.stationTags)
		// V2 flow: go directly to ManagementChannel with StationAuth
		// No RegisterStation RPC needed - auth happens via StationAuth message
	} else {
		// V1 LEGACY FLOW: Require RegisterStation before ManagementChannel
		// ✅ REQUIRED: Verify RegisterStation completed successfully before opening stream
		// CloudShip requirement: Station MUST call RegisterStation gRPC before ManagementChannel
		if !mcs.lighthouseClient.IsRegistered() {
			logging.Info("Station not registered (v1 flow), attempting registration before opening ManagementChannel...")
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
			logging.Info("✅ Station successfully registered (v1), proceeding with ManagementChannel")
		} else {
			logging.Debug("✅ Station already registered (v1), proceeding with ManagementChannel stream creation")
		}
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

	// Set stream on memory client for CloudShip memory integration
	if mcs.memoryClient != nil {
		mcs.memoryClient.SetStream(stream, mcs.registrationKey)
		logging.Debug("Memory client stream configured for CloudShip integration")
	}

	// Send auth message based on version
	if mcs.useV2Auth {
		// v2 auth flow: send StationAuth with full station identity
		if err := mcs.sendV2Auth(stream); err != nil {
			mcs.currentStream = nil
			return fmt.Errorf("failed to send v2 auth: %w", err)
		}
	} else {
		// v1 auth flow: send StationRegistrationMessage (legacy)
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

		logging.Info("Sent station registration to CloudShip (v1 flow)")
	}

	// Start goroutine for receiving messages from CloudShip
	// This goroutine will run independently and handle connection errors
	go func() {
		if err := mcs.receiveMessages(stream); err != nil {
			logging.Error("Management channel receive error: %v", err)
			// Clear stream reference on error
			mcs.currentStream = nil
			// Clear memory client stream
			if mcs.memoryClient != nil {
				mcs.memoryClient.ClearStream()
			}
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

			// Handle responses vs requests
			if msg.IsResponse {
				// Route responses to appropriate handlers
				mcs.handleResponse(msg)
			} else {
				// Process incoming requests
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

// ForceReconnect clears the current stream to trigger reconnection.
// This is called by the LighthouseClient when heartbeat is rejected with "not registered",
// indicating the ManagementChannel is dead but the station didn't detect it.
func (mcs *ManagementChannelService) ForceReconnect() {
	logging.Info("ForceReconnect called - clearing current stream to trigger reconnection")

	// Clear the current stream - this will cause maintainConnection loop to reconnect
	if mcs.currentStream != nil {
		mcs.currentStream = nil
	}

	// Clear memory client stream
	if mcs.memoryClient != nil {
		mcs.memoryClient.ClearStream()
	}

	// Update global status to disconnected
	lighthouse.SetConnected(false, "")
	lighthouse.SetRegistered(false, "")

	// Reset registration state so we can reconnect
	mcs.registrationState = RegistrationStateUnregistered

	logging.Info("ManagementChannel stream cleared - reconnection will be attempted by maintainConnection loop")
}

// handleResponse routes responses to appropriate handlers
func (mcs *ManagementChannelService) handleResponse(msg *proto.ManagementMessage) {
	switch resp := msg.Message.(type) {
	case *proto.ManagementMessage_AuthResult:
		// v2 auth response
		mcs.handleAuthResult(resp.AuthResult)
	case *proto.ManagementMessage_Disconnect:
		// Server-initiated disconnect
		mcs.handleDisconnect(resp.Disconnect)
	case *proto.ManagementMessage_GetMemoryContextResponse:
		// Route memory context responses to the memory client
		if mcs.memoryClient != nil {
			mcs.memoryClient.HandleResponse(msg.RequestId, resp.GetMemoryContextResponse)
		} else {
			logging.Info("Received GetMemoryContextResponse but memory client not initialized")
		}
	case *proto.ManagementMessage_SendRunResponse:
		// SendRun responses are fire-and-forget, just log
		logging.Debug("Received SendRunResponse for request: %s, success: %v", msg.RequestId, msg.Success)
	default:
		logging.Debug("Received unhandled response type for request: %s", msg.RequestId)
	}
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
	case *proto.ManagementMessage_StationAuth:
		return "StationAuth"
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

// ============================================================================
// V2 AUTH FLOW METHODS
// ============================================================================

// sendV2Auth sends the StationAuth message for v2 authentication
func (mcs *ManagementChannelService) sendV2Auth(stream proto.LighthouseService_ManagementChannelClient) error {
	logging.Info("Sending v2 StationAuth: name=%s tags=%v", mcs.stationName, mcs.stationTags)

	// Build StationAuth message with full station identity
	authMsg := &proto.ManagementMessage{
		RequestId:       fmt.Sprintf("auth_%d", time.Now().Unix()),
		RegistrationKey: mcs.registrationKey,
		IsResponse:      false,
		Success:         true,
		Message: &proto.ManagementMessage_StationAuth{
			StationAuth: mcs.buildStationAuth(),
		},
	}

	if err := stream.Send(authMsg); err != nil {
		return fmt.Errorf("failed to send StationAuth: %w", err)
	}

	logging.Info("Sent StationAuth to CloudShip (v2 flow), waiting for AuthResult...")
	return nil
}

// buildStationAuth constructs the StationAuth protobuf message
func (mcs *ManagementChannelService) buildStationAuth() *proto.StationAuth {
	hostname := mcs.getHostname()
	version := mcs.getVersion()

	return &proto.StationAuth{
		RegistrationKey: mcs.registrationKey,
		Name:            mcs.stationName,
		Tags:            mcs.stationTags,
		Hostname:        hostname,
		Version:         version,
		Os:              mcs.getOS(),
		Arch:            mcs.getArch(),
		Environment:     mcs.lighthouseClient.GetEnvironment(),
		// Note: agents, tools, and mcp_servers can be added later if needed
		// For now, CloudShip can fetch these via ListAgents/ListTools
		HardwareInfo: map[string]string{
			"go_version": mcs.getGoVersion(),
		},
	}
}

// handleAuthResult processes the AuthResult response from CloudShip
func (mcs *ManagementChannelService) handleAuthResult(result *proto.AuthResult) {
	if result.Success {
		mcs.stationID = result.StationId
		mcs.orgID = result.OrgId // Capture org ID for telemetry filtering
		if result.HeartbeatIntervalMs > 0 {
			mcs.heartbeatInterval = time.Duration(result.HeartbeatIntervalMs) * time.Millisecond
		}
		mcs.registrationState = RegistrationStateRegistered

		logging.Info("V2 auth successful: station_id=%s name=%s org_id=%s heartbeat_interval=%v replaced_existing=%v",
			result.StationId, result.Name, result.OrgId, mcs.heartbeatInterval, result.ReplacedExisting)

		// Update global lighthouse status
		lighthouse.SetConnected(true, "cloudship")
		lighthouse.SetRegistered(true, mcs.registrationKey)

		// Notify callback (used for telemetry service to get org_id)
		if mcs.onAuthSuccess != nil {
			mcs.onAuthSuccess(mcs.stationID, mcs.stationName, mcs.orgID)
		}
	} else {
		logging.Error("V2 auth failed: %s", result.Error)

		// Check if this is a limit rejection
		if strings.Contains(result.Error, "limit") || strings.Contains(result.Error, "max_stations") {
			mcs.registrationState = RegistrationStateRejectedLimit
		}

		// Clear stream on auth failure
		mcs.currentStream = nil
		if mcs.memoryClient != nil {
			mcs.memoryClient.ClearStream()
		}
	}
}

// handleDisconnect processes server-initiated disconnect messages
func (mcs *ManagementChannelService) handleDisconnect(disconnect *proto.Disconnect) {
	logging.Info("Received disconnect from CloudShip: reason=%s code=%v should_reconnect=%v delay=%dms",
		disconnect.Reason, disconnect.Code, disconnect.ShouldReconnect, disconnect.ReconnectDelayMs)

	// Clear current connection state
	mcs.currentStream = nil
	if mcs.memoryClient != nil {
		mcs.memoryClient.ClearStream()
	}

	// Update global status
	lighthouse.SetConnected(false, "")
	lighthouse.SetRegistered(false, "")

	// Handle different disconnect reasons
	switch disconnect.Code {
	case proto.DisconnectReason_DISCONNECT_REASON_REPLACED:
		// Another station with same name connected - enter rejected state briefly
		logging.Info("Station was replaced by another connection with same name")
		mcs.registrationState = RegistrationStateRejectedLimit
	case proto.DisconnectReason_DISCONNECT_REASON_AUTH_FAILED:
		logging.Error("Authentication failed - check registration key")
		mcs.registrationState = RegistrationStateRejectedLimit
	case proto.DisconnectReason_DISCONNECT_REASON_KEY_REVOKED:
		logging.Error("Registration key was revoked - contact administrator")
		mcs.registrationState = RegistrationStateRejectedLimit
	case proto.DisconnectReason_DISCONNECT_REASON_LIMIT_REACHED:
		logging.Error("Max stations limit reached for registration key")
		mcs.registrationState = RegistrationStateRejectedLimit
	case proto.DisconnectReason_DISCONNECT_REASON_SHUTDOWN:
		logging.Info("CloudShip is shutting down, will attempt reconnection")
		mcs.registrationState = RegistrationStateUnregistered
	case proto.DisconnectReason_DISCONNECT_REASON_HEARTBEAT_TIMEOUT:
		logging.Info("Heartbeat timeout, will attempt reconnection")
		mcs.registrationState = RegistrationStateUnregistered
	default:
		mcs.registrationState = RegistrationStateUnregistered
	}

	// If reconnection is suggested, the maintainConnection loop will handle it
	// based on the registrationState
}

// Helper methods for getting station metadata
func (mcs *ManagementChannelService) getHostname() string {
	// Import os package - will be done at top of file
	hostname, err := getHostnameHelper()
	if err != nil {
		return "unknown"
	}
	return hostname
}

func (mcs *ManagementChannelService) getVersion() string {
	return "v0.11.0" // TODO: get from version package
}

func (mcs *ManagementChannelService) getOS() string {
	return getRuntimeOS()
}

func (mcs *ManagementChannelService) getArch() string {
	return getRuntimeArch()
}

func (mcs *ManagementChannelService) getGoVersion() string {
	return getRuntimeVersion()
}
