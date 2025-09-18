package schema

import (
	"encoding/json"
	"fmt"
	"strings"
	"station/pkg/models"

	"github.com/xeipuuv/gojsonschema"
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
	
	// Start input schema section with JSON Schema format
	content.WriteString("input:\n")
	content.WriteString("  schema:\n")
	content.WriteString("    type: object\n")
	content.WriteString("    properties:\n")
	
	// Always include the mandatory userInput
	content.WriteString("      userInput:\n")
	content.WriteString("        type: string\n")
	content.WriteString("        description: User input for the agent\n")
	
	// Track required fields
	requiredFields := []string{"userInput"}
	
	// Add custom schema if defined
	if agent.InputSchema != nil && *agent.InputSchema != "" {
		// First validate the schema
		if err := h.ValidateInputSchema(*agent.InputSchema); err != nil {
			return "", fmt.Errorf("invalid input schema in agent: %w", err)
		}
		
		// Parse the JSON schema to extract properties
		var schema map[string]interface{}
		if err := json.Unmarshal([]byte(*agent.InputSchema), &schema); err != nil {
			return "", fmt.Errorf("failed to parse input schema: %w", err)
		}
		
		// Extract properties from the schema
		if properties, ok := schema["properties"].(map[string]interface{}); ok {
			for key, prop := range properties {
				// Skip userInput as it's already added
				if key != "userInput" {
					h.writeJSONSchemaPropertyFromRaw(&content, key, prop)
				}
			}
		}
		
		// Extract required fields from the schema
		if required, ok := schema["required"].([]interface{}); ok {
			for _, field := range required {
				if fieldName, ok := field.(string); ok && fieldName != "userInput" {
					requiredFields = append(requiredFields, fieldName)
				}
			}
		}
	}
	
	// Add required fields array
	if len(requiredFields) > 0 {
		content.WriteString("    required:\n")
		for _, field := range requiredFields {
			content.WriteString(fmt.Sprintf("      - %s\n", field))
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
		// First validate the schema itself
		if err := h.ValidateInputSchema(*agent.InputSchema); err != nil {
			return nil, fmt.Errorf("invalid agent input schema: %w", err)
		}
		
		// Merge custom data (excluding userInput)
		if customData != nil {
			for key, value := range customData {
				if key != "userInput" {
					result[key] = value
				}
			}
			
			// Validate merged data against schema using gojsonschema
			if err := h.validateDataAgainstSchema(result, *agent.InputSchema); err != nil {
				return nil, fmt.Errorf("input validation failed: %w", err)
			}
		}
	}
	
	return result, nil
}

// validateDataAgainstSchema validates input data against a JSON Schema
func (h *ExportHelper) validateDataAgainstSchema(data map[string]interface{}, schemaJSON string) error {
	// Load the schema
	schemaLoader := gojsonschema.NewStringLoader(schemaJSON)
	
	// Convert data to JSON for validation
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data for validation: %w", err)
	}
	dataLoader := gojsonschema.NewStringLoader(string(dataJSON))
	
	// Validate
	result, err := gojsonschema.Validate(schemaLoader, dataLoader)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}
	
	if !result.Valid() {
		var errors []string
		for _, desc := range result.Errors() {
			errors = append(errors, desc.String())
		}
		return fmt.Errorf("validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}


// writeJSONSchemaPropertyFromRaw writes a JSON Schema property from raw parsed JSON
func (h *ExportHelper) writeJSONSchemaPropertyFromRaw(content *strings.Builder, key string, property interface{}) {
	content.WriteString(fmt.Sprintf("      %s:\n", key))
	
	if propMap, ok := property.(map[string]interface{}); ok {
		// Write type
		if propType, exists := propMap["type"]; exists {
			content.WriteString(fmt.Sprintf("        type: %v\n", propType))
		}
		
		// Write enum if present
		if enumValues, exists := propMap["enum"]; exists {
			if enumArray, ok := enumValues.([]interface{}); ok {
				content.WriteString("        enum:\n")
				for _, val := range enumArray {
					content.WriteString(fmt.Sprintf("          - %v\n", val))
				}
			}
		}
		
		// Write description if present
		if desc, exists := propMap["description"]; exists {
			content.WriteString(fmt.Sprintf("        description: %v\n", desc))
		}
		
		// Write default if present
		if defaultVal, exists := propMap["default"]; exists {
			content.WriteString(fmt.Sprintf("        default: %v\n", defaultVal))
		}
	}
}

// ValidateInputSchema validates that an input schema JSON is valid using proper JSON Schema validation
func (h *ExportHelper) ValidateInputSchema(schemaJSON string) error {
	if schemaJSON == "" {
		return nil // Empty schema is valid
	}
	
	// Parse as JSON to ensure it's valid JSON first
	var schemaObj interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &schemaObj); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	
	// Validate as JSON Schema using gojsonschema
	schemaLoader := gojsonschema.NewStringLoader(schemaJSON)
	_, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return fmt.Errorf("invalid JSON Schema: %w", err)
	}
	
	return nil
}

// ValidateOutputSchema validates that an output schema JSON is valid using proper JSON Schema validation
func (h *ExportHelper) ValidateOutputSchema(schemaJSON string) error {
	if schemaJSON == "" {
		return nil // Empty schema is valid
	}
	
	// Parse as JSON to ensure it's valid JSON first
	var schemaObj interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &schemaObj); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	
	// Validate as JSON Schema using gojsonschema
	schemaLoader := gojsonschema.NewStringLoader(schemaJSON)
	_, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return fmt.Errorf("invalid JSON Schema: %w", err)
	}
	
	return nil
}