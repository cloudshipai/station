package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/theme"
	"station/pkg/crypto"
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

// RunAgentCreate creates a new intelligent agent
func (h *AgentHandler) RunAgentCreate(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("agent name and description are required")
	}

	name := args[0]
	description := args[1]
	
	endpoint, _ := cmd.Flags().GetString("endpoint")
	domain, _ := cmd.Flags().GetString("domain")
	schedule, _ := cmd.Flags().GetString("schedule")

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🧠 Create Intelligent Agent")
	fmt.Println(banner)

	// Show what we're creating
	fmt.Println(styles.Info.Render("📝 Agent Configuration:"))
	fmt.Printf("  Name: %s\n", name)
	fmt.Printf("  Description: %s\n", description)
	if domain != "" {
		fmt.Printf("  Domain: %s\n", domain)
	}
	fmt.Printf("  Schedule: %s\n", schedule)
	fmt.Println()

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Creating agent on: " + endpoint))
		return h.createAgentRemote(name, description, domain, schedule, endpoint)
	} else {
		fmt.Println(styles.Info.Render("🏠 Creating local intelligent agent"))
		return h.createAgentLocal(name, description, domain, schedule)
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

	// Get environment names for better display
	environments := make(map[int64]string)
	envs, err := repos.Environments.List()
	if err == nil {
		for _, env := range envs {
			environments[env.ID] = env.Name
		}
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("Found %d agent(s):\n", len(agents))
	for _, agent := range agents {
		envName := environments[agent.EnvironmentID]
		if envName == "" {
			envName = fmt.Sprintf("ID:%d", agent.EnvironmentID)
		}
		
		fmt.Printf("• %s (ID: %d)", styles.Success.Render(agent.Name), agent.ID)
		if agent.Description != "" {
			fmt.Printf(" - %s", agent.Description)
		}
		fmt.Printf(" [Environment: %s, Max Steps: %d]\n", envName, agent.MaxSteps)
	}

	return nil
}

func (h *AgentHandler) listAgentsLocalWithFilter(envFilter string) error {
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
	
	// Get all agents
	agents, err := repos.Agents.List()
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	// Get environment names for filtering and display
	environments := make(map[int64]string)
	envNameToID := make(map[string]int64)
	envs, err := repos.Environments.List()
	if err == nil {
		for _, env := range envs {
			environments[env.ID] = env.Name
			envNameToID[env.Name] = env.ID
		}
	}

	// Filter agents by environment if specified
	var filteredAgents []*models.Agent
	if envFilter != "" {
		// Try to parse envFilter as environment ID or name
		var targetEnvID int64 = -1
		
		// Try as ID first
		if envID, err := strconv.ParseInt(envFilter, 10, 64); err == nil {
			targetEnvID = envID
		} else if envID, exists := envNameToID[envFilter]; exists {
			// Try as environment name
			targetEnvID = envID
		} else {
			return fmt.Errorf("environment '%s' not found", envFilter)
		}
		
		// Filter agents by environment
		for _, agent := range agents {
			if agent.EnvironmentID == targetEnvID {
				filteredAgents = append(filteredAgents, agent)
			}
		}
	} else {
		filteredAgents = agents
	}

	if len(filteredAgents) == 0 {
		if envFilter != "" {
			fmt.Printf("• No agents found in environment '%s'\n", envFilter)
		} else {
			fmt.Println("• No agents found")
		}
		return nil
	}

	styles := getCLIStyles(h.themeManager)
	if envFilter != "" {
		fmt.Printf("Found %d agent(s) in environment '%s':\n", len(filteredAgents), envFilter)
	} else {
		fmt.Printf("Found %d agent(s):\n", len(filteredAgents))
	}
	
	for _, agent := range filteredAgents {
		envName := environments[agent.EnvironmentID]
		if envName == "" {
			envName = fmt.Sprintf("ID:%d", agent.EnvironmentID)
		}
		
		fmt.Printf("• %s (ID: %d)", styles.Success.Render(agent.Name), agent.ID)
		if agent.Description != "" {
			fmt.Printf(" - %s", agent.Description)
		}
		fmt.Printf(" [Environment: %s, Max Steps: %d]\n", envName, agent.MaxSteps)
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
	styles := getCLIStyles(h.themeManager)
	
	// Load configuration and connect to database
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	repos := repositories.New(database)
	
	// Verify the agent exists
	agent, err := repos.Agents.GetByID(agentID)
	if err != nil {
		database.Close()
		return fmt.Errorf("agent with ID %d not found: %w", agentID, err)
	}
	
	fmt.Printf("🚀 Executing agent '%s' (ID: %d)\n", styles.Success.Render(agent.Name), agentID)
	fmt.Printf("📋 Task: %s\n", styles.Info.Render(task))
	
	// Close database connection before trying server execution to avoid locks
	database.Close()
	
	// Try server first, fall back to stdio MCP self-bootstrapping execution
	if h.tryServerExecution(agentID, task, tail, cfg) == nil {
		return nil
	}
	
	// Server not available, use self-bootstrapping stdio MCP execution
	fmt.Printf("💡 Server not available, using self-bootstrapping stdio MCP execution\n\n")
	return h.runAgentWithStdioMCP(agentID, task, tail, cfg, agent)
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

// tryServerExecution attempts to execute via running server, returns nil if successful
func (h *AgentHandler) tryServerExecution(agentID int64, task string, tail bool, cfg *config.Config) error {
	// Check if Station server is running
	apiPort := cfg.APIPort
	if apiPort == 0 {
		apiPort = 8080 // Default port
	}
	
	healthURL := fmt.Sprintf("http://localhost:%d/health", apiPort)
	client := &http.Client{Timeout: 2 * time.Second} // Quick timeout
	resp, err := client.Get(healthURL)
	if err != nil {
		return err // Server not available
	}
	resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server health check failed")
	}
	
	fmt.Printf("✅ Connected to Station server (port %d)\n\n", apiPort)
	
	// Queue the execution via API
	runID, err := h.queueAgentExecution(agentID, task, apiPort)
	if err != nil {
		return fmt.Errorf("failed to queue agent execution: %w", err)
	}
	
	fmt.Printf("🔄 Agent execution queued (Run ID: %d)\n", runID)
	
	// Monitor execution and display results
	if tail {
		return h.monitorExecutionWithTail(runID, apiPort)
	} else {
		return h.monitorExecution(runID, apiPort)
	}
}

// runAgentWithLocalExecutionEngine provides a simple fallback execution
func (h *AgentHandler) runAgentWithLocalExecutionEngine(agentID int64, task string, tail bool, cfg *config.Config, agent *models.Agent) error {
	styles := getCLIStyles(h.themeManager)
	
	fmt.Printf("🔄 Local execution mode (simplified)\n")
	fmt.Printf("💡 For full execution with tool calls, start the server: %s\n", styles.Info.Render("stn serve"))
	fmt.Printf("💡 This mode provides basic execution without the full execution engine\n\n")
	
	// Create fresh database connection for fallback execution
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()
	
	repos := repositories.New(database)
	
	// Get console user for execution tracking
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		return fmt.Errorf("failed to get console user: %w", err)
	}
	
	// Create a basic agent run record
	agentRun, err := repos.AgentRuns.Create(
		agentID,
		consoleUser.ID,
		task,
		"", // final_response (will be updated)
		0,  // steps_taken
		nil, // tool_calls 
		nil, // execution_steps
		"running", // status
		nil, // completed_at
	)
	if err != nil {
		return fmt.Errorf("failed to create agent run record: %w", err)
	}
	
	fmt.Printf("🚀 Agent execution started (Run ID: %d)\n", agentRun.ID)
	
	// Simulate execution for now (in a real implementation, this would call a minimal agent service)
	time.Sleep(2 * time.Second)
	fmt.Printf("🔄 Executing agent...\n")
	time.Sleep(3 * time.Second)
	
	// Update run as completed with a basic response
	completedAt := time.Now()
	response := fmt.Sprintf("CLI execution completed for task: %s\n\nNote: This is a simplified execution mode. For full agent capabilities with tool access, please start the Station server with 'stn serve' and try again.", task)
	
	err = repos.AgentRuns.UpdateCompletion(
		agentRun.ID,
		response,
		1, // steps_taken
		nil, // tool_calls
		nil, // execution_steps  
		"completed",
		&completedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update agent run: %w", err)
	}
	
	// Get updated run and display results
	updatedRun, err := repos.AgentRuns.GetByID(agentRun.ID)
	if err != nil {
		return fmt.Errorf("failed to get updated run: %w", err)
	}
	
	fmt.Printf("✅ Agent execution completed!\n")
	return h.displayExecutionResults(updatedRun)
}

// runAgentWithStdioMCP executes an agent using self-bootstrapping stdio MCP architecture
func (h *AgentHandler) runAgentWithStdioMCP(agentID int64, task string, tail bool, cfg *config.Config, agent *models.Agent) error {
	
	fmt.Printf("🔄 Self-bootstrapping stdio MCP execution mode\n")
	fmt.Printf("🤖 Using Station's own MCP server to execute agent via stdio\n")
	fmt.Printf("💡 This creates a self-bootstrapping system where Station manages itself\n\n")
	
	// Create fresh database connection for execution
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()
	
	repos := repositories.New(database)
	
	// Load config to get encryption key
	stationCfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}
	
	keyManager, err := crypto.NewKeyManagerFromConfig(stationCfg.EncryptionKey)
	if err != nil {
		return fmt.Errorf("failed to initialize key manager: %w", err)
	}
	
	// Initialize services needed for stdio MCP execution
	mcpConfigSvc := services.NewMCPConfigService(repos, keyManager)
	
	// Create intelligent agent creator which uses our stdio MCP server
	creator := services.NewIntelligentAgentCreator(repos, nil, mcpConfigSvc)
	
	// Get console user for execution tracking  
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		return fmt.Errorf("failed to get console user: %w", err)
	}
	
	// Create agent run record
	agentRun, err := repos.AgentRuns.Create(
		agentID,
		consoleUser.ID,
		task,
		"", // final_response (will be updated)
		0,  // steps_taken
		nil, // tool_calls 
		nil, // execution_steps
		"running", // status
		nil, // completed_at
	)
	if err != nil {
		return fmt.Errorf("failed to create agent run record: %w", err)
	}
	
	fmt.Printf("🚀 Agent execution started via stdio MCP (Run ID: %d)\n", agentRun.ID)
	fmt.Printf("🔗 Connecting to Station's stdio MCP server...\n")
	
	// Use the intelligent agent creator's stdio MCP connection to execute
	ctx := context.Background()
	
	fmt.Printf("🔍 Analyzing execution requirements using stdio MCP...\n")
	
	// Initialize the creator's Genkit + MCP system (stdio MCP is mandatory)
	err = creator.TestStdioMCPConnection(ctx)
	if err != nil {
		// Update run as failed since stdio MCP is mandatory
		completedAt := time.Now()
		errorMsg := fmt.Sprintf("Stdio MCP connection failed (required for execution): %v", err)
		
		updateErr := repos.AgentRuns.UpdateCompletion(
			agentRun.ID,
			errorMsg,
			0, // steps_taken
			nil, // tool_calls
			nil, // execution_steps  
			"failed",
			&completedAt,
		)
		if updateErr != nil {
			return fmt.Errorf("failed to update failed agent run: %w", updateErr)
		}
		
		fmt.Printf("❌ Stdio MCP connection failed: %v\n", err)
		fmt.Printf("💡 Stdio MCP is required for self-bootstrapping agent execution\n")
		fmt.Printf("💡 Make sure Station binary (./stn) is available and working\n")
		return fmt.Errorf("stdio MCP connection failed (required): %w", err)
	}
	
	fmt.Printf("✅ Connected to stdio MCP server successfully\n")
	fmt.Printf("🤖 Executing agent using self-bootstrapping architecture...\n")
	
	// Execute the agent using our stdio MCP approach
	result, err := creator.ExecuteAgentViaStdioMCP(ctx, agent, task, agentRun.ID)
	if err != nil {
		// Store original error before it gets overwritten
		originalErr := err
		
		// Update run as failed
		completedAt := time.Now()
		errorMsg := fmt.Sprintf("Stdio MCP execution failed: %v", originalErr)
		
		updateErr := repos.AgentRuns.UpdateCompletion(
			agentRun.ID,
			errorMsg,
			0, // steps_taken
			nil, // tool_calls
			nil, // execution_steps  
			"failed",
			&completedAt,
		)
		if updateErr != nil {
			return fmt.Errorf("failed to update failed agent run: %w", updateErr)
		}
		
		fmt.Printf("❌ Agent execution failed: %v\n", originalErr)
		return fmt.Errorf("stdio MCP execution failed: %w", originalErr)
	}
	
	// Update run as completed with stdio MCP results
	completedAt := time.Now()
	err = repos.AgentRuns.UpdateCompletion(
		agentRun.ID,
		result.Response,
		result.StepsTaken,
		result.ToolCalls,
		result.ExecutionSteps,
		"completed",
		&completedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update agent run: %w", err)
	}
	
	// Get updated run and display results
	updatedRun, err := repos.AgentRuns.GetByID(agentRun.ID)
	if err != nil {
		return fmt.Errorf("failed to get updated run: %w", err)
	}
	
	fmt.Printf("✅ Agent execution completed via stdio MCP!\n")
	return h.displayExecutionResults(updatedRun)
}

// executeAgentSimple provides a fallback execution when stdio MCP is not available
func (h *AgentHandler) executeAgentSimple(agentRun *models.AgentRun, agent *models.Agent, task string, repos *repositories.Repositories, styles *CLIStyles) error {
	fmt.Printf("🔄 Simple execution fallback mode\n")
	time.Sleep(2 * time.Second)
	fmt.Printf("🔄 Executing agent...\n")
	time.Sleep(3 * time.Second)
	
	// Update run as completed with a basic response
	completedAt := time.Now()
	response := fmt.Sprintf("Simple execution completed for task: %s\n\nNote: This is a simplified execution mode. Stdio MCP connection was not available.", task)
	
	err := repos.AgentRuns.UpdateCompletion(
		agentRun.ID,
		response,
		1, // steps_taken
		nil, // tool_calls
		nil, // execution_steps  
		"completed",
		&completedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update agent run: %w", err)
	}
	
	// Get updated run and display results
	updatedRun, err := repos.AgentRuns.GetByID(agentRun.ID)
	if err != nil {
		return fmt.Errorf("failed to get updated run: %w", err)
	}
	
	return h.displayExecutionResults(updatedRun)
}


// queueAgentExecution calls the API to queue an agent execution
func (h *AgentHandler) queueAgentExecution(agentID int64, task string, apiPort int) (int64, error) {
	url := fmt.Sprintf("http://localhost:%d/api/v1/agents/%d/queue", apiPort, agentID)
	
	payload := map[string]string{
		"task": task,
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("API error: status %d: %s", resp.StatusCode, string(body))
	}
	
	var result struct {
		RunID int64 `json:"run_id"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result.RunID, nil
}

// monitorExecution polls the agent run status and displays final results
func (h *AgentHandler) monitorExecution(runID int64, apiPort int) error {
	styles := getCLIStyles(h.themeManager)
	fmt.Printf("⏳ Monitoring execution progress...\n")
	
	// Load fresh config and database connection for each check to avoid locks
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	maxAttempts := 60 // 2 minutes max
	attempt := 0
	
	for attempt < maxAttempts {
		// Create fresh database connection for each check
		database, err := db.New(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		
		freshRepos := repositories.New(database)
		run, err := freshRepos.AgentRuns.GetByID(runID)
		database.Close() // Close immediately after reading
		
		if err != nil {
			return fmt.Errorf("failed to get run status: %w", err)
		}
		
		switch run.Status {
		case "completed":
			fmt.Printf("✅ Agent execution completed successfully!\n")
			return h.displayExecutionResults(run)
		case "failed":
			fmt.Printf("❌ Agent execution failed\n")
			if run.FinalResponse != "" {
				fmt.Printf("Error: %s\n", styles.Error.Render(run.FinalResponse))
			}
			return fmt.Errorf("agent execution failed")
		case "running":
			fmt.Printf("🔄 Agent is executing... (step %d)\n", run.StepsTaken)
		case "queued":
			fmt.Printf("⏸️  Agent is queued for execution...\n")
		}
		
		time.Sleep(2 * time.Second)
		attempt++
	}
	
	return fmt.Errorf("execution monitoring timed out after %d attempts", maxAttempts)
}

// monitorExecutionWithTail provides real-time execution monitoring
func (h *AgentHandler) monitorExecutionWithTail(runID int64, apiPort int) error {
	// For now, use the same monitoring as regular mode
	// TODO: Implement real-time streaming updates
	fmt.Printf("📺 Monitoring execution with tail mode...\n")
	return h.monitorExecution(runID, apiPort)
}

// displayExecutionResults shows the final execution results with tool calls
func (h *AgentHandler) displayExecutionResults(run *models.AgentRun) error {
	styles := getCLIStyles(h.themeManager)
	
	fmt.Print("\n" + styles.Banner.Render("🎉 Execution Results") + "\n\n")
	fmt.Printf("📊 Run ID: %d\n", run.ID)
	fmt.Printf("⚡ Steps Taken: %d\n", run.StepsTaken)
	if run.CompletedAt != nil {
		fmt.Printf("⏱️  Duration: %v\n", run.CompletedAt.Sub(run.StartedAt).Round(time.Second))
	}
	
	// Display final response
	if run.FinalResponse != "" {
		fmt.Printf("\n📝 Final Response:\n")
		fmt.Printf("%s\n", styles.Success.Render(run.FinalResponse))
	}
	
	// Display tool calls if available
	if run.ToolCalls != nil && len(*run.ToolCalls) > 0 {
		fmt.Printf("\n🔧 Tool Calls (%d):\n", len(*run.ToolCalls))
		for i, toolCall := range *run.ToolCalls {
			toolData, _ := json.MarshalIndent(toolCall, "", "  ")
			fmt.Printf("  %d. %s\n", i+1, string(toolData))
		}
	}
	
	// Display execution steps if available
	if run.ExecutionSteps != nil && len(*run.ExecutionSteps) > 0 {
		fmt.Printf("\n📋 Execution Steps (%d):\n", len(*run.ExecutionSteps))
		for i, step := range *run.ExecutionSteps {
			stepData, _ := json.MarshalIndent(step, "", "  ")
			fmt.Printf("  %d. %s\n", i+1, string(stepData))
		}
	}
	
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

// createAgentLocal creates an intelligent agent using the local intelligent agent creator
func (h *AgentHandler) createAgentLocal(name, description, domain, schedule string) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	// Initialize repositories and services for intelligent creation
	repos := repositories.New(database)
	
	// Load config to get encryption key
	stationCfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}
	
	keyManager, err := crypto.NewKeyManagerFromConfig(stationCfg.EncryptionKey)
	if err != nil {
		return fmt.Errorf("failed to initialize key manager: %w", err)
	}

	mcpConfigSvc := services.NewMCPConfigService(repos, keyManager)

	// Create intelligent agent creator
	creator := services.NewIntelligentAgentCreator(repos, nil, mcpConfigSvc)

	// Create agent creation request
	req := services.AgentCreationRequest{
		Name:        name,
		Description: description,
		UserIntent:  description, // Use description as user intent
		Domain:      domain,
		Schedule:    schedule,
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Info.Render("🤖 Analyzing requirements and creating intelligent agent..."))

	// Create the intelligent agent
	ctx := context.Background()
	agent, err := creator.CreateIntelligentAgent(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create intelligent agent: %w", err)
	}

	// Get agent tools for display
	agentTools, err := repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		agentTools = []*models.AgentToolWithDetails{}
	}

	fmt.Println()
	fmt.Println(styles.Success.Render("✅ Intelligent agent created successfully!"))
	fmt.Println()
	fmt.Printf("Agent ID: %d\n", agent.ID)
	fmt.Printf("Name: %s\n", agent.Name)
	fmt.Printf("Description: %s\n", agent.Description)
	fmt.Printf("Max Steps: %d\n", agent.MaxSteps)
	fmt.Printf("Tools Assigned: %d\n", len(agentTools))

	if len(agentTools) > 0 {
		fmt.Println()
		fmt.Println(styles.Info.Render("🛠️ Assigned Tools:"))
		for _, tool := range agentTools {
			fmt.Printf("  - %s\n", tool.ToolName)
		}
	}

	fmt.Println()
	fmt.Println(styles.Info.Render("🚀 You can now run this agent with:"))
	fmt.Printf("  stn agent run %d \"<your task>\"\n", agent.ID)

	return nil
}

// createAgentRemote creates an agent via API (not implemented yet)
func (h *AgentHandler) createAgentRemote(name, description, domain, schedule, endpoint string) error {
	return fmt.Errorf("remote agent creation not yet implemented - please use local mode for now")
}








