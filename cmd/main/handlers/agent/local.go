package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/theme"
	"station/pkg/models"
)

// CLIStyles contains all styled components for the CLI
type CLIStyles struct {
	Title    lipgloss.Style
	Banner   lipgloss.Style
	Success  lipgloss.Style
	Error    lipgloss.Style
	Info     lipgloss.Style
	Focused  lipgloss.Style
	Blurred  lipgloss.Style
	Cursor   lipgloss.Style
	No       lipgloss.Style
	Help     lipgloss.Style
	Form     lipgloss.Style
}

// Helper functions

// loadStationConfig loads the Station configuration
func loadStationConfig() (*config.Config, error) {
	encryptionKey := viper.GetString("encryption_key")
	if encryptionKey == "" {
		return nil, fmt.Errorf("no encryption key found. Run 'station init' first")
	}

	return &config.Config{
		DatabaseURL:   viper.GetString("database_url"),
		APIPort:       viper.GetInt("api_port"),
		SSHPort:       viper.GetInt("ssh_port"),
		MCPPort:       viper.GetInt("mcp_port"),
		EncryptionKey: encryptionKey,
	}, nil
}

// getCLIStyles returns theme-aware CLI styles
func getCLIStyles(themeManager *theme.ThemeManager) CLIStyles {
	if themeManager == nil {
		// Fallback to hardcoded Tokyo Night styles
		return CLIStyles{
			Title: lipgloss.NewStyle().
				Background(lipgloss.Color("#bb9af7")).
				Foreground(lipgloss.Color("#1a1b26")).
				Bold(true).
				Padding(0, 2).
				MarginBottom(1),
			Banner: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#bb9af7")).
				Padding(1, 2).
				MarginBottom(1),
			Success: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9ece6a")).
				Bold(true),
			Error: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f7768e")).
				Bold(true),
			Info: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7dcfff")),
			Focused: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#bb9af7")).
				Bold(true),
			Blurred: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#565f89")),
			Cursor: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#bb9af7")),
			No: lipgloss.NewStyle(),
			Help: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#565f89")).
				Italic(true),
			Form: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#414868")).
				Padding(1, 2).
				MarginTop(1).
				MarginBottom(1),
		}
	}

	themeStyles := themeManager.GetStyles()
	palette := themeManager.GetPalette()

	return CLIStyles{
		Title: themeStyles.Header.Copy().
			Background(lipgloss.Color(palette.Secondary)).
			Foreground(lipgloss.Color(palette.BackgroundDark)).
			Padding(0, 2).
			MarginBottom(1),
		Banner: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(palette.Secondary)).
			Padding(1, 2).
			MarginBottom(1),
		Success: themeStyles.Success,
		Error:   themeStyles.Error,
		Info:    themeStyles.Info,
		Focused: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Secondary)).
			Bold(true),
		Blurred: themeStyles.Muted,
		Cursor: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Secondary)),
		No: lipgloss.NewStyle(),
		Help: themeStyles.Muted.Copy().
			Italic(true),
		Form: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(palette.Border)).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1),
	}
}

// getAPIKeyFromEnv returns the API key from environment
func getAPIKeyFromEnv() string {
	return os.Getenv("STATION_API_KEY")
}

// makeAuthenticatedRequest creates an HTTP request with authentication header if available
func makeAuthenticatedRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	
	// Add authentication header if available
	if apiKey := getAPIKeyFromEnv(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	return req, nil
}

// Local agent operations

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

// createAgentLocal creates an intelligent agent using the local intelligent agent creator
func (h *AgentHandler) createAgentLocal(name, description, domain, schedule, environment string) error {
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
	
	// Note: User specified environment preference: %s
	// The intelligent agent creator will analyze requirements and determine optimal environment
	_ = environment // Acknowledge environment parameter
	
	// Load config to get encryption key
	_, err = loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}
	
	// keyManager removed - no longer needed for file-based configs

	// MCPConfigService removed - using file-based configs only

	// Create intelligent agent creator (simplified for file-based system)
	// Note: Intelligent agent creator will analyze requirements and determine optimal environment
	// The user-specified environment (%s) preference is noted but may be overridden for optimal performance
	creator := services.NewIntelligentAgentCreator(repos, nil)

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
	_, err = loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}
	
	// keyManager removed - no longer needed for file-based configs
	
	// Initialize services for file-based configs (MCPConfigService removed)
	// Create intelligent agent creator (simplified for file-based system)
	creator := services.NewIntelligentAgentCreator(repos, nil)
	
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