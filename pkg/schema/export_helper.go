package schema

import (
	"fmt"
	"strings"
	"station/pkg/models"
)

// ExportHelper handles dotprompt export with input schema merging
type ExportHelper struct{}

// NewExportHelper creates a new export helper
func NewExportHelper() *ExportHelper {
	return &ExportHelper{}
}

// GenerateInputSchemaSection generates the input schema section for dotprompt export
func (h *ExportHelper) GenerateInputSchemaSection(agent *models.Agent) (string, error) {
	var content strings.Builder
	
	// Start input schema section
	content.WriteString("input:\n")
	content.WriteString("  schema:\n")
	
	// Always include the mandatory userInput
	content.WriteString("    userInput: string\n")
	
	// Add custom schema if defined
	if agent.InputSchema != nil && *agent.InputSchema != "" {
		customSchema, err := ParseInputSchema(*agent.InputSchema)
		if err != nil {
			return "", fmt.Errorf("invalid input schema in agent: %w", err)
		}
		
		// Convert to dotprompt format and add custom variables
		for key, variable := range customSchema {
			// Skip userInput as it's already added
			if key != "userInput" {
				dotpromptType := h.convertTypeToDotprompt(variable.Type)
				content.WriteString(fmt.Sprintf("    %s: %s\n", key, dotpromptType))
			}
		}
	}
	
	return content.String(), nil
}

// GetMergedInputData merges userInput with custom input variables for execution
func (h *ExportHelper) GetMergedInputData(agent *models.Agent, userInput string, customData map[string]interface{}) (map[string]interface{}, error) {
	// Start with mandatory userInput
	result := map[string]interface{}{
		"userInput": userInput,
	}
	
	// If agent has custom schema, validate and merge
	if agent.InputSchema != nil && *agent.InputSchema != "" {
		customSchema, err := ParseInputSchema(*agent.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("invalid agent input schema: %w", err)
		}
		
		// Validate custom data against schema
		if customData != nil {
			if err := customSchema.ValidateInputData(customData); err != nil {
				return nil, fmt.Errorf("input validation failed: %w", err)
			}
			
			// Merge custom data (excluding userInput)
			for key, value := range customData {
				if key != "userInput" {
					result[key] = value
				}
			}
		}
		
		// Add default values for missing non-required fields
		for key, variable := range customSchema {
			if key != "userInput" && result[key] == nil && variable.Default != nil {
				result[key] = variable.Default
			}
		}
	}
	
	return result, nil
}

// convertTypeToDotprompt converts schema type to dotprompt type string
func (h *ExportHelper) convertTypeToDotprompt(schemaType InputSchemaType) string {
	switch schemaType {
	case TypeString:
		return "string"
	case TypeNumber:
		return "number"
	case TypeBoolean:
		return "boolean"
	case TypeArray:
		return "array"
	case TypeObject:
		return "object"
	default:
		return "string" // fallback
	}
}

// ValidateInputSchema validates that an input schema JSON is valid
func (h *ExportHelper) ValidateInputSchema(schemaJSON string) error {
	if schemaJSON == "" {
		return nil // Empty schema is valid
	}
	
	_, err := ParseInputSchema(schemaJSON)
	return err
}