package lighthouse

import (
	"context"
	"fmt"
	"station/internal/lighthouse/proto"
	"station/internal/logging"
	"time"
)

// ManagementChannel establishes a bidirectional management channel for firewall-friendly communication
func (lc *LighthouseClient) ManagementChannel(ctx context.Context) (proto.LighthouseService_ManagementChannelClient, error) {
	if !lc.IsConnected() {
		return nil, fmt.Errorf("lighthouse client not connected")
	}

	logging.Debug("Establishing bidirectional ManagementChannel stream with CloudShip")

	stream, err := lc.client.ManagementChannel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create ManagementChannel stream: %w", err)
	}

	logging.Info("Successfully established bidirectional ManagementChannel stream with CloudShip")
	return stream, nil
}

// Connect establishes a bidirectional streaming connection for server mode remote control
func (lc *LighthouseClient) Connect(ctx context.Context) (proto.LighthouseService_ConnectClient, error) {
	if !lc.IsConnected() {
		return nil, fmt.Errorf("lighthouse client not connected")
	}

	logging.Debug("Establishing bidirectional Connect stream with CloudShip")

	stream, err := lc.client.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create Connect stream: %w", err)
	}

	logging.Info("Successfully established bidirectional Connect stream with CloudShip")
	return stream, nil
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

	// Health data worker (serve mode and stdio mode)
	if lc.mode == ModeServe || lc.mode == ModeStdio {
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

		// NOTE: Heartbeat worker removed for stdio/serve modes to prevent "too_many_pings" issue
		// Management channel handles connection health and reconnection automatically
		// This eliminates dual heartbeat conflict with management channel keepalives
	}

	// Heartbeat worker (all modes - required for Lighthouse status tracking)
	// Heartbeat is separate from management channel and required to keep station marked as online
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

// sendHeartbeat sends periodic heartbeat to Lighthouse
// Required for all modes to keep station marked as online in CloudShip UI
// Separate from management channel which handles bidirectional communication
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
	} else {
		logging.Debug("Heartbeat sent successfully to CloudShip")
	}
}

// timestampNow is defined in converters.go
