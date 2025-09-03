package agent

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// extractInt64FromTokenUsage safely extracts int64 from various numeric types in token usage
func extractInt64FromTokenUsage(value interface{}) *int64 {
	if value == nil {
		return nil
	}
	
	switch v := value.(type) {
	case int64:
		return &v
	case int:
		val := int64(v)
		return &val
	case int32:
		val := int64(v)
		return &val
	case float64:
		val := int64(v)
		return &val
	case float32:
		val := int64(v)
		return &val
	default:
		return nil
	}
}

// loadVariablesFromFile loads variables from a YAML file
func (h *AgentHandler) loadVariablesFromFile(filePath string) (map[string]interface{}, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return make(map[string]interface{}), nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read variables file: %v", err)
	}

	var variables map[string]interface{}
	if err := yaml.Unmarshal(content, &variables); err != nil {
		return nil, fmt.Errorf("failed to parse variables file: %v", err)
	}

	return variables, nil
}


