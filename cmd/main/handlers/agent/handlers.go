package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"station/cmd/main/handlers/common"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/theme"
	"station/pkg/models"
)

// AgentHandler handles agent-related CLI commands
type AgentHandler struct {
	themeManager *theme.ThemeManager
}

func NewAgentHandler(themeManager *theme.ThemeManager) *AgentHandler {
	return &AgentHandler{themeManager: themeManager}
}

// RunAgentList lists all agents
func (h *AgentHandler) RunAgentList(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🤖 Agents")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")

	if endpoint != "" {
		return h.listAgentsRemote(endpoint)
	} else {
		return h.listAgentsLocalWithFilter(environment)
	}
}

// RunAgentShow shows details of a specific agent
func (h *AgentHandler) RunAgentShow(cmd *cobra.Command, args []string) error {
	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")

	if len(args) != 1 {
		return fmt.Errorf("usage: stn agent show <agent_name>")
	}

	agentName := args[0]

	if endpoint != "" {
		agentID, err := strconv.ParseInt(agentName, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid agent ID: %v", err)
		}
		return h.showAgentRemote(agentID, endpoint)
	} else {
		return h.showAgentLocalByName(agentName, environment)
	}
}

// showAgentLocalByName shows details of an agent by name and environment
func (h *AgentHandler) showAgentLocalByName(agentName, environment string) error {
	styles := getCLIStyles(h.themeManager)

	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load station config: %v", err)
	}

	database, err := db.NewSQLite(cfg.GetDatabasePath())
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
	fmt.Printf("%s📋 Agent Details%s\n", styles.Primary, styles.Reset)
	fmt.Printf("%sName:%s %s\n", styles.Info, styles.Reset, targetAgent.Name)
	fmt.Printf("%sDescription:%s %s\n", styles.Info, styles.Reset, targetAgent.Description)
	fmt.Printf("%sEnvironment:%s %s\n", styles.Info, styles.Reset, environmentName)
	fmt.Printf("%sMax Steps:%s %d\n", styles.Info, styles.Reset, targetAgent.MaxSteps)
	fmt.Printf("%sCreated:%s %s\n", styles.Info, styles.Reset, targetAgent.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("%sUpdated:%s %s\n", styles.Info, styles.Reset, targetAgent.UpdatedAt.Format("2006-01-02 15:04:05"))

	if len(tools) > 0 {
		fmt.Printf("\n%s🛠 Tools (%d):%s\n", styles.Info, len(tools), styles.Reset)
		for _, tool := range tools {
			fmt.Printf("  • %s\n", tool.ToolName)
			if tool.Description != "" {
				fmt.Printf("    %s\n", tool.Description)
			}
		}
	}

	fmt.Printf("\n%s📝 Prompt:%s\n%s\n", styles.Primary, styles.Reset, targetAgent.Prompt)
	return nil
}

// RunAgentDelete deletes an agent
func (h *AgentHandler) RunAgentDelete(cmd *cobra.Command, args []string) error {
	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")

	if len(args) != 1 {
		return fmt.Errorf("usage: stn agent delete <agent_name>")
	}

	agentName := args[0]

	if endpoint != "" {
		agentID, err := strconv.ParseInt(agentName, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid agent ID: %v", err)
		}
		return h.deleteAgentRemote(agentID, endpoint)
	} else {
		return h.deleteAgentLocalByName(agentName, environment)
	}
}

// deleteAgentLocalByName deletes an agent by name and environment
func (h *AgentHandler) deleteAgentLocalByName(agentName, environment string) error {
	styles := getCLIStyles(h.themeManager)

	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load station config: %v", err)
	}

	database, err := db.NewSQLite(cfg.GetDatabasePath())
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
	fmt.Printf("%s⚠ Are you sure you want to delete agent '%s'? [y/N]: %s", 
		styles.Warning, targetAgent.Name, styles.Reset)
	
	var response string
	fmt.Scanln(&response)
	
	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		fmt.Printf("%s❌ Deletion cancelled%s\n", styles.Info, styles.Reset)
		return nil
	}

	// Delete the agent
	err = agentService.DeleteAgent(context.Background(), targetAgent.ID)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %v", err)
	}

	fmt.Printf("%s✅ Agent '%s' deleted successfully%s\n", 
		styles.Success, targetAgent.Name, styles.Reset)
	return nil
}