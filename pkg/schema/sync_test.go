package schema

import (
	"encoding/json"
	"testing"
)

func TestInputSchemaExtraction(t *testing.T) {
	// Test case: dotprompt with custom input schema
	frontmatterInput := map[string]interface{}{
		"schema": map[interface{}]interface{}{
			"userInput": "string",
			"projectPath": map[interface{}]interface{}{
				"type":        "string",
				"description": "Path to the project directory",
				"required":    true,
			},
			"environment": map[interface{}]interface{}{
				"type":        "string",
				"enum":        []interface{}{"dev", "staging", "prod"},
				"description": "Target deployment environment",
				"default":     "dev",
			},
			"enableDebug": map[interface{}]interface{}{
				"type":        "boolean",
				"description": "Enable debug mode",
				"default":     false,
			},
		},
	}

	// Simulate the extraction process (from agent_sync.go)
	schemaData, exists := frontmatterInput["schema"]
	if !exists {
		t.Fatal("Expected schema to exist in input")
	}

	schemaMap, ok := schemaData.(map[interface{}]interface{})
	if !ok {
		t.Fatal("Expected schema to be a map")
	}

	// Convert to our format (excluding userInput)
	customSchema := make(map[string]*InputVariable)

	for key, value := range schemaMap {
		keyStr, ok := key.(string)
		if !ok {
			continue
		}

		// Skip userInput as it's automatically provided
		if keyStr == "userInput" {
			continue
		}

		var variable *InputVariable

		switch v := value.(type) {
		case string:
			variable = &InputVariable{
				Type: InputSchemaType(v),
			}
		case map[interface{}]interface{}:
			variable = &InputVariable{}

			if typeVal, exists := v["type"]; exists {
				if typeStr, ok := typeVal.(string); ok {
					variable.Type = InputSchemaType(typeStr)
				}
			}
			if descVal, exists := v["description"]; exists {
				if descStr, ok := descVal.(string); ok {
					variable.Description = descStr
				}
			}
			if defaultVal, exists := v["default"]; exists {
				variable.Default = defaultVal
			}
			if enumVal, exists := v["enum"]; exists {
				if enumList, ok := enumVal.([]interface{}); ok {
					variable.Enum = enumList
				}
			}
			if reqVal, exists := v["required"]; exists {
				if reqBool, ok := reqVal.(bool); ok {
					variable.Required = reqBool
				}
			}
		}

		if variable != nil && variable.Type != "" {
			customSchema[keyStr] = variable
		}
	}

	// Verify the extracted schema
	if len(customSchema) != 3 {
		t.Errorf("Expected 3 custom variables, got %d", len(customSchema))
	}

	// Check projectPath
	if projectPath, exists := customSchema["projectPath"]; exists {
		if projectPath.Type != TypeString {
			t.Errorf("Expected projectPath type to be string, got %s", projectPath.Type)
		}
		if !projectPath.Required {
			t.Error("Expected projectPath to be required")
		}
		if projectPath.Description != "Path to the project directory" {
			t.Errorf("Unexpected projectPath description: %s", projectPath.Description)
		}
	} else {
		t.Error("Expected projectPath to exist in custom schema")
	}

	// Check environment
	if environment, exists := customSchema["environment"]; exists {
		if environment.Type != TypeString {
			t.Errorf("Expected environment type to be string, got %s", environment.Type)
		}
		if len(environment.Enum) != 3 {
			t.Errorf("Expected environment to have 3 enum values, got %d", len(environment.Enum))
		}
		if environment.Default != "dev" {
			t.Errorf("Expected environment default to be 'dev', got %v", environment.Default)
		}
	} else {
		t.Error("Expected environment to exist in custom schema")
	}

	// Check enableDebug
	if enableDebug, exists := customSchema["enableDebug"]; exists {
		if enableDebug.Type != TypeBoolean {
			t.Errorf("Expected enableDebug type to be boolean, got %s", enableDebug.Type)
		}
		if enableDebug.Default != false {
			t.Errorf("Expected enableDebug default to be false, got %v", enableDebug.Default)
		}
	} else {
		t.Error("Expected enableDebug to exist in custom schema")
	}

	// Test JSON serialization
	schemaJSON, err := json.Marshal(customSchema)
	if err != nil {
		t.Fatalf("Failed to serialize schema: %v", err)
	}

	// Test validation
	helper := NewExportHelper()
	if err := helper.ValidateInputSchema(string(schemaJSON)); err != nil {
		t.Errorf("Schema validation failed: %v", err)
	}

	t.Logf("Extracted schema JSON: %s", string(schemaJSON))
}
