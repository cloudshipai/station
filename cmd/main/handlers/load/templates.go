package load

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/firebase/genkit/go/genkit"
	oai "github.com/firebase/genkit/go/plugins/compat_oai/openai"

	"station/internal/services"
)

// detectTemplates checks if the configuration has template placeholders using AI analysis
func (h *LoadHandler) detectTemplates(config *LoadMCPConfig) (bool, []string) {
	var missingValues []string
	hasTemplates := false

	// Check if there's a templates section
	if len(config.Templates) > 0 {
		hasTemplates = true
	}

	// Try AI-powered intelligent placeholder detection only if enabled
	if h.placeholderAnalyzer != nil {
		configJSON, err := json.Marshal(config)
		if err == nil {
			ctx := context.Background()
			analyses, err := h.placeholderAnalyzer.AnalyzeConfiguration(ctx, string(configJSON))
			if err == nil && len(analyses) > 0 {
				hasTemplates = true

				// Initialize templates map if needed
				if config.Templates == nil {
					config.Templates = make(map[string]TemplateField)
				}

				// Convert AI analyses to template fields
				for _, analysis := range analyses {
					// Only add if not already defined
					if _, exists := config.Templates[analysis.Placeholder]; !exists {
						config.Templates[analysis.Placeholder] = TemplateField{
							Description: analysis.Description,
							Type:        analysis.Type,
							Required:    analysis.Required,
							Sensitive:   analysis.Sensitive,
							Default:     analysis.Default,
							Help:        analysis.Help,
						}
					}
					missingValues = append(missingValues, analysis.Placeholder)
				}

				// Replace the original placeholders in the configuration with template format
				h.replaceDetectedPlaceholders(config, analyses)

				return hasTemplates, missingValues
			}
		}
	}

	// Fallback to traditional regex-based detection
	templatePattern := regexp.MustCompile(`\{\{([^}]+)\}\}`)

	for _, serverConfig := range config.MCPServers {
		for key, value := range serverConfig.Env {
			matches := templatePattern.FindAllStringSubmatch(value, -1)
			for _, match := range matches {
				if len(match) > 1 {
					placeholder := match[1]
					hasTemplates = true

					// Check if we have a template definition for this placeholder
					if _, exists := config.Templates[placeholder]; exists {
						missingValues = append(missingValues, placeholder)
					} else {
						// Create a basic template for unknown placeholders
						if config.Templates == nil {
							config.Templates = make(map[string]TemplateField)
						}
						config.Templates[placeholder] = TemplateField{
							Description: fmt.Sprintf("Value for %s in %s", placeholder, key),
							Type:        "string",
							Required:    true,
						}
						missingValues = append(missingValues, placeholder)
					}
				}
			}
		}
	}

	return hasTemplates, missingValues
}

// replaceDetectedPlaceholders replaces AI-detected placeholders with template format {{placeholder}}
func (h *LoadHandler) replaceDetectedPlaceholders(config *LoadMCPConfig, analyses []services.PlaceholderAnalysis) {
	for _, analysis := range analyses {
		// Replace the original placeholder pattern with template format
		templatePlaceholder := fmt.Sprintf("{{%s}}", analysis.Placeholder)

		// Search and replace in all server configurations
		for _, serverConfig := range config.MCPServers {
			// Replace in environment variables
			for key, value := range serverConfig.Env {
				if strings.Contains(value, analysis.Original) {
					serverConfig.Env[key] = strings.ReplaceAll(value, analysis.Original, templatePlaceholder)
				}
			}

			// Replace in command arguments
			for i, arg := range serverConfig.Args {
				if strings.Contains(arg, analysis.Original) {
					serverConfig.Args[i] = strings.ReplaceAll(arg, analysis.Original, templatePlaceholder)
				}
			}
		}
	}
}

// initializeAI initializes the AI placeholder analyzer if not already set
func (h *LoadHandler) initializeAI() {
	if h.placeholderAnalyzer != nil {
		return // Already initialized
	}

	// Initialize Genkit and OpenAI plugin
	genkitApp, err := genkit.Init(context.Background(), nil)
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to initialize AI engine: %v\n", err)
		return
	}

	// Initialize OpenAI plugin with API key
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		fmt.Printf("⚠️  Warning: OPENAI_API_KEY not set, AI detection disabled\n")
		return
	}

	openaiPlugin := &oai.OpenAI{APIKey: openaiAPIKey}
	h.placeholderAnalyzer = services.NewPlaceholderAnalyzer(genkitApp, openaiPlugin)

	fmt.Println(getCLIStyles(h.themeManager).Success.Render("🤖 AI placeholder detection enabled"))
}

// processTemplateConfig shows credential forms and processes templates
func (h *LoadHandler) processTemplateConfig(config *LoadMCPConfig, missingValues []string) (*LoadMCPConfig, error) {
	if len(missingValues) == 0 {
		return config, nil
	}

	fmt.Printf("🔑 Configuration requires %d credential(s):\n", len(missingValues))

	// Collect values from user
	values := make(map[string]string)

	for _, placeholder := range missingValues {
		template := config.Templates[placeholder]

		fmt.Printf("\n📝 %s\n", template.Description)
		if template.Help != "" {
			fmt.Printf("💡 %s\n", template.Help)
		}

		var value string
		if template.Default != "" {
			fmt.Printf("Enter value (default: %s): ", template.Default)
		} else if template.Required {
			fmt.Printf("Enter value (required): ")
		} else {
			fmt.Printf("Enter value (optional): ")
		}

		// Read input
		var input string
		if _, err := fmt.Scanln(&input); err != nil && template.Required {
			return nil, fmt.Errorf("input required for %s", placeholder)
		}

		if input == "" && template.Default != "" {
			value = template.Default
		} else if input == "" && template.Required {
			return nil, fmt.Errorf("value required for %s", placeholder)
		} else {
			value = input
		}

		values[placeholder] = value

		if template.Sensitive {
			fmt.Printf("✅ Secured credential for %s\n", placeholder)
		} else {
			fmt.Printf("✅ Set %s = %s\n", placeholder, value)
		}
	}

	// Process templates by replacing placeholders
	processedConfig := *config

	for serverName, serverConfig := range processedConfig.MCPServers {
		for envKey, envValue := range serverConfig.Env {
			processedValue := envValue
			for placeholder, value := range values {
				processedValue = strings.ReplaceAll(processedValue, fmt.Sprintf("{{%s}}", placeholder), value)
			}
			serverConfig.Env[envKey] = processedValue
		}
		processedConfig.MCPServers[serverName] = serverConfig
	}

	return &processedConfig, nil
}
