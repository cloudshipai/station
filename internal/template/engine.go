package template

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"station/pkg/config"
)

// GoTemplateEngine implements the TemplateEngine interface using Go's text/template
type GoTemplateEngine struct {
	funcMap template.FuncMap
}

// NewGoTemplateEngine creates a new Go template engine
func NewGoTemplateEngine() *GoTemplateEngine {
	return &GoTemplateEngine{
		funcMap: template.FuncMap{
			"toJSON":   toJSON,
			"fromJSON": fromJSON,
			"toUpper":  strings.ToUpper,
			"toLower":  strings.ToLower,
			"contains": strings.Contains,
			"split":    strings.Split,
			"join":     strings.Join,
			"default":  defaultValue,
			"required": requiredValue,
			"env":      lookupEnv,
		},
	}
}

// Parse parses a template string and returns a compiled template
func (e *GoTemplateEngine) Parse(ctx context.Context, templateContent string) (*config.ParsedTemplate, error) {
	// Create a new template with custom functions
	tmpl, err := template.New("mcp-config").Funcs(e.funcMap).Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	
	// Extract variables from the template
	variables, err := e.extractVariablesFromTemplate(templateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to extract variables: %w", err)
	}
	
	return &config.ParsedTemplate{
		Name:      "mcp-config",
		Template:  tmpl,
		Variables: variables,
	}, nil
}

// Render renders a parsed template with the given variables
func (e *GoTemplateEngine) Render(ctx context.Context, parsedTemplate *config.ParsedTemplate, variables map[string]interface{}) (string, error) {
	tmpl, ok := parsedTemplate.Template.(*template.Template)
	if !ok {
		return "", fmt.Errorf("invalid template type")
	}
	
	var buf bytes.Buffer
	err := tmpl.Execute(&buf, variables)
	if err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}
	
	return buf.String(), nil
}

// ExtractVariables extracts template variables from template content
func (e *GoTemplateEngine) ExtractVariables(ctx context.Context, templateContent string) ([]config.TemplateVariable, error) {
	return e.extractVariablesFromTemplate(templateContent)
}

// Validate validates template syntax
func (e *GoTemplateEngine) Validate(ctx context.Context, templateContent string) error {
	_, err := template.New("validate").Funcs(e.funcMap).Parse(templateContent)
	if err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}
	
	// Additional validation checks
	if err := e.validateTemplateStructure(templateContent); err != nil {
		return fmt.Errorf("template structure validation failed: %w", err)
	}
	
	return nil
}

// extractVariablesFromTemplate uses regex to find template variables
func (e *GoTemplateEngine) extractVariablesFromTemplate(templateContent string) ([]config.TemplateVariable, error) {
	// Regex to match Go template variables: {{.VarName}} or {{.Path.To.Var}}
	varRegex := regexp.MustCompile(`{{\s*\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*(?:\|\s*[^}]+)?}}`)
	
	// Find all matches
	matches := varRegex.FindAllStringSubmatch(templateContent, -1)
	
	// Use a map to avoid duplicates
	varMap := make(map[string]config.TemplateVariable)
	
	for _, match := range matches {
		if len(match) > 1 {
			varPath := match[1]
			
			// For nested paths like "GitHub.ApiKey", we want the full path
			variable := config.TemplateVariable{
				Name:     varPath,
				Type:     config.VarTypeString, // Default type
				Required: true,                  // Default to required
				Secret:   e.isSecretVariable(varPath),
			}
			
			varMap[varPath] = variable
		}
	}
	
	// Also look for function calls that might indicate variable usage
	e.extractFunctionVariables(templateContent, varMap)
	
	// Convert map to slice
	var variables []config.TemplateVariable
	for _, variable := range varMap {
		variables = append(variables, variable)
	}
	
	return variables, nil
}

// extractFunctionVariables looks for variables used in template functions
func (e *GoTemplateEngine) extractFunctionVariables(templateContent string, varMap map[string]config.TemplateVariable) {
	// Look for required function calls: {{required "VarName" .VarName}}
	requiredRegex := regexp.MustCompile(`{{\s*required\s+"([^"]+)"\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*}}`)
	matches := requiredRegex.FindAllStringSubmatch(templateContent, -1)
	
	for _, match := range matches {
		if len(match) > 2 {
			description := match[1]
			varPath := match[2]
			
			variable := config.TemplateVariable{
				Name:        varPath,
				Description: description,
				Type:        config.VarTypeString,
				Required:    true,
				Secret:      e.isSecretVariable(varPath),
			}
			
			varMap[varPath] = variable
		}
	}
	
	// Look for default function calls: {{default "defaultValue" .VarName}}
	defaultRegex := regexp.MustCompile(`{{\s*default\s+"([^"]+)"\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*}}`)
	matches = defaultRegex.FindAllStringSubmatch(templateContent, -1)
	
	for _, match := range matches {
		if len(match) > 2 {
			defaultVal := match[1]
			varPath := match[2]
			
			if existing, exists := varMap[varPath]; exists {
				existing.Default = defaultVal
				existing.Required = false
				varMap[varPath] = existing
			} else {
				variable := config.TemplateVariable{
					Name:     varPath,
					Type:     config.VarTypeString,
					Required: false,
					Default:  defaultVal,
					Secret:   e.isSecretVariable(varPath),
				}
				varMap[varPath] = variable
			}
		}
	}
}

// isSecretVariable determines if a variable contains secret information
func (e *GoTemplateEngine) isSecretVariable(varName string) bool {
	secretKeywords := []string{
		"token", "key", "secret", "password", "pass", "pwd",
		"auth", "credential", "cert", "private", "api_key",
		"access_key", "secret_key",
	}
	
	lowerVarName := strings.ToLower(varName)
	for _, keyword := range secretKeywords {
		if strings.Contains(lowerVarName, keyword) {
			return true
		}
	}
	
	return false
}

// validateTemplateStructure performs additional validation on template structure
func (e *GoTemplateEngine) validateTemplateStructure(templateContent string) error {
	// Check if it's valid JSON structure (for MCP configs)
	// We'll render with dummy variables to test structure
	dummyVars := e.createDummyVariables(templateContent)
	
	tmpl, err := template.New("validate").Funcs(e.funcMap).Parse(templateContent)
	if err != nil {
		return err
	}
	
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, dummyVars)
	if err != nil {
		return fmt.Errorf("template execution failed with dummy variables: %w", err)
	}
	
	// Try to parse the result as JSON
	var result interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		return fmt.Errorf("rendered template is not valid JSON: %w", err)
	}
	
	return nil
}

// createDummyVariables creates dummy variables for template validation
func (e *GoTemplateEngine) createDummyVariables(templateContent string) map[string]interface{} {
	variables, _ := e.extractVariablesFromTemplate(templateContent)
	
	dummyVars := make(map[string]interface{})
	
	for _, variable := range variables {
		// Create nested structure for dotted variables
		e.setNestedValue(dummyVars, variable.Name, e.getDummyValue(variable))
	}
	
	return dummyVars
}

// setNestedValue sets a value in a nested map structure
func (e *GoTemplateEngine) setNestedValue(m map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := m
	
	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part, set the value
			current[part] = value
		} else {
			// Intermediate part, ensure it's a map
			if _, exists := current[part]; !exists {
				current[part] = make(map[string]interface{})
			}
			
			if next, ok := current[part].(map[string]interface{}); ok {
				current = next
			}
		}
	}
}

// getDummyValue returns a dummy value based on variable type
func (e *GoTemplateEngine) getDummyValue(variable config.TemplateVariable) interface{} {
	if variable.Default != nil {
		return variable.Default
	}
	
	switch variable.Type {
	case config.VarTypeString:
		if variable.Secret {
			return "****"
		}
		return "dummy-value"
	case config.VarTypeNumber:
		return 42
	case config.VarTypeBoolean:
		return true
	case config.VarTypeArray:
		return []string{"dummy-item"}
	case config.VarTypeObject:
		return map[string]interface{}{"key": "value"}
	default:
		return "dummy-value"
	}
}

// Template helper functions

func toJSON(v interface{}) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func fromJSON(s string) (interface{}, error) {
	var result interface{}
	err := json.Unmarshal([]byte(s), &result)
	return result, err
}

func defaultValue(defaultVal, value interface{}) interface{} {
	if value == nil || (reflect.ValueOf(value).Kind() == reflect.String && value.(string) == "") {
		return defaultVal
	}
	return value
}

func requiredValue(name string, value interface{}) (interface{}, error) {
	if value == nil || (reflect.ValueOf(value).Kind() == reflect.String && value.(string) == "") {
		return nil, fmt.Errorf("required variable %s is not set", name)
	}
	return value, nil
}

func lookupEnv(key string) string {
	// This would look up environment variables
	// For now, return empty string
	return ""
}

// Ensure we implement the interface
var _ config.TemplateEngine = (*GoTemplateEngine)(nil)