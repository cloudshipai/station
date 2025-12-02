package types

import (
	"time"
)

// AgentRun represents a complete agent execution with all metadata
type AgentRun struct {
	ID                 string            `json:"id"`
	AgentID            string            `json:"agent_id"`
	AgentName          string            `json:"agent_name"`
	Task               string            `json:"task"`
	Response           string            `json:"response"`
	Status             string            `json:"status"` // completed, failed, timeout, cancelled
	DurationMs         int64             `json:"duration_ms"`
	ModelName          string            `json:"model_name"`
	StartedAt          time.Time         `json:"started_at"`
	CompletedAt        time.Time         `json:"completed_at"`
	ToolCalls          []ToolCall        `json:"tool_calls"`
	ExecutionSteps     []ExecutionStep   `json:"execution_steps"`
	TokenUsage         *TokenUsage       `json:"token_usage"`
	Metadata           map[string]string `json:"metadata"`
	OutputSchema       string            `json:"output_schema,omitempty"`        // JSON schema for output format
	OutputSchemaPreset string            `json:"output_schema_preset,omitempty"` // Preset name (e.g., "finops")
	MemoryTopicKey     string            `json:"memory_topic_key,omitempty"`     // Memory topic for NATS publishing
}

// ToolCall represents a single tool execution within an agent run
type ToolCall struct {
	ToolName   string      `json:"tool_name"`
	Parameters interface{} `json:"parameters"`
	Result     string      `json:"result"`
	DurationMs int64       `json:"duration_ms"`
	Success    bool        `json:"success"`
	Timestamp  time.Time   `json:"timestamp"`
}

// ExecutionStep represents a step in agent execution
type ExecutionStep struct {
	StepNumber  int       `json:"step_number"`
	Description string    `json:"description"`
	Type        string    `json:"type"` // tool_call, llm_call, processing
	DurationMs  int64     `json:"duration_ms"`
	Timestamp   time.Time `json:"timestamp"`
}

// TokenUsage represents LLM token consumption and cost
type TokenUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd"`
}

// DeploymentContext represents rich execution context for CLI mode
type DeploymentContext struct {
	CommandLine      string            `json:"command_line"`
	WorkingDirectory string            `json:"working_directory"`
	EnvVars          map[string]string `json:"env_vars"`
	Arguments        []string          `json:"arguments"`
	GitBranch        string            `json:"git_branch"`
	GitCommit        string            `json:"git_commit"`
	StationVersion   string            `json:"station_version"`
}

// SystemSnapshot represents complete system state at execution time
type SystemSnapshot struct {
	Agents         []AgentConfig     `json:"agents"`
	MCPServers     []MCPConfig       `json:"mcp_servers"`
	Variables      map[string]string `json:"variables"`
	AvailableTools []ToolInfo        `json:"available_tools"`
	Metrics        *SystemMetrics    `json:"metrics"`
}

// AgentConfig represents agent configuration
type AgentConfig struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	PromptTemplate string            `json:"prompt_template"`
	ModelName      string            `json:"model_name"`
	MaxSteps       int               `json:"max_steps"`
	Tools          []string          `json:"tools"`
	Variables      map[string]string `json:"variables"`
	Tags           []string          `json:"tags"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// MCPConfig represents MCP server configuration
type MCPConfig struct {
	Name      string            `json:"name"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	EnvVars   map[string]string `json:"env_vars"`
	Variables map[string]string `json:"variables"`
	Enabled   bool              `json:"enabled"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// ToolInfo represents information about an available tool
type ToolInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	MCPServer   string   `json:"mcp_server"`
	Categories  []string `json:"categories"`
}

// SystemMetrics represents system performance metrics
type SystemMetrics struct {
	CPUUsagePercent    float64           `json:"cpu_usage_percent"`
	MemoryUsagePercent float64           `json:"memory_usage_percent"`
	DiskUsageMB        int64             `json:"disk_usage_mb"`
	UptimeSeconds      int64             `json:"uptime_seconds"`
	ActiveConnections  int               `json:"active_connections"`
	ActiveRuns         int               `json:"active_runs"`
	NetworkInBytes     int64             `json:"network_in_bytes"`
	NetworkOutBytes    int64             `json:"network_out_bytes"`
	AdditionalMetrics  map[string]string `json:"additional_metrics"`
}
