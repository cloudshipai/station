package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
		"sum":  starlark.NewBuiltin("sum", builtinSum),
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

	for key, value := range runContext {
		valJSON, err := json.Marshal(value)
		if err != nil {
			continue
		}
		val, err := starlarkjson.Module.Members["decode"].(*starlark.Builtin).CallInternal(
			nil,
			starlark.Tuple{starlark.String(valJSON)},
			nil,
		)
		if err != nil {
			continue
		}
		predeclared[sanitizeIdentifier(key)] = val
	}

	thread := &starlark.Thread{Name: "transform"}

	contextKeys := make([]string, 0, len(runContext))
	for key := range runContext {
		contextKeys = append(contextKeys, key)
	}

	wrappedExpr := wrapStarlarkExpression(expression, contextKeys)

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

func wrapStarlarkExpression(expression string, contextKeys []string) string {
	expression = strings.TrimSpace(expression)

	if strings.Contains(expression, "__result__") {
		return expression
	}

	lines := strings.Split(expression, "\n")

	if len(lines) == 1 && !hasTopLevelControlFlow(expression) {
		return fmt.Sprintf("__result__ = %s", expression)
	}

	if hasTopLevelControlFlow(expression) {
		return wrapInFunction(lines, contextKeys)
	}

	return wrapSimpleMultiline(lines)
}

func hasTopLevelControlFlow(expression string) bool {
	for _, line := range strings.Split(expression, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			if strings.HasPrefix(trimmed, "for ") ||
				strings.HasPrefix(trimmed, "while ") ||
				strings.HasPrefix(trimmed, "if ") {
				return true
			}
		}
	}
	return false
}

func sanitizeIdentifier(name string) string {
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}
	return name
}

func wrapInFunction(lines []string, contextKeys []string) string {
	var result strings.Builder

	sanitizedKeys := make([]string, len(contextKeys))
	for i, key := range contextKeys {
		sanitizedKeys[i] = sanitizeIdentifier(key)
	}

	params := strings.Join(sanitizedKeys, ", ")
	result.WriteString(fmt.Sprintf("def __transform__(%s):\n", params))

	lastMeaningfulIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			lastMeaningfulIdx = i
			break
		}
	}

	var lastAssignedVar string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			result.WriteString("\n")
			continue
		}

		result.WriteString("    ")

		if i == lastMeaningfulIdx {
			if isReturnableExpression(trimmed) {
				result.WriteString("return ")
				result.WriteString(trimmed)
				result.WriteString("\n")
				continue
			}

			if isAssignment(trimmed) {
				lastAssignedVar = extractAssignedVar(trimmed)
				result.WriteString(line)
				result.WriteString("\n")
				continue
			}
		}

		result.WriteString(line)
		result.WriteString("\n")
	}

	if lastAssignedVar != "" {
		result.WriteString("    return ")
		result.WriteString(lastAssignedVar)
		result.WriteString("\n")
	}

	result.WriteString(fmt.Sprintf("__result__ = __transform__(%s)\n", params))

	return result.String()
}

func wrapSimpleMultiline(lines []string) string {
	lastMeaningfulIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			lastMeaningfulIdx = i
			break
		}
	}

	if lastMeaningfulIdx == -1 {
		return strings.Join(lines, "\n")
	}

	lastLine := strings.TrimSpace(lines[lastMeaningfulIdx])

	if isAssignment(lastLine) {
		varName := extractAssignedVar(lastLine)
		lines = append(lines, fmt.Sprintf("__result__ = %s", varName))
		return strings.Join(lines, "\n")
	}

	lines[lastMeaningfulIdx] = fmt.Sprintf("__result__ = %s", lastLine)
	return strings.Join(lines, "\n")
}

func isReturnableExpression(line string) bool {
	if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[") {
		return true
	}
	if !strings.Contains(line, "=") || strings.Contains(line, "==") ||
		strings.Contains(line, "!=") || strings.Contains(line, "<=") ||
		strings.Contains(line, ">=") {
		if !strings.HasPrefix(line, "def ") &&
			!strings.HasPrefix(line, "if ") &&
			!strings.HasPrefix(line, "for ") &&
			!strings.HasPrefix(line, "while ") &&
			!strings.HasPrefix(line, "return ") &&
			!strings.HasPrefix(line, "pass") &&
			!strings.HasPrefix(line, "break") &&
			!strings.HasPrefix(line, "continue") {
			return true
		}
	}
	return false
}

func isAssignment(line string) bool {
	return strings.Contains(line, "=") &&
		!strings.Contains(line, "==") &&
		!strings.Contains(line, "!=") &&
		!strings.Contains(line, "<=") &&
		!strings.Contains(line, ">=")
}

func extractAssignedVar(line string) string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return ""
	}
	varName := strings.TrimSpace(parts[0])
	if idx := strings.Index(varName, "["); idx != -1 {
		varName = strings.TrimSpace(varName[:idx])
	}
	return varName
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

func builtinSum(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("sum: expected 1 argument, got %d", len(args))
	}

	iter, ok := args[0].(starlark.Iterable)
	if !ok {
		return nil, fmt.Errorf("sum: argument must be iterable")
	}

	var total float64
	iterator := iter.Iterate()
	defer iterator.Done()

	var x starlark.Value
	for iterator.Next(&x) {
		switch v := x.(type) {
		case starlark.Int:
			i, _ := v.Int64()
			total += float64(i)
		case starlark.Float:
			total += float64(v)
		default:
			return nil, fmt.Errorf("sum: unsupported type %s", x.Type())
		}
	}

	if total == float64(int64(total)) {
		return starlark.MakeInt64(int64(total)), nil
	}
	return starlark.Float(total), nil
}
