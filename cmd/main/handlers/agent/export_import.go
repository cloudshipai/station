package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/models"
)

// RunAgentExport exports an agent to a .prompt file
func (h *AgentHandler) RunAgentExport(cmd *cobra.Command, args []string) error {
	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")

	// Validate arguments
	if len(args) != 1 {
		return fmt.Errorf("usage: stn agent export <agent_name>")
	}

	agentName := args[0]

	if endpoint != "" {
		return fmt.Errorf("remote agent export not yet implemented")
	} else {
		return h.exportAgentLocalByName(agentName, environment)
	}
}

// exportAgentLocalByName exports an agent from the local database
func (h *AgentHandler) exportAgentLocalByName(agentName, environment string) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load station config: %v", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	agentService := services.NewAgentService(repos)

	// Find agent by name and environment
	agents, err := agentService.ListAgentsByEnvironment(context.Background(), 0)
	if err != nil {
		return fmt.Errorf("failed to list agents: %v", err)
	}

	var targetAgent *models.Agent
	var targetEnvironment string
	for _, agent := range agents {
		if agent.Name == agentName {
			// Get environment name
			env, err := repos.Environments.GetByID(agent.EnvironmentID)
			if err != nil {
				continue
			}
			
			// Filter by environment if specified
			if environment != "" && env.Name != environment {
				continue
			}
			
			targetAgent = agent
			targetEnvironment = env.Name
			break
		}
	}

	if targetAgent == nil {
		return fmt.Errorf("agent '%s' not found", agentName)
	}

	// Determine output file path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %v", err)
	}

	promptFilePath := fmt.Sprintf("%s/.config/station/environments/%s/agents/%s.prompt", 
		homeDir, targetEnvironment, targetAgent.Name)

	return h.exportAgentFromDatabase(targetAgent.Name, targetEnvironment, promptFilePath)
}

// exportAgentFromDatabase exports an agent from the database to a .prompt file
func (h *AgentHandler) exportAgentFromDatabase(agentName, environment, promptFilePath string) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load station config: %v", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer database.Close()

	repos := repositories.New(database)

	// Get agent details
	agents, err := repos.Agents.List()
	if err != nil {
		return fmt.Errorf("failed to list agents: %v", err)
	}

	var targetAgent *models.Agent
	for _, agent := range agents {
		if agent.Name == agentName {
			env, err := repos.Environments.GetByID(agent.EnvironmentID)
			if err != nil {
				continue
			}
			if env.Name == environment {
				targetAgent = agent
				break
			}
		}
	}

	if targetAgent == nil {
		return fmt.Errorf("agent '%s' not found in environment '%s'", agentName, environment)
	}

	// Get agent tools with details
	toolsWithDetails, err := repos.AgentTools.ListAgentTools(targetAgent.ID)
	if err != nil {
		return fmt.Errorf("failed to get agent tools: %v", err)
	}

	// Generate dotprompt content
	dotpromptContent := h.generateDotpromptContent(targetAgent, toolsWithDetails, environment)

	// Ensure directory exists
	agentsDir := filepath.Dir(promptFilePath)
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %v", err)
	}

	// Write .prompt file to filesystem
	if err := os.WriteFile(promptFilePath, []byte(dotpromptContent), 0644); err != nil {
		return fmt.Errorf("failed to write .prompt file: %v", err)
	}

	fmt.Printf("✅ Agent '%s' successfully exported to: %s\n", 
		agentName, promptFilePath)
	
	return nil
}

// generateDotpromptContent generates the dotprompt file content
func (h *AgentHandler) generateDotpromptContent(agent *models.Agent, tools []*models.AgentToolWithDetails, environment string) string {
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

%s`, agent.MaxSteps, environment, agent.ID, agent.Prompt)

	return frontmatter
}

// isMultiRolePrompt checks if a prompt contains multiple role definitions
func (h *AgentHandler) isMultiRolePrompt(prompt string) bool {
	return strings.Contains(prompt, "role:") || strings.Contains(prompt, "Role:")
}

// RunAgentImport imports agents from .prompt files
func (h *AgentHandler) RunAgentImport(cmd *cobra.Command, args []string) error {
	environment, _ := cmd.Flags().GetString("environment")

	if environment == "" {
		environment = "default"
	}

	fmt.Printf("🔄 Importing agents from environment '%s'...\n", environment)

	return h.importAgentsLocal(environment)
}