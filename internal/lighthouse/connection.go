package lighthouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"station/internal/lighthouse/proto"
	"station/internal/logging"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

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

	// Configure keep-alive with CloudShip-compatible settings
	// CloudShip considers frequent pings as spam, so use conservative settings
	keepaliveParams := keepalive.ClientParameters{
		Time:                60 * time.Second, // Ping every 60s (much more conservative)
		Timeout:             10 * time.Second, // Wait 10s for ping response
		PermitWithoutStream: false,            // Only ping when there are active streams
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

// IsConnected returns true if connected to Lighthouse
func (lc *LighthouseClient) IsConnected() bool {
	if lc == nil || lc.conn == nil {
		return false
	}
	return lc.conn.GetState().String() == "READY"
}

// Reconnect re-establishes connection and registration with CloudShip Lighthouse
func (lc *LighthouseClient) Reconnect() error {
	if lc == nil {
		return fmt.Errorf("lighthouse client is nil")
	}

	// Close existing connection if present
	if lc.conn != nil {
		lc.conn.Close()
		lc.conn = nil
		lc.client = nil
		lc.registered = false
	}

	// Attempt reconnection
	if err := lc.connect(); err != nil {
		return fmt.Errorf("failed to reconnect: %v", err)
	}

	// Attempt re-registration
	if err := lc.register(); err != nil {
		return fmt.Errorf("failed to re-register: %v", err)
	}

	logging.Info("Successfully reconnected and re-registered with CloudShip Lighthouse")
	return nil
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
