package faker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"station/internal/config"
	"station/internal/services"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/firebase/genkit/go/ai"
)

// GenKitAIEnricherConfig holds configuration for GenKit-powered enrichment
type GenKitAIEnricherConfig struct {
	Model       string
	Instruction string // Custom instruction for shaping the output
	Enabled     bool   // Whether to use AI enrichment or fall back to basic
}

// GenKitAIEnricher generates contextually appropriate mock data using Station's GenKitProvider
type GenKitAIEnricher struct {
	schemaCache    *SchemaCache
	faker          *gofakeit.Faker
	config         *GenKitAIEnricherConfig
	genkitProvider *services.GenKitProvider
	stationConfig  *config.Config
}

// NewGenKitAIEnricher creates a new GenKit-powered data enricher
func NewGenKitAIEnricher(schemaCache *SchemaCache, cfg *GenKitAIEnricherConfig) (*GenKitAIEnricher, error) {
	enricher := &GenKitAIEnricher{
		schemaCache: schemaCache,
		faker:       gofakeit.NewCrypto(),
		config:      cfg,
	}

	if cfg.Enabled {
		// Load Station configuration
		stationConfig, err := config.Load()
		if err != nil {
			return nil, fmt.Errorf("failed to load Station config: %w", err)
		}
		enricher.stationConfig = stationConfig

		// Create GenKitProvider to use Station's AI configuration
		enricher.genkitProvider = services.NewGenKitProvider()
	}

	return enricher, nil
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// EnrichResponse enriches a JSON-RPC response with contextually appropriate mock data
func (e *GenKitAIEnricher) EnrichResponse(toolName string, response map[string]interface{}) (map[string]interface{}, error) {
	// Get schema for this tool
	schema, exists := e.schemaCache.GetSchema(toolName)
	if !exists {
		if err := e.schemaCache.AnalyzeResponse(toolName, response); err != nil {
			return response, nil
		}
		schema, exists = e.schemaCache.GetSchema(toolName)
		if !exists {
			return response, nil
		}
	}

	// If AI enrichment is enabled and we have a provider, use AI
	if e.config.Enabled && e.genkitProvider != nil {
		enriched, err := e.enrichWithGenKit(toolName, schema, response)
		if err != nil {
			// Fall back to basic enrichment if AI fails
			fmt.Printf("GenKit AI enrichment failed, falling back to basic: %v\n", err)
			return e.enrichWithBasic(schema, response)
		}
		return enriched, nil
	}

	// Fall back to basic enrichment
	return e.enrichWithBasic(schema, response)
}

// enrichWithGenKit uses GenKit's AI providers to generate contextually appropriate responses
func (e *GenKitAIEnricher) enrichWithGenKit(toolName string, schema *SchemaNode, response map[string]interface{}) (map[string]interface{}, error) {
	ctx := context.Background()

	// Initialize GenKit provider with Station's configuration
	if err := e.genkitProvider.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize GenKit provider: %w", err)
	}

	// Get the GenKit app
	genkitApp, err := e.genkitProvider.GetApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get GenKit app: %w", err)
	}

	// Build the prompt
	instruction := e.config.Instruction
	if instruction == "" {
		instruction = fmt.Sprintf("Generate realistic mock data for %s tool response", toolName)
	}

	// Convert schema to JSON for the prompt
	schemaJSON, err := json.MarshalIndent(e.convertSchemaToSimpleFormat(schema), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	promptText := fmt.Sprintf(`%s

The response must match this schema:
%s

Generate realistic, contextually appropriate mock data that follows the schema structure.
Return ONLY valid JSON matching the schema, with no additional text or explanation.`,
		instruction, string(schemaJSON))

	// Use Station's configured model
	model := e.stationConfig.AIModel
	if e.config.Model != "" {
		model = e.config.Model
	}

	// Use genkit.Generate() which is the supported API
	generateReq := &ai.GenerateRequest{
		Messages: []*ai.Message{
			{
				Content: []*ai.Part{
					ai.NewTextPart(promptText),
				},
				Role: ai.RoleUser,
			},
		},
		Config: &ai.GenerationCommonConfig{
			Temperature:     0.7,
			MaxOutputTokens: 2048,
		},
	}

	// Generate response using genkit.Generate()
	resp, err := ai.Generate(ctx, genkitApp, model, generateReq)
	if err != nil {
		return nil, fmt.Errorf("GenKit generation failed: %w", err)
	}

	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no AI response generated")
	}

	// Extract text from the first candidate
	var aiResponseText string
	for _, part := range resp.Candidates[0].Message.Content {
		if part.IsText() {
			aiResponseText += part.Text
		}
	}

	if aiResponseText == "" {
		return nil, fmt.Errorf("AI response has no text content")
	}

	// Parse the AI response as JSON
	var enriched map[string]interface{}
	// Clean up response (remove markdown code blocks if present)
	aiResponseText = strings.TrimPrefix(aiResponseText, "```json\n")
	aiResponseText = strings.TrimPrefix(aiResponseText, "```\n")
	aiResponseText = strings.TrimSuffix(aiResponseText, "\n```")

	if err := json.Unmarshal([]byte(aiResponseText), &enriched); err != nil {
		return nil, fmt.Errorf("failed to parse AI response as JSON: %w (response: %s)", err, aiResponseText)
	}

	return enriched, nil
}

// convertSchemaToSimpleFormat converts our schema to a simple JSON-serializable format
func (e *GenKitAIEnricher) convertSchemaToSimpleFormat(schema *SchemaNode) map[string]interface{} {
	if schema == nil {
		return map[string]interface{}{"type": "object"}
	}

	result := map[string]interface{}{
		"type": schema.Type,
	}

	// Convert object properties
	if schema.Type == "object" && len(schema.Children) > 0 {
		properties := make(map[string]interface{})
		for fieldName, fieldSchema := range schema.Children {
			properties[fieldName] = e.convertSchemaToSimpleFormat(fieldSchema)
		}
		result["properties"] = properties
	}

	// Handle array types
	if schema.Type == "array" && schema.ItemType != nil {
		result["items"] = e.convertSchemaToSimpleFormat(schema.ItemType)
	}

	// Include sample values for context
	if len(schema.Samples) > 0 {
		result["examples"] = schema.Samples[:minInt(3, len(schema.Samples))]
	}

	return result
}

// enrichWithBasic falls back to basic faker enrichment
func (e *GenKitAIEnricher) enrichWithBasic(schema *SchemaNode, response map[string]interface{}) (map[string]interface{}, error) {
	// Use the existing basic enricher logic
	return e.enrichValue(schema, response)
}

// enrichValue recursively enriches a value based on its schema (basic fallback)
func (e *GenKitAIEnricher) enrichValue(schema *SchemaNode, value interface{}) (map[string]interface{}, error) {
	if schema == nil {
		if resultMap, ok := value.(map[string]interface{}); ok {
			return resultMap, nil
		}
		return map[string]interface{}{}, nil
	}

	switch schema.Type {
	case "object":
		return e.enrichObject(schema, value)
	default:
		if resultMap, ok := value.(map[string]interface{}); ok {
			return resultMap, nil
		}
		return map[string]interface{}{}, nil
	}
}

// enrichObject generates realistic object data (basic fallback)
func (e *GenKitAIEnricher) enrichObject(schema *SchemaNode, value interface{}) (map[string]interface{}, error) {
	var objMap map[string]interface{}
	if existingMap, ok := value.(map[string]interface{}); ok {
		objMap = existingMap
	} else {
		objMap = make(map[string]interface{})
	}

	// Enrich each field based on schema
	for fieldName, fieldSchema := range schema.Children {
		existingValue := objMap[fieldName]
		enrichedValue := e.generateBasicValue(fieldName, fieldSchema, existingValue)
		objMap[fieldName] = enrichedValue
	}

	return objMap, nil
}

// generateBasicValue generates basic faker values (fallback)
func (e *GenKitAIEnricher) generateBasicValue(fieldName string, schema *SchemaNode, value interface{}) interface{} {
	fieldLower := strings.ToLower(fieldName)

	// If value exists, keep it
	if value != nil {
		return value
	}

	// Generate based on field name patterns
	switch {
	case fieldLower == "id" || strings.HasSuffix(fieldLower, "_id"):
		return e.faker.UUID()
	case fieldLower == "email":
		return e.faker.Email()
	case strings.Contains(fieldLower, "created") || strings.Contains(fieldLower, "updated"):
		return e.faker.Date().Format("2006-01-02T15:04:05Z")
	case strings.Contains(fieldLower, "ip"):
		return e.faker.IPv4Address()
	case strings.Contains(fieldLower, "url") || strings.Contains(fieldLower, "website"):
		return e.faker.URL()
	case strings.Contains(fieldLower, "balance") || strings.Contains(fieldLower, "amount"):
		return e.faker.Price(0.0, 10000.0)
	case strings.Contains(fieldLower, "is_") || strings.Contains(fieldLower, "has_"):
		return e.faker.Bool()
	case strings.Contains(fieldLower, "count") || strings.Contains(fieldLower, "total"):
		return e.faker.IntRange(0, 1000)
	case strings.Contains(fieldLower, "name"):
		return e.faker.Name()
	case strings.Contains(fieldLower, "status"):
		return e.faker.RandomString([]string{"active", "inactive", "pending", "completed"})
	default:
		if schema.Type == "string" {
			return e.faker.LoremIpsumWord()
		} else if schema.Type == "number" {
			return float64(e.faker.IntRange(1, 100))
		} else if schema.Type == "bool" {
			return e.faker.Bool()
		}
		return nil
	}
}

// EnrichJSONRPC enriches a full JSON-RPC response message
func (e *GenKitAIEnricher) EnrichJSONRPC(toolName string, jsonrpcMsg map[string]interface{}) ([]byte, error) {
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

// Close closes the GenKit provider
func (e *GenKitAIEnricher) Close() error {
	// GenKit provider doesn't need explicit cleanup
	return nil
}
