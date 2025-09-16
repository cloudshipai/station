package lighthouse

import (
	"context"
	"fmt"
	"station/internal/lighthouse/proto"
	"station/internal/logging"
	"station/pkg/types"
)

// SendRun sends agent run data to CloudShip (async, buffered)
func (lc *LighthouseClient) SendRun(runData *types.AgentRun, environment string, labels map[string]string) {
	if !lc.IsRegistered() {
		return // Graceful degradation - no cloud integration
	}

	req := &proto.SendRunRequest{
		RegistrationKey: lc.config.RegistrationKey,
		Environment:     environment,
		Mode:            convertDeploymentModeToProto(lc.mode),
		Source:          proto.RunSource_RUN_SOURCE_ANALYTICS, // Station completed run data
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

	// Send to Data Ingestion service if finops preset is used
	if runData.OutputSchemaPreset == "finops" {
		logging.Debug("Detected finops preset - structured data will be sent via regular lighthouse telemetry with preset metadata")
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
		Source:          proto.RunSource_RUN_SOURCE_CLI_SNAPSHOT, // CLI ephemeral snapshot
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
