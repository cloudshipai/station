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
	
	// Handle schema conversion
	if agent.InputSchema != nil && *agent.InputSchema != "" {
		// First ensure userInput is merged into the schema
		mergedSchema, err := MergeUserInputWithSchema(*agent.InputSchema)
		if err != nil {
			return "", fmt.Errorf("failed to merge schema for agent %s (ID: %d): %w", agent.Name, agent.ID, err)
		}
		
		// Convert JSON Schema to dotprompt YAML format
		schemaYAML, err := GenerateDotpromptSchema(mergedSchema)
		if err != nil {
			return "", fmt.Errorf("failed to generate dotprompt schema for agent %s (ID: %d): %w", agent.Name, agent.ID, err)
		}
		
		content.WriteString(schemaYAML)
	} else {
		// Default schema with just userInput
		content.WriteString("    userInput: string\n")
	}
	
	return content.String(), nil
}

// GetMergedInputData merges userInput with custom input variables for execution
func (h *ExportHelper) GetMergedInputData(agent *models.Agent, userInput string, customData map[string]interface{}) (map[string]interface{}, error) {
	// Start with mandatory userInput
	result := map[string]interface{}{
		"userInput": userInput,
	}
	
	// Merge custom data if provided
	if customData != nil {
		for key, value := range customData {
			// Don't allow overriding userInput
			if key != "userInput" {
				result[key] = value
			}
		}
	}
	
	// If agent has custom schema, validate the merged data
	if agent.InputSchema != nil && *agent.InputSchema != "" {
		// Ensure userInput is merged into schema for validation
		mergedSchema, err := MergeUserInputWithSchema(*agent.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to merge agent schema: %w", err)
		}
		
		// Validate the complete input data against the schema
		if err := ValidateInputData(mergedSchema, result); err != nil {
			return nil, fmt.Errorf("input validation failed: %w", err)
		}
	}
	
	return result, nil
}

// ValidateInputSchema validates that a JSON Schema is valid
func (h *ExportHelper) ValidateInputSchema(schemaJSON string) error {
	return ValidateJSONSchema(schemaJSON)
}