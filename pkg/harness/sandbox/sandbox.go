package sandbox

import (
	"context"
	"fmt"
	"io"
	"time"
)

type Mode string

const (
	ModeHost        Mode = "host"
	ModeDocker      Mode = "docker"
	ModeFirecracker Mode = "firecracker"
	ModeGVisor      Mode = "gvisor"
	ModeWASM        Mode = "wasm"
)

type Config struct {
	Mode          Mode              `json:"mode" yaml:"mode"`
	Image         string            `json:"image,omitempty" yaml:"image,omitempty"`
	Resources     ResourceConfig    `json:"resources,omitempty" yaml:"resources,omitempty"`
	Network       NetworkConfig     `json:"network,omitempty" yaml:"network,omitempty"`
	Filesystem    FilesystemConfig  `json:"filesystem,omitempty" yaml:"filesystem,omitempty"`
	Timeout       time.Duration     `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	WorkspacePath string            `json:"workspace_path,omitempty" yaml:"workspace_path,omitempty"`
	Environment   map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
}

type ResourceConfig struct {
	CPU    float64 `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory string  `json:"memory,omitempty" yaml:"memory,omitempty"`
	Disk   string  `json:"disk,omitempty" yaml:"disk,omitempty"`
	PIDs   int64   `json:"pids,omitempty" yaml:"pids,omitempty"`
}

type NetworkConfig struct {
	Enabled      bool     `json:"enabled" yaml:"enabled"`
	AllowedHosts []string `json:"allowed_hosts,omitempty" yaml:"allowed_hosts,omitempty"`
	AllowedPorts []int    `json:"allowed_ports,omitempty" yaml:"allowed_ports,omitempty"`
	DNSServers   []string `json:"dns_servers,omitempty" yaml:"dns_servers,omitempty"`
}

type FilesystemConfig struct {
	ReadOnly  []string `json:"read_only,omitempty" yaml:"read_only,omitempty"`
	ReadWrite []string `json:"read_write,omitempty" yaml:"read_write,omitempty"`
	Denied    []string `json:"denied,omitempty" yaml:"denied,omitempty"`
	TempDir   string   `json:"temp_dir,omitempty" yaml:"temp_dir,omitempty"`
}

type ExecResult struct {
	ExitCode   int           `json:"exit_code"`
	Stdout     string        `json:"stdout"`
	Stderr     string        `json:"stderr"`
	Duration   time.Duration `json:"duration"`
	Killed     bool          `json:"killed,omitempty"`
	KillReason string        `json:"kill_reason,omitempty"`
}

type Metrics struct {
	CPUUsage         float64 `json:"cpu_usage"`
	MemoryUsageBytes int64   `json:"memory_usage_bytes"`
	MemoryMaxBytes   int64   `json:"memory_max_bytes"`
	DiskReadBytes    int64   `json:"disk_read_bytes"`
	DiskWriteBytes   int64   `json:"disk_write_bytes"`
	NetworkRxBytes   int64   `json:"network_rx_bytes"`
	NetworkTxBytes   int64   `json:"network_tx_bytes"`
	ProcessCount     int     `json:"process_count"`
}

type Sandbox interface {
	Create(ctx context.Context) error
	Exec(ctx context.Context, command string, args ...string) (*ExecResult, error)
	ExecWithStdin(ctx context.Context, stdin io.Reader, command string, args ...string) (*ExecResult, error)
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, path string, content []byte, mode uint32) error
	DeleteFile(ctx context.Context, path string) error
	ListFiles(ctx context.Context, path string) ([]FileInfo, error)
	FileExists(ctx context.Context, path string) (bool, error)
	CopyIn(ctx context.Context, hostPath, sandboxPath string) error
	CopyOut(ctx context.Context, sandboxPath, hostPath string) error
	GetMetrics(ctx context.Context) (*Metrics, error)
	Destroy(ctx context.Context) error
	ID() string
	Config() *Config
}

type FileInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	Mode    uint32    `json:"mode"`
	ModTime time.Time `json:"mod_time"`
	IsDir   bool      `json:"is_dir"`
}

type Factory struct {
	DefaultConfig Config
}

func NewFactory(defaults Config) *Factory {
	return &Factory{DefaultConfig: defaults}
}

func (f *Factory) Create(cfg Config) (Sandbox, error) {
	merged := f.mergeConfig(cfg)

	switch merged.Mode {
	case ModeHost, "":
		return NewHostSandbox(merged)
	case ModeDocker:
		return NewDockerSandbox(merged)
	case ModeFirecracker:
		return nil, fmt.Errorf("firecracker sandbox not yet implemented")
	case ModeGVisor:
		return nil, fmt.Errorf("gvisor sandbox not yet implemented")
	case ModeWASM:
		return nil, fmt.Errorf("wasm sandbox not yet implemented")
	default:
		return nil, fmt.Errorf("unknown sandbox mode: %s", merged.Mode)
	}
}

func (f *Factory) mergeConfig(cfg Config) Config {
	merged := cfg

	if merged.Mode == "" {
		merged.Mode = f.DefaultConfig.Mode
		if merged.Mode == "" {
			merged.Mode = ModeHost
		}
	}

	if merged.Timeout == 0 {
		merged.Timeout = f.DefaultConfig.Timeout
		if merged.Timeout == 0 {
			merged.Timeout = 30 * time.Minute
		}
	}

	if merged.Image == "" {
		merged.Image = f.DefaultConfig.Image
	}

	if merged.Resources.CPU == 0 {
		merged.Resources.CPU = f.DefaultConfig.Resources.CPU
	}
	if merged.Resources.Memory == "" {
		merged.Resources.Memory = f.DefaultConfig.Resources.Memory
	}

	return merged
}

func DefaultConfig() Config {
	return Config{
		Mode:    ModeHost,
		Timeout: 30 * time.Minute,
		Resources: ResourceConfig{
			CPU:    2,
			Memory: "4Gi",
			PIDs:   1000,
		},
		Network: NetworkConfig{
			Enabled: false,
		},
		Filesystem: FilesystemConfig{
			TempDir: "/tmp",
		},
	}
}
