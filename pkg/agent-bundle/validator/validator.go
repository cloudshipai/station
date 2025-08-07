package validator

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/spf13/afero"

	agent_bundle "station/pkg/agent-bundle"
)

// Validator implements the AgentBundleValidator interface
type Validator struct {
	fs afero.Fs
}

// New creates a new Validator instance
func New(fs afero.Fs) *Validator {
	return &Validator{
		fs: fs,
	}
}

// Validate performs comprehensive validation of an agent bundle
func (v *Validator) Validate(bundlePath string) (*agent_bundle.ValidationResult, error) {
	result := &agent_bundle.ValidationResult{
		Valid:    true,
		Errors:   []agent_bundle.ValidationError{},
		Warnings: []agent_bundle.ValidationWarning{},
	}

	// Check if bundle directory exists
	exists, err := afero.DirExists(v.fs, bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to check bundle directory: %w", err)
	}
	if !exists {
		result.Valid = false
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "missing_bundle",
			Message:    fmt.Sprintf("Bundle directory not found: %s", bundlePath),
			Suggestion: "Ensure the bundle path is correct and the directory exists",
		})
		return result, nil
	}

	// Validate manifest.json
	manifest, err := v.validateManifestFile(bundlePath, result)
	if err != nil {
		result.Valid = false
		result.ManifestValid = false
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "manifest_error",
			Message:    err.Error(),
			Field:      "manifest.json",
			Suggestion: "Fix the manifest.json file structure and content",
		})
	}

	// Validate agent.json
	agentConfig, err := v.validateAgentConfigFile(bundlePath, result)
	if err != nil {
		result.Valid = false
		result.AgentConfigValid = false
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "agent_config_error",
			Message:    err.Error(),
			Field:      "agent.json",
			Suggestion: "Fix the agent.json file structure and content",
		})
	}

	// Validate tools.json
	v.validateToolsFile(bundlePath, result)

	// Validate variables.schema.json
	variableSchema, err := v.validateVariablesFile(bundlePath, result)
	if err != nil {
		result.Valid = false
		result.VariablesValid = false
	}

	// Cross-validate variables between agent config and schema
	if manifest != nil && agentConfig != nil && variableSchema != nil {
		v.validateVariableConsistency(agentConfig, variableSchema, result)
	}

	// Validate tool mappings against MCP dependencies
	if manifest != nil {
		v.validateToolDependencies(manifest, result)
	}

	// Check for optional files and directories
	v.validateOptionalFiles(bundlePath, result)

	// Calculate statistics
	if variableSchema != nil {
		v.calculateStatistics(manifest, variableSchema, result)
	}

	// Set overall validity based on errors
	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result, nil
}

// ValidateManifest validates the bundle manifest
func (v *Validator) ValidateManifest(manifest *agent_bundle.AgentBundleManifest) error {
	if manifest.Name == "" {
		return fmt.Errorf("name is required")
	}

	if manifest.Version == "" {
		return fmt.Errorf("version is required")
	}

	if !v.isValidSemver(manifest.Version) {
		return fmt.Errorf("invalid semantic version: %s", manifest.Version)
	}

	if manifest.Description == "" {
		return fmt.Errorf("description is required")
	}

	if manifest.Author == "" {
		return fmt.Errorf("author is required")
	}

	// Validate agent type
	if manifest.AgentType != "" {
		validTypes := []string{"task", "scheduled", "interactive"}
		isValid := false
		for _, validType := range validTypes {
			if manifest.AgentType == validType {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("invalid agent type '%s', must be one of: %s", 
				manifest.AgentType, strings.Join(validTypes, ", "))
		}
	}

	// Validate station version constraint
	if manifest.StationVersion == "" {
		return fmt.Errorf("station_version is required")
	}

	return nil
}

// ValidateAgentConfig validates the agent configuration
func (v *Validator) ValidateAgentConfig(config *agent_bundle.AgentTemplateConfig) error {
	if config.Name == "" {
		return fmt.Errorf("name is required")
	}

	if config.Description == "" {
		return fmt.Errorf("description is required")
	}

	if config.Prompt == "" {
		return fmt.Errorf("prompt is required")
	}

	if config.MaxSteps <= 0 {
		return fmt.Errorf("max_steps must be positive")
	}

	// Validate template syntax in templated fields
	templateFields := map[string]string{
		"prompt":          config.Prompt,
		"name_template":   config.NameTemplate,
		"prompt_template": config.PromptTemplate,
		"description":     config.Description,
	}

	if config.ScheduleTemplate != nil {
		templateFields["schedule_template"] = *config.ScheduleTemplate
	}

	for fieldName, fieldValue := range templateFields {
		if fieldValue != "" {
			if err := v.validateTemplateString(fieldValue); err != nil {
				return fmt.Errorf("invalid template syntax in %s: %w", fieldName, err)
			}
		}
	}

	return nil
}

// ValidateToolMappings ensures tool requirements can be satisfied by MCP dependencies
func (v *Validator) ValidateToolMappings(tools []agent_bundle.ToolRequirement, mcpBundles []agent_bundle.MCPBundleDependency) error {
	// Create map of available MCP bundles
	bundleMap := make(map[string]agent_bundle.MCPBundleDependency)
	for _, bundle := range mcpBundles {
		bundleMap[bundle.Name] = bundle
	}

	// Check each tool requirement
	for _, tool := range tools {
		// Check if referenced MCP bundle exists
		bundle, exists := bundleMap[tool.MCPBundle]
		if !exists {
			return fmt.Errorf("tool '%s' references undefined MCP bundle '%s'", 
				tool.Name, tool.MCPBundle)
		}

		// Check if required tool references optional bundle
		if tool.Required && !bundle.Required {
			return fmt.Errorf("required tool '%s' from optional MCP bundle '%s'", 
				tool.Name, bundle.Name)
		}
	}

	return nil
}

// ValidateDependencies checks that all MCP bundle dependencies are valid and accessible
func (v *Validator) ValidateDependencies(dependencies []agent_bundle.MCPBundleDependency) error {
	for _, dep := range dependencies {
		if dep.Name == "" {
			return fmt.Errorf("MCP bundle dependency missing name")
		}

		if dep.Version == "" {
			return fmt.Errorf("MCP bundle '%s' missing version constraint", dep.Name)
		}

		// Validate version constraint format (basic check)
		if !v.isValidVersionConstraint(dep.Version) {
			return fmt.Errorf("invalid version constraint for MCP bundle '%s': %s", 
				dep.Name, dep.Version)
		}

		// Validate source
		validSources := []string{"registry", "local", "url", "git"}
		if dep.Source != "" {
			isValidSource := false
			for _, validSource := range validSources {
				if dep.Source == validSource {
					isValidSource = true
					break
				}
			}
			if !isValidSource {
				return fmt.Errorf("invalid source '%s' for MCP bundle '%s', must be one of: %s",
					dep.Source, dep.Name, strings.Join(validSources, ", "))
			}
		}
	}

	return nil
}

// ValidateVariables ensures variable definitions are consistent and valid
func (v *Validator) ValidateVariables(variables map[string]agent_bundle.VariableSpec, templates []string) error {
	// Validate each variable specification
	for varName, varSpec := range variables {
		if err := v.validateVariableSpec(varName, varSpec); err != nil {
			return fmt.Errorf("invalid variable '%s': %w", varName, err)
		}
	}

	// Extract variables used in templates
	usedVariables := make(map[string]bool)
	for _, templateStr := range templates {
		templateVars := v.extractTemplateVariables(templateStr)
		for _, varName := range templateVars {
			usedVariables[varName] = true
		}
	}

	// Check that all used variables are defined in schema
	var undefinedVars []string
	for varName := range usedVariables {
		if _, defined := variables[varName]; !defined {
			undefinedVars = append(undefinedVars, varName)
		}
	}

	if len(undefinedVars) > 0 {
		return fmt.Errorf("template variables not defined in schema: %s", 
			strings.Join(undefinedVars, ", "))
	}

	return nil
}

// Helper methods

func (v *Validator) validateManifestFile(bundlePath string, result *agent_bundle.ValidationResult) (*agent_bundle.AgentBundleManifest, error) {
	manifestPath := filepath.Join(bundlePath, "manifest.json")
	
	exists, err := afero.Exists(v.fs, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check manifest file: %w", err)
	}
	if !exists {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "missing_file",
			Message:    "manifest.json not found",
			Field:      "manifest.json",
			Suggestion: "Create a manifest.json file with bundle metadata",
		})
		return nil, fmt.Errorf("manifest.json not found")
	}

	data, err := afero.ReadFile(v.fs, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest agent_bundle.AgentBundleManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "invalid_json",
			Message:    fmt.Sprintf("invalid JSON in manifest.json: %v", err),
			Field:      "manifest.json",
			Suggestion: "Fix JSON syntax errors in manifest.json",
		})
		return nil, fmt.Errorf("invalid JSON in manifest.json: %w", err)
	}

	// Validate manifest content - check each field individually for better error reporting
	v.validateManifestFields(&manifest, result)

	if len(result.Errors) == 0 {
		result.ManifestValid = true
	}
	return &manifest, nil
}

func (v *Validator) validateManifestFields(manifest *agent_bundle.AgentBundleManifest, result *agent_bundle.ValidationResult) {
	// Check each required field individually
	if manifest.Name == "" {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "missing_field",
			Message:    "manifest missing required field: name",
			Field:      "name",
			Suggestion: "Add a name for your agent bundle",
		})
		result.ManifestValid = false
	}

	if manifest.Version == "" {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "missing_field",
			Message:    "manifest missing required field: version",
			Field:      "version",
			Suggestion: "Add a semantic version (e.g., '1.0.0') to the manifest",
		})
		result.ManifestValid = false
	} else if !v.isValidSemver(manifest.Version) {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "invalid_field",
			Message:    fmt.Sprintf("invalid semantic version: %s", manifest.Version),
			Field:      "version",
			Suggestion: "Use semantic version format (e.g., '1.0.0')",
		})
		result.ManifestValid = false
	}

	if manifest.Description == "" {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "missing_field",
			Message:    "manifest missing required field: description",
			Field:      "description",
			Suggestion: "Add a description explaining what this agent does",
		})
		result.ManifestValid = false
	}

	if manifest.Author == "" {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "missing_field",
			Message:    "manifest missing required field: author",
			Field:      "author",
			Suggestion: "Add the author name or organization",
		})
		result.ManifestValid = false
	}

	// Validate agent type
	if manifest.AgentType != "" {
		validTypes := []string{"task", "scheduled", "interactive"}
		isValid := false
		for _, validType := range validTypes {
			if manifest.AgentType == validType {
				isValid = true
				break
			}
		}
		if !isValid {
			result.Errors = append(result.Errors, agent_bundle.ValidationError{
				Type:       "invalid_field",
				Message:    fmt.Sprintf("invalid agent type '%s', must be one of: %s", 
					manifest.AgentType, strings.Join(validTypes, ", ")),
				Field:      "agent_type",
				Suggestion: "Use a valid agent type: task, scheduled, or interactive",
			})
			result.ManifestValid = false
		}
	}

	// Validate station version constraint
	if manifest.StationVersion == "" {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "missing_field",
			Message:    "manifest missing required field: station_version",
			Field:      "station_version",
			Suggestion: "Add station version constraint (e.g., '>=0.1.0')",
		})
		result.ManifestValid = false
	}
}

func (v *Validator) validateAgentConfigFile(bundlePath string, result *agent_bundle.ValidationResult) (*agent_bundle.AgentTemplateConfig, error) {
	agentPath := filepath.Join(bundlePath, "agent.json")
	
	exists, err := afero.Exists(v.fs, agentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check agent config file: %w", err)
	}
	if !exists {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "missing_file",
			Message:    "agent.json not found",
			Field:      "agent.json",
			Suggestion: "Create an agent.json file with agent configuration",
		})
		return nil, fmt.Errorf("agent.json not found")
	}

	data, err := afero.ReadFile(v.fs, agentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent config file: %w", err)
	}

	var agentConfig agent_bundle.AgentTemplateConfig
	if err := json.Unmarshal(data, &agentConfig); err != nil {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "invalid_json",
			Message:    fmt.Sprintf("invalid JSON in agent.json: %v", err),
			Field:      "agent.json",
			Suggestion: "Fix JSON syntax errors in agent.json",
		})
		return nil, fmt.Errorf("invalid JSON in agent.json: %w", err)
	}

	// Validate agent config content
	if err := v.ValidateAgentConfig(&agentConfig); err != nil {
		return &agentConfig, err
	}

	// Check for template variables usage
	templateFields := []string{
		agentConfig.Prompt,
		agentConfig.Description,
		agentConfig.NameTemplate,
		agentConfig.PromptTemplate,
	}
	
	hasTemplateVars := false
	for _, field := range templateFields {
		if strings.Contains(field, "{{ .") {
			hasTemplateVars = true
			break
		}
	}
	
	if !hasTemplateVars {
		result.Warnings = append(result.Warnings, agent_bundle.ValidationWarning{
			Type:       "no_template_variables",
			Message:    "Agent configuration contains no template variables",
			Suggestion: "Consider using template variables like {{ .CLIENT_NAME }} for dynamic configuration",
		})
	}

	result.AgentConfigValid = true
	return &agentConfig, nil
}

func (v *Validator) validateToolsFile(bundlePath string, result *agent_bundle.ValidationResult) {
	toolsPath := filepath.Join(bundlePath, "tools.json")
	
	exists, err := afero.Exists(v.fs, toolsPath)
	if err != nil || !exists {
		result.Warnings = append(result.Warnings, agent_bundle.ValidationWarning{
			Type:       "missing_optional_file",
			Message:    "tools.json not found (optional)",
			Suggestion: "Consider adding tools.json to define tool requirements",
		})
		result.ToolsValid = true // Optional file, so valid if missing
		return
	}

	data, err := afero.ReadFile(v.fs, toolsPath)
	if err != nil {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "file_read_error",
			Message:    fmt.Sprintf("failed to read tools.json: %v", err),
			Field:      "tools.json",
		})
		result.ToolsValid = false
		return
	}

	var toolsConfig map[string]interface{}
	if err := json.Unmarshal(data, &toolsConfig); err != nil {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "invalid_json",
			Message:    fmt.Sprintf("invalid JSON in tools.json: %v", err),
			Field:      "tools.json",
			Suggestion: "Fix JSON syntax errors in tools.json",
		})
		result.ToolsValid = false
		return
	}

	result.ToolsValid = true
}

func (v *Validator) validateVariablesFile(bundlePath string, result *agent_bundle.ValidationResult) (map[string]agent_bundle.VariableSpec, error) {
	variablesPath := filepath.Join(bundlePath, "variables.schema.json")
	
	exists, err := afero.Exists(v.fs, variablesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check variables file: %w", err)
	}
	if !exists {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "missing_file",
			Message:    "variables.schema.json not found",
			Field:      "variables.schema.json",
			Suggestion: "Create variables.schema.json to define required variables",
		})
		return nil, fmt.Errorf("variables.schema.json not found")
	}

	data, err := afero.ReadFile(v.fs, variablesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read variables file: %w", err)
	}

	var variables map[string]agent_bundle.VariableSpec
	if err := json.Unmarshal(data, &variables); err != nil {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "invalid_json",
			Message:    fmt.Sprintf("invalid JSON in variables.schema.json: %v", err),
			Field:      "variables.schema.json",
			Suggestion: "Fix JSON syntax errors in variables.schema.json",
		})
		return nil, fmt.Errorf("invalid JSON in variables.schema.json: %w", err)
	}

	// Validate variable specifications
	for varName, varSpec := range variables {
		if err := v.validateVariableSpec(varName, varSpec); err != nil {
			result.Errors = append(result.Errors, agent_bundle.ValidationError{
				Type:       "invalid_variable_spec",
				Message:    fmt.Sprintf("invalid variable specification for '%s': %v", varName, err),
				Field:      varName,
				Suggestion: "Fix the variable specification in variables.schema.json",
			})
			return variables, fmt.Errorf("invalid variable specification: %w", err)
		}
	}

	result.VariablesValid = true
	return variables, nil
}

func (v *Validator) validateVariableConsistency(agentConfig *agent_bundle.AgentTemplateConfig, variables map[string]agent_bundle.VariableSpec, result *agent_bundle.ValidationResult) {
	// Extract template variables from agent config
	templateFields := []string{
		agentConfig.Prompt,
		agentConfig.Description,
		agentConfig.NameTemplate,
		agentConfig.PromptTemplate,
	}
	
	if agentConfig.ScheduleTemplate != nil {
		templateFields = append(templateFields, *agentConfig.ScheduleTemplate)
	}

	usedVariables := make(map[string]bool)
	for _, field := range templateFields {
		vars := v.extractTemplateVariables(field)
		for _, varName := range vars {
			usedVariables[varName] = true
		}
	}

	// Check for undefined variables
	for varName := range usedVariables {
		if _, defined := variables[varName]; !defined {
			result.Errors = append(result.Errors, agent_bundle.ValidationError{
				Type:       "undefined_variable",
				Message:    fmt.Sprintf("template variable '%s' not defined in schema", varName),
				Field:      varName,
				Suggestion: fmt.Sprintf("Add '%s' to variables.schema.json", varName),
			})
			result.VariablesValid = false
		}
	}
}

func (v *Validator) validateToolDependencies(manifest *agent_bundle.AgentBundleManifest, result *agent_bundle.ValidationResult) {
	if err := v.ValidateToolMappings(manifest.RequiredTools, manifest.MCPBundles); err != nil {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "tool_mapping_error",
			Message:    err.Error(),
			Suggestion: "Ensure all required tools reference valid MCP bundles",
		})
		result.DependenciesValid = false
	} else {
		result.DependenciesValid = true
	}

	if err := v.ValidateDependencies(manifest.MCPBundles); err != nil {
		result.Errors = append(result.Errors, agent_bundle.ValidationError{
			Type:       "dependency_error",
			Message:    err.Error(),
			Suggestion: "Fix MCP bundle dependency specifications",
		})
		result.DependenciesValid = false
	}
}

func (v *Validator) validateOptionalFiles(bundlePath string, result *agent_bundle.ValidationResult) {
	// Check for README.md
	readmePath := filepath.Join(bundlePath, "README.md")
	if exists, _ := afero.Exists(v.fs, readmePath); !exists {
		result.Warnings = append(result.Warnings, agent_bundle.ValidationWarning{
			Type:       "missing_readme",
			Message:    "README.md not found",
			Suggestion: "Add README.md with usage instructions and documentation",
		})
	}

	// Check for examples directory
	examplesPath := filepath.Join(bundlePath, "examples")
	if exists, _ := afero.DirExists(v.fs, examplesPath); !exists {
		result.Warnings = append(result.Warnings, agent_bundle.ValidationWarning{
			Type:       "missing_examples",
			Message:    "examples directory not found",
			Suggestion: "Add examples/ directory with sample configurations",
		})
	}
}

func (v *Validator) calculateStatistics(manifest *agent_bundle.AgentBundleManifest, variables map[string]agent_bundle.VariableSpec, result *agent_bundle.ValidationResult) {
	stats := &result.Statistics
	
	stats.TotalVariables = len(variables)
	
	requiredCount := 0
	for _, varSpec := range variables {
		if varSpec.Required {
			requiredCount++
		}
	}
	stats.RequiredVariables = requiredCount
	stats.OptionalVariables = stats.TotalVariables - requiredCount

	if manifest != nil {
		stats.MCPDependencies = len(manifest.MCPBundles)
		
		requiredTools := 0
		for _, tool := range manifest.RequiredTools {
			if tool.Required {
				requiredTools++
			}
		}
		stats.RequiredTools = requiredTools
		stats.OptionalTools = len(manifest.RequiredTools) - requiredTools
	}
}

func (v *Validator) validateTemplateString(templateStr string) error {
	// Try to parse as Go template to validate syntax
	_, err := template.New("test").Parse(templateStr)
	return err
}

func (v *Validator) validateVariableSpec(varName string, spec agent_bundle.VariableSpec) error {
	// Check variable name format
	if !v.isValidVariableName(varName) {
		return fmt.Errorf("invalid variable name format")
	}

	// Validate type
	validTypes := []string{"string", "number", "boolean", "secret"}
	isValidType := false
	for _, validType := range validTypes {
		if spec.Type == validType {
			isValidType = true
			break
		}
	}
	if !isValidType {
		return fmt.Errorf("invalid variable type '%s', must be one of: %s", 
			spec.Type, strings.Join(validTypes, ", "))
	}

	// Validate pattern if provided
	if spec.Pattern != "" {
		if _, err := regexp.Compile(spec.Pattern); err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	return nil
}

func (v *Validator) extractTemplateVariables(templateStr string) []string {
	// Extract variables from Go template patterns {{ .VAR }}
	re := regexp.MustCompile(`\{\{\s*\.([A-Z_][A-Z0-9_]*)\s*\}\}`)
	matches := re.FindAllStringSubmatch(templateStr, -1)
	
	variables := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			variables[match[1]] = true
		}
	}

	var result []string
	for varName := range variables {
		result = append(result, varName)
	}
	
	return result
}

func (v *Validator) isValidSemver(version string) bool {
	// Basic semver validation
	semverPattern := `^(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-(?:(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?:[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`
	matched, _ := regexp.MatchString(semverPattern, version)
	return matched
}

func (v *Validator) isValidVersionConstraint(constraint string) bool {
	// Basic version constraint validation (supports >=, >, <=, <, =, ~, ^)
	constraintPattern := `^(>=|>|<=|<|=|~|\^)?(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-(?:(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?:[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`
	matched, _ := regexp.MatchString(constraintPattern, constraint)
	return matched
}

func (v *Validator) isValidVariableName(name string) bool {
	// Variable names should be uppercase with underscores
	pattern := `^[A-Z_][A-Z0-9_]*$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched
}