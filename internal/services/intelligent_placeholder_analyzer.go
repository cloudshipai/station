package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	oai "github.com/firebase/genkit/go/plugins/compat_oai/openai"
)

// PlaceholderAnalyzer uses AI to intelligently detect and analyze configuration placeholders
type PlaceholderAnalyzer struct {
	genkit       *genkit.Genkit
	openaiPlugin *oai.OpenAI
}

// NewPlaceholderAnalyzer creates a new intelligent placeholder analyzer
func NewPlaceholderAnalyzer(genkitApp *genkit.Genkit, openaiPlugin *oai.OpenAI) *PlaceholderAnalyzer {
	return &PlaceholderAnalyzer{
		genkit:       genkitApp,
		openaiPlugin: openaiPlugin,
	}
}

// PlaceholderAnalysis represents the AI's analysis of a detected placeholder
type PlaceholderAnalysis struct {
	Original    string            `json:"original"`
	Placeholder string            `json:"placeholder"`
	Type        string            `json:"type"`        // path, api_key, url, string, password, etc.
	Description string            `json:"description"`
	Required    bool              `json:"required"`
	Sensitive   bool              `json:"sensitive"`
	Default     string            `json:"default,omitempty"`
	Help        string            `json:"help,omitempty"`
	Validation  map[string]string `json:"validation,omitempty"` // regex, format, etc.
}

// AnalyzeConfiguration uses AI to detect and analyze all types of placeholders in a configuration
func (pa *PlaceholderAnalyzer) AnalyzeConfiguration(ctx context.Context, configJSON string) ([]PlaceholderAnalysis, error) {
	// First, detect potential placeholders using multiple patterns
	placeholders := pa.detectAllPlaceholderPatterns(configJSON)
	
	if len(placeholders) == 0 {
		return nil, nil
	}

	// Use AI to analyze each placeholder's context and purpose
	prompt := pa.buildAnalysisPrompt(configJSON, placeholders)
	
	// Get model from OpenAI plugin - use GPT-4o for better analysis
	model := pa.openaiPlugin.Model(pa.genkit, "gpt-4o")
	
	// Use Genkit to analyze the configuration with structured JSON output
	response, err := genkit.Generate(ctx, pa.genkit,
		ai.WithModel(model),
		ai.WithPrompt(prompt),
		ai.WithOutputType([]PlaceholderAnalysis{}),
	)
	
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	// Get the structured output directly from Genkit
	var analyses []PlaceholderAnalysis
	if err := response.Output(&analyses); err != nil {
		return nil, fmt.Errorf("failed to parse AI analysis: %w", err)
	}

	return analyses, nil
}

// detectAllPlaceholderPatterns detects various placeholder patterns
func (pa *PlaceholderAnalyzer) detectAllPlaceholderPatterns(content string) []string {
	var placeholders []string
	patterns := []*regexp.Regexp{
		// Standard mustache templates: {{placeholder}}
		regexp.MustCompile(`\{\{([^}]+)\}\}`),
		// Angle bracket placeholders: <path-to-file>
		regexp.MustCompile(`<([^>]+)>`),
		// Square bracket placeholders: [TOKEN]
		regexp.MustCompile(`\[([A-Z_][A-Z0-9_]*)\]`),
		// ALL CAPS environment variables: YOUR_API_KEY
		regexp.MustCompile(`\b([A-Z][A-Z0-9_]{2,})\b`),
		// Path-like placeholders: /path/to/your/file
		regexp.MustCompile(`(/[a-z]+/to/[a-z]+/[a-z.]+)`),
		// URL placeholders: https://your-domain.com
		regexp.MustCompile(`https?://your-[a-z.-]+`),
		// Generic "your" placeholders: your-database-name
		regexp.MustCompile(`\byour-[a-z-]+\b`),
	}

	seen := make(map[string]bool)
	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				placeholder := strings.TrimSpace(match[1])
				if placeholder != "" && !seen[placeholder] {
					placeholders = append(placeholders, match[0]) // Full match for context
					seen[placeholder] = true
				}
			}
		}
	}

	return placeholders
}

// buildAnalysisPrompt creates a prompt for AI to analyze placeholders
func (pa *PlaceholderAnalyzer) buildAnalysisPrompt(configJSON string, placeholders []string) string {
	return fmt.Sprintf(`You are an expert at analyzing MCP (Model Context Protocol) server configurations. 

Analyze the following configuration and the detected placeholders to provide intelligent form field definitions.

Configuration:
%s

Detected placeholders:
%s

For each placeholder, provide a JSON analysis with:
- original: the exact placeholder string found
- placeholder: a clean variable name (e.g., "SQLITE_DB_PATH")
- type: field type (string, password, path, url, api_key, etc.)
- description: user-friendly description
- required: true/false
- sensitive: true if it should be masked/encrypted
- default: suggested default value if appropriate
- help: helpful text explaining what this field is for
- validation: any validation rules (regex patterns, format requirements)

Consider the context:
- Database paths should be "path" type with file validation
- API keys/tokens should be "password" type and sensitive
- URLs should be "url" type with URL validation
- File paths should offer file picker capabilities
- Provide helpful defaults and examples

Return ONLY a JSON array of PlaceholderAnalysis objects, no other text.`, configJSON, strings.Join(placeholders, "\n"))
}

// ConvertToTemplateFields converts AI analysis to station template format
func (pa *PlaceholderAnalyzer) ConvertToTemplateFields(analyses []PlaceholderAnalysis) map[string]TemplateField {
	templates := make(map[string]TemplateField)
	
	for _, analysis := range analyses {
		templates[analysis.Placeholder] = TemplateField{
			Description: analysis.Description,
			Type:        analysis.Type,
			Required:    analysis.Required,
			Sensitive:   analysis.Sensitive,
			Default:     analysis.Default,
			Help:        analysis.Help,
		}
	}
	
	return templates
}

// TemplateField matches the existing station template structure
type TemplateField struct {
	Description string `json:"description"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Sensitive   bool   `json:"sensitive"`
	Default     string `json:"default,omitempty"`
	Help        string `json:"help,omitempty"`
}