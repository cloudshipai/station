package dataflow

import (
	"encoding/json"
	"fmt"
	"strings"

	"station/internal/workflows"
)

type Resolver struct {
	definition *workflows.Definition
	stateIndex map[string]int
}

func NewResolver(def *workflows.Definition) *Resolver {
	idx := make(map[string]int)
	for i, s := range def.States {
		idx[s.StableID()] = i
	}
	return &Resolver{definition: def, stateIndex: idx}
}

func (r *Resolver) ResolveInput(stepID string, runContext map[string]any) (map[string]any, error) {
	prevStepID := r.findPreviousStep(stepID)

	if prevStepID == "" {
		return r.getWorkflowInput(runContext), nil
	}

	return r.getPreviousStepOutput(prevStepID, runContext), nil
}

func (r *Resolver) findPreviousStep(targetStepID string) string {
	if r.definition.Start == targetStepID {
		return ""
	}

	for _, state := range r.definition.States {
		nextID := state.Transition
		if nextID == "" {
			nextID = state.Next
		}
		if nextID == targetStepID {
			return state.StableID()
		}

		for _, cond := range state.Conditions {
			if cond.Next == targetStepID {
				return state.StableID()
			}
		}
		if state.DefaultNext == targetStepID {
			return state.StableID()
		}
	}

	return ""
}

func (r *Resolver) getWorkflowInput(runContext map[string]any) map[string]any {
	if input, ok := runContext["workflow"].(map[string]any); ok {
		if workflowInput, ok := input["input"].(map[string]any); ok {
			return workflowInput
		}
	}

	result := make(map[string]any)
	for k, v := range runContext {
		if !strings.HasPrefix(k, "_") && k != "steps" && k != "workflow" {
			result[k] = v
		}
	}
	return result
}

func (r *Resolver) getPreviousStepOutput(stepID string, runContext map[string]any) map[string]any {
	if steps, ok := runContext["steps"].(map[string]any); ok {
		if stepData, ok := steps[stepID].(map[string]any); ok {
			if output, ok := stepData["output"].(map[string]any); ok {
				return output
			}
			return stepData
		}
	}

	if stepData, ok := runContext[stepID].(map[string]any); ok {
		if output, ok := stepData["output"].(map[string]any); ok {
			return output
		}
		return stepData
	}

	return make(map[string]any)
}

func (r *Resolver) ApplyInputPath(input map[string]any, inputPath string) (map[string]any, error) {
	if inputPath == "" || inputPath == "$" {
		return input, nil
	}

	path := strings.TrimPrefix(inputPath, "$.")
	return getNestedValue(input, path)
}

func (r *Resolver) ApplyOutputPath(output map[string]any, outputPath string) (map[string]any, error) {
	if outputPath == "" || outputPath == "$" {
		return output, nil
	}

	path := strings.TrimPrefix(outputPath, "$.")
	return getNestedValue(output, path)
}

func getNestedValue(data map[string]any, path string) (map[string]any, error) {
	if path == "" {
		return data, nil
	}

	parts := strings.Split(path, ".")
	current := any(data)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil, fmt.Errorf("path '%s' not found at '%s'", path, part)
			}
		default:
			return nil, fmt.Errorf("cannot traverse path '%s': not an object at '%s'", path, part)
		}
	}

	if result, ok := current.(map[string]any); ok {
		return result, nil
	}

	return map[string]any{"value": current}, nil
}

func AggregateParallelOutputs(branchOutputs map[string]map[string]any, mode string) map[string]any {
	if mode == "" {
		mode = "merge"
	}

	switch mode {
	case "merge":
		result := make(map[string]any)
		for branchName, output := range branchOutputs {
			result[branchName] = output
		}
		return result

	case "array":
		var results []any
		for _, output := range branchOutputs {
			results = append(results, output)
		}
		return map[string]any{"results": results}

	case "first":
		for _, output := range branchOutputs {
			return output
		}
		return make(map[string]any)

	default:
		result := make(map[string]any)
		for branchName, output := range branchOutputs {
			result[branchName] = output
		}
		return result
	}
}

type ForeachIterationResult struct {
	Item   any            `json:"item"`
	Index  int            `json:"index"`
	Output map[string]any `json:"output"`
}

func AggregateForeachOutputs(results []ForeachIterationResult) map[string]any {
	var items []any
	for _, r := range results {
		items = append(items, map[string]any{
			"item":   r.Item,
			"index":  r.Index,
			"output": r.Output,
		})
	}
	return map[string]any{
		"count":   len(results),
		"results": items,
	}
}

func PrepareIterationInput(baseInput map[string]any, item any, index int, itemVariable string) map[string]any {
	result := make(map[string]any)
	for k, v := range baseInput {
		result[k] = v
	}

	result["_item"] = item
	result["_index"] = index

	if itemVariable != "" {
		result[itemVariable] = item
	}

	return result
}

func ToJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
