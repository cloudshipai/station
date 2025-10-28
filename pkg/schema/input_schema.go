package schema

import (
	"encoding/json"
	"fmt"
)

// InputSchemaType represents the type of an input variable
type InputSchemaType string

const (
	TypeString  InputSchemaType = "string"
	TypeNumber  InputSchemaType = "number"
	TypeBoolean InputSchemaType = "boolean"
	TypeArray   InputSchemaType = "array"
	TypeObject  InputSchemaType = "object"
)

// InputVariable defines a single input variable in the schema
type InputVariable struct {
	Type        InputSchemaType `json:"type"`
	Description string          `json:"description,omitempty"`
	Default     interface{}     `json:"default,omitempty"`
	Enum        []interface{}   `json:"enum,omitempty"`
	Required    bool            `json:"required,omitempty"`
}

// InputSchema represents the complete input schema for an agent
type InputSchema map[string]*InputVariable

// DefaultInputSchema returns the default schema with userInput
func DefaultInputSchema() InputSchema {
	return InputSchema{
		"userInput": {
			Type:        TypeString,
			Description: "The main user input/task for the agent",
			Required:    true,
		},
	}
}

// ParseInputSchema parses a JSON string into an InputSchema
func ParseInputSchema(schemaJSON string) (InputSchema, error) {
	if schemaJSON == "" {
		return DefaultInputSchema(), nil
	}

	var schema InputSchema
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return nil, fmt.Errorf("invalid input schema JSON: %w", err)
	}

	return schema, nil
}

// MergeWithDefault merges a custom schema with the default userInput schema
func (s InputSchema) MergeWithDefault() InputSchema {
	merged := DefaultInputSchema()

	// Add custom schema variables
	for key, variable := range s {
		// Don't allow overriding userInput
		if key != "userInput" {
			merged[key] = variable
		}
	}

	return merged
}

// ToJSON converts the schema to a JSON string
func (s InputSchema) ToJSON() (string, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ValidateInputData validates input data against the schema
func (s InputSchema) ValidateInputData(inputData map[string]interface{}) error {
	// Check required fields
	for key, variable := range s {
		if variable.Required {
			if _, exists := inputData[key]; !exists {
				return fmt.Errorf("required field '%s' is missing", key)
			}
		}
	}

	// Validate data types (basic validation)
	for key, value := range inputData {
		if variable, exists := s[key]; exists {
			if err := validateType(key, value, variable.Type); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateType performs basic type validation
func validateType(key string, value interface{}, expectedType InputSchemaType) error {
	switch expectedType {
	case TypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field '%s' must be a string", key)
		}
	case TypeNumber:
		switch value.(type) {
		case float64, int, int64:
			// Valid number types
		default:
			return fmt.Errorf("field '%s' must be a number", key)
		}
	case TypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field '%s' must be a boolean", key)
		}
	case TypeArray:
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("field '%s' must be an array", key)
		}
	case TypeObject:
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("field '%s' must be an object", key)
		}
	}
	return nil
}

// ToDotpromptInputSchema converts to dotprompt input schema format
func (s InputSchema) ToDotpromptInputSchema() map[string]string {
	dotpromptSchema := make(map[string]string)

	for key, variable := range s {
		switch variable.Type {
		case TypeString:
			dotpromptSchema[key] = "string"
		case TypeNumber:
			dotpromptSchema[key] = "number"
		case TypeBoolean:
			dotpromptSchema[key] = "boolean"
		case TypeArray:
			dotpromptSchema[key] = "array"
		case TypeObject:
			dotpromptSchema[key] = "object"
		default:
			dotpromptSchema[key] = "string" // fallback
		}
	}

	return dotpromptSchema
}
