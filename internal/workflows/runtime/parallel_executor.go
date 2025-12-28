package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"station/internal/workflows"
)

var (
	ErrNoBranches       = errors.New("no branches defined for parallel state")
	ErrBranchFailed     = errors.New("one or more branches failed")
	ErrBranchNoStates   = errors.New("branch has no states")
	ErrUnsupportedJoin  = errors.New("unsupported join mode")
	ErrBranchesRequired = errors.New("branches required for parallel state")
)

type BranchExecutorDeps interface {
	ExecuteStep(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error)
}

type ParallelExecutor struct {
	deps BranchExecutorDeps
}

func NewParallelExecutor(deps BranchExecutorDeps) *ParallelExecutor {
	return &ParallelExecutor{deps: deps}
}

func (e *ParallelExecutor) SupportedTypes() []workflows.ExecutionStepType {
	return []workflows.ExecutionStepType{workflows.StepTypeParallel}
}

type branchResult struct {
	Name   string
	Output map[string]interface{}
	Error  error
}

func (e *ParallelExecutor) Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
	raw := step.Raw

	if raw.Type != "parallel" {
		return StepResult{
			Status:   StepStatusCompleted,
			Output:   map[string]interface{}{"skipped": true, "reason": "not a parallel state"},
			NextStep: step.Next,
			End:      step.End,
		}, nil
	}

	if len(raw.Branches) == 0 {
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr("no branches defined"),
		}, ErrNoBranches
	}

	joinMode := "all"
	if raw.Join != nil && raw.Join.Mode != "" {
		joinMode = raw.Join.Mode
	}

	if joinMode != "all" {
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr(fmt.Sprintf("unsupported join mode: %s", joinMode)),
		}, ErrUnsupportedJoin
	}

	results := make(chan branchResult, len(raw.Branches))
	var wg sync.WaitGroup

	for _, branch := range raw.Branches {
		wg.Add(1)
		go func(b workflows.BranchSpec) {
			defer wg.Done()
			output, err := e.executeBranch(ctx, b, deepCopyMap(runContext))
			results <- branchResult{
				Name:   b.Name,
				Output: output,
				Error:  err,
			}
		}(branch)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	mergedOutput := make(map[string]interface{})
	var errs []error

	for result := range results {
		if result.Error != nil {
			errs = append(errs, fmt.Errorf("branch %s: %w", result.Name, result.Error))
		}
		if result.Output != nil {
			mergedOutput[result.Name] = result.Output
		}
	}

	if len(errs) > 0 {
		errMsg := fmt.Sprintf("%d branch(es) failed", len(errs))
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr(errMsg),
			Output: mergedOutput,
		}, fmt.Errorf("%w: %v", ErrBranchFailed, errs)
	}

	if raw.ResultPath != "" {
		SetNestedValue(runContext, raw.ResultPath, mergedOutput)
	}

	return StepResult{
		Status:   StepStatusCompleted,
		Output:   mergedOutput,
		NextStep: step.Next,
		End:      step.End,
	}, nil
}

func (e *ParallelExecutor) executeBranch(ctx context.Context, branch workflows.BranchSpec, branchContext map[string]interface{}) (map[string]interface{}, error) {
	if len(branch.States) == 0 {
		return nil, ErrBranchNoStates
	}

	currentOutput := make(map[string]interface{})

	for i, state := range branch.States {
		select {
		case <-ctx.Done():
			return currentOutput, ctx.Err()
		default:
		}

		branchStep := workflows.ExecutionStep{
			ID:   fmt.Sprintf("%s-%d", branch.Name, i),
			Type: classifyBranchState(state),
			Next: "",
			End:  state.End || i == len(branch.States)-1,
			Raw:  state,
		}

		if i < len(branch.States)-1 {
			branchStep.Next = branch.States[i+1].StableID()
		}

		result, err := e.deps.ExecuteStep(ctx, branchStep, branchContext)
		if err != nil {
			return currentOutput, err
		}

		if result.Status == StepStatusFailed {
			errMsg := "step failed"
			if result.Error != nil {
				errMsg = *result.Error
			}
			return currentOutput, errors.New(errMsg)
		}

		if result.Output != nil {
			for k, v := range result.Output {
				currentOutput[k] = v
			}
		}

		if state.ResultPath != "" {
			SetNestedValue(branchContext, state.ResultPath, result.Output)
		}

		if branchStep.End || result.End {
			break
		}
	}

	return currentOutput, nil
}

func classifyBranchState(state workflows.StateSpec) workflows.ExecutionStepType {
	switch state.Type {
	case "agent":
		return workflows.StepTypeAgent
	case "operation", "action", "function":
		if task, ok := state.Input["task"]; ok {
			if taskStr, ok := task.(string); ok {
				if taskStr == "agent.run" || taskStr == "agent.hierarchy.run" {
					return workflows.StepTypeAgent
				}
			}
		}
		return workflows.StepTypeCustom
	case "switch":
		return workflows.StepTypeBranch
	case "foreach", "while":
		return workflows.StepTypeLoop
	case "parallel":
		return workflows.StepTypeParallel
	case "set", "transform", "context", "inject":
		return workflows.StepTypeContextOp
	case "await", "await.signal", "await.event", "human_approval":
		return workflows.StepTypeAwait
	case "sleep", "delay", "timer":
		return workflows.StepTypeTimer
	case "cron", "schedule":
		return workflows.StepTypeCron
	case "try":
		return workflows.StepTypeTryCatch
	default:
		return workflows.StepTypeCustom
	}
}
