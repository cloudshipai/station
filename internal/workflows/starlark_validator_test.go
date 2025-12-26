package workflows

import (
	"encoding/json"
	"testing"
)

func TestStarlarkValidator_ValidTransformExpressions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{"simple_dict", `{"key": "value"}`},
		{"dict_with_variable", `status = "ready"
{"status": status}`},
		{"list_comprehension", `[x * 2 for x in [1, 2, 3]]`},
		{"getattr_with_default", `getattr(ctx, "field", "default")`},
		{"hasattr_check", `hasattr(ctx, "field")`},
		{"conditional_expression", `"yes" if True else "no"`},
		{"arithmetic", `1 + 2 * 3`},
		{"string_formatting", `"Hello %s" % "World"`},
		{"dict_comprehension", `{k: v for k, v in [("a", 1), ("b", 2)]}`},
		{"explicit_result", `__result__ = {"done": True}`},
	}

	validator := NewStarlarkValidator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateTransformExpression(tt.expression)
			if err != nil {
				t.Errorf("expected no error for valid expression %q, got: %v", tt.name, err)
			}
		})
	}
}

func TestStarlarkValidator_InvalidTransformExpressions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{"unclosed_brace", `{"key": "value"`},
		{"unclosed_bracket", `[1, 2, 3`},
		{"unclosed_paren", `func(arg`},
		{"invalid_syntax", `def incomplete`},
		{"missing_colon_in_dict", `{"key" "value"}`},
	}

	validator := NewStarlarkValidator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateTransformExpression(tt.expression)
			if err == nil {
				t.Errorf("expected error for invalid expression %q, got nil", tt.name)
			}
		})
	}
}

func TestStarlarkValidator_ValidConditionExpressions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{"simple_comparison", `status == "ready"`},
		{"hasattr_check", `hasattr(ctx, "field")`},
		{"boolean_and", `a and b`},
		{"boolean_or", `a or b`},
		{"not_expression", `not done`},
		{"in_expression", `"key" in data`},
		{"greater_than", `count > 0`},
		{"less_than_or_equal", `value <= 100`},
		{"combined_conditions", `hasattr(ctx, "x") and ctx.x > 10`},
		{"true_literal", `True`},
		{"false_literal", `False`},
	}

	validator := NewStarlarkValidator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateConditionExpression(tt.expression)
			if err != nil {
				t.Errorf("expected no error for valid condition %q, got: %v", tt.name, err)
			}
		})
	}
}

func TestStarlarkValidator_InvalidConditionExpressions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{"unclosed_paren", `hasattr(ctx, "field"`},
		{"invalid_operator", `status === "ready"`},
		{"missing_operand", `status ==`},
	}

	validator := NewStarlarkValidator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateConditionExpression(tt.expression)
			if err == nil {
				t.Errorf("expected error for invalid condition %q, got nil", tt.name)
			}
		})
	}
}

func TestStarlarkValidator_WorkflowDefinition(t *testing.T) {
	t.Run("valid_transform_step", func(t *testing.T) {
		def := &Definition{
			ID: "test",
			States: []StateSpec{
				{
					ID:         "transform-step",
					Type:       "transform",
					Expression: `{"status": "done"}`,
				},
			},
		}

		validator := NewStarlarkValidator()
		issues := validator.ValidateWorkflowExpressions(def)

		if len(issues) != 0 {
			t.Errorf("expected no issues for valid transform, got %d: %+v", len(issues), issues)
		}
	})

	t.Run("invalid_transform_step", func(t *testing.T) {
		def := &Definition{
			ID: "test",
			States: []StateSpec{
				{
					ID:         "transform-step",
					Type:       "transform",
					Expression: `{"status": "done"`,
				},
			},
		}

		validator := NewStarlarkValidator()
		issues := validator.ValidateWorkflowExpressions(def)

		if len(issues) != 1 {
			t.Fatalf("expected 1 issue for invalid transform, got %d: %+v", len(issues), issues)
		}

		if issues[0].Code != "STARLARK_SYNTAX_ERROR" {
			t.Errorf("expected STARLARK_SYNTAX_ERROR, got %s", issues[0].Code)
		}
	})

	t.Run("valid_switch_conditions", func(t *testing.T) {
		def := &Definition{
			ID: "test",
			States: []StateSpec{
				{
					ID:   "switch-step",
					Type: "switch",
					Conditions: []SwitchCondition{
						{If: `status == "ready"`, Next: "ready-path"},
						{If: `status == "error"`, Next: "error-path"},
					},
				},
			},
		}

		validator := NewStarlarkValidator()
		issues := validator.ValidateWorkflowExpressions(def)

		if len(issues) != 0 {
			t.Errorf("expected no issues for valid switch, got %d: %+v", len(issues), issues)
		}
	})

	t.Run("invalid_switch_condition", func(t *testing.T) {
		def := &Definition{
			ID: "test",
			States: []StateSpec{
				{
					ID:   "switch-step",
					Type: "switch",
					Conditions: []SwitchCondition{
						{If: `status == "ready"`, Next: "ready-path"},
						{If: `hasattr(ctx, "field"`, Next: "field-path"},
					},
				},
			},
		}

		validator := NewStarlarkValidator()
		issues := validator.ValidateWorkflowExpressions(def)

		if len(issues) != 1 {
			t.Fatalf("expected 1 issue for invalid condition, got %d: %+v", len(issues), issues)
		}

		if issues[0].Code != "STARLARK_SYNTAX_ERROR" {
			t.Errorf("expected STARLARK_SYNTAX_ERROR, got %s", issues[0].Code)
		}
	})

	t.Run("nested_branch_validation", func(t *testing.T) {
		def := &Definition{
			ID: "test",
			States: []StateSpec{
				{
					ID:   "parallel-step",
					Type: "parallel",
					Branches: []BranchSpec{
						{
							States: []StateSpec{
								{
									ID:         "nested-transform",
									Type:       "transform",
									Expression: `{"invalid"`,
								},
							},
						},
					},
				},
			},
		}

		validator := NewStarlarkValidator()
		issues := validator.ValidateWorkflowExpressions(def)

		if len(issues) != 1 {
			t.Fatalf("expected 1 issue for nested invalid transform, got %d: %+v", len(issues), issues)
		}

		if issues[0].Code != "STARLARK_SYNTAX_ERROR" {
			t.Errorf("expected STARLARK_SYNTAX_ERROR, got %s", issues[0].Code)
		}
	})

	t.Run("iterator_validation", func(t *testing.T) {
		def := &Definition{
			ID: "test",
			States: []StateSpec{
				{
					ID:   "foreach-step",
					Type: "foreach",
					Iterator: &IteratorSpec{
						States: []StateSpec{
							{
								ID:         "iter-transform",
								Type:       "transform",
								Expression: `[x for x in`,
							},
						},
					},
				},
			},
		}

		validator := NewStarlarkValidator()
		issues := validator.ValidateWorkflowExpressions(def)

		if len(issues) != 1 {
			t.Fatalf("expected 1 issue for iterator invalid expression, got %d: %+v", len(issues), issues)
		}
	})
}

func TestStarlarkValidator_EmptyExpressions(t *testing.T) {
	validator := NewStarlarkValidator()

	t.Run("empty_transform", func(t *testing.T) {
		err := validator.validateTransformExpression("")
		if err != nil {
			t.Errorf("expected no error for empty transform, got: %v", err)
		}
	})

	t.Run("whitespace_transform", func(t *testing.T) {
		err := validator.validateTransformExpression("   ")
		if err != nil {
			t.Errorf("expected no error for whitespace transform, got: %v", err)
		}
	})

	t.Run("empty_condition", func(t *testing.T) {
		err := validator.validateConditionExpression("")
		if err != nil {
			t.Errorf("expected no error for empty condition, got: %v", err)
		}
	})
}

func TestStarlarkValidator_NilDefinition(t *testing.T) {
	validator := NewStarlarkValidator()
	issues := validator.ValidateWorkflowExpressions(nil)

	if len(issues) != 0 {
		t.Errorf("expected no issues for nil definition, got %d", len(issues))
	}
}

func TestStarlarkValidator_IntegrationWithValidateDefinition(t *testing.T) {
	t.Run("catches_invalid_transform", func(t *testing.T) {
		raw := json.RawMessage(`{
			"id": "test-workflow",
			"start": "transform-step",
			"states": [
				{
					"id": "transform-step",
					"type": "transform",
					"expression": "{\"unclosed\": \"dict\"",
					"input": {},
					"output": {},
					"retry": {"max_attempts": 3},
					"timeout": "5m"
				}
			]
		}`)

		_, result, err := ValidateDefinition(raw)
		if err == nil {
			t.Fatal("expected validation error for invalid Starlark expression")
		}

		foundStarlarkError := false
		for _, issue := range result.Errors {
			if issue.Code == "STARLARK_SYNTAX_ERROR" {
				foundStarlarkError = true
				break
			}
		}

		if !foundStarlarkError {
			t.Errorf("expected STARLARK_SYNTAX_ERROR in errors, got: %+v", result.Errors)
		}
	})

	t.Run("catches_invalid_switch_condition", func(t *testing.T) {
		raw := json.RawMessage(`{
			"id": "test-workflow",
			"start": "switch-step",
			"states": [
				{
					"id": "switch-step",
					"type": "switch",
					"conditions": [
						{"if": "status == \"ready\"", "next": "done"},
						{"if": "hasattr(ctx, \"field\"", "next": "field-path"}
					],
					"input": {},
					"output": {},
					"retry": {"max_attempts": 3},
					"timeout": "5m"
				},
				{
					"id": "done",
					"type": "operation",
					"input": {},
					"output": {},
					"retry": {"max_attempts": 3},
					"timeout": "5m"
				},
				{
					"id": "field-path",
					"type": "operation",
					"input": {},
					"output": {},
					"retry": {"max_attempts": 3},
					"timeout": "5m"
				}
			]
		}`)

		_, result, err := ValidateDefinition(raw)
		if err == nil {
			t.Fatal("expected validation error for invalid switch condition")
		}

		foundStarlarkError := false
		for _, issue := range result.Errors {
			if issue.Code == "STARLARK_SYNTAX_ERROR" {
				foundStarlarkError = true
				break
			}
		}

		if !foundStarlarkError {
			t.Errorf("expected STARLARK_SYNTAX_ERROR in errors, got: %+v", result.Errors)
		}
	})
}

func TestTruncateExpression(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a longer string", 10, "this is..."},
		{"multi\nline\nstring", 20, "multi line string"},
		{"  spaces   everywhere  ", 25, "spaces everywhere"},
	}

	for _, tt := range tests {
		result := truncateExpression(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateExpression(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}
