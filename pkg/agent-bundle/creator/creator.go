package creator

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/afero"

	agent_bundle "station/pkg/agent-bundle"
	"station/pkg/bundle"
	"station/pkg/models"
)

// Repository interfaces for database access
type AgentRepository interface {
	GetByID(agentID int64) (*models.Agent, error)
}

type AgentToolRepository interface {
	GetAgentToolsWithDetails(agentID int64) ([]*models.AgentToolWithDetails, error)
}

type EnvironmentRepository interface {
	GetByID(environmentID int64) (*models.Environment, error)
}

// Creator implements the AgentBundleCreator interface
type Creator struct {
	fs       afero.Fs
	registry bundle.BundleRegistry // For future MCP bundle integration
	// Database repositories for export functionality
	agentRepo     AgentRepository
	toolRepo      AgentToolRepository
	environmentRepo EnvironmentRepository
}

// New creates a new Creator instance
func New(fs afero.Fs, registry bundle.BundleRegistry) *Creator {
	return &Creator{
		fs:       fs,
		registry: registry,
	}
}

// NewWithDatabaseAccess creates a Creator with database access for export functionality
func NewWithDatabaseAccess(fs afero.Fs, registry bundle.BundleRegistry, agentRepo AgentRepository, toolRepo AgentToolRepository, envRepo EnvironmentRepository) *Creator {
	return &Creator{
		fs:              fs,
		registry:        registry,
		agentRepo:       agentRepo,
		toolRepo:        toolRepo,
		environmentRepo: envRepo,
	}
}

// Create creates a new agent bundle from scratch
func (c *Creator) Create(path string, opts agent_bundle.CreateOptions) error {
	// Validate options
	if err := c.validateOptions(opts); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	// Check if directory already exists
	exists, err := afero.DirExists(c.fs, path)
	if err != nil {
		return fmt.Errorf("failed to check directory existence: %w", err)
	}
	if exists {
		return fmt.Errorf("directory already exists: %s", path)
	}

	// Create the bundle manifest
	manifest := c.createManifest(opts)

	// Generate scaffolding
	if err := c.GenerateScaffolding(path, manifest); err != nil {
		return fmt.Errorf("failed to generate scaffolding: %w", err)
	}

	return nil
}

// ExportFromAgent exports an existing agent to a bundle
func (c *Creator) ExportFromAgent(agentID int64, path string, opts agent_bundle.ExportOptions) error {
	// Check if database access is available
	if c.agentRepo == nil || c.toolRepo == nil || c.environmentRepo == nil {
		return fmt.Errorf("database access not configured - use NewWithDatabaseAccess() to enable export functionality")
	}

	// 1. Query agent from database
	agent, err := c.agentRepo.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	// 2. Get environment information
	environment, err := c.environmentRepo.GetByID(agent.EnvironmentID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// 3. Get agent tools and MCP dependencies  
	agentTools, err := c.toolRepo.GetAgentToolsWithDetails(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent tools: %w", err)
	}

	// 4. Extract template variables from agent configuration
	variables := c.extractVariablesFromAgent(agent, opts)

	// 5. Build MCP dependencies from tools
	mcpDependencies := c.buildMCPDependenciesFromTools(agentTools)

	// 6. Build tool requirements
	toolRequirements := c.buildToolRequirementsFromTools(agentTools)

	// 7. Create bundle manifest
	manifest := &agent_bundle.AgentBundleManifest{
		Name:         c.generateBundleName(agent.Name),
		Version:      "1.0.0",
		Description:  fmt.Sprintf("Exported agent bundle for '%s' from %s environment", agent.Name, environment.Name),
		Author:       "Station Export",
		AgentType:    "exported",
		MCPBundles:   mcpDependencies,
		RequiredTools: toolRequirements,
		RequiredVariables: variables,
		StationVersion: ">=0.1.0",
		CreatedAt:    time.Now(),
		Tags:         []string{"exported", "agent-bundle"},
	}

	// 8. Create agent configuration template
	agentConfig := c.createAgentConfigFromExport(agent, opts)

	// 9. Check if directory already exists
	exists, err := afero.DirExists(c.fs, path)
	if err != nil {
		return fmt.Errorf("failed to check directory existence: %w", err)
	}
	if exists {
		return fmt.Errorf("directory already exists: %s", path)
	}

	// 10. Generate bundle structure
	if err := c.GenerateScaffolding(path, manifest); err != nil {
		return fmt.Errorf("failed to generate bundle structure: %w", err)
	}

	// 11. Write the agent configuration
	agentConfigPath := filepath.Join(path, "agent.json")
	agentConfigJSON, err := json.MarshalIndent(agentConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal agent config: %w", err)
	}
	if err := afero.WriteFile(c.fs, agentConfigPath, agentConfigJSON, 0644); err != nil {
		return fmt.Errorf("failed to write agent config: %w", err)
	}

	// 12. Create variables schema
	if err := c.createVariablesSchema(path, variables); err != nil {
		return fmt.Errorf("failed to create variables schema: %w", err)
	}

	// 13. Create README with export information
	if err := c.createExportReadme(path, agent, environment, manifest); err != nil {
		return fmt.Errorf("failed to create README: %w", err)
	}

	return nil
}

// Helper methods for export functionality

// extractVariablesFromAgent extracts template variables from agent configuration
func (c *Creator) extractVariablesFromAgent(agent *models.Agent, opts agent_bundle.ExportOptions) map[string]agent_bundle.VariableSpec {
	variables := make(map[string]agent_bundle.VariableSpec)
	
	if opts.VariableAnalysis {
		// Analyze agent configuration for template patterns
		templateVars := c.findTemplateVariables(agent.Name, agent.Description, agent.Prompt)
		
		for _, varName := range templateVars {
			variables[varName] = agent_bundle.VariableSpec{
				Type:        "string",
				Description: fmt.Sprintf("Extracted variable from agent configuration"),
				Required:    true,
			}
		}
	}
	
	// Add common agent variables
	variables["AGENT_NAME"] = agent_bundle.VariableSpec{
		Type:        "string",
		Description: "Name of the agent",
		Required:    true,
		Default:     agent.Name,
	}
	
	variables["AGENT_DESCRIPTION"] = agent_bundle.VariableSpec{
		Type:        "string", 
		Description: "Description of the agent",
		Required:    false,
		Default:     agent.Description,
	}
	
	variables["MAX_STEPS"] = agent_bundle.VariableSpec{
		Type:        "number",
		Description: "Maximum steps the agent can take",
		Required:    false,
		Default:     agent.MaxSteps,
	}
	
	return variables
}

// findTemplateVariables finds template variable patterns in text
func (c *Creator) findTemplateVariables(texts ...string) []string {
	variableRegex := regexp.MustCompile(`\{\{\s*\.([A-Z_][A-Z0-9_]*)\s*\}\}`)
	variableSet := make(map[string]bool)
	
	for _, text := range texts {
		matches := variableRegex.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				variableSet[match[1]] = true
			}
		}
	}
	
	var variables []string
	for varName := range variableSet {
		variables = append(variables, varName)
	}
	
	return variables
}

// buildMCPDependenciesFromTools builds MCP bundle dependencies from agent tools
func (c *Creator) buildMCPDependenciesFromTools(tools []*models.AgentToolWithDetails) []agent_bundle.MCPBundleDependency {
	serverMap := make(map[string]agent_bundle.MCPBundleDependency)
	
	for _, tool := range tools {
		bundleName := c.inferMCPBundleName(tool.ServerName)
		
		if _, exists := serverMap[bundleName]; !exists {
			serverMap[bundleName] = agent_bundle.MCPBundleDependency{
				Name:        bundleName,
				Version:     ">=1.0.0",
				Source:      "registry",
				Required:    true,
				Description: fmt.Sprintf("Tools for %s server", tool.ServerName),
			}
		}
	}
	
	var dependencies []agent_bundle.MCPBundleDependency
	for _, dep := range serverMap {
		dependencies = append(dependencies, dep)
	}
	
	return dependencies
}

// buildToolRequirementsFromTools builds tool requirements from agent tools
func (c *Creator) buildToolRequirementsFromTools(tools []*models.AgentToolWithDetails) []agent_bundle.ToolRequirement {
	var requirements []agent_bundle.ToolRequirement
	
	for _, tool := range tools {
		requirements = append(requirements, agent_bundle.ToolRequirement{
			Name:       tool.ToolName,
			ServerName: tool.ServerName,
			MCPBundle:  c.inferMCPBundleName(tool.ServerName),
			Required:   true,
		})
	}
	
	return requirements
}

// inferMCPBundleName infers MCP bundle name from server name
func (c *Creator) inferMCPBundleName(serverName string) string {
	// Common server name to bundle mappings
	bundleMap := map[string]string{
		"filesystem": "filesystem-tools",
		"web":        "web-tools",
		"data":       "data-processing",
		"database":   "database-tools",
		"docker":     "docker-tools",
		"aws":        "aws-tools",
	}
	
	if bundle, exists := bundleMap[serverName]; exists {
		return bundle
	}
	
	// Default pattern: server-name-tools
	return fmt.Sprintf("%s-tools", strings.ToLower(serverName))
}

// generateBundleName generates a bundle name from agent name
func (c *Creator) generateBundleName(agentName string) string {
	// Convert agent name to bundle-friendly format
	bundleName := strings.ToLower(agentName)
	bundleName = strings.ReplaceAll(bundleName, " ", "-")
	bundleName = regexp.MustCompile(`[^a-z0-9\-]`).ReplaceAllString(bundleName, "")
	
	return bundleName + "-bundle"
}

// createAgentConfigFromExport creates agent configuration template from exported agent
func (c *Creator) createAgentConfigFromExport(agent *models.Agent, opts agent_bundle.ExportOptions) *agent_bundle.AgentTemplateConfig {
	// Convert agent fields to template format
	config := &agent_bundle.AgentTemplateConfig{
		Name:         "{{ .AGENT_NAME }}",
		Description:  "{{ .AGENT_DESCRIPTION }}",
		Prompt:       c.convertToTemplate(agent.Prompt, opts),
		MaxSteps:     agent.MaxSteps, // Use actual value, not template
		NameTemplate: "{{ .AGENT_NAME }}",
		PromptTemplate: c.convertToTemplate(agent.Prompt, opts),
	}
	
	return config
}

// convertToTemplate converts static text to template format by identifying variable patterns
func (c *Creator) convertToTemplate(text string, opts agent_bundle.ExportOptions) string {
	if !opts.VariableAnalysis {
		return text
	}
	
	// This is a simplified conversion - in production, you might want more sophisticated analysis
	// For now, we'll just return the original text as it might already contain template variables
	return text
}

// createVariablesSchema creates the variables.schema.json file
func (c *Creator) createVariablesSchema(path string, variables map[string]agent_bundle.VariableSpec) error {
	schemaPath := filepath.Join(path, "variables.schema.json")
	schemaJSON, err := json.MarshalIndent(variables, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal variables schema: %w", err)
	}
	
	return afero.WriteFile(c.fs, schemaPath, schemaJSON, 0644)
}

// createExportReadme creates a README.md file with export information
func (c *Creator) createExportReadme(path string, agent *models.Agent, environment *models.Environment, manifest *agent_bundle.AgentBundleManifest) error {
	readmeContent := fmt.Sprintf(`# %s

## Overview

This agent bundle was exported from Station on %s.

**Original Agent Information:**
- Agent ID: %d
- Agent Name: %s
- Environment: %s (%d)
- Description: %s
- Max Steps: %d

## Bundle Information

- Bundle Name: %s
- Version: %s
- MCP Dependencies: %d
- Required Tools: %d
- Variables: %d

## Installation

### CLI Installation
'''bash
stn agent bundle install ./ --env production --vars-file ./variables.json
'''

### API Installation
'''bash
curl -X POST http://localhost:8080/api/v1/agents/templates/install \
  -H "Content-Type: application/json" \
  -d '{
    "bundle_path": "/path/to/this/bundle",
    "environment": "production",
    "variables": {
      "AGENT_NAME": "%s",
      "AGENT_DESCRIPTION": "%s", 
      "MAX_STEPS": %d
    }
  }'
'''

## Required Variables

This bundle requires the following variables to be configured:

`, 
		manifest.Name,
		manifest.CreatedAt.Format("2006-01-02 15:04:05"),
		agent.ID,
		agent.Name,
		environment.Name,
		environment.ID,
		agent.Description,
		agent.MaxSteps,
		manifest.Name,
		manifest.Version,
		len(manifest.MCPBundles),
		len(manifest.RequiredTools),
		len(manifest.RequiredVariables),
		agent.Name,
		agent.Description,
		agent.MaxSteps)
	
	// Add variables documentation
	for varName, spec := range manifest.RequiredVariables {
		required := "Optional"
		if spec.Required {
			required = "Required"
		}
		
		readmeContent += fmt.Sprintf("- **%s** (%s, %s): %s\n", 
			varName, spec.Type, required, spec.Description)
		
		if spec.Default != nil {
			readmeContent += fmt.Sprintf("  - Default: %v\n", spec.Default)
		}
	}
	
	readmeContent += `
## MCP Dependencies

This bundle requires the following MCP bundles:

`
	
	for _, dep := range manifest.MCPBundles {
		readmeContent += fmt.Sprintf("- **%s** (%s): %s\n", 
			dep.Name, dep.Version, dep.Description)
	}
	
	readmeContent += `
## Export Information

This bundle was automatically generated from an existing Station agent. The original configuration has been converted to a template format with configurable variables.

To modify this bundle:
1. Edit 'agent.json' to adjust the agent configuration template
2. Modify 'variables.schema.json' to add/remove/change variables
3. Update 'manifest.json' to adjust dependencies and metadata
4. Validate with: 'stn agent bundle validate ./'
`
	
	readmePath := filepath.Join(path, "README.md")
	return afero.WriteFile(c.fs, readmePath, []byte(readmeContent), 0644)
}

// AnalyzeDependencies analyzes the dependencies for an agent
func (c *Creator) AnalyzeDependencies(agentID int64) (*agent_bundle.DependencyAnalysis, error) {
	// TODO: Implement when database integration is available
	// This will:
	// 1. Query agent tools from database
	// 2. Map tools to MCP servers and bundles
	// 3. Identify required MCP bundles
	// 4. Detect conflicts or missing dependencies
	return nil, fmt.Errorf("AnalyzeDependencies not yet implemented - requires database integration")
}

// GenerateScaffolding creates the basic bundle structure
func (c *Creator) GenerateScaffolding(path string, manifest *agent_bundle.AgentBundleManifest) error {
	// Create base directory
	if err := c.fs.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create manifest.json
	if err := c.writeManifest(path, manifest); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Create agent.json template
	if err := c.writeAgentConfig(path, manifest); err != nil {
		return fmt.Errorf("failed to write agent config: %w", err)
	}

	// Create tools.json
	if err := c.writeToolsConfig(path, manifest); err != nil {
		return fmt.Errorf("failed to write tools config: %w", err)
	}

	// Create variables.schema.json
	if err := c.writeVariablesSchema(path, manifest); err != nil {
		return fmt.Errorf("failed to write variables schema: %w", err)
	}

	// Create README.md
	if err := c.writeReadme(path, manifest); err != nil {
		return fmt.Errorf("failed to write README: %w", err)
	}

	// Create examples directory with sample files
	if err := c.writeExamples(path, manifest); err != nil {
		return fmt.Errorf("failed to write examples: %w", err)
	}

	return nil
}

// validateOptions validates the create options
func (c *Creator) validateOptions(opts agent_bundle.CreateOptions) error {
	if opts.Name == "" {
		return fmt.Errorf("name is required")
	}

	if opts.Author == "" {
		return fmt.Errorf("author is required")
	}

	if opts.Description == "" {
		return fmt.Errorf("description is required")
	}

	// Validate agent type if provided
	if opts.AgentType != "" {
		validTypes := []string{"task", "scheduled", "interactive"}
		isValid := false
		for _, validType := range validTypes {
			if opts.AgentType == validType {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("invalid agent type '%s', must be one of: %s", opts.AgentType, strings.Join(validTypes, ", "))
		}
	}

	return nil
}

// createManifest creates a manifest from create options
func (c *Creator) createManifest(opts agent_bundle.CreateOptions) *agent_bundle.AgentBundleManifest {
	agentType := opts.AgentType
	if agentType == "" {
		agentType = "task" // Default to task type
	}

	manifest := &agent_bundle.AgentBundleManifest{
		Name:           opts.Name,
		Version:        "1.0.0",
		Description:    opts.Description,
		Author:         opts.Author,
		AgentType:      agentType,
		StationVersion: ">=0.1.0",
		CreatedAt:      time.Now(),
		Tags:           opts.Tags,
	}

	// Add basic required variables if not provided
	if manifest.RequiredVariables == nil {
		manifest.RequiredVariables = make(map[string]agent_bundle.VariableSpec)
	}

	// Add example variable if none provided
	if len(manifest.RequiredVariables) == 0 {
		manifest.RequiredVariables["EXAMPLE_VAR"] = agent_bundle.VariableSpec{
			Type:        "string",
			Description: "An example variable - replace with your actual variables",
			Required:    false,
			Default:     "example-value",
		}
	}

	// Add default MCP bundles and tools (examples)
	manifest.MCPBundles = []agent_bundle.MCPBundleDependency{
		{
			Name:        "filesystem-tools",
			Version:     ">=1.0.0",
			Source:      "registry",
			Required:    true,
			Description: "Basic filesystem operations",
		},
	}

	manifest.RequiredTools = []agent_bundle.ToolRequirement{
		{
			Name:       "list_directory",
			ServerName: "filesystem",
			MCPBundle:  "filesystem-tools",
			Required:   true,
		},
	}

	return manifest
}

// writeManifest writes the manifest.json file
func (c *Creator) writeManifest(path string, manifest *agent_bundle.AgentBundleManifest) error {
	manifestPath := filepath.Join(path, "manifest.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return afero.WriteFile(c.fs, manifestPath, data, 0644)
}

// writeAgentConfig writes the agent.json template file
func (c *Creator) writeAgentConfig(path string, manifest *agent_bundle.AgentBundleManifest) error {
	agentConfig := &agent_bundle.AgentTemplateConfig{
		Name:           manifest.Name,
		Description:    manifest.Description,
		Prompt:         fmt.Sprintf("You are an AI agent helping with %s. Always be helpful and accurate.", manifest.Description),
		MaxSteps:       5,
		NameTemplate:   fmt.Sprintf("{{ .ENVIRONMENT }}-%s", manifest.Name),
		PromptTemplate: fmt.Sprintf("You are working on %s for {{ .CLIENT_NAME }}. {{ .CONTEXT }}", manifest.Description),
		SupportedEnvs:  []string{"development", "staging", "production"},
		DefaultVars: map[string]interface{}{
			"MAX_RETRIES": 3,
			"TIMEOUT":     30,
			"LOG_LEVEL":   "info",
		},
		Version:   manifest.Version,
		CreatedAt: manifest.CreatedAt,
		UpdatedAt: manifest.CreatedAt,
	}

	// Add schedule template for scheduled agents
	if manifest.AgentType == "scheduled" {
		schedule := "0 */4 * * *" // Every 4 hours
		agentConfig.ScheduleTemplate = &schedule
		manifest.SupportedSchedules = []string{"0 */4 * * *", "@daily", "@hourly"}
	}

	agentPath := filepath.Join(path, "agent.json")
	data, err := json.MarshalIndent(agentConfig, "", "  ")
	if err != nil {
		return err
	}
	return afero.WriteFile(c.fs, agentPath, data, 0644)
}

// writeToolsConfig writes the tools.json file
func (c *Creator) writeToolsConfig(path string, manifest *agent_bundle.AgentBundleManifest) error {
	toolsConfig := map[string]interface{}{
		"required_tools": manifest.RequiredTools,
		"mcp_bundles":    manifest.MCPBundles,
		"description":    "Tool requirements and MCP bundle dependencies for this agent",
	}

	toolsPath := filepath.Join(path, "tools.json")
	data, err := json.MarshalIndent(toolsConfig, "", "  ")
	if err != nil {
		return err
	}
	return afero.WriteFile(c.fs, toolsPath, data, 0644)
}

// writeVariablesSchema writes the variables.schema.json file
func (c *Creator) writeVariablesSchema(path string, manifest *agent_bundle.AgentBundleManifest) error {
	// Start with manifest variables or create basic schema
	variables := manifest.RequiredVariables
	if variables == nil {
		variables = make(map[string]agent_bundle.VariableSpec)
	}

	// Ensure we have example variables
	if len(variables) == 0 {
		variables["EXAMPLE_VAR"] = agent_bundle.VariableSpec{
			Type:        "string",
			Description: "An example variable - replace with your actual variables",
			Required:    false,
			Default:     "example-value",
		}
	}

	variablesPath := filepath.Join(path, "variables.schema.json")
	data, err := json.MarshalIndent(variables, "", "  ")
	if err != nil {
		return err
	}
	return afero.WriteFile(c.fs, variablesPath, data, 0644)
}

// writeReadme writes the README.md file
func (c *Creator) writeReadme(path string, manifest *agent_bundle.AgentBundleManifest) error {
	readme := fmt.Sprintf(`# %s

%s

**Author:** %s  
**Version:** %s  
**Type:** %s

## Description

%s

## Installation

To install this agent bundle:

%s

%s

%s

%s

%s

## Configuration

This agent requires the following variables:

`, manifest.Name, manifest.Description, manifest.Author, manifest.Version, manifest.AgentType, manifest.Description,
		"```bash",
		fmt.Sprintf("stn agent install %s.tar.gz --environment your-environment", manifest.Name),
		"```",
		"",
		"## Variables")

	// Add variable documentation
	for varName, varSpec := range manifest.RequiredVariables {
		required := "Optional"
		if varSpec.Required {
			required = "**Required**"
		}
		readme += fmt.Sprintf("- **%s** (%s): %s %s\n", varName, varSpec.Type, varSpec.Description, required)
	}

	readme += `
## MCP Dependencies

This agent requires the following MCP bundles:

`

	// Add MCP bundle documentation
	for _, mcpBundle := range manifest.MCPBundles {
		required := "Optional"
		if mcpBundle.Required {
			required = "**Required**"
		}
		readme += fmt.Sprintf("- **%s** (%s): %s %s\n", mcpBundle.Name, mcpBundle.Version, mcpBundle.Description, required)
	}

	readme += `
## Usage Examples

See the ` + "`examples/`" + ` directory for configuration examples for different environments.

## Development

To modify this agent bundle:

1. Edit ` + "`agent.json`" + ` for agent configuration
2. Update ` + "`tools.json`" + ` for tool requirements  
3. Modify ` + "`variables.schema.json`" + ` for variable definitions
4. Test with ` + "`stn agent validate .`" + `
5. Package with ` + "`stn agent bundle .`" + `

## License

` + getDefaultLicense(manifest.License) + `
`

	readmePath := filepath.Join(path, "README.md")
	return afero.WriteFile(c.fs, readmePath, []byte(readme), 0644)
}

// writeExamples writes example configuration files
func (c *Creator) writeExamples(path string, manifest *agent_bundle.AgentBundleManifest) error {
	examplesDir := filepath.Join(path, "examples")
	if err := c.fs.MkdirAll(examplesDir, 0755); err != nil {
		return err
	}

	// Create development example
	devExample := c.generateExampleVariables(manifest, "development")
	devPath := filepath.Join(examplesDir, "development.yml")
	if err := afero.WriteFile(c.fs, devPath, []byte(devExample), 0644); err != nil {
		return err
	}

	// Create production example
	prodExample := c.generateExampleVariables(manifest, "production")
	prodPath := filepath.Join(examplesDir, "production.yml")
	if err := afero.WriteFile(c.fs, prodPath, []byte(prodExample), 0644); err != nil {
		return err
	}

	return nil
}

// generateExampleVariables generates example variable files for different environments
func (c *Creator) generateExampleVariables(manifest *agent_bundle.AgentBundleManifest, environment string) string {
	example := fmt.Sprintf(`# Example variables for %s environment
# Copy this file and customize for your needs

`, environment)

	for varName, varSpec := range manifest.RequiredVariables {
		example += fmt.Sprintf("# %s\n", varSpec.Description)
		if varSpec.Required {
			example += "# REQUIRED\n"
		}
		if varSpec.Default != nil {
			example += fmt.Sprintf("# Default: %v\n", varSpec.Default)
		}
		if len(varSpec.Enum) > 0 {
			example += fmt.Sprintf("# Options: %s\n", strings.Join(varSpec.Enum, ", "))
		}

		// Generate example values based on environment and type
		exampleValue := c.generateExampleValue(varName, varSpec, environment)
		example += fmt.Sprintf("%s: %s\n\n", varName, exampleValue)
	}

	return example
}

// generateExampleValue generates an example value for a variable
func (c *Creator) generateExampleValue(varName string, spec agent_bundle.VariableSpec, environment string) string {
	if spec.Default != nil && environment == "development" {
		return fmt.Sprintf("%v", spec.Default)
	}

	switch spec.Type {
	case "string":
		if spec.Sensitive {
			return "\"your-secret-key-here\""
		}
		if len(spec.Enum) > 0 {
			// Use environment-appropriate enum value
			for _, option := range spec.Enum {
				if strings.Contains(option, environment) {
					return fmt.Sprintf("\"%s\"", option)
				}
			}
			return fmt.Sprintf("\"%s\"", spec.Enum[0])
		}
		// Generate environment-specific example
		if environment == "production" {
			return "\"production-value\""
		}
		return "\"development-value\""
	case "number":
		if environment == "production" {
			return "10"
		}
		return "5"
	case "boolean":
		if environment == "production" {
			return "true"
		}
		return "false"
	default:
		return "\"example-value\""
	}
}

// getDefaultLicense returns default license text
func getDefaultLicense(license string) string {
	if license != "" {
		return license
	}
	return "See LICENSE file"
}