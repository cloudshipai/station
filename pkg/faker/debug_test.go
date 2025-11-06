package faker

import (
	"fmt"
	"testing"
)

// TestFieldSpecificEnrichmentDebug debugs the field enrichment issues
func TestFieldSpecificEnrichmentDebug(t *testing.T) {
	schemaCache, err := NewSchemaCache("/tmp/test-cache")
	if err != nil {
		t.Fatalf("Failed to create schema cache: %v", err)
	}
	enricher := NewEnricher(schemaCache)

	// First, analyze sample data to build schema
	sampleData := map[string]interface{}{
		"user_id":    "123e4567-e89b-12d3-a456-426614174000",
		"email":      "test@example.com",
		"created_at": "2024-01-01T00:00:00Z",
		"is_active":  true,
		"balance":    100.50,
		"status":     "active",
		"ip_address": "192.168.1.1",
		"website":    "https://example.com",
		"phone":      "+1-555-0123",
	}

	// Analyze sample data to build schema
	err = schemaCache.AnalyzeResponse("field-test", sampleData)
	if err != nil {
		t.Fatalf("Failed to analyze sample data: %v", err)
	}

	// Get the learned schema
	schema, exists := schemaCache.GetSchema("field-test")
	if !exists {
		t.Fatal("Schema not found after analysis")
	}

	fmt.Printf("Learned schema type: %s\n", schema.Type)
	fmt.Printf("Learned schema children: %v\n", getSchemaChildKeys(schema.Children))

	// Check each field's schema
	for fieldName, fieldSchema := range schema.Children {
		fmt.Printf("Field '%s': type=%s, samples=%v\n", fieldName, fieldSchema.Type, fieldSchema.Samples)
	}

	// Now test enrichment with empty data
	testData := map[string]interface{}{}

	// Enrich the test data
	enriched, err := enricher.EnrichResponse("field-test", testData)
	if err != nil {
		t.Fatalf("Failed to enrich test data: %v", err)
	}

	// Print what was actually generated
	fmt.Printf("\nEnriched response:\n")
	for field, value := range enriched {
		fmt.Printf("  %s: %v (type: %T)\n", field, value, value)
	}

	// Test specific problematic fields
	problemFields := []string{"balance", "ip_address", "website"}
	for _, field := range problemFields {
		if value, ok := enriched[field]; ok {
			fmt.Printf("\nField '%s' generated: %v (type: %T)\n", field, value, value)
			
			// Check if this field has a schema
			if fieldSchema, exists := schema.Children[field]; exists {
				fmt.Printf("  Schema: type=%s, samples=%v\n", fieldSchema.Type, fieldSchema.Samples)
			} else {
				fmt.Printf("  No schema found for this field!\n")
			}
		} else {
			fmt.Printf("\nField '%s' NOT generated in enriched response\n", field)
		}
	}
}

func getSchemaChildKeys(children map[string]*SchemaNode) []string {
	var keys []string
	for k := range children {
		keys = append(keys, k)
	}
	return keys
}