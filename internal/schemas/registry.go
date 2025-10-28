package schemas

import (
	"fmt"
	"station/pkg/schema"
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

	return registry
}

// GetPresetInfo returns preset information including app, app_type, and schema
func (r *SchemaRegistry) GetPresetInfo(presetName string) (schema.PresetInfo, bool) {
	return schema.GetPresetInfo(presetName)
}

// GetPresetSchema returns a predefined schema by name
func (r *SchemaRegistry) GetPresetSchema(presetName string) (string, error) {
	info, exists := schema.GetPresetInfo(presetName)
	if !exists {
		return "", fmt.Errorf("unknown schema preset: %s", presetName)
	}
	return schema.SchemaToJSON(info.Schema), nil
}

// ListPresets returns all available preset names
func (r *SchemaRegistry) ListPresets() []string {
	return schema.ListPresets()
}

// HasPreset checks if a preset exists
func (r *SchemaRegistry) HasPreset(presetName string) bool {
	_, exists := schema.GetPresetInfo(presetName)
	return exists
}
