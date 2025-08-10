package services

import (
	"fmt"
	"time"
)

// ToolDiscoveryErrorType represents different types of tool discovery errors
type ToolDiscoveryErrorType string

const (
	ErrorTypeTimeout           ToolDiscoveryErrorType = "timeout"
	ErrorTypeConnection        ToolDiscoveryErrorType = "connection"
	ErrorTypeDecryption        ToolDiscoveryErrorType = "decryption"
	ErrorTypeInvalidConfig     ToolDiscoveryErrorType = "invalid_config"
	ErrorTypeServerStart       ToolDiscoveryErrorType = "server_start"
	ErrorTypeToolParsing       ToolDiscoveryErrorType = "tool_parsing"
	ErrorTypeDatabase          ToolDiscoveryErrorType = "database"
	ErrorTypeTemplateRendering ToolDiscoveryErrorType = "template_rendering"
)

// ToolDiscoveryError represents a structured error from tool discovery
type ToolDiscoveryError struct {
	Type       ToolDiscoveryErrorType `json:"type"`
	ServerName string                 `json:"server_name,omitempty"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

func (e *ToolDiscoveryError) Error() string {
	if e.ServerName != "" {
		return fmt.Sprintf("Tool discovery error for server '%s': %s", e.ServerName, e.Message)
	}
	return fmt.Sprintf("Tool discovery error: %s", e.Message)
}

// ToolDiscoveryResult represents the result of tool discovery for an environment
type ToolDiscoveryResult struct {
	EnvironmentID    int64                 `json:"environment_id"`
	ConfigID         int64                 `json:"config_id"`
	ConfigName       string                `json:"config_name,omitempty"`
	TotalServers     int                   `json:"total_servers"`
	SuccessfulServers int                  `json:"successful_servers"`
	FailedServers    int                   `json:"failed_servers"`
	TotalTools       int                   `json:"total_tools"`
	Errors           []*ToolDiscoveryError `json:"errors,omitempty"`
	StartedAt        time.Time             `json:"started_at"`
	CompletedAt      time.Time             `json:"completed_at"`
	Success          bool                  `json:"success"`
}

// NewToolDiscoveryError creates a new structured error
func NewToolDiscoveryError(errorType ToolDiscoveryErrorType, serverName, message, details string) *ToolDiscoveryError {
	return &ToolDiscoveryError{
		Type:       errorType,
		ServerName: serverName,
		Message:    message,
		Details:    details,
		Timestamp:  time.Now(),
	}
}

// AddError adds an error to the discovery result
func (r *ToolDiscoveryResult) AddError(err *ToolDiscoveryError) {
	if r.Errors == nil {
		r.Errors = make([]*ToolDiscoveryError, 0)
	}
	r.Errors = append(r.Errors, err)
	r.FailedServers++
}

// HasErrors returns true if there are any errors
func (r *ToolDiscoveryResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// GetErrorSummary returns a summary of errors by type
func (r *ToolDiscoveryResult) GetErrorSummary() map[ToolDiscoveryErrorType]int {
	summary := make(map[ToolDiscoveryErrorType]int)
	for _, err := range r.Errors {
		summary[err.Type]++
	}
	return summary
}