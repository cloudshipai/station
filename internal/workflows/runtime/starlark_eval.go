package runtime

import (
	"fmt"
	"sort"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

type AttrDict struct {
	dict      *starlark.Dict
	evaluator *StarlarkEvaluator
}

var (
	_ starlark.Value      = (*AttrDict)(nil)
	_ starlark.Mapping    = (*AttrDict)(nil)
	_ starlark.HasAttrs   = (*AttrDict)(nil)
	_ starlark.Iterable   = (*AttrDict)(nil)
	_ starlark.Comparable = (*AttrDict)(nil)
)

func NewAttrDict(evaluator *StarlarkEvaluator, data map[string]interface{}) *AttrDict {
	dict := starlark.NewDict(len(data))
	for k, v := range data {
		_ = dict.SetKey(starlark.String(k), evaluator.goToStarlark(v))
	}
	return &AttrDict{dict: dict, evaluator: evaluator}
}

func (d *AttrDict) String() string        { return d.dict.String() }
func (d *AttrDict) Type() string          { return "attrdict" }
func (d *AttrDict) Freeze()               { d.dict.Freeze() }
func (d *AttrDict) Truth() starlark.Bool  { return d.dict.Truth() }
func (d *AttrDict) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: attrdict") }

func (d *AttrDict) Get(key starlark.Value) (v starlark.Value, found bool, err error) {
	return d.dict.Get(key)
}

func (d *AttrDict) Iterate() starlark.Iterator {
	return d.dict.Iterate()
}

func (d *AttrDict) CompareSameType(op syntax.Token, y starlark.Value, depth int) (bool, error) {
	other, ok := y.(*AttrDict)
	if !ok {
		return false, nil
	}
	return starlark.Compare(op, d.dict, other.dict)
}

func (d *AttrDict) Attr(name string) (starlark.Value, error) {
	val, found, err := d.dict.Get(starlark.String(name))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("attrdict has no .%s field or method", name))
	}
	return val, nil
}

func (d *AttrDict) AttrNames() []string {
	var names []string
	for _, item := range d.dict.Items() {
		if key, ok := item[0].(starlark.String); ok {
			names = append(names, string(key))
		}
	}
	sort.Strings(names)
	return names
}

func (d *AttrDict) Len() int {
	return d.dict.Len()
}

func (d *AttrDict) Items() []starlark.Tuple {
	return d.dict.Items()
}

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
		return NewAttrDict(e, val)
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
	case *AttrDict:
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
