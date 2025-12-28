package dotprompt

import (
	"time"

	"github.com/firebase/genkit/go/ai"
	"station/pkg/models"
)

// DotpromptConfig represents the complete dotprompt configuration
type DotpromptConfig struct {
	Model        string                 `yaml:"model"`
	Config       GenerationConfig       `yaml:"config,omitempty"`
	Input        InputConfig            `yaml:"input,omitempty"`
	Output       OutputConfig           `yaml:"output,omitempty"`
	Tools        []string               `yaml:"tools,omitempty"`
	Metadata     AgentMetadata          `yaml:"metadata"`
	Station      ExecutionMetadata      `yaml:"station,omitempty"`
	Sandbox      *SandboxConfig         `yaml:"sandbox,omitempty"`
	CustomFields map[string]interface{} `yaml:",inline"`
}

type SandboxConfig struct {
	// Mode selects sandbox type: "compute" (V1/Dagger, default) or "code" (V2/Docker persistent)
	Mode string `yaml:"mode,omitempty"`

	// Session scoping for code mode: "workflow" (share across steps) or "agent" (per-agent-run)
	Session string `yaml:"session,omitempty"`

	// Runtime environment: "python", "node", or "bash"
	Runtime string `yaml:"runtime,omitempty"`

	// Image overrides the default container image
	Image string `yaml:"image,omitempty"`

	// TimeoutSeconds for command execution
	TimeoutSeconds int `yaml:"timeout_seconds,omitempty"`

	// MaxStdoutBytes truncates output after this limit
	MaxStdoutBytes int `yaml:"max_stdout_bytes,omitempty"`

	// AllowNetwork enables network access in sandbox
	AllowNetwork bool `yaml:"allow_network,omitempty"`

	// PipPackages to install (Python runtime)
	PipPackages []string `yaml:"pip_packages,omitempty"`

	// NpmPackages to install (Node runtime)
	NpmPackages []string `yaml:"npm_packages,omitempty"`

	// Limits for code mode resource constraints
	Limits *SandboxLimits `yaml:"limits,omitempty"`
}

// SandboxLimits defines resource constraints for code mode
type SandboxLimits struct {
	TimeoutSeconds   int   `yaml:"timeout_seconds,omitempty"`
	MaxFileSizeBytes int64 `yaml:"max_file_size_bytes,omitempty"`
	MaxFiles         int   `yaml:"max_files,omitempty"`
	MaxStdoutBytes   int   `yaml:"max_stdout_bytes,omitempty"`
}

func (s *SandboxConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var str string
	if err := unmarshal(&str); err == nil {
		s.Runtime = str
		return nil
	}

	type plain SandboxConfig
	return unmarshal((*plain)(s))
}

// GenerationConfig contains model generation parameters
type GenerationConfig struct {
	// All temperature configuration removed for gpt-5 compatibility
}

// InputConfig defines input schema for the agent
type InputConfig struct {
	Schema map[string]interface{} `yaml:"schema,omitempty"`
}

// OutputConfig defines output schema for the agent
type OutputConfig struct {
	Format string                 `yaml:"format,omitempty"`
	Schema map[string]interface{} `yaml:"schema,omitempty"`
}

// AgentMetadata contains agent metadata
type AgentMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Version     string `yaml:"version,omitempty"`
	AgentID     int64  `yaml:"agent_id,omitempty"`
	MaxSteps    int    `yaml:"max_steps,omitempty"`
}

// ExecutionMetadata contains execution configuration for Station agents
type ExecutionMetadata struct {
	MaxSteps       int    `yaml:"max_steps,omitempty"`
	TimeoutSeconds int    `yaml:"timeout_seconds,omitempty"`
	MaxRetries     int    `yaml:"max_retries,omitempty"`
	Priority       string `yaml:"priority,omitempty"`
}

// ExecutionRequest represents a request to execute an agent
type ExecutionRequest struct {
	Task       string                 `json:"task"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Config     *GenerationConfig      `json:"config,omitempty"`
}

// ExecutionResponse represents the response from agent execution
type ExecutionResponse struct {
	Success        bool                   `json:"success"`
	Response       string                 `json:"response"`
	ToolCalls      *models.JSONArray      `json:"tool_calls,omitempty"`
	ExecutionSteps *models.JSONArray      `json:"execution_steps,omitempty"`
	Duration       time.Duration          `json:"duration"`
	ModelName      string                 `json:"model_name"`
	StepsUsed      int                    `json:"steps_used"`
	ToolsUsed      int                    `json:"tools_used"`
	TokenUsage     map[string]interface{} `json:"token_usage,omitempty"`
	Error          string                 `json:"error,omitempty"`
	RawResponse    *ai.ModelResponse      `json:"-"` // Don't serialize raw response
	// Metadata from dotprompt for data ingestion classification
	App     string `json:"app,omitempty"`      // CloudShip data ingestion app classification
	AppType string `json:"app_type,omitempty"` // CloudShip data ingestion app_type classification
}

// ToolMapping represents the mapping between agent tools and MCP configs
type ToolMapping struct {
	MCPConfigName string   `json:"mcp_config_name"`
	ServerName    string   `json:"server_name"`
	AssignedTools []string `json:"assigned_tools"`
}

// Message represents a single message in a multi-role prompt
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"` // The message content
}

// ParsedPrompt represents a parsed dotprompt with separated config and messages
type ParsedPrompt struct {
	Config      *DotpromptConfig `json:"config"`
	Messages    []*Message       `json:"messages"`
	IsMultiRole bool             `json:"is_multi_role"`
}

// RenderContext contains all variables available for template rendering
type RenderContext struct {
	UserInput     string                 `json:"user_input"`     // The task from user request
	AgentName     string                 `json:"agent_name"`     // Agent name from database
	Environment   string                 `json:"environment"`    // Environment name
	UserVariables map[string]interface{} `json:"user_variables"` // Custom user-defined variables
}

// AutomaticVariable represents built-in variables always available
type AutomaticVariable struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Type         string      `json:"type"`
	DefaultValue interface{} `json:"default_value,omitempty"`
}

// GetAutomaticVariables returns the list of built-in variables
func GetAutomaticVariables() []AutomaticVariable {
	return []AutomaticVariable{
		{
			Name:        "userInput",
			Description: "The task provided by the user",
			Type:        "string",
		},
		{
			Name:        "agentName",
			Description: "The name of the executing agent",
			Type:        "string",
		},
		{
			Name:        "environment",
			Description: "The environment where the agent is running",
			Type:        "string",
		},
	}
}
