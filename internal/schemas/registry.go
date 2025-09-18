package schemas

import (
	"fmt"
	"station/internal/schemas/presets"
)

// SchemaRegistry manages predefined output schemas
type SchemaRegistry struct {
	presets map[string]string
}

// NewSchemaRegistry creates a new schema registry
func NewSchemaRegistry() *SchemaRegistry {
	registry := &SchemaRegistry{
		presets: make(map[string]string),
	}
	
	// Register predefined schemas
	registry.registerPresets()
	
	return registry
}

// registerPresets registers all predefined schema presets
func (r *SchemaRegistry) registerPresets() {
	r.presets["finops"] = presets.FinOpsSchema
}

// GetPresetSchema returns a predefined schema by name
func (r *SchemaRegistry) GetPresetSchema(presetName string) (string, error) {
	schema, exists := r.presets[presetName]
	if !exists {
		return "", fmt.Errorf("unknown schema preset: %s", presetName)
	}
	return schema, nil
}

// ListPresets returns all available preset names
func (r *SchemaRegistry) ListPresets() []string {
	presets := make([]string, 0, len(r.presets))
	for name := range r.presets {
		presets = append(presets, name)
	}
	return presets
}

// HasPreset checks if a preset exists
func (r *SchemaRegistry) HasPreset(presetName string) bool {
	_, exists := r.presets[presetName]
	return exists
}