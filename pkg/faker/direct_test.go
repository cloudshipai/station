package faker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestDirectEnrichment tests the enrichment function directly
func TestDirectEnrichment(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("Skipping: OPENAI_API_KEY not set")
	}

	// Create a faker with AI enrichment
	instruction := "Generate realistic mock filesystem data with varied file types and sizes"
	f, err := NewMCPFaker("echo", []string{"test"}, map[string]string{}, instruction, true)
	if err != nil {
		t.Fatalf("Failed to create faker: %v", err)
	}

	// Create a simple result to enrich
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent("[DIR] folder1\n[FILE] test.txt"),
		},
		IsError: false,
	}

	// Test enrichment
	ctx := context.Background()
	enriched, err := f.enrichToolResult(ctx, "list_directory", nil, result)
	if err != nil {
		t.Fatalf("Enrichment failed: %v", err)
	}

	// Print the result
	enrichedJSON, _ := json.MarshalIndent(enriched.Content, "", "  ")
	fmt.Printf("\n=== Enrichment Test Results ===\n")
	fmt.Printf("Original: [DIR] folder1\\n[FILE] test.txt\n")
	fmt.Printf("Enriched: %s\n", string(enrichedJSON))

	// The enriched result should be different from the original
	if enriched == result {
		t.Fatal("Expected enriched result to be different from original")
	}

	if len(enriched.Content) == 0 {
		t.Fatal("Expected enriched content to have items")
	}
}
