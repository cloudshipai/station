package faker

import (
	"testing"
)

// TestEnricherStringGeneration tests string field enrichment
func TestEnricherStringGeneration(t *testing.T) {
	cache := &SchemaCache{
		schemas: make(map[string]*SchemaNode),
	}
	enricher := NewEnricher(cache)

	tests := []struct {
		name   string
		schema *SchemaNode
		value  interface{}
		verify func(result interface{}) bool
	}{
		{
			name:   "email sample",
			schema: &SchemaNode{Type: "string", Samples: []interface{}{"test@example.com"}},
			value:  "",
			verify: func(result interface{}) bool {
				str, ok := result.(string)
				return ok && len(str) > 0
			},
		},
		{
			name:   "existing value",
			schema: &SchemaNode{Type: "string"},
			value:  "existing",
			verify: func(result interface{}) bool {
				return result == "existing"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := enricher.enrichString(tt.schema, tt.value)
			if err != nil {
				t.Fatalf("enrichString failed: %v", err)
			}
			if !tt.verify(result) {
				t.Errorf("enrichString result verification failed: %v", result)
			}
		})
	}
}

// TestEnricherFieldByName tests field-specific generation
func TestEnricherFieldByName(t *testing.T) {
	cache := &SchemaCache{
		schemas: make(map[string]*SchemaNode),
	}
	enricher := NewEnricher(cache)

	tests := []struct {
		fieldName string
		schema    *SchemaNode
		verify    func(result interface{}) bool
	}{
		{
			fieldName: "email",
			schema:    &SchemaNode{Type: "string"},
			verify: func(result interface{}) bool {
				str, ok := result.(string)
				return ok && len(str) > 0
			},
		},
		{
			fieldName: "id",
			schema:    &SchemaNode{Type: "string"},
			verify: func(result interface{}) bool {
				str, ok := result.(string)
				return ok && len(str) > 0
			},
		},
		{
			fieldName: "count",
			schema:    &SchemaNode{Type: "number"},
			verify: func(result interface{}) bool {
				_, ok := result.(float64)
				return ok
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			result, err := enricher.enrichFieldByName(tt.fieldName, tt.schema, nil)
			if err != nil {
				t.Fatalf("enrichFieldByName failed: %v", err)
			}
			if !tt.verify(result) {
				t.Errorf("enrichFieldByName result verification failed: %v", result)
			}
		})
	}
}

// TestEnricherObject tests object enrichment
func TestEnricherObject(t *testing.T) {
	cache := &SchemaCache{
		schemas: make(map[string]*SchemaNode),
	}
	enricher := NewEnricher(cache)

	schema := &SchemaNode{
		Type: "object",
		Children: map[string]*SchemaNode{
			"name":  {Type: "string"},
			"email": {Type: "string"},
			"age":   {Type: "number"},
		},
	}

	result, err := enricher.enrichObject(schema, nil)
	if err != nil {
		t.Fatalf("enrichObject failed: %v", err)
	}

	objMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", result)
	}

	if len(objMap) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(objMap))
	}

	if _, ok := objMap["name"].(string); !ok {
		t.Error("Expected name to be string")
	}

	if _, ok := objMap["email"].(string); !ok {
		t.Error("Expected email to be string")
	}

	if _, ok := objMap["age"].(float64); !ok {
		t.Error("Expected age to be float64")
	}
}

// TestEnricherArray tests array enrichment
func TestEnricherArray(t *testing.T) {
	cache := &SchemaCache{
		schemas: make(map[string]*SchemaNode),
	}
	enricher := NewEnricher(cache)

	schema := &SchemaNode{
		Type:     "array",
		IsArray:  true,
		ArrayMin: 2,
		ArrayMax: 5,
		ItemType: &SchemaNode{Type: "string"},
	}

	result, err := enricher.enrichArray(schema, nil)
	if err != nil {
		t.Fatalf("enrichArray failed: %v", err)
	}

	arr, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}

	if len(arr) < 2 || len(arr) > 5 {
		t.Errorf("Expected array length between 2 and 5, got %d", len(arr))
	}

	for i, item := range arr {
		if _, ok := item.(string); !ok {
			t.Errorf("Expected item %d to be string, got %T", i, item)
		}
	}
}
