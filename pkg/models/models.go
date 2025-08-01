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
	ConfigJSON      string    `json:"config_json" db:"config_json"` // encrypted
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
	ID                int64      `json:"id" db:"id"`
	Name              string     `json:"name" db:"name"`
	Description       string     `json:"description" db:"description"`
	Prompt            string     `json:"prompt" db:"prompt"`
	MaxSteps          int64      `json:"max_steps" db:"max_steps"`
	EnvironmentID     int64      `json:"environment_id" db:"environment_id"`
	CreatedBy         int64      `json:"created_by" db:"created_by"`
	CronSchedule      *string    `json:"cron_schedule,omitempty" db:"cron_schedule"`
	IsScheduled       bool       `json:"is_scheduled" db:"is_scheduled"`
	LastScheduledRun  *time.Time `json:"last_scheduled_run,omitempty" db:"last_scheduled_run"`
	NextScheduledRun  *time.Time `json:"next_scheduled_run,omitempty" db:"next_scheduled_run"`
	ScheduleEnabled   bool       `json:"schedule_enabled" db:"schedule_enabled"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}

// AgentEnvironment represents the many-to-many relationship between agents and environments
type AgentEnvironment struct {
	ID            int64     `json:"id" db:"id"`
	AgentID       int64     `json:"agent_id" db:"agent_id"`
	EnvironmentID int64     `json:"environment_id" db:"environment_id"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// AgentEnvironmentWithDetails includes environment details
type AgentEnvironmentWithDetails struct {
	AgentEnvironment
	EnvironmentName        string `json:"environment_name" db:"environment_name"`
	EnvironmentDescription string `json:"environment_description" db:"environment_description"`
}

// AgentTool represents the many-to-many relationship between agents and tools
type AgentTool struct {
	ID            int64     `json:"id" db:"id"`
	AgentID       int64     `json:"agent_id" db:"agent_id"`
	ToolName      string    `json:"tool_name" db:"tool_name"`
	EnvironmentID int64     `json:"environment_id" db:"environment_id"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// AgentToolWithDetails includes environment information for cross-environment context
type AgentToolWithDetails struct {
	AgentTool
	ToolName        string `json:"tool_name" db:"tool_name"`
	ToolDescription string `json:"tool_description" db:"tool_description"`
	ToolSchema      string `json:"tool_schema" db:"tool_schema"`
	ServerName      string `json:"server_name" db:"server_name"`
	EnvironmentID   int64  `json:"environment_id" db:"environment_id"`
	EnvironmentName string `json:"environment_name" db:"environment_name"`
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
	
	bytes, ok := value.([]byte)
	if !ok {
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
	StepNumber int       `json:"step_number"`
	Action     string    `json:"action"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	Response   string    `json:"response"`
	Timestamp  time.Time `json:"timestamp"`
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
	Events         string    `json:"events" db:"events"` // JSON array of event types
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