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
	Input        InputConfig           `yaml:"input,omitempty"`
	Output       OutputConfig          `yaml:"output,omitempty"`
	Tools        []string              `yaml:"tools,omitempty"`
	Metadata     AgentMetadata         `yaml:"metadata"`
	Station      ExecutionMetadata     `yaml:"station,omitempty"`
	CustomFields map[string]interface{} `yaml:",inline"`
}

// GenerationConfig contains model generation parameters
type GenerationConfig struct {
	Temperature *float32 `yaml:"temperature,omitempty"`
	MaxTokens   *int     `yaml:"max_tokens,omitempty"`
	TopP        *float32 `yaml:"top_p,omitempty"`
	TopK        *int     `yaml:"top_k,omitempty"`
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
}

// ToolMapping represents the mapping between agent tools and MCP configs
type ToolMapping struct {
	MCPConfigName string   `json:"mcp_config_name"`
	ServerName    string   `json:"server_name"`
	AssignedTools []string `json:"assigned_tools"`
}