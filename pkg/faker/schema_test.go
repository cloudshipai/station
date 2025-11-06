package faker

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSchemaAnalyzerPrimitives tests schema analysis of primitive types
func TestSchemaAnalyzerPrimitives(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		wantType string
	}{
		{"string", "test", "string"},
		{"number", 123.45, "number"},
		{"bool", true, "bool"},
		{"null", nil, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := analyzeValue(tt.value)
			if schema.Type != tt.wantType {
				t.Errorf("Expected type %s, got %s", tt.wantType, schema.Type)
			}
		})
	}
}

// TestSchemaAnalyzerArray tests array schema analysis
func TestSchemaAnalyzerArray(t *testing.T) {
	arrayValue := []interface{}{
		"item1",
		"item2",
		"item3",
	}

	schema := analyzeValue(arrayValue)

	if schema.Type != "array" {
		t.Errorf("Expected type array, got %s", schema.Type)
	}

	if !schema.IsArray {
		t.Error("Expected IsArray to be true")
	}

	if schema.ArrayMin != 3 || schema.ArrayMax != 3 {
		t.Errorf("Expected array bounds [3,3], got [%d,%d]", schema.ArrayMin, schema.ArrayMax)
	}

	if schema.ItemType == nil {
		t.Fatal("Expected ItemType to be set")
	}

	if schema.ItemType.Type != "string" {
		t.Errorf("Expected item type string, got %s", schema.ItemType.Type)
	}
}

// TestSchemaAnalyzerObject tests object schema analysis
func TestSchemaAnalyzerObject(t *testing.T) {
	objectValue := map[string]interface{}{
		"name":  "test",
		"count": 42.0,
		"active": true,
	}

	schema := analyzeValue(objectValue)

	if schema.Type != "object" {
		t.Errorf("Expected type object, got %s", schema.Type)
	}

	if len(schema.Children) != 3 {
		t.Errorf("Expected 3 children, got %d", len(schema.Children))
	}

	if schema.Children["name"].Type != "string" {
		t.Errorf("Expected name to be string, got %s", schema.Children["name"].Type)
	}

	if schema.Children["count"].Type != "number" {
		t.Errorf("Expected count to be number, got %s", schema.Children["count"].Type)
	}

	if schema.Children["active"].Type != "bool" {
		t.Errorf("Expected active to be bool, got %s", schema.Children["active"].Type)
	}
}

// TestSchemaAnalyzerNested tests nested structure analysis
func TestSchemaAnalyzerNested(t *testing.T) {
	nestedValue := map[string]interface{}{
		"user": map[string]interface{}{
			"id":   1.0,
			"name": "Alice",
			"tags": []interface{}{"admin", "user"},
		},
		"items": []interface{}{
			map[string]interface{}{
				"id":    1.0,
				"title": "Item 1",
			},
			map[string]interface{}{
				"id":    2.0,
				"title": "Item 2",
			},
		},
	}

	schema := analyzeValue(nestedValue)

	// Check top-level structure
	if schema.Type != "object" {
		t.Fatalf("Expected object type, got %s", schema.Type)
	}

	// Check user object
	userSchema := schema.Children["user"]
	if userSchema == nil || userSchema.Type != "object" {
		t.Fatal("Expected user to be an object")
	}

	if userSchema.Children["id"].Type != "number" {
		t.Errorf("Expected user.id to be number")
	}

	if userSchema.Children["tags"].Type != "array" {
		t.Errorf("Expected user.tags to be array")
	}

	// Check items array
	itemsSchema := schema.Children["items"]
	if itemsSchema == nil || itemsSchema.Type != "array" {
		t.Fatal("Expected items to be an array")
	}

	if itemsSchema.ItemType == nil || itemsSchema.ItemType.Type != "object" {
		t.Fatal("Expected items to contain objects")
	}

	if itemsSchema.ItemType.Children["title"].Type != "string" {
		t.Errorf("Expected item.title to be string")
	}
}

// TestSchemaMerge tests schema merging functionality
func TestSchemaMerge(t *testing.T) {
	schema1 := &SchemaNode{
		Type:     "object",
		Children: map[string]*SchemaNode{
			"field1": {Type: "string"},
			"field2": {Type: "number"},
		},
	}

	schema2 := &SchemaNode{
		Type:     "object",
		Children: map[string]*SchemaNode{
			"field2": {Type: "number"},
			"field3": {Type: "bool"},
		},
	}

	merged := mergeSchemas(schema1, schema2)

	if len(merged.Children) != 3 {
		t.Errorf("Expected 3 fields after merge, got %d", len(merged.Children))
	}

	if merged.Children["field1"] == nil {
		t.Error("Expected field1 to be present")
	}

	if merged.Children["field2"] == nil {
		t.Error("Expected field2 to be present")
	}

	if merged.Children["field3"] == nil {
		t.Error("Expected field3 to be present")
	}
}

// TestSchemaMergeArrayBounds tests array bounds merging
func TestSchemaMergeArrayBounds(t *testing.T) {
	schema1 := &SchemaNode{
		Type:     "array",
		IsArray:  true,
		ArrayMin: 2,
		ArrayMax: 5,
		ItemType: &SchemaNode{Type: "string"},
	}

	schema2 := &SchemaNode{
		Type:     "array",
		IsArray:  true,
		ArrayMin: 3,
		ArrayMax: 8,
		ItemType: &SchemaNode{Type: "string"},
	}

	merged := mergeSchemas(schema1, schema2)

	if merged.ArrayMin != 2 {
		t.Errorf("Expected min bound 2, got %d", merged.ArrayMin)
	}

	if merged.ArrayMax != 8 {
		t.Errorf("Expected max bound 8, got %d", merged.ArrayMax)
	}
}

// TestSchemaCache tests schema caching functionality
func TestSchemaCache(t *testing.T) {
	tempDir := t.TempDir()

	cache, err := NewSchemaCache(tempDir)
	if err != nil {
		t.Fatalf("Failed to create schema cache: %v", err)
	}

	// Test storing a schema
	testResponse := map[string]interface{}{
		"result": map[string]interface{}{
			"id":    1.0,
			"name":  "test",
			"count": 42.0,
		},
	}

	err = cache.AnalyzeResponse("test-tool", testResponse["result"])
	if err != nil {
		t.Fatalf("Failed to analyze response: %v", err)
	}

	// Test retrieving the schema
	schema, exists := cache.GetSchema("test-tool")
	if !exists {
		t.Fatal("Expected schema to exist in cache")
	}

	if schema.Type != "object" {
		t.Errorf("Expected object type, got %s", schema.Type)
	}

	if len(schema.Children) != 3 {
		t.Errorf("Expected 3 children, got %d", len(schema.Children))
	}

	// Verify schema was persisted to disk
	schemaFile := filepath.Join(tempDir, "test-tool.json")
	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		t.Error("Expected schema file to be created on disk")
	}
}

// TestSchemaCachePersistence tests loading schemas from disk
func TestSchemaCachePersistence(t *testing.T) {
	tempDir := t.TempDir()

	// Create first cache and store a schema
	cache1, err := NewSchemaCache(tempDir)
	if err != nil {
		t.Fatalf("Failed to create first cache: %v", err)
	}

	testResponse := map[string]interface{}{
		"status": "success",
		"count":  10.0,
	}

	err = cache1.AnalyzeResponse("persistent-tool", testResponse)
	if err != nil {
		t.Fatalf("Failed to analyze response: %v", err)
	}

	// Create second cache (should load from disk)
	cache2, err := NewSchemaCache(tempDir)
	if err != nil {
		t.Fatalf("Failed to create second cache: %v", err)
	}

	// Verify schema was loaded from disk
	schema, exists := cache2.GetSchema("persistent-tool")
	if !exists {
		t.Fatal("Expected schema to be loaded from disk")
	}

	if schema.Type != "object" {
		t.Errorf("Expected object type, got %s", schema.Type)
	}

	if schema.Children["status"].Type != "string" {
		t.Error("Expected status field to be string")
	}

	if schema.Children["count"].Type != "number" {
		t.Error("Expected count field to be number")
	}
}

// TestSchemaAnalyzerMCPResponse tests realistic MCP response
func TestSchemaAnalyzerMCPResponse(t *testing.T) {
	// Simulate a tools/list response
	mcpResponse := map[string]interface{}{
		"tools": []interface{}{
			map[string]interface{}{
				"name":        "read_file",
				"description": "Read a file",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
			map[string]interface{}{
				"name":        "write_file",
				"description": "Write a file",
				"inputSchema": map[string]interface{}{
					"type": "object",
				},
			},
		},
	}

	schema := analyzeValue(mcpResponse)

	// Verify structure
	if schema.Type != "object" {
		t.Fatalf("Expected object, got %s", schema.Type)
	}

	toolsSchema := schema.Children["tools"]
	if toolsSchema == nil || toolsSchema.Type != "array" {
		t.Fatal("Expected tools array")
	}

	if toolsSchema.ItemType == nil || toolsSchema.ItemType.Type != "object" {
		t.Fatal("Expected tools to contain objects")
	}

	// Check tool object structure
	toolSchema := toolsSchema.ItemType
	if toolSchema.Children["name"] == nil || toolSchema.Children["name"].Type != "string" {
		t.Error("Expected tool.name to be string")
	}

	if toolSchema.Children["inputSchema"] == nil || toolSchema.Children["inputSchema"].Type != "object" {
		t.Error("Expected tool.inputSchema to be object")
	}
}

// TestSchemaUpdateVariations tests that cache captures variations
func TestSchemaUpdateVariations(t *testing.T) {
	tempDir := t.TempDir()

	cache, err := NewSchemaCache(tempDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// First response with fields A and B
	response1 := map[string]interface{}{
		"fieldA": "value",
		"fieldB": 123.0,
	}

	err = cache.AnalyzeResponse("varying-tool", response1)
	if err != nil {
		t.Fatalf("Failed to analyze first response: %v", err)
	}

	// Second response with fields B and C
	response2 := map[string]interface{}{
		"fieldB": 456.0,
		"fieldC": true,
	}

	err = cache.AnalyzeResponse("varying-tool", response2)
	if err != nil {
		t.Fatalf("Failed to analyze second response: %v", err)
	}

	// Verify merged schema has all three fields
	schema, exists := cache.GetSchema("varying-tool")
	if !exists {
		t.Fatal("Expected schema to exist")
	}

	if len(schema.Children) != 3 {
		t.Errorf("Expected 3 fields (A, B, C), got %d", len(schema.Children))
	}

	if schema.Children["fieldA"] == nil {
		t.Error("Expected fieldA from first response")
	}

	if schema.Children["fieldB"] == nil {
		t.Error("Expected fieldB from both responses")
	}

	if schema.Children["fieldC"] == nil {
		t.Error("Expected fieldC from second response")
	}
}
