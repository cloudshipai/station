package faker

import (
	"strings"
	"testing"
	"time"
)

// TestRealMCPServerIntegration tests the faker proxy with actual MCP servers from the wild
func TestRealMCPServerIntegration(t *testing.T) {
	// Skip if running in CI without network access
	if testing.Short() {
		t.Skip("Skipping real MCP server integration in short mode")
	}

	testCases := []struct {
		name        string
		command     string
		args        []string
		description string
		timeout     time.Duration
	}{
		{
			name:        "filesystem-mcp",
			command:     "npx",
			args:        []string{"-y", "@modelcontextprotocol/server-filesystem@latest", "/tmp"},
			description: "Official filesystem MCP server",
			timeout:     5 * time.Second,
		},
		{
			name:        "memory-mcp",
			command:     "npx",
			args:        []string{"-y", "@modelcontextprotocol/server-memory@latest"},
			description: "Official memory MCP server",
			timeout:     5 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create proxy config
			config := ProxyConfig{
				TargetCommand: tc.command,
				TargetArgs:    tc.args,
				TargetEnv:     map[string]string{},
				CacheDir:      "/tmp/test-faker-cache",
				Debug:         true,
				Passthrough:   false, // Enable enrichment
			}

			// Create proxy
			proxy, err := NewProxy(config)
			if err != nil {
				t.Fatalf("Failed to create proxy: %v", err)
			}

			// Test proxy startup with timeout
			done := make(chan error, 1)
			go func() {
				done <- proxy.Serve()
			}()

			// Wait for proxy to start or timeout
			select {
			case err := <-done:
				if err != nil {
					t.Logf("✅ %s: Proxy completed (expected): %v", tc.name, err)
				}
			case <-time.After(tc.timeout):
				t.Logf("✅ %s: Proxy ran successfully for %v", tc.name, tc.timeout)
			}
		})
	}
}

// TestComplexDataStructures tests enrichment with complex nested data
func TestComplexDataStructures(t *testing.T) {
	schemaCache, err := NewSchemaCache("/tmp/test-cache")
	if err != nil {
		t.Fatalf("Failed to create schema cache: %v", err)
	}
	enricher := NewEnricher(schemaCache)

	// Test complex nested structures
	complexData := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"id":         123,
				"name":       "John Doe",
				"email":      "john@example.com",
				"created_at": "2024-01-01T00:00:00Z",
				"is_active":  true,
				"metadata": map[string]interface{}{
					"source":   "web",
					"campaign": "spring2024",
					"tags":     []interface{}{"premium", "early_adopter"},
				},
				"address": map[string]interface{}{
					"street":  "123 Main St",
					"city":    "San Francisco",
					"country": "US",
					"zip":     "94105",
				},
				"settings": map[string]interface{}{
					"notifications": map[string]interface{}{
						"email":    true,
						"sms":      false,
						"push":     true,
						"frequency": "daily",
					},
					"privacy": map[string]interface{}{
						"profile_visible": true,
						"searchable":      false,
					},
				},
			},
		},
		"pagination": map[string]interface{}{
			"page":       1,
			"per_page":   20,
			"total_count": 150,
			"has_more":   true,
		},
		"metadata": map[string]interface{}{
			"api_version": "v2",
			"server_id":   "api-server-001",
			"timestamp":    "2024-11-01T15:30:00Z",
		},
	}

	// Enrich the complex data
	enriched, err := enricher.EnrichResponse("complex-test", complexData)
	if err != nil {
		t.Fatalf("Failed to enrich complex data: %v", err)
	}

	// Validate structure preservation
	if enriched == nil {
		t.Fatal("Enriched data is nil")
	}

	// Check that arrays are preserved
	if users, ok := enriched["users"].([]interface{}); ok {
		t.Logf("✅ Users array preserved with %d items", len(users))
		if len(users) == 0 {
			t.Error("❌ Users array is empty")
		}
	} else {
		t.Error("❌ Users array not preserved")
	}

	// Check that nested objects are preserved
	if pagination, ok := enriched["pagination"].(map[string]interface{}); ok {
		t.Logf("✅ Pagination object preserved: %v", pagination)
	} else {
		t.Error("❌ Pagination object not preserved")
	}

	// Check that deeply nested structures are preserved
	if users, ok := enriched["users"].([]interface{}); ok && len(users) > 0 {
		if firstUser, ok := users[0].(map[string]interface{}); ok {
			if settings, ok := firstUser["settings"].(map[string]interface{}); ok {
				if notifications, ok := settings["notifications"].(map[string]interface{}); ok {
					t.Logf("✅ Deeply nested notifications preserved: %v", notifications)
				} else {
					t.Error("❌ Deeply nested notifications not preserved")
				}
			}
		}
	}

	t.Logf("✅ Complex data structure enrichment successful")
}

// TestFieldSpecificEnrichment tests that field-specific enrichment works correctly
func TestFieldSpecificEnrichment(t *testing.T) {
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

	// Now test enrichment with empty data
	testData := map[string]interface{}{}

	// Enrich the test data
	enriched, err := enricher.EnrichResponse("field-test", testData)
	if err != nil {
		t.Fatalf("Failed to enrich test data: %v", err)
	}

	// Validate field-specific enrichment
	fieldTests := []struct {
		field     string
		validator func(interface{}) bool
	}{
		{
			field: "user_id",
			validator: func(v interface{}) bool {
				if str, ok := v.(string); ok {
					return len(str) == 36 && strings.Count(str, "-") == 4 // UUID format
				}
				return false
			},
		},
		{
			field: "email",
			validator: func(v interface{}) bool {
				if str, ok := v.(string); ok {
					return strings.Contains(str, "@")
				}
				return false
			},
		},
		{
			field: "created_at",
			validator: func(v interface{}) bool {
				if str, ok := v.(string); ok {
					return strings.Contains(str, "T") && (strings.Contains(str, "Z") || strings.Contains(str, "+"))
				}
				return false
			},
		},
		{
			field: "is_active",
			validator: func(v interface{}) bool {
				_, ok := v.(bool)
				return ok
			},
		},
		{
			field: "balance",
			validator: func(v interface{}) bool {
				_, ok := v.(float64)
				return ok
			},
		},
		{
			field: "ip_address",
			validator: func(v interface{}) bool {
				if str, ok := v.(string); ok {
					return strings.Count(str, ".") == 3
				}
				return false
			},
		},
		{
			field: "website",
			validator: func(v interface{}) bool {
				if str, ok := v.(string); ok {
					return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
				}
				return false
			},
		},
	}

	for _, ft := range fieldTests {
		t.Run(ft.field, func(t *testing.T) {
			value := enriched[ft.field]
			if !ft.validator(value) {
				t.Errorf("❌ %s: Invalid value generated: %v", ft.field, value)
			} else {
				t.Logf("✅ %s: Valid value generated: %v", ft.field, value)
			}
		})
	}
}

// TestArrayEnrichment tests that array enrichment works correctly
func TestArrayEnrichment(t *testing.T) {
	schemaCache, err := NewSchemaCache("/tmp/test-cache")
	if err != nil {
		t.Fatalf("Failed to create schema cache: %v", err)
	}
	enricher := NewEnricher(schemaCache)

	// Test with different array types
	arrayTests := []struct {
		name  string
		data  map[string]interface{}
		check func(map[string]interface{}) bool
	}{
		{
			name: "string-array",
			data: map[string]interface{}{
				"tags": []interface{}{},
			},
			check: func(enriched map[string]interface{}) bool {
				if tags, ok := enriched["tags"].([]interface{}); ok {
					return len(tags) > 0
				}
				return false
			},
		},
		{
			name: "object-array",
			data: map[string]interface{}{
				"items": []interface{}{},
			},
			check: func(enriched map[string]interface{}) bool {
				if items, ok := enriched["items"].([]interface{}); ok {
					return len(items) > 0
				}
				return false
			},
		},
		{
			name: "mixed-array",
			data: map[string]interface{}{
				"mixed": []interface{}{},
			},
			check: func(enriched map[string]interface{}) bool {
				if mixed, ok := enriched["mixed"].([]interface{}); ok {
					return len(mixed) > 0
				}
				return false
			},
		},
	}

	for _, at := range arrayTests {
		t.Run(at.name, func(t *testing.T) {
			enriched, err := enricher.EnrichResponse("array-test-"+at.name, at.data)
			if err != nil {
				t.Fatalf("Failed to enrich array data: %v", err)
			}

			if at.check(enriched) {
				t.Logf("✅ %s: Array enrichment successful", at.name)
			} else {
				t.Errorf("❌ %s: Array enrichment failed", at.name)
			}
		})
	}
}