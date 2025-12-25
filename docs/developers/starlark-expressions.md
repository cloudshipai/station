# Starlark Expression Evaluation in Workflows

This document covers the implementation of Starlark expression evaluation in Station's workflow engine, including the `AttrDict` type that enables Python-like dot notation for accessing nested fields.

## Overview

Station uses [Starlark](https://github.com/google/starlark-go) as the expression language for workflow conditions (switch states, conditional branching). Starlark is a Python-like language designed for configuration, with the following properties:

- **Deterministic**: Same inputs always produce same outputs
- **Hermetic**: No I/O, no network, no file system access
- **Sandboxed**: Execution limits prevent infinite loops
- **Familiar**: Python-like syntax for easy adoption

## Core Implementation

**File**: `internal/workflows/runtime/starlark_eval.go`

### StarlarkEvaluator

The `StarlarkEvaluator` struct provides the main interface for evaluating expressions:

```go
type StarlarkEvaluator struct {
    maxSteps uint64  // Execution step limit (default: 10000)
}

// EvaluateCondition evaluates a boolean expression
func (e *StarlarkEvaluator) EvaluateCondition(expression string, data map[string]interface{}) (bool, error)

// EvaluateExpression evaluates an expression and returns the result
func (e *StarlarkEvaluator) EvaluateExpression(expression string, data map[string]interface{}) (interface{}, error)
```

### Type Conversion

Go types are converted to Starlark types via `goToStarlark()`:

| Go Type | Starlark Type |
|---------|---------------|
| `nil` | `starlark.None` |
| `bool` | `starlark.Bool` |
| `int`, `int64` | `starlark.Int` |
| `float64` | `starlark.Float` |
| `string` | `starlark.String` |
| `[]interface{}` | `*starlark.List` |
| `map[string]interface{}` | `*AttrDict` (custom type) |

## AttrDict: Enabling Dot Notation

### The Problem

Standard Starlark dictionaries (`*starlark.Dict`) only support bracket notation:

```python
# Works with standard Starlark dict
vuln["severity"] == "critical"

# FAILS with standard Starlark dict
vuln.severity == "critical"  # Error: dict has no .severity field or method
```

This is problematic for workflow conditions in `foreach` loops where users naturally expect dot notation:

```yaml
- id: route_by_severity
  type: switch
  dataPath: vuln
  conditions:
    - if: "vuln.severity == 'critical'"  # Users expect this to work!
      next: critical_path
```

### The Solution: AttrDict

`AttrDict` is a custom Starlark type that wraps a dictionary and implements the `HasAttrs` interface to enable dot notation access.

```go
type AttrDict struct {
    dict      *starlark.Dict
    evaluator *StarlarkEvaluator
}
```

### Interfaces Implemented

```go
var (
    _ starlark.Value      = (*AttrDict)(nil)  // Basic value interface
    _ starlark.Mapping    = (*AttrDict)(nil)  // dict["key"] access
    _ starlark.HasAttrs   = (*AttrDict)(nil)  // dict.key access (dot notation!)
    _ starlark.Iterable   = (*AttrDict)(nil)  // for key in dict
    _ starlark.Comparable = (*AttrDict)(nil)  // dict1 == dict2
)
```

### Key Methods

#### `Attr(name string)` - Enables Dot Notation

```go
func (d *AttrDict) Attr(name string) (starlark.Value, error) {
    val, found, err := d.dict.Get(starlark.String(name))
    if err != nil {
        return nil, err
    }
    if !found {
        return nil, starlark.NoSuchAttrError(
            fmt.Sprintf("attrdict has no .%s field or method", name))
    }
    return val, nil
}
```

When Starlark encounters `vuln.severity`, it:
1. Checks if `vuln` implements `HasAttrs`
2. Calls `vuln.Attr("severity")`
3. Returns the value or an error

#### `AttrNames()` - Lists Available Attributes

```go
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
```

### Recursive Wrapping

Nested maps are recursively wrapped with `AttrDict`, enabling deep access:

```go
func (e *StarlarkEvaluator) goToStarlark(v interface{}) starlark.Value {
    switch val := v.(type) {
    case map[string]interface{}:
        return NewAttrDict(e, val)  // Wraps nested maps too!
    // ... other types
    }
}

func NewAttrDict(evaluator *StarlarkEvaluator, data map[string]interface{}) *AttrDict {
    dict := starlark.NewDict(len(data))
    for k, v := range data {
        // goToStarlark recursively wraps nested maps
        _ = dict.SetKey(starlark.String(k), evaluator.goToStarlark(v))
    }
    return &AttrDict{dict: dict, evaluator: evaluator}
}
```

This enables expressions like:
```python
result.analysis.metrics.error_rate > 0.05
```

## Usage in Workflow Engine

### Switch Conditions

The switch executor uses `StarlarkEvaluator` to evaluate conditions:

```go
// internal/workflows/runtime/switch_inject.go
func (e *SwitchExecutor) Execute(ctx context.Context, step *workflows.StateSpec, runContext map[string]interface{}) (*StepResult, error) {
    evaluator := NewStarlarkEvaluator()
    
    for _, cond := range step.Conditions {
        result, err := evaluator.EvaluateCondition(cond.If, runContext)
        if err != nil {
            return nil, fmt.Errorf("evaluating condition %q: %w", cond.If, err)
        }
        if result {
            return &StepResult{Next: cond.Next}, nil
        }
    }
    // ... default handling
}
```

### Transform Expressions

The transform executor uses `EvaluateExpression` for data transformation:

```go
// internal/workflows/runtime/transform_executor.go
func (e *TransformExecutor) Execute(ctx context.Context, step *workflows.StateSpec, runContext map[string]interface{}) (*StepResult, error) {
    evaluator := NewStarlarkEvaluator()
    
    // Bind input data
    evalData := map[string]interface{}{
        "input": runContext["_stepInput"],
        "ctx":   runContext,
    }
    
    result, err := evaluator.EvaluateExpression(step.Expression, evalData)
    // ...
}
```

## Test Coverage

**File**: `internal/workflows/runtime/switch_inject_test.go`

### Dot Notation Test Cases

```go
{
    name:       "dot notation access",
    expression: "vuln.severity == 'critical'",
    data: map[string]interface{}{
        "vuln": map[string]interface{}{"severity": "critical", "exploitable": true},
    },
    want: true,
},
{
    name:       "dot notation with boolean",
    expression: "vuln.exploitable",
    data: map[string]interface{}{
        "vuln": map[string]interface{}{"severity": "critical", "exploitable": true},
    },
    want: true,
},
{
    name:       "dot notation compound expression",
    expression: "\"critical\" in str(vuln.severity).lower() and vuln.exploitable",
    data: map[string]interface{}{
        "vuln": map[string]interface{}{"severity": "CRITICAL", "exploitable": true},
    },
    want: true,
},
{
    name:       "nested dot notation",
    expression: "result.analysis.severity == 'high'",
    data: map[string]interface{}{
        "result": map[string]interface{}{
            "analysis": map[string]interface{}{"severity": "high"},
        },
    },
    want: true,
},
```

### Running Tests

```bash
cd /path/to/station
go test ./internal/workflows/runtime/... -v -run "TestStarlark"
```

## Error Handling

### Missing Attribute

When accessing a non-existent attribute:

```python
vuln.nonexistent  # Error: attrdict has no .nonexistent field or method
```

The error message clearly indicates:
1. The type (`attrdict`)
2. The missing attribute name

### Type Errors

When the value isn't a map:

```python
# If vuln is a string, not a map
vuln.severity  # Error: string has no .severity field or method
```

## Performance Considerations

1. **Execution Limits**: `maxSteps = 10000` prevents infinite loops
2. **Memory**: Each AttrDict wraps a Starlark dict; consider memory for very large nested structures
3. **Parsing**: Expression parsing happens once per evaluation; consider caching for hot paths

## Related Files

| File | Purpose |
|------|---------|
| `internal/workflows/runtime/starlark_eval.go` | AttrDict and StarlarkEvaluator implementation |
| `internal/workflows/runtime/switch_inject_test.go` | Test cases including dot notation |
| `internal/workflows/runtime/switch_inject.go` | Switch executor using the evaluator |
| `internal/workflows/runtime/transform_executor.go` | Transform executor using the evaluator |

## References

- [Starlark Language Spec](https://github.com/google/starlark-go/blob/master/doc/spec.md)
- [Starlark Go Implementation](https://github.com/google/starlark-go)
- [HasAttrs Interface](https://pkg.go.dev/go.starlark.net/starlark#HasAttrs)
