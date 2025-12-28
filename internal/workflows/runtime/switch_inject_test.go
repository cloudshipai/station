package runtime

import (
	"context"
	"testing"

	"station/internal/workflows"
)

// ============================================================================
// Starlark Evaluator Tests
// ============================================================================

func TestStarlarkEvaluator_EvaluateCondition(t *testing.T) {
	eval := NewStarlarkEvaluator()

	tests := []struct {
		name       string
		expression string
		data       map[string]interface{}
		want       bool
		wantErr    bool
	}{
		// Basic boolean expressions
		{
			name:       "simple true",
			expression: "True",
			data:       map[string]interface{}{},
			want:       true,
		},
		{
			name:       "simple false",
			expression: "False",
			data:       map[string]interface{}{},
			want:       false,
		},

		// Numeric comparisons
		{
			name:       "greater than true",
			expression: "error_rate > 0.05",
			data:       map[string]interface{}{"error_rate": 0.1},
			want:       true,
		},
		{
			name:       "greater than false",
			expression: "error_rate > 0.05",
			data:       map[string]interface{}{"error_rate": 0.01},
			want:       false,
		},
		{
			name:       "less than or equal",
			expression: "count <= 10",
			data:       map[string]interface{}{"count": 10},
			want:       true,
		},
		{
			name:       "integer comparison",
			expression: "retries >= 3",
			data:       map[string]interface{}{"retries": 5},
			want:       true,
		},

		// String comparisons
		{
			name:       "string equality true",
			expression: "status == 'degraded'",
			data:       map[string]interface{}{"status": "degraded"},
			want:       true,
		},
		{
			name:       "string equality false",
			expression: "status == 'healthy'",
			data:       map[string]interface{}{"status": "degraded"},
			want:       false,
		},
		{
			name:       "string inequality",
			expression: "env != 'production'",
			data:       map[string]interface{}{"env": "staging"},
			want:       true,
		},

		// Boolean variables
		{
			name:       "boolean variable true",
			expression: "is_critical",
			data:       map[string]interface{}{"is_critical": true},
			want:       true,
		},
		{
			name:       "boolean variable false",
			expression: "is_critical",
			data:       map[string]interface{}{"is_critical": false},
			want:       false,
		},
		{
			name:       "negation",
			expression: "not is_healthy",
			data:       map[string]interface{}{"is_healthy": false},
			want:       true,
		},

		// Compound expressions
		{
			name:       "and expression true",
			expression: "error_rate > 0.05 and status == 'degraded'",
			data:       map[string]interface{}{"error_rate": 0.1, "status": "degraded"},
			want:       true,
		},
		{
			name:       "and expression false",
			expression: "error_rate > 0.05 and status == 'healthy'",
			data:       map[string]interface{}{"error_rate": 0.1, "status": "degraded"},
			want:       false,
		},
		{
			name:       "or expression",
			expression: "error_rate > 0.1 or latency > 500",
			data:       map[string]interface{}{"error_rate": 0.05, "latency": 600},
			want:       true,
		},

		{
			name:       "dict access",
			expression: "result['status'] == 'ok'",
			data: map[string]interface{}{
				"result": map[string]interface{}{"status": "ok", "code": 200},
			},
			want: true,
		},
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
				"vuln": map[string]interface{}{"severity": "critical", "exploitable": true},
			},
			want: true,
		},
		{
			name:       "nested dot notation",
			expression: "result.data.status == 'ok'",
			data: map[string]interface{}{
				"result": map[string]interface{}{
					"data": map[string]interface{}{"status": "ok"},
				},
			},
			want: true,
		},

		// List operations
		{
			name:       "in list",
			expression: "'error' in tags",
			data:       map[string]interface{}{"tags": []interface{}{"error", "critical"}},
			want:       true,
		},
		{
			name:       "not in list",
			expression: "'warning' in tags",
			data:       map[string]interface{}{"tags": []interface{}{"error", "critical"}},
			want:       false,
		},
		{
			name:       "list length",
			expression: "len(items) > 0",
			data:       map[string]interface{}{"items": []interface{}{"a", "b"}},
			want:       true,
		},

		// Nil/None handling
		{
			name:       "none equals none",
			expression: "value == None",
			data:       map[string]interface{}{"value": nil},
			want:       true,
		},

		// Truthiness
		{
			name:       "empty string is falsy",
			expression: "not message",
			data:       map[string]interface{}{"message": ""},
			want:       true,
		},
		{
			name:       "non-empty string is truthy",
			expression: "bool(message)",
			data:       map[string]interface{}{"message": "hello"},
			want:       true,
		},

		// Error cases
		{
			name:       "syntax error",
			expression: "error_rate >",
			data:       map[string]interface{}{"error_rate": 0.1},
			wantErr:    true,
		},
		{
			name:       "undefined variable",
			expression: "undefined_var > 5",
			data:       map[string]interface{}{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := eval.EvaluateCondition(tt.expression, tt.data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("EvaluateCondition(%q) = %v, want %v", tt.expression, got, tt.want)
			}
		})
	}
}

func TestStarlarkEvaluator_EvaluateExpression(t *testing.T) {
	eval := NewStarlarkEvaluator()

	tests := []struct {
		name       string
		expression string
		data       map[string]interface{}
		want       interface{}
		wantErr    bool
	}{
		{
			name:       "simple arithmetic",
			expression: "a + b",
			data:       map[string]interface{}{"a": 10, "b": 5},
			want:       int64(15),
		},
		{
			name:       "string concatenation",
			expression: "prefix + '-' + suffix",
			data:       map[string]interface{}{"prefix": "hello", "suffix": "world"},
			want:       "hello-world",
		},
		{
			name:       "list access",
			expression: "items[0]",
			data:       map[string]interface{}{"items": []interface{}{"first", "second"}},
			want:       "first",
		},
		{
			name:       "dict access",
			expression: "config['timeout']",
			data: map[string]interface{}{
				"config": map[string]interface{}{"timeout": 30},
			},
			want: int64(30),
		},
		{
			name:       "conditional expression",
			expression: "'high' if severity > 5 else 'low'",
			data:       map[string]interface{}{"severity": 8},
			want:       "high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := eval.EvaluateExpression(tt.expression, tt.data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("EvaluateExpression(%q) = %v (%T), want %v (%T)", tt.expression, got, got, tt.want, tt.want)
			}
		})
	}
}

// ============================================================================
// GetNestedValue / SetNestedValue Tests
// ============================================================================

func TestGetNestedValue(t *testing.T) {
	tests := []struct {
		name   string
		data   map[string]interface{}
		path   string
		want   interface{}
		wantOK bool
	}{
		{
			name:   "empty path returns full map",
			data:   map[string]interface{}{"a": 1},
			path:   "",
			want:   map[string]interface{}{"a": 1},
			wantOK: true,
		},
		{
			name:   "single level",
			data:   map[string]interface{}{"status": "ok"},
			path:   "status",
			want:   "ok",
			wantOK: true,
		},
		{
			name: "nested two levels",
			data: map[string]interface{}{
				"result": map[string]interface{}{"code": 200},
			},
			path:   "result.code",
			want:   200,
			wantOK: true,
		},
		{
			name: "nested three levels",
			data: map[string]interface{}{
				"steps": map[string]interface{}{
					"analysis": map[string]interface{}{
						"error_rate": 0.05,
					},
				},
			},
			path:   "steps.analysis.error_rate",
			want:   0.05,
			wantOK: true,
		},
		{
			name:   "missing key",
			data:   map[string]interface{}{"a": 1},
			path:   "b",
			want:   nil,
			wantOK: false,
		},
		{
			name: "missing nested key",
			data: map[string]interface{}{
				"result": map[string]interface{}{"code": 200},
			},
			path:   "result.message",
			want:   nil,
			wantOK: false,
		},
		{
			name:   "path through non-map",
			data:   map[string]interface{}{"value": "string"},
			path:   "value.nested",
			want:   nil,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := GetNestedValue(tt.data, tt.path)

			if ok != tt.wantOK {
				t.Errorf("GetNestedValue() ok = %v, want %v", ok, tt.wantOK)
				return
			}

			if tt.wantOK {
				// For map comparison, check if both are maps
				if gotMap, isMap := got.(map[string]interface{}); isMap {
					wantMap, _ := tt.want.(map[string]interface{})
					if len(gotMap) != len(wantMap) {
						t.Errorf("GetNestedValue() = %v, want %v", got, tt.want)
					}
				} else if got != tt.want {
					t.Errorf("GetNestedValue() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestSetNestedValue(t *testing.T) {
	tests := []struct {
		name     string
		initial  map[string]interface{}
		path     string
		value    interface{}
		checkKey string
		wantVal  interface{}
	}{
		{
			name:     "single level",
			initial:  map[string]interface{}{},
			path:     "status",
			value:    "ok",
			checkKey: "status",
			wantVal:  "ok",
		},
		{
			name:     "nested creates intermediate maps",
			initial:  map[string]interface{}{},
			path:     "config.timeout",
			value:    30,
			checkKey: "config.timeout",
			wantVal:  30,
		},
		{
			name:     "deep nesting",
			initial:  map[string]interface{}{},
			path:     "a.b.c.d",
			value:    "deep",
			checkKey: "a.b.c.d",
			wantVal:  "deep",
		},
		{
			name: "overwrites existing",
			initial: map[string]interface{}{
				"config": map[string]interface{}{"timeout": 10},
			},
			path:     "config.timeout",
			value:    60,
			checkKey: "config.timeout",
			wantVal:  60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetNestedValue(tt.initial, tt.path, tt.value)

			got, ok := GetNestedValue(tt.initial, tt.checkKey)
			if !ok {
				t.Errorf("SetNestedValue() did not set value at path %q", tt.checkKey)
				return
			}

			if got != tt.wantVal {
				t.Errorf("SetNestedValue() value = %v, want %v", got, tt.wantVal)
			}
		})
	}
}

func TestSetNestedValue_EmptyPath(t *testing.T) {
	data := map[string]interface{}{"existing": "value"}
	SetNestedValue(data, "", "ignored")

	// Empty path should do nothing
	if _, ok := data["existing"]; !ok {
		t.Error("SetNestedValue with empty path modified the map unexpectedly")
	}
}

// ============================================================================
// Switch Executor Tests
// ============================================================================

func TestSwitchExecutor_SupportedTypes(t *testing.T) {
	executor := NewSwitchExecutor()
	types := executor.SupportedTypes()

	if len(types) != 1 {
		t.Errorf("expected 1 supported type, got %d", len(types))
	}

	if types[0] != workflows.StepTypeBranch {
		t.Errorf("expected StepTypeBranch, got %v", types[0])
	}
}

func TestSwitchExecutor_Execute(t *testing.T) {
	executor := NewSwitchExecutor()

	tests := []struct {
		name         string
		step         workflows.ExecutionStep
		runContext   map[string]interface{}
		wantStatus   StepStatus
		wantNextStep string
		wantErr      error
	}{
		{
			name: "matches first condition",
			step: workflows.ExecutionStep{
				ID:   "decide",
				Type: workflows.StepTypeBranch,
				Raw: workflows.StateSpec{
					Type: "switch",
					Conditions: []workflows.SwitchCondition{
						{If: "error_rate > 0.05", Next: "alert"},
						{If: "error_rate > 0.01", Next: "warn"},
					},
					DefaultNext: "ok",
				},
			},
			runContext:   map[string]interface{}{"error_rate": 0.1},
			wantStatus:   StepStatusCompleted,
			wantNextStep: "alert",
		},
		{
			name: "matches second condition",
			step: workflows.ExecutionStep{
				ID:   "decide",
				Type: workflows.StepTypeBranch,
				Raw: workflows.StateSpec{
					Type: "switch",
					Conditions: []workflows.SwitchCondition{
						{If: "error_rate > 0.05", Next: "alert"},
						{If: "error_rate > 0.01", Next: "warn"},
					},
					DefaultNext: "ok",
				},
			},
			runContext:   map[string]interface{}{"error_rate": 0.03},
			wantStatus:   StepStatusCompleted,
			wantNextStep: "warn",
		},
		{
			name: "falls through to default",
			step: workflows.ExecutionStep{
				ID:   "decide",
				Type: workflows.StepTypeBranch,
				Raw: workflows.StateSpec{
					Type: "switch",
					Conditions: []workflows.SwitchCondition{
						{If: "error_rate > 0.05", Next: "alert"},
						{If: "error_rate > 0.01", Next: "warn"},
					},
					DefaultNext: "ok",
				},
			},
			runContext:   map[string]interface{}{"error_rate": 0.001},
			wantStatus:   StepStatusCompleted,
			wantNextStep: "ok",
		},
		{
			name: "no match and no default",
			step: workflows.ExecutionStep{
				ID:   "decide",
				Type: workflows.StepTypeBranch,
				Raw: workflows.StateSpec{
					Type: "switch",
					Conditions: []workflows.SwitchCondition{
						{If: "error_rate > 0.05", Next: "alert"},
					},
				},
			},
			runContext: map[string]interface{}{"error_rate": 0.01},
			wantStatus: StepStatusFailed,
			wantErr:    ErrNoMatchingCondition,
		},
		{
			name: "uses dataPath to scope evaluation",
			step: workflows.ExecutionStep{
				ID:   "decide",
				Type: workflows.StepTypeBranch,
				Raw: workflows.StateSpec{
					Type:     "switch",
					DataPath: "steps.analysis.result",
					Conditions: []workflows.SwitchCondition{
						{If: "status == 'critical'", Next: "escalate"},
						{If: "status == 'warning'", Next: "notify"},
					},
					DefaultNext: "ok",
				},
			},
			runContext: map[string]interface{}{
				"steps": map[string]interface{}{
					"analysis": map[string]interface{}{
						"result": map[string]interface{}{
							"status":     "critical",
							"error_rate": 0.15,
						},
					},
				},
			},
			wantStatus:   StepStatusCompleted,
			wantNextStep: "escalate",
		},
		{
			name: "dataPath with scalar value uses val (documented)",
			step: workflows.ExecutionStep{
				ID:   "decide",
				Type: workflows.StepTypeBranch,
				Raw: workflows.StateSpec{
					Type:     "switch",
					DataPath: "result.code",
					Conditions: []workflows.SwitchCondition{
						{If: "val == 200", Next: "success"},
						{If: "val >= 400", Next: "error"},
					},
					DefaultNext: "retry",
				},
			},
			runContext: map[string]interface{}{
				"result": map[string]interface{}{
					"code": 200,
				},
			},
			wantStatus:   StepStatusCompleted,
			wantNextStep: "success",
		},
		{
			name: "dataPath with scalar value uses _value",
			step: workflows.ExecutionStep{
				ID:   "decide",
				Type: workflows.StepTypeBranch,
				Raw: workflows.StateSpec{
					Type:     "switch",
					DataPath: "result.code",
					Conditions: []workflows.SwitchCondition{
						{If: "_value == 200", Next: "success"},
						{If: "_value >= 400", Next: "error"},
					},
					DefaultNext: "retry",
				},
			},
			runContext: map[string]interface{}{
				"result": map[string]interface{}{
					"code": 200,
				},
			},
			wantStatus:   StepStatusCompleted,
			wantNextStep: "success",
		},
		{
			name: "invalid dataPath",
			step: workflows.ExecutionStep{
				ID:   "decide",
				Type: workflows.StepTypeBranch,
				Raw: workflows.StateSpec{
					Type:     "switch",
					DataPath: "nonexistent.path",
					Conditions: []workflows.SwitchCondition{
						{If: "status == 'ok'", Next: "continue"},
					},
				},
			},
			runContext: map[string]interface{}{"other": "data"},
			wantStatus: StepStatusFailed,
			wantErr:    ErrInvalidDataPath,
		},
		{
			name: "condition evaluation error",
			step: workflows.ExecutionStep{
				ID:   "decide",
				Type: workflows.StepTypeBranch,
				Raw: workflows.StateSpec{
					Type: "switch",
					Conditions: []workflows.SwitchCondition{
						{If: "undefined_var > 5", Next: "next"},
					},
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusFailed,
			wantErr:    ErrConditionEvalFailed,
		},
		{
			name: "skips non-switch type",
			step: workflows.ExecutionStep{
				ID:   "not-switch",
				Type: workflows.StepTypeBranch,
				Next: "next-step",
				Raw: workflows.StateSpec{
					Type: "operation",
				},
			},
			runContext:   map[string]interface{}{},
			wantStatus:   StepStatusCompleted,
			wantNextStep: "next-step",
		},
		{
			name: "string comparison with status",
			step: workflows.ExecutionStep{
				ID:   "status-check",
				Type: workflows.StepTypeBranch,
				Raw: workflows.StateSpec{
					Type: "switch",
					Conditions: []workflows.SwitchCondition{
						{If: "status == 'healthy'", Next: "done"},
						{If: "status == 'degraded'", Next: "investigate"},
						{If: "status == 'critical'", Next: "page"},
					},
					DefaultNext: "unknown",
				},
			},
			runContext:   map[string]interface{}{"status": "degraded"},
			wantStatus:   StepStatusCompleted,
			wantNextStep: "investigate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.Execute(context.Background(), tt.step, tt.runContext)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				// Check if it's wrapped correctly
				if err != tt.wantErr && !containsError(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", result.Status, tt.wantStatus)
			}

			if tt.wantNextStep != "" && result.NextStep != tt.wantNextStep {
				t.Errorf("NextStep = %v, want %v", result.NextStep, tt.wantNextStep)
			}
		})
	}
}

// ============================================================================
// Inject Executor Tests
// ============================================================================

func TestInjectExecutor_SupportedTypes(t *testing.T) {
	executor := NewInjectExecutor()
	types := executor.SupportedTypes()

	if len(types) != 1 {
		t.Errorf("expected 1 supported type, got %d", len(types))
	}

	if types[0] != workflows.StepTypeContextOp {
		t.Errorf("expected StepTypeContextOp, got %v", types[0])
	}
}

func TestInjectExecutor_Execute(t *testing.T) {
	executor := NewInjectExecutor()

	tests := []struct {
		name           string
		step           workflows.ExecutionStep
		runContext     map[string]interface{}
		wantStatus     StepStatus
		wantErr        error
		checkPath      string
		checkValue     interface{}
		checkValueFunc func(t *testing.T, ctx map[string]interface{})
	}{
		{
			name: "injects data at default path (ctx)",
			step: workflows.ExecutionStep{
				ID:   "set-config",
				Type: workflows.StepTypeContextOp,
				Next: "next-step",
				Raw: workflows.StateSpec{
					Type: "inject",
					Data: map[string]interface{}{
						"timeout":    30,
						"retries":    3,
						"batch_size": 100,
					},
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusCompleted,
			checkValueFunc: func(t *testing.T, ctx map[string]interface{}) {
				injected, ok := GetNestedValue(ctx, "ctx")
				if !ok {
					t.Error("expected ctx to be set")
					return
				}
				injectedMap, ok := injected.(map[string]interface{})
				if !ok {
					t.Errorf("expected ctx to be map, got %T", injected)
					return
				}
				if injectedMap["timeout"] != 30 {
					t.Errorf("timeout = %v, want 30", injectedMap["timeout"])
				}
			},
		},
		{
			name: "injects data at custom resultPath",
			step: workflows.ExecutionStep{
				ID:   "set-thresholds",
				Type: workflows.StepTypeContextOp,
				Next: "analyze",
				Raw: workflows.StateSpec{
					Type: "inject",
					Data: map[string]interface{}{
						"error_rate_max": 0.05,
						"latency_p99":    500,
					},
					ResultPath: "config.thresholds",
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusCompleted,
			checkValueFunc: func(t *testing.T, ctx map[string]interface{}) {
				val, ok := GetNestedValue(ctx, "config.thresholds.error_rate_max")
				if !ok {
					t.Error("expected config.thresholds.error_rate_max to be set")
					return
				}
				if val != 0.05 {
					t.Errorf("error_rate_max = %v, want 0.05", val)
				}
			},
		},
		{
			name: "injects nested data",
			step: workflows.ExecutionStep{
				ID:   "set-nested",
				Type: workflows.StepTypeContextOp,
				Raw: workflows.StateSpec{
					Type: "inject",
					Data: map[string]interface{}{
						"database": map[string]interface{}{
							"host": "localhost",
							"port": 5432,
						},
						"cache": map[string]interface{}{
							"ttl": 300,
						},
					},
					ResultPath: "services",
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusCompleted,
			checkValueFunc: func(t *testing.T, ctx map[string]interface{}) {
				host, ok := GetNestedValue(ctx, "services.database.host")
				if !ok || host != "localhost" {
					t.Errorf("services.database.host = %v, want 'localhost'", host)
				}
				port, ok := GetNestedValue(ctx, "services.database.port")
				if !ok || port != 5432 {
					t.Errorf("services.database.port = %v, want 5432", port)
				}
			},
		},
		{
			name: "fails when no data provided",
			step: workflows.ExecutionStep{
				ID:   "empty-inject",
				Type: workflows.StepTypeContextOp,
				Raw: workflows.StateSpec{
					Type:       "inject",
					Data:       nil,
					ResultPath: "config",
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusFailed,
			wantErr:    ErrNoDataToInject,
		},
		{
			name: "fails when data is empty map",
			step: workflows.ExecutionStep{
				ID:   "empty-data",
				Type: workflows.StepTypeContextOp,
				Raw: workflows.StateSpec{
					Type:       "inject",
					Data:       map[string]interface{}{},
					ResultPath: "config",
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusFailed,
			wantErr:    ErrNoDataToInject,
		},
		{
			name: "skips non-inject type",
			step: workflows.ExecutionStep{
				ID:   "not-inject",
				Type: workflows.StepTypeContextOp,
				Next: "next",
				Raw: workflows.StateSpec{
					Type: "operation",
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusCompleted,
		},
		{
			name: "supports 'set' type alias",
			step: workflows.ExecutionStep{
				ID:   "set-vars",
				Type: workflows.StepTypeContextOp,
				Raw: workflows.StateSpec{
					Type: "set",
					Data: map[string]interface{}{
						"var1": "value1",
					},
					ResultPath: "vars",
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusCompleted,
			checkValueFunc: func(t *testing.T, ctx map[string]interface{}) {
				val, ok := GetNestedValue(ctx, "vars.var1")
				if !ok || val != "value1" {
					t.Errorf("vars.var1 = %v, want 'value1'", val)
				}
			},
		},
		{
			name: "supports 'transform' type alias",
			step: workflows.ExecutionStep{
				ID:   "transform-data",
				Type: workflows.StepTypeContextOp,
				Raw: workflows.StateSpec{
					Type: "transform",
					Data: map[string]interface{}{
						"transformed": true,
					},
					ResultPath: "output",
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusCompleted,
			checkValueFunc: func(t *testing.T, ctx map[string]interface{}) {
				val, ok := GetNestedValue(ctx, "output.transformed")
				if !ok || val != true {
					t.Errorf("output.transformed = %v, want true", val)
				}
			},
		},
		{
			name: "preserves existing context data",
			step: workflows.ExecutionStep{
				ID:   "add-more",
				Type: workflows.StepTypeContextOp,
				Raw: workflows.StateSpec{
					Type: "inject",
					Data: map[string]interface{}{
						"new_key": "new_value",
					},
					ResultPath: "added",
				},
			},
			runContext: map[string]interface{}{
				"existing": "data",
				"count":    42,
			},
			wantStatus: StepStatusCompleted,
			checkValueFunc: func(t *testing.T, ctx map[string]interface{}) {
				// Check new data was added
				newVal, ok := GetNestedValue(ctx, "added.new_key")
				if !ok || newVal != "new_value" {
					t.Errorf("added.new_key = %v, want 'new_value'", newVal)
				}
				// Check existing data preserved
				if ctx["existing"] != "data" {
					t.Errorf("existing = %v, want 'data'", ctx["existing"])
				}
				if ctx["count"] != 42 {
					t.Errorf("count = %v, want 42", ctx["count"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Deep copy runContext to avoid mutation between tests
			ctx := deepCopyMap(tt.runContext)
			if ctx == nil {
				ctx = make(map[string]interface{})
			}

			result, err := executor.Execute(context.Background(), tt.step, ctx)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if err != tt.wantErr {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", result.Status, tt.wantStatus)
			}

			if tt.checkValueFunc != nil && err == nil {
				tt.checkValueFunc(t, ctx)
			}
		})
	}
}

func TestInjectExecutor_DeepCopy(t *testing.T) {
	executor := NewInjectExecutor()

	// Create step with nested data
	originalData := map[string]interface{}{
		"nested": map[string]interface{}{
			"value": "original",
		},
		"list": []interface{}{"a", "b"},
	}

	step := workflows.ExecutionStep{
		ID:   "inject-test",
		Type: workflows.StepTypeContextOp,
		Raw: workflows.StateSpec{
			Type:       "inject",
			Data:       originalData,
			ResultPath: "injected",
		},
	}

	ctx := make(map[string]interface{})
	_, err := executor.Execute(context.Background(), step, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Modify the original data
	originalData["nested"].(map[string]interface{})["value"] = "modified"
	originalData["list"].([]interface{})[0] = "modified"

	// Check that injected data is unchanged (deep copy worked)
	injected, ok := GetNestedValue(ctx, "injected.nested.value")
	if !ok {
		t.Fatal("injected.nested.value not found")
	}
	if injected != "original" {
		t.Errorf("injected.nested.value = %v, want 'original' (deep copy failed)", injected)
	}
}

// ============================================================================
// Registry Integration Tests
// ============================================================================

func TestExecutorRegistry_WithSwitchAndInject(t *testing.T) {
	registry := NewExecutorRegistry()
	registry.Register(NewSwitchExecutor())
	registry.Register(NewInjectExecutor())

	t.Run("get switch executor", func(t *testing.T) {
		executor, err := registry.GetExecutor(workflows.StepTypeBranch)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if executor == nil {
			t.Error("expected non-nil executor")
		}
	})

	t.Run("get inject executor", func(t *testing.T) {
		executor, err := registry.GetExecutor(workflows.StepTypeContextOp)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if executor == nil {
			t.Error("expected non-nil executor")
		}
	})

	t.Run("execute switch via registry", func(t *testing.T) {
		step := workflows.ExecutionStep{
			ID:   "branch",
			Type: workflows.StepTypeBranch,
			Raw: workflows.StateSpec{
				Type: "switch",
				Conditions: []workflows.SwitchCondition{
					{If: "x > 5", Next: "high"},
				},
				DefaultNext: "low",
			},
		}
		ctx := map[string]interface{}{"x": 10}

		result, err := registry.Execute(context.Background(), step, ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.NextStep != "high" {
			t.Errorf("NextStep = %v, want 'high'", result.NextStep)
		}
	})

	t.Run("execute inject via registry", func(t *testing.T) {
		step := workflows.ExecutionStep{
			ID:   "inject",
			Type: workflows.StepTypeContextOp,
			Raw: workflows.StateSpec{
				Type: "inject",
				Data: map[string]interface{}{"key": "value"},
			},
		}
		ctx := make(map[string]interface{})

		result, err := registry.Execute(context.Background(), step, ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Status != StepStatusCompleted {
			t.Errorf("Status = %v, want %v", result.Status, StepStatusCompleted)
		}
	})
}

// ============================================================================
// Helper Functions
// ============================================================================

func containsError(err, target error) bool {
	if err == nil || target == nil {
		return false
	}
	return err.Error() == target.Error() || findSubstring(err.Error(), target.Error())
}
