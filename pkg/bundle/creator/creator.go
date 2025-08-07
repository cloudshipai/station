package creator

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"

	"station/pkg/bundle"
)

// Creator implements the BundleCreator interface
type Creator struct{}

// NewCreator creates a new bundle creator
func NewCreator() *Creator {
	return &Creator{}
}

// Create creates a new bundle with the given options
func (c *Creator) Create(fs afero.Fs, bundlePath string, opts bundle.CreateOptions) error {
	// Validate options
	if opts.Name == "" {
		return fmt.Errorf("bundle name is required")
	}
	if opts.Author == "" {
		return fmt.Errorf("bundle author is required")
	}
	if opts.Description == "" {
		return fmt.Errorf("bundle description is required")
	}

	// Create bundle directory
	if err := fs.MkdirAll(bundlePath, 0755); err != nil {
		return fmt.Errorf("failed to create bundle directory: %w", err)
	}

	// Create manifest
	manifest := bundle.BundleManifest{
		Name:           opts.Name,
		Version:        "1.0.0",
		Description:    opts.Description,
		Author:         opts.Author,
		License:        opts.License,
		Repository:     opts.Repository,
		StationVersion: ">=0.1.0",
		CreatedAt:      time.Now().UTC(),
		Tags:           opts.Tags,
		RequiredVariables: opts.Variables,
		Dependencies:   opts.Dependencies,
	}

	// Set defaults
	if manifest.License == "" {
		manifest.License = "MIT"
	}

	if err := c.createManifest(fs, bundlePath, manifest); err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	// Create template.json
	if err := c.createTemplate(fs, bundlePath, opts.Name); err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}

	// Create variables.schema.json
	if err := c.createVariablesSchema(fs, bundlePath, opts.Variables); err != nil {
		return fmt.Errorf("failed to create variables schema: %w", err)
	}

	// Create README.md
	if err := c.createREADME(fs, bundlePath, opts); err != nil {
		return fmt.Errorf("failed to create README: %w", err)
	}

	// Create examples directory
	examplesDir := filepath.Join(bundlePath, "examples")
	if err := fs.MkdirAll(examplesDir, 0755); err != nil {
		return fmt.Errorf("failed to create examples directory: %w", err)
	}

	// Create example variable files
	if err := c.createExampleVariables(fs, examplesDir, opts.Variables); err != nil {
		return fmt.Errorf("failed to create example variables: %w", err)
	}

	return nil
}

func (c *Creator) createManifest(fs afero.Fs, bundlePath string, manifest bundle.BundleManifest) error {
	manifestPath := filepath.Join(bundlePath, "manifest.json")
	
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	return afero.WriteFile(fs, manifestPath, data, 0644)
}

func (c *Creator) createTemplate(fs afero.Fs, bundlePath, bundleName string) error {
	templatePath := filepath.Join(bundlePath, "template.json")
	
	template := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			bundleName: map[string]interface{}{
				"command": "echo",
				"args":    []string{"Replace with your MCP server configuration"},
				"env": map[string]string{
					"EXAMPLE_VAR": "{{EXAMPLE_VAR}}",
				},
			},
		},
	}

	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}

	return afero.WriteFile(fs, templatePath, data, 0644)
}

func (c *Creator) createVariablesSchema(fs afero.Fs, bundlePath string, variables map[string]bundle.VariableSpec) error {
	schemaPath := filepath.Join(bundlePath, "variables.schema.json")
	
	schema := map[string]interface{}{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type":    "object",
		"title":   "Bundle Variables Schema",
		"properties": map[string]interface{}{},
		"required": []string{},
	}

	properties := schema["properties"].(map[string]interface{})
	required := []string{}

	// Add example variable if none provided
	if len(variables) == 0 {
		properties["EXAMPLE_VAR"] = map[string]interface{}{
			"type":        "string",
			"description": "Example variable - replace with your actual variables",
			"default":     "example-value",
		}
	} else {
		for name, spec := range variables {
			prop := map[string]interface{}{
				"type":        spec.Type,
				"description": spec.Description,
			}
			
			if spec.Default != nil {
				prop["default"] = spec.Default
			}
			
			if len(spec.Enum) > 0 {
				prop["enum"] = spec.Enum
			}
			
			properties[name] = prop
			
			if spec.Required {
				required = append(required, name)
			}
		}
	}

	schema["required"] = required

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}

	return afero.WriteFile(fs, schemaPath, data, 0644)
}

func (c *Creator) createREADME(fs afero.Fs, bundlePath string, opts bundle.CreateOptions) error {
	readmePath := filepath.Join(bundlePath, "README.md")
	
	readme := fmt.Sprintf("# %s\n\n%s\n\n## Installation\n\n```bash\nstn template install %s\n```\n\n## Usage\n\n```bash\n# Sync with your environment (will prompt for required variables)\nstn mcp sync development\n\n# Or provide variables via environment variables\nexport EXAMPLE_VAR=\"your-value\"\nstn mcp sync production\n```\n\n## Required Variables\n\n", opts.Name, opts.Description, opts.Name)

	if len(opts.Variables) == 0 {
		readme += "- `EXAMPLE_VAR`: Example variable - replace with your actual variables\n"
	} else {
		for name, spec := range opts.Variables {
			secretNote := ""
			if spec.Secret {
				secretNote = " (secret)"
			}
			
			defaultNote := ""
			if spec.Default != nil {
				defaultNote = fmt.Sprintf(" - default: `%v`", spec.Default)
			}
			
			readme += fmt.Sprintf("- `%s`: %s%s%s\n", name, spec.Description, secretNote, defaultNote)
		}
	}

	readme += "\n## Configuration\n\nThis bundle provides the following tools:\n\n- Replace with actual tool descriptions\n\n## Examples\n\nSee the `examples/` directory for sample configurations.\n\n## License\n\n" + opts.License + "\n"

	return afero.WriteFile(fs, readmePath, []byte(readme), 0644)
}

func (c *Creator) createExampleVariables(fs afero.Fs, examplesDir string, variables map[string]bundle.VariableSpec) error {
	// Create development example
	devVars := make(map[string]interface{})
	
	if len(variables) == 0 {
		devVars["EXAMPLE_VAR"] = "development-value"
	} else {
		for name, spec := range variables {
			if spec.Default != nil {
				devVars[name] = spec.Default
			} else {
				switch spec.Type {
				case "string":
					devVars[name] = "development-" + name
				case "boolean":
					devVars[name] = false
				case "number":
					devVars[name] = 0
				default:
					devVars[name] = "development-" + name
				}
			}
		}
	}

	devData, err := yaml.Marshal(devVars)
	if err != nil {
		return err
	}

	devPath := filepath.Join(examplesDir, "development.vars.yml")
	if err := afero.WriteFile(fs, devPath, devData, 0644); err != nil {
		return err
	}

	// Create production example (with placeholders for secrets)
	prodVars := make(map[string]interface{})
	
	if len(variables) == 0 {
		prodVars["EXAMPLE_VAR"] = "production-value"
	} else {
		for name, spec := range variables {
			if spec.Secret {
				prodVars[name] = "# Set via environment variable or secure secrets management"
			} else if spec.Default != nil {
				prodVars[name] = spec.Default
			} else {
				switch spec.Type {
				case "string":
					prodVars[name] = "production-" + name
				case "boolean":
					prodVars[name] = true
				case "number":
					prodVars[name] = 1
				default:
					prodVars[name] = "production-" + name
				}
			}
		}
	}

	prodData, err := yaml.Marshal(prodVars)
	if err != nil {
		return err
	}

	prodPath := filepath.Join(examplesDir, "production.vars.yml")
	return afero.WriteFile(fs, prodPath, prodData, 0644)
}