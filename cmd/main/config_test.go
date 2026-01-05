package main

import (
	"os"
	"path/filepath"
	"testing"

	"station/internal/config"
)

func TestParseValueForType(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldType config.FieldType
		want      interface{}
		wantErr   bool
	}{
		{"string value", "hello", config.FieldTypeString, "hello", false},
		{"int value", "42", config.FieldTypeInt, 42, false},
		{"invalid int", "not-a-number", config.FieldTypeInt, nil, true},
		{"bool true", "true", config.FieldTypeBool, true, false},
		{"bool false", "false", config.FieldTypeBool, false, false},
		{"invalid bool", "maybe", config.FieldTypeBool, nil, true},
		{"string slice", "a,b,c", config.FieldTypeStringSlice, []string{"a", "b", "c"}, false},
		{"empty string slice", "", config.FieldTypeStringSlice, []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseValueForType(tt.value, tt.fieldType)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseValueForType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			switch want := tt.want.(type) {
			case []string:
				gotSlice, ok := got.([]string)
				if !ok {
					t.Errorf("parseValueForType() got type %T, want []string", got)
					return
				}
				if len(gotSlice) != len(want) {
					t.Errorf("parseValueForType() got %v, want %v", got, tt.want)
				}
			default:
				if got != tt.want {
					t.Errorf("parseValueForType() got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestInferValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  interface{}
	}{
		{"integer", "42", 42},
		{"bool true", "true", true},
		{"bool false", "false", false},
		{"comma-separated", "a,b,c", []string{"a", "b", "c"}},
		{"plain string", "hello world", "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferValue(tt.value)

			switch want := tt.want.(type) {
			case []string:
				gotSlice, ok := got.([]string)
				if !ok {
					t.Errorf("inferValue() got type %T, want []string", got)
					return
				}
				if len(gotSlice) != len(want) {
					t.Errorf("inferValue() got %v, want %v", got, tt.want)
				}
			default:
				if got != tt.want {
					t.Errorf("inferValue() got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestSetNestedValue(t *testing.T) {
	tests := []struct {
		name     string
		initial  map[string]interface{}
		key      string
		value    interface{}
		expected map[string]interface{}
	}{
		{
			name:     "top level key",
			initial:  map[string]interface{}{},
			key:      "foo",
			value:    "bar",
			expected: map[string]interface{}{"foo": "bar"},
		},
		{
			name:     "nested key",
			initial:  map[string]interface{}{},
			key:      "foo.bar",
			value:    "baz",
			expected: map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}},
		},
		{
			name:     "deeply nested key",
			initial:  map[string]interface{}{},
			key:      "a.b.c",
			value:    123,
			expected: map[string]interface{}{"a": map[string]interface{}{"b": map[string]interface{}{"c": 123}}},
		},
		{
			name:     "update existing",
			initial:  map[string]interface{}{"foo": map[string]interface{}{"bar": "old"}},
			key:      "foo.bar",
			value:    "new",
			expected: map[string]interface{}{"foo": map[string]interface{}{"bar": "new"}},
		},
		{
			name:     "add to existing nested",
			initial:  map[string]interface{}{"foo": map[string]interface{}{"bar": "existing"}},
			key:      "foo.baz",
			value:    "new",
			expected: map[string]interface{}{"foo": map[string]interface{}{"bar": "existing", "baz": "new"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.initial
			setNestedValue(m, tt.key, tt.value)

			if !mapsEqual(m, tt.expected) {
				t.Errorf("setNestedValue() got %v, want %v", m, tt.expected)
			}
		})
	}
}

func TestDeleteNestedKey(t *testing.T) {
	tests := []struct {
		name     string
		initial  map[string]interface{}
		parts    []string
		expected map[string]interface{}
	}{
		{
			name:     "delete top level",
			initial:  map[string]interface{}{"foo": "bar", "baz": "qux"},
			parts:    []string{"foo"},
			expected: map[string]interface{}{"baz": "qux"},
		},
		{
			name:     "delete nested key",
			initial:  map[string]interface{}{"foo": map[string]interface{}{"bar": "baz", "qux": "quux"}},
			parts:    []string{"foo", "bar"},
			expected: map[string]interface{}{"foo": map[string]interface{}{"qux": "quux"}},
		},
		{
			name:     "delete nested cleans empty parent",
			initial:  map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}},
			parts:    []string{"foo", "bar"},
			expected: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.initial
			deleteNestedKey(m, tt.parts)

			if !mapsEqual(m, tt.expected) {
				t.Errorf("deleteNestedKey() got %v, want %v", m, tt.expected)
			}
		})
	}
}

func TestRedactSecrets(t *testing.T) {
	input := map[string]interface{}{
		"ai_provider": "openai",
		"ai_api_key":  "sk-secret-key",
		"cloudship": map[string]interface{}{
			"enabled":          true,
			"registration_key": "reg-key-123",
		},
	}

	result := redactSecrets(input)

	if result["ai_provider"] != "openai" {
		t.Errorf("non-secret should not be redacted: got %v", result["ai_provider"])
	}

	if result["ai_api_key"] != "***REDACTED***" {
		t.Errorf("ai_api_key should be redacted: got %v", result["ai_api_key"])
	}

	cloudship, ok := result["cloudship"].(map[string]interface{})
	if !ok {
		t.Fatal("cloudship should be a map")
	}

	if cloudship["enabled"] != true {
		t.Errorf("non-secret nested should not be redacted: got %v", cloudship["enabled"])
	}

	if cloudship["registration_key"] != "***REDACTED***" {
		t.Errorf("registration_key should be redacted: got %v", cloudship["registration_key"])
	}
}

func TestFilterTopLevelKeys(t *testing.T) {
	input := map[string]interface{}{
		"ai_provider":    "openai",
		"ai_model":       "gpt-4",
		"ai_api_key":     "secret",
		"coding_backend": "opencode",
	}

	result := filterTopLevelKeys(input, "ai_")

	if len(result) != 3 {
		t.Errorf("expected 3 keys, got %d", len(result))
	}

	if _, ok := result["coding_backend"]; ok {
		t.Error("coding_backend should not be included")
	}
}

func TestConfigFileOperations(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initial := map[string]interface{}{
		"ai_provider": "openai",
		"ai_model":    "gpt-4",
	}

	setNestedValue(initial, "coding.backend", "claudecode")

	coding, ok := initial["coding"].(map[string]interface{})
	if !ok {
		t.Fatal("coding should be a map")
	}
	if coding["backend"] != "claudecode" {
		t.Errorf("coding.backend = %v, want claudecode", coding["backend"])
	}

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("config file should not exist yet")
	}
}

func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		switch va := va.(type) {
		case map[string]interface{}:
			vbMap, ok := vb.(map[string]interface{})
			if !ok || !mapsEqual(va, vbMap) {
				return false
			}
		default:
			if va != vb {
				return false
			}
		}
	}
	return true
}
