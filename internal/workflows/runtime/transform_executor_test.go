package runtime

import (
	"errors"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

func TestEnhanceStarlarkError_UndefinedVariable(t *testing.T) {
	predeclared := starlark.StringDict{
		"json":            starlark.None,
		"sum":             starlark.None,
		"hasattr":         starlark.None,
		"getattr":         starlark.None,
		"input":           starlark.None,
		"ctx":             starlark.None,
		"ticket_id":       starlark.String("T123"),
		"customer_name":   starlark.String("John"),
		"classify_ticket": starlark.None,
	}

	originalErr := errors.New("transform.star:5:43: undefined: urgency_data")

	enhanced := enhanceStarlarkError(originalErr, predeclared)
	errStr := enhanced.Error()

	if !strings.Contains(errStr, "undefined: urgency_data") {
		t.Errorf("Expected error to contain original undefined message, got: %s", errStr)
	}

	if !strings.Contains(errStr, "Available variables:") {
		t.Errorf("Expected error to contain 'Available variables:', got: %s", errStr)
	}

	if !strings.Contains(errStr, "ticket_id") {
		t.Errorf("Expected error to list 'ticket_id' as available variable, got: %s", errStr)
	}

	if !strings.Contains(errStr, "customer_name") {
		t.Errorf("Expected error to list 'customer_name' as available variable, got: %s", errStr)
	}

	if !strings.Contains(errStr, "classify_ticket") {
		t.Errorf("Expected error to list 'classify_ticket' as available variable, got: %s", errStr)
	}

	if strings.Contains(errStr, "Available variables:") && strings.Contains(errStr, "json") {
		t.Errorf("Expected error to NOT list builtin 'json' as available variable, got: %s", errStr)
	}

	if !strings.Contains(errStr, "Hints:") {
		t.Errorf("Expected error to contain 'Hints:', got: %s", errStr)
	}

	if !strings.Contains(errStr, "inputs are flattened") {
		t.Errorf("Expected error to contain hint about flattened inputs, got: %s", errStr)
	}
}

func TestEnhanceStarlarkError_NonUndefinedError(t *testing.T) {
	predeclared := starlark.StringDict{
		"ticket_id": starlark.String("T123"),
	}

	originalErr := errors.New("syntax error: unexpected token")

	enhanced := enhanceStarlarkError(originalErr, predeclared)
	errStr := enhanced.Error()

	if !strings.Contains(errStr, "starlark execution failed") {
		t.Errorf("Expected wrapped error, got: %s", errStr)
	}

	if strings.Contains(errStr, "Available variables:") {
		t.Errorf("Expected NO available variables hint for non-undefined errors, got: %s", errStr)
	}

	if strings.Contains(errStr, "Hints:") {
		t.Errorf("Expected NO hints for non-undefined errors, got: %s", errStr)
	}
}

func TestEnhanceStarlarkError_EmptyContext(t *testing.T) {
	predeclared := starlark.StringDict{
		"json":    starlark.None,
		"sum":     starlark.None,
		"hasattr": starlark.None,
		"getattr": starlark.None,
		"input":   starlark.None,
		"ctx":     starlark.None,
	}

	originalErr := errors.New("transform.star:1:1: undefined: some_var")

	enhanced := enhanceStarlarkError(originalErr, predeclared)
	errStr := enhanced.Error()

	if !strings.Contains(errStr, "Hints:") {
		t.Errorf("Expected hints even with empty context, got: %s", errStr)
	}
}

func TestSortStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "already sorted",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "reverse order",
			input:    []string{"c", "b", "a"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "mixed",
			input:    []string{"ticket_id", "customer_name", "classify_ticket"},
			expected: []string{"classify_ticket", "customer_name", "ticket_id"},
		},
		{
			name:     "empty",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "single element",
			input:    []string{"only"},
			expected: []string{"only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortStrings(tt.input)
			if len(tt.input) != len(tt.expected) {
				t.Errorf("Length mismatch: got %d, want %d", len(tt.input), len(tt.expected))
				return
			}
			for i := range tt.input {
				if tt.input[i] != tt.expected[i] {
					t.Errorf("At index %d: got %s, want %s", i, tt.input[i], tt.expected[i])
				}
			}
		})
	}
}
