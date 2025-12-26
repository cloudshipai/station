package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"station/internal/workflows"
)

var (
	ErrNoIterator         = errors.New("no iterator defined for foreach state")
	ErrNoItemsPath        = errors.New("itemsPath required for foreach state")
	ErrItemsNotFound      = errors.New("items not found at itemsPath")
	ErrItemsNotArray      = errors.New("items at itemsPath is not an array")
	ErrIterationFailed    = errors.New("one or more iterations failed")
	ErrIteratorNoStates   = errors.New("iterator has no states")
	ErrConcurrencyInvalid = errors.New("maxConcurrency must be >= 1")
)

type IteratorExecutorDeps interface {
	ExecuteStep(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error)
}

type ForeachExecutor struct {
	deps IteratorExecutorDeps
}

func NewForeachExecutor(deps IteratorExecutorDeps) *ForeachExecutor {
	return &ForeachExecutor{deps: deps}
}

func (e *ForeachExecutor) SupportedTypes() []workflows.ExecutionStepType {
	return []workflows.ExecutionStepType{workflows.StepTypeLoop}
}

type iterationResult struct {
	Index  int
	Output map[string]interface{}
	Error  error
}

func (e *ForeachExecutor) Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
	raw := step.Raw

	if raw.Type != "foreach" && raw.Type != "for" && raw.Type != "loop" {
		return StepResult{
			Status:   StepStatusCompleted,
			Output:   map[string]interface{}{"skipped": true, "reason": "not a foreach state"},
			NextStep: step.Next,
			End:      step.End,
		}, nil
	}

	if raw.ItemsPath == "" {
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr("itemsPath is required"),
		}, ErrNoItemsPath
	}

	if raw.Iterator == nil || len(raw.Iterator.States) == 0 {
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr("iterator with states is required"),
		}, ErrNoIterator
	}

	itemsRaw, ok := GetNestedValue(runContext, raw.ItemsPath)
	if !ok {
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr(fmt.Sprintf("items not found at path: %s", raw.ItemsPath)),
		}, ErrItemsNotFound
	}

	items, ok := itemsRaw.([]interface{})
	if !ok {
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr(fmt.Sprintf("items at %s is not an array", raw.ItemsPath)),
		}, ErrItemsNotArray
	}

	if len(items) == 0 {
		if raw.ResultPath != "" {
			SetNestedValue(runContext, raw.ResultPath, []interface{}{})
		}
		return StepResult{
			Status:   StepStatusCompleted,
			Output:   map[string]interface{}{"results": []interface{}{}, "count": 0},
			NextStep: step.Next,
			End:      step.End,
		}, nil
	}

	itemName := raw.ItemName
	if itemName == "" {
		itemName = "item"
	}

	maxConcurrency := raw.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}

	var results []iterationResult

	if maxConcurrency == 1 {
		results = e.executeSequential(ctx, items, itemName, raw.Iterator, runContext)
	} else {
		results = e.executeConcurrent(ctx, items, itemName, raw.Iterator, runContext, maxConcurrency)
	}

	outputs := make([]interface{}, len(items))
	var errs []error

	for _, result := range results {
		if result.Error != nil {
			errs = append(errs, fmt.Errorf("iteration %d: %w", result.Index, result.Error))
		}
		if result.Index < len(outputs) {
			outputs[result.Index] = result.Output
		}
	}

	if len(errs) > 0 {
		errMsg := fmt.Sprintf("%d iteration(s) failed", len(errs))
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr(errMsg),
			Output: map[string]interface{}{"results": outputs, "errors": len(errs)},
		}, fmt.Errorf("%w: %v", ErrIterationFailed, errs)
	}

	if raw.ResultPath != "" {
		SetNestedValue(runContext, raw.ResultPath, outputs)
	}

	return StepResult{
		Status:   StepStatusCompleted,
		Output:   map[string]interface{}{"results": outputs, "count": len(outputs)},
		NextStep: step.Next,
		End:      step.End,
	}, nil
}

func (e *ForeachExecutor) executeSequential(
	ctx context.Context,
	items []interface{},
	itemName string,
	iterator *workflows.IteratorSpec,
	baseContext map[string]interface{},
) []iterationResult {
	results := make([]iterationResult, len(items))

	for i, item := range items {
		select {
		case <-ctx.Done():
			results[i] = iterationResult{
				Index: i,
				Error: ctx.Err(),
			}
			return results
		default:
		}

		iterContext := deepCopyMap(baseContext)
		iterContext[itemName] = item
		iterContext["_index"] = i
		iterContext["_total"] = len(items)

		output, err := e.executeIterator(ctx, iterator, iterContext)
		results[i] = iterationResult{
			Index:  i,
			Output: output,
			Error:  err,
		}
	}

	return results
}

func (e *ForeachExecutor) executeConcurrent(
	ctx context.Context,
	items []interface{},
	itemName string,
	iterator *workflows.IteratorSpec,
	baseContext map[string]interface{},
	maxConcurrency int,
) []iterationResult {
	results := make([]iterationResult, len(items))
	sem := make(chan struct{}, maxConcurrency)
	resultChan := make(chan iterationResult, len(items))

	for i, item := range items {
		sem <- struct{}{}
		go func(index int, itm interface{}) {
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				resultChan <- iterationResult{
					Index: index,
					Error: ctx.Err(),
				}
				return
			default:
			}

			iterContext := deepCopyMap(baseContext)
			iterContext[itemName] = itm
			iterContext["_index"] = index
			iterContext["_total"] = len(items)

			output, err := e.executeIterator(ctx, iterator, iterContext)
			resultChan <- iterationResult{
				Index:  index,
				Output: output,
				Error:  err,
			}
		}(i, item)
	}

	for range items {
		result := <-resultChan
		results[result.Index] = result
	}

	return results
}

func (e *ForeachExecutor) executeIterator(
	ctx context.Context,
	iterator *workflows.IteratorSpec,
	iterContext map[string]interface{},
) (map[string]interface{}, error) {
	if len(iterator.States) == 0 {
		return nil, ErrIteratorNoStates
	}

	currentOutput := make(map[string]interface{})

	for i, state := range iterator.States {
		select {
		case <-ctx.Done():
			return currentOutput, ctx.Err()
		default:
		}

		iterStep := workflows.ExecutionStep{
			ID:   fmt.Sprintf("iter-%d", i),
			Type: classifyBranchState(state),
			Next: "",
			End:  state.End || i == len(iterator.States)-1,
			Raw:  state,
		}

		if i < len(iterator.States)-1 {
			iterStep.Next = iterator.States[i+1].StableID()
		}

		result, err := e.deps.ExecuteStep(ctx, iterStep, iterContext)
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
			SetNestedValue(iterContext, state.ResultPath, result.Output)
		}

		applyIteratorOutputMappings(iterContext, state.Output, result.Output)

		if iterStep.End || result.End {
			break
		}
	}

	return currentOutput, nil
}

func applyIteratorOutputMappings(context map[string]interface{}, outputMappings map[string]interface{}, stepOutput map[string]interface{}) {
	if outputMappings == nil || stepOutput == nil {
		return
	}

	for key, pathRaw := range outputMappings {
		path, ok := pathRaw.(string)
		if !ok {
			continue
		}

		value := resolveIteratorPath(stepOutput, path)
		if value != nil {
			context[key] = value
		}
	}
}

func resolveIteratorPath(data map[string]interface{}, path string) interface{} {
	if path == "" || path == "$" {
		return data
	}

	path = strings.TrimPrefix(path, "$.")

	if path == "result" {
		return extractIteratorResult(data)
	}

	parts := strings.Split(path, ".")

	var current interface{} = data
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil
			}
		default:
			return nil
		}
	}

	return current
}

func extractIteratorResult(data map[string]interface{}) interface{} {
	_, hasResponse := data["response"]
	_, hasAgentID := data["agent_id"]
	if hasResponse && hasAgentID {
		responseStr, ok := data["response"].(string)
		if ok {
			var parsed interface{}
			if err := json.Unmarshal([]byte(responseStr), &parsed); err == nil {
				return parsed
			}
		}
		return data["response"]
	}
	return data
}
