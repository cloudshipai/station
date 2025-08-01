package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/spf13/cobra"
	"station/internal/db"
	"station/internal/db/repositories"
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
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🤖 Agents")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Listing agents from: " + endpoint))
		return h.listAgentsRemote(endpoint)
	} else {
		fmt.Println(styles.Info.Render("🏠 Listing local agents"))
		return h.listAgentsLocal()
	}
}

// RunAgentShow shows agent details
func (h *AgentHandler) RunAgentShow(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent ID is required")
	}

	agentID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid agent ID: %s", args[0])
	}

	endpoint, _ := cmd.Flags().GetString("endpoint")

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🤖 Agent Details")
	fmt.Println(banner)

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Getting agent from: " + endpoint))
		return h.showAgentRemote(agentID, endpoint)
	} else {
		fmt.Println(styles.Info.Render("🏠 Getting local agent"))
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
		return fmt.Errorf("invalid agent ID: %s", args[0])
	}

	task := args[1]
	endpoint, _ := cmd.Flags().GetString("endpoint")
	tail, _ := cmd.Flags().GetBool("tail")

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🚀 Run Agent")
	fmt.Println(banner)

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

	agentID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid agent ID: %s", args[0])
	}

	endpoint, _ := cmd.Flags().GetString("endpoint")
	confirm, _ := cmd.Flags().GetBool("confirm")

	if !confirm {
		fmt.Printf("⚠️  This will permanently delete agent %d and all associated data.\n", agentID)
		fmt.Printf("Use --confirm flag to proceed.\n")
		return nil
	}

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🗑️ Delete Agent")
	fmt.Println(banner)

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Deleting agent from: " + endpoint))
		return h.deleteAgentRemote(agentID, endpoint)
	} else {
		fmt.Println(styles.Info.Render("🏠 Deleting local agent"))
		return h.deleteAgentLocal(agentID)
	}
}

// Local operations

func (h *AgentHandler) listAgentsLocal() error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	agents, err := repos.Agents.List()
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	if len(agents) == 0 {
		fmt.Println("• No agents found")
		return nil
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("Found %d agent(s):\n", len(agents))
	for _, agent := range agents {
		fmt.Printf("• %s (ID: %d)", styles.Success.Render(agent.Name), agent.ID)
		if agent.Description != "" {
			fmt.Printf(" - %s", agent.Description)
		}
		fmt.Printf(" [Environment: %d, Max Steps: %d]\n", agent.EnvironmentID, agent.MaxSteps)
	}

	return nil
}

func (h *AgentHandler) showAgentLocal(agentID int64) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	agent, err := repos.Agents.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("agent with ID %d not found", agentID)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("Agent: %s\n", styles.Success.Render(agent.Name))
	fmt.Printf("ID: %d\n", agent.ID)
	fmt.Printf("Description: %s\n", agent.Description)
	fmt.Printf("Environment ID: %d\n", agent.EnvironmentID)
	fmt.Printf("Max Steps: %d\n", agent.MaxSteps)
	if agent.CronSchedule != nil {
		fmt.Printf("Schedule: %s (Enabled: %t)\n", *agent.CronSchedule, agent.ScheduleEnabled)
	}
	fmt.Printf("Created: %s\n", agent.CreatedAt.Format("Jan 2, 2006 15:04"))
	fmt.Printf("Updated: %s\n", agent.UpdatedAt.Format("Jan 2, 2006 15:04"))

	// Show recent runs
	runs, err := repos.AgentRuns.ListByAgent(agentID)
	if err == nil && len(runs) > 0 {
		fmt.Printf("\nRecent runs (%d):\n", len(runs))
		for i, run := range runs {
			if i >= 5 { // Show only last 5 runs
				break
			}
			fmt.Printf("• Run %d: %s [%s]\n", run.ID, run.Status, run.StartedAt.Format("Jan 2 15:04"))
		}
	}

	return nil
}

func (h *AgentHandler) runAgentLocal(agentID int64, task string, tail bool) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	_ = repositories.New(database)
	
	// For local agent execution, we'll need to use the execution service directly
	// For now, let's implement a simpler approach without full Genkit service
	// TODO: Implement proper agent execution service integration

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("🚀 Executing agent %d with task: %s\n", agentID, styles.Info.Render(task))

	if tail {
		fmt.Println(styles.Error.Render("⚠️  Tail mode not yet implemented for local agents"))
		fmt.Println(styles.Info.Render("💡 Use remote endpoint with proper server setup for tail functionality"))
		return nil
	}

	// For now, we'll just show that the agent would be executed
	// TODO: Implement proper local agent execution
	fmt.Printf("⚠️  Local agent execution not yet fully implemented\n")
	fmt.Printf("💡 Use remote endpoint with a running Station server for agent execution\n")

	return nil
}

func (h *AgentHandler) deleteAgentLocal(agentID int64) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	
	// Get agent name for confirmation
	agent, err := repos.Agents.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("agent with ID %d not found", agentID)
	}

	err = repos.Agents.Delete(agentID)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("✅ Agent deleted: %s\n", styles.Success.Render(agent.Name))
	return nil
}

// Remote operations

func (h *AgentHandler) listAgentsRemote(endpoint string) error {
	url := fmt.Sprintf("%s/api/v1/agents", endpoint)
	
	req, err := makeAuthenticatedRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Agents []*models.Agent `json:"agents"`
		Count  int             `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Count == 0 {
		fmt.Println("• No agents found")
		return nil
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("Found %d agent(s):\n", result.Count)
	for _, agent := range result.Agents {
		fmt.Printf("• %s (ID: %d)", styles.Success.Render(agent.Name), agent.ID)
		if agent.Description != "" {
			fmt.Printf(" - %s", agent.Description)
		}
		fmt.Printf(" [Environment: %d, Max Steps: %d]\n", agent.EnvironmentID, agent.MaxSteps)
	}

	return nil
}

func (h *AgentHandler) showAgentRemote(agentID int64, endpoint string) error {
	url := fmt.Sprintf("%s/api/v1/agents/%d", endpoint, agentID)
	
	req, err := makeAuthenticatedRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Agent *models.Agent `json:"agent"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	agent := result.Agent
	styles := getCLIStyles(h.themeManager)
	fmt.Printf("Agent: %s\n", styles.Success.Render(agent.Name))
	fmt.Printf("ID: %d\n", agent.ID)
	fmt.Printf("Description: %s\n", agent.Description)
	fmt.Printf("Environment ID: %d\n", agent.EnvironmentID)
	fmt.Printf("Max Steps: %d\n", agent.MaxSteps)
	if agent.CronSchedule != nil {
		fmt.Printf("Schedule: %s (Enabled: %t)\n", *agent.CronSchedule, agent.ScheduleEnabled)
	}
	fmt.Printf("Created: %s\n", agent.CreatedAt.Format("Jan 2, 2006 15:04"))
	fmt.Printf("Updated: %s\n", agent.UpdatedAt.Format("Jan 2, 2006 15:04"))

	// Get recent runs for this agent
	runsURL := fmt.Sprintf("%s/api/v1/runs/agent/%d", endpoint, agentID)
	runsReq, err := makeAuthenticatedRequest(http.MethodGet, runsURL, nil)
	if err == nil {
		client := &http.Client{}
		runsResp, err := client.Do(runsReq)
		if err == nil && runsResp.StatusCode == http.StatusOK {
			defer runsResp.Body.Close()
			var runsResult struct {
				Runs  []*models.AgentRun `json:"runs"`
				Count int                `json:"count"`
			}
			if json.NewDecoder(runsResp.Body).Decode(&runsResult) == nil && len(runsResult.Runs) > 0 {
				fmt.Printf("\nRecent runs (%d):\n", runsResult.Count)
				for i, run := range runsResult.Runs {
					if i >= 5 { // Show only last 5 runs
						break
					}
					fmt.Printf("• Run %d: %s [%s]\n", run.ID, run.Status, run.StartedAt.Format("Jan 2 15:04"))
				}
			}
		}
	}

	return nil
}

func (h *AgentHandler) runAgentRemote(agentID int64, task string, endpoint string, tail bool) error {
	runRequest := struct {
		Task string `json:"task"`
	}{
		Task: task,
	}

	jsonData, err := json.Marshal(runRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/agents/%d/execute", endpoint, agentID)
	req, err := makeAuthenticatedRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{}
	
	styles := getCLIStyles(h.themeManager)
	fmt.Printf("🚀 Executing agent %d with task: %s\n", agentID, styles.Info.Render(task))

	if tail {
		// For remote tail, we'll need to implement polling or WebSocket
		// For now, we'll do a simple execution and show result
		fmt.Println(styles.Error.Render("⚠️  Tail mode not yet implemented for remote agents"))
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AgentID  int64  `json:"agent_id"`
		Task     string `json:"task"`
		Response string `json:"response"`
		Success  bool   `json:"success"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Printf("✅ Agent execution completed\n")
	fmt.Printf("Response: %s\n", result.Response)

	return nil
}

func (h *AgentHandler) deleteAgentRemote(agentID int64, endpoint string) error {
	url := fmt.Sprintf("%s/api/v1/agents/%d", endpoint, agentID)
	req, err := makeAuthenticatedRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("✅ Agent deleted: %s\n", styles.Success.Render(fmt.Sprintf("ID %d", agentID)))

	return nil
}

// runAgentWithTail provides a simple TUI for watching agent execution
func (h *AgentHandler) runAgentWithTail(agentID int64, task string) error {
	styles := getCLIStyles(h.themeManager)
	
	// Start execution in background
	fmt.Println(styles.Info.Render("📡 Starting agent execution..."))
	fmt.Println(styles.Info.Render("💡 Press Ctrl+C to exit tail mode"))
	fmt.Println()

	// TODO: Implement real-time tail functionality with run status polling
	// This would require:
	// 1. Starting execution asynchronously
	// 2. Polling run status and logs
	// 3. Real-time display updates
	// 4. Proper signal handling for Ctrl+C
	
	fmt.Printf("⚠️  Tail functionality not yet implemented\n")
	fmt.Printf("Agent: %d, Task: %s\n", agentID, task)

	return nil
}