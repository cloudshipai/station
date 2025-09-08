package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"station/cmd/main/handlers/common"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/telemetry"
	"station/internal/theme"
	"station/pkg/models"
)

// AgentHandler handles agent-related CLI commands
type AgentHandler struct {
	themeManager     *theme.ThemeManager
	telemetryService *telemetry.TelemetryService
}

func NewAgentHandler(themeManager *theme.ThemeManager, telemetryService *telemetry.TelemetryService) *AgentHandler {
	return &AgentHandler{
		themeManager:     themeManager,
		telemetryService: telemetryService,
	}
}

// RunAgentList lists all agents
func (h *AgentHandler) RunAgentList(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🤖 Agents")
	fmt.Println(banner)

	environment, _ := cmd.Flags().GetString("environment")
	err := h.listAgentsLocalWithFilter(environment)
	
	// Track telemetry
	if h.telemetryService != nil {
		h.telemetryService.TrackCLICommand("agent", "list", err == nil, time.Since(startTime).Milliseconds())
	}
	
	return err
}

// RunAgentShow shows details of a specific agent
func (h *AgentHandler) RunAgentShow(cmd *cobra.Command, args []string) error {
	environment, _ := cmd.Flags().GetString("environment")

	if len(args) != 1 {
		return fmt.Errorf("usage: stn agent show <agent_name>")
	}

	agentName := args[0]
	return h.showAgentLocalByName(agentName, environment)
}

// showAgentLocalByName shows details of an agent by name and environment
func (h *AgentHandler) showAgentLocalByName(agentName, environment string) error {
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
	var environmentName string

	for _, agent := range agents {
		if agent.Name == agentName {
			env, err := repos.Environments.GetByID(agent.EnvironmentID)
			if err != nil {
				continue
			}

			// Filter by environment if specified
			if environment != "" && env.Name != environment {
				continue
			}

			targetAgent = agent
			environmentName = env.Name
			break
		}
	}

	if targetAgent == nil {
		if environment != "" {
			return fmt.Errorf("agent '%s' not found in environment '%s'", agentName, environment)
		}
		return fmt.Errorf("agent '%s' not found", agentName)
	}

	// Get agent tools
	tools, err := repos.AgentTools.ListAgentTools(targetAgent.ID)
	if err != nil {
		return fmt.Errorf("failed to get agent tools: %v", err)
	}

	// Display agent details
	fmt.Printf("📋 Agent Details\n")
	fmt.Printf("Name: %s\n", targetAgent.Name)
	fmt.Printf("Description: %s\n", targetAgent.Description)
	fmt.Printf("Environment: %s\n", environmentName)
	fmt.Printf("Max Steps: %d\n", targetAgent.MaxSteps)
	fmt.Printf("Created: %s\n", targetAgent.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated: %s\n", targetAgent.UpdatedAt.Format("2006-01-02 15:04:05"))

	if len(tools) > 0 {
		fmt.Printf("\n🛠 Tools (%d):\n", len(tools))
		for _, tool := range tools {
			fmt.Printf("  • %s\n", tool.ToolName)
			if tool.ToolDescription != "" {
				fmt.Printf("    %s\n", tool.ToolDescription)
			}
		}
	}

	fmt.Printf("\n📝 Prompt:\n%s\n", targetAgent.Prompt)
	return nil
}

// RunAgentDelete deletes an agent
func (h *AgentHandler) RunAgentDelete(cmd *cobra.Command, args []string) error {
	environment, _ := cmd.Flags().GetString("environment")

	if len(args) != 1 {
		return fmt.Errorf("usage: stn agent delete <agent_name>")
	}

	agentName := args[0]
	return h.deleteAgentLocalByName(agentName, environment)
}

// deleteAgentLocalByName deletes an agent by name and environment
func (h *AgentHandler) deleteAgentLocalByName(agentName, environment string) error {
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
	for _, agent := range agents {
		if agent.Name == agentName {
			// Get environment name if filtering by environment
			if environment != "" {
				env, err := repos.Environments.GetByID(agent.EnvironmentID)
				if err != nil {
					continue
				}
				if env.Name != environment {
					continue
				}
			}
			targetAgent = agent
			break
		}
	}

	if targetAgent == nil {
		return fmt.Errorf("agent '%s' not found", agentName)
	}

	// Confirm deletion
	fmt.Printf("⚠ Are you sure you want to delete agent '%s'? [y/N]: ", targetAgent.Name)
	
	var response string
	fmt.Scanln(&response)
	
	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		fmt.Printf("❌ Deletion cancelled\n")
		return nil
	}

	// Delete the agent
	err = agentService.DeleteAgent(context.Background(), targetAgent.ID)
	if err != nil {
		// Track failed deletion
		if h.telemetryService != nil {
			h.telemetryService.TrackError("agent_deletion_failed", err.Error(), map[string]interface{}{
				"agent_name": targetAgent.Name,
				"agent_id":   targetAgent.ID,
			})
		}
		return fmt.Errorf("failed to delete agent: %v", err)
	}

	// Track successful deletion
	if h.telemetryService != nil {
		h.telemetryService.TrackEvent("stn_agent_deleted", map[string]interface{}{
			"agent_id":      targetAgent.ID,
			"environment_id": targetAgent.EnvironmentID,
		})
	}

	fmt.Printf("✅ Agent '%s' deleted successfully\n", targetAgent.Name)
	return nil
}