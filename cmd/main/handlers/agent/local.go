package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/lighthouse"
	"station/internal/services"
	"station/internal/theme"
	"station/pkg/models"
	"station/pkg/types"
)

// CLIStyles contains all styled components for the CLI
type CLIStyles struct {
	Title   lipgloss.Style
	Banner  lipgloss.Style
	Success lipgloss.Style
	Error   lipgloss.Style
	Info    lipgloss.Style
	Focused lipgloss.Style
	Blurred lipgloss.Style
	Cursor  lipgloss.Style
	No      lipgloss.Style
	Help    lipgloss.Style
	Form    lipgloss.Style
}

// Helper functions

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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

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
	runs, err := repos.AgentRuns.ListByAgent(context.Background(), agentID)
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

func (h *AgentHandler) runAgentLocal(agentID int64, task string, tail bool, codingSession ...string) error {
	styles := getCLIStyles(h.themeManager)

	// Load configuration and connect to database (including environment variables)
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	if err := cfg.LoadSecretsFromBackend(); err != nil {
		return fmt.Errorf("failed to load secrets from backend: %w", err)
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

	fmt.Printf("📋 Task: %s\n", styles.Info.Render(task))

	// Close database connection before trying server execution to avoid locks
	database.Close()

	// Try server first, fall back to stdio MCP self-bootstrapping execution
	if h.tryServerExecution(agentID, task, tail, cfg) == nil {
		return nil
	}

	// Server not available, use self-bootstrapping stdio MCP execution
	fmt.Printf("💡 Server not available, using self-bootstrapping stdio MCP execution\n\n")

	var sessionID string
	if len(codingSession) > 0 {
		sessionID = codingSession[0]
	}
	return h.runAgentWithStdioMCP(agentID, task, tail, cfg, agent, sessionID)
}

func (h *AgentHandler) deleteAgentLocal(agentID int64) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

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

func (h *AgentHandler) runAgentWithStdioMCP(agentID int64, task string, tail bool, cfg *config.Config, agent *models.Agent, codingSessionID string) error {
	// Create execution context
	ctx := context.Background()

	fmt.Printf("🔄 Self-bootstrapping stdio MCP execution mode\n")
	fmt.Printf("🤖 Using Station's own MCP server to execute agent via stdio\n")
	fmt.Printf("💡 This creates a self-bootstrapping system where Station manages itself\n\n")

	// Create fresh database connection for execution
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)

	// Setup signal handling to update run status on interruption (Ctrl+C, timeout, etc.)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Track whether execution completed normally
	executionCompleted := false
	defer func() {
		signal.Stop(sigChan)
		close(sigChan)
	}()

	// Use the config passed to the function (includes environment variables)
	// No need to reload - cfg already contains CloudShip settings from environment variables

	// keyManager removed - no longer needed for file-based configs

	// Initialize services for file-based configs (MCPConfigService removed)
	// Initialize Lighthouse client for CloudShip integration
	mode := lighthouse.DetectModeFromCommand()
	fmt.Printf("Debug: CloudShip config - Enabled: %v, Key: %v, Endpoint: %v\n",
		cfg.CloudShip.Enabled,
		cfg.CloudShip.RegistrationKey != "",
		cfg.CloudShip.Endpoint)
	lighthouseClient, err := lighthouse.InitializeLighthouseFromConfig(cfg, mode)
	if err != nil {
		fmt.Printf("Warning: Failed to initialize Lighthouse client: %v\n", err)
	} else if lighthouseClient != nil {
		fmt.Printf("✅ Lighthouse client initialized successfully for mode: %s\n", mode)
	} else {
		fmt.Printf("💡 Lighthouse client disabled (CloudShip integration not configured)\n")
	}

	// Create agent service with Lighthouse integration
	agentService := services.NewAgentService(repos, lighthouseClient)

	// Initialize direct API memory client for CLI mode CloudShip memory integration
	// CLI mode doesn't have a persistent management channel, so we use direct HTTP calls
	if cfg.CloudShip.Enabled && cfg.CloudShip.APIURL != "" && cfg.CloudShip.RegistrationKey != "" {
		memoryAPIClient := lighthouse.NewMemoryAPIClient(
			cfg.CloudShip.APIURL,
			cfg.CloudShip.RegistrationKey,
			2*time.Second, // 2 second timeout per PRD
		)
		agentService.SetMemoryAPIClient(memoryAPIClient)
		fmt.Printf("✅ CloudShip memory integration configured (direct API mode)\n")
	}

	// Get console user for execution tracking
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		return fmt.Errorf("failed to get console user: %w", err)
	}

	// Create agent run record
	agentRun, err := repos.AgentRuns.Create(
		ctx,
		agentID,
		consoleUser.ID,
		task,
		"",        // final_response (will be updated)
		0,         // steps_taken
		nil,       // tool_calls
		nil,       // execution_steps
		"running", // status
		nil,       // completed_at
	)
	if err != nil {
		return fmt.Errorf("failed to create agent run record: %w", err)
	}

	// Handle interruption signals in goroutine
	go func() {
		sig := <-sigChan
		if !executionCompleted {
			fmt.Printf("\n\n⚠️  Received signal %v - updating run status to cancelled\n", sig)

			// Update run status to cancelled
			completedAt := time.Now()
			errorMsg := fmt.Sprintf("Execution interrupted by signal: %v", sig)

			// Create fresh database connection to update status
			updateDB, err := db.New(cfg.DatabaseURL)
			if err != nil {
				fmt.Printf("❌ Failed to update run status on interruption: %v\n", err)
				return
			}
			defer updateDB.Close()

			updateRepos := repositories.New(updateDB)
			updateRepos.AgentRuns.UpdateCompletionWithMetadata(
				context.Background(),
				agentRun.ID,
				errorMsg,
				0,
				nil,
				nil,
				"cancelled",
				&completedAt,
				nil, nil, nil, nil, nil, nil,
				&errorMsg,
			)

			fmt.Printf("✅ Run %d marked as cancelled\n", agentRun.ID)
		}
	}()

	fmt.Printf("🔗 Connecting to Station's stdio MCP server...\n")

	// Use the intelligent agent creator's stdio MCP connection to execute
	ctx = context.Background()

	fmt.Printf("🤖 Executing agent using self-bootstrapping architecture...\n")

	variables := make(map[string]interface{})
	if codingSessionID != "" {
		variables["coding_session_id"] = codingSessionID
		fmt.Printf("🔗 Using existing coding session: %s\n", codingSessionID)
	}

	result, err := agentService.GetExecutionEngine().Execute(ctx, agent, task, agentRun.ID, variables)
	if err != nil {
		// Store original error before it gets overwritten
		originalErr := err

		// Update run as failed
		completedAt := time.Now()
		errorMsg := fmt.Sprintf("Stdio MCP execution failed: %v", originalErr)

		updateErr := repos.AgentRuns.UpdateCompletionWithMetadata(
			ctx,
			agentRun.ID,
			errorMsg,
			0,   // steps_taken
			nil, // tool_calls
			nil, // execution_steps
			"failed",
			&completedAt,
			nil,       // inputTokens
			nil,       // outputTokens
			nil,       // totalTokens
			nil,       // durationSeconds
			nil,       // modelName
			nil,       // toolsUsed
			&errorMsg, // error
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

	// Determine status based on execution result success
	status := "completed"
	var errorMsg *string
	if !result.Success {
		status = "failed"
		if result.Error != "" {
			errorMsg = &result.Error
		}
	}

	err = repos.AgentRuns.UpdateCompletionWithMetadata(
		ctx,
		agentRun.ID,
		result.Response,
		result.StepsTaken,
		result.ToolCalls,
		result.ExecutionSteps,
		status,
		&completedAt,
		inputTokens,
		outputTokens,
		totalTokens,
		&durationSeconds,
		&result.ModelName,
		toolsUsed,
		errorMsg,
	)
	if err != nil {
		return fmt.Errorf("failed to update agent run: %w", err)
	}

	// Get updated run and display results
	updatedRun, err := repos.AgentRuns.GetByID(ctx, agentRun.ID)
	if err != nil {
		return fmt.Errorf("failed to get updated run: %w", err)
	}

	// Track agent execution telemetry
	if h.telemetryService != nil {
		h.telemetryService.TrackAgentExecuted(
			agent.ID,
			int64(result.Duration.Milliseconds()),
			result.Success,
			int(result.StepsTaken),
		)
	}

	// 🚀 Lighthouse Integration: Send telemetry AFTER execution completes (same as MCP flow)
	// This ensures CLI execution sends telemetry data to CloudShip just like MCP mode
	if lighthouseClient != nil && lighthouseClient.IsRegistered() {
		// DEBUG: File logging to verify telemetry (matches MCP debug approach)
		debugFile := "/tmp/station-lighthouse-debug.log"
		debugLog := func(msg string) {
			os.WriteFile(debugFile, []byte(fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)), os.ModeAppend|0644)
		}

		debugLog(fmt.Sprintf("CLI Lighthouse integration starting for run %d", agentRun.ID))
		debugLog(fmt.Sprintf("CLI Agent Info: Name='%s', ID=%d", agent.Name, agent.ID))
		debugLog(fmt.Sprintf("CLI Result: Success=%t, Duration=%v", result.Success, result.Duration))
		// IMPORTANT: CLI mode uses SYNCHRONOUS telemetry to avoid client shutdown race condition
		// Unlike MCP mode, CLI lighthouse client shuts down immediately after agent completion
		func() {
			defer func() {
				if r := recover(); r != nil {
					debugLog(fmt.Sprintf("CLI Lighthouse telemetry panic: %v", r))
				}
			}()

			// Convert result to lighthouse format (simplified version)
			status := "completed"
			if !result.Success {
				status = "failed"
			}

			// Calculate times based on result duration
			completedAt := time.Now()
			startedAt := completedAt.Add(-result.Duration)

			// Generate UUID for run ID to prevent collisions across multiple stations
			runUUID := uuid.New().String()

			// Create proper types.AgentRun structure (same as MCP conversion function)
			lighthouseRun := &types.AgentRun{
				ID:             runUUID,
				AgentID:        fmt.Sprintf("agent_%d", agent.ID),
				AgentName:      agent.Name,
				Task:           task,
				Response:       result.Response,
				Status:         status,
				DurationMs:     result.Duration.Milliseconds(),
				ModelName:      result.ModelName,
				StartedAt:      startedAt,
				CompletedAt:    completedAt, // Use time.Time not pointer
				ToolCalls:      convertToolCallsToLighthouse(result.ToolCalls),
				ExecutionSteps: convertExecutionStepsToLighthouse(result.ExecutionSteps),
				TokenUsage: &types.TokenUsage{
					PromptTokens:     int(getValueOrZero(inputTokens)),
					CompletionTokens: int(getValueOrZero(outputTokens)),
					TotalTokens:      int(getValueOrZero(totalTokens)),
					CostUSD:          0.0, // Cost calculation not implemented in CLI
				},
				OutputSchema: func() string {
					if agent.OutputSchema != nil {
						return *agent.OutputSchema
					}
					return ""
				}(),
				OutputSchemaPreset: func() string {
					if agent.OutputSchemaPreset != nil {
						return *agent.OutputSchemaPreset
					}
					return ""
				}(),
				Metadata: map[string]string{
					"source":         "cli",
					"mode":           "cli",
					"run_uuid":       runUUID,
					"station_run_id": fmt.Sprintf("%d", agentRun.ID), // Keep local DB ID for correlation
				},
			}

			debugLog(fmt.Sprintf("CLI Lighthouse run created: ID=%s, AgentID=%s, Status=%s", lighthouseRun.ID, lighthouseRun.AgentID, lighthouseRun.Status))
			debugLog(fmt.Sprintf("Sending CLI run %d to lighthouse - Data: AgentID=%s, Status=%s, Response length=%d",
				agentRun.ID, lighthouseRun.AgentID, lighthouseRun.Status, len(lighthouseRun.Response)))

			// Send via lighthouse client (same as MCP flow)
			lighthouseClient.SendRun(lighthouseRun, "default", map[string]string{
				"source": "cli",
				"mode":   "cli",
			})

			lighthouse.RecordSuccess()
			debugLog(fmt.Sprintf("Lighthouse telemetry sent successfully for CLI run %d", agentRun.ID))

			// 🚀 Dual Flow: Send structured data if conditions are met (same logic as AgentExecutionEngine)
			sendStructuredDataIfEligible := func() {
				// Extract app/app_type metadata from dotprompt file (same as AgentExecutionEngine)
				app := result.App
				appType := result.AppType

				// Fallback: Check if agent has preset-based app/app_type
				if app == "" && appType == "" && agent.OutputSchemaPreset != nil && *agent.OutputSchemaPreset != "" {
					debugLog(fmt.Sprintf("No app/app_type from frontmatter for agent %d, trying preset fallback: %s", agent.ID, *agent.OutputSchemaPreset))
					switch *agent.OutputSchemaPreset {
					case "finops":
						app = "finops"
						appType = "cost-analysis"
						debugLog(fmt.Sprintf("✅ Applied finops preset for agent %d: app='%s', app_type='%s'", agent.ID, app, appType))
					}
				}

				// Skip if no app/app_type identified (either from frontmatter or preset fallback)
				if app == "" || appType == "" {
					debugLog(fmt.Sprintf("No app/app_type metadata found for agent %d (app='%s', app_type='%s'), skipping structured data ingestion", agent.ID, app, appType))
					return
				}

				// Skip if agent execution failed (no meaningful structured data)
				if !result.Success {
					debugLog(fmt.Sprintf("Agent execution failed for agent %d, skipping structured data ingestion", agent.ID))
					return
				}

				// Attempt to parse the response as structured JSON
				var structuredData map[string]interface{}
				if err := json.Unmarshal([]byte(result.Response), &structuredData); err != nil {
					debugLog(fmt.Sprintf("Agent response is not valid JSON for agent %d, skipping structured data ingestion: %v", agent.ID, err))
					return
				}

				// Prepare metadata for ingestion
				metadata := map[string]string{
					"source":            "cli",
					"mode":              "cli",
					"agent_id":          fmt.Sprintf("%d", agent.ID),
					"agent_name":        agent.Name,
					"run_id":            fmt.Sprintf("%d", agentRun.ID),
					"execution_success": fmt.Sprintf("%t", result.Success),
					"duration_ms":       fmt.Sprintf("%d", result.Duration.Milliseconds()),
				}

				if agent.OutputSchemaPreset != nil {
					metadata["output_schema_preset"] = *agent.OutputSchemaPreset
				}

				// Send structured data to CloudShip Data Ingestion service
				// Use UUID for correlation to prevent collisions across multiple stations
				correlationID := uuid.New().String()
				runID := fmt.Sprintf("%d", agentRun.ID)
				agentName := agent.Name
				agentID := fmt.Sprintf("%d", agent.ID)
				if err := lighthouseClient.IngestData(app, appType, structuredData, metadata, correlationID, runID, agentName, agentID); err != nil {
					debugLog(fmt.Sprintf("Failed to send structured data to CloudShip: %v", err))
					// Don't fail the execution - this is supplementary data
				} else {
					debugLog(fmt.Sprintf("Successfully sent structured data to CloudShip (app: %s, app_type: %s, run_id: %d)", app, appType, agentRun.ID))
					fmt.Printf("✅ Structured data sent to CloudShip (app: %s, app_type: %s)\n", app, appType)
				}
			}

			// Execute dual flow structured data ingestion
			sendStructuredDataIfEligible()

			fmt.Printf("✅ Lighthouse telemetry sent for CLI run %d\n", agentRun.ID)
		}()
	} else if lighthouseClient != nil {
		lighthouse.RecordError("CLI: Lighthouse client is not registered")
	} else {
		lighthouse.RecordError("CLI: Lighthouse client is not initialized")
	}

	// Stop Lighthouse client
	if lighthouseClient != nil {
		if err := lighthouseClient.Close(); err != nil {
			fmt.Printf("Warning: Error stopping Lighthouse client: %v\n", err)
		}
	}

	// Mark execution as completed to prevent signal handler from updating status
	executionCompleted = true

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
	defer func() { _ = resp.Body.Close() }()

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
	cfg, err := config.Load()
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
		run, err := freshRepos.AgentRuns.GetByID(context.Background(), runID)
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

// convertToolCallsToLighthouse converts tool calls to lighthouse format (simplified version of MCP function)
func convertToolCallsToLighthouse(toolCalls interface{}) []types.ToolCall {
	if toolCalls == nil {
		return nil
	}

	// Handle the case where toolCalls is a pointer to a slice
	if ptr, ok := toolCalls.(*[]interface{}); ok && ptr != nil {
		toolCalls = *ptr
	}

	calls, ok := toolCalls.([]interface{})
	if !ok {
		return nil
	}

	var lighthouseCalls []types.ToolCall
	for _, call := range calls {
		if callMap, ok := call.(map[string]interface{}); ok {
			toolCall := types.ToolCall{
				ToolName:   getStringFromMap(callMap, "tool_name"),
				Parameters: callMap["parameters"],
				Timestamp:  time.Now(),
			}
			lighthouseCalls = append(lighthouseCalls, toolCall)
		}
	}
	return lighthouseCalls
}

// convertExecutionStepsToLighthouse converts execution steps to lighthouse format (simplified version of MCP function)
func convertExecutionStepsToLighthouse(executionSteps interface{}) []types.ExecutionStep {
	if executionSteps == nil {
		return nil
	}

	// Handle the case where executionSteps is a pointer to a slice
	if ptr, ok := executionSteps.(*[]interface{}); ok && ptr != nil {
		executionSteps = *ptr
	}

	steps, ok := executionSteps.([]interface{})
	if !ok {
		return nil
	}

	var lighthouseSteps []types.ExecutionStep
	for _, step := range steps {
		if stepMap, ok := step.(map[string]interface{}); ok {
			execStep := types.ExecutionStep{
				StepNumber:  getIntFromMap(stepMap, "step_number"),
				Description: getStringFromMap(stepMap, "description"),
				Type:        getStringFromMap(stepMap, "type"),
				DurationMs:  int64(getIntFromMap(stepMap, "duration_ms")),
				Timestamp:   time.Now(),
			}
			lighthouseSteps = append(lighthouseSteps, execStep)
		}
	}
	return lighthouseSteps
}

// Helper functions for type conversion
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getIntFromMap(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		if i, ok := val.(int); ok {
			return i
		}
		if f, ok := val.(float64); ok {
			return int(f)
		}
	}
	return 0
}

func getValueOrZero(ptr *int64) int64 {
	if ptr != nil {
		return *ptr
	}
	return 0
}
