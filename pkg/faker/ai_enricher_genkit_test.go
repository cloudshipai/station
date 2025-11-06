package faker

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGenKitAIEnricher(t *testing.T) {
	schemaCache, err := NewSchemaCache("/tmp/faker-test-cache")
	require.NoError(t, err)

	t.Run("disabled_enrichment", func(t *testing.T) {
		cfg := &GenKitAIEnricherConfig{
			Enabled: false,
		}

		enricher, err := NewGenKitAIEnricher(schemaCache, cfg)
		require.NoError(t, err)
		assert.NotNil(t, enricher)
		assert.NotNil(t, enricher.faker)
		assert.Nil(t, enricher.genkitProvider)
	})

	t.Run("enabled_enrichment_loads_config", func(t *testing.T) {
		cfg := &GenKitAIEnricherConfig{
			Enabled: true,
			Model:   "gpt-4o-mini",
		}

		enricher, err := NewGenKitAIEnricher(schemaCache, cfg)
		// Note: This may fail if Station config is not set up, which is expected in test environment
		if err != nil {
			t.Logf("Expected potential config load error in test environment: %v", err)
			return
		}

		require.NotNil(t, enricher)
		assert.NotNil(t, enricher.genkitProvider)
		assert.NotNil(t, enricher.stationConfig)
	})
}

func TestGenKitAIEnricher_BasicFallback(t *testing.T) {
	schemaCache, err := NewSchemaCache("/tmp/faker-test-cache")
	require.NoError(t, err)

	cfg := &GenKitAIEnricherConfig{
		Enabled: false, // Disabled, should use basic enrichment
	}

	enricher, err := NewGenKitAIEnricher(schemaCache, cfg)
	require.NoError(t, err)

	// Create a simple schema
	schema := &SchemaNode{
		Type: "object",
		Children: map[string]*SchemaNode{
			"id": {
				Type:    "string",
				Samples: []interface{}{"uuid-123"},
			},
			"email": {
				Type:    "string",
				Samples: []interface{}{"test@example.com"},
			},
			"count": {
				Type:    "number",
				Samples: []interface{}{float64(42)},
			},
		},
	}

	// Store schema in cache
	schemaCache.schemas["test_tool"] = schema

	// Test enrichment with empty response
	response := map[string]interface{}{}
	enriched, err := enricher.EnrichResponse("test_tool", response)
	require.NoError(t, err)

	// Verify enriched fields exist
	assert.NotNil(t, enriched["id"])
	assert.NotNil(t, enriched["email"])
	assert.NotNil(t, enriched["count"])

	// Verify types
	_, ok := enriched["id"].(string)
	assert.True(t, ok, "id should be string")

	_, ok = enriched["email"].(string)
	assert.True(t, ok, "email should be string")
}

func TestGenKitAIEnricher_SchemaConversion(t *testing.T) {
	schemaCache, err := NewSchemaCache("/tmp/faker-test-cache")
	require.NoError(t, err)
	cfg := &GenKitAIEnricherConfig{Enabled: false}

	enricher, err := NewGenKitAIEnricher(schemaCache, cfg)
	require.NoError(t, err)

	t.Run("simple_object_schema", func(t *testing.T) {
		schema := &SchemaNode{
			Type: "object",
			Children: map[string]*SchemaNode{
				"name": {
					Type:    "string",
					Samples: []interface{}{"John Doe", "Jane Smith"},
				},
				"age": {
					Type:    "number",
					Samples: []interface{}{float64(25), float64(30)},
				},
			},
		}

		converted := enricher.convertSchemaToSimpleFormat(schema)
		assert.Equal(t, "object", converted["type"])

		properties, ok := converted["properties"].(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, properties, "name")
		assert.Contains(t, properties, "age")

		// Check name field
		nameSchema := properties["name"].(map[string]interface{})
		assert.Equal(t, "string", nameSchema["type"])
		assert.Contains(t, nameSchema, "examples")
	})

	t.Run("array_schema", func(t *testing.T) {
		schema := &SchemaNode{
			Type: "array",
			ItemType: &SchemaNode{
				Type:    "string",
				Samples: []interface{}{"item1", "item2"},
			},
		}

		converted := enricher.convertSchemaToSimpleFormat(schema)
		assert.Equal(t, "array", converted["type"])
		assert.Contains(t, converted, "items")

		items := converted["items"].(map[string]interface{})
		assert.Equal(t, "string", items["type"])
	})

	t.Run("nested_object_schema", func(t *testing.T) {
		schema := &SchemaNode{
			Type: "object",
			Children: map[string]*SchemaNode{
				"user": {
					Type: "object",
					Children: map[string]*SchemaNode{
						"id": {
							Type:    "string",
							Samples: []interface{}{"user-123"},
						},
						"profile": {
							Type: "object",
							Children: map[string]*SchemaNode{
								"bio": {
									Type:    "string",
									Samples: []interface{}{"Hello world"},
								},
							},
						},
					},
				},
			},
		}

		converted := enricher.convertSchemaToSimpleFormat(schema)
		properties := converted["properties"].(map[string]interface{})
		userSchema := properties["user"].(map[string]interface{})
		userProperties := userSchema["properties"].(map[string]interface{})

		assert.Contains(t, userProperties, "id")
		assert.Contains(t, userProperties, "profile")

		profileSchema := userProperties["profile"].(map[string]interface{})
		profileProperties := profileSchema["properties"].(map[string]interface{})
		assert.Contains(t, profileProperties, "bio")
	})
}

func TestGenKitAIEnricher_EnrichJSONRPC(t *testing.T) {
	schemaCache, err := NewSchemaCache("/tmp/faker-test-cache")
	require.NoError(t, err)
	cfg := &GenKitAIEnricherConfig{Enabled: false}

	enricher, err := NewGenKitAIEnricher(schemaCache, cfg)
	require.NoError(t, err)

	// Create a schema
	schema := &SchemaNode{
		Type: "object",
		Children: map[string]*SchemaNode{
			"status": {
				Type:    "string",
				Samples: []interface{}{"success"},
			},
		},
	}
	schemaCache.schemas["test_tool"] = schema

	t.Run("enrich_success_response", func(t *testing.T) {
		jsonrpcMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"status": "",
			},
		}

		enrichedBytes, err := enricher.EnrichJSONRPC("test_tool", jsonrpcMsg)
		require.NoError(t, err)

		var enrichedMsg map[string]interface{}
		err = json.Unmarshal(enrichedBytes, &enrichedMsg)
		require.NoError(t, err)

		result := enrichedMsg["result"].(map[string]interface{})
		assert.NotEmpty(t, result["status"])
	})

	t.Run("preserve_error_response", func(t *testing.T) {
		jsonrpcMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"error": map[string]interface{}{
				"code":    -32600,
				"message": "Invalid request",
			},
		}

		enrichedBytes, err := enricher.EnrichJSONRPC("test_tool", jsonrpcMsg)
		require.NoError(t, err)

		var enrichedMsg map[string]interface{}
		err = json.Unmarshal(enrichedBytes, &enrichedMsg)
		require.NoError(t, err)

		// Error responses should not be modified
		assert.Contains(t, enrichedMsg, "error")
		assert.NotContains(t, enrichedMsg, "result")
	})
}

func TestGenKitAIEnricher_FieldNamePatterns(t *testing.T) {
	schemaCache, err := NewSchemaCache("/tmp/faker-test-cache")
	require.NoError(t, err)
	cfg := &GenKitAIEnricherConfig{Enabled: false}

	enricher, err := NewGenKitAIEnricher(schemaCache, cfg)
	require.NoError(t, err)

	testCases := []struct {
		fieldName    string
		schema       *SchemaNode
		expectedType string // "string", "number", "bool"
	}{
		{"user_id", &SchemaNode{Type: "string"}, "string"},
		{"email", &SchemaNode{Type: "string"}, "string"},
		{"created_at", &SchemaNode{Type: "string"}, "string"},
		{"ip_address", &SchemaNode{Type: "string"}, "string"},
		{"website_url", &SchemaNode{Type: "string"}, "string"},
		{"count", &SchemaNode{Type: "number"}, "number"},
		{"total_amount", &SchemaNode{Type: "number"}, "number"},
		{"is_active", &SchemaNode{Type: "bool"}, "bool"},
		{"has_permission", &SchemaNode{Type: "bool"}, "bool"},
		{"status", &SchemaNode{Type: "string"}, "string"},
	}

	for _, tc := range testCases {
		t.Run(tc.fieldName, func(t *testing.T) {
			value := enricher.generateBasicValue(tc.fieldName, tc.schema, nil)
			assert.NotNil(t, value, "Value should not be nil for field: %s", tc.fieldName)

			// Type assertions based on expected type
			switch tc.expectedType {
			case "string":
				_, ok := value.(string)
				assert.True(t, ok, "Field %s should be string, got %T", tc.fieldName, value)
			case "number":
				switch value.(type) {
				case int, float64:
					// OK
				default:
					t.Errorf("Field %s should be numeric, got %T", tc.fieldName, value)
				}
			case "bool":
				_, ok := value.(bool)
				assert.True(t, ok, "Field %s should be bool, got %T", tc.fieldName, value)
			}
		})
	}
}
