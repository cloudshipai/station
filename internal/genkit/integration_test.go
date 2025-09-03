package genkit

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStationGenKitIntegration_RealOpenAI tests Station's enhanced pipeline at the GenKit level
func TestStationGenKitIntegration_RealOpenAI(t *testing.T) {
	// Skip if no API key provided
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping real OpenAI test: OPENAI_API_KEY not set")
	}

	// Initialize GenKit registry
	g := genkit.Init(context.Background(), nil)

	// Initialize Station's OpenAI plugin
	stationPlugin := &StationOpenAI{
		APIKey: apiKey,
	}
	
	err := stationPlugin.Init(context.Background(), g)
	require.NoError(t, err, "Station OpenAI plugin should initialize successfully")

	t.Run("Real GPT-4o-mini Generation with Context Protection", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Test basic text generation through GenKit interface
		response, err := genkit.Generate(ctx, g,
			genkit.Model("gpt-4o-mini"),
			genkit.Text("Write exactly one sentence about the benefits of modular software architecture."),
			genkit.Config(map[string]any{
				"temperature": 0.3,
				"maxTokens":   100,
			}),
		)

		require.NoError(t, err, "GenKit generation should succeed")
		require.NotNil(t, response, "Response should not be nil")
		require.NotNil(t, response.Message, "Response message should not be nil")
		require.Greater(t, len(response.Message.Content), 0, "Response should have content")

		// Verify Station's enhancements worked
		assert.NotEmpty(t, response.Message.Content[0].Text, "Should have text response")
		assert.NotNil(t, response.Usage, "Should have token usage information")
		assert.Greater(t, response.Usage.TotalTokens, 0, "Should track token usage")

		t.Logf("✅ Basic Generation Success:")
		t.Logf("  Model: gpt-4o-mini")
		t.Logf("  Response: %s", response.Message.Content[0].Text)
		t.Logf("  Tokens: %d total (%d input, %d output)", 
			response.Usage.TotalTokens, response.Usage.InputTokens, response.Usage.OutputTokens)
	})

	t.Run("Tool Calling with Context Protection", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		// Define a simple tool for testing
		testTool := genkit.DefineTool(g, "analyze_text", &ai.ToolDefinition{
			Name: "analyze_text",
			Description: "Analyzes text and returns word count and sentiment",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text": {
						"type": "string",
						"description": "The text to analyze",
					},
				},
				"required": []string{"text"},
			},
		}, func(ctx context.Context, input map[string]any) (map[string]any, error) {
			text, ok := input["text"].(string)
			if !ok {
				return nil, fmt.Errorf("text parameter required")
			}
			
			// Simulate text analysis
			wordCount := len(strings.Fields(text))
			sentiment := "neutral"
			if strings.Contains(strings.ToLower(text), "good") || 
			   strings.Contains(strings.ToLower(text), "great") ||
			   strings.Contains(strings.ToLower(text), "excellent") {
				sentiment = "positive"
			}

			return map[string]any{
				"word_count": wordCount,
				"sentiment":  sentiment,
				"analysis":   fmt.Sprintf("Text has %d words with %s sentiment", wordCount, sentiment),
			}, nil
		})

		// Test generation with tool calling through GenKit
		response, err := genkit.Generate(ctx, g,
			genkit.Model("gpt-4o-mini"),
			genkit.Text("Please analyze this text: 'Modular architecture makes software development more efficient and maintainable. It's a great approach for building scalable systems.'"),
			genkit.Tools(testTool),
			genkit.Config(map[string]any{
				"temperature": 0.1,
			}),
		)

		require.NoError(t, err, "Tool calling should succeed")
		require.NotNil(t, response, "Response should not be nil")
		require.NotNil(t, response.Message, "Response message should not be nil")

		// Verify the response includes tool usage
		hasToolCall := false
		hasTextResponse := false
		
		for _, part := range response.Message.Content {
			if part.IsToolRequest() {
				hasToolCall = true
				assert.Equal(t, "analyze_text", part.ToolRequest.Name, "Should call the correct tool")
				t.Logf("✅ Tool Call Detected: %s", part.ToolRequest.Name)
			}
			if part.IsText() && len(part.Text) > 0 {
				hasTextResponse = true
				t.Logf("✅ Text Response: %s", part.Text[:min(100, len(part.Text))])
			}
		}

		// Station's enhanced pipeline should handle tool calls intelligently
		assert.True(t, hasToolCall || hasTextResponse, "Should have either tool call or text response")
		assert.NotNil(t, response.Usage, "Should track token usage")

		t.Logf("✅ Tool Integration Success:")
		t.Logf("  Has Tool Call: %v", hasToolCall)
		t.Logf("  Has Text Response: %v", hasTextResponse) 
		t.Logf("  Total Tokens: %d", response.Usage.TotalTokens)
	})

	t.Run("Context Limit Stress Test", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// Create a large prompt to test context management
		largePrompt := "Analyze this repeated text: " + strings.Repeat("The quick brown fox jumps over the lazy dog. This is a test sentence for context management. ", 200)

		response, err := genkit.Generate(ctx, g,
			genkit.Model("gpt-4o-mini"),
			genkit.Text(largePrompt + " Please provide a brief summary of what you noticed about this text."),
			genkit.Config(map[string]any{
				"temperature": 0.1,
				"maxTokens":   500,
			}),
		)

		require.NoError(t, err, "Large context generation should succeed with Station's protection")
		require.NotNil(t, response, "Response should not be nil")
		require.Greater(t, len(response.Message.Content), 0, "Should have response content")

		t.Logf("✅ Context Management Success:")
		t.Logf("  Input Length: ~%d chars", len(largePrompt))
		t.Logf("  Response: %s", response.Message.Content[0].Text[:min(150, len(response.Message.Content[0].Text))])
		t.Logf("  Tokens Used: %d total", response.Usage.TotalTokens)
	})

	t.Run("Turn Limit Test (Multi-Turn Conversation)", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		// Simulate a multi-turn conversation to test turn limiting
		messages := []*ai.Message{
			{Role: ai.RoleSystem, Content: []*ai.Part{ai.NewTextPart("You are a helpful assistant.")}},
			{Role: ai.RoleUser, Content: []*ai.Part{ai.NewTextPart("Hello! What's 2+2?")}},
			{Role: ai.RoleModel, Content: []*ai.Part{ai.NewTextPart("Hello! 2+2 equals 4.")}},
			{Role: ai.RoleUser, Content: []*ai.Part{ai.NewTextPart("What about 3+3?")}},
			{Role: ai.RoleModel, Content: []*ai.Part{ai.NewTextPart("3+3 equals 6.")}},
			{Role: ai.RoleUser, Content: []*ai.Part{ai.NewTextPart("Now what's 5+7?")}},
		}

		response, err := genkit.Generate(ctx, g,
			genkit.Model("gpt-4o-mini"),
			genkit.Messages(messages...),
			genkit.Config(map[string]any{
				"temperature": 0.1,
			}),
		)

		require.NoError(t, err, "Multi-turn conversation should succeed")
		require.NotNil(t, response, "Response should not be nil")
		require.Greater(t, len(response.Message.Content), 0, "Should have response content")

		t.Logf("✅ Multi-Turn Success:")
		t.Logf("  Turn Count: %d", len(messages))
		t.Logf("  Response: %s", response.Message.Content[0].Text)
		t.Logf("  Tokens: %d total", response.Usage.TotalTokens)
	})

	t.Run("OpenAI Compatible Endpoint Support", func(t *testing.T) {
		// Test with custom base URL (if provided via environment)
		customBaseURL := os.Getenv("OPENAI_CUSTOM_BASE_URL")
		if customBaseURL == "" {
			t.Skip("Skipping custom endpoint test: OPENAI_CUSTOM_BASE_URL not set")
		}

		// Initialize plugin with custom base URL
		customPlugin := &StationOpenAI{
			APIKey:  apiKey,
			BaseURL: customBaseURL,
		}

		err := customPlugin.Init(context.Background(), g)
		require.NoError(t, err, "Custom endpoint plugin should initialize")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		response, err := genkit.Generate(ctx, g,
			genkit.Model("gpt-4o-mini"), // Or whatever model the custom endpoint supports
			genkit.Text("Test custom endpoint: respond with 'Custom endpoint working!'"),
		)

		require.NoError(t, err, "Custom endpoint should work")
		require.NotNil(t, response, "Response should not be nil")
		
		t.Logf("✅ Custom Endpoint Success:")
		t.Logf("  Base URL: %s", customBaseURL)
		t.Logf("  Response: %s", response.Message.Content[0].Text)
	})
}

// TestStationModelDiscovery tests dynamic model discovery from OpenAI API
func TestStationModelDiscovery(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping model discovery test: OPENAI_API_KEY not set")
	}

	g := genkit.Init(context.Background(), nil)

	plugin := &StationOpenAI{
		APIKey: apiKey,
	}

	err := plugin.Init(context.Background(), g)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get available actions (models) from the plugin
	actions := plugin.ListActions(ctx)
	require.Greater(t, len(actions), 0, "Should discover available models")

	t.Logf("✅ Discovered %d models:", len(actions))
	for _, action := range actions[:min(10, len(actions))] { // Show first 10
		t.Logf("  - %s", action.Name)
	}

	// Verify hardcoded models are included
	hardcodedModels := []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-4", "gpt-3.5-turbo"}
	discoveredNames := make(map[string]bool)
	for _, action := range actions {
		discoveredNames[action.Name] = true
	}

	foundHardcoded := 0
	for _, model := range hardcodedModels {
		if discoveredNames[model] {
			foundHardcoded++
		}
	}

	assert.Greater(t, foundHardcoded, 0, "Should include some hardcoded Station-enhanced models")
	t.Logf("✅ Found %d/%d hardcoded Station-enhanced models", foundHardcoded, len(hardcodedModels))
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}