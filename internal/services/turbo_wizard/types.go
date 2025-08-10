package turbo_wizard

import "station/internal/theme"

// TurboWizardState represents the current state of the wizard
type TurboWizardState int

const (
	StateShowingBlocks TurboWizardState = iota
	StateConfiguringServer
	StateEditingField
	StateSelectingEnvironment
	StateReviewConfig
	StateCompleted
)

// MCPTransportType represents the type of MCP transport
type MCPTransportType string

const (
	TransportSTDIO  MCPTransportType = "stdio"
	TransportDocker MCPTransportType = "docker"
	TransportHTTP   MCPTransportType = "http"
	TransportSSE    MCPTransportType = "sse"
)

// ServerConfig represents a configured MCP server with full transport support
type ServerConfig struct {
	Name         string                 `json:"name"`
	Transport    MCPTransportType       `json:"transport"`
	Command      string                 `json:"command,omitempty"`
	Args         []string               `json:"args,omitempty"`
	URL          string                 `json:"url,omitempty"`
	Env          map[string]string      `json:"env,omitempty"`
	DockerMounts []DockerMount          `json:"dockerMounts,omitempty"`
	RawBlock     string                 `json:"rawBlock"`
	Description  string                 `json:"description"`
	RequiredEnv  []EnvironmentVariable  `json:"requiredEnv,omitempty"`
}

// DockerMount represents a Docker mount configuration
type DockerMount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Type     string `json:"type"` // bind, volume, tmpfs
	ReadOnly bool   `json:"readonly,omitempty"`
}

// EnvironmentVariable represents a required environment variable
type EnvironmentVariable struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Type        string `json:"type"` // string, api_key, path, url
	Value       string `json:"value,omitempty"`
}

// MCPServerBlock represents a parsed MCP server configuration block
type MCPServerBlock struct {
	ServerName  string `json:"serverName"`
	Description string `json:"description"`
	RawBlock    string `json:"rawBlock"`
	Transport   MCPTransportType `json:"transport"`
}

// TurboWizardModel implements the TurboTax-style MCP configuration wizard
type TurboWizardModel struct {
	state           TurboWizardState
	blocks          []MCPServerBlock
	selectedBlocks  []bool           // Which blocks user wants to configure
	currentBlock    int              // Current block being shown/configured
	configurations  []ServerConfig   // Completed configurations
	currentConfig   *ServerConfig    // Currently being configured
	configStep      int              // Current step in server configuration
	editingField    string           // Field currently being edited
	fieldValue      string           // Current field value being edited
	environments    []string         // Available environments
	selectedEnv     int              // Selected environment index
	completed       bool
	cancelled       bool
	width           int
	height          int
	themeManager    *theme.ThemeManager // Theme manager for UI styling
}

// ConfigStep represents a step in the configuration process
type ConfigStep struct {
	Name        string
	Description string
	Required    bool
	FieldType   string // text, api_key, path, url, select
	Options     []string // For select fields
}