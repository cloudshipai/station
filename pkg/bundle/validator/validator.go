package validator

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"

	"station/pkg/bundle"
)

// Validator implements the BundleValidator interface
type Validator struct{}

// NewValidator creates a new bundle validator
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates a bundle and returns any issues
func (v *Validator) Validate(fs afero.Fs, bundlePath string) (*bundle.ValidationResult, error) {
	result := &bundle.ValidationResult{
		Valid:    true,
		Issues:   []bundle.ValidationIssue{},
		Warnings: []bundle.ValidationIssue{},
	}

	// Check required files
	if err := v.validateRequiredFiles(fs, bundlePath, result); err != nil {
		return nil, fmt.Errorf("failed to validate required files: %w", err)
	}

	// Validate manifest.json
	manifest, err := v.validateManifest(fs, bundlePath, result)
	if err != nil {
		return nil, fmt.Errorf("failed to validate manifest: %w", err)
	}

	// Validate template.json
	if err := v.validateTemplate(fs, bundlePath, result); err != nil {
		return nil, fmt.Errorf("failed to validate template: %w", err)
	}

	// Validate variables.schema.json
	if err := v.validateVariablesSchema(fs, bundlePath, result); err != nil {
		return nil, fmt.Errorf("failed to validate variables schema: %w", err)
	}

	// Cross-validation between files
	if manifest != nil {
		if err := v.validateConsistency(fs, bundlePath, manifest, result); err != nil {
			return nil, fmt.Errorf("failed to validate consistency: %w", err)
		}
	}

	// Check for common issues
	v.checkCommonIssues(fs, bundlePath, result)

	// Set overall validity
	result.Valid = len(result.Issues) == 0

	return result, nil
}

func (v *Validator) validateRequiredFiles(fs afero.Fs, bundlePath string, result *bundle.ValidationResult) error {
	requiredFiles := []struct {
		filename    string
		description string
		required    bool
	}{
		{"manifest.json", "Bundle metadata", true},
		{"template.json", "MCP template configuration", true},
		{"variables.schema.json", "Variables schema", true},
		{"README.md", "Documentation", false},
	}

	for _, file := range requiredFiles {
		filePath := filepath.Join(bundlePath, file.filename)
		exists, err := afero.Exists(fs, filePath)
		if err != nil {
			return fmt.Errorf("failed to check file %s: %w", file.filename, err)
		}

		if !exists {
			issue := bundle.ValidationIssue{
				Type:    "missing_file",
				File:    file.filename,
				Message: fmt.Sprintf("%s file is missing", file.description),
			}

			if file.required {
				issue.Suggestion = fmt.Sprintf("Create %s file with proper structure", file.filename)
				result.Issues = append(result.Issues, issue)
			} else {
				issue.Suggestion = fmt.Sprintf("Consider adding %s file for better user experience", file.filename)
				result.Warnings = append(result.Warnings, issue)
			}
		}
	}

	return nil
}

func (v *Validator) validateManifest(fs afero.Fs, bundlePath string, result *bundle.ValidationResult) (*bundle.BundleManifest, error) {
	manifestPath := filepath.Join(bundlePath, "manifest.json")

	// Check if file exists (should be caught by validateRequiredFiles)
	exists, err := afero.Exists(fs, manifestPath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil // Already handled by validateRequiredFiles
	}

	// Read file
	data, err := afero.ReadFile(fs, manifestPath)
	if err != nil {
		result.Issues = append(result.Issues, bundle.ValidationIssue{
			Type:       "file_read_error",
			File:       "manifest.json",
			Message:    "Cannot read manifest file",
			Suggestion: "Check file permissions and content",
		})
		return nil, nil
	}

	// Parse JSON
	var manifest bundle.BundleManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		result.Issues = append(result.Issues, bundle.ValidationIssue{
			Type:       "invalid_json",
			File:       "manifest.json",
			Message:    fmt.Sprintf("Invalid JSON format: %v", err),
			Suggestion: "Fix JSON syntax errors",
		})
		return nil, nil
	}

	// Validate required fields
	v.validateManifestFields(&manifest, result)

	return &manifest, nil
}

func (v *Validator) validateManifestFields(manifest *bundle.BundleManifest, result *bundle.ValidationResult) {
	requiredFields := map[string]string{
		"name":            manifest.Name,
		"version":         manifest.Version,
		"description":     manifest.Description,
		"author":          manifest.Author,
		"station_version": manifest.StationVersion,
	}

	for fieldName, fieldValue := range requiredFields {
		if strings.TrimSpace(fieldValue) == "" {
			result.Issues = append(result.Issues, bundle.ValidationIssue{
				Type:       "missing_required_field",
				File:       "manifest.json",
				Field:      fieldName,
				Message:    fmt.Sprintf("Required field '%s' is missing or empty", fieldName),
				Suggestion: fmt.Sprintf("Add a valid value for '%s' field", fieldName),
			})
		}
	}

	// Validate version format (basic semver check)
	if manifest.Version != "" && !v.isValidSemver(manifest.Version) {
		result.Warnings = append(result.Warnings, bundle.ValidationIssue{
			Type:       "invalid_version_format",
			File:       "manifest.json",
			Field:      "version",
			Message:    "Version should follow semantic versioning (e.g., 1.0.0)",
			Suggestion: "Use semantic versioning format: MAJOR.MINOR.PATCH",
		})
	}

	// Validate station_version format
	if manifest.StationVersion != "" && !strings.Contains(manifest.StationVersion, ">=") {
		result.Warnings = append(result.Warnings, bundle.ValidationIssue{
			Type:       "invalid_station_version",
			File:       "manifest.json",
			Field:      "station_version",
			Message:    "Station version should specify minimum required version",
			Suggestion: "Use format like '>=0.1.0' to specify minimum Station version",
		})
	}
}

func (v *Validator) validateTemplate(fs afero.Fs, bundlePath string, result *bundle.ValidationResult) error {
	templatePath := filepath.Join(bundlePath, "template.json")

	exists, err := afero.Exists(fs, templatePath)
	if err != nil {
		return err
	}
	if !exists {
		return nil // Already handled by validateRequiredFiles
	}

	data, err := afero.ReadFile(fs, templatePath)
	if err != nil {
		result.Issues = append(result.Issues, bundle.ValidationIssue{
			Type:       "file_read_error",
			File:       "template.json",
			Message:    "Cannot read template file",
			Suggestion: "Check file permissions and content",
		})
		return nil
	}

	// Parse JSON
	var template map[string]interface{}
	if err := json.Unmarshal(data, &template); err != nil {
		result.Issues = append(result.Issues, bundle.ValidationIssue{
			Type:       "invalid_json",
			File:       "template.json",
			Message:    fmt.Sprintf("Invalid JSON format: %v", err),
			Suggestion: "Fix JSON syntax errors in template file",
		})
		return nil
	}

	// Check for MCP servers configuration
	if _, hasServers := template["mcpServers"]; !hasServers {
		if _, hasServersAlt := template["servers"]; !hasServersAlt {
			result.Issues = append(result.Issues, bundle.ValidationIssue{
				Type:       "missing_mcp_servers",
				File:       "template.json",
				Message:    "Template must contain 'mcpServers' or 'servers' configuration",
				Suggestion: "Add MCP server configuration to the template",
			})
		}
	}

	return nil
}

func (v *Validator) validateVariablesSchema(fs afero.Fs, bundlePath string, result *bundle.ValidationResult) error {
	schemaPath := filepath.Join(bundlePath, "variables.schema.json")

	exists, err := afero.Exists(fs, schemaPath)
	if err != nil {
		return err
	}
	if !exists {
		return nil // Already handled by validateRequiredFiles
	}

	data, err := afero.ReadFile(fs, schemaPath)
	if err != nil {
		result.Issues = append(result.Issues, bundle.ValidationIssue{
			Type:       "file_read_error",
			File:       "variables.schema.json",
			Message:    "Cannot read variables schema file",
			Suggestion: "Check file permissions and content",
		})
		return nil
	}

	// Parse JSON
	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		result.Issues = append(result.Issues, bundle.ValidationIssue{
			Type:       "invalid_json",
			File:       "variables.schema.json",
			Message:    fmt.Sprintf("Invalid JSON format: %v", err),
			Suggestion: "Fix JSON syntax errors in schema file",
		})
		return nil
	}

	// Basic schema validation
	if schemaType, ok := schema["type"].(string); !ok || schemaType != "object" {
		result.Issues = append(result.Issues, bundle.ValidationIssue{
			Type:       "invalid_schema",
			File:       "variables.schema.json",
			Field:      "type",
			Message:    "Schema type must be 'object'",
			Suggestion: "Set schema type to 'object' for variable definitions",
		})
	}

	if _, hasProperties := schema["properties"]; !hasProperties {
		result.Warnings = append(result.Warnings, bundle.ValidationIssue{
			Type:       "empty_schema",
			File:       "variables.schema.json",
			Message:    "Schema has no properties defined",
			Suggestion: "Add variable properties to the schema",
		})
	}

	return nil
}

func (v *Validator) validateConsistency(fs afero.Fs, bundlePath string, manifest *bundle.BundleManifest, result *bundle.ValidationResult) error {
	// Check if template references variables that are defined in schema
	templatePath := filepath.Join(bundlePath, "template.json")
	schemaPath := filepath.Join(bundlePath, "variables.schema.json")

	templateData, err := afero.ReadFile(fs, templatePath)
	if err != nil {
		return nil // File validation errors already caught
	}

	schemaData, err := afero.ReadFile(fs, schemaPath)
	if err != nil {
		return nil // File validation errors already caught
	}

	var template map[string]interface{}
	var schema map[string]interface{}

	_ = json.Unmarshal(templateData, &template)
	_ = json.Unmarshal(schemaData, &schema)

	// Extract variables from template (look for {{VAR}} patterns)
	templateStr := string(templateData)
	templateVars := v.extractTemplateVariables(templateStr)

	// Extract variables from schema
	schemaVars := make(map[string]bool)
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for varName := range properties {
			schemaVars[varName] = true
		}
	}

	// Check for template variables not defined in schema
	for _, templateVar := range templateVars {
		if !schemaVars[templateVar] {
			result.Warnings = append(result.Warnings, bundle.ValidationIssue{
				Type:       "undefined_variable",
				File:       "template.json",
				Message:    fmt.Sprintf("Template uses variable '%s' not defined in schema", templateVar),
				Suggestion: fmt.Sprintf("Add '%s' to variables.schema.json or remove from template", templateVar),
			})
		}
	}

	// Check manifest variables against schema
	if len(manifest.RequiredVariables) > 0 {
		for varName := range manifest.RequiredVariables {
			if !schemaVars[varName] {
				result.Warnings = append(result.Warnings, bundle.ValidationIssue{
					Type:       "manifest_schema_mismatch",
					File:       "manifest.json",
					Message:    fmt.Sprintf("Manifest defines variable '%s' not found in schema", varName),
					Suggestion: fmt.Sprintf("Add '%s' to variables.schema.json or remove from manifest", varName),
				})
			}
		}
	}

	return nil
}

func (v *Validator) checkCommonIssues(fs afero.Fs, bundlePath string, result *bundle.ValidationResult) {
	// Check for examples directory
	examplesPath := filepath.Join(bundlePath, "examples")
	if exists, _ := afero.DirExists(fs, examplesPath); !exists {
		result.Warnings = append(result.Warnings, bundle.ValidationIssue{
			Type:       "missing_examples",
			File:       "examples/",
			Message:    "No examples directory found",
			Suggestion: "Add examples directory with sample variable files",
		})
	} else {
		// Check if examples directory is empty
		if files, err := afero.ReadDir(fs, examplesPath); err == nil && len(files) == 0 {
			result.Warnings = append(result.Warnings, bundle.ValidationIssue{
				Type:       "empty_examples",
				File:       "examples/",
				Message:    "Examples directory is empty",
				Suggestion: "Add sample variable files to help users get started",
			})
		}
	}
}

func (v *Validator) extractTemplateVariables(templateStr string) []string {
	// Extract variables from Go template patterns {{ .VAR }}
	vars := make(map[string]bool)

	// Look for {{ .VARIABLE_NAME }} patterns
	i := 0
	for i < len(templateStr) {
		start := strings.Index(templateStr[i:], "{{")
		if start == -1 {
			break
		}
		start += i

		end := strings.Index(templateStr[start:], "}}")
		if end == -1 {
			break
		}
		end += start

		if end > start+2 {
			content := strings.TrimSpace(templateStr[start+2 : end])
			// Handle Go template syntax: {{ .VAR }} or {{.VAR}}
			if strings.HasPrefix(content, ".") {
				varName := strings.TrimSpace(content[1:]) // Remove the dot
				if varName != "" && v.isValidVariableName(varName) {
					vars[varName] = true
				}
			} else if content != "" && v.isValidVariableName(content) {
				// Also support legacy {{VAR}} syntax for compatibility
				vars[content] = true
			}
		}

		i = end + 2
	}

	result := make([]string, 0, len(vars))
	for varName := range vars {
		result = append(result, varName)
	}

	return result
}

func (v *Validator) isValidSemver(version string) bool {
	// Basic semver validation - should match X.Y.Z format
	parts := strings.Split(version, ".")
	return len(parts) == 3 &&
		v.isNumeric(parts[0]) &&
		v.isNumeric(parts[1]) &&
		v.isNumeric(parts[2])
}

func (v *Validator) isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (v *Validator) isValidVariableName(name string) bool {
	if name == "" {
		return false
	}

	// Variable names should be uppercase letters, numbers, and underscores
	for _, c := range name {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}

	return true
}
