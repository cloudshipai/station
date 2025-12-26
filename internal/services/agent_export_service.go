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

func (s *AgentExportService) ExportAgentAfterSaveWithMetadata(agentID int64, app, appType string) error {
	return s.ExportAgentWithSandbox(agentID, app, appType, "")
}

func (s *AgentExportService) ExportAgentWithSandbox(agentID int64, app, appType, sandboxConfig string) error {
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %v", err)
	}

	if app == "" && agent.App != "" {
		app = agent.App
	}
	if appType == "" && agent.AppType != "" {
		appType = agent.AppType
	}

	if app == "" && appType == "" && agent.OutputSchemaPreset != nil && *agent.OutputSchemaPreset != "" {
		if presetInfo, exists := s.schemaRegistry.GetPresetInfo(*agent.OutputSchemaPreset); exists {
			app = presetInfo.App
			appType = presetInfo.AppType
		}
	}

	environment, err := s.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %v", err)
	}

	toolsWithDetails, err := s.repos.AgentTools.ListAgentTools(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent tools: %v", err)
	}

	childAgents, err := s.repos.AgentAgents.ListChildAgents(agentID)
	if err != nil {
		return fmt.Errorf("failed to get child agents: %v", err)
	}

	dotpromptContent := s.generateDotpromptContentWithSandbox(agent, toolsWithDetails, childAgents, environment.Name, app, appType, sandboxConfig)

	outputPath := config.GetAgentPromptPath(environment.Name, agent.Name)

	agentsDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %v", err)
	}

	if err := os.WriteFile(outputPath, []byte(dotpromptContent), 0644); err != nil {
		return fmt.Errorf("failed to write .prompt file: %v", err)
	}

	log.Printf("Agent '%s' (ID: %d) successfully exported to: %s", agent.Name, agent.ID, outputPath)
	return nil
}

func (s *AgentExportService) generateDotpromptContent(agent *models.Agent, tools []*models.AgentToolWithDetails, childAgents []*models.ChildAgent, environmentName, app, appType string) string {
	return s.generateDotpromptContentWithSandbox(agent, tools, childAgents, environmentName, app, appType, "")
}

func (s *AgentExportService) generateDotpromptContentWithSandbox(agent *models.Agent, tools []*models.AgentToolWithDetails, childAgents []*models.ChildAgent, environmentName, app, appType, sandboxConfig string) string {
	var toolNames []string
	for _, tool := range tools {
		toolNames = append(toolNames, tool.ToolName)
	}

	content := fmt.Sprintf(`---
metadata:
  name: "%s"
  description: "%s"
  tags: ["station", "agent"]`, agent.Name, agent.Description)

	if app != "" || appType != "" {
		if app != "" {
			content += fmt.Sprintf("\n  app: \"%s\"", app)
		}
		if appType != "" {
			content += fmt.Sprintf("\n  app_type: \"%s\"", appType)
		}
	}

	content += fmt.Sprintf("\nmodel: gpt-5-mini\nmax_steps: %d", agent.MaxSteps)

	if agent.MemoryTopicKey != nil && *agent.MemoryTopicKey != "" {
		content += "\ncloudshipai:"
		content += fmt.Sprintf("\n  memory: \"%s\"", *agent.MemoryTopicKey)
		if agent.MemoryMaxTokens != nil && *agent.MemoryMaxTokens > 0 {
			content += fmt.Sprintf("\n  memory_max_tokens: %d", *agent.MemoryMaxTokens)
		}
	}

	if sandboxConfig != "" {
		sandboxYAML := s.convertSandboxJSONToYAML(sandboxConfig)
		if sandboxYAML != "" {
			content += "\nsandbox:\n" + s.indentLines(sandboxYAML, "  ")
		}
	}

	if len(toolNames) > 0 {
		content += "\ntools:\n"
		for _, toolName := range toolNames {
			content += fmt.Sprintf("  - \"%s\"\n", toolName)
		}
	}

	if len(childAgents) > 0 {
		content += "\nagents:\n"
		for _, childRel := range childAgents {
			content += fmt.Sprintf("  - \"%s\"\n", childRel.ChildAgent.Name)
		}
	}

	inputSchemaSection, err := s.generateInputSchemaSection(agent)
	if err == nil {
		content += "\n" + inputSchemaSection
	}

	outputSchemaSection := s.generateOutputSchemaSection(agent)
	if outputSchemaSection != "" {
		content += outputSchemaSection
	}

	content += "---\n\n"

	if strings.Contains(agent.Prompt, "{{role") {
		content += agent.Prompt
	} else {
		content += "{{role \"system\"}}\n"
		content += agent.Prompt
		content += "\n\n"

		content += "{{role \"user\"}}\n"
		content += "{{userInput}}"
	}

	if agent.InputSchema != nil && *agent.InputSchema != "" {
		customVars := s.extractCustomVariableNames(agent)
		for _, varName := range customVars {
			content += fmt.Sprintf("\n\n**%s:** {{%s}}", varName, varName)
		}
	}

	return content
}

func (s *AgentExportService) convertSandboxJSONToYAML(sandboxJSON string) string {
	if sandboxJSON == "" || sandboxJSON == "{}" {
		return ""
	}

	var sandboxMap map[string]interface{}
	if err := json.Unmarshal([]byte(sandboxJSON), &sandboxMap); err != nil {
		log.Printf("Warning: Failed to parse sandbox JSON: %v", err)
		return ""
	}

	if len(sandboxMap) == 0 {
		return ""
	}

	yamlBytes, err := yaml.Marshal(sandboxMap)
	if err != nil {
		log.Printf("Warning: Failed to convert sandbox to YAML: %v", err)
		return ""
	}

	return strings.TrimSpace(string(yamlBytes))
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
	// Add the last line (including empty string if ends with \n)
	if start <= len(s) {
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
