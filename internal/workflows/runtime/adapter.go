package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"station/internal/db/repositories"
	"station/internal/workflows"
)

type WorkflowServiceAdapter struct {
	repos     *repositories.Repositories
	engine    Engine
	planCache map[string]workflows.ExecutionPlan
	telemetry *WorkflowTelemetry
}

func NewWorkflowServiceAdapter(repos *repositories.Repositories, engine Engine) *WorkflowServiceAdapter {
	return &WorkflowServiceAdapter{
		repos:     repos,
		engine:    engine,
		planCache: make(map[string]workflows.ExecutionPlan),
	}
}

func (a *WorkflowServiceAdapter) CachePlan(runID string, plan workflows.ExecutionPlan) {
	a.planCache[runID] = plan
}

func (a *WorkflowServiceAdapter) GetCachedPlan(runID string) (workflows.ExecutionPlan, bool) {
	plan, ok := a.planCache[runID]
	return plan, ok
}

func (a *WorkflowServiceAdapter) RemoveCachedPlan(runID string) {
	delete(a.planCache, runID)
}

func (a *WorkflowServiceAdapter) SetTelemetry(t *WorkflowTelemetry) {
	a.telemetry = t
}

func (a *WorkflowServiceAdapter) UpdateRunStatus(ctx context.Context, runID, status string, currentStep *string, errMsg *string) error {
	return a.repos.WorkflowRuns.Update(ctx, repositories.UpdateWorkflowRunParams{
		RunID:       runID,
		Status:      status,
		CurrentStep: currentStep,
		Error:       errMsg,
	})
}

func (a *WorkflowServiceAdapter) GetRunContext(ctx context.Context, runID string) (map[string]interface{}, error) {
	run, err := a.repos.WorkflowRuns.Get(ctx, runID)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if run.Context == nil || len(run.Context) == 0 {
		result = make(map[string]interface{})
	} else {
		if err := json.Unmarshal(run.Context, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal run context: %w", err)
		}
	}

	// Add default environment ID if not already set
	// This is needed for agent name resolution in workflow steps
	if _, ok := result["_environmentID"]; !ok {
		env, err := a.repos.Environments.GetByName("default")
		if err == nil && env != nil {
			result["_environmentID"] = env.ID
		}
	}

	return result, nil
}

func (a *WorkflowServiceAdapter) UpdateRunContext(ctx context.Context, runID string, context map[string]interface{}) error {
	contextJSON, err := json.Marshal(context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	run, err := a.repos.WorkflowRuns.Get(ctx, runID)
	if err != nil {
		return err
	}

	return a.repos.WorkflowRuns.Update(ctx, repositories.UpdateWorkflowRunParams{
		RunID:   runID,
		Status:  run.Status,
		Context: contextJSON,
	})
}

func (a *WorkflowServiceAdapter) CompleteRun(ctx context.Context, runID string, result map[string]interface{}) error {
	now := time.Now()
	var resultJSON json.RawMessage
	if result != nil {
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		resultJSON = data
	}

	a.RemoveCachedPlan(runID)

	var workflowID string
	var duration time.Duration
	if a.telemetry != nil {
		if run, err := a.repos.WorkflowRuns.Get(ctx, runID); err == nil {
			workflowID = run.WorkflowID
			duration = now.Sub(run.StartedAt)
		}
	}

	err := a.repos.WorkflowRuns.Update(ctx, repositories.UpdateWorkflowRunParams{
		RunID:       runID,
		Status:      "completed",
		Result:      resultJSON,
		CompletedAt: &now,
	})

	if a.telemetry != nil && workflowID != "" {
		a.telemetry.EndRunSpan(ctx, runID, workflowID, "completed", duration, nil)
	}

	return err
}

func (a *WorkflowServiceAdapter) FailRun(ctx context.Context, runID string, errMsg string) error {
	now := time.Now()
	a.RemoveCachedPlan(runID)

	var workflowID string
	var duration time.Duration
	if a.telemetry != nil {
		if run, err := a.repos.WorkflowRuns.Get(ctx, runID); err == nil {
			workflowID = run.WorkflowID
			duration = now.Sub(run.StartedAt)
		}
	}

	err := a.repos.WorkflowRuns.Update(ctx, repositories.UpdateWorkflowRunParams{
		RunID:       runID,
		Status:      "failed",
		Error:       &errMsg,
		CompletedAt: &now,
	})

	if a.telemetry != nil && workflowID != "" {
		a.telemetry.EndRunSpan(ctx, runID, workflowID, "failed", duration, fmt.Errorf("%s", errMsg))
	}

	return err
}

func (a *WorkflowServiceAdapter) RecordStepStart(ctx context.Context, runID, stepID string, stepType string) error {
	existing, err := a.repos.WorkflowRunSteps.Get(ctx, runID, stepID, 1)
	if err == nil && existing != nil {
		return nil
	}

	now := time.Now()
	metadata, _ := json.Marshal(map[string]interface{}{
		"step_type": stepType,
	})

	_, err = a.repos.WorkflowRunSteps.Create(ctx, repositories.CreateWorkflowRunStepParams{
		RunID:     runID,
		StepID:    stepID,
		Attempt:   1,
		Status:    "running",
		Metadata:  metadata,
		StartedAt: &now,
	})
	if err != nil && isUniqueConstraintError(err) {
		return nil
	}
	return err
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func (a *WorkflowServiceAdapter) RecordStepComplete(ctx context.Context, runID, stepID string, output map[string]interface{}) error {
	now := time.Now()
	var outputJSON json.RawMessage
	if output != nil {
		data, err := json.Marshal(output)
		if err != nil {
			return fmt.Errorf("failed to marshal output: %w", err)
		}
		outputJSON = data
	}

	return a.repos.WorkflowRunSteps.Update(ctx, repositories.UpdateWorkflowRunStepParams{
		RunID:       runID,
		StepID:      stepID,
		Attempt:     1,
		Status:      "completed",
		Output:      outputJSON,
		CompletedAt: &now,
	})
}

func (a *WorkflowServiceAdapter) RecordStepFailed(ctx context.Context, runID, stepID string, errMsg string) error {
	now := time.Now()
	return a.repos.WorkflowRunSteps.Update(ctx, repositories.UpdateWorkflowRunStepParams{
		RunID:       runID,
		StepID:      stepID,
		Attempt:     1,
		Status:      "failed",
		Error:       &errMsg,
		CompletedAt: &now,
	})
}

func (a *WorkflowServiceAdapter) RecordStepWaiting(ctx context.Context, runID, stepID string, approvalID string) error {
	metadata, _ := json.Marshal(map[string]interface{}{
		"approval_id": approvalID,
	})

	return a.repos.WorkflowRunSteps.Update(ctx, repositories.UpdateWorkflowRunStepParams{
		RunID:    runID,
		StepID:   stepID,
		Attempt:  1,
		Status:   "waiting_approval",
		Metadata: metadata,
	})
}

func (a *WorkflowServiceAdapter) GetExecutionPlan(ctx context.Context, runID string) (workflows.ExecutionPlan, error) {
	if plan, ok := a.GetCachedPlan(runID); ok {
		return plan, nil
	}

	run, err := a.repos.WorkflowRuns.Get(ctx, runID)
	if err != nil {
		return workflows.ExecutionPlan{}, fmt.Errorf("failed to get run: %w", err)
	}

	def, err := a.repos.Workflows.Get(ctx, run.WorkflowID, run.WorkflowVersion)
	if err != nil {
		return workflows.ExecutionPlan{}, fmt.Errorf("failed to get workflow definition: %w", err)
	}

	var parsed workflows.Definition
	if err := json.Unmarshal(def.Definition, &parsed); err != nil {
		return workflows.ExecutionPlan{}, fmt.Errorf("failed to parse workflow definition: %w", err)
	}

	plan := workflows.CompileExecutionPlan(&parsed)
	a.CachePlan(runID, plan)

	return plan, nil
}
func (a *WorkflowServiceAdapter) GetStep(ctx context.Context, runID, stepID string) (workflows.ExecutionStep, error) {
	plan, err := a.GetExecutionPlan(ctx, runID)
	if err != nil {
		return workflows.ExecutionStep{}, err
	}

	step, ok := plan.Steps[stepID]
	if !ok {
		return workflows.ExecutionStep{}, fmt.Errorf("step %s not found in execution plan", stepID)
	}

	return step, nil
}

func (a *WorkflowServiceAdapter) ListPendingRuns(ctx context.Context, limit int64) ([]PendingRunInfo, error) {
	runs, err := a.repos.WorkflowRuns.List(ctx, "", "pending", limit, 0)
	if err != nil {
		return nil, err
	}

	result := make([]PendingRunInfo, 0, len(runs))
	for _, run := range runs {
		currentStep := ""
		if run.CurrentStep != nil {
			currentStep = *run.CurrentStep
		}
		result = append(result, PendingRunInfo{
			RunID:       run.RunID,
			WorkflowID:  run.WorkflowID,
			CurrentStep: currentStep,
			CreatedAt:   run.CreatedAt,
		})
	}
	return result, nil
}
