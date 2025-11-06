package faker

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRealAIEnrichment tests AI enrichment with real Google API
func TestRealAIEnrichment(t *testing.T) {
	// Skip if no API key
	apiKey := os.Getenv("GOOGLE_GENAI_API_KEY")
	if apiKey == "" {
		t.Skip("No GOOGLE_GENAI_API_KEY found, skipping real AI test")
		return
	}

	schemaCache, err := NewSchemaCache("/tmp/test-ai-real-cache")
	require.NoError(t, err)

	// Test data - GitHub repository response
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

	// Test scenarios
	scenarios := []struct {
		name        string
		instruction string
		description string
	}{
		{
			name:        "GitHub Repository",
			instruction: "Generate realistic GitHub repository data with proper repository names, accurate programming languages, plausible star counts, appropriate issue numbers, and realistic owner information.",
			description: "Standard GitHub repository scenario",
		},
		{
			name:        "High-Activity Repository", 
			instruction: "Generate data for a highly active GitHub repository with many stars, forks, and issues. Use popular programming languages and realistic maintainer information.",
			description: "Popular repository scenario",
		},
		{
			name:        "Enterprise Repository",
			instruction: "Generate enterprise GitHub repository data with corporate naming conventions, business-focused programming languages, professional maintainer profiles, and company information.",
			description: "Enterprise repository scenario",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Create AI enricher
			config := &AIEnricherConfig{
				Model:       "gemini-1.5-flash",
				APIKey:      apiKey,
				Instruction: scenario.instruction,
				Enabled:     true,
			}

			enricher, err := NewAIEnricher(schemaCache, config)
			require.NoError(t, err)
			defer enricher.Close()

			// Enrich response
			enriched, err := enricher.EnrichResponse("github-repo", map[string]interface{}{})
			require.NoError(t, err)

			// Validate structure is preserved
			repo, ok := enriched["repository"].(map[string]interface{})
			require.True(t, ok, "Repository object should be preserved")

			// Check required fields exist
			requiredFields := []string{"id", "name", "fullName", "description", "language", "stars", "forks", "openIssues", "owner"}
			for _, field := range requiredFields {
				_, exists := repo[field]
				require.True(t, exists, "Field '%s' should exist in enriched response", field)
			}

			// Log results for manual inspection
			t.Logf("Scenario: %s", scenario.description)
			t.Logf("Instruction: %s", scenario.instruction)
			
			enrichedJSON, _ := json.MarshalIndent(enriched, "", "  ")
			t.Logf("AI Enriched Response:\n%s", string(enrichedJSON))

			// Validate data quality
			validateGitHubData(t, repo, scenario.instruction)
		})
	}
}

// validateGitHubData validates the quality of AI-generated GitHub data
func validateGitHubData(t *testing.T, repo map[string]interface{}, instruction string) {
	// Check repository name
	if name, ok := repo["name"].(string); ok {
		// Should not be random Latin words
		assertNotLatinSlop(t, name, "Repository name")
		
		// Should follow GitHub naming conventions (kebab-case, no spaces)
		assertValidGitHubName(t, name)
	}

	// Check programming language
	if language, ok := repo["language"].(string); ok {
		validLanguages := []string{"JavaScript", "Python", "Go", "TypeScript", "Java", "C++", "Rust", "Ruby", "PHP", "Swift", "Kotlin"}
		assertContains(t, validLanguages, language, "Programming language should be realistic")
	}

	// Check numeric fields are reasonable
	if stars, ok := repo["stars"].(float64); ok {
		assert.True(t, stars >= 0 && stars <= 1000000, "Stars should be reasonable (0-1M)")
	}

	if forks, ok := repo["forks"].(float64); ok {
		assert.True(t, forks >= 0 && forks <= 500000, "Forks should be reasonable (0-500K)")
	}

	// Check owner information
	if owner, ok := repo["owner"].(map[string]interface{}); ok {
		if login, ok := owner["login"].(string); ok {
			assertNotLatinSlop(t, login, "Owner login")
			assertValidGitHubName(t, login)
		}

		if company, ok := owner["company"].(string); ok {
			// Should be realistic company name or empty
			if company != "" {
				assertNotLatinSlop(t, company, "Owner company")
			}
		}
	}
}

// Helper assertion functions
func assertNotLatinSlop(t *testing.T, value, fieldName string) {
	latinWords := []string{"voluptas", "quas", "rem", "sit", "et", "aut", "dolores", "dolorem", "consequatur"}
	for _, word := range latinWords {
		assert.False(t, containsIgnoreCase(value, word), 
			"%s should not contain Latin slop word '%s': %s", fieldName, word, value)
	}
}

func assertValidGitHubName(t *testing.T, name string) {
	// GitHub names are typically kebab-case, no spaces, no special chars except hyphens
	assert.NotContains(t, name, " ", "GitHub name should not contain spaces")
	assert.NotContains(t, name, "_", "GitHub name should not contain underscores (usually)")
	assert.True(t, len(name) > 0, "GitHub name should not be empty")
}

func assertContains(t *testing.T, slice []string, item, message string) {
	for _, s := range slice {
		if s == item {
			return
		}
	}
	t.Errorf("%s: '%s' not found in %v", message, item, slice)
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}