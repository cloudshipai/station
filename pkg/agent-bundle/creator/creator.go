package creator

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"

	agent_bundle "station/pkg/agent-bundle"
	"station/pkg/bundle"
)

// Creator implements the AgentBundleCreator interface
type Creator struct {
	fs       afero.Fs
	registry bundle.BundleRegistry // For future MCP bundle integration
}

// New creates a new Creator instance
func New(fs afero.Fs, registry bundle.BundleRegistry) *Creator {
	return &Creator{
		fs:       fs,
		registry: registry,
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
	// TODO: Implement when database integration is available
	// This will:
	// 1. Query agent from database
	// 2. Get agent tools and MCP dependencies
	// 3. Extract template variables from agent configuration
	// 4. Generate bundle with actual configuration
	return fmt.Errorf("ExportFromAgent not yet implemented - requires database integration")
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