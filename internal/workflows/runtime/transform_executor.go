package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkjson"

	"station/internal/workflows"
)

type TransformExecutor struct{}

func NewTransformExecutor() *TransformExecutor {
	return &TransformExecutor{}
}

func (e *TransformExecutor) SupportedTypes() []workflows.ExecutionStepType {
	return []workflows.ExecutionStepType{workflows.StepTypeTransform}
}

func (e *TransformExecutor) Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]any) (StepResult, error) {
	expression := step.Raw.Expression
	if expression == "" {
		return StepResult{
			Status:   StepStatusCompleted,
			Output:   runContext,
			NextStep: step.Next,
			End:      step.End,
		}, nil
	}

	inputData := e.resolveInput(step, runContext)

	output, err := e.evaluateStarlark(expression, inputData, runContext)
	if err != nil {
		errStr := fmt.Sprintf("transform expression failed: %v", err)
		return StepResult{
			Status: StepStatusFailed,
			Error:  &errStr,
		}, err
	}

	return StepResult{
		Status:   StepStatusCompleted,
		Output:   output,
		NextStep: step.Next,
		End:      step.End,
	}, nil
}

func (e *TransformExecutor) resolveInput(step workflows.ExecutionStep, runContext map[string]any) map[string]any {
	if step.Raw.InputPath != "" && step.Raw.InputPath != "$" {
		return runContext
	}
	return runContext
}

func (e *TransformExecutor) evaluateStarlark(expression string, input map[string]any, runContext map[string]any) (map[string]any, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	ctxJSON, err := json.Marshal(runContext)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal context: %w", err)
	}

	predeclared := starlark.StringDict{
		"json": starlarkjson.Module,
	}

	inputVal, err := starlarkjson.Module.Members["decode"].(*starlark.Builtin).CallInternal(
		nil,
		starlark.Tuple{starlark.String(inputJSON)},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to decode input for starlark: %w", err)
	}
	predeclared["input"] = inputVal

	ctxVal, err := starlarkjson.Module.Members["decode"].(*starlark.Builtin).CallInternal(
		nil,
		starlark.Tuple{starlark.String(ctxJSON)},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to decode context for starlark: %w", err)
	}
	predeclared["ctx"] = ctxVal

	thread := &starlark.Thread{Name: "transform"}

	wrappedExpr := fmt.Sprintf("__result__ = %s", expression)

	globals, err := starlark.ExecFile(thread, "transform.star", wrappedExpr, predeclared)
	if err != nil {
		return nil, fmt.Errorf("starlark execution failed: %w", err)
	}

	result, ok := globals["__result__"]
	if !ok {
		return nil, fmt.Errorf("transform expression did not produce a result")
	}

	return starlarkToGo(result)
}

func starlarkToGo(v starlark.Value) (map[string]any, error) {
	encoded, err := starlarkjson.Module.Members["encode"].(*starlark.Builtin).CallInternal(
		nil,
		starlark.Tuple{v},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to encode starlark result: %w", err)
	}

	jsonStr := string(encoded.(starlark.String))

	var result map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		var singleValue any
		if err2 := json.Unmarshal([]byte(jsonStr), &singleValue); err2 == nil {
			return map[string]any{"result": singleValue}, nil
		}
		return nil, fmt.Errorf("failed to unmarshal starlark result: %w", err)
	}

	return result, nil
}
