package schema

import (
	"encoding/json"
	"fmt"
	"strings"
	
	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v2"
)

// DefaultJSONSchema returns the default JSON Schema with userInput
func DefaultJSONSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"userInput": map[string]interface{}{
				"type":        "string",
				"description": "The main user input/task for the agent",
			},
		},
		"required": []string{"userInput"},
	}
}

// ValidateJSONSchema validates a JSON Schema string
func ValidateJSONSchema(schemaJSON string) error {
	if schemaJSON == "" {
		return nil // Empty schema is valid
	}
	
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(schemaJSON)); err != nil {
		return fmt.Errorf("invalid JSON schema: %w", err)
	}
	
	_, err := compiler.Compile("schema.json")
	return err
}

// GenerateDotpromptSchema converts JSON Schema to dotprompt YAML format
func GenerateDotpromptSchema(jsonSchemaStr string) (string, error) {
	if jsonSchemaStr == "" {
		return "    userInput: string\n", nil
	}
	
	// Parse JSON Schema
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(jsonSchemaStr), &schema); err != nil {
		return "", fmt.Errorf("invalid JSON schema: %w", err)
	}
	
	// Convert to YAML and indent for dotprompt
	yamlBytes, err := yaml.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("failed to convert schema to YAML: %w", err)
	}
	
	// Indent each line by 4 spaces
	lines := strings.Split(strings.TrimSpace(string(yamlBytes)), "\n")
	var result strings.Builder
	for _, line := range lines {
		result.WriteString("    ")
		result.WriteString(line)
		result.WriteString("\n")
	}
	
	return result.String(), nil
}

// MergeUserInputWithSchema ensures userInput is always present in schema
func MergeUserInputWithSchema(jsonSchemaStr string) (string, error) {
	if jsonSchemaStr == "" {
		defaultSchema := DefaultJSONSchema()
		jsonBytes, err := json.Marshal(defaultSchema)
		return string(jsonBytes), err
	}
	
	// Parse existing schema
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(jsonSchemaStr), &schema); err != nil {
		return "", fmt.Errorf("invalid JSON schema: %w", err)
	}
	
	// Ensure schema is an object type
	if schema["type"] != "object" {
		schema["type"] = "object"
	}
	
	// Ensure properties exists
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		properties = make(map[string]interface{})
		schema["properties"] = properties
	}
	
	// Always ensure userInput is present
	if _, exists := properties["userInput"]; !exists {
		properties["userInput"] = map[string]interface{}{
			"type":        "string",
			"description": "The main user input/task for the agent",
		}
	}
	
	// Ensure required array includes userInput
	required, ok := schema["required"].([]interface{})
	if !ok {
		required = []interface{}{}
	}
	
	userInputRequired := false
	for _, req := range required {
		if req == "userInput" {
			userInputRequired = true
			break
		}
	}
	
	if !userInputRequired {
		required = append(required, "userInput")
		schema["required"] = required
	}
	
	// Return updated schema as JSON string
	jsonBytes, err := json.Marshal(schema)
	return string(jsonBytes), err
}

// ExtractVariablesFromSchema extracts variable names from a JSON Schema
func ExtractVariablesFromSchema(jsonSchemaStr string) ([]string, error) {
	if jsonSchemaStr == "" {
		return []string{"userInput"}, nil
	}
	
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(jsonSchemaStr), &schema); err != nil {
		return nil, fmt.Errorf("invalid JSON schema: %w", err)
	}
	
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return []string{"userInput"}, nil
	}
	
	variables := make([]string, 0, len(properties))
	for key := range properties {
		variables = append(variables, key)
	}
	
	return variables, nil
}

// ValidateInputData validates input data against a JSON Schema
func ValidateInputData(jsonSchemaStr string, inputData map[string]interface{}) error {
	if jsonSchemaStr == "" {
		// For empty schema, just ensure userInput exists
		if _, exists := inputData["userInput"]; !exists {
			return fmt.Errorf("required field 'userInput' is missing")
		}
		return nil
	}
	
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(jsonSchemaStr)); err != nil {
		return fmt.Errorf("invalid JSON schema: %w", err)
	}
	
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("failed to compile schema: %w", err)
	}
	
	if err := schema.Validate(inputData); err != nil {
		return fmt.Errorf("input validation failed: %w", err)
	}
	
	return nil
}

// Temporary types for backwards compatibility during migration
// TODO: Remove these once all references are updated
type InputSchemaType string
type InputVariable struct {
	Type        InputSchemaType `json:"type"`
	Description string          `json:"description,omitempty"`
	Default     interface{}     `json:"default,omitempty"`
	Enum        []interface{}   `json:"enum,omitempty"`
	Required    bool            `json:"required,omitempty"`
}