package workflows

import (
	"testing"
)

func TestSchemaChecker_CheckCompatibility(t *testing.T) {
	checker := NewSchemaChecker()

	tests := []struct {
		name           string
		outputSchema   string
		inputSchema    string
		wantCompatible bool
		wantIssues     int
		wantWarnings   int
	}{
		{
			name:           "exact match",
			outputSchema:   `{"type":"object","properties":{"pods":{"type":"array"}},"required":["pods"]}`,
			inputSchema:    `{"type":"object","properties":{"pods":{"type":"array"}},"required":["pods"]}`,
			wantCompatible: true,
			wantIssues:     0,
			wantWarnings:   0,
		},
		{
			name:           "output superset - compatible",
			outputSchema:   `{"type":"object","properties":{"pods":{"type":"array"},"timestamp":{"type":"string"}},"required":["pods"]}`,
			inputSchema:    `{"type":"object","properties":{"pods":{"type":"array"}},"required":["pods"]}`,
			wantCompatible: true,
			wantIssues:     0,
			wantWarnings:   0,
		},
		{
			name:           "missing required field - incompatible",
			outputSchema:   `{"type":"object","properties":{"pods":{"type":"array"}},"required":["pods"]}`,
			inputSchema:    `{"type":"object","properties":{"pods":{"type":"array"},"filters":{"type":"array"}},"required":["pods","filters"]}`,
			wantCompatible: false,
			wantIssues:     1,
			wantWarnings:   0,
		},
		{
			name:           "missing optional field - warning",
			outputSchema:   `{"type":"object","properties":{"pods":{"type":"array"}},"required":["pods"]}`,
			inputSchema:    `{"type":"object","properties":{"pods":{"type":"array"},"filters":{"type":"array"}},"required":["pods"]}`,
			wantCompatible: true,
			wantIssues:     0,
			wantWarnings:   1,
		},
		{
			name:           "type mismatch - incompatible",
			outputSchema:   `{"type":"object","properties":{"count":{"type":"string"}},"required":["count"]}`,
			inputSchema:    `{"type":"object","properties":{"count":{"type":"integer"}},"required":["count"]}`,
			wantCompatible: false,
			wantIssues:     1,
			wantWarnings:   0,
		},
		{
			name:           "integer to number - compatible",
			outputSchema:   `{"type":"object","properties":{"count":{"type":"integer"}},"required":["count"]}`,
			inputSchema:    `{"type":"object","properties":{"count":{"type":"number"}},"required":["count"]}`,
			wantCompatible: true,
			wantIssues:     0,
			wantWarnings:   0,
		},
		{
			name:           "no schemas - compatible",
			outputSchema:   "",
			inputSchema:    "",
			wantCompatible: true,
			wantIssues:     0,
			wantWarnings:   0,
		},
		{
			name:           "only output schema - compatible",
			outputSchema:   `{"type":"object","properties":{"pods":{"type":"array"}}}`,
			inputSchema:    "",
			wantCompatible: true,
			wantIssues:     0,
			wantWarnings:   0,
		},
		{
			name:           "only input schema - compatible",
			outputSchema:   "",
			inputSchema:    `{"type":"object","properties":{"pods":{"type":"array"}}}`,
			wantCompatible: true,
			wantIssues:     0,
			wantWarnings:   0,
		},
		{
			name:           "array item type match",
			outputSchema:   `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object"}}},"required":["items"]}`,
			inputSchema:    `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object"}}},"required":["items"]}`,
			wantCompatible: true,
			wantIssues:     0,
			wantWarnings:   0,
		},
		{
			name:           "array item type mismatch",
			outputSchema:   `{"type":"object","properties":{"items":{"type":"array","items":{"type":"string"}}},"required":["items"]}`,
			inputSchema:    `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object"}}},"required":["items"]}`,
			wantCompatible: false,
			wantIssues:     1,
			wantWarnings:   0,
		},
		{
			name:           "invalid output schema JSON - warning",
			outputSchema:   `{invalid json`,
			inputSchema:    `{"type":"object"}`,
			wantCompatible: true,
			wantIssues:     0,
			wantWarnings:   1,
		},
		{
			name:           "invalid input schema JSON - warning",
			outputSchema:   `{"type":"object"}`,
			inputSchema:    `{invalid json`,
			wantCompatible: true,
			wantIssues:     0,
			wantWarnings:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.CheckCompatibility(tt.outputSchema, tt.inputSchema)

			if result.Compatible != tt.wantCompatible {
				t.Errorf("Compatible = %v, want %v", result.Compatible, tt.wantCompatible)
			}

			if len(result.Issues) != tt.wantIssues {
				t.Errorf("Issues count = %d, want %d. Issues: %v", len(result.Issues), tt.wantIssues, result.Issues)
			}

			if len(result.Warnings) != tt.wantWarnings {
				t.Errorf("Warnings count = %d, want %d. Warnings: %v", len(result.Warnings), tt.wantWarnings, result.Warnings)
			}
		})
	}
}

func TestValidateInputAgainstSchema(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]interface{}
		schemaJSON string
		wantErr    bool
	}{
		{
			name:       "valid input with all required fields",
			input:      map[string]interface{}{"namespace": "production", "service": "api"},
			schemaJSON: `{"type":"object","properties":{"namespace":{"type":"string"},"service":{"type":"string"}},"required":["namespace","service"]}`,
			wantErr:    false,
		},
		{
			name:       "missing required field",
			input:      map[string]interface{}{"namespace": "production"},
			schemaJSON: `{"type":"object","properties":{"namespace":{"type":"string"},"service":{"type":"string"}},"required":["namespace","service"]}`,
			wantErr:    true,
		},
		{
			name:       "wrong type for field",
			input:      map[string]interface{}{"count": "not-a-number"},
			schemaJSON: `{"type":"object","properties":{"count":{"type":"integer"}},"required":["count"]}`,
			wantErr:    true,
		},
		{
			name:       "valid integer field",
			input:      map[string]interface{}{"count": float64(42)},
			schemaJSON: `{"type":"object","properties":{"count":{"type":"integer"}},"required":["count"]}`,
			wantErr:    false,
		},
		{
			name:       "empty schema - always valid",
			input:      map[string]interface{}{"anything": "goes"},
			schemaJSON: "",
			wantErr:    false,
		},
		{
			name:       "extra fields allowed",
			input:      map[string]interface{}{"namespace": "production", "extra": "field"},
			schemaJSON: `{"type":"object","properties":{"namespace":{"type":"string"}},"required":["namespace"]}`,
			wantErr:    false,
		},
		{
			name:       "valid boolean field",
			input:      map[string]interface{}{"enabled": true},
			schemaJSON: `{"type":"object","properties":{"enabled":{"type":"boolean"}},"required":["enabled"]}`,
			wantErr:    false,
		},
		{
			name:       "invalid boolean field",
			input:      map[string]interface{}{"enabled": "yes"},
			schemaJSON: `{"type":"object","properties":{"enabled":{"type":"boolean"}},"required":["enabled"]}`,
			wantErr:    true,
		},
		{
			name:       "valid array field",
			input:      map[string]interface{}{"items": []interface{}{"a", "b"}},
			schemaJSON: `{"type":"object","properties":{"items":{"type":"array"}},"required":["items"]}`,
			wantErr:    false,
		},
		{
			name:       "valid object field",
			input:      map[string]interface{}{"config": map[string]interface{}{"key": "value"}},
			schemaJSON: `{"type":"object","properties":{"config":{"type":"object"}},"required":["config"]}`,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInputAgainstSchema(tt.input, tt.schemaJSON)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInputAgainstSchema() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
