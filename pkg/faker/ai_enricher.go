package faker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brianvoe/gofakeit/v6"
	"google.golang.org/genai"
)

// AIEnricherConfig holds configuration for AI-powered enrichment
type AIEnricherConfig struct {
	Model       string
	APIKey      string
	Instruction string // Custom instruction for shaping the output
	Enabled     bool   // Whether to use AI enrichment or fall back to basic
}

// AIEnricher generates contextually appropriate mock data using AI
type AIEnricher struct {
	schemaCache *SchemaCache
	faker       *gofakeit.Faker
	config      *AIEnricherConfig
	client      *genai.Client
}

// NewAIEnricher creates a new AI-powered data enricher
func NewAIEnricher(schemaCache *SchemaCache, config *AIEnricherConfig) (*AIEnricher, error) {
	enricher := &AIEnricher{
		schemaCache: schemaCache,
		faker:       gofakeit.NewCrypto(),
		config:      config,
	}

	if config.Enabled && config.APIKey != "" {
		ctx := context.Background()
		client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: config.APIKey})
		if err != nil {
			return nil, fmt.Errorf("failed to create GenAI client: %w", err)
		}
		enricher.client = client
	}

	return enricher, nil
}

// EnrichResponse enriches a JSON-RPC response with contextually appropriate mock data
func (e *AIEnricher) EnrichResponse(toolName string, response map[string]interface{}) (map[string]interface{}, error) {
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

	// If AI enrichment is enabled and we have a client, use AI
	if e.config.Enabled && e.client != nil {
		enriched, err := e.enrichWithAI(toolName, schema, response)
		if err != nil {
			// Fall back to basic enrichment if AI fails
			return e.enrichWithBasic(schema, response)
		}
		return enriched, nil
	}

	// Fall back to basic enrichment
	return e.enrichWithBasic(schema, response)
}

// enrichWithAI uses GenAI to generate contextually appropriate responses
func (e *AIEnricher) enrichWithAI(toolName string, schema *SchemaNode, response map[string]interface{}) (map[string]interface{}, error) {
	// Create the AI prompt
	prompt := e.createAIPrompt(toolName, schema, response)

	// Generate response with GenAI
	ctx := context.Background()
	contents := genai.Text(prompt)
	resp, err := e.client.Models.GenerateContent(ctx, e.config.Model, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no AI response generated")
	}

	// Parse the AI response
	aiResponse := resp.Candidates[0].Content.Parts[0].Text
	var enriched map[string]interface{}
	
	if err := json.Unmarshal([]byte(aiResponse), &enriched); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return enriched, nil
}

// createAIPrompt creates a prompt for AI generation
func (e *AIEnricher) createAIPrompt(toolName string, schema *SchemaNode, response map[string]interface{}) string {
	// Get schema structure as JSON for the prompt
	schemaJSON, _ := json.MarshalIndent(schema, "", "  ")
	
	// Get sample response if available
	sampleJSON := "{}"
	if len(response) > 0 {
		sampleBytes, _ := json.MarshalIndent(response, "", "  ")
		sampleJSON = string(sampleBytes)
	}

	prompt := fmt.Sprintf(`You are generating mock data for an MCP tool response. 

TOOL: %s

INSTRUCTION: %s

RESPONSE SCHEMA:
%s

SAMPLE RESPONSE:
%s

REQUIREMENTS:
1. Generate contextually appropriate data that matches the tool's purpose
2. Maintain the exact JSON structure from the schema
3. Use realistic, meaningful values (not random words)
4. Follow the data types shown in the schema
5. Return ONLY valid JSON (no explanations or markdown)

Generate a complete mock response:`, toolName, e.config.Instruction, string(schemaJSON), sampleJSON)

	return prompt
}

// enrichWithBasic falls back to basic faker enrichment
func (e *AIEnricher) enrichWithBasic(schema *SchemaNode, response map[string]interface{}) (map[string]interface{}, error) {
	// Use the existing basic enricher logic
	return e.enrichValue(schema, response)
}

// enrichValue recursively enriches a value based on its schema (basic fallback)
func (e *AIEnricher) enrichValue(schema *SchemaNode, value interface{}) (map[string]interface{}, error) {
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
func (e *AIEnricher) enrichObject(schema *SchemaNode, value interface{}) (map[string]interface{}, error) {
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
func (e *AIEnricher) generateBasicValue(fieldName string, schema *SchemaNode, value interface{}) interface{} {
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
		return e.faker.Date().Format("2006-01-15T10:30:00Z")
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
func (e *AIEnricher) EnrichJSONRPC(toolName string, jsonrpcMsg map[string]interface{}) ([]byte, error) {
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

// Close closes the AI client
func (e *AIEnricher) Close() error {
	// GenAI client doesn't have an explicit close method
	return nil
}