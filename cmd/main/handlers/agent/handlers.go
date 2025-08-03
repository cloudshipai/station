package agent

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"station/cmd/main/handlers/common"
	"station/internal/theme"
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
	envFilter, _ := cmd.Flags().GetString("env")

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Listing agents from: " + endpoint))
		return h.listAgentsRemote(endpoint)
	} else {
		if envFilter != "" {
			fmt.Println(styles.Info.Render(fmt.Sprintf("🏠 Listing local agents (Environment: %s)", envFilter)))
		} else {
			fmt.Println(styles.Info.Render("🏠 Listing local agents"))
		}
		return h.listAgentsLocalWithFilter(envFilter)
	}
}

// RunAgentShow shows details of a specific agent
func (h *AgentHandler) RunAgentShow(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent ID is required")
	}

	agentID, err := common.ParseIDFromString(args[0])
	if err != nil {
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render(fmt.Sprintf("🤖 Agent #%d", agentID))
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Showing agent from: " + endpoint))
		return h.showAgentRemote(agentID, endpoint)
	} else {
		fmt.Println(styles.Info.Render("🏠 Showing local agent"))
		return h.showAgentLocal(agentID)
	}
}

// RunAgentRun executes an agent
func (h *AgentHandler) RunAgentRun(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("agent ID and task are required")
	}

	agentID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	task := args[1]
	tail, _ := cmd.Flags().GetBool("tail")

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render(fmt.Sprintf("▶️  Running Agent #%d", agentID))
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Running agent on: " + endpoint))
		return h.runAgentRemote(agentID, task, endpoint, tail)
	} else {
		fmt.Println(styles.Info.Render("🏠 Running local agent"))
		return h.runAgentLocal(agentID, task, tail)
	}
}

// RunAgentDelete deletes an agent
func (h *AgentHandler) RunAgentDelete(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent ID is required")
	}

	agentID, err := common.ParseIDFromString(args[0])
	if err != nil {
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render(fmt.Sprintf("🗑️  Delete Agent #%d", agentID))
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint != "" {
		fmt.Println(styles.Error.Render("❌ Deleting remote agents is not currently supported"))
		return fmt.Errorf("remote agent deletion not supported")
	} else {
		fmt.Println(styles.Info.Render("🏠 Deleting local agent"))
		return h.deleteAgentLocal(agentID)
	}
}

// RunAgentCreate creates a new agent
func (h *AgentHandler) RunAgentCreate(cmd *cobra.Command, args []string) error {
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("➕ Create Agent")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")
	domain, _ := cmd.Flags().GetString("domain")
	schedule, _ := cmd.Flags().GetString("schedule")
	environment, _ := cmd.Flags().GetString("environment")

	if endpoint != "" {
		fmt.Println(styles.Error.Render("❌ Creating remote agents is not currently supported"))
		return fmt.Errorf("remote agent creation not supported")
	} else {
		fmt.Println(styles.Info.Render("🏠 Creating local agent"))
		return h.createAgentLocal(name, description, domain, schedule, environment)
	}
}