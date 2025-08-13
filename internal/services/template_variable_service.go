package services

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
	"station/internal/db/repositories"
)

// TemplateVariableService handles variable detection, resolution, and management
type TemplateVariableService struct {
	configDir string
	repos     *repositories.Repositories
}

// VariableInfo represents a detected variable in a template
type VariableInfo struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`  
	Description string `json:"description"`
	Default     string `json:"default"`
	Secret      bool   `json:"secret"`
}

// VariableResolutionResult contains the result of variable resolution
type VariableResolutionResult struct {
	AllResolved      bool                       `json:"all_resolved"`
	ResolvedVars     map[string]string         `json:"resolved_vars"`
	MissingVars      []VariableInfo            `json:"missing_vars"`
	RenderedContent  string                    `json:"rendered_content"`
}

func NewTemplateVariableService(configDir string, repos *repositories.Repositories) *TemplateVariableService {
	return &TemplateVariableService{
		configDir: configDir,
		repos:     repos,
	}
}

// ProcessTemplateWithVariables handles the complete variable workflow for a template
func (tvs *TemplateVariableService) ProcessTemplateWithVariables(envID int64, configName, templateContent string, interactive bool) (*VariableResolutionResult, error) {
	log.Printf("Processing template %s for variable resolution", configName)
	
	// 1. Get environment name
	envName, err := tvs.getEnvironmentName(envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment name: %w", err)
	}
	
	// 2. Load existing variables from environment's variables.yml
	existingVars, err := tvs.loadEnvironmentVariables(envName)
	if err != nil {
		log.Printf("No variables.yml found for environment %s: %v", envName, err)
		existingVars = make(map[string]string)
	}
	
	log.Printf("Loaded %d existing variables from environment %s", len(existingVars), envName)
	
	// 3. Apply environment variable overrides
	for key := range existingVars {
		if envValue := os.Getenv(key); envValue != "" {
			existingVars[key] = envValue
			log.Printf("Override variable %s with environment value", key)
		}
	}
	
	// 4. Try to render template with available variables
	renderedContent, err := tvs.renderTemplate(templateContent, existingVars)
	if err != nil {
		log.Printf("Template rendering failed for %s: %v", configName, err)
		
		if interactive {
			// In interactive mode, we can try to extract missing variable from error and prompt
			missingVar := tvs.extractMissingVariableFromError(err)
			if missingVar != "" {
				log.Printf("Extracted missing variable '%s' from template error", missingVar)
				
				// Prompt for the missing variable
				newVars, err := tvs.promptForMissingVariables([]VariableInfo{{
					Name:     missingVar,
					Required: true,
					Secret:   tvs.isSecretVariable(missingVar),
				}})
				if err != nil {
					return nil, fmt.Errorf("failed to collect missing variable: %w", err)
				}
				
				// Merge new variable and try rendering again
				for k, v := range newVars {
					existingVars[k] = v
				}
				
				// Save new variable to environment file
				if err := tvs.saveVariablesToEnvironment(envName, newVars); err != nil {
					log.Printf("Warning: failed to save variables to environment file: %v", err)
				}
				
				// Try rendering again
				renderedContent, err = tvs.renderTemplate(templateContent, existingVars)
				if err != nil {
					// Still failing - return error
					return &VariableResolutionResult{
						AllResolved:     false,
						ResolvedVars:    existingVars,
						MissingVars:     []VariableInfo{{Name: missingVar, Required: true}},
						RenderedContent: "",
					}, fmt.Errorf("template rendering failed even after providing variable: %w", err)
				}
			} else {
				// Couldn't extract variable name from error
				return &VariableResolutionResult{
					AllResolved:     false,
					ResolvedVars:    existingVars,
					MissingVars:     []VariableInfo{},
					RenderedContent: "",
				}, fmt.Errorf("template rendering failed and couldn't determine missing variable: %w", err)
			}
		} else {
			// Non-interactive mode - return failure
			return &VariableResolutionResult{
				AllResolved:     false,
				ResolvedVars:    existingVars,
				MissingVars:     []VariableInfo{},
				RenderedContent: "",
			}, fmt.Errorf("template rendering failed: %w", err)
		}
	}
	
	// Check for "<no value>" in rendered content - indicates optional missing variables
	hasNoValue := strings.Contains(renderedContent, "<no value>")
	
	result := &VariableResolutionResult{
		AllResolved:     !hasNoValue, // If no "<no value>", all variables resolved
		ResolvedVars:    existingVars,
		MissingVars:     []VariableInfo{}, // We don't track individual missing vars anymore
		RenderedContent: renderedContent,
	}
	
	if hasNoValue {
		log.Printf("Template rendering completed for %s with some '<no value>' placeholders", configName)
	} else {
		log.Printf("Template rendering completed for %s: all variables resolved", configName)
	}
	
	return result, nil
}

// Note: DetectVariables function removed as part of the fix to eliminate regex-based detection.
// The new approach directly attempts template rendering and handles missing variables through
// Go template engine errors, making upfront variable detection unnecessary.

// renderTemplate applies variables to template content using Go's text/template library
func (tvs *TemplateVariableService) renderTemplate(templateContent string, variables map[string]string) (string, error) {
	// Create a new template with the content
	tmpl, err := template.New("mcp-config").Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Convert variables to interface{} map for template execution
	templateData := make(map[string]interface{})
	for key, value := range variables {
		templateData[key] = value
	}

	// Execute the template with the variables
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, templateData); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	renderedContent := rendered.String()
	log.Printf("Template rendering completed for %d variables", len(variables))
	
	return renderedContent, nil
}

// loadEnvironmentVariables loads variables from environment's variables.yml
func (tvs *TemplateVariableService) loadEnvironmentVariables(envName string) (map[string]string, error) {
	variablesPath := filepath.Join(tvs.configDir, "environments", envName, "variables.yml")
	
	data, err := os.ReadFile(variablesPath)
	if err != nil {
		return nil, err
	}
	
	var variables map[string]interface{}
	if err := yaml.Unmarshal(data, &variables); err != nil {
		return nil, err
	}
	
	// Convert to string map
	stringVars := make(map[string]string)
	for key, value := range variables {
		stringVars[key] = fmt.Sprintf("%v", value)
	}
	
	return stringVars, nil
}

// saveVariablesToEnvironment saves new variables to environment's variables.yml
func (tvs *TemplateVariableService) saveVariablesToEnvironment(envName string, newVars map[string]string) error {
	variablesPath := filepath.Join(tvs.configDir, "environments", envName, "variables.yml")
	
	// Load existing variables
	existingVars := make(map[string]interface{})
	if data, err := os.ReadFile(variablesPath); err == nil {
		yaml.Unmarshal(data, &existingVars)
	}
	
	// Merge new variables
	for key, value := range newVars {
		existingVars[key] = value
	}
	
	// Save updated variables
	data, err := yaml.Marshal(existingVars)
	if err != nil {
		return fmt.Errorf("failed to marshal variables: %w", err)
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(variablesPath), 0755); err != nil {
		return fmt.Errorf("failed to create variables directory: %w", err)
	}
	
	if err := os.WriteFile(variablesPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write variables file: %w", err)
	}
	
	log.Printf("Saved %d variables to %s", len(newVars), variablesPath)
	return nil
}

// promptForMissingVariables interactively collects missing variables from user
func (tvs *TemplateVariableService) promptForMissingVariables(missingVars []VariableInfo) (map[string]string, error) {
	result := make(map[string]string)
	
	fmt.Printf("\n🔧 Missing Variables Detected\n")
	fmt.Printf("The following variables need to be configured:\n\n")
	
	for _, variable := range missingVars {
		var prompt string
		if variable.Secret {
			prompt = fmt.Sprintf("🔐 %s (secret): ", variable.Name)
		} else {
			prompt = fmt.Sprintf("📝 %s: ", variable.Name)
		}
		
		if variable.Description != "" {
			fmt.Printf("   Description: %s\n", variable.Description)
		}
		
		fmt.Print(prompt)
		
		var value string
		if _, err := fmt.Scanln(&value); err != nil {
			return nil, fmt.Errorf("failed to read variable %s: %w", variable.Name, err)
		}
		
		if value == "" && variable.Default != "" {
			value = variable.Default
		}
		
		if value == "" && variable.Required {
			return nil, fmt.Errorf("variable %s is required but no value provided", variable.Name)
		}
		
		result[variable.Name] = value
		fmt.Printf("   ✅ Set %s\n\n", variable.Name)
	}
	
	return result, nil
}

// Helper methods

func (tvs *TemplateVariableService) isSecretVariable(name string) bool {
	secretKeywords := []string{"TOKEN", "KEY", "SECRET", "PASSWORD", "CREDENTIAL", "AUTH"}
	upperName := strings.ToUpper(name)
	for _, keyword := range secretKeywords {
		if strings.Contains(upperName, keyword) {
			return true
		}
	}
	return false
}

func (tvs *TemplateVariableService) getVariableNames(variables []VariableInfo) []string {
	names := make([]string, len(variables))
	for i, v := range variables {
		names[i] = v.Name
	}
	return names
}

func (tvs *TemplateVariableService) getEnvironmentName(envID int64) (string, error) {
	env, err := tvs.repos.Environments.GetByID(envID)
	if err != nil {
		return "", err
	}
	return env.Name, nil
}

// extractMissingVariableFromError extracts variable name from Go template error messages
func (tvs *TemplateVariableService) extractMissingVariableFromError(err error) string {
	errorStr := err.Error()
	
	// Go template errors for missing variables look like:
	// "template: detect:1:23: executing \"detect\" at <.MISSING_VAR>: map has no entry for key \"MISSING_VAR\""
	if strings.Contains(errorStr, "map has no entry for key") {
		start := strings.Index(errorStr, "map has no entry for key \"")
		if start != -1 {
			start += len("map has no entry for key \"")
			end := strings.Index(errorStr[start:], "\"")
			if end != -1 {
				return errorStr[start : start+end]
			}
		}
	}
	
	// Alternative pattern: executing "template" at <.VAR_NAME>:
	if strings.Contains(errorStr, "executing") && strings.Contains(errorStr, "at <.") {
		start := strings.Index(errorStr, "at <.")
		if start != -1 {
			start += len("at <.")
			end := strings.Index(errorStr[start:], ">:")
			if end != -1 {
				return errorStr[start : start+end]
			}
		}
	}
	
	return ""
}