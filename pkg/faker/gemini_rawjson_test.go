package faker

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/stretchr/testify/require"
)

// TestGeminiRawJSONBug reproduces the exact panic we're seeing in faker
//
// Error: panic: interface conversion: interface {} is bool, not map[string]interface {}
// Location: github.com/firebase/genkit/go/plugins/googlegenai.toGeminiSchema
//
// This happens when using GenerateData with a struct containing json.RawMessage
func TestGeminiRawJSONBug(t *testing.T) {
	if os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("GEMINI_API_KEY not set")
	}

	ctx := context.Background()

	// Initialize GenKit with Gemini (exactly as faker does on line 187)
	plugin := &googlegenai.GoogleAI{}
	app := genkit.Init(ctx, genkit.WithPlugins(plugin))

	// Define the same structs faker uses
	type ToolDefinition struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"inputSchema"` // THIS causes the panic
	}

	type ToolsResponse struct {
		Tools []ToolDefinition `json:"tools"`
	}

	// This call will panic with Gemini
	t.Run("gemini_panics_with_rawjson", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("✅ Caught expected panic: %v", r)
			}
		}()

		_, _, err := genkit.GenerateData[ToolsResponse](ctx, app,
			ai.WithPrompt("Generate 2 tools"),
			ai.WithModelName("googleai/gemini-2.0-flash-exp"))

		if err != nil {
			t.Logf("Error instead of panic: %v", err)
		}
	})
}

// TestOpenAIRawJSONWorks verifies OpenAI handles the same struct fine
func TestOpenAIRawJSONWorks(t *testing.T) {
	t.Skip("OpenAI test - enable manually to verify it works")
	// This would need openai plugin import which we don't have in go.mod
}

// TestGeminiWithStringSchema tests the FIX - using string for InputSchema
func TestGeminiWithStringSchema(t *testing.T) {
	if os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("GEMINI_API_KEY not set")
	}

	ctx := context.Background()

	plugin := &googlegenai.GoogleAI{}
	app := genkit.Init(ctx, genkit.WithPlugins(plugin))

	// THE FIX: Use string instead of json.RawMessage or map
	type ToolDefinitionFixed struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		InputSchema string `json:"inputSchema"` // JSON string - no schema conversion issues!
	}

	type ToolsResponseFixed struct {
		Tools []ToolDefinitionFixed `json:"tools"`
	}

	result, _, err := genkit.GenerateData[ToolsResponseFixed](ctx, app,
		ai.WithPrompt("Generate 2 MCP tools. For each tool, provide a JSON schema as a string for inputSchema field."),
		ai.WithModelName("googleai/gemini-2.0-flash-exp"))

	require.NoError(t, err, "String schema should work with Gemini")
	require.Greater(t, len(result.Tools), 0)

	// Verify we got valid JSON strings
	for _, tool := range result.Tools {
		var schema map[string]interface{}
		err := json.Unmarshal([]byte(tool.InputSchema), &schema)
		require.NoError(t, err, "InputSchema should be valid JSON string")
		t.Logf("Tool %s schema: %s", tool.Name, tool.InputSchema)
	}

	t.Logf("✅ Gemini works with string InputSchema: %d tools generated", len(result.Tools))
}
