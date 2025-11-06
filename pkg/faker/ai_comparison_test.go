package faker

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAIvsBasicEnrichment compares AI vs basic enrichment
func TestAIvsBasicEnrichment(t *testing.T) {
	// Skip if no API key
	if testing.Short() {
		t.Skip("Skipping AI enrichment test in short mode")
	}

	schemaCache, err := NewSchemaCache("/tmp/test-ai-cache")
	require.NoError(t, err)

	// Test data
	originalData := map[string]interface{}{
		"repository": map[string]interface{}{
			"id":           123456789,
			"name":         "awesome-project",
			"fullName":     "octocat/awesome-project",
			"description":  "An awesome project for testing",
			"private":      false,
			"language":     "Go",
			"stars":        42,
			"forks":        15,
			"openIssues":   7,
			"createdAt":    "2024-01-15T10:30:00Z",
			"updatedAt":    "2024-11-01T15:45:00Z",
			"owner": map[string]interface{}{
				"id":         583231,
				"login":      "octocat",
				"name":       "Octo Cat",
				"email":      "octocat@github.com",
				"company":    "GitHub",
				"location":   "San Francisco",
				"blog":       "https://github.blog",
				"followers":   150,
				"following":   75,
			},
		},
	}

	// Learn schema
	err = schemaCache.AnalyzeResponse("github-repo", originalData)
	require.NoError(t, err)

	// Test basic enrichment
	basicEnricher := NewEnricher(schemaCache)
	basicEnriched, err := basicEnricher.EnrichResponse("github-repo", map[string]interface{}{})
	require.NoError(t, err)

	// Show basic enrichment results
	t.Logf("Basic enrichment results:")
	basicJSON, _ := json.MarshalIndent(basicEnriched, "", "  ")
	t.Logf("%s", string(basicJSON))

	// Verify basic enrichment maintains structure
	assert.IsType(t, map[string]interface{}{}, basicEnriched)
	repo, ok := basicEnriched["repository"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, repo, "id")
	assert.Contains(t, repo, "name")
	assert.Contains(t, repo, "owner")

	// Check for "slop" indicators in basic enrichment
	if name, ok := repo["name"].(string); ok {
		// Basic faker often generates random Latin words
		t.Logf("Basic faker name: %s", name)
	}

	if company, ok := repo["owner"].(map[string]interface{})["company"].(string); ok {
		t.Logf("Basic faker company: %s", company)
	}

	// Note: AI enrichment test would require real API key
	// This demonstrates the concept and shows the current "slop" issue
	t.Logf("AI enrichment would require GOOGLE_GENAI_API_KEY environment variable")
}

// TestCustomInstructions demonstrates how custom instructions would work
func TestCustomInstructions(t *testing.T) {
	scenarios := []struct {
		name        string
		instruction string
		expected    string
	}{
		{
			name:        "High Alert Scenario",
			instruction: "Generate alert-heavy monitoring data with high error rates, critical warnings, and urgent status indicators. Use values that would trigger monitoring alerts.",
			expected:    "Would generate high-priority alerts, critical error rates, urgent warnings",
		},
		{
			name:        "Financial Scenario",
			instruction: "Generate realistic financial transaction data with proper monetary amounts, valid account numbers, appropriate timestamps, and plausible merchant information.",
			expected:    "Would generate realistic financial data with proper amounts and account info",
		},
		{
			name:        "Healthcare Scenario",
			instruction: "Generate realistic medical data with proper patient IDs, valid vital signs ranges, appropriate medical terminology, and plausible clinical information.",
			expected:    "Would generate realistic medical data with proper terminology and ranges",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("Scenario: %s", scenario.name)
			t.Logf("Instruction: %s", scenario.instruction)
			t.Logf("Expected: %s", scenario.expected)
		})
	}
}