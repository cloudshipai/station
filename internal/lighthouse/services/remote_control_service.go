package services

import (
	"context"
	"station/internal/db/repositories"
	"station/internal/lighthouse"
	"station/internal/logging"
	"station/internal/services"
)

// RemoteControlConfig holds configuration for the remote control service
type RemoteControlConfig struct {
	RegistrationKey string
	Environment     string
	StationName     string   // v2: user-defined unique station name
	StationTags     []string // v2: optional tags for filtering
}

// RemoteControlService orchestrates server mode remote control functionality
// This service manages the ManagementChannel and command handling for CloudShip remote control.
type RemoteControlService struct {
	lighthouseClient  *lighthouse.LighthouseClient
	managementChannel *ManagementChannelService // ManagementChannel for bidirectional RPC
	managementHandler *ManagementHandlerService // Handles management commands
	registrationKey   string
	environment       string
}

// NewRemoteControlService creates a new remote control service
func NewRemoteControlService(
	lighthouseClient *lighthouse.LighthouseClient,
	agentService services.AgentServiceInterface,
	repos *repositories.Repositories,
	registrationKey string,
	environment string,
) *RemoteControlService {
	// Create with empty v2 config for backward compatibility
	return NewRemoteControlServiceWithConfig(
		lighthouseClient,
		agentService,
		repos,
		RemoteControlConfig{
			RegistrationKey: registrationKey,
			Environment:     environment,
		},
	)
}

// NewRemoteControlServiceWithConfig creates a new remote control service with full v2 config
func NewRemoteControlServiceWithConfig(
	lighthouseClient *lighthouse.LighthouseClient,
	agentService services.AgentServiceInterface,
	repos *repositories.Repositories,
	config RemoteControlConfig,
) *RemoteControlService {
	var managementChannel *ManagementChannelService

	// Use v2 constructor if station name is provided
	if config.StationName != "" {
		managementChannel = NewManagementChannelServiceV2(
			lighthouseClient,
			nil, // Will set handler after creating it
			ManagementChannelConfig{
				RegistrationKey: config.RegistrationKey,
				StationName:     config.StationName,
				StationTags:     config.StationTags,
			},
		)
	} else {
		// Use v1 constructor for backward compatibility
		managementChannel = NewManagementChannelService(
			lighthouseClient,
			nil, // Will set handler after creating it
			config.RegistrationKey,
		)
	}

	// Create management handler service with management channel reference for SendRun
	managementHandler := NewManagementHandlerServiceWithChannel(agentService, repos, lighthouseClient, config.RegistrationKey, managementChannel)

	// Set the handler in the management channel
	managementChannel.managementHandler = managementHandler

	// Wire up reconnect callback: when heartbeat is rejected with "not registered",
	// the LighthouseClient calls this callback to force ManagementChannel reconnection.
	// This fixes the bug where ManagementChannel dies silently but heartbeat keeps running.
	lighthouseClient.SetReconnectCallback(func() {
		logging.Info("Reconnect callback triggered by heartbeat rejection")
		managementChannel.ForceReconnect()
	})

	// Legacy streaming service disabled - CloudShip team only implemented ManagementChannel
	// metricsService := NewMetricsService()
	// commandHandler := NewCommandHandlerService(agentService, metricsService, repos, lighthouseClient)
	// streamingService := NewStreamingService(
	// 	lighthouseClient,
	// 	commandHandler,
	// 	metricsService,
	// 	config.RegistrationKey,
	// 	config.Environment,
	// )

	return &RemoteControlService{
		lighthouseClient:  lighthouseClient,
		managementChannel: managementChannel,
		managementHandler: managementHandler,
		registrationKey:   config.RegistrationKey,
		environment:       config.Environment,
	}
}

// Start initializes and starts the remote control services
func (rcs *RemoteControlService) Start(ctx context.Context) error {
	logging.Info("Starting Station remote control service")

	// Verify we have a Lighthouse client (will attempt registration/reconnection)
	if rcs.lighthouseClient == nil {
		logging.Info("No Lighthouse client available - remote control functionality will be disabled")
		return nil
	}

	// Verify supported mode (serve only - stdio mode doesn't connect to platform)
	if rcs.lighthouseClient.GetMode() != lighthouse.ModeServe {
		logging.Info("Station not in serve mode - remote control functionality will be disabled")
		return nil
	}

	logging.Info("Starting bidirectional management channel for CloudShip remote control")

	// Start the new ManagementChannel service
	if err := rcs.managementChannel.Start(ctx); err != nil {
		return err
	}

	// Legacy streaming service disabled - CloudShip team only implemented ManagementChannel
	// logging.Info("Starting legacy bidirectional streaming for backward compatibility")
	// if err := rcs.streamingService.Start(ctx); err != nil {
	// 	logging.Error("Failed to start legacy streaming service: %v (continuing with ManagementChannel only)", err)
	// }

	logging.Info("Station remote control service started successfully")
	return nil
}

// Stop gracefully shuts down the remote control services
func (rcs *RemoteControlService) Stop() error {
	logging.Info("Stopping Station remote control service")

	if rcs.managementChannel != nil {
		if err := rcs.managementChannel.Stop(); err != nil {
			logging.Error("Error stopping management channel service: %v", err)
		}
	}

	logging.Info("Station remote control service stopped")
	return nil
}

// IsConnected returns whether the remote control service has an active connection
func (rcs *RemoteControlService) IsConnected() bool {
	return rcs.lighthouseClient != nil && rcs.lighthouseClient.IsConnected()
}

// GetMemoryClient returns the memory client for CloudShip memory integration
// This allows the agent execution engine to use CloudShip memory when connected
func (rcs *RemoteControlService) GetMemoryClient() *lighthouse.MemoryClient {
	if rcs.managementChannel != nil {
		return rcs.managementChannel.GetMemoryClient()
	}
	return nil
}
