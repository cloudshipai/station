package faker

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v6"
)

// Enricher generates realistic mock data based on learned schemas
type Enricher struct {
	schemaCache *SchemaCache
	faker       *gofakeit.Faker
}

// NewEnricher creates a new data enricher
func NewEnricher(schemaCache *SchemaCache) *Enricher {
	// Create a new faker instance with a random seed
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	faker := gofakeit.NewCrypto()
	_ = seed // Use deterministic seed if needed for testing

	return &Enricher{
		schemaCache: schemaCache,
		faker:       faker,
	}
}

// EnrichResponse enriches a JSON-RPC response with realistic mock data
func (e *Enricher) EnrichResponse(toolName string, response map[string]interface{}) (map[string]interface{}, error) {
	// Get schema for this tool
	schema, exists := e.schemaCache.GetSchema(toolName)

	// If no cached schema, analyze the response to create one on-the-fly
	if !exists {
		if err := e.schemaCache.AnalyzeResponse(toolName, response); err != nil {
			// If we can't analyze, return original response
			return response, nil
		}

		// Try to get the newly created schema
		schema, exists = e.schemaCache.GetSchema(toolName)
		if !exists {
			// Still no schema (shouldn't happen), return original
			return response, nil
		}
	}

	// Enrich the response based on schema
	enriched, err := e.enrichValue(schema, response)
	if err != nil {
		return response, fmt.Errorf("failed to enrich response: %w", err)
	}

	// Return enriched response as map
	if enrichedMap, ok := enriched.(map[string]interface{}); ok {
		return enrichedMap, nil
	}

	return response, nil
}

// enrichValue recursively enriches a value based on its schema
func (e *Enricher) enrichValue(schema *SchemaNode, value interface{}) (interface{}, error) {
	if schema == nil {
		return value, nil
	}

	switch schema.Type {
	case "string":
		return e.enrichString(schema, value)
	case "number":
		return e.enrichNumber(schema, value)
	case "bool":
		return e.enrichBool(schema, value)
	case "array":
		return e.enrichArray(schema, value)
	case "object":
		return e.enrichObject(schema, value)
	case "null":
		return nil, nil
	default:
		return value, nil
	}
}

// enrichString generates realistic string data based on field name patterns
func (e *Enricher) enrichString(schema *SchemaNode, value interface{}) (interface{}, error) {
	// If value already exists and is a string, use pattern detection
	if strVal, ok := value.(string); ok && strVal != "" {
		return strVal, nil
	}

	// For empty/missing strings, try field name detection first if we have context
	// This is a fallback - ideally we want the field name from the calling context
	if len(schema.Samples) > 0 {
		// Use a sample value as reference for pattern detection
		if sample, ok := schema.Samples[0].(string); ok {
			return e.generateStringBySample(sample), nil
		}
	}

	// Default to generic string
	return e.faker.LoremIpsumWord(), nil
}

// generateStringBySample generates string data based on a sample value
func (e *Enricher) generateStringBySample(sample string) string {
	sampleLower := strings.ToLower(sample)

	// Email patterns
	if strings.Contains(sampleLower, "@") {
		return e.faker.Email()
	}

	// URL patterns - more comprehensive detection
	if strings.HasPrefix(sampleLower, "http://") || strings.HasPrefix(sampleLower, "https://") ||
		strings.Contains(sampleLower, "://") || strings.Contains(sampleLower, ".com") ||
		strings.Contains(sampleLower, ".org") || strings.Contains(sampleLower, ".net") ||
		strings.Contains(sampleLower, ".dev") || strings.Contains(sampleLower, ".app") {
		return e.faker.URL()
	}

	// UUID patterns
	if len(sample) == 36 && strings.Count(sample, "-") == 4 {
		return e.faker.UUID()
	}

	// ISO timestamp patterns
	if strings.Contains(sample, "T") && (strings.Contains(sample, "Z") || strings.Contains(sample, "+")) {
		return e.faker.Date().Format(time.RFC3339)
	}

	// Path patterns
	if strings.HasPrefix(sample, "/") && strings.Count(sample, "/") > 1 {
		return "/" + e.faker.LoremIpsumWord() + "/" + e.faker.LoremIpsumWord()
	}

	// IP address patterns - more specific validation
	parts := strings.Split(sample, ".")
	if len(parts) == 4 {
		allNumeric := true
		for _, part := range parts {
			if len(part) == 0 || len(part) > 3 {
				allNumeric = false
				break
			}
			for _, char := range part {
				if char < '0' || char > '9' {
					allNumeric = false
					break
				}
			}
		}
		if allNumeric {
			return e.faker.IPv4Address()
		}
	}

	// Default to word of similar length
	if len(sample) < 10 {
		return e.faker.LoremIpsumWord()
	}
	return e.faker.LoremIpsumSentence(5)
}

// enrichNumber generates realistic number data
func (e *Enricher) enrichNumber(schema *SchemaNode, value interface{}) (interface{}, error) {
	// If value already exists, keep it
	if value != nil {
		return value, nil
	}

	// Use sample values as reference
	if len(schema.Samples) > 0 {
		if sample, ok := schema.Samples[0].(float64); ok {
			// Generate number in similar range
			if sample < 100 {
				return float64(e.faker.IntRange(0, 100)), nil
			}
			if sample < 1000 {
				return float64(e.faker.IntRange(0, 1000)), nil
			}
			// For larger numbers, use similar order of magnitude
			return float64(e.faker.IntRange(0, 10000)), nil
		}
	}

	// Default to small integer
	return float64(e.faker.IntRange(1, 100)), nil
}

// enrichBool generates realistic boolean data
func (e *Enricher) enrichBool(schema *SchemaNode, value interface{}) (interface{}, error) {
	// If value already exists, keep it
	if value != nil {
		return value, nil
	}

	// Use sample distribution if available
	if len(schema.Samples) > 0 {
		trueCount := 0
		for _, sample := range schema.Samples {
			if boolVal, ok := sample.(bool); ok && boolVal {
				trueCount++
			}
		}
		// Maintain similar true/false ratio
		probability := float64(trueCount) / float64(len(schema.Samples))
		return e.faker.Float64Range(0, 1) < probability, nil
	}

	// Default to random boolean
	return e.faker.Bool(), nil
}

// enrichArray generates realistic array data
func (e *Enricher) enrichArray(schema *SchemaNode, value interface{}) (interface{}, error) {
	// If value already exists as array, enrich its items
	if arrVal, ok := value.([]interface{}); ok {
		enrichedArray := make([]interface{}, len(arrVal))
		for i, item := range arrVal {
			if schema.ItemType != nil {
				enriched, err := e.enrichValue(schema.ItemType, item)
				if err != nil {
					return nil, err
				}
				enrichedArray[i] = enriched
			} else {
				enrichedArray[i] = item
			}
		}
		return enrichedArray, nil
	}

	// Generate new array with random length within bounds
	arrayLen := schema.ArrayMin
	if schema.ArrayMax > schema.ArrayMin {
		arrayLen = e.faker.IntRange(schema.ArrayMin, schema.ArrayMax)
	}

	result := make([]interface{}, arrayLen)
	for i := 0; i < arrayLen; i++ {
		if schema.ItemType != nil {
			enriched, err := e.enrichValue(schema.ItemType, nil)
			if err != nil {
				return nil, err
			}
			result[i] = enriched
		}
	}

	return result, nil
}

// enrichObject generates realistic object data
func (e *Enricher) enrichObject(schema *SchemaNode, value interface{}) (interface{}, error) {
	// Start with existing object or create new one
	var objMap map[string]interface{}
	if existingMap, ok := value.(map[string]interface{}); ok {
		objMap = existingMap
	} else {
		objMap = make(map[string]interface{})
	}

	// Enrich each field based on schema
	for fieldName, fieldSchema := range schema.Children {
		existingValue := objMap[fieldName]

		// Generate smart field value based on field name
		enrichedValue, err := e.enrichFieldByName(fieldName, fieldSchema, existingValue)
		if err != nil {
			return nil, err
		}

		objMap[fieldName] = enrichedValue
	}

	return objMap, nil
}

// generateValueByFieldName generates data based solely on field name patterns
func (e *Enricher) generateValueByFieldName(fieldName string) interface{} {
	fieldLower := strings.ToLower(fieldName)

	switch {
	// Identity fields
	case fieldLower == "id" || strings.HasSuffix(fieldLower, "_id"):
		return e.faker.UUID()
	case fieldLower == "name" || strings.HasSuffix(fieldLower, "_name"):
		return e.faker.Name()
	case fieldLower == "email":
		return e.faker.Email()
	case fieldLower == "username" || fieldLower == "user":
		return e.faker.Username()

	// Temporal fields
	case strings.Contains(fieldLower, "created") || strings.Contains(fieldLower, "updated"):
		return e.faker.Date().Format(time.RFC3339)
	case strings.Contains(fieldLower, "date") || strings.Contains(fieldLower, "time"):
		return e.faker.Date().Format(time.RFC3339)

	// Location fields
	case strings.Contains(fieldLower, "address"):
		return e.faker.Address().Address
	case strings.Contains(fieldLower, "city"):
		return e.faker.Address().City
	case strings.Contains(fieldLower, "country"):
		return e.faker.Address().Country
	case strings.Contains(fieldLower, "zip") || strings.Contains(fieldLower, "postal"):
		return e.faker.Address().Zip

	// Network fields
	case strings.Contains(fieldLower, "ip"):
		return e.faker.IPv4Address()
	case strings.Contains(fieldLower, "url") || strings.Contains(fieldLower, "uri"):
		return e.faker.URL()
	case strings.Contains(fieldLower, "domain"):
		return e.faker.DomainName()
	case strings.Contains(fieldLower, "website"):
		return e.faker.URL()

	// Boolean fields
	case strings.Contains(fieldLower, "is_") || strings.Contains(fieldLower, "has_") || strings.Contains(fieldLower, "can_") || strings.Contains(fieldLower, "should_") || strings.Contains(fieldLower, "are_") || strings.Contains(fieldLower, "will_") || strings.Contains(fieldLower, "would_"):
		return e.faker.Bool()
	case strings.Contains(fieldLower, "enabled") || strings.Contains(fieldLower, "disabled"):
		return e.faker.Bool()
	case strings.Contains(fieldLower, "active") || strings.Contains(fieldLower, "inactive"):
		return e.faker.Bool()

	// Status/Type fields
	case strings.Contains(fieldLower, "status"):
		return e.faker.RandomString([]string{"active", "inactive", "pending", "completed"})
	case strings.Contains(fieldLower, "type"):
		return e.faker.RandomString([]string{"standard", "premium", "basic", "advanced"})

	// Metric fields
	case strings.Contains(fieldLower, "count") || strings.Contains(fieldLower, "total"):
		return float64(e.faker.IntRange(0, 1000))
	case strings.Contains(fieldLower, "price") || strings.Contains(fieldLower, "amount"):
		return e.faker.Price(1.0, 1000.0)
	case strings.Contains(fieldLower, "balance") || strings.Contains(fieldLower, "cost"):
		return e.faker.Price(0.0, 10000.0)
	case strings.Contains(fieldLower, "percent") || strings.Contains(fieldLower, "rate"):
		return e.faker.Float64Range(0, 100)

	// Currency fields
	case strings.Contains(fieldLower, "currency"):
		return e.faker.Currency()

	// Default to string
	default:
		return e.faker.LoremIpsumWord()
	}
}

// generateStringByFieldName generates string data based on field name patterns
func (e *Enricher) generateStringByFieldName(fieldName string) string {
	fieldLower := strings.ToLower(fieldName)

	switch {
	// Network fields
	case strings.Contains(fieldLower, "ip"):
		return e.faker.IPv4Address()
	case strings.Contains(fieldLower, "url") || strings.Contains(fieldLower, "uri") || strings.Contains(fieldLower, "website"):
		return e.faker.URL()
	case strings.Contains(fieldLower, "domain"):
		return e.faker.DomainName()

	// Contact fields
	case fieldLower == "email":
		return e.faker.Email()
	case strings.Contains(fieldLower, "phone"):
		return e.faker.Phone()

	// Identity fields
	case strings.Contains(fieldLower, "name"):
		return e.faker.Name()

	// Default to empty string to let normal enrichment handle it
	default:
		return ""
	}
}

// enrichFieldByName generates field-specific realistic data
func (e *Enricher) enrichFieldByName(fieldName string, schema *SchemaNode, value interface{}) (interface{}, error) {
	// If value exists, enrich it based on schema
	if value != nil {
		return e.enrichValue(schema, value)
	}

	// Special handling for null schema types - infer from field name
	if schema.Type == "null" || schema.Type == "" {
		return e.generateValueByFieldName(fieldName), nil
	}

	// For string fields, try field-specific pattern detection first
	if schema.Type == "string" {
		if fieldValue := e.generateStringByFieldName(fieldName); fieldValue != "" {
			return fieldValue, nil
		}
	}

	// Generate based on common field name patterns
	fieldLower := strings.ToLower(fieldName)

	switch {
	// Identity fields
	case fieldLower == "id" || strings.HasSuffix(fieldLower, "_id"):
		return e.faker.UUID(), nil
	case fieldLower == "name" || strings.HasSuffix(fieldLower, "_name"):
		return e.faker.Name(), nil
	case fieldLower == "email":
		return e.faker.Email(), nil
	case fieldLower == "username" || fieldLower == "user":
		return e.faker.Username(), nil

	// Temporal fields
	case strings.Contains(fieldLower, "created") || strings.Contains(fieldLower, "updated"):
		return e.faker.Date().Format(time.RFC3339), nil
	case strings.Contains(fieldLower, "date") || strings.Contains(fieldLower, "time"):
		return e.faker.Date().Format(time.RFC3339), nil

	// Location fields
	case strings.Contains(fieldLower, "address"):
		return e.faker.Address().Address, nil
	case strings.Contains(fieldLower, "city"):
		return e.faker.Address().City, nil
	case strings.Contains(fieldLower, "country"):
		return e.faker.Address().Country, nil
	case strings.Contains(fieldLower, "zip") || strings.Contains(fieldLower, "postal"):
		return e.faker.Address().Zip, nil

	// Network fields
	case strings.Contains(fieldLower, "ip"):
		return e.faker.IPv4Address(), nil
	case strings.Contains(fieldLower, "url") || strings.Contains(fieldLower, "uri"):
		return e.faker.URL(), nil
	case strings.Contains(fieldLower, "domain"):
		return e.faker.DomainName(), nil

	// Boolean fields
	case strings.Contains(fieldLower, "is_") || strings.Contains(fieldLower, "has_") || strings.Contains(fieldLower, "can_") || strings.Contains(fieldLower, "should_") || strings.Contains(fieldLower, "are_") || strings.Contains(fieldLower, "will_") || strings.Contains(fieldLower, "would_"):
		return e.faker.Bool(), nil
	case strings.Contains(fieldLower, "enabled") || strings.Contains(fieldLower, "disabled"):
		return e.faker.Bool(), nil
	case strings.Contains(fieldLower, "active") || strings.Contains(fieldLower, "inactive"):
		return e.faker.Bool(), nil

	// Status/Type fields
	case strings.Contains(fieldLower, "status"):
		return e.faker.RandomString([]string{"active", "inactive", "pending", "completed"}), nil
	case strings.Contains(fieldLower, "type"):
		return e.faker.RandomString([]string{"standard", "premium", "basic", "advanced"}), nil

	// Metric fields
	case strings.Contains(fieldLower, "count") || strings.Contains(fieldLower, "total"):
		return float64(e.faker.IntRange(0, 1000)), nil
	case strings.Contains(fieldLower, "price") || strings.Contains(fieldLower, "amount"):
		return e.faker.Price(1.0, 1000.0), nil
	case strings.Contains(fieldLower, "balance") || strings.Contains(fieldLower, "cost"):
		return e.faker.Price(0.0, 10000.0), nil
	case strings.Contains(fieldLower, "percent") || strings.Contains(fieldLower, "rate"):
		return e.faker.Float64Range(0, 100), nil

	default:
		// Fall back to schema-based generation
		return e.enrichValue(schema, nil)
	}
}

// EnrichJSONRPC enriches a full JSON-RPC response message
func (e *Enricher) EnrichJSONRPC(toolName string, jsonrpcMsg map[string]interface{}) ([]byte, error) {
	// Only enrich successful responses with "result" field
	result, hasResult := jsonrpcMsg["result"]
	if !hasResult {
		// Return original error response
		return json.Marshal(jsonrpcMsg)
	}

	// Enrich the result
	if resultMap, ok := result.(map[string]interface{}); ok {
		enriched, err := e.EnrichResponse(toolName, resultMap)
		if err != nil {
			return nil, err
		}
		jsonrpcMsg["result"] = enriched
	}

	return json.Marshal(jsonrpcMsg)
}
