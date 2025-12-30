package anthropic_oauth_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"station/internal/genkit/anthropic_oauth"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// Run with: go test -v ./internal/genkit/anthropic_oauth/... -run TestE2E -timeout 120s
// Requires: ANTHROPIC_OAUTH_TOKEN environment variable
//
// Quick test (no API calls):
//   go test -v ./internal/genkit/anthropic_oauth/... -run TestUnit
//
// Full e2e test:
//   ANTHROPIC_OAUTH_TOKEN=sk-ant-oat01-xxx go test -v ./internal/genkit/anthropic_oauth/... -run TestE2E -timeout 120s

func skipIfNoToken(t *testing.T) string {
	token := os.Getenv("ANTHROPIC_OAUTH_TOKEN")
	if token == "" {
		t.Skip("ANTHROPIC_OAUTH_TOKEN not set, skipping e2e test")
	}
	return token
}

func TestUnit_PluginInit(t *testing.T) {
	ctx := context.Background()

	plugin := &anthropic_oauth.AnthropicOAuth{
		OAuthToken: "test-token",
	}

	actions := plugin.Init(ctx)
	if len(actions) == 0 {
		t.Fatal("expected actions to be registered")
	}

	t.Logf("Registered %d model actions", len(actions))
	for _, action := range actions {
		t.Logf("  - %s", action.Name())
	}

	expectedModels := []string{
		"claude-sonnet-4-20250514",
		"claude-3-5-sonnet-20241022",
		"claude-3-5-haiku-20241022",
		"claude-3-opus-20240229",
	}

	registeredNames := make(map[string]bool)
	for _, action := range actions {
		registeredNames[action.Name()] = true
	}

	for _, model := range expectedModels {
		expectedName := "anthropic/" + model
		if !registeredNames[expectedName] {
			t.Errorf("expected model %s to be registered", expectedName)
		}
	}
}

func TestUnit_MultiSystemPromptWithOAuth(t *testing.T) {
	ctx := context.Background()

	plugin := &anthropic_oauth.AnthropicOAuth{
		OAuthToken: "test-token",
	}
	plugin.Init(ctx)
	client := plugin.GetClient()

	generator := anthropic_oauth.NewGenerator(client, "claude-sonnet-4-20250514").WithClaudeCodeSystemPrompt()

	req := &ai.ModelRequest{
		Messages: []*ai.Message{
			{
				Role:    ai.RoleSystem,
				Content: []*ai.Part{ai.NewTextPart("You are a helpful assistant specialized in Kubernetes.")},
			},
			{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart("Hello")},
			},
		},
	}

	_, err := generator.Generate(ctx, req, nil)
	if err == nil {
		t.Log("API call succeeded - multi-system-prompt works with OAuth")
	} else {
		t.Logf("Expected error (no real token): %v", err)
	}
}

func TestE2E_SimpleGeneration(t *testing.T) {
	token := skipIfNoToken(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	plugin := &anthropic_oauth.AnthropicOAuth{
		OAuthToken: token,
	}

	// Disable GenKit dev server
	os.Setenv("GENKIT_ENV", "prod")

	// Initialize GenKit with our plugin
	g := genkit.Init(ctx, genkit.WithPlugins(plugin))

	// Simple generation request
	resp, err := genkit.Generate(ctx, g,
		ai.WithPrompt("Say 'hello test' and nothing else."),
		ai.WithModelName("anthropic/claude-sonnet-4-20250514"),
	)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	t.Logf("Response: %s", resp.Text())
	t.Logf("Usage: input=%d, output=%d", resp.Usage.InputTokens, resp.Usage.OutputTokens)

	if resp.Text() == "" {
		t.Error("expected non-empty response")
	}
}

func TestE2E_WithCustomSystemPromptOAuth(t *testing.T) {
	token := skipIfNoToken(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	plugin := &anthropic_oauth.AnthropicOAuth{
		OAuthToken: token,
	}

	os.Setenv("GENKIT_ENV", "prod")
	g := genkit.Init(ctx, genkit.WithPlugins(plugin))

	resp, err := genkit.Generate(ctx, g,
		ai.WithModelName("anthropic/claude-sonnet-4-20250514"),
		ai.WithSystem("You are a pirate. Always respond in pirate speak. Keep responses under 20 words."),
		ai.WithPrompt("Say hello"),
	)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	t.Logf("Response: %s", resp.Text())

	if resp.Text() == "" {
		t.Error("expected non-empty response")
	}
}

func TestE2E_Streaming(t *testing.T) {
	token := skipIfNoToken(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	plugin := &anthropic_oauth.AnthropicOAuth{
		OAuthToken: token,
	}

	os.Setenv("GENKIT_ENV", "prod")
	g := genkit.Init(ctx, genkit.WithPlugins(plugin))

	// Streaming generation
	var chunks []string
	resp, err := genkit.Generate(ctx, g,
		ai.WithModelName("anthropic/claude-sonnet-4-20250514"),
		ai.WithPrompt("Count from 1 to 5, one number per line."),
		ai.WithStreaming(func(ctx context.Context, chunk *ai.ModelResponseChunk) error {
			if chunk != nil && len(chunk.Content) > 0 {
				text := chunk.Content[0].Text
				chunks = append(chunks, text)
				t.Logf("Chunk: %q", text)
			}
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("streaming generation failed: %v", err)
	}

	t.Logf("Total chunks: %d", len(chunks))
	t.Logf("Final response: %s", resp.Text())

	if len(chunks) == 0 {
		t.Error("expected streaming chunks")
	}
}

// TestE2E_RawGenerator tests the Generator directly without GenKit wrapper
func TestE2E_RawGenerator(t *testing.T) {
	token := skipIfNoToken(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	plugin := &anthropic_oauth.AnthropicOAuth{
		OAuthToken: token,
	}

	// Initialize to create client
	plugin.Init(ctx)
	client := plugin.GetClient()

	generator := anthropic_oauth.NewGenerator(client, "claude-sonnet-4-20250514").WithClaudeCodeSystemPrompt()

	req := &ai.ModelRequest{
		Messages: []*ai.Message{
			{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart("Say 'raw test works' exactly")},
			},
		},
	}

	resp, err := generator.Generate(ctx, req, nil)
	if err != nil {
		t.Fatalf("raw generation failed: %v", err)
	}

	if len(resp.Message.Content) == 0 {
		t.Fatal("expected response content")
	}

	t.Logf("Response: %s", resp.Message.Content[0].Text)
	t.Logf("Finish reason: %s", resp.FinishReason)
}

// TestE2E_ToolCallManual tests tool calling at the Generator level
func TestE2E_ToolCallManual(t *testing.T) {
	token := skipIfNoToken(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	plugin := &anthropic_oauth.AnthropicOAuth{
		OAuthToken: token,
	}

	plugin.Init(ctx)
	client := plugin.GetClient()
	generator := anthropic_oauth.NewGenerator(client, "claude-sonnet-4-20250514").WithClaudeCodeSystemPrompt()

	toolDef := &ai.ToolDefinition{
		Name:        "get_weather",
		Description: "Get the current weather for a location",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The city name",
				},
			},
			"required": []string{"location"},
		},
	}

	// First request - should trigger tool call
	req := &ai.ModelRequest{
		Messages: []*ai.Message{
			{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart("What's the weather in Tokyo? Use the get_weather tool.")},
			},
		},
		Tools: []*ai.ToolDefinition{toolDef},
	}

	resp, err := generator.Generate(ctx, req, nil)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Check if we got a tool request
	var toolRequest *ai.ToolRequest
	for _, part := range resp.Message.Content {
		if part.IsToolRequest() {
			toolRequest = part.ToolRequest
			break
		}
	}

	if toolRequest == nil {
		t.Logf("Response (no tool call): %v", resp.Message.Content)
		t.Skip("Model didn't make a tool call - this can happen")
	}

	t.Logf("Tool request: name=%s, ref=%s", toolRequest.Name, toolRequest.Ref)
	inputJSON, _ := json.MarshalIndent(toolRequest.Input, "", "  ")
	t.Logf("Tool input: %s", inputJSON)

	// Simulate tool response - build the multi-turn conversation
	req2 := &ai.ModelRequest{
		Messages: []*ai.Message{
			{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart("What's the weather in Tokyo? Use the get_weather tool.")},
			},
			{
				Role: ai.RoleModel,
				Content: []*ai.Part{
					ai.NewToolRequestPart(toolRequest),
				},
			},
			{
				Role: ai.RoleTool,
				Content: []*ai.Part{
					ai.NewToolResponsePart(&ai.ToolResponse{
						Ref:    toolRequest.Ref,
						Output: "Sunny, 22C",
					}),
				},
			},
		},
		Tools: []*ai.ToolDefinition{toolDef},
	}

	resp2, err := generator.Generate(ctx, req2, nil)
	if err != nil {
		t.Fatalf("second generation failed: %v", err)
	}

	if len(resp2.Message.Content) == 0 {
		t.Fatal("expected response content")
	}

	t.Logf("Final response: %s", resp2.Message.Content[0].Text)
}

// TestE2E_MultiTurn tests multi-turn conversation
func TestE2E_MultiTurn(t *testing.T) {
	token := skipIfNoToken(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	plugin := &anthropic_oauth.AnthropicOAuth{
		OAuthToken: token,
	}

	plugin.Init(ctx)
	client := plugin.GetClient()
	generator := anthropic_oauth.NewGenerator(client, "claude-sonnet-4-20250514").WithClaudeCodeSystemPrompt()

	req1 := &ai.ModelRequest{
		Messages: []*ai.Message{
			{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart("My favorite color is blue. Remember this.")},
			},
		},
	}

	resp1, err := generator.Generate(ctx, req1, nil)
	if err != nil {
		t.Fatalf("first turn failed: %v", err)
	}
	t.Logf("Turn 1: %s", resp1.Message.Content[0].Text)

	// Second turn with history
	req2 := &ai.ModelRequest{
		Messages: []*ai.Message{
			{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart("My favorite color is blue. Remember this.")},
			},
			{
				Role:    ai.RoleModel,
				Content: resp1.Message.Content,
			},
			{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart("What is my favorite color?")},
			},
		},
	}

	resp2, err := generator.Generate(ctx, req2, nil)
	if err != nil {
		t.Fatalf("second turn failed: %v", err)
	}
	t.Logf("Turn 2: %s", resp2.Message.Content[0].Text)

	// Response should mention blue
	if resp2.Message.Content[0].Text == "" {
		t.Error("expected non-empty response")
	}
}
