package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type Environment struct {
	ID          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description" db:"description"`
	CreatedBy   int64     `json:"created_by" db:"created_by"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type User struct {
	ID        int64     `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`
	PublicKey string    `json:"public_key" db:"public_key"`
	IsAdmin   bool      `json:"is_admin" db:"is_admin"`
	APIKey    *string   `json:"api_key,omitempty" db:"api_key"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type MCPConfig struct {
	ID              int64     `json:"id" db:"id"`
	EnvironmentID   int64     `json:"environment_id" db:"environment_id"`
	ConfigName      string    `json:"config_name" db:"config_name"`
	Version         int64     `json:"version" db:"version"`
	ConfigJSON      string    `json:"config_json" db:"config_json"`      // encrypted
	EncryptedConfig string    `json:"encrypted_config" db:"config_json"` // alias for consistency
	EncryptionKeyID string    `json:"encryption_key_id" db:"encryption_key_id"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

type MCPServer struct {
	ID             int64             `json:"id" db:"id"`
	Name           string            `json:"name" db:"name"`
	Command        string            `json:"command" db:"command"`
	Args           []string          `json:"args" db:"args"`
	Env            map[string]string `json:"env" db:"env"`
	WorkingDir     *string           `json:"working_dir" db:"working_dir"`
	TimeoutSeconds *int64            `json:"timeout_seconds" db:"timeout_seconds"`
	AutoRestart    *bool             `json:"auto_restart" db:"auto_restart"`
	EnvironmentID  int64             `json:"environment_id" db:"environment_id"`
	FileConfigID   *int64            `json:"file_config_id,omitempty" db:"file_config_id"`
	CreatedAt      time.Time         `json:"created_at" db:"created_at"`
}

type MCPTool struct {
	ID          int64           `json:"id" db:"id"`
	MCPServerID int64           `json:"mcp_server_id" db:"mcp_server_id"`
	Name        string          `json:"name" db:"name"`
	Description string          `json:"description" db:"description"`
	Schema      json.RawMessage `json:"schema" db:"input_schema"` // JSON schema
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}

type Agent struct {
	ID                 int64      `json:"id" db:"id"`
	Name               string     `json:"name" db:"name"`
	Description        string     `json:"description" db:"description"`
	Prompt             string     `json:"prompt" db:"prompt"`
	MaxSteps           int64      `json:"max_steps" db:"max_steps"`
	EnvironmentID      int64      `json:"environment_id" db:"environment_id"`
	CreatedBy          int64      `json:"created_by" db:"created_by"`
	InputSchema        *string    `json:"input_schema,omitempty" db:"input_schema"`
	OutputSchema       *string    `json:"output_schema,omitempty" db:"output_schema"`
	OutputSchemaPreset *string    `json:"output_schema_preset,omitempty" db:"output_schema_preset"`
	App                string     `json:"app,omitempty" db:"app"`
	AppType            string     `json:"app_type,omitempty" db:"app_subtype"` // Note: DB column is app_subtype but we use app_type in code
	CronSchedule       *string    `json:"cron_schedule,omitempty" db:"cron_schedule"`
	IsScheduled        bool       `json:"is_scheduled" db:"is_scheduled"`
	LastScheduledRun   *time.Time `json:"last_scheduled_run,omitempty" db:"last_scheduled_run"`
	NextScheduledRun   *time.Time `json:"next_scheduled_run,omitempty" db:"next_scheduled_run"`
	ScheduleEnabled    bool       `json:"schedule_enabled" db:"schedule_enabled"`
	ScheduleVariables  *string    `json:"schedule_variables,omitempty" db:"schedule_variables"`
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at" db:"updated_at"`
}

// AgentTool represents the many-to-many relationship between agents and tools (environment-specific)
type AgentTool struct {
	ID        int64     `json:"id" db:"id"`
	AgentID   int64     `json:"agent_id" db:"agent_id"`
	ToolID    int64     `json:"tool_id" db:"tool_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// AgentToolWithDetails includes tool and server information with environment context
type AgentToolWithDetails struct {
	AgentTool
	ToolName        string `json:"tool_name" db:"tool_name"`
	ToolDescription string `json:"tool_description" db:"tool_description"`
	ToolSchema      string `json:"tool_schema" db:"tool_schema"`
	ServerName      string `json:"server_name" db:"server_name"`
	EnvironmentID   int64  `json:"environment_id" db:"environment_id"`
	EnvironmentName string `json:"environment_name" db:"environment_name"`
}

// AgentAuditLog tracks changes to agent configuration for health monitoring
type AgentAuditLog struct {
	ID          int64     `json:"id" db:"id"`
	AgentID     int64     `json:"agent_id" db:"agent_id"`
	EventType   string    `json:"event_type" db:"event_type"`     // "tool_removed", "tool_added", "config_changed", etc.
	EventReason string    `json:"event_reason" db:"event_reason"` // "orphaned_config", "manual_removal", etc.
	Details     string    `json:"details" db:"details"`           // JSON with specifics like tool names, config names
	Impact      string    `json:"impact" db:"impact"`             // "high", "medium", "low"
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type MCPToolWithDetails struct {
	MCPTool
	ServerName      string `json:"server_name" db:"server_name"`
	ConfigID        int64  `json:"config_id" db:"config_id"`
	ConfigName      string `json:"config_name" db:"config_name"`
	ConfigVersion   int64  `json:"config_version" db:"config_version"`
	EnvironmentID   int64  `json:"environment_id" db:"environment_id"`
	EnvironmentName string `json:"environment_name" db:"environment_name"`
}

type AgentRun struct {
	ID             int64      `json:"id" db:"id"`
	AgentID        int64      `json:"agent_id" db:"agent_id"`
	UserID         int64      `json:"user_id" db:"user_id"`
	Task           string     `json:"task" db:"task"`
	FinalResponse  string     `json:"final_response" db:"final_response"`
	StepsTaken     int64      `json:"steps_taken" db:"steps_taken"`
	ToolCalls      *JSONArray `json:"tool_calls" db:"tool_calls"`
	ExecutionSteps *JSONArray `json:"execution_steps" db:"execution_steps"`
	Status         string     `json:"status" db:"status"`
	StartedAt      time.Time  `json:"started_at" db:"started_at"`
	CompletedAt    *time.Time `json:"completed_at" db:"completed_at"`
	// Response object metadata from Station's OpenAI plugin
	InputTokens     *int64     `json:"input_tokens,omitempty" db:"input_tokens"`
	OutputTokens    *int64     `json:"output_tokens,omitempty" db:"output_tokens"`
	TotalTokens     *int64     `json:"total_tokens,omitempty" db:"total_tokens"`
	DurationSeconds *float64   `json:"duration_seconds,omitempty" db:"duration_seconds"`
	ModelName       *string    `json:"model_name,omitempty" db:"model_name"`
	ToolsUsed       *int64     `json:"tools_used,omitempty" db:"tools_used"`
	DebugLogs       *JSONArray `json:"debug_logs,omitempty" db:"debug_logs"`
	Error           *string    `json:"error,omitempty" db:"error"`
	ParentRunID     *int64     `json:"parent_run_id,omitempty" db:"parent_run_id"` // Track parent run for hierarchical agent execution
}

type AgentRunWithDetails struct {
	AgentRun
	AgentName string `json:"agent_name" db:"agent_name"`
	Username  string `json:"username" db:"username"`
}

// JSONArray is a custom type for handling JSON arrays in SQLite
type JSONArray []interface{}

func (j JSONArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// MCPConfigData represents the decrypted MCP configuration
type MCPConfigData struct {
	Name    string                     `json:"name,omitempty"`
	Servers map[string]MCPServerConfig `json:"servers"`
}

type MCPServerConfig struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// Tool execution types for agent framework integration
type ToolCall struct {
	ToolName   string                 `json:"tool_name"`
	ServerName string                 `json:"server_name"`
	Arguments  map[string]interface{} `json:"arguments"`
	Result     interface{}            `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

type ExecutionStep struct {
	StepNumber int        `json:"step_number"`
	Action     string     `json:"action"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	Response   string     `json:"response"`
	Timestamp  time.Time  `json:"timestamp"`
}

// Setting represents a system setting
type Setting struct {
	ID          int64     `json:"id" db:"id"`
	Key         string    `json:"key" db:"key"`
	Value       string    `json:"value" db:"value"`
	Description *string   `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Webhook represents a webhook endpoint configuration
type Webhook struct {
	ID             int64     `json:"id" db:"id"`
	Name           string    `json:"name" db:"name"`
	URL            string    `json:"url" db:"url"`
	Secret         string    `json:"secret,omitempty" db:"secret"`
	Enabled        bool      `json:"enabled" db:"enabled"`
	Events         string    `json:"events" db:"events"`             // JSON array of event types
	Headers        string    `json:"headers,omitempty" db:"headers"` // JSON object of custom headers
	TimeoutSeconds int       `json:"timeout_seconds" db:"timeout_seconds"`
	RetryAttempts  int       `json:"retry_attempts" db:"retry_attempts"`
	CreatedBy      int64     `json:"created_by" db:"created_by"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	ID              int64      `json:"id" db:"id"`
	WebhookID       int64      `json:"webhook_id" db:"webhook_id"`
	EventType       string     `json:"event_type" db:"event_type"`
	Payload         string     `json:"payload" db:"payload"`
	Status          string     `json:"status" db:"status"` // pending, success, failed
	HTTPStatusCode  *int       `json:"http_status_code,omitempty" db:"http_status_code"`
	ResponseBody    *string    `json:"response_body,omitempty" db:"response_body"`
	ResponseHeaders *string    `json:"response_headers,omitempty" db:"response_headers"`
	ErrorMessage    *string    `json:"error_message,omitempty" db:"error_message"`
	AttemptCount    int        `json:"attempt_count" db:"attempt_count"`
	LastAttemptAt   *time.Time `json:"last_attempt_at,omitempty" db:"last_attempt_at"`
	NextRetryAt     *time.Time `json:"next_retry_at,omitempty" db:"next_retry_at"`
	DeliveredAt     *time.Time `json:"delivered_at,omitempty" db:"delivered_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
}

// AgentAgent represents a parent-child agent relationship for hierarchical orchestration
type AgentAgent struct {
	ID            int64     `json:"id" db:"id"`
	ParentAgentID int64     `json:"parent_agent_id" db:"parent_agent_id"`
	ChildAgentID  int64     `json:"child_agent_id" db:"child_agent_id"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// ChildAgent includes the child agent details with the relationship information
type ChildAgent struct {
	RelationshipID int64     `json:"relationship_id"`
	ParentAgentID  int64     `json:"parent_agent_id"`
	ChildAgentID   int64     `json:"child_agent_id"`
	ChildAgent     Agent     `json:"child_agent"`
	CreatedAt      time.Time `json:"created_at"`
}
