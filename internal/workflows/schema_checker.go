package workflows

import (
	"encoding/json"
	"fmt"
)

type SchemaCompatibility struct {
	Compatible bool
	Issues     []string
	Warnings   []string
}

type SchemaChecker struct{}

func NewSchemaChecker() *SchemaChecker {
	return &SchemaChecker{}
}

func (c *SchemaChecker) CheckCompatibility(outputSchemaJSON, inputSchemaJSON string) SchemaCompatibility {
	result := SchemaCompatibility{
		Compatible: true,
		Issues:     []string{},
		Warnings:   []string{},
	}

	if outputSchemaJSON == "" || inputSchemaJSON == "" {
		return result
	}

	var outputSchema, inputSchema map[string]interface{}

	if err := json.Unmarshal([]byte(outputSchemaJSON), &outputSchema); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to parse output schema: %v", err))
		return result
	}

	if err := json.Unmarshal([]byte(inputSchemaJSON), &inputSchema); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to parse input schema: %v", err))
		return result
	}

	inputRequired := c.extractRequired(inputSchema)
	inputProps := c.extractProperties(inputSchema)
	outputProps := c.extractProperties(outputSchema)

	for _, reqField := range inputRequired {
		if _, exists := outputProps[reqField]; !exists {
			result.Compatible = false
			result.Issues = append(result.Issues,
				fmt.Sprintf("input requires field '%s' but output schema does not provide it", reqField))
		}
	}

	for fieldName, inputPropDef := range inputProps {
		outputPropDef, exists := outputProps[fieldName]
		if !exists {
			if c.isRequired(fieldName, inputRequired) {
				continue
			}
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("input expects optional field '%s' which output may not provide", fieldName))
			continue
		}

		if err := c.checkTypeCompatibility(fieldName, outputPropDef, inputPropDef); err != nil {
			result.Compatible = false
			result.Issues = append(result.Issues, err.Error())
		}
	}

	return result
}

func (c *SchemaChecker) extractRequired(schema map[string]interface{}) []string {
	required := []string{}
	if reqRaw, ok := schema["required"].([]interface{}); ok {
		for _, r := range reqRaw {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
	}
	return required
}

func (c *SchemaChecker) extractProperties(schema map[string]interface{}) map[string]map[string]interface{} {
	props := make(map[string]map[string]interface{})
	if propsRaw, ok := schema["properties"].(map[string]interface{}); ok {
		for name, def := range propsRaw {
			if defMap, ok := def.(map[string]interface{}); ok {
				props[name] = defMap
			}
		}
	}
	return props
}

func (c *SchemaChecker) isRequired(field string, required []string) bool {
	for _, r := range required {
		if r == field {
			return true
		}
	}
	return false
}

func (c *SchemaChecker) checkTypeCompatibility(fieldName string, outputDef, inputDef map[string]interface{}) error {
	outputType, _ := outputDef["type"].(string)
	inputType, _ := inputDef["type"].(string)

	if outputType == "" || inputType == "" {
		return nil
	}

	if !c.typesCompatible(outputType, inputType) {
		return fmt.Errorf("field '%s': output type '%s' is not compatible with input type '%s'",
			fieldName, outputType, inputType)
	}

	if outputType == "array" && inputType == "array" {
		outputItems, _ := outputDef["items"].(map[string]interface{})
		inputItems, _ := inputDef["items"].(map[string]interface{})

		if outputItems != nil && inputItems != nil {
			outputItemType, _ := outputItems["type"].(string)
			inputItemType, _ := inputItems["type"].(string)

			if outputItemType != "" && inputItemType != "" && !c.typesCompatible(outputItemType, inputItemType) {
				return fmt.Errorf("field '%s': array item type '%s' is not compatible with expected '%s'",
					fieldName, outputItemType, inputItemType)
			}
		}
	}

	return nil
}

func (c *SchemaChecker) typesCompatible(outputType, inputType string) bool {
	if outputType == inputType {
		return true
	}

	if outputType == "integer" && inputType == "number" {
		return true
	}

	return false
}
