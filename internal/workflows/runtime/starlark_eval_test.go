package runtime

import (
	"strings"
	"testing"
)

func TestStarlarkEvaluator_EvaluateCondition_EnhancedError(t *testing.T) {
	evaluator := NewStarlarkEvaluator()

	data := map[string]interface{}{
		"ticket_id":     "T123",
		"customer_name": "John",
		"priority":      "high",
	}

	_, err := evaluator.EvaluateCondition("undefined_variable == True", data)
	if err == nil {
		t.Fatal("Expected error for undefined variable")
	}

	errStr := err.Error()

	if !strings.Contains(errStr, "undefined") {
		t.Errorf("Expected error to contain 'undefined', got: %s", errStr)
	}

	if !strings.Contains(errStr, "Available variables:") {
		t.Errorf("Expected error to contain 'Available variables:', got: %s", errStr)
	}

	if !strings.Contains(errStr, "ticket_id") {
		t.Errorf("Expected error to list 'ticket_id', got: %s", errStr)
	}

	if !strings.Contains(errStr, "Hints:") {
		t.Errorf("Expected error to contain 'Hints:', got: %s", errStr)
	}
}

func TestStarlarkEvaluator_EvaluateExpression_EnhancedError(t *testing.T) {
	evaluator := NewStarlarkEvaluator()

	data := map[string]interface{}{
		"items": []interface{}{"a", "b", "c"},
		"count": 5,
	}

	_, err := evaluator.EvaluateExpression("nonexistent_var + 1", data)
	if err == nil {
		t.Fatal("Expected error for undefined variable")
	}

	errStr := err.Error()

	if !strings.Contains(errStr, "Available variables:") {
		t.Errorf("Expected error to contain 'Available variables:', got: %s", errStr)
	}

	if !strings.Contains(errStr, "count") {
		t.Errorf("Expected error to list 'count', got: %s", errStr)
	}

	if !strings.Contains(errStr, "items") {
		t.Errorf("Expected error to list 'items', got: %s", errStr)
	}
}

func TestStarlarkEvaluator_NonUndefinedError_NotEnhanced(t *testing.T) {
	evaluator := NewStarlarkEvaluator()

	data := map[string]interface{}{
		"value": 10,
	}

	_, err := evaluator.EvaluateExpression("1 / 0", data)
	if err == nil {
		t.Fatal("Expected error for division by zero")
	}

	errStr := err.Error()

	if strings.Contains(errStr, "Available variables:") {
		t.Errorf("Expected NO 'Available variables:' for non-undefined errors, got: %s", errStr)
	}

	if strings.Contains(errStr, "Hints:") {
		t.Errorf("Expected NO 'Hints:' for non-undefined errors, got: %s", errStr)
	}
}

func TestStarlarkEvaluator_EnhancedError_ExcludesBuiltins(t *testing.T) {
	evaluator := NewStarlarkEvaluator()

	data := map[string]interface{}{
		"my_data": "value",
	}

	_, err := evaluator.EvaluateCondition("unknown_var == True", data)
	if err == nil {
		t.Fatal("Expected error for undefined variable")
	}

	errStr := err.Error()

	if !strings.Contains(errStr, "my_data") {
		t.Errorf("Expected 'my_data' in available variables, got: %s", errStr)
	}

	availableSection := extractAvailableVariables(errStr)
	if strings.Contains(availableSection, "hasattr") {
		t.Errorf("Expected 'hasattr' builtin to be excluded from available variables, got: %s", availableSection)
	}
	if strings.Contains(availableSection, "getattr") {
		t.Errorf("Expected 'getattr' builtin to be excluded from available variables, got: %s", availableSection)
	}
}

func extractAvailableVariables(errStr string) string {
	idx := strings.Index(errStr, "Available variables:")
	if idx == -1 {
		return ""
	}
	remaining := errStr[idx:]
	newlineIdx := strings.Index(remaining, "\n")
	if newlineIdx == -1 {
		return remaining
	}
	return remaining[:newlineIdx]
}
