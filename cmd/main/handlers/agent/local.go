package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// AgentExportConfig represents the exported agent configuration
type AgentExportConfig struct {
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Prompt        string    `json:"prompt"`
	MaxSteps      int64     `json:"max_steps"`
	Environment   string    `json:"environment"`
	CronSchedule  *string   `json:"cron_schedule,omitempty"`
	ScheduleEnabled bool    `json:"schedule_enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Version       string    `json:"version"`
}

// AgentToolsConfig represents the tool dependencies and MCP server mappings
type AgentToolsConfig struct {
	AgentName    string                `json:"agent_name"`
	Tools        []AgentToolExport     `json:"tools"`
	MCPServers   map[string]MCPServerInfo `json:"mcp_servers"`
	Environment  string                `json:"environment"`
	Version      string                `json:"version"`
}

// AgentToolExport represents an exported tool assignment
type AgentToolExport struct {
	ToolName     string `json:"tool_name"`
	ServerName   string `json:"server_name"`
	MCPServerID  string `json:"mcp_server_id"`
}

// MCPServerInfo represents MCP server dependency information
type MCPServerInfo struct {
	Name        string `json:"name"`
	ConfigPath  string `json:"config_path"`
	ServerType  string `json:"server_type"`
	Command     string `json:"command,omitempty"`
	Args        []string `json:"args,omitempty"`
	Environment string `json:"environment"`
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
	
	fmt.Printf("🔍 Initializing agent execution with stdio MCP...\n")
	fmt.Printf("🤖 Executing agent using self-bootstrapping architecture...\n")
	
	// Execute the agent using our stdio MCP approach
	result, err := creator.ExecuteAgentViaStdioMCP(ctx, agent, task, agentRun.ID)
	if err != nil {
		// Store original error before it gets overwritten
		originalErr := err
		
		// Update run as failed
		completedAt := time.Now()
		errorMsg := fmt.Sprintf("Stdio MCP execution failed: %v", originalErr)
		
		updateErr := repos.AgentRuns.UpdateCompletionWithMetadata(
			agentRun.ID,
			errorMsg,
			0, // steps_taken
			nil, // tool_calls
			nil, // execution_steps  
			"failed",
			&completedAt,
			nil, // inputTokens
			nil, // outputTokens
			nil, // totalTokens
			nil, // durationSeconds
			nil, // modelName
			nil, // toolsUsed
		)
		if updateErr != nil {
			return fmt.Errorf("failed to update failed agent run: %w", updateErr)
		}
		
		fmt.Printf("❌ Agent execution failed: %v\n", originalErr)
		return fmt.Errorf("stdio MCP execution failed: %w", originalErr)
	}
	
	// Update run as completed with stdio MCP results and metadata
	completedAt := time.Now()
	durationSeconds := result.Duration.Seconds()
	
	// Extract token usage from result using robust type conversion
	var inputTokens, outputTokens, totalTokens *int64
	var toolsUsed *int64
	
	if result.TokenUsage != nil {
		fmt.Printf("DEBUG CLI: TokenUsage map contains: %+v\n", result.TokenUsage)
		
		// Handle input_tokens with multiple numeric types
		if inputVal := extractInt64FromTokenUsage(result.TokenUsage["input_tokens"]); inputVal != nil {
			inputTokens = inputVal
			fmt.Printf("DEBUG CLI: Extracted input_tokens: %d\n", *inputTokens)
		} else {
			fmt.Printf("DEBUG CLI: Failed to extract input_tokens from: %+v (type: %T)\n", result.TokenUsage["input_tokens"], result.TokenUsage["input_tokens"])
		}
		
		// Handle output_tokens with multiple numeric types
		if outputVal := extractInt64FromTokenUsage(result.TokenUsage["output_tokens"]); outputVal != nil {
			outputTokens = outputVal
			fmt.Printf("DEBUG CLI: Extracted output_tokens: %d\n", *outputTokens)
		} else {
			fmt.Printf("DEBUG CLI: Failed to extract output_tokens from: %+v (type: %T)\n", result.TokenUsage["output_tokens"], result.TokenUsage["output_tokens"])
		}
		
		// Handle total_tokens with multiple numeric types
		if totalVal := extractInt64FromTokenUsage(result.TokenUsage["total_tokens"]); totalVal != nil {
			totalTokens = totalVal
			fmt.Printf("DEBUG CLI: Extracted total_tokens: %d\n", *totalTokens)
		} else {
			fmt.Printf("DEBUG CLI: Failed to extract total_tokens from: %+v (type: %T)\n", result.TokenUsage["total_tokens"], result.TokenUsage["total_tokens"])
		}
	} else {
		fmt.Printf("DEBUG CLI: result.TokenUsage is nil\n")
	}
	
	if result.ToolsUsed > 0 {
		toolsUsedVal := int64(result.ToolsUsed)
		toolsUsed = &toolsUsedVal
	}
	
	err = repos.AgentRuns.UpdateCompletionWithMetadata(
		agentRun.ID,
		result.Response,
		result.StepsTaken,
		result.ToolCalls,
		result.ExecutionSteps,
		"completed",
		&completedAt,
		inputTokens,
		outputTokens,
		totalTokens,
		&durationSeconds,
		&result.ModelName,
		toolsUsed,
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


// exportAgentLocal exports an agent to file-based configuration
func (h *AgentHandler) exportAgentLocal(agentID int64, environment string) error {
	// Load Station config and connect to database
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

	// Get agent details
	agent, err := repos.Agents.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	// Get agent tools and their MCP server mappings
	toolsData, err := repos.AgentTools.ListAgentTools(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent tools: %w", err)
	}

	// Create environments directory structure
	configDir := viper.GetString("config_dir")
	if configDir == "" {
		configDir = filepath.Join(os.Getenv("HOME"), ".config", "station")
	}
	
	envDir := filepath.Join(configDir, "environments", environment)
	agentsDir := filepath.Join(envDir, "agents")
	
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}

	// Create safe filename from agent name
	safeAgentName := strings.ReplaceAll(strings.ToLower(agent.Name), " ", "-")
	safeAgentName = strings.ReplaceAll(safeAgentName, "_", "-")
	
	// Export agent configuration
	agentConfig := AgentExportConfig{
		Name:            agent.Name,
		Description:     agent.Description,
		Prompt:          agent.Prompt,
		MaxSteps:        agent.MaxSteps,
		Environment:     environment,
		CronSchedule:    agent.CronSchedule,
		ScheduleEnabled: agent.ScheduleEnabled,
		CreatedAt:       agent.CreatedAt,
		UpdatedAt:       agent.UpdatedAt,
		Version:         "1.0",
	}

	agentConfigPath := filepath.Join(agentsDir, safeAgentName+".json")
	agentConfigData, err := json.MarshalIndent(agentConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal agent config: %w", err)
	}

	if err := os.WriteFile(agentConfigPath, agentConfigData, 0644); err != nil {
		return fmt.Errorf("failed to write agent config: %w", err)
	}

	// Export tools configuration with MCP server mappings
	var toolExports []AgentToolExport
	mcpServers := make(map[string]MCPServerInfo)

	for _, tool := range toolsData {
		toolExport := AgentToolExport{
			ToolName:    tool.ToolName,
			ServerName:  tool.ServerName,
			MCPServerID: fmt.Sprintf("tool-%d", tool.ToolID),
		}
		toolExports = append(toolExports, toolExport)

		// Add MCP server info if not already present
		serverKey := fmt.Sprintf("tool-%d", tool.ToolID)
		if _, exists := mcpServers[serverKey]; !exists {
			mcpServers[serverKey] = MCPServerInfo{
				Name:        tool.ServerName,
				ConfigPath:  fmt.Sprintf("environments/%s/mcp-servers/%s.json", environment, tool.ServerName),
				ServerType:  "stdio", // Default to stdio, could be enhanced to detect actual type
				Environment: environment,
			}
		}
	}

	toolsConfig := AgentToolsConfig{
		AgentName:   agent.Name,
		Tools:       toolExports,
		MCPServers:  mcpServers,
		Environment: environment,
		Version:     "1.0",
	}

	toolsConfigPath := filepath.Join(agentsDir, safeAgentName+"-tools.json")
	toolsConfigData, err := json.MarshalIndent(toolsConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tools config: %w", err)
	}

	if err := os.WriteFile(toolsConfigPath, toolsConfigData, 0644); err != nil {
		return fmt.Errorf("failed to write tools config: %w", err)
	}

	fmt.Printf("✅ Agent exported successfully to environment '%s':\n", environment)
	fmt.Printf("📄 Agent config: %s\n", agentConfigPath)
	fmt.Printf("🔧 Tools config: %s\n", toolsConfigPath)
	fmt.Printf("🔗 %d tool(s) and %d MCP server(s) mapped\n", len(toolExports), len(mcpServers))

	return nil
}

// importAgentsLocal imports agents from file-based configuration
func (h *AgentHandler) importAgentsLocal(environment string) error {
	styles := getCLIStyles(h.themeManager)
	
	// Load Station config and connect to database
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

	// Find agents directory
	configDir := viper.GetString("config_dir")
	if configDir == "" {
		configDir = filepath.Join(os.Getenv("HOME"), ".config", "station")
	}
	
	agentsDir := filepath.Join(configDir, "environments", environment, "agents")
	
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		return fmt.Errorf("agents directory not found: %s", agentsDir)
	}

	// Scan for agent configuration files
	files, err := filepath.Glob(filepath.Join(agentsDir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to scan agents directory: %w", err)
	}

	var imported, skipped int
	
	for _, file := range files {
		fileName := filepath.Base(file)
		
		// Skip tool configuration files
		if strings.HasSuffix(fileName, "-tools.json") {
			continue
		}

		fmt.Printf("📄 Processing: %s\n", fileName)

		// Read agent configuration
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("❌ Failed to read %s: %v\n", fileName, err)
			continue
		}

		var agentConfig AgentExportConfig
		if err := json.Unmarshal(data, &agentConfig); err != nil {
			fmt.Printf("❌ Failed to parse %s: %v\n", fileName, err)
			continue
		}

		// Check if agent already exists
		existingAgents, err := repos.Agents.List()
		if err != nil {
			fmt.Printf("❌ Failed to check existing agents: %v\n", err)
			continue
		}

		var exists bool
		for _, existing := range existingAgents {
			if existing.Name == agentConfig.Name {
				exists = true
				break
			}
		}

		if exists {
			fmt.Printf("⏭️  Agent '%s' already exists, skipping\n", agentConfig.Name)
			skipped++
			continue
		}

		// Get or create environment
		environments, err := repos.Environments.List()
		if err != nil {
			fmt.Printf("❌ Failed to list environments: %v\n", err)
			continue
		}

		var envID int64 = 1 // Default environment
		for _, env := range environments {
			if env.Name == environment {
				envID = env.ID
				break
			}
		}

		// Create agent
		newAgent, err := repos.Agents.Create(
			agentConfig.Name,
			agentConfig.Description,
			agentConfig.Prompt,
			agentConfig.MaxSteps,
			envID,
			1, // Default user
			nil, // input_schema - not set in local handler
			agentConfig.CronSchedule,
			agentConfig.ScheduleEnabled,
		)
		if err != nil {
			fmt.Printf("❌ Failed to create agent '%s': %v\n", agentConfig.Name, err)
			continue
		}

		// Load and apply tools configuration if available
		toolsFile := strings.TrimSuffix(file, ".json") + "-tools.json"
		if _, err := os.Stat(toolsFile); err == nil {
			if err := h.importAgentTools(newAgent.ID, toolsFile, repos); err != nil {
				fmt.Printf("⚠️  Agent created but failed to import tools: %v\n", err)
			}
		}

		fmt.Printf("✅ Imported agent: %s (ID: %d)\n", agentConfig.Name, newAgent.ID)
		imported++
	}

	fmt.Printf("\n📊 Import Summary:\n")
	fmt.Printf("✅ Imported: %d agents\n", imported)
	fmt.Printf("⏭️  Skipped: %d agents (already exist)\n", skipped)

	if imported > 0 {
		fmt.Println(styles.Success.Render("🎉 Agent import completed successfully!"))
	}

	return nil
}

// importAgentTools imports tool assignments for an agent
func (h *AgentHandler) importAgentTools(agentID int64, toolsFile string, repos *repositories.Repositories) error {
	data, err := os.ReadFile(toolsFile)
	if err != nil {
		return fmt.Errorf("failed to read tools file: %w", err)
	}

	var toolsConfig AgentToolsConfig
	if err := json.Unmarshal(data, &toolsConfig); err != nil {
		return fmt.Errorf("failed to parse tools config: %w", err)
	}

	assignedCount := 0
	fmt.Printf("🔧 Found %d tool assignments to import\n", len(toolsConfig.Tools))
	
	// Get agent's environment to ensure tools are from the same environment
	agent, err := repos.Agents.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	for _, toolExport := range toolsConfig.Tools {
		// Find tool by name in the agent's environment
		tool, err := repos.MCPTools.FindByNameInEnvironment(agent.EnvironmentID, toolExport.ToolName)
		if err != nil {
			fmt.Printf("⚠️  Warning: Tool '%s' not found in environment, skipping\n", toolExport.ToolName)
			continue
		}
		
		// Add the tool assignment to the agent
		_, err = repos.AgentTools.AddAgentTool(agentID, tool.ID)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to assign tool '%s' to agent: %v\n", toolExport.ToolName, err)
			continue
		}
		
		assignedCount++
		fmt.Printf("✓ Successfully assigned tool '%s' to agent\n", toolExport.ToolName)
	}
	
	if assignedCount > 0 {
		fmt.Printf("🎉 Successfully imported %d/%d tool assignments\n", assignedCount, len(toolsConfig.Tools))
	}
	
	return nil
}