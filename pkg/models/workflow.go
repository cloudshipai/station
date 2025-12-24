package models

import (
	"encoding/json"
	"time"
)

// WorkflowDefinition represents a versioned workflow document persisted in the database.
type WorkflowDefinition struct {
	ID          int64           `json:"id"`
	WorkflowID  string          `json:"workflow_id"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Version     int64           `json:"version"`
	Definition  json.RawMessage `json:"definition"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// WorkflowRun captures execution metadata for a workflow instance.
type WorkflowRun struct {
	ID              int64           `json:"id"`
	RunID           string          `json:"run_id"`
	WorkflowID      string          `json:"workflow_id"`
	WorkflowVersion int64           `json:"workflow_version"`
	Status          string          `json:"status"`
	CurrentStep     *string         `json:"current_step,omitempty"`
	Input           json.RawMessage `json:"input,omitempty"`
	Context         json.RawMessage `json:"context,omitempty"`
	Result          json.RawMessage `json:"result,omitempty"`
	Error           *string         `json:"error,omitempty"`
	Summary         *string         `json:"summary,omitempty"`
	Options         json.RawMessage `json:"options,omitempty"`
	LastSignal      json.RawMessage `json:"last_signal,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	StartedAt       time.Time       `json:"started_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
}

// WorkflowRunStep records durable step history for a workflow run.
type WorkflowRunStep struct {
	ID          int64           `json:"id"`
	RunID       string          `json:"run_id"`
	StepID      string          `json:"step_id"`
	Attempt     int64           `json:"attempt"`
	Status      string          `json:"status"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	Error       *string         `json:"error,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// WorkflowRunEvent records audit trail events for workflow runs.
type WorkflowRunEvent struct {
	ID        int64     `json:"id"`
	RunID     string    `json:"run_id"`
	Seq       int64     `json:"seq"`
	EventType string    `json:"event_type"`
	StepID    *string   `json:"step_id,omitempty"`
	Payload   *string   `json:"payload,omitempty"`
	Actor     *string   `json:"actor,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// WorkflowApproval tracks human approval requests for workflow steps.
type WorkflowApproval struct {
	ID             int64      `json:"id"`
	ApprovalID     string     `json:"approval_id"`
	RunID          string     `json:"run_id"`
	StepID         string     `json:"step_id"`
	Message        string     `json:"message"`
	SummaryPath    *string    `json:"summary_path,omitempty"`
	Approvers      *string    `json:"approvers,omitempty"`
	Status         string     `json:"status"`
	DecidedBy      *string    `json:"decided_by,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	DecisionReason *string    `json:"decision_reason,omitempty"`
	TimeoutAt      *time.Time `json:"timeout_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

const (
	EventTypeRunStarted      = "run_started"
	EventTypeRunCompleted    = "run_completed"
	EventTypeRunFailed       = "run_failed"
	EventTypeRunCanceled     = "run_canceled"
	EventTypeRunPaused       = "run_paused"
	EventTypeRunResumed      = "run_resumed"
	EventTypeStepStarted     = "step_started"
	EventTypeStepCompleted   = "step_completed"
	EventTypeStepFailed      = "step_failed"
	EventTypeSignalReceived  = "signal_received"
	EventTypeApprovalDecided = "approval_decided"
)

type WorkflowSchedule struct {
	ID              int64           `json:"id"`
	WorkflowID      string          `json:"workflow_id"`
	WorkflowVersion int64           `json:"workflow_version"`
	CronExpression  string          `json:"cron_expression"`
	Timezone        string          `json:"timezone"`
	Enabled         bool            `json:"enabled"`
	Input           json.RawMessage `json:"input,omitempty"`
	LastRunAt       *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt       *time.Time      `json:"next_run_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

const (
	ApprovalStatusPending  = "pending"
	ApprovalStatusApproved = "approved"
	ApprovalStatusRejected = "rejected"
	ApprovalStatusTimedOut = "timed_out"
)
