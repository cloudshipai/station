package agent

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// RunAgentRun executes an agent with a given task
// This delegates to existing working implementations
func (h *AgentHandler) RunAgentRun(cmd *cobra.Command, args []string) error {
	endpoint, _ := cmd.Flags().GetString("endpoint")
	tail, _ := cmd.Flags().GetBool("tail")

	// Validate arguments
	if len(args) != 2 {
		return fmt.Errorf("usage: stn agent run <agent_name> <task>")
	}

	agentName := args[0]
	task := args[1]

	if endpoint != "" {
		// Remote execution
		agentID, err := strconv.ParseInt(agentName, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid agent ID: %v", err)
		}
		
		return h.runAgentRemote(agentID, task, endpoint, tail)
	} else {
		// Local execution - delegate to the existing working function
		agentID, err := strconv.ParseInt(agentName, 10, 64)
		if err != nil {
			// Not a number, try to find by name
			agentID, err = h.findAgentByName(agentName, cmd)
			if err != nil {
				return fmt.Errorf("failed to find agent '%s': %v", agentName, err)
			}
		}
		
		return h.runAgentLocal(agentID, task, tail)
	}
}

// displayExecutionResults shows the results of an agent execution
func (h *AgentHandler) displayExecutionResults(run *models.AgentRun) error {
	styles := getCLIStyles(h.themeManager)
	
	fmt.Printf("📋 %s\n", styles.Success.Render("Execution Results"))
	fmt.Printf("Run ID: %d\n", run.ID)
	fmt.Printf("Status: %s\n", run.Status)
	fmt.Printf("Started: %s\n", run.StartedAt.Format(time.RFC3339))
	
	if run.CompletedAt != nil {
		duration := run.CompletedAt.Sub(run.StartedAt)
		fmt.Printf("Completed: %s (took %s)\n", 
			run.CompletedAt.Format(time.RFC3339), duration)
	}
	
	if run.Status == "failed" || run.Status == "error" {
		fmt.Printf("Error: %s\n", styles.Error.Render(run.FinalResponse))
		return nil
	}
	
	if run.FinalResponse != "" {
		fmt.Printf("\nResult:\n%s\n", run.FinalResponse)
	}
	
	// Show token usage if available
	if run.InputTokens != nil || run.OutputTokens != nil {
		fmt.Printf("\nToken Usage:\n")
		if run.InputTokens != nil {
			fmt.Printf("  Input tokens: %d\n", *run.InputTokens)
		}
		if run.OutputTokens != nil {
			fmt.Printf("  Output tokens: %d\n", *run.OutputTokens)
		}
	}
	
	// Show tool calls if available
	if run.ToolCalls != nil && len(*run.ToolCalls) > 0 {
		fmt.Printf("\nTool Calls: %d\n", len(*run.ToolCalls))
	}
	
	return nil
}

// findAgentByName finds an agent by name, optionally filtering by environment
func (h *AgentHandler) findAgentByName(agentName string, cmd *cobra.Command) (int64, error) {
	environment, _ := cmd.Flags().GetString("env")
	
	cfg, err := loadStationConfig()
	if err != nil {
		return 0, fmt.Errorf("failed to load station config: %v", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return 0, fmt.Errorf("failed to open database: %v", err)
	}
	defer database.Close()

	repos := repositories.New(database)

	// List all agents and filter by name and environment
	agents, err := repos.Agents.List()
	if err != nil {
		return 0, fmt.Errorf("failed to list agents: %v", err)
	}

	var targetAgent *models.Agent

	for _, agent := range agents {
		if agent.Name == agentName {
			if environment != "" {
				env, err := repos.Environments.GetByID(agent.EnvironmentID)
				if err != nil {
					continue
				}
				// Filter by environment if specified
				if env.Name != environment {
					continue
				}
			}
			targetAgent = agent
			break
		}
	}

	if targetAgent == nil {
		envFilter := ""
		if environment != "" {
			envFilter = fmt.Sprintf(" in environment '%s'", environment)
		}
		return 0, fmt.Errorf("agent '%s' not found%s", agentName, envFilter)
	}

	return targetAgent.ID, nil
}