package agent

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
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
			// If not a number, try to find by name (this would need more complex logic)
			return fmt.Errorf("agent execution by name not yet implemented in modular version, use ID")
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