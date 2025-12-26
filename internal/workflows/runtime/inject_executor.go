package runtime

import (
	"context"
	"errors"

	"station/internal/workflows"
)

var (
	ErrNoDataToInject = errors.New("no data to inject")
)

type InjectExecutor struct{}

func NewInjectExecutor() *InjectExecutor {
	return &InjectExecutor{}
}

func (e *InjectExecutor) SupportedTypes() []workflows.ExecutionStepType {
	return []workflows.ExecutionStepType{workflows.StepTypeContextOp}
}

func (e *InjectExecutor) Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
	raw := step.Raw

	if raw.Type != "inject" && raw.Type != "set" && raw.Type != "transform" {
		return StepResult{
			Status:   StepStatusCompleted,
			Output:   map[string]interface{}{"skipped": true, "reason": "not an inject/set/transform state"},
			NextStep: step.Next,
			End:      step.End,
		}, nil
	}

	if raw.Data == nil || len(raw.Data) == 0 {
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr("no data provided for inject"),
		}, ErrNoDataToInject
	}

	resultPath := raw.ResultPath
	if resultPath == "" {
		resultPath = "ctx"
	}

	injectedData := deepCopyMap(raw.Data)

	SetNestedValue(runContext, resultPath, injectedData)

	return StepResult{
		Status:   StepStatusCompleted,
		Output:   injectedData,
		NextStep: step.Next,
		End:      step.End,
	}, nil
}

func deepCopyMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}

	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		switch val := v.(type) {
		case map[string]interface{}:
			dst[k] = deepCopyMap(val)
		case []interface{}:
			dst[k] = deepCopySlice(val)
		default:
			dst[k] = v
		}
	}
	return dst
}

func deepCopySlice(src []interface{}) []interface{} {
	if src == nil {
		return nil
	}

	dst := make([]interface{}, len(src))
	for i, v := range src {
		switch val := v.(type) {
		case map[string]interface{}:
			dst[i] = deepCopyMap(val)
		case []interface{}:
			dst[i] = deepCopySlice(val)
		default:
			dst[i] = v
		}
	}
	return dst
}
