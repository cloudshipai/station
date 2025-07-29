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
	ConfigJSON      string    `json:"config_json" db:"config_json"` // encrypted
	EncryptedConfig string    `json:"encrypted_config" db:"config_json"` // alias for consistency
	EncryptionKeyID string    `json:"encryption_key_id" db:"encryption_key_id"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

type MCPServer struct {
	ID          int64             `json:"id" db:"id"`
	MCPConfigID int64             `json:"mcp_config_id" db:"mcp_config_id"`
	Name        string            `json:"name" db:"name"`
	Command     string            `json:"command" db:"command"`
	Args        []string          `json:"args" db:"args"`
	Env         map[string]string `json:"env" db:"env"`
	CreatedAt   time.Time         `json:"created_at" db:"created_at"`
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
	ID            int64     `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	Description   string    `json:"description" db:"description"`
	Prompt        string    `json:"prompt" db:"prompt"`
	MaxSteps      int64     `json:"max_steps" db:"max_steps"`
	EnvironmentID int64     `json:"environment_id" db:"environment_id"`
	CreatedBy     int64     `json:"created_by" db:"created_by"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

type AgentTool struct {
	ID        int64     `json:"id" db:"id"`
	AgentID   int64     `json:"agent_id" db:"agent_id"`
	ToolID    int64     `json:"tool_id" db:"tool_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type AgentToolWithDetails struct {
	AgentTool
	ToolName        string          `json:"tool_name" db:"tool_name"`
	ToolDescription string          `json:"tool_description" db:"tool_description"`
	ToolSchema      json.RawMessage `json:"tool_schema" db:"tool_schema"`
	ServerName      string          `json:"server_name" db:"server_name"`
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
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
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