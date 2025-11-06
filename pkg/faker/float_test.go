package faker

import (
	"fmt"
	"testing"
)

// TestFloatDetection tests if float64 values are being detected correctly
func TestFloatDetection(t *testing.T) {
	// Test direct schema analysis
	balanceValue := 100.50
	schema := analyzeValue(balanceValue)
	
	fmt.Printf("Direct float64 analysis: type=%s, samples=%v\n", schema.Type, schema.Samples)
	
	if schema.Type != "number" {
		t.Errorf("Expected 'number', got '%s'", schema.Type)
	}
	
	// Test with map analysis
	sampleData := map[string]interface{}{
		"balance": 100.50,
	}
	
	schemaCache, err := NewSchemaCache("/tmp/test-cache")
	if err != nil {
		t.Fatalf("Failed to create schema cache: %v", err)
	}
	
	err = schemaCache.AnalyzeResponse("float-test", sampleData)
	if err != nil {
		t.Fatalf("Failed to analyze sample data: %v", err)
	}
	
	schema, exists := schemaCache.GetSchema("float-test")
	if !exists {
		t.Fatal("Schema not found after analysis")
	}
	
	fmt.Printf("Map analysis: type=%s, children=%v\n", schema.Type, getSchemaChildKeys(schema.Children))
	
	if balanceSchema, ok := schema.Children["balance"]; ok {
		fmt.Printf("Balance field: type=%s, samples=%v\n", balanceSchema.Type, balanceSchema.Samples)
		if balanceSchema.Type != "number" {
			t.Errorf("Expected balance type 'number', got '%s'", balanceSchema.Type)
		}
	} else {
		t.Error("Balance field not found in schema")
	}
}