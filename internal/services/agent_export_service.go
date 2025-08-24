package services

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"station/internal/db/repositories"
	"station/pkg/models"
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

// generateDotpromptContent generates the dotprompt file content
// This is extracted from the MCP handler logic to be reusable
func (s *AgentExportService) generateDotpromptContent(agent *models.Agent, tools []*models.AgentToolWithDetails, environmentName string) string {
	// Build tools list
	var toolNames []string
	for _, tool := range tools {
		toolNames = append(toolNames, tool.ToolName)
	}

	// Generate YAML frontmatter
	frontmatter := fmt.Sprintf(`---
model: "gemini-2.0-flash-exp"
config:
  temperature: 0.3
  max_tokens: 2000
metadata:
  name: "%s"
  description: "%s"
  version: "1.0.0"`, agent.Name, agent.Description)

	// Add tools if any
	if len(toolNames) > 0 {
		frontmatter += "\ntools:\n"
		for _, toolName := range toolNames {
			frontmatter += fmt.Sprintf("  - \"%s\"\n", toolName)
		}
	}

	// Add station-specific metadata
	frontmatter += fmt.Sprintf(`station:
  execution_metadata:
    max_steps: %d
    environment: "%s"
    agent_id: %d
---

%s`, agent.MaxSteps, environmentName, agent.ID, agent.Prompt)

	return frontmatter
}