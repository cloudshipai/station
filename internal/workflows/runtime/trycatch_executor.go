package runtime

import (
	"context"

	"station/internal/workflows"
)

const (
	StepStatusTryFailed    StepStatus = "try_failed"
	StepStatusCatchRunning StepStatus = "catch_running"
)

type TryCatchExecutor struct {
	registry *ExecutorRegistry
}

func NewTryCatchExecutor(registry *ExecutorRegistry) *TryCatchExecutor {
	return &TryCatchExecutor{registry: registry}
}

func (e *TryCatchExecutor) SupportedTypes() []workflows.ExecutionStepType {
	return []workflows.ExecutionStepType{workflows.StepTypeTryCatch}
}

func (e *TryCatchExecutor) Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
	raw := step.Raw

	if raw.Try == nil || len(raw.Try.States) == 0 {
		return StepResult{
			Status:   StepStatusCompleted,
			Output:   map[string]interface{}{"skipped": true, "reason": "no try block defined"},
			NextStep: step.Next,
			End:      step.End,
		}, nil
	}

	tryResult, tryErr := e.executeBlock(ctx, raw.Try, runContext)

	if tryErr == nil && tryResult.Status == StepStatusCompleted {
		output := map[string]interface{}{
			"block":      "try",
			"try_output": tryResult.Output,
		}

		if raw.Finally != nil && len(raw.Finally.States) > 0 {
			finallyResult, _ := e.executeBlock(ctx, raw.Finally, runContext)
			output["finally_output"] = finallyResult.Output
		}

		return StepResult{
			Status:   StepStatusCompleted,
			Output:   output,
			NextStep: step.Next,
			End:      step.End,
		}, nil
	}

	output := map[string]interface{}{
		"block": "catch",
	}

	if tryErr != nil {
		runContext["_error"] = tryErr.Error()
		output["try_error"] = tryErr.Error()
	}
	if tryResult.Error != nil {
		runContext["_error"] = *tryResult.Error
		output["try_error"] = *tryResult.Error
	}

	if raw.Catch != nil && len(raw.Catch.States) > 0 {
		catchResult, catchErr := e.executeBlock(ctx, raw.Catch, runContext)
		output["catch_output"] = catchResult.Output

		if catchErr != nil {
			output["catch_error"] = catchErr.Error()
		}
	}

	if raw.Finally != nil && len(raw.Finally.States) > 0 {
		finallyResult, _ := e.executeBlock(ctx, raw.Finally, runContext)
		output["finally_output"] = finallyResult.Output
	}

	return StepResult{
		Status:   StepStatusCompleted,
		Output:   output,
		NextStep: step.Next,
		End:      step.End,
	}, nil
}

func (e *TryCatchExecutor) executeBlock(ctx context.Context, block *workflows.IteratorSpec, runContext map[string]interface{}) (StepResult, error) {
	if block == nil || len(block.States) == 0 {
		return StepResult{Status: StepStatusCompleted}, nil
	}

	startState := block.Start
	if startState == "" && len(block.States) > 0 {
		startState = block.States[0].StableID()
	}

	stateMap := make(map[string]workflows.StateSpec)
	for _, s := range block.States {
		stateMap[s.StableID()] = s
	}

	currentID := startState
	var lastResult StepResult
	var lastErr error

	for currentID != "" {
		state, ok := stateMap[currentID]
		if !ok {
			break
		}

		step := workflows.ExecutionStep{
			ID:   state.StableID(),
			Type: classifyState(state),
			Next: state.Next,
			End:  state.End,
			Raw:  state,
		}

		executor, err := e.registry.GetExecutor(step.Type)
		if err != nil {
			return StepResult{
				Status: StepStatusFailed,
				Error:  strPtr(err.Error()),
			}, err
		}

		lastResult, lastErr = executor.Execute(ctx, step, runContext)
		if lastErr != nil {
			return lastResult, lastErr
		}

		if lastResult.Status == StepStatusFailed {
			return lastResult, nil
		}

		if lastResult.End || lastResult.NextStep == "" {
			break
		}
		currentID = lastResult.NextStep
	}

	return lastResult, lastErr
}

func classifyState(state workflows.StateSpec) workflows.ExecutionStepType {
	switch state.Type {
	case "agent":
		return workflows.StepTypeAgent
	case "switch":
		return workflows.StepTypeBranch
	case "inject", "set", "transform", "context":
		return workflows.StepTypeContextOp
	case "cron", "schedule":
		return workflows.StepTypeCron
	case "sleep", "delay", "timer":
		return workflows.StepTypeTimer
	case "foreach", "while":
		return workflows.StepTypeLoop
	case "parallel":
		return workflows.StepTypeParallel
	case "try":
		return workflows.StepTypeTryCatch
	case "human_approval", "await":
		return workflows.StepTypeAwait
	default:
		return workflows.StepTypeCustom
	}
}
