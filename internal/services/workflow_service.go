package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"station/internal/db/repositories"
	"station/internal/workflows"
	"station/internal/workflows/runtime"
	"station/pkg/models"
)

type WorkflowService struct {
	repos     *repositories.Repositories
	engine    runtime.Engine
	scheduler *WorkflowSchedulerService
	telemetry *runtime.WorkflowTelemetry
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
	WorkflowID    string
	Version       int64
	EnvironmentID int64
	Input         json.RawMessage
	Options       json.RawMessage
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

// ApproveWorkflowStepRequest defines parameters for approving a workflow step.
type ApproveWorkflowStepRequest struct {
	ApprovalID string
	ApproverID string
	Comment    string
}

// RejectWorkflowStepRequest defines parameters for rejecting a workflow step.
type RejectWorkflowStepRequest struct {
	ApprovalID string
	RejecterID string
	Reason     string
}

func NewWorkflowService(repos *repositories.Repositories) *WorkflowService {
	return &WorkflowService{repos: repos}
}

func NewWorkflowServiceWithEngine(repos *repositories.Repositories, engine runtime.Engine) *WorkflowService {
	return &WorkflowService{repos: repos, engine: engine}
}

func (s *WorkflowService) SetScheduler(scheduler *WorkflowSchedulerService) {
	s.scheduler = scheduler
}

// SetTelemetry sets the workflow telemetry for distributed tracing.
// When set, workflow runs will create parent spans that connect all step and agent spans.
func (s *WorkflowService) SetTelemetry(t *runtime.WorkflowTelemetry) {
	s.telemetry = t
}

// GetTelemetry returns the workflow telemetry instance (used by consumer to share telemetry).
func (s *WorkflowService) GetTelemetry() *runtime.WorkflowTelemetry {
	return s.telemetry
}

func (s *WorkflowService) registerCronScheduleIfNeeded(ctx context.Context, def *workflows.Definition, version int64) {
	if s.scheduler == nil || def == nil {
		return
	}
	if err := s.scheduler.RegisterWorkflowSchedule(ctx, def, version); err != nil {
		fmt.Printf("Warning: Failed to register cron schedule for %s: %v\n", def.ID, err)
	}
}

// agentLookupAdapter adapts repositories to workflows.AgentLookup interface
type agentLookupAdapter struct {
	repos *repositories.Repositories
}

func (a *agentLookupAdapter) GetAgentByNameAndEnvironment(ctx context.Context, name string, environmentID int64) (*models.Agent, error) {
	return a.repos.Agents.GetByNameAndEnvironment(name, environmentID)
}

func (a *agentLookupAdapter) GetAgentByNameGlobal(ctx context.Context, name string) (*models.Agent, error) {
	return a.repos.Agents.GetByNameGlobal(name)
}

func (a *agentLookupAdapter) GetEnvironmentIDByName(ctx context.Context, name string) (int64, error) {
	env, err := a.repos.Environments.GetByName(name)
	if err != nil {
		return 0, err
	}
	return env.ID, nil
}

func (s *WorkflowService) ValidateDefinition(ctx context.Context, definition json.RawMessage) (*workflows.Definition, workflows.ValidationResult, error) {
	def, basicResult, err := workflows.ValidateDefinition(definition)
	if err != nil {
		return def, basicResult, err
	}

	// Perform deeper agent and schema validation
	// Use default environment (id=1) as primary context if we can determine it, otherwise 0
	// TODO: Get actual environment ID from context or request if possible
	var envID int64 = 1
	if env, err := s.repos.Environments.GetByName("default"); err == nil {
		envID = env.ID
	}

	validator := workflows.NewAgentValidator(&agentLookupAdapter{repos: s.repos})
	agentResult := validator.ValidateAgents(ctx, def, envID)

	// Merge results
	basicResult.Errors = append(basicResult.Errors, agentResult.Errors...)
	basicResult.Warnings = append(basicResult.Warnings, agentResult.Warnings...)

	if len(basicResult.Errors) > 0 {
		return def, basicResult, workflows.ErrValidation
	}

	return def, basicResult, nil
}

func (s *WorkflowService) CreateWorkflow(ctx context.Context, input WorkflowDefinitionInput) (*models.WorkflowDefinition, workflows.ValidationResult, error) {
	parsed, validation, err := s.ValidateDefinition(ctx, input.Definition)
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

	nextVersion, err := s.repos.Workflows.GetNextVersion(ctx, input.WorkflowID)
	if err != nil {
		return nil, validation, err
	}

	record, err := s.repos.Workflows.Insert(ctx, input.WorkflowID, input.Name, input.Description, nextVersion, input.Definition, "active")
	if err != nil {
		return nil, validation, err
	}

	s.registerCronScheduleIfNeeded(ctx, parsed, nextVersion)

	return record, validation, nil
}

func (s *WorkflowService) UpdateWorkflow(ctx context.Context, input WorkflowDefinitionInput) (*models.WorkflowDefinition, workflows.ValidationResult, error) {
	parsed, validation, err := s.ValidateDefinition(ctx, input.Definition)
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

	if _, err := s.repos.Workflows.GetLatest(ctx, input.WorkflowID); err != nil {
		return nil, validation, err
	}

	nextVersion, err := s.repos.Workflows.GetNextVersion(ctx, input.WorkflowID)
	if err != nil {
		return nil, validation, err
	}

	record, err := s.repos.Workflows.Insert(ctx, input.WorkflowID, input.Name, input.Description, nextVersion, input.Definition, "active")
	if err != nil {
		return nil, validation, err
	}

	s.registerCronScheduleIfNeeded(ctx, parsed, nextVersion)

	return record, validation, nil
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

type DeleteWorkflowsRequest struct {
	WorkflowIDs []string
	All         bool
}

func (s *WorkflowService) DeleteWorkflows(ctx context.Context, req DeleteWorkflowsRequest) (int64, error) {
	if req.All {
		count, err := s.repos.Workflows.Count(ctx)
		if err != nil {
			return 0, err
		}
		if err := s.repos.Workflows.DeleteAll(ctx); err != nil {
			return 0, err
		}
		return count, nil
	}

	if len(req.WorkflowIDs) > 0 {
		for _, id := range req.WorkflowIDs {
			if err := s.repos.Workflows.Delete(ctx, id); err != nil {
				return 0, fmt.Errorf("failed to delete workflow %s: %w", id, err)
			}
		}
		return int64(len(req.WorkflowIDs)), nil
	}

	return 0, nil
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

	if parsed.InputSchema != nil && len(parsed.InputSchema) > 0 {
		var inputData map[string]interface{}
		if len(req.Input) > 0 {
			if err := json.Unmarshal(req.Input, &inputData); err != nil {
				validation.Errors = append(validation.Errors, workflows.ValidationIssue{
					Code:    "INVALID_INPUT_JSON",
					Path:    "/input",
					Message: fmt.Sprintf("Failed to parse input JSON: %v", err),
					Hint:    "Provide valid JSON for the workflow input",
				})
				return nil, validation, workflows.ErrValidation
			}
		} else {
			inputData = make(map[string]interface{})
		}

		schemaJSON, _ := json.Marshal(parsed.InputSchema)
		if err := workflows.ValidateInputAgainstSchema(inputData, string(schemaJSON)); err != nil {
			validation.Errors = append(validation.Errors, workflows.ValidationIssue{
				Code:    "INPUT_SCHEMA_VIOLATION",
				Path:    "/input",
				Message: err.Error(),
				Hint:    "Ensure the workflow input matches the defined inputSchema",
			})
			return nil, validation, workflows.ErrValidation
		}
	}

	startStep := parsed.Start
	runID := uuid.NewString()
	now := time.Now()

	if s.telemetry != nil {
		ctx = s.telemetry.StartRunSpan(ctx, runID, req.WorkflowID)
	}

	plan := workflows.CompileExecutionPlan(parsed)

	// If start step is a cron trigger, skip to its next step (cron is just a trigger definition)
	if step, ok := plan.Steps[startStep]; ok && step.Type == workflows.StepTypeCron {
		if step.Next != "" {
			log.Printf("[WorkflowService] Cron trigger step '%s' → skipping to executable step '%s'", startStep, step.Next)
			startStep = step.Next
		} else {
			log.Printf("[WorkflowService] WARNING: Cron step '%s' has no next step defined", startStep)
		}
	}

	initialContext := s.buildInitialContext(req.Input, req.EnvironmentID)

	run, err := s.repos.WorkflowRuns.Create(ctx, repositories.CreateWorkflowRunParams{
		RunID:           runID,
		WorkflowID:      req.WorkflowID,
		WorkflowVersion: definition.Version,
		Status:          "pending",
		CurrentStep:     optionalString(startStep),
		Input:           req.Input,
		Context:         initialContext,
		Options:         req.Options,
		StartedAt:       now,
	})
	if err == nil {
		_ = s.emitRunEvent(ctx, runID, map[string]interface{}{
			"type":     models.EventTypeRunStarted,
			"workflow": req.WorkflowID,
			"version":  definition.Version,
			"step":     startStep,
		})
		if startStep != "" && s.engine != nil {
			step := plan.Steps[startStep]
			if err := s.engine.PublishStepWithTrace(ctx, runID, startStep, step); err != nil {
				log.Printf("[WorkflowService] ERROR: Failed to publish initial step '%s': %v", startStep, err)
			}
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

type DeleteRunsRequest struct {
	RunIDs     []string
	WorkflowID string
	Status     string
	All        bool
}

func (s *WorkflowService) DeleteRuns(ctx context.Context, req DeleteRunsRequest) (int64, error) {
	if req.All {
		count, err := s.repos.WorkflowRuns.Count(ctx)
		if err != nil {
			return 0, err
		}
		if err := s.repos.WorkflowRuns.DeleteAll(ctx); err != nil {
			return 0, err
		}
		return count, nil
	}

	if len(req.RunIDs) > 0 {
		if err := s.repos.WorkflowRuns.DeleteByIDs(ctx, req.RunIDs); err != nil {
			return 0, err
		}
		return int64(len(req.RunIDs)), nil
	}

	if req.Status != "" {
		runs, err := s.repos.WorkflowRuns.List(ctx, "", req.Status, 10000)
		if err != nil {
			return 0, err
		}
		if err := s.repos.WorkflowRuns.DeleteByStatus(ctx, req.Status); err != nil {
			return 0, err
		}
		return int64(len(runs)), nil
	}

	if req.WorkflowID != "" {
		runs, err := s.repos.WorkflowRuns.List(ctx, req.WorkflowID, "", 10000)
		if err != nil {
			return 0, err
		}
		if err := s.repos.WorkflowRuns.DeleteByWorkflowID(ctx, req.WorkflowID); err != nil {
			return 0, err
		}
		return int64(len(runs)), nil
	}

	return 0, nil
}

func (s *WorkflowService) CancelRun(ctx context.Context, runID, reason string) (*models.WorkflowRun, error) {
	run, err := s.repos.WorkflowRuns.Get(ctx, runID)
	if err != nil {
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

	if s.telemetry != nil {
		duration := now.Sub(run.StartedAt)
		s.telemetry.EndRunSpan(ctx, runID, run.WorkflowID, "canceled", duration, fmt.Errorf("canceled: %s", reason))
	}

	_ = s.emitRunEvent(ctx, runID, map[string]interface{}{
		"type":   models.EventTypeRunCanceled,
		"reason": reason,
		"time":   now.UTC().Format(time.RFC3339),
	})
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

	_ = s.emitRunEvent(ctx, req.RunID, map[string]interface{}{
		"type":    models.EventTypeSignalReceived,
		"signal":  req.Name,
		"payload": string(req.Payload),
		"time":    time.Now().UTC().Format(time.RFC3339),
	})

	return s.repos.WorkflowRuns.Get(ctx, req.RunID)
}

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
		"type":   models.EventTypeRunPaused,
		"reason": reason,
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

func (s *WorkflowService) CompleteRun(ctx context.Context, runID string, result json.RawMessage, summary string) (*models.WorkflowRun, error) {
	run, err := s.repos.WorkflowRuns.Get(ctx, runID)
	if err != nil {
		return nil, err
	}

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

	if s.telemetry != nil {
		duration := now.Sub(run.StartedAt)
		s.telemetry.EndRunSpan(ctx, runID, run.WorkflowID, "completed", duration, nil)
	}

	_ = s.emitRunEvent(ctx, runID, map[string]interface{}{
		"type":   models.EventTypeRunCompleted,
		"result": string(result),
		"time":   now.UTC().Format(time.RFC3339),
	})
	return s.repos.WorkflowRuns.Get(ctx, runID)
}

func (s *WorkflowService) RecordStepStart(ctx context.Context, runID, stepID string, attempt int64, input json.RawMessage, metadata json.RawMessage) (*models.WorkflowRunStep, error) {
	step, err := s.repos.WorkflowRunSteps.Create(ctx, repositories.CreateWorkflowRunStepParams{
		RunID:     runID,
		StepID:    stepID,
		Attempt:   attempt,
		Status:    "running",
		Input:     input,
		Metadata:  metadata,
		StartedAt: timePointer(time.Now()),
	})
	if err == nil {
		_ = s.emitRunEvent(ctx, runID, map[string]interface{}{
			"type":    models.EventTypeStepStarted,
			"step_id": stepID,
			"attempt": attempt,
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	}
	return step, err
}

func (s *WorkflowService) RecordStepUpdate(ctx context.Context, update StepUpdate) error {
	err := s.repos.WorkflowRunSteps.Update(ctx, repositories.UpdateWorkflowRunStepParams{
		RunID:       update.RunID,
		StepID:      update.StepID,
		Attempt:     update.Attempt,
		Status:      update.Status,
		Output:      update.Output,
		Error:       update.Error,
		Metadata:    update.Metadata,
		CompletedAt: update.CompletedAt,
	})
	if err == nil {
		eventType := models.EventTypeStepCompleted
		if update.Status == "failed" {
			eventType = models.EventTypeStepFailed
		}
		_ = s.emitRunEvent(ctx, update.RunID, map[string]interface{}{
			"type":    eventType,
			"step_id": update.StepID,
			"attempt": update.Attempt,
			"status":  update.Status,
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	}
	return err
}

func (s *WorkflowService) ListSteps(ctx context.Context, runID string) ([]*models.WorkflowRunStep, error) {
	return s.repos.WorkflowRunSteps.ListByRun(ctx, runID)
}

func (s *WorkflowService) GetApproval(ctx context.Context, approvalID string) (*models.WorkflowApproval, error) {
	if s.repos.WorkflowApprovals == nil {
		return nil, errors.New("approval repository not initialized")
	}
	return s.repos.WorkflowApprovals.Get(ctx, approvalID)
}

func (s *WorkflowService) ListPendingApprovals(ctx context.Context, runID string, limit int64) ([]*models.WorkflowApproval, error) {
	if s.repos.WorkflowApprovals == nil {
		return nil, errors.New("approval repository not initialized")
	}
	if limit == 0 {
		limit = 50
	}

	if runID != "" {
		approvals, err := s.repos.WorkflowApprovals.ListByRun(ctx, runID)
		if err != nil {
			return nil, err
		}
		pending := make([]*models.WorkflowApproval, 0)
		for _, a := range approvals {
			if a.Status == models.ApprovalStatusPending {
				pending = append(pending, a)
			}
		}
		return pending, nil
	}

	return s.repos.WorkflowApprovals.ListPending(ctx, limit)
}

func (s *WorkflowService) ApproveWorkflowStep(ctx context.Context, req ApproveWorkflowStepRequest) (*models.WorkflowApproval, error) {
	if s.repos.WorkflowApprovals == nil {
		return nil, errors.New("approval repository not initialized")
	}

	approval, err := s.repos.WorkflowApprovals.Get(ctx, req.ApprovalID)
	if err != nil {
		return nil, fmt.Errorf("approval not found: %w", err)
	}

	if approval.Status != models.ApprovalStatusPending {
		return nil, fmt.Errorf("approval is not pending, current status: %s", approval.Status)
	}

	var approverID *string
	if req.ApproverID != "" {
		approverID = &req.ApproverID
	}
	var comment *string
	if req.Comment != "" {
		comment = &req.Comment
	}

	if err := s.repos.WorkflowApprovals.Approve(ctx, req.ApprovalID, approverID, comment); err != nil {
		return nil, fmt.Errorf("failed to approve: %w", err)
	}

	_ = s.emitRunEvent(ctx, approval.RunID, map[string]interface{}{
		"type":        models.EventTypeApprovalDecided,
		"approval_id": req.ApprovalID,
		"step_id":     approval.StepID,
		"decision":    models.ApprovalStatusApproved,
		"actor":       req.ApproverID,
		"comment":     req.Comment,
		"time":        time.Now().UTC().Format(time.RFC3339),
	})

	s.resumeAfterApproval(ctx, approval.RunID, approval.StepID)

	return s.repos.WorkflowApprovals.Get(ctx, req.ApprovalID)
}

func (s *WorkflowService) RejectWorkflowStep(ctx context.Context, req RejectWorkflowStepRequest) (*models.WorkflowApproval, error) {
	if s.repos.WorkflowApprovals == nil {
		return nil, errors.New("approval repository not initialized")
	}

	approval, err := s.repos.WorkflowApprovals.Get(ctx, req.ApprovalID)
	if err != nil {
		return nil, fmt.Errorf("approval not found: %w", err)
	}

	if approval.Status != models.ApprovalStatusPending {
		return nil, fmt.Errorf("approval is not pending, current status: %s", approval.Status)
	}

	var rejecterID *string
	if req.RejecterID != "" {
		rejecterID = &req.RejecterID
	}
	var reason *string
	if req.Reason != "" {
		reason = &req.Reason
	}

	if err := s.repos.WorkflowApprovals.Reject(ctx, req.ApprovalID, rejecterID, reason); err != nil {
		return nil, fmt.Errorf("failed to reject: %w", err)
	}

	_ = s.emitRunEvent(ctx, approval.RunID, map[string]interface{}{
		"type":        models.EventTypeApprovalDecided,
		"approval_id": req.ApprovalID,
		"step_id":     approval.StepID,
		"decision":    models.ApprovalStatusRejected,
		"actor":       req.RejecterID,
		"reason":      req.Reason,
		"time":        time.Now().UTC().Format(time.RFC3339),
	})

	s.failAfterRejection(ctx, approval.RunID, req.Reason)

	return s.repos.WorkflowApprovals.Get(ctx, req.ApprovalID)
}

func (s *WorkflowService) ListApprovalsByRun(ctx context.Context, runID string) ([]*models.WorkflowApproval, error) {
	if s.repos.WorkflowApprovals == nil {
		return nil, errors.New("approval repository not initialized")
	}
	return s.repos.WorkflowApprovals.ListByRun(ctx, runID)
}

func (s *WorkflowService) TimeoutExpiredApprovals(ctx context.Context) error {
	if s.repos.WorkflowApprovals == nil {
		return errors.New("approval repository not initialized")
	}
	return s.repos.WorkflowApprovals.TimeoutExpired(ctx)
}

func (s *WorkflowService) emitRunEvent(ctx context.Context, runID string, event map[string]interface{}) error {
	eventType, _ := event["type"].(string)
	if eventType == "" {
		eventType = "unknown"
	}

	var stepID *string
	if sid, ok := event["step_id"].(string); ok && sid != "" {
		stepID = &sid
	} else if sid, ok := event["step"].(string); ok && sid != "" {
		stepID = &sid
	}

	var payload *string
	if payloadBytes, err := json.Marshal(event); err == nil {
		payloadStr := string(payloadBytes)
		payload = &payloadStr
	}

	var actor *string
	if a, ok := event["actor"].(string); ok && a != "" {
		actor = &a
	}

	if s.repos.WorkflowRunEvents != nil {
		_, _ = s.repos.WorkflowRunEvents.Insert(ctx, repositories.CreateWorkflowRunEventParams{
			RunID:     runID,
			EventType: eventType,
			StepID:    stepID,
			Payload:   payload,
			Actor:     actor,
		})
	}

	if s.engine != nil {
		return s.engine.PublishRunEvent(ctx, runID, event)
	}
	return nil
}

func (s *WorkflowService) buildInitialContext(input json.RawMessage, environmentID int64) json.RawMessage {
	ctx := make(map[string]interface{})

	var inputData map[string]interface{}
	if len(input) > 0 {
		_ = json.Unmarshal(input, &inputData)
	}
	if inputData == nil {
		inputData = make(map[string]interface{})
	}

	ctx["workflow"] = map[string]interface{}{
		"input": inputData,
	}
	ctx["steps"] = make(map[string]interface{})

	for k, v := range inputData {
		ctx[k] = v
	}

	if environmentID > 0 {
		ctx["_environmentID"] = environmentID
	}

	result, err := json.Marshal(ctx)
	if err != nil {
		return input
	}
	return result
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

func (s *WorkflowService) failAfterRejection(ctx context.Context, runID, reason string) {
	if reason == "" {
		reason = "approval rejected"
	}

	run, _ := s.repos.WorkflowRuns.Get(ctx, runID)

	now := time.Now()
	_ = s.repos.WorkflowRuns.Update(ctx, repositories.UpdateWorkflowRunParams{
		RunID:       runID,
		Status:      "failed",
		Error:       &reason,
		CompletedAt: &now,
	})

	if s.telemetry != nil && run != nil {
		duration := now.Sub(run.StartedAt)
		s.telemetry.EndRunSpan(ctx, runID, run.WorkflowID, "failed", duration, fmt.Errorf("rejected: %s", reason))
	}
}

func (s *WorkflowService) resumeAfterApproval(ctx context.Context, runID, stepID string) {
	if s.engine == nil {
		return
	}

	run, err := s.repos.WorkflowRuns.Get(ctx, runID)
	if err != nil {
		return
	}

	definition, err := s.repos.Workflows.Get(ctx, run.WorkflowID, run.WorkflowVersion)
	if err != nil {
		return
	}

	parsed, _, err := workflows.ValidateDefinition(definition.Definition)
	if err != nil || parsed == nil {
		return
	}

	plan := workflows.CompileExecutionPlan(parsed)
	step, exists := plan.Steps[stepID]
	if !exists {
		return
	}

	if step.End || step.Next == "" {
		now := time.Now()
		_ = s.repos.WorkflowRuns.Update(ctx, repositories.UpdateWorkflowRunParams{
			RunID:       runID,
			Status:      "completed",
			CompletedAt: &now,
		})
		return
	}

	nextStep, exists := plan.Steps[step.Next]
	if !exists {
		return
	}

	_ = s.engine.PublishStepWithTrace(ctx, runID, step.Next, nextStep)
}

type WorkflowSyncResult struct {
	WorkflowsProcessed int
	WorkflowsSynced    int
	WorkflowsSkipped   int
	WorkflowsDisabled  int
	Errors             []WorkflowSyncError
}

type WorkflowSyncError struct {
	WorkflowID string
	FilePath   string
	Error      string
}

func (s *WorkflowService) SyncWorkflowFiles(ctx context.Context, workflowsDir string) (*WorkflowSyncResult, error) {
	result := &WorkflowSyncResult{
		Errors: []WorkflowSyncError{},
	}

	loader := workflows.NewLoader(workflowsDir)
	loadResult, err := loader.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow files: %w", err)
	}

	result.WorkflowsProcessed = loadResult.TotalFiles

	for _, loadErr := range loadResult.Errors {
		result.Errors = append(result.Errors, WorkflowSyncError{
			FilePath: loadErr.FilePath,
			Error:    loadErr.Error.Error(),
		})
	}

	existingWorkflows, err := s.repos.Workflows.ListLatest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing workflows: %w", err)
	}

	existingMap := make(map[string]*models.WorkflowDefinition)
	for _, wf := range existingWorkflows {
		existingMap[wf.WorkflowID] = wf
	}

	fileWorkflowIDs := make(map[string]bool)

	for _, wf := range loadResult.Workflows {
		fileWorkflowIDs[wf.WorkflowID] = true

		existing, exists := existingMap[wf.WorkflowID]
		if exists && existing.Status == "active" {
			var existingDef workflows.Definition
			if err := json.Unmarshal(existing.Definition, &existingDef); err == nil {
				if existingDef.Version == wf.Definition.Version {
					result.WorkflowsSkipped++
					continue
				}
			}
		}

		name := wf.Definition.Name
		if name == "" {
			name = wf.WorkflowID
		}
		description := wf.Definition.Description

		input := WorkflowDefinitionInput{
			WorkflowID:  wf.WorkflowID,
			Name:        name,
			Description: description,
			Definition:  wf.RawContent,
		}

		var createErr error
		if exists {
			_, _, createErr = s.UpdateWorkflow(ctx, input)
		} else {
			_, _, createErr = s.CreateWorkflow(ctx, input)
		}

		if createErr != nil {
			result.Errors = append(result.Errors, WorkflowSyncError{
				WorkflowID: wf.WorkflowID,
				FilePath:   wf.FilePath,
				Error:      createErr.Error(),
			})
			continue
		}

		result.WorkflowsSynced++
	}

	for workflowID, existing := range existingMap {
		if !fileWorkflowIDs[workflowID] && existing.Status == "active" {
			if err := s.repos.Workflows.Disable(ctx, workflowID); err != nil {
				result.Errors = append(result.Errors, WorkflowSyncError{
					WorkflowID: workflowID,
					Error:      fmt.Sprintf("failed to disable orphaned workflow: %v", err),
				})
				continue
			}
			result.WorkflowsDisabled++
		}
	}

	return result, nil
}

func (s *WorkflowService) RegisterCronSchedules(ctx context.Context, scheduler *WorkflowSchedulerService) error {
	workflowList, err := s.repos.Workflows.ListLatest(ctx)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	for _, wf := range workflowList {
		if wf.Status != "active" {
			continue
		}

		def, _, err := workflows.ValidateDefinition(wf.Definition)
		if err != nil || def == nil {
			continue
		}

		if err := scheduler.RegisterWorkflowSchedule(ctx, def, wf.Version); err != nil {
			return fmt.Errorf("failed to register schedule for %s: %w", wf.WorkflowID, err)
		}
	}

	return nil
}
