package dotprompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"github.com/adrg/frontmatter"

	"station/pkg/models"
)

// Converter handles conversion between Station agents and dotprompt format
type Converter struct {
	options ConversionOptions
}

// NewConverter creates a new converter with default options
func NewConverter(options ConversionOptions) *Converter {
	return &Converter{
		options: options,
	}
}

// AgentToDotprompt converts a Station agent to dotprompt format
func (c *Converter) AgentToDotprompt(agent *models.Agent, toolMappings []ToolMapping) (*AgentDotprompt, error) {
	// Build dotprompt config
	config := DotpromptConfig{
		Model: c.getModelForProvider(agent), // Will determine based on config
		Input: InputSchema{
			Schema: map[string]interface{}{
				"task": "string",
				"context": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": true,
					"description":         "Additional context for task execution",
				},
				"parameters": map[string]interface{}{
					"type":                 "object", 
					"additionalProperties": true,
					"description":         "Task-specific parameters",
				},
			},
		},
		Output: OutputSchema{
			Format: "json",
			Schema: c.getOutputSchemaForDomain(agent.Description),
		},
		Tools: c.extractToolNames(toolMappings),
		Metadata: AgentMetadata{
			AgentID:         agent.ID,
			Name:            agent.Name,
			Description:     agent.Description,
			MaxSteps:        agent.MaxSteps,
			Environment:     "default", // TODO: Get from context
			ToolsVersion:    c.generateToolsVersion(toolMappings),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
			Version:         "1.0",
			ScheduleEnabled: false, // TODO: Get from agent schedule info
		},
	}

	// Generate prompt template
	template := c.generatePromptTemplate(agent, toolMappings)

	return &AgentDotprompt{
		Config:   config,
		Template: template,
		FilePath: c.getPromptFilePath(agent.Name),
	}, nil
}

// DotpromptToAgent converts a dotprompt file back to Station agent format
func (c *Converter) DotpromptToAgent(dotpromptPath string) (*models.Agent, []ToolMapping, error) {
	// Read dotprompt file
	content, err := os.ReadFile(dotpromptPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read dotprompt file: %w", err)
	}

	// Parse frontmatter and template
	var config DotpromptConfig
	templateBody, err := frontmatter.Parse(strings.NewReader(string(content)), &config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Convert to Station agent
	agent := &models.Agent{
		ID:          config.Metadata.AgentID,
		Name:        config.Metadata.Name,
		Description: config.Metadata.Description,
		Prompt:      c.extractSystemPrompt(string(templateBody)),
		MaxSteps:    config.Metadata.MaxSteps,
		// Environment and other fields would be set during import
	}

	// Extract tool mappings from frontmatter tools list
	toolMappings := c.reconstructToolMappings(config.Tools, config.Metadata.Environment)

	return agent, toolMappings, nil
}

// SaveDotprompt saves an AgentDotprompt to file
func (c *Converter) SaveDotprompt(dotprompt *AgentDotprompt) error {
	// Ensure directory exists
	dir := filepath.Dir(dotprompt.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal frontmatter
	yamlData, err := yaml.Marshal(dotprompt.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Combine frontmatter and template
	content := fmt.Sprintf("---\n%s---\n\n%s", string(yamlData), dotprompt.Template)

	// Write to file
	err = os.WriteFile(dotprompt.FilePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write dotprompt file: %w", err)
	}

	return nil
}

// LoadDotprompt loads a dotprompt from file
func (c *Converter) LoadDotprompt(filePath string) (*AgentDotprompt, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var config DotpromptConfig
	templateBody, err := frontmatter.Parse(strings.NewReader(string(content)), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	return &AgentDotprompt{
		Config:   config,
		Template: string(templateBody),
		FilePath: filePath,
	}, nil
}

// Helper methods

func (c *Converter) getModelForProvider(agent *models.Agent) string {
	// TODO: Get from current Station config
	return "googleai/gemini-1.5-flash" // Default for now
}

func (c *Converter) getOutputSchemaForDomain(description string) map[string]interface{} {
	// Generate output schema based on agent description/domain
	return map[string]interface{}{
		"result": map[string]interface{}{
			"type":        "object",
			"description": "The result of task execution",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the task was completed successfully",
				},
				"summary": map[string]interface{}{
					"type":        "string", 
					"description": "Summary of actions taken",
				},
				"data": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": true,
					"description":         "Task-specific result data",
				},
			},
			"required": []string{"success", "summary"},
		},
		"metadata": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tools_used": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "List of tools used during execution",
				},
				"execution_time": map[string]interface{}{
					"type":        "number",
					"description": "Execution time in seconds",
				},
			},
		},
	}
}

func (c *Converter) extractToolNames(toolMappings []ToolMapping) []string {
	var toolNames []string
	for _, mapping := range toolMappings {
		toolNames = append(toolNames, mapping.ToolName)
	}
	return toolNames
}

func (c *Converter) generateToolsVersion(toolMappings []ToolMapping) string {
	// Generate a hash of tool mappings for versioning
	// Simple implementation - in production you'd want a proper hash
	toolsStr := ""
	for _, mapping := range toolMappings {
		toolsStr += mapping.ToolName + mapping.ServerName
	}
	return fmt.Sprintf("v1-%d", len(toolsStr))
}

func (c *Converter) generatePromptTemplate(agent *models.Agent, toolMappings []ToolMapping) string {
	var template strings.Builder

	// System section
	template.WriteString("{{#system}}\n")
	template.WriteString(fmt.Sprintf("%s\n\n", agent.Prompt))
	
	if c.options.GenerateToolDocs && len(toolMappings) > 0 {
		template.WriteString("Available tools:\n")
		for _, tool := range toolMappings {
			template.WriteString(fmt.Sprintf("- %s: Tool for %s operations\n", tool.ToolName, tool.ServerName))
		}
		template.WriteString("\n")
	}
	
	template.WriteString("Execute tasks methodically using available tools.\n")
	template.WriteString("Provide structured responses with clear summaries.\n")
	template.WriteString("{{/system}}\n\n")

	// User section
	template.WriteString("Execute the following task: {{task}}\n\n")
	
	template.WriteString("{{#if context}}\n")
	template.WriteString("Additional Context:\n")
	template.WriteString("{{toJson context}}\n\n")
	template.WriteString("{{/if}}\n")
	
	template.WriteString("{{#if parameters}}\n")
	template.WriteString("Task Parameters:\n")
	template.WriteString("{{toJson parameters}}\n\n")
	template.WriteString("{{/if}}\n")

	template.WriteString("Provide a structured JSON response with the results.")

	return template.String()
}

func (c *Converter) extractSystemPrompt(template string) string {
	// Extract system prompt from template for backwards compatibility
	// Simple extraction - look for content between {{#system}} and {{/system}}
	start := strings.Index(template, "{{#system}}")
	end := strings.Index(template, "{{/system}}")
	
	if start == -1 || end == -1 {
		// No system section found, return first paragraph
		lines := strings.Split(template, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.Contains(line, "{{") {
				return line
			}
		}
		return "Intelligent agent for task execution"
	}
	
	systemContent := template[start+len("{{#system}}\n"):end]
	systemContent = strings.TrimSpace(systemContent)
	
	// Remove tool documentation if present
	if strings.Contains(systemContent, "Available tools:") {
		parts := strings.Split(systemContent, "Available tools:")
		systemContent = strings.TrimSpace(parts[0])
	}
	
	return systemContent
}

func (c *Converter) reconstructToolMappings(toolNames []string, environment string) []ToolMapping {
	// This would need to be enhanced to actually reconstruct proper mappings
	// For now, create basic mappings that would need to be resolved during import
	var mappings []ToolMapping
	for _, toolName := range toolNames {
		mappings = append(mappings, ToolMapping{
			ToolName:    toolName,
			Environment: environment,
			// ServerName, MCPServerID, etc. would need to be resolved during import
		})
	}
	return mappings
}

func (c *Converter) getPromptFilePath(agentName string) string {
	// Sanitize agent name for filename
	filename := strings.ToLower(strings.ReplaceAll(agentName, " ", "-"))
	return filepath.Join("prompts", "agents", filename+".prompt")
}