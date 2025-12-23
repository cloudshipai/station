package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"station/internal/db/repositories"
	"station/internal/workflows"
	"station/internal/workflows/runtime"
	"station/pkg/models"
)

// WorkflowService manages workflow definitions, validation, and durable run metadata.
type WorkflowService struct {
	repos  *repositories.Repositories
	engine runtime.Engine
}

// WorkflowDefinitionInput captures required fields to create or update a workflow.
type WorkflowDefinitionInput struct {
	WorkflowID  string
	Name        string
	Description string
	Definition  json.RawMessage
}

// StartWorkflowRunRequest defines parameters for starting a workflow run.
type StartWorkflowRunRequest struct {
	WorkflowID string
	Version    int64
	Input      json.RawMessage
	Options    json.RawMessage
}

// SignalWorkflowRunRequest represents a signal payload delivered to a run.
type SignalWorkflowRunRequest struct {
	RunID   string
	Name    string
	Payload json.RawMessage
}

// StepUpdate captures step progress updates.
type StepUpdate struct {
	RunID       string
	StepID      string
	Attempt     int64
	Status      string
	Output      json.RawMessage
	Error       *string
	Metadata    json.RawMessage
	CompletedAt *time.Time
}

func NewWorkflowService(repos *repositories.Repositories) *WorkflowService {
	return &WorkflowService{repos: repos}
}

// NewWorkflowServiceWithEngine allows injecting a runtime engine (e.g., NATS-backed) for scheduling.
func NewWorkflowServiceWithEngine(repos *repositories.Repositories, engine runtime.Engine) *WorkflowService {
	return &WorkflowService{repos: repos, engine: engine}
}

func (s *WorkflowService) ValidateDefinition(definition json.RawMessage) (*workflows.Definition, workflows.ValidationResult, error) {
	return workflows.ValidateDefinition(definition)
}

func (s *WorkflowService) CreateWorkflow(ctx context.Context, input WorkflowDefinitionInput) (*models.WorkflowDefinition, workflows.ValidationResult, error) {
	parsed, validation, err := workflows.ValidateDefinition(input.Definition)
	if err != nil && !errors.Is(err, workflows.ErrValidation) {
		return nil, validation, err
	}

	// Surface mismatch between body workflow_id and definition id as a warning
	if parsed != nil && parsed.ID != "" && parsed.ID != input.WorkflowID {
		validation.Warnings = append(validation.Warnings, workflows.ValidationIssue{
			Code:    "WORKFLOW_ID_MISMATCH",
			Path:    "/id",
			Message: fmt.Sprintf("Definition id '%s' does not match requested workflowId '%s'", parsed.ID, input.WorkflowID),
			Hint:    "Align the workflow id in the definition with the API request path for deterministic versioning.",
		})
	}

	if len(validation.Errors) > 0 {
		return nil, validation, workflows.ErrValidation
	}

	nextVersion, err := s.repos.Workflows.GetNextVersion(ctx, input.WorkflowID)
	if err != nil {
		return nil, validation, err
	}

	record, err := s.repos.Workflows.Insert(ctx, input.WorkflowID, input.Name, input.Description, nextVersion, input.Definition, "active")
	return record, validation, err
}

func (s *WorkflowService) UpdateWorkflow(ctx context.Context, input WorkflowDefinitionInput) (*models.WorkflowDefinition, workflows.ValidationResult, error) {
	parsed, validation, err := workflows.ValidateDefinition(input.Definition)
	if err != nil && !errors.Is(err, workflows.ErrValidation) {
		return nil, validation, err
	}

	if parsed != nil && parsed.ID != "" && parsed.ID != input.WorkflowID {
		validation.Warnings = append(validation.Warnings, workflows.ValidationIssue{
			Code:    "WORKFLOW_ID_MISMATCH",
			Path:    "/id",
			Message: fmt.Sprintf("Definition id '%s' does not match requested workflowId '%s'", parsed.ID, input.WorkflowID),
			Hint:    "Align the workflow id in the definition with the API request path for deterministic versioning.",
		})
	}

	if len(validation.Errors) > 0 {
		return nil, validation, workflows.ErrValidation
	}

	// Ensure workflow exists before version bump
	if _, err := s.repos.Workflows.GetLatest(ctx, input.WorkflowID); err != nil {
		return nil, validation, err
	}

	nextVersion, err := s.repos.Workflows.GetNextVersion(ctx, input.WorkflowID)
	if err != nil {
		return nil, validation, err
	}

	record, err := s.repos.Workflows.Insert(ctx, input.WorkflowID, input.Name, input.Description, nextVersion, input.Definition, "active")
	return record, validation, err
}

func (s *WorkflowService) GetWorkflow(ctx context.Context, workflowID string, version int64) (*models.WorkflowDefinition, error) {
	if version > 0 {
		return s.repos.Workflows.Get(ctx, workflowID, version)
	}
	return s.repos.Workflows.GetLatest(ctx, workflowID)
}

func (s *WorkflowService) ListWorkflows(ctx context.Context) ([]*models.WorkflowDefinition, error) {
	return s.repos.Workflows.ListLatest(ctx)
}

func (s *WorkflowService) ListWorkflowVersions(ctx context.Context, workflowID string) ([]*models.WorkflowDefinition, error) {
	return s.repos.Workflows.ListVersions(ctx, workflowID)
}

func (s *WorkflowService) DisableWorkflow(ctx context.Context, workflowID string) error {
	return s.repos.Workflows.Disable(ctx, workflowID)
}

func (s *WorkflowService) StartRun(ctx context.Context, req StartWorkflowRunRequest) (*models.WorkflowRun, workflows.ValidationResult, error) {
	definition, err := s.GetWorkflow(ctx, req.WorkflowID, req.Version)
	if err != nil {
		return nil, workflows.ValidationResult{}, err
	}

	parsed, validation, err := workflows.ValidateDefinition(definition.Definition)
	if err != nil && !errors.Is(err, workflows.ErrValidation) {
		return nil, validation, err
	}
	if len(validation.Errors) > 0 {
		return nil, validation, workflows.ErrValidation
	}

	startStep := parsed.Start
	runID := uuid.NewString()
	now := time.Now()

	run, err := s.repos.WorkflowRuns.Create(ctx, repositories.CreateWorkflowRunParams{
		RunID:           runID,
		WorkflowID:      req.WorkflowID,
		WorkflowVersion: definition.Version,
		Status:          "pending",
		CurrentStep:     optionalString(startStep),
		Input:           req.Input,
		Context:         req.Input, // bootstrap context with provided inputs
		Options:         req.Options,
		StartedAt:       now,
	})
	if err == nil && s.engine != nil {
		_ = s.engine.PublishRunEvent(ctx, runID, map[string]interface{}{
			"type":     "run_started",
			"workflow": req.WorkflowID,
			"version":  definition.Version,
			"step":     startStep,
		})
		if startStep != "" {
			plan := workflows.CompileExecutionPlan(parsed)
			step := plan.Steps[startStep]
			_ = s.engine.PublishStepSchedule(ctx, runID, startStep, step)
		}
	}
	return run, validation, err
}

func (s *WorkflowService) GetRun(ctx context.Context, runID string) (*models.WorkflowRun, error) {
	return s.repos.WorkflowRuns.Get(ctx, runID)
}

func (s *WorkflowService) ListRuns(ctx context.Context, workflowID, status string, limit int64) ([]*models.WorkflowRun, error) {
	if limit == 0 {
		limit = 50
	}
	return s.repos.WorkflowRuns.List(ctx, workflowID, status, limit)
}

func (s *WorkflowService) CancelRun(ctx context.Context, runID, reason string) (*models.WorkflowRun, error) {
	if _, err := s.repos.WorkflowRuns.Get(ctx, runID); err != nil {
		return nil, err
	}

	if reason == "" {
		reason = "Run canceled"
	}
	now := time.Now()
	if err := s.repos.WorkflowRuns.Update(ctx, repositories.UpdateWorkflowRunParams{
		RunID:       runID,
		Status:      "canceled",
		Error:       &reason,
		CompletedAt: &now,
	}); err != nil {
		return nil, err
	}
	return s.repos.WorkflowRuns.Get(ctx, runID)
}

func (s *WorkflowService) SignalRun(ctx context.Context, req SignalWorkflowRunRequest) (*models.WorkflowRun, error) {
	run, err := s.repos.WorkflowRuns.Get(ctx, req.RunID)
	if err != nil {
		return nil, err
	}

	signal := map[string]interface{}{
		"name":      req.Name,
		"payload":   json.RawMessage(req.Payload),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	signalBytes, _ := json.Marshal(signal)

	newStatus := run.Status
	if run.Status == "blocked" {
		newStatus = "pending"
	}

	if err := s.repos.WorkflowRuns.Update(ctx, repositories.UpdateWorkflowRunParams{
		RunID:      req.RunID,
		Status:     newStatus,
		LastSignal: signalBytes,
	}); err != nil {
		return nil, err
	}

	return s.repos.WorkflowRuns.Get(ctx, req.RunID)
}

// PauseRun marks a workflow run as blocked (pause).
func (s *WorkflowService) PauseRun(ctx context.Context, runID, reason string) (*models.WorkflowRun, error) {
	if reason == "" {
		reason = "Run paused"
	}
	now := time.Now()
	if err := s.repos.WorkflowRuns.Update(ctx, repositories.UpdateWorkflowRunParams{
		RunID:       runID,
		Status:      "blocked",
		Error:       &reason,
		CompletedAt: nil,
	}); err != nil {
		return nil, err
	}
	_ = s.emitRunEvent(ctx, runID, map[string]interface{}{
		"type":   "run_paused",
		"reason": reason,
		"paused": true,
		"time":   now.UTC().Format(time.RFC3339),
	})
	return s.repos.WorkflowRuns.Get(ctx, runID)
}

// ResumeRun is a convenience wrapper around SignalRun that unblocks a paused run.
func (s *WorkflowService) ResumeRun(ctx context.Context, runID, note string) (*models.WorkflowRun, error) {
	return s.SignalRun(ctx, SignalWorkflowRunRequest{
		RunID:   runID,
		Name:    "resume",
		Payload: json.RawMessage([]byte(fmt.Sprintf(`{"note":"%s"}`, note))),
	})
}

// CompleteRun marks a workflow run as completed.
func (s *WorkflowService) CompleteRun(ctx context.Context, runID string, result json.RawMessage, summary string) (*models.WorkflowRun, error) {
	now := time.Now()
	if err := s.repos.WorkflowRuns.Update(ctx, repositories.UpdateWorkflowRunParams{
		RunID:       runID,
		Status:      "completed",
		Result:      result,
		Summary:     optionalString(summary),
		CompletedAt: &now,
	}); err != nil {
		return nil, err
	}
	_ = s.emitRunEvent(ctx, runID, map[string]interface{}{
		"type":   "run_completed",
		"result": string(result),
		"time":   now.UTC().Format(time.RFC3339),
	})
	return s.repos.WorkflowRuns.Get(ctx, runID)
}

func (s *WorkflowService) RecordStepStart(ctx context.Context, runID, stepID string, attempt int64, input json.RawMessage, metadata json.RawMessage) (*models.WorkflowRunStep, error) {
	return s.repos.WorkflowRunSteps.Create(ctx, repositories.CreateWorkflowRunStepParams{
		RunID:     runID,
		StepID:    stepID,
		Attempt:   attempt,
		Status:    "running",
		Input:     input,
		Metadata:  metadata,
		StartedAt: timePointer(time.Now()),
	})
}

func (s *WorkflowService) RecordStepUpdate(ctx context.Context, update StepUpdate) error {
	return s.repos.WorkflowRunSteps.Update(ctx, repositories.UpdateWorkflowRunStepParams{
		RunID:       update.RunID,
		StepID:      update.StepID,
		Attempt:     update.Attempt,
		Status:      update.Status,
		Output:      update.Output,
		Error:       update.Error,
		Metadata:    update.Metadata,
		CompletedAt: update.CompletedAt,
	})
}

func (s *WorkflowService) ListSteps(ctx context.Context, runID string) ([]*models.WorkflowRunStep, error) {
	return s.repos.WorkflowRunSteps.ListByRun(ctx, runID)
}

func (s *WorkflowService) emitRunEvent(ctx context.Context, runID string, event map[string]interface{}) error {
	if s.engine == nil {
		return nil
	}
	return s.engine.PublishRunEvent(ctx, runID, event)
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func timePointer(t time.Time) *time.Time {
	return &t
}
