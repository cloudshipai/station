package agent

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
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