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

type StateSpec struct {
	ID         string                 `json:"id,omitempty" yaml:"id,omitempty"`
	Name       string                 `json:"name,omitempty" yaml:"name,omitempty"`
	Type       string                 `json:"type" yaml:"type"`
	Input      map[string]interface{} `json:"input,omitempty" yaml:"input,omitempty"`
	Output     map[string]interface{} `json:"output,omitempty" yaml:"output,omitempty"`
	Transition string                 `json:"transition,omitempty" yaml:"transition,omitempty"`
	Next       string                 `json:"next,omitempty" yaml:"next,omitempty"`
	End        bool                   `json:"end,omitempty" yaml:"end,omitempty"`
	Retry      *RetryPolicy           `json:"retry,omitempty" yaml:"retry,omitempty"`
	Timeout    string                 `json:"timeout,omitempty" yaml:"timeout,omitempty"`

	// Agent step fields (convenience - can also be specified via Input)
	Agent string `json:"agent,omitempty" yaml:"agent,omitempty"`
	Task  string `json:"task,omitempty" yaml:"task,omitempty"`

	// Human approval fields
	Message       string   `json:"message,omitempty" yaml:"message,omitempty"`
	ApprovalTitle string   `json:"approval_title,omitempty" yaml:"approval_title,omitempty"`
	Approvers     []string `json:"approvers,omitempty" yaml:"approvers,omitempty"`

	DataPath    string            `json:"dataPath,omitempty" yaml:"dataPath,omitempty"`
	Conditions  []SwitchCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	DefaultNext string            `json:"defaultNext,omitempty" yaml:"defaultNext,omitempty"`

	Data       map[string]interface{} `json:"data,omitempty" yaml:"data,omitempty"`
	ResultPath string                 `json:"resultPath,omitempty" yaml:"resultPath,omitempty"`

	// Parallel state fields
	Branches []BranchSpec `json:"branches,omitempty" yaml:"branches,omitempty"`
	Join     *JoinSpec    `json:"join,omitempty" yaml:"join,omitempty"`

	// Foreach state fields
	ItemsPath      string        `json:"itemsPath,omitempty" yaml:"itemsPath,omitempty"`
	ItemName       string        `json:"itemName,omitempty" yaml:"itemName,omitempty"`
	MaxConcurrency int           `json:"maxConcurrency,omitempty" yaml:"maxConcurrency,omitempty"`
	Iterator       *IteratorSpec `json:"iterator,omitempty" yaml:"iterator,omitempty"`

	// Cron state fields
	Cron     string `json:"cron,omitempty" yaml:"cron,omitempty"`
	Timezone string `json:"timezone,omitempty" yaml:"timezone,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// Timer state fields
	Duration string `json:"duration,omitempty" yaml:"duration,omitempty"`

	// TryCatch state fields
	Try     *IteratorSpec `json:"try,omitempty" yaml:"try,omitempty"`
	Catch   *IteratorSpec `json:"catch,omitempty" yaml:"catch,omitempty"`
	Finally *IteratorSpec `json:"finally,omitempty" yaml:"finally,omitempty"`
}

// BranchSpec defines a parallel branch with its own mini-workflow.
type BranchSpec struct {
	Name   string      `json:"name" yaml:"name"`
	States []StateSpec `json:"states" yaml:"states"`
}

// JoinSpec defines how parallel branches are joined.
type JoinSpec struct {
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty"` // "all" (default), "any" (future)
}

// IteratorSpec defines the inline workflow for foreach iteration.
type IteratorSpec struct {
	Start  string      `json:"start,omitempty" yaml:"start,omitempty"`
	States []StateSpec `json:"states" yaml:"states"`
}

type SwitchCondition struct {
	If   string `json:"if" yaml:"if"`
	Next string `json:"next" yaml:"next"`
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
