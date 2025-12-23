package workflows

import (
	"encoding/json"
	"errors"
)

// Definition represents the Station workflow profile (a subset of Serverless Workflow).
type Definition struct {
	ID          string      `json:"id" yaml:"id"`
	Name        string      `json:"name" yaml:"name"`
	Version     string      `json:"version,omitempty" yaml:"version,omitempty"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	Start       string      `json:"start,omitempty" yaml:"start,omitempty"`
	States      []StateSpec `json:"states" yaml:"states"`
}

// StateSpec captures the minimal fields required for Station workflow states.
type StateSpec struct {
	ID         string                 `json:"id,omitempty" yaml:"id,omitempty"`
	Name       string                 `json:"name,omitempty" yaml:"name,omitempty"`
	Type       string                 `json:"type" yaml:"type"`
	Input      map[string]interface{} `json:"input,omitempty" yaml:"input,omitempty"`
	Output     map[string]interface{} `json:"output,omitempty" yaml:"output,omitempty"`
	Transition string                 `json:"transition,omitempty" yaml:"transition,omitempty"`
	Next       string                 `json:"next,omitempty" yaml:"next,omitempty"`
	Retry      *RetryPolicy           `json:"retry,omitempty" yaml:"retry,omitempty"`
	Timeout    string                 `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// StableID returns the normalized identifier for a state.
func (s StateSpec) StableID() string {
	if s.ID != "" {
		return s.ID
	}
	return s.Name
}

// RetryPolicy is a lightweight representation of step retry configuration.
type RetryPolicy struct {
	MaxAttempts *int     `json:"max_attempts,omitempty" yaml:"max_attempts,omitempty"`
	Backoff     string   `json:"backoff,omitempty" yaml:"backoff,omitempty"`
	RetryOn     []string `json:"retry_on,omitempty" yaml:"retry_on,omitempty"`
}

// ValidationIssue is a structured validation error or warning for LLM-friendly authoring.
type ValidationIssue struct {
	Code     string      `json:"code"`
	Path     string      `json:"path"`
	Message  string      `json:"message"`
	Expected interface{} `json:"expected,omitempty"`
	Actual   interface{} `json:"actual,omitempty"`
	Hint     string      `json:"hint,omitempty"`
}

// ValidationResult aggregates validation errors and warnings.
type ValidationResult struct {
	Errors   []ValidationIssue `json:"errors"`
	Warnings []ValidationIssue `json:"warnings"`
}

// ErrValidation indicates the definition failed validation.
var ErrValidation = errors.New("workflow validation failed")

// MarshalDefinition re-serializes a parsed definition for persistence or inspection.
func MarshalDefinition(def *Definition) (json.RawMessage, error) {
	if def == nil {
		return nil, nil
	}
	data, err := json.Marshal(def)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
