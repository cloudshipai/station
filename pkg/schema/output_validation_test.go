package schema

import (
	"testing"
)

func TestValidateOutputSchema(t *testing.T) {
	helper := NewExportHelper()

	tests := []struct {
		name        string
		schema      string
		shouldError bool
		description string
	}{
		{
			name:        "empty schema",
			schema:      "",
			shouldError: false,
			description: "Empty schema should be valid",
		},
		{
			name: "valid simple schema",
			schema: `{
				"type": "object",
				"properties": {
					"summary": {"type": "string"},
					"status": {"type": "string", "enum": ["success", "error"]}
				},
				"required": ["summary", "status"]
			}`,
			shouldError: false,
			description: "Valid JSON Schema should pass",
		},
		{
			name: "complex valid schema",
			schema: `{
				"type": "object",
				"properties": {
					"cost_analysis": {
						"type": "object",
						"properties": {
							"monthly_cost": {"type": "number"},
							"currency": {"type": "string", "enum": ["USD", "EUR", "GBP"]}
						}
					},
					"recommendations": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"type": {"type": "string"},
								"priority": {"type": "string", "enum": ["high", "medium", "low"]},
								"description": {"type": "string"}
							}
						}
					}
				}
			}`,
			shouldError: false,
			description: "Complex nested schema should be valid",
		},
		{
			name:        "invalid JSON",
			schema:      `{"type": "object", "properties": {`,
			shouldError: true,
			description: "Invalid JSON should be rejected",
		},
		{
			name:        "invalid schema structure",
			schema:      `{"type": "invalid_type"}`,
			shouldError: true,
			description: "Invalid JSON Schema should be rejected",
		},
		{
			name: "malformed enum",
			schema: `{
				"type": "object",
				"properties": {
					"status": {"type": "string", "enum": "not_array"}
				}
			}`,
			shouldError: true,
			description: "Schema with malformed enum should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := helper.ValidateOutputSchema(tt.schema)
			
			if tt.shouldError && err == nil {
				t.Errorf("Expected error for %s, but got none. %s", tt.name, tt.description)
			}
			
			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected error for %s: %v. %s", tt.name, err, tt.description)
			}
		})
	}
}

func TestValidateOutputSchemaRejectsInvalidJSON(t *testing.T) {
	helper := NewExportHelper()
	
	// Test with invalid JSON syntax
	invalidJSON := `{"type": "object", "properties": {`
	
	err := helper.ValidateOutputSchema(invalidJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON syntax, but got none")
	}
}

func TestValidateOutputSchemaMatchesInputValidation(t *testing.T) {
	helper := NewExportHelper()
	
	// Same schema should be valid for both input and output validation
	validSchema := `{
		"type": "object",
		"properties": {
			"userInput": {"type": "string"},
			"customField": {"type": "string"}
		},
		"required": ["userInput"]
	}`
	
	inputErr := helper.ValidateInputSchema(validSchema)
	outputErr := helper.ValidateOutputSchema(validSchema)
	
	if inputErr != nil {
		t.Errorf("Input validation failed: %v", inputErr)
	}
	
	if outputErr != nil {
		t.Errorf("Output validation failed: %v", outputErr)
	}
	
	// Both should have same validation behavior
	if (inputErr == nil) != (outputErr == nil) {
		t.Error("Input and output validation should have consistent behavior for the same schema")
	}
}