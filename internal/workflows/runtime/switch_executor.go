package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"station/internal/workflows"
)

var (
	ErrNoMatchingCondition = errors.New("no matching condition found")
	ErrInvalidDataPath     = errors.New("invalid data path")
	ErrConditionEvalFailed = errors.New("condition evaluation failed")
)

type SwitchExecutor struct {
	evaluator *StarlarkEvaluator
}

func NewSwitchExecutor() *SwitchExecutor {
	return &SwitchExecutor{
		evaluator: NewStarlarkEvaluator(),
	}
}

func (e *SwitchExecutor) SupportedTypes() []workflows.ExecutionStepType {
	return []workflows.ExecutionStepType{workflows.StepTypeBranch}
}

func (e *SwitchExecutor) Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
	raw := step.Raw

	if raw.Type != "switch" {
		return StepResult{
			Status:   StepStatusCompleted,
			Output:   map[string]interface{}{"skipped": true, "reason": "not a switch state"},
			NextStep: step.Next,
			End:      step.End,
		}, nil
	}

	evalData := runContext
	if raw.DataPath != "" {
		dataPath := raw.DataPath
		if !strings.HasPrefix(dataPath, "$.") && !strings.HasPrefix(dataPath, "$") {
			log.Printf("[DEPRECATION] switch step '%s': dataPath '%s' uses dot-notation. Use JSONPath '$.%s' instead.", step.ID, dataPath, dataPath)
		}

		val, ok := GetNestedValue(runContext, dataPath)
		if !ok {
			return StepResult{
				Status: StepStatusFailed,
				Error:  strPtr(fmt.Sprintf("data path not found: %s", dataPath)),
			}, ErrInvalidDataPath
		}

		merged := make(map[string]interface{})
		for k, v := range runContext {
			merged[k] = v
		}

		if m, ok := val.(map[string]interface{}); ok {
			for k, v := range m {
				merged[k] = v
			}
		}

		// 'result' and '_value' are standard variable names for condition expressions
		// 'val' is also supported to match documentation
		merged["result"] = val
		merged["_value"] = val
		merged["val"] = val

		evalData = merged
	}

	for _, cond := range raw.Conditions {
		match, err := e.evaluator.EvaluateCondition(cond.If, evalData)
		if err != nil {
			return StepResult{
				Status: StepStatusFailed,
				Error:  strPtr(fmt.Sprintf("condition eval failed: %s - %v", cond.If, err)),
			}, fmt.Errorf("%w: %v", ErrConditionEvalFailed, err)
		}

		if match {
			return StepResult{
				Status: StepStatusCompleted,
				Output: map[string]interface{}{
					"matched_condition": cond.If,
					"next_state":        cond.Next,
				},
				NextStep: cond.Next,
				End:      false,
			}, nil
		}
	}

	if raw.DefaultNext != "" {
		return StepResult{
			Status: StepStatusCompleted,
			Output: map[string]interface{}{
				"matched_condition": "default",
				"next_state":        raw.DefaultNext,
			},
			NextStep: raw.DefaultNext,
			End:      false,
		}, nil
	}

	return StepResult{
		Status: StepStatusFailed,
		Error:  strPtr("no matching condition and no default"),
	}, ErrNoMatchingCondition
}

func strPtr(s string) *string {
	return &s
}
