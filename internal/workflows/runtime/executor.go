package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"station/internal/workflows"
)

var (
	ErrAgentNotFound      = errors.New("agent not found")
	ErrAgentIDRequired    = errors.New("agent_id is required")
	ErrInvalidAgentID     = errors.New("invalid agent_id type")
	ErrExecutorNotFound   = errors.New("executor not found for step type")
	ErrExecutionFailed    = errors.New("step execution failed")
	ErrApprovalRejected   = errors.New("approval was rejected")
	ErrApprovalTimedOut   = errors.New("approval timed out")
	ErrMessageRequired    = errors.New("message is required for approval")
	ErrApprovalCreateFail = errors.New("failed to create approval request")
)

type StepStatus string

const (
	StepStatusCompleted       StepStatus = "completed"
	StepStatusFailed          StepStatus = "failed"
	StepStatusWaitingApproval StepStatus = "waiting_approval"
	StepStatusApproved        StepStatus = "approved"
	StepStatusRejected        StepStatus = "rejected"
	StepStatusTimedOut        StepStatus = "timed_out"
)

type StepExecutor interface {
	Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error)
	SupportedTypes() []workflows.ExecutionStepType
}

type StepResult struct {
	Status     StepStatus             `json:"status"`
	Output     map[string]interface{} `json:"output,omitempty"`
	Error      *string                `json:"error,omitempty"`
	NextStep   string                 `json:"next_step,omitempty"`
	End        bool                   `json:"end,omitempty"`
	ApprovalID string                 `json:"approval_id,omitempty"`
}

type AgentExecutorDeps interface {
	GetAgentByID(id int64) (AgentInfo, error)
	ExecuteAgent(ctx context.Context, agentID int64, task string, variables map[string]interface{}) (AgentExecutionResult, error)
}

type AgentInfo struct {
	ID   int64
	Name string
}

type AgentExecutionResult struct {
	Response  string
	StepCount int64
	ToolsUsed int
}

type AgentRunExecutor struct {
	deps AgentExecutorDeps
}

func NewAgentRunExecutor(deps AgentExecutorDeps) *AgentRunExecutor {
	return &AgentRunExecutor{deps: deps}
}

func (e *AgentRunExecutor) SupportedTypes() []workflows.ExecutionStepType {
	return []workflows.ExecutionStepType{workflows.StepTypeAgent}
}

func (e *AgentRunExecutor) Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
	input := step.Raw.Input
	if input == nil {
		input = make(map[string]interface{})
	}

	agentIDRaw, ok := input["agent_id"]
	if !ok {
		return StepResult{}, ErrAgentIDRequired
	}

	var agentID int64
	switch v := agentIDRaw.(type) {
	case float64:
		agentID = int64(v)
	case int64:
		agentID = v
	case int:
		agentID = int64(v)
	case json.Number:
		id, err := v.Int64()
		if err != nil {
			return StepResult{}, fmt.Errorf("%w: %v", ErrInvalidAgentID, err)
		}
		agentID = id
	default:
		return StepResult{}, fmt.Errorf("%w: got %T", ErrInvalidAgentID, agentIDRaw)
	}

	agent, err := e.deps.GetAgentByID(agentID)
	if err != nil {
		return StepResult{}, fmt.Errorf("%w: %v", ErrAgentNotFound, err)
	}

	task, _ := input["task"].(string)
	if task == "" {
		task = fmt.Sprintf("Execute workflow step: %s", step.ID)
	}

	variables := make(map[string]interface{})
	if varsRaw, ok := input["variables"].(map[string]interface{}); ok {
		variables = varsRaw
	}

	for k, v := range runContext {
		if _, exists := variables[k]; !exists {
			variables[k] = v
		}
	}

	result, err := e.deps.ExecuteAgent(ctx, agentID, task, variables)
	if err != nil {
		errStr := err.Error()
		return StepResult{
			Status:   StepStatusFailed,
			Error:    &errStr,
			NextStep: step.Next,
			End:      step.End,
		}, fmt.Errorf("%w: %v", ErrExecutionFailed, err)
	}

	return StepResult{
		Status: StepStatusCompleted,
		Output: map[string]interface{}{
			"response":   result.Response,
			"agent_id":   agentID,
			"agent_name": agent.Name,
			"step_count": result.StepCount,
			"tools_used": result.ToolsUsed,
		},
		NextStep: step.Next,
		End:      step.End,
	}, nil
}

type ApprovalExecutorDeps interface {
	CreateApproval(ctx context.Context, params CreateApprovalParams) (ApprovalInfo, error)
	GetApproval(ctx context.Context, approvalID string) (ApprovalInfo, error)
}

type CreateApprovalParams struct {
	ApprovalID  string
	RunID       string
	StepID      string
	Message     string
	SummaryPath string
	Approvers   []string
	TimeoutSecs int64
}

type ApprovalInfo struct {
	ID             string
	Status         string
	DecidedBy      string
	DecisionReason string
}

type HumanApprovalExecutor struct {
	deps  ApprovalExecutorDeps
	runID string
}

func NewHumanApprovalExecutor(deps ApprovalExecutorDeps, runID string) *HumanApprovalExecutor {
	return &HumanApprovalExecutor{deps: deps, runID: runID}
}

func (e *HumanApprovalExecutor) SupportedTypes() []workflows.ExecutionStepType {
	return []workflows.ExecutionStepType{workflows.StepTypeAwait}
}

func (e *HumanApprovalExecutor) Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
	input := step.Raw.Input
	if input == nil {
		input = make(map[string]interface{})
	}

	action, _ := input["task"].(string)
	if action != "human.approval" {
		return StepResult{
			Status:   StepStatusCompleted,
			Output:   map[string]interface{}{"skipped": true, "reason": "not a human.approval action"},
			NextStep: step.Next,
			End:      step.End,
		}, nil
	}

	message, _ := input["message"].(string)
	if message == "" {
		return StepResult{Status: StepStatusFailed}, ErrMessageRequired
	}

	summaryPath, _ := input["summary_path"].(string)

	var approvers []string
	if approversRaw, ok := input["approvers"].([]interface{}); ok {
		for _, a := range approversRaw {
			if s, ok := a.(string); ok {
				approvers = append(approvers, s)
			}
		}
	}

	var timeoutSecs int64 = 3600
	if ts, ok := input["timeout_seconds"].(float64); ok {
		timeoutSecs = int64(ts)
	} else if ts, ok := input["timeout_seconds"].(int64); ok {
		timeoutSecs = ts
	}

	approvalID := fmt.Sprintf("appr-%s-%s", e.runID, step.ID)

	params := CreateApprovalParams{
		ApprovalID:  approvalID,
		RunID:       e.runID,
		StepID:      step.ID,
		Message:     message,
		SummaryPath: summaryPath,
		Approvers:   approvers,
		TimeoutSecs: timeoutSecs,
	}

	approval, err := e.deps.CreateApproval(ctx, params)
	if err != nil {
		errStr := err.Error()
		return StepResult{
			Status: StepStatusFailed,
			Error:  &errStr,
		}, fmt.Errorf("%w: %v", ErrApprovalCreateFail, err)
	}

	return StepResult{
		Status:     StepStatusWaitingApproval,
		ApprovalID: approval.ID,
		Output: map[string]interface{}{
			"approval_id": approval.ID,
			"message":     message,
			"status":      "pending",
		},
		NextStep: step.Next,
		End:      step.End,
	}, nil
}

func (e *HumanApprovalExecutor) HandleApprovalDecision(ctx context.Context, approvalID string, step workflows.ExecutionStep) (StepResult, error) {
	approval, err := e.deps.GetApproval(ctx, approvalID)
	if err != nil {
		errStr := err.Error()
		return StepResult{Status: StepStatusFailed, Error: &errStr}, err
	}

	switch approval.Status {
	case "approved":
		return StepResult{
			Status: StepStatusApproved,
			Output: map[string]interface{}{
				"approval_id":     approvalID,
				"status":          "approved",
				"decided_by":      approval.DecidedBy,
				"decision_reason": approval.DecisionReason,
			},
			NextStep: step.Next,
			End:      step.End,
		}, nil

	case "rejected":
		reason := "Approval was rejected"
		if approval.DecisionReason != "" {
			reason = approval.DecisionReason
		}
		return StepResult{
			Status: StepStatusRejected,
			Output: map[string]interface{}{
				"approval_id":     approvalID,
				"status":          "rejected",
				"decided_by":      approval.DecidedBy,
				"decision_reason": approval.DecisionReason,
			},
			Error: &reason,
		}, ErrApprovalRejected

	case "timed_out":
		reason := "Approval timed out"
		return StepResult{
			Status: StepStatusTimedOut,
			Output: map[string]interface{}{
				"approval_id": approvalID,
				"status":      "timed_out",
			},
			Error: &reason,
		}, ErrApprovalTimedOut

	default:
		return StepResult{
			Status:     StepStatusWaitingApproval,
			ApprovalID: approvalID,
		}, nil
	}
}

type ExecutorRegistry struct {
	executors map[workflows.ExecutionStepType]StepExecutor
}

func NewExecutorRegistry() *ExecutorRegistry {
	return &ExecutorRegistry{
		executors: make(map[workflows.ExecutionStepType]StepExecutor),
	}
}

func (r *ExecutorRegistry) Register(executor StepExecutor) {
	for _, stepType := range executor.SupportedTypes() {
		r.executors[stepType] = executor
	}
}

func (r *ExecutorRegistry) GetExecutor(stepType workflows.ExecutionStepType) (StepExecutor, error) {
	executor, ok := r.executors[stepType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrExecutorNotFound, stepType)
	}
	return executor, nil
}

func (r *ExecutorRegistry) Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
	executor, err := r.GetExecutor(step.Type)
	if err != nil {
		return StepResult{}, err
	}
	return executor.Execute(ctx, step, runContext)
}
