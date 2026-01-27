package heartbeat

import (
	"time"

	"station/pkg/harness/prompt"
)

// ConfigFromAgentConfig extracts heartbeat configuration from an agent config
func ConfigFromAgentConfig(cfg *prompt.AgentConfig) *prompt.HeartbeatConfig {
	if cfg == nil || cfg.Harness == nil || cfg.Harness.Heartbeat == nil {
		return nil
	}
	return cfg.Harness.Heartbeat
}

// GetInterval returns the heartbeat interval, with default fallback
func GetInterval(cfg *prompt.HeartbeatConfig) time.Duration {
	if cfg == nil || cfg.Every == "" {
		return 30 * time.Minute
	}

	d, err := time.ParseDuration(cfg.Every)
	if err != nil {
		return 30 * time.Minute
	}
	return d
}

// IsEnabled checks if heartbeat is enabled in config
func IsEnabled(cfg *prompt.HeartbeatConfig) bool {
	return cfg != nil && cfg.Enabled
}

// GetSessionMode returns the session mode ("main" or "isolated")
func GetSessionMode(cfg *prompt.HeartbeatConfig) string {
	if cfg == nil || cfg.Session == "" {
		return "main"
	}
	return cfg.Session
}

// IsIsolatedSession returns true if heartbeats should run in isolated sessions
func IsIsolatedSession(cfg *prompt.HeartbeatConfig) bool {
	return GetSessionMode(cfg) == "isolated"
}
