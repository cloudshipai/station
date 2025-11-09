package faker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"station/internal/config"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go/option"
)

// TestEnrichmentLogic tests JUST the enrichment logic without MCP setup
func TestEnrichmentLogic(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("Skipping: OPENAI_API_KEY not set")
	}

	ctx := context.Background()

	// Load Station config
	stationConfig, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Set up test environment
	if os.Getenv("GENKIT_ENV") == "" {
		os.Setenv("GENKIT_ENV", "prod")
	}
	os.Setenv("OTEL_SDK_DISABLED", "true")

	// Initialize GenKit
	fmt.Println("[TEST] Initializing GenKit for enrichment test")
	plugin := &openai.OpenAI{
		APIKey: stationConfig.AIAPIKey,
		Opts:   []option.RequestOption{},
	}
	app := genkit.Init(ctx, genkit.WithPlugins(plugin))
	fmt.Println("[TEST] GenKit initialized")

	// Create a faker instance (minimal setup)
	f := &MCPFaker{
		targetClient:  nil, // Not needed for this test
		genkitApp:     app,
		stationConfig: stationConfig,
		instruction:   "Generate realistic mock filesystem data with varied file types, sizes, and timestamps",
		debug:         true,
	}

	// Create a simple result to enrich
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent("[DIR] folder1\n[FILE] test.txt"),
		},
		IsError: false,
	}

	fmt.Println("\n[TEST] Starting enrichment...")
	fmt.Println("[TEST] Original content: [DIR] folder1\\n[FILE] test.txt")

	// Test enrichment
	enriched, err := f.enrichToolResult(ctx, "list_directory", result)
	if err != nil {
		t.Fatalf("Enrichment failed: %v", err)
	}

	// Print the result
	enrichedJSON, _ := json.MarshalIndent(enriched.Content, "", "  ")
	fmt.Printf("\n[TEST] === Enrichment Test Results ===\n")
	fmt.Printf("[TEST] Enriched content:\n%s\n", string(enrichedJSON))

	// Verify enrichment occurred
	if len(enriched.Content) == 0 {
		t.Fatal("Expected enriched content to have items")
	}

	// Verify enrichment worked by checking if we got mock data
	// The original had 1 item, enriched should have many more with different content
	if len(enriched.Content) <= 1 {
		t.Fatal("Expected enriched content to have more items than original")
	}

	fmt.Println("\n[TEST] ✓ Enrichment successful! Generated", len(enriched.Content), "mock items vs 1 original item")
}
