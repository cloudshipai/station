package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"station/internal/db/repositories"
	"station/pkg/models"
	"station/pkg/schema"
)

// AgentExportService handles automatic export of agents to file-based config
type AgentExportService struct {
	repos *repositories.Repositories
}

// NewAgentExportService creates a new agent export service
func NewAgentExportService(repos *repositories.Repositories) *AgentExportService {
	return &AgentExportService{
		repos: repos,
	}
}

// ExportAgentAfterSave automatically exports an agent to file-based config after DB save
func (s *AgentExportService) ExportAgentAfterSave(agentID int64) error {
	// Get agent details
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %v", err)
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
	dotpromptContent := s.generateDotpromptContent(agent, toolsWithDetails, environment.Name)

	// Determine output file path
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %v", err)
		}
	}
	outputPath := fmt.Sprintf("%s/.config/station/environments/%s/agents/%s.prompt", homeDir, environment.Name, agent.Name)

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
func (s *AgentExportService) generateDotpromptContent(agent *models.Agent, tools []*models.AgentToolWithDetails, environmentName string) string {
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
  tags: ["station", "agent"]
model: gpt-4o-mini
max_steps: %d`, agent.Name, agent.Description, agent.MaxSteps)

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
		content += inputSchemaSection
	}

	// Close frontmatter and add role-based prompt structure
	content += "---\n\n"

	// Add system role with the agent's prompt
	content += "{{role \"system\"}}\n"
	content += agent.Prompt
	content += "\n\n"

	// Add user role with handlebars template
	content += "{{role \"user\"}}\n"
	content += "{{userInput}}"

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