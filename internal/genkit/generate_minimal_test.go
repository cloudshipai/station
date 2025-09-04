package genkit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Ensure tests run without requiring hardcoded API keys
	code := m.Run()
	os.Exit(code)
}

func TestNewMinimalModelGenerator(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	logCalled := false
	logCallback := func(data map[string]interface{}) {
		logCalled = true
	}

	generator := NewMinimalModelGenerator(&client, "gpt-4o", logCallback)

	assert.NotNil(t, generator)
	assert.Equal(t, "gpt-4o", generator.modelName)
	assert.Equal(t, &client, generator.client)
	assert.NotNil(t, generator.logCallback)
	assert.NotNil(t, generator.config)
	assert.Equal(t, "gpt-4o", generator.config.Model)
	assert.Nil(t, generator.err)
	assert.Empty(t, generator.messages)
	assert.Empty(t, generator.tools)

	// Test log callback works
	generator.logCallback(map[string]interface{}{"test": "data"})
	assert.True(t, logCalled)
}

func TestWithMessages(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	messages := []*ai.Message{
		{
			Role:    ai.RoleSystem,
			Content: []*ai.Part{ai.NewTextPart("You are a helpful assistant")},
		},
		{
			Role:    ai.RoleUser,
			Content: []*ai.Part{ai.NewTextPart("Hello, how are you?")},
		},
	}

	result := generator.WithMessages(messages)

	assert.Equal(t, generator, result) // Should return self for chaining
	assert.Len(t, generator.messages, 2)
	assert.Nil(t, generator.err)
}

func TestWithMessagesToolResponse(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	// Create a tool response message with Station's Ref field
	toolResponse := &ai.ToolResponse{
		Ref:    "call_abc123", // This should be used as tool_call_id
		Name:   "test_tool",
		Output: map[string]interface{}{"result": "success"},
	}

	messages := []*ai.Message{
		{
			Role:    ai.RoleTool,
			Content: []*ai.Part{ai.NewToolResponsePart(toolResponse)},
		},
	}

	generator.WithMessages(messages)

	require.Len(t, generator.messages, 1)
	
	// Verify the OpenAI message was created correctly
	msg := generator.messages[0]
	assert.NotNil(t, msg.OfTool)
	
	// CRITICAL TEST: Verify our fix uses the correct tool_call_id
	assert.Equal(t, "call_abc123", msg.OfTool.ToolCallID)
}

func TestWithMessagesToolResponseFallback(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	// Tool response without Ref (should fall back to Name)
	toolResponse := &ai.ToolResponse{
		Name:   "test_tool",
		Output: map[string]interface{}{"result": "success"},
	}

	messages := []*ai.Message{
		{
			Role:    ai.RoleTool,
			Content: []*ai.Part{ai.NewToolResponsePart(toolResponse)},
		},
	}

	generator.WithMessages(messages)

	require.Len(t, generator.messages, 1)
	
	msg := generator.messages[0]
	assert.NotNil(t, msg.OfTool)
	
	// Should fall back to tool name when no Ref
	assert.Equal(t, "test_tool", msg.OfTool.ToolCallID)
}

func TestWithMessagesToolCallIDTruncation(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	// Tool response with very long ID (>40 chars)
	longID := "this_is_a_very_long_tool_call_id_that_exceeds_forty_characters"
	toolResponse := &ai.ToolResponse{
		Ref:    longID,
		Name:   "test_tool",
		Output: map[string]interface{}{"result": "success"},
	}

	messages := []*ai.Message{
		{
			Role:    ai.RoleTool,
			Content: []*ai.Part{ai.NewToolResponsePart(toolResponse)},
		},
	}

	generator.WithMessages(messages)

	require.Len(t, generator.messages, 1)
	
	msg := generator.messages[0]
	assert.NotNil(t, msg.OfTool)
	
	// Should be truncated to 40 characters
	assert.Len(t, msg.OfTool.ToolCallID, 40)
	assert.Equal(t, longID[:40], msg.OfTool.ToolCallID)
}

func TestWithTools(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	tools := []*ai.ToolDefinition{
		{
			Name:        "test_tool",
			Description: "A test tool",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	result := generator.WithTools(tools)

	assert.Equal(t, generator, result)
	assert.Len(t, generator.tools, 1)
	assert.Equal(t, "test_tool", generator.tools[0].Function.Name)
	assert.Equal(t, "A test tool", generator.tools[0].Function.Description.Value)
}

func TestWithConfig(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	config := map[string]interface{}{
		"temperature":    0.7,
		// max_tokens removed - different models have vastly different limits
		// top_p removed - focus on temperature only
	}

	result := generator.WithConfig(config)

	assert.Equal(t, generator, result)
	assert.Nil(t, generator.err)
	// Model name should be preserved
	assert.Equal(t, "gpt-4o", generator.config.Model)
}

func TestWithConfigError(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	// Invalid config that can't be marshaled
	config := map[string]interface{}{
		"invalid": make(chan int), // Channels can't be marshaled to JSON
	}

	result := generator.WithConfig(config)

	assert.Equal(t, generator, result)
	assert.NotNil(t, generator.err)
	assert.Contains(t, generator.err.Error(), "failed to convert config")
}

func TestConcatenateContent(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	parts := []*ai.Part{
		ai.NewTextPart("Hello, "),
		ai.NewTextPart("world!"),
		ai.NewTextPart(" How are you?"),
	}

	result := generator.concatenateContent(parts)

	assert.Equal(t, "Hello, world! How are you?", result)
}

func TestConvertToolCalls(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	// Test tool request with proper Ref
	toolRequest := &ai.ToolRequest{
		Ref:   "call_xyz789",
		Name:  "search_tool",
		Input: map[string]interface{}{"query": "test"},
	}

	parts := []*ai.Part{ai.NewToolRequestPart(toolRequest)}

	toolCalls := generator.convertToolCalls(parts)

	require.Len(t, toolCalls, 1)
	
	tc := toolCalls[0]
	assert.Equal(t, "call_xyz789", tc.ID) // Should use Ref as ID
	assert.Equal(t, "search_tool", tc.Function.Name)
}

func TestConvertToolCallsWithFallback(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	// Tool request without Ref (should fall back to Name)
	toolRequest := &ai.ToolRequest{
		Name:  "search_tool",
		Input: map[string]interface{}{"query": "test"},
	}

	parts := []*ai.Part{ai.NewToolRequestPart(toolRequest)}

	toolCalls := generator.convertToolCalls(parts)

	require.Len(t, toolCalls, 1)
	
	tc := toolCalls[0]
	assert.Equal(t, "search_tool", tc.ID) // Should fall back to Name
	assert.Equal(t, "search_tool", tc.Function.Name)
}

func TestAnyToJSONString(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	data := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}

	result := generator.anyToJSONString(data)

	assert.Contains(t, result, "key1")
	assert.Contains(t, result, "value1")
	assert.Contains(t, result, "42")
	assert.Contains(t, result, "true")
}

func TestJsonStringToMap(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	jsonStr := `{"query": "test", "limit": 10, "enabled": true}`

	result := generator.jsonStringToMap(jsonStr)

	assert.Equal(t, "test", result["query"])
	assert.Equal(t, float64(10), result["limit"]) // JSON numbers become float64
	assert.Equal(t, true, result["enabled"])
}

func TestJsonStringToMapInvalid(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	invalidJSON := `{"invalid": json}`

	result := generator.jsonStringToMap(invalidJSON)

	assert.Empty(t, result) // Should return empty map on parse error
}

func TestGenerateWithoutMessages(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	// Try to generate without any messages
	_, err := generator.Generate(context.Background(), nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no messages provided")
}

func TestGenerateWithError(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	// Set an error in the generator
	generator.err = assert.AnError

	_, err := generator.Generate(context.Background(), nil)

	assert.Equal(t, assert.AnError, err)
}

func TestLogCallbackIntegration(t *testing.T) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	
	var logEntries []map[string]interface{}
	logCallback := func(data map[string]interface{}) {
		logEntries = append(logEntries, data)
	}

	generator := NewMinimalModelGenerator(&client, "gpt-4o", logCallback)

	messages := []*ai.Message{
		{Role: ai.RoleUser, Content: []*ai.Part{ai.NewTextPart("Hello")}},
	}

	generator.WithMessages(messages)

	// This will fail due to invalid API key, but should still trigger logs
	_, err := generator.Generate(context.Background(), nil)

	// Should have error but also should have logged
	assert.Error(t, err)
	assert.NotEmpty(t, logEntries)

	// Find the API call log entry
	var apiCallLog map[string]interface{}
	for _, entry := range logEntries {
		if entry["event"] == "openai_api_call" {
			apiCallLog = entry
			break
		}
	}

	require.NotNil(t, apiCallLog)
	assert.Equal(t, "debug", apiCallLog["level"])
	assert.Equal(t, "gpt-4o", apiCallLog["model"])
	assert.Equal(t, 1, apiCallLog["messages"])
	assert.Equal(t, 0, apiCallLog["tools"])
}

// Integration test that requires real OpenAI API key
func TestGenerateIntegrationWithRealAPI(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test - OPENAI_API_KEY not set")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))
	
	var logEntries []map[string]interface{}
	logCallback := func(data map[string]interface{}) {
		logEntries = append(logEntries, data)
		t.Logf("Log: %+v", data)
	}

	generator := NewMinimalModelGenerator(&client, "gpt-4o-mini", logCallback)

	messages := []*ai.Message{
		{
			Role:    ai.RoleUser,
			Content: []*ai.Part{ai.NewTextPart("Say 'Hello, World!' in exactly those words.")},
		},
	}

	generator = generator.WithMessages(messages)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := generator.Generate(ctx, nil)

	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotNil(t, response.Message)
	assert.NotEmpty(t, response.Message.Content)
	
	// Check that we got a text response
	textPart := response.Message.Content[0]
	assert.True(t, textPart.IsText())
	assert.Contains(t, textPart.Text, "Hello")

	// Verify usage information is present
	assert.NotNil(t, response.Usage)
	assert.Greater(t, response.Usage.InputTokens, 0)
	assert.Greater(t, response.Usage.OutputTokens, 0)
	assert.Equal(t, response.Usage.InputTokens+response.Usage.OutputTokens, response.Usage.TotalTokens)

	// Verify logs were generated
	assert.NotEmpty(t, logEntries)
	
	// Should have both API call and success logs
	var hasAPICall, hasSuccess bool
	for _, entry := range logEntries {
		switch entry["event"] {
		case "openai_api_call":
			hasAPICall = true
		case "openai_api_success":
			hasSuccess = true
		}
	}
	
	assert.True(t, hasAPICall, "Should have logged API call")
	assert.True(t, hasSuccess, "Should have logged API success")

	t.Logf("Response: %s", textPart.Text)
	t.Logf("Token usage: %d input, %d output, %d total", 
		response.Usage.InputTokens, response.Usage.OutputTokens, response.Usage.TotalTokens)
}

// Benchmark test to ensure performance is reasonable
func BenchmarkMessageConversion(b *testing.B) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	messages := []*ai.Message{
		{Role: ai.RoleSystem, Content: []*ai.Part{ai.NewTextPart("You are a helpful assistant")}},
		{Role: ai.RoleUser, Content: []*ai.Part{ai.NewTextPart("Hello, world!")}},
		{Role: ai.RoleModel, Content: []*ai.Part{ai.NewTextPart("Hello! How can I help you today?")}},
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		generator.WithMessages(messages)
	}
}

func BenchmarkToolConversion(b *testing.B) {
	client := openai.NewClient(option.WithAPIKey("test-key"))
	generator := NewMinimalModelGenerator(&client, "gpt-4o", nil)

	tools := []*ai.ToolDefinition{
		{Name: "tool1", Description: "Tool 1", InputSchema: map[string]interface{}{"type": "object"}},
		{Name: "tool2", Description: "Tool 2", InputSchema: map[string]interface{}{"type": "object"}},
		{Name: "tool3", Description: "Tool 3", InputSchema: map[string]interface{}{"type": "object"}},
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		generator.WithTools(tools)
	}
}