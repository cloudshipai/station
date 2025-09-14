package lighthouse

import (
	"context"
	"fmt"
	"station/internal/lighthouse/proto"
	"station/internal/logging"
)

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

// deploymentModeToProto converts deployment mode to proto enum
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
