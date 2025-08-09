package dotprompt

import (
	"time"

	"station/pkg/models"
)

// DotpromptConfig represents the frontmatter configuration for a Station agent
type DotpromptConfig struct {
	Model    string                 `yaml:"model"`
	Input    InputSchema           `yaml:"input"`
	Output   OutputSchema          `yaml:"output"`
	Tools    []string              `yaml:"tools,omitempty"`    // Tool names/IDs
	Config   GenerationConfig      `yaml:"config,omitempty"`   // Generation parameters
	Metadata AgentMetadata         `yaml:"metadata"`           // Station-specific metadata
	
	// Custom frontmatter - any additional YAML fields
	CustomFields map[string]interface{} `yaml:",inline"`
}

type InputSchema struct {
	Schema map[string]interface{} `yaml:"schema"`
}

type OutputSchema struct {
	Format string                 `yaml:"format"`
	Schema map[string]interface{} `yaml:"schema"`
}

type GenerationConfig struct {
	Temperature  *float32 `yaml:"temperature,omitempty"`
	MaxTokens    *int     `yaml:"max_tokens,omitempty"`
	TopP         *float32 `yaml:"top_p,omitempty"`
	TopK         *int     `yaml:"top_k,omitempty"`
	StopSequences []string `yaml:"stop_sequences,omitempty"`
}

type AgentMetadata struct {
	AgentID         int64     `yaml:"agent_id"`
	Name            string    `yaml:"name"`
	Description     string    `yaml:"description"`
	Domain          string    `yaml:"domain,omitempty"`
	MaxSteps        int64     `yaml:"max_steps"`
	Environment     string    `yaml:"environment"`
	ToolsVersion    string    `yaml:"tools_version"`  // Hash of MCP config when created
	CreatedAt       time.Time `yaml:"created_at"`
	UpdatedAt       time.Time `yaml:"updated_at"`
	Version         string    `yaml:"version"`
	ScheduleEnabled bool      `yaml:"schedule_enabled"`
}

// AgentDotprompt represents a complete agent configuration in dotprompt format
type AgentDotprompt struct {
	Config   DotpromptConfig `json:"config"`
	Template string          `json:"template"`
	FilePath string          `json:"file_path"`
}

// ExecutionRequest represents input for dotprompt execution
type ExecutionRequest struct {
	Task       string                 `json:"task"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Config     *GenerationConfig      `json:"config,omitempty"` // Override default config
}

// ExecutionResponse represents the result from dotprompt execution
type ExecutionResponse struct {
	Success        bool                     `json:"success"`
	Response       string                   `json:"response"`
	ToolCalls      *models.JSONArray        `json:"tool_calls,omitempty"`
	ExecutionSteps *models.JSONArray        `json:"execution_steps,omitempty"`
	Duration       time.Duration            `json:"duration"`
	TokenUsage     map[string]interface{}   `json:"token_usage,omitempty"`
	ModelName      string                   `json:"model_name"`
	StepsUsed      int                      `json:"steps_used"`
	ToolsUsed      int                      `json:"tools_used"`
	Error          string                   `json:"error,omitempty"`
	RawResponse    interface{}              `json:"-"` // Keep raw GenKit response for debugging
}

// ToolMapping represents the mapping between dotprompt tools and MCP servers
type ToolMapping struct {
	ToolName      string `json:"tool_name"`
	ServerName    string `json:"server_name"`
	MCPServerID   string `json:"mcp_server_id"`
	ServerType    string `json:"server_type"`
	ConfigPath    string `json:"config_path"`
	Environment   string `json:"environment"`
}

// AgentBundle combines dotprompt with tool mappings for portability
type AgentBundle struct {
	Dotprompt    AgentDotprompt            `json:"dotprompt"`
	ToolMappings []ToolMapping             `json:"tool_mappings"`
	MCPServers   map[string]MCPServerInfo  `json:"mcp_servers"`
	Environment  string                    `json:"environment"`
	Version      string                    `json:"version"`
	ExportedAt   time.Time                 `json:"exported_at"`
}

// MCPServerInfo contains MCP server configuration for portability
type MCPServerInfo struct {
	Name        string `json:"name"`
	ConfigPath  string `json:"config_path"`
	ServerType  string `json:"server_type"`
	Environment string `json:"environment"`
}

// ConversionOptions controls how agents are converted to/from dotprompt
type ConversionOptions struct {
	IncludeMetadata   bool `json:"include_metadata"`
	PreserveFormatting bool `json:"preserve_formatting"`
	GenerateToolDocs   bool `json:"generate_tool_docs"` // Include tool descriptions in template
	UseMinimalSchema   bool `json:"use_minimal_schema"` // Simplified input/output schemas
}