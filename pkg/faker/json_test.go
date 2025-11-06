package faker

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestJSONUnmarshalingFloats tests how JSON unmarshaling affects float detection
func TestJSONUnmarshalingFloats(t *testing.T) {
	// Test how JSON unmarshaling affects float values
	jsonData := `{"balance": 100.50, "is_active": true}`
	
	var data map[string]interface{}
	err := json.Unmarshal([]byte(jsonData), &data)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	
	fmt.Printf("JSON unmarshaled data:\n")
	for key, value := range data {
		fmt.Printf("  %s: %v (type: %T)\n", key, value, value)
	}
	
	// Test schema analysis on JSON-unmarshaled data
	schemaCache, err := NewSchemaCache("/tmp/test-cache")
	if err != nil {
		t.Fatalf("Failed to create schema cache: %v", err)
	}
	
	err = schemaCache.AnalyzeResponse("json-test", data)
	if err != nil {
		t.Fatalf("Failed to analyze JSON data: %v", err)
	}
	
	schema, exists := schemaCache.GetSchema("json-test")
	if !exists {
		t.Fatal("Schema not found after analysis")
	}
	
	fmt.Printf("\nSchema from JSON data:\n")
	for fieldName, fieldSchema := range schema.Children {
		fmt.Printf("  %s: type=%s, samples=%v\n", fieldName, fieldSchema.Type, fieldSchema.Samples)
	}
}