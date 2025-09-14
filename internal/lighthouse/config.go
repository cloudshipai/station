package lighthouse

import (
	"time"
)

// DeploymentMode represents how Station is currently running
type DeploymentMode int

const (
	ModeUnknown DeploymentMode = iota
	ModeStdio                  // stn stdio - local development
	ModeServe                  // stn serve - team/production server
	ModeCLI                    // all other commands - CI/CD & ephemeral
)

// String returns the string representation of the deployment mode
func (mode DeploymentMode) String() string {
	switch mode {
	case ModeStdio:
		return "stdio"
	case ModeServe:
		return "serve"
	case ModeCLI:
		return "cli"
	default:
		return "unknown"
	}
}

// LighthouseConfig holds configuration for connecting to CloudShip Lighthouse
type LighthouseConfig struct {
	// Core connection settings
	Endpoint        string `yaml:"endpoint"`         // lighthouse.cloudship.ai:443
	RegistrationKey string `yaml:"registration_key"` // CloudShip registration key
	StationID       string `yaml:"station_id"`       // Generated station ID
	TLS             bool   `yaml:"tls"`              // Enable TLS (default: true)

	// Optional settings
	Environment    string        `yaml:"environment"`     // Environment name (default: "default")
	ConnectTimeout time.Duration `yaml:"connect_timeout"` // Connection timeout (default: 10s)
	RequestTimeout time.Duration `yaml:"request_timeout"` // Request timeout (default: 30s)
	KeepAlive      time.Duration `yaml:"keepalive"`       // Keep alive interval (default: 30s)

	// Mode-specific settings
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"` // serve mode heartbeat (default: 30s)
	BufferSize        int           `yaml:"buffer_size"`        // Local buffer size (default: 100)
}

// DefaultLighthouseConfig returns sensible defaults
func DefaultLighthouseConfig() *LighthouseConfig {
	return &LighthouseConfig{
		Endpoint:          "lighthouse.cloudship.ai:443",
		TLS:               true,
		Environment:       "default",
		ConnectTimeout:    10 * time.Second,
		RequestTimeout:    30 * time.Second,
		KeepAlive:         30 * time.Second,
		HeartbeatInterval: 30 * time.Second,
		BufferSize:        100,
	}
}

// ApplyDefaults ensures all config values have sensible defaults
func (cfg *LighthouseConfig) ApplyDefaults() {
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 10 * time.Second
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 30 * time.Second
	}
	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = 30 * time.Second
	}
	if cfg.KeepAlive == 0 {
		cfg.KeepAlive = 30 * time.Second
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 100
	}
}
