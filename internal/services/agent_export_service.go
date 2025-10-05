package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/schemas"
	"station/pkg/models"
	"station/pkg/schema"
)

// AgentExportService handles automatic export of agents to file-based config
type AgentExportService struct {
	repos          *repositories.Repositories
	schemaRegistry *schemas.SchemaRegistry
}

// NewAgentExportService creates a new agent export service
func NewAgentExportService(repos *repositories.Repositories) *AgentExportService {
	return &AgentExportService{
		repos:          repos,
		schemaRegistry: schemas.NewSchemaRegistry(),
	}
}

// ExportAgentAfterSave automatically exports an agent to file-based config after DB save
func (s *AgentExportService) ExportAgentAfterSave(agentID int64) error {
	return s.ExportAgentAfterSaveWithMetadata(agentID, "", "")
}

// ExportAgentAfterSaveWithMetadata exports an agent with CloudShip metadata fields
func (s *AgentExportService) ExportAgentAfterSaveWithMetadata(agentID int64, app, appType string) error {
	// Get agent details
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %v", err)
	}

	// Auto-populate app/app_type from agent model if available
	if app == "" && agent.App != "" {
		app = agent.App
	}
	if appType == "" && agent.AppType != "" {
		appType = agent.AppType
	}

	// Auto-populate app/app_type for known presets if still not set
	if app == "" && appType == "" && agent.OutputSchemaPreset != nil && *agent.OutputSchemaPreset != "" {
		if presetInfo, exists := s.schemaRegistry.GetPresetInfo(*agent.OutputSchemaPreset); exists {
			app = presetInfo.App
			appType = presetInfo.AppType
		}
	}

	// Get environment info
	environment, err := s.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %v", err)
	}

	// Get agent tools with details
	toolsWithDetails, err := s.repos.AgentTools.ListAgentTools(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent tools: %v", err)
	}

	// Generate dotprompt content using the same logic as MCP handler
	dotpromptContent := s.generateDotpromptContent(agent, toolsWithDetails, environment.Name, app, appType)

	// Determine output file path using centralized path resolution
	outputPath := config.GetAgentPromptPath(environment.Name, agent.Name)

	// Ensure directory exists
	agentsDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %v", err)
	}

	// Write .prompt file to filesystem
	if err := os.WriteFile(outputPath, []byte(dotpromptContent), 0644); err != nil {
		return fmt.Errorf("failed to write .prompt file: %v", err)
	}

	log.Printf("Agent '%s' (ID: %d) successfully exported to: %s", agent.Name, agent.ID, outputPath)
	return nil
}

// generateDotpromptContent generates the dotprompt file content with proper role structure
func (s *AgentExportService) generateDotpromptContent(agent *models.Agent, tools []*models.AgentToolWithDetails, environmentName, app, appType string) string {
	// Build tools list
	var toolNames []string
	for _, tool := range tools {
		toolNames = append(toolNames, tool.ToolName)
	}

	// Start with YAML frontmatter (temperature config removed for gpt-5 compatibility)
	content := fmt.Sprintf(`---
metadata:
  name: "%s"
  description: "%s"
  tags: ["station", "agent"]`, agent.Name, agent.Description)

	// Add app/app_type for CloudShip data ingestion classification if provided
	if app != "" || appType != "" {
		content += "\n  # Data ingestion classification for CloudShip"
		if app != "" {
			content += fmt.Sprintf("\n  app: \"%s\"", app)
		}
		if appType != "" {
			content += fmt.Sprintf("\n  app_type: \"%s\"", appType)
		}
	}

	content += fmt.Sprintf("\nmodel: gpt-4o-mini\nmax_steps: %d", agent.MaxSteps)

	// Add tools if any
	if len(toolNames) > 0 {
		content += "\ntools:\n"
		for _, toolName := range toolNames {
			content += fmt.Sprintf("  - \"%s\"\n", toolName)
		}
	}

	// Add input schema (always include - contains at minimum userInput)
	inputSchemaSection, err := s.generateInputSchemaSection(agent)
	if err == nil {
		content += "\n" + inputSchemaSection
	}

	// Add output schema handling
	outputSchemaSection := s.generateOutputSchemaSection(agent)
	if outputSchemaSection != "" {
		content += outputSchemaSection
	}

	// Close frontmatter and add role-based prompt structure
	content += "---\n\n"

	// Check if the prompt already contains role templates
	if strings.Contains(agent.Prompt, "{{role") {
		// Prompt already has role templates, use as-is
		content += agent.Prompt
	} else {
		// Add system role with the agent's prompt
		content += "{{role \"system\"}}\n"
		content += agent.Prompt
		content += "\n\n"

		// Add user role with handlebars template
		content += "{{role \"user\"}}\n"
		content += "{{userInput}}"
	}

	// Add custom variable handlebars if they exist
	if agent.InputSchema != nil && *agent.InputSchema != "" {
		customVars := s.extractCustomVariableNames(agent)
		for _, varName := range customVars {
			content += fmt.Sprintf("\n\n**%s:** {{%s}}", varName, varName)
		}
	}

	return content
}

// generateInputSchemaSection generates the input schema for dotprompt
func (s *AgentExportService) generateInputSchemaSection(agent *models.Agent) (string, error) {
	// Use the existing ExportHelper for proper schema generation
	helper := schema.NewExportHelper()
	return helper.GenerateInputSchemaSection(agent)
}

// extractCustomVariableNames extracts variable names from input schema JSON
func (s *AgentExportService) extractCustomVariableNames(agent *models.Agent) []string {
	var varNames []string
	
	if agent.InputSchema == nil || *agent.InputSchema == "" {
		return varNames
	}
	
	// Parse the JSON schema to extract variable names
	var schemaMap map[string]interface{}
	if err := json.Unmarshal([]byte(*agent.InputSchema), &schemaMap); err != nil {
		return varNames
	}
	
	// Extract all keys except userInput
	for key := range schemaMap {
		if key != "userInput" {
			varNames = append(varNames, key)
		}
	}
	
	return varNames
}

// generateOutputSchemaSection generates the output schema section for dotprompt
func (s *AgentExportService) generateOutputSchemaSection(agent *models.Agent) string {
	var schemaYAML string
	
	// First, check if there's a preset that needs to be resolved
	if agent.OutputSchemaPreset != nil && *agent.OutputSchemaPreset != "" {
		presetSchema, err := s.schemaRegistry.GetPresetSchema(*agent.OutputSchemaPreset)
		if err != nil {
			log.Printf("Warning: Failed to resolve output schema preset '%s': %v", *agent.OutputSchemaPreset, err)
		} else {
			// Convert JSON schema to GenKit dotprompt YAML format
			schemaYAML = s.convertJSONSchemaToYAML(presetSchema)
		}
	}
	
	// If no preset or preset failed, check for direct output schema
	if schemaYAML == "" && agent.OutputSchema != nil && *agent.OutputSchema != "" {
		schemaYAML = s.convertJSONSchemaToYAML(*agent.OutputSchema)
	}
	
	// If no output schema at all, return empty
	if schemaYAML == "" {
		return ""
	}
	
	// Format the output schema for GenKit dotprompt YAML frontmatter
	return fmt.Sprintf("\noutput:\n  schema:\n%s", s.indentLines(schemaYAML, "    "))
}

// indentJSON indents JSON string with the specified prefix
func (s *AgentExportService) indentJSON(jsonStr, prefix string) string {
	var result string
	for _, line := range splitLines(jsonStr) {
		if line != "" {
			result += prefix + line + "\n"
		} else {
			result += "\n"
		}
	}
	return result
}

// splitLines splits a string into lines
func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	// Add the last line if it doesn't end with \n
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// convertJSONSchemaToYAML converts JSON schema to GenKit dotprompt YAML format
func (s *AgentExportService) convertJSONSchemaToYAML(schemaStr string) string {
	// First check if it's already YAML format (presets)
	if !strings.HasPrefix(strings.TrimSpace(schemaStr), "{") {
		// Already YAML format, return as-is
		return schemaStr
	}
	
	// Parse JSON schema
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		// If JSON parsing fails, return as-is for now
		log.Printf("Warning: Failed to parse JSON schema: %v", err)
		return ""
	}
	
	// Convert to YAML using the yaml library
	yamlBytes, err := yaml.Marshal(schema)
	if err != nil {
		log.Printf("Warning: Failed to convert schema to YAML: %v", err)
		return ""
	}
	
	return string(yamlBytes)
}

// indentLines indents each line with the specified prefix
func (s *AgentExportService) indentLines(text, prefix string) string {
	var result string
	for _, line := range splitLines(text) {
		if line != "" {
			result += prefix + line + "\n"
		} else {
			result += "\n"
		}
	}
	return result
}