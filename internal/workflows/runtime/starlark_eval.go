package runtime

import (
	"fmt"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

type StarlarkEvaluator struct {
	maxSteps uint64
}

func NewStarlarkEvaluator() *StarlarkEvaluator {
	return &StarlarkEvaluator{
		maxSteps: 10000,
	}
}

func (e *StarlarkEvaluator) EvaluateCondition(expression string, data map[string]interface{}) (bool, error) {
	thread := &starlark.Thread{
		Name: "condition",
	}

	thread.SetMaxExecutionSteps(e.maxSteps)

	globals := e.convertToStarlark(data)

	fileOpts := syntax.FileOptions{}
	expr, err := fileOpts.ParseExpr("condition", expression, 0)
	if err != nil {
		return false, fmt.Errorf("parse error: %w", err)
	}

	result, err := starlark.EvalExprOptions(&fileOpts, thread, expr, globals)
	if err != nil {
		return false, fmt.Errorf("eval error: %w", err)
	}

	switch v := result.(type) {
	case starlark.Bool:
		return bool(v), nil
	case starlark.NoneType:
		return false, nil
	default:
		return result.Truth() == starlark.True, nil
	}
}

func (e *StarlarkEvaluator) EvaluateExpression(expression string, data map[string]interface{}) (interface{}, error) {
	thread := &starlark.Thread{
		Name: "expression",
	}

	thread.SetMaxExecutionSteps(e.maxSteps)

	globals := e.convertToStarlark(data)

	fileOpts := syntax.FileOptions{}
	expr, err := fileOpts.ParseExpr("expression", expression, 0)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	result, err := starlark.EvalExprOptions(&fileOpts, thread, expr, globals)
	if err != nil {
		return nil, fmt.Errorf("eval error: %w", err)
	}

	return e.convertFromStarlark(result), nil
}

func (e *StarlarkEvaluator) convertToStarlark(data map[string]interface{}) starlark.StringDict {
	globals := make(starlark.StringDict)
	for k, v := range data {
		globals[k] = e.goToStarlark(v)
	}
	return globals
}

func (e *StarlarkEvaluator) goToStarlark(v interface{}) starlark.Value {
	switch val := v.(type) {
	case nil:
		return starlark.None
	case bool:
		return starlark.Bool(val)
	case int:
		return starlark.MakeInt(val)
	case int64:
		return starlark.MakeInt64(val)
	case float64:
		return starlark.Float(val)
	case string:
		return starlark.String(val)
	case []interface{}:
		elems := make([]starlark.Value, len(val))
		for i, elem := range val {
			elems[i] = e.goToStarlark(elem)
		}
		return starlark.NewList(elems)
	case map[string]interface{}:
		dict := starlark.NewDict(len(val))
		for k, v := range val {
			_ = dict.SetKey(starlark.String(k), e.goToStarlark(v))
		}
		return dict
	default:
		return starlark.String(fmt.Sprintf("%v", val))
	}
}

func (e *StarlarkEvaluator) convertFromStarlark(v starlark.Value) interface{} {
	switch val := v.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(val)
	case starlark.Int:
		i, _ := val.Int64()
		return i
	case starlark.Float:
		return float64(val)
	case starlark.String:
		return string(val)
	case *starlark.List:
		result := make([]interface{}, val.Len())
		for i := 0; i < val.Len(); i++ {
			result[i] = e.convertFromStarlark(val.Index(i))
		}
		return result
	case *starlark.Dict:
		result := make(map[string]interface{})
		for _, item := range val.Items() {
			key := e.convertFromStarlark(item[0])
			if keyStr, ok := key.(string); ok {
				result[keyStr] = e.convertFromStarlark(item[1])
			}
		}
		return result
	default:
		return val.String()
	}
}

func GetNestedValue(data map[string]interface{}, path string) (interface{}, bool) {
	if path == "" {
		return data, true
	}

	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val
		default:
			return nil, false
		}
	}

	return current, true
}

func SetNestedValue(data map[string]interface{}, path string, value interface{}) {
	if path == "" {
		return
	}

	parts := strings.Split(path, ".")
	current := data

	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if _, ok := current[part]; !ok {
			current[part] = make(map[string]interface{})
		}
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			current[part] = make(map[string]interface{})
			current = current[part].(map[string]interface{})
		}
	}

	current[parts[len(parts)-1]] = value
}
