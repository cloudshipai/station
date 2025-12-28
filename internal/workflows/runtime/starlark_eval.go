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

	globals["hasattr"] = starlark.NewBuiltin("hasattr", e.builtinHasattr)
	globals["getattr"] = starlark.NewBuiltin("getattr", e.builtinGetattr)

	fileOpts := syntax.FileOptions{}
	expr, err := fileOpts.ParseExpr("condition", expression, 0)
	if err != nil {
		return false, fmt.Errorf("parse error: %w", err)
	}

	result, err := starlark.EvalExprOptions(&fileOpts, thread, expr, globals)
	if err != nil {
		return false, e.enhanceStarlarkError(err, globals)
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
	globals["hasattr"] = starlark.NewBuiltin("hasattr", e.builtinHasattr)
	globals["getattr"] = starlark.NewBuiltin("getattr", e.builtinGetattr)

	fileOpts := syntax.FileOptions{}
	expr, err := fileOpts.ParseExpr("expression", expression, 0)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	result, err := starlark.EvalExprOptions(&fileOpts, thread, expr, globals)
	if err != nil {
		return nil, e.enhanceStarlarkError(err, globals)
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

	// Strip JSONPath prefix if present (e.g., "$.foo.bar" -> "foo.bar")
	if strings.HasPrefix(path, "$.") {
		path = path[2:]
	} else if path == "$" {
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

func (e *StarlarkEvaluator) builtinHasattr(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("hasattr: got %d arguments, want 2", len(args))
	}

	name, ok := args[1].(starlark.String)
	if !ok {
		return nil, fmt.Errorf("hasattr: name must be string, not %s", args[1].Type())
	}

	switch obj := args[0].(type) {
	case *AttrDict:
		_, found, _ := obj.dict.Get(starlark.String(name))
		return starlark.Bool(found), nil
	case *starlark.Dict:
		_, found, _ := obj.Get(starlark.String(name))
		return starlark.Bool(found), nil
	case starlark.HasAttrs:
		_, err := obj.Attr(string(name))
		return starlark.Bool(err == nil), nil
	default:
		return starlark.False, nil
	}
}

func (e *StarlarkEvaluator) builtinGetattr(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("getattr: got %d arguments, want 2 or 3", len(args))
	}

	name, ok := args[1].(starlark.String)
	if !ok {
		return nil, fmt.Errorf("getattr: name must be string, not %s", args[1].Type())
	}

	var defaultVal starlark.Value
	if len(args) == 3 {
		defaultVal = args[2]
	}

	switch obj := args[0].(type) {
	case *AttrDict:
		val, found, _ := obj.dict.Get(starlark.String(name))
		if found {
			return val, nil
		}
		if defaultVal != nil {
			return defaultVal, nil
		}
		return nil, fmt.Errorf("attrdict has no attribute '%s'", name)
	case *starlark.Dict:
		val, found, _ := obj.Get(starlark.String(name))
		if found {
			return val, nil
		}
		if defaultVal != nil {
			return defaultVal, nil
		}
		return nil, fmt.Errorf("dict has no key '%s'", name)
	case starlark.HasAttrs:
		val, err := obj.Attr(string(name))
		if err == nil {
			return val, nil
		}
		if defaultVal != nil {
			return defaultVal, nil
		}
		return nil, err
	default:
		if defaultVal != nil {
			return defaultVal, nil
		}
		return nil, fmt.Errorf("'%s' object has no attribute '%s'", args[0].Type(), name)
	}
}

func (e *StarlarkEvaluator) enhanceStarlarkError(err error, globals starlark.StringDict) error {
	errStr := err.Error()

	if !strings.Contains(errStr, "undefined:") && !strings.Contains(errStr, "undefined ") {
		return err
	}

	builtins := map[string]bool{
		"hasattr": true, "getattr": true,
	}

	var userVars []string
	for name := range globals {
		if !builtins[name] {
			userVars = append(userVars, name)
		}
	}
	sort.Strings(userVars)

	var sb strings.Builder
	sb.WriteString(errStr)
	sb.WriteString("\n\n")

	if len(userVars) > 0 {
		sb.WriteString("Available variables: ")
		sb.WriteString(strings.Join(userVars, ", "))
		sb.WriteString("\n")
	}

	sb.WriteString("\nHints:\n")
	sb.WriteString("  - Workflow inputs are flattened into context. Use 'ticket_id' not 'input.ticket_id'\n")
	sb.WriteString("  - Step outputs are stored under the step ID (e.g., 'classify_ticket' not 'classification')\n")
	sb.WriteString("  - Parallel branch results are stored under the parallel step ID as a list\n")
	sb.WriteString("  - Use hasattr(obj, 'field') to safely check if a field exists before accessing\n")
	sb.WriteString("  - Use getattr(obj, 'field', default) to get a field with a fallback value\n")

	return fmt.Errorf("%s", sb.String())
}
