package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
	"station/cmd/main/handlers/common"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/theme"
	"station/pkg/models"
	agent_bundle "station/pkg/agent-bundle"
	"station/pkg/agent-bundle/creator"
	"station/pkg/agent-bundle/manager"
	"station/pkg/agent-bundle/validator"
	"station/pkg/bundle"
	"station/pkg/dotprompt"
)

// extractInt64FromTokenUsage safely extracts int64 from various numeric types in token usage
func extractInt64FromTokenUsage(value interface{}) *int64 {
	if value == nil {
		return nil
	}
	
	switch v := value.(type) {
	case int64:
		return &v
	case int:
		val := int64(v)
		return &val
	case int32:
		val := int64(v)
		return &val
	case float64:
		val := int64(v)
		return &val
	case float32:
		val := int64(v)
		return &val
	default:
		return nil
	}
}

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
		return fmt.Errorf("agent name is required")
	}

	agentName := args[0]
	
	// Get environment from flag or default
	environment, _ := cmd.Flags().GetString("env")
	if environment == "" {
		environment = "default"
	}

	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render(fmt.Sprintf("🤖 Agent: %s", agentName))
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Showing agent from: " + endpoint))
		return fmt.Errorf("remote agent show with names not yet implemented")
	} else {
		fmt.Println(styles.Info.Render("🏠 Showing local agent"))
		return h.showAgentLocalByName(agentName, environment)
	}
}

// RunAgentRun executes an agent using dotprompt methodology
func (h *AgentHandler) RunAgentRun(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("agent name and task are required")
	}

	agentName := args[0]
	task := args[1]
	
	// Get environment from flag or default
	environment, _ := cmd.Flags().GetString("env")
	if environment == "" {
		environment = "default"
	}
	
	tail, _ := cmd.Flags().GetBool("tail")

	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render(fmt.Sprintf("▶️  Running Agent: %s", agentName))
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Running agent on: " + endpoint))
		// TODO: Update runAgentRemote to use names instead of IDs
		return fmt.Errorf("remote agent execution with names not yet implemented")
	} else {
		fmt.Println(styles.Info.Render("🚀 Running local agent with full execution engine"))
		return h.runAgentLocalWithFullEngine(agentName, task, environment, tail)
	}
}

// runAgentLocalDotprompt executes an agent using database-driven ai.Generate() with multi-step tool support
func (h *AgentHandler) runAgentLocalDotprompt(agentName, task, environment string, tail bool) error {
	styles := common.GetCLIStyles(h.themeManager)
	
	// 1. Connect to database and get agent configuration (runtime source of truth)
	cfg, err := common.LoadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	
	// 2. Get environment ID
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found: %w", environment, err)
	}
	
	// 3. Get agents in environment and find by name (runtime source of truth)
	agents, err := repos.Agents.ListByEnvironment(env.ID)
	if err != nil {
		return fmt.Errorf("failed to get agents from environment '%s': %w", environment, err)
	}
	
	var agent *models.Agent
	for _, a := range agents {
		if a.Name == agentName {
			agent = a
			break
		}
	}
	
	if agent == nil {
		return fmt.Errorf("agent '%s' not found in environment '%s'", agentName, environment)
	}
	
	fmt.Println(styles.Info.Render(fmt.Sprintf("🤖 Agent: %s", agent.Name)))
	fmt.Println(styles.Info.Render(fmt.Sprintf("📝 Description: %s", agent.Description)))
	
	// 4. Get agent tools from database
	agentTools, err := repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		return fmt.Errorf("failed to get agent tools: %w", err)
	}
	
	fmt.Printf("🔧 Tools Available: %d\n", len(agentTools))
	for _, tool := range agentTools {
		fmt.Printf("    • %s\n", tool.ToolName)
	}
	
	fmt.Println(styles.Info.Render(fmt.Sprintf("🚀 Executing task: %s", task)))
	if tail {
		fmt.Println(styles.Info.Render("👀 Following execution with real-time output..."))
	}
	
	// 5. Get console user and create agent run record
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		return fmt.Errorf("failed to get console user: %w", err)
	}
	
	agentRun, err := repos.AgentRuns.Create(
		agent.ID,
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

	// 6. Execute using hybrid approach: database config + dotprompt rendering + real execution
	// TODO: This CLI execution path needs to be updated to use the new dotprompt system
	// For now, use the traditional execution engine path
	creator := services.NewIntelligentAgentCreator(repos, services.NewAgentService(repos))
	result, err := creator.ExecuteAgentViaStdioMCP(context.Background(), agent, task, agentRun.ID)
	
	// Convert to expected response format
	response := &dotprompt.ExecutionResponse{
		Success:   result.Success,
		Response:  result.Response,
		Duration:  result.Duration,
		ModelName: result.ModelName,
		StepsUsed: result.StepsUsed,
		ToolsUsed: result.ToolsUsed,
		Error:     result.Error,
	}
	if err != nil {
		// Update run as failed
		completedAt := time.Now()
		durationSeconds := time.Since(time.Now().Add(-time.Second)).Seconds() // Minimal duration for failed runs
		
		updateErr := repos.AgentRuns.UpdateCompletionWithMetadata(
			agentRun.ID,
			fmt.Sprintf("Execution failed: %v", err),
			0, // steps_taken
			nil, // tool_calls
			nil, // execution_steps  
			"failed",
			&completedAt,
			nil, nil, nil, // token usage
			&durationSeconds,
			&response.ModelName,
			nil, // tools_used
		)
		if updateErr != nil {
			fmt.Printf("Warning: Failed to update failed run record: %v\n", updateErr)
		}
		
		fmt.Println(styles.Error.Render(fmt.Sprintf("❌ Execution failed: %v", err)))
		return fmt.Errorf("agent execution failed: %w", err)
	}
	
	// 7. Save successful execution to database
	completedAt := time.Now()
	durationSeconds := response.Duration.Seconds()
	
	// Extract token usage if available
	var inputTokens, outputTokens, totalTokens *int64
	if response.TokenUsage != nil {
		if val, ok := response.TokenUsage["input_tokens"].(int64); ok {
			inputTokens = &val
		} else if val, ok := response.TokenUsage["input_tokens"].(float64); ok {
			inputVal := int64(val)
			inputTokens = &inputVal
		}
		if val, ok := response.TokenUsage["output_tokens"].(int64); ok {
			outputTokens = &val
		} else if val, ok := response.TokenUsage["output_tokens"].(float64); ok {
			outputVal := int64(val)
			outputTokens = &outputVal
		}
		if val, ok := response.TokenUsage["total_tokens"].(int64); ok {
			totalTokens = &val
		} else if val, ok := response.TokenUsage["total_tokens"].(float64); ok {
			totalVal := int64(val)
			totalTokens = &totalVal
		}
	}
	
	toolsUsed := int64(response.ToolsUsed)
	stepsTaken := int64(response.StepsUsed)
	
	err = repos.AgentRuns.UpdateCompletionWithMetadata(
		agentRun.ID,
		response.Response,
		stepsTaken,
		nil, // tool_calls (dotprompt doesn't provide detailed tool calls)
		nil, // execution_steps
		"completed",
		&completedAt,
		inputTokens,
		outputTokens,
		totalTokens,
		&durationSeconds,
		&response.ModelName,
		&toolsUsed,
	)
	if err != nil {
		fmt.Printf("Warning: Failed to update run record: %v\n", err)
	}

	// 8. Display results
	fmt.Println("\n" + styles.Banner.Render("🎯 Execution Results"))
	fmt.Printf("⏱️  Duration: %v\n", response.Duration)
	fmt.Printf("✅ Success: %t\n", response.Success)
	fmt.Printf("🤖 Model: %s\n", response.ModelName)
	fmt.Printf("📊 Steps Used: %d\n", response.StepsUsed)
	fmt.Printf("🔧 Tools Used: %d\n", response.ToolsUsed)
	
	if response.Response != "" {
		fmt.Println("\n" + styles.Info.Render("📄 Response:"))
		fmt.Println(response.Response)
	}
	
	if response.Error != "" {
		fmt.Println("\n" + styles.Error.Render("❌ Error:"))
		fmt.Println(response.Error)
		return fmt.Errorf("agent execution error: %s", response.Error)
	}
	
	return nil
}

// runAgentLocalWithFullEngine executes an agent using the full AgentExecutionEngine with detailed capture
func (h *AgentHandler) runAgentLocalWithFullEngine(agentName, task, environment string, tail bool) error {
	styles := common.GetCLIStyles(h.themeManager)
	
	// 1. Connect to database and get agent configuration
	cfg, err := common.LoadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	
	// 2. Get environment and agent
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found: %w", environment, err)
	}
	
	agents, err := repos.Agents.ListByEnvironment(env.ID)
	if err != nil {
		return fmt.Errorf("failed to get agents from environment '%s': %w", environment, err)
	}
	
	var agent *models.Agent
	for _, a := range agents {
		if a.Name == agentName {
			agent = a
			break
		}
	}
	
	if agent == nil {
		return fmt.Errorf("agent '%s' not found in environment '%s'", agentName, environment)
	}
	
	fmt.Println(styles.Info.Render(fmt.Sprintf("🤖 Agent: %s", agent.Name)))
	fmt.Println(styles.Info.Render(fmt.Sprintf("📝 Description: %s", agent.Description)))
	
	// 3. Get agent tools
	agentTools, err := repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		return fmt.Errorf("failed to get agent tools: %w", err)
	}
	
	fmt.Printf("🔧 Tools Available: %d\n", len(agentTools))
	for _, tool := range agentTools {
		fmt.Printf("    • %s\n", tool.ToolName)
	}
	
	fmt.Println(styles.Info.Render(fmt.Sprintf("🚀 Executing task: %s", task)))
	if tail {
		fmt.Println(styles.Info.Render("👀 Following execution with real-time output..."))
	}
	
	// 4. Get console user and create agent run record  
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		return fmt.Errorf("failed to get console user: %w", err)
	}
	
	agentRun, err := repos.AgentRuns.Create(
		agent.ID,
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

	// 5. Use the full AgentExecutionEngine for detailed execution
	creator := services.NewIntelligentAgentCreator(repos, nil)
	ctx := context.Background()
	
	result, err := creator.ExecuteAgentViaStdioMCP(ctx, agent, task, agentRun.ID)
	if err != nil {
		// Update run as failed
		completedAt := time.Now()
		durationSeconds := time.Since(time.Now().Add(-time.Second)).Seconds()
		
		updateErr := repos.AgentRuns.UpdateCompletionWithMetadata(
			agentRun.ID,
			fmt.Sprintf("Execution failed: %v", err),
			0, // steps_taken
			nil, // tool_calls
			nil, // execution_steps  
			"failed",
			&completedAt,
			nil, nil, nil, // token usage
			&durationSeconds,
			nil, // model name
			nil, // tools_used
		)
		if updateErr != nil {
			fmt.Printf("Warning: Failed to update failed run record: %v\n", updateErr)
		}
		
		fmt.Println(styles.Error.Render(fmt.Sprintf("❌ Execution failed: %v", err)))
		return fmt.Errorf("agent execution failed: %w", err)
	}

	// 6. Update run as completed with detailed execution metadata
	completedAt := time.Now()
	durationSeconds := result.Duration.Seconds()
	
	// Extract token usage from result using robust type conversion
	var inputTokens, outputTokens, totalTokens *int64
	var toolsUsed *int64
	
	if result.TokenUsage != nil {
		// Handle input_tokens with multiple numeric types
		if inputVal := extractInt64FromTokenUsage(result.TokenUsage["input_tokens"]); inputVal != nil {
			inputTokens = inputVal
		}
		
		// Handle output_tokens with multiple numeric types
		if outputVal := extractInt64FromTokenUsage(result.TokenUsage["output_tokens"]); outputVal != nil {
			outputTokens = outputVal
		}
		
		// Handle total_tokens with multiple numeric types
		if totalVal := extractInt64FromTokenUsage(result.TokenUsage["total_tokens"]); totalVal != nil {
			totalTokens = totalVal
		}
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
		fmt.Printf("Warning: Failed to update run record: %v\n", err)
	}

	// 7. Get the updated run for detailed display
	updatedRun, err := repos.AgentRuns.GetByID(agentRun.ID)
	if err != nil {
		fmt.Printf("Warning: Failed to get updated run: %v\n", err)
		// Fall back to displaying basic results
		fmt.Println("\n" + styles.Banner.Render("🎯 Execution Results"))
		fmt.Printf("⏱️  Duration: %v\n", result.Duration)
		fmt.Printf("✅ Success: %t\n", result.Success)
		fmt.Printf("🤖 Model: %s\n", result.ModelName)
		fmt.Printf("📊 Steps Used: %d\n", result.StepsUsed)
		fmt.Printf("🔧 Tools Used: %d\n", result.ToolsUsed)
		
		if result.Response != "" {
			fmt.Println("\n" + styles.Info.Render("📄 Response:"))
			fmt.Println(result.Response)
		}
		return nil
	}

	// 8. Display detailed results with all the captured tool calls and execution steps
	return h.displayExecutionResults(updatedRun)
}

// displayExecutionResults shows the final execution results with tool calls
func (h *AgentHandler) displayExecutionResults(run *models.AgentRun) error {
	styles := common.GetCLIStyles(h.themeManager)
	
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

// showAgentLocalByName shows agent details by name using dotprompt file
func (h *AgentHandler) showAgentLocalByName(agentName, environment string) error {
	styles := common.GetCLIStyles(h.themeManager)
	
	// Construct .prompt file path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	promptFilePath := fmt.Sprintf("%s/.config/station/environments/%s/agents/%s.prompt", homeDir, environment, agentName)
	
	// Check if .prompt file exists
	if _, err := os.Stat(promptFilePath); os.IsNotExist(err) {
		return fmt.Errorf("agent '%s' not found in environment '%s' (file: %s)", agentName, environment, promptFilePath)
	}
	
	fmt.Println(styles.Info.Render(fmt.Sprintf("📄 Loading agent from: %s", promptFilePath)))
	
	// Parse the .prompt file
	extractor, err := dotprompt.NewRuntimeExtraction(promptFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse dotprompt file: %w", err)
	}
	
	config := extractor.GetConfig()
	
	// Display agent information
	fmt.Printf("\n📋 %s\n", styles.Info.Render("Agent Details:"))
	fmt.Printf("   Name: %s\n", config.Metadata.Name)
	if config.Metadata.Description != "" {
		fmt.Printf("   Description: %s\n", config.Metadata.Description)
	}
	if config.Metadata.Version != "" {
		fmt.Printf("   Version: %s\n", config.Metadata.Version)
	}
	fmt.Printf("   Environment: %s\n", environment)
	
	// Display model configuration
	fmt.Printf("\n🤖 %s\n", styles.Info.Render("Model Configuration:"))
	fmt.Printf("   Model: %s\n", config.Model)
	if config.Config.Temperature != nil {
		fmt.Printf("   Temperature: %.1f\n", *config.Config.Temperature)
	}
	if config.Config.MaxTokens != nil {
		fmt.Printf("   Max Tokens: %d\n", *config.Config.MaxTokens)
	}
	
	// Display tools
	if len(config.Tools) > 0 {
		fmt.Printf("\n🔧 %s\n", styles.Info.Render("Tools:"))
		for _, tool := range config.Tools {
			fmt.Printf("   • %s\n", tool)
		}
	}
	
	// Display MCP dependencies
	if mcpDeps, err := extractor.ExtractCustomField("station.mcp_dependencies"); err == nil && mcpDeps != nil {
		fmt.Printf("\n🔗 %s\n", styles.Info.Render("MCP Dependencies:"))
		if depsMap, ok := mcpDeps.(map[string]interface{}); ok {
			for mcpConfig, deps := range depsMap {
				fmt.Printf("   • %s:\n", mcpConfig)
				if depMap, ok := deps.(map[string]interface{}); ok {
					if tools, exists := depMap["assigned_tools"]; exists {
						if toolsList, ok := tools.([]interface{}); ok {
							fmt.Printf("     Tools: %v\n", toolsList)
						}
					}
					if cmd, exists := depMap["server_command"]; exists {
						fmt.Printf("     Command: %v\n", cmd)
					}
					if envVars, exists := depMap["environment_vars"]; exists {
						fmt.Printf("     Environment Variables: %v\n", envVars)
					}
				}
			}
		}
	}
	
	// Display execution metadata
	if metadata, err := extractor.ExtractCustomField("station.execution_metadata"); err == nil && metadata != nil {
		fmt.Printf("\n📊 %s\n", styles.Info.Render("Execution Metadata:"))
		if metadataMap, ok := metadata.(map[string]interface{}); ok {
			for key, value := range metadataMap {
				fmt.Printf("   %s: %v\n", key, value)
			}
		}
	}
	
	// Display feature flags
	if flags, err := extractor.ExtractCustomField("station.feature_flags"); err == nil && flags != nil {
		fmt.Printf("\n🚩 %s\n", styles.Info.Render("Feature Flags:"))
		if flagsMap, ok := flags.(map[string]interface{}); ok {
			for key, value := range flagsMap {
				fmt.Printf("   %s: %v\n", key, value)
			}
		}
	}
	
	return nil
}

// RunAgentDelete deletes an agent by name
func (h *AgentHandler) RunAgentDelete(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent name is required")
	}

	agentName := args[0]
	
	// Get environment from flag or default
	environment, _ := cmd.Flags().GetString("env")
	if environment == "" {
		environment = "default"
	}

	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render(fmt.Sprintf("🗑️  Delete Agent: %s", agentName))
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint != "" {
		fmt.Println(styles.Error.Render("❌ Deleting remote agents is not currently supported"))
		return fmt.Errorf("remote agent deletion not supported")
	} else {
		fmt.Println(styles.Info.Render("🏠 Deleting local agent"))
		return h.deleteAgentLocalByName(agentName, environment)
	}
}

// deleteAgentLocalByName deletes an agent by name from a specific environment
func (h *AgentHandler) deleteAgentLocalByName(agentName, environment string) error {
	// Load Station config and connect to database
	cfg, err := common.LoadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	
	// Get environment
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found: %w", environment, err)
	}
	
	// Find agent by name in the environment
	agents, err := repos.Agents.ListByEnvironment(env.ID)
	if err != nil {
		return fmt.Errorf("failed to get agents from environment '%s': %w", environment, err)
	}
	
	var agent *models.Agent
	for _, a := range agents {
		if a.Name == agentName {
			agent = a
			break
		}
	}
	
	if agent == nil {
		return fmt.Errorf("agent '%s' not found in environment '%s'", agentName, environment)
	}

	// Delete the agent using AgentService for proper file cleanup
	agentService := services.NewAgentService(repos)
	err = agentService.DeleteAgent(context.Background(), agent.ID)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	styles := common.GetCLIStyles(h.themeManager)
	fmt.Printf("✅ Agent deleted: %s\n", styles.Success.Render(agent.Name))
	return nil
}

// RunAgentCreate creates a new agent
func (h *AgentHandler) RunAgentCreate(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("➕ Create Agent")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")
	domain, _ := cmd.Flags().GetString("domain")
	schedule, _ := cmd.Flags().GetString("schedule")
	environment, _ := cmd.Flags().GetString("env")

	if endpoint != "" {
		fmt.Println(styles.Error.Render("❌ Creating remote agents is not currently supported"))
		return fmt.Errorf("remote agent creation not supported")
	} else {
		fmt.Println(styles.Info.Render("🏠 Creating local agent"))
		return h.createAgentLocal(name, description, domain, schedule, environment)
	}
}

// RunAgentExport exports an agent to file-based config
func (h *AgentHandler) RunAgentExport(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📤 Export Agent")
	fmt.Println(banner)

	agentName := args[0]

	var environment string
	if len(args) > 1 {
		environment = args[1]
	} else {
		environment = "default"
	}

	fmt.Println(styles.Info.Render(fmt.Sprintf("🏠 Exporting agent '%s' to environment '%s'", agentName, environment)))
	return h.exportAgentLocalByName(agentName, environment)
}

// exportAgentLocalByName exports an agent by name to dotprompt format
func (h *AgentHandler) exportAgentLocalByName(agentName, environment string) error {
	// For now, since we don't have the new name-based database queries yet,
	// let's look up the agent by checking if a .prompt file exists and just
	// report that it's already in dotprompt format
	
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	promptFilePath := fmt.Sprintf("%s/.config/station/environments/%s/agents/%s.prompt", homeDir, environment, agentName)
	
	styles := common.GetCLIStyles(h.themeManager)
	
	if _, err := os.Stat(promptFilePath); err == nil {
		fmt.Println(styles.Info.Render(fmt.Sprintf("✅ Agent '%s' already exists in dotprompt format", agentName)))
		fmt.Println(styles.Info.Render(fmt.Sprintf("📄 File: %s", promptFilePath)))
		return nil
	}
	
	// Agent doesn't exist as .prompt file, try to export from database
	fmt.Println(styles.Info.Render(fmt.Sprintf("🔍 Looking up agent '%s' in database...", agentName)))
	return h.exportAgentFromDatabase(agentName, environment, promptFilePath)
}

// exportAgentFromDatabase exports an agent from database to .prompt file
func (h *AgentHandler) exportAgentFromDatabase(agentName, environment, promptFilePath string) error {
	styles := common.GetCLIStyles(h.themeManager)
	
	// Load configuration and connect to database
	cfg, err := common.LoadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}
	
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()
	
	repos := repositories.New(database)
	
	// Get environment ID
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found: %w", environment, err)
	}
	
	// Try to find agent by name (for now, we'll use a simple query)
	// TODO: Replace with proper name-based lookup when SQLC queries are ready
	agents, err := repos.Agents.ListByEnvironment(env.ID)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}
	
	var targetAgent *models.Agent
	for _, agent := range agents {
		if agent.Name == agentName {
			targetAgent = agent
			break
		}
	}
	
	if targetAgent == nil {
		return fmt.Errorf("agent '%s' not found in database", agentName)
	}
	
	fmt.Println(styles.Info.Render(fmt.Sprintf("📄 Found agent: %s (ID: %d)", targetAgent.Name, targetAgent.ID)))
	
	// Get agent tools
	tools, err := repos.AgentTools.ListAgentTools(targetAgent.ID)
	if err != nil {
		return fmt.Errorf("failed to get agent tools: %w", err)
	}
	
	// Generate .prompt file content
	promptContent := h.generateDotpromptContent(targetAgent, tools, environment)
	
	// Ensure directory exists
	agentsDir := filepath.Dir(promptFilePath)
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}
	
	// Write .prompt file
	if err := os.WriteFile(promptFilePath, []byte(promptContent), 0644); err != nil {
		return fmt.Errorf("failed to write .prompt file: %w", err)
	}
	
	fmt.Println(styles.Success.Render(fmt.Sprintf("✅ Successfully exported agent '%s'", agentName)))
	fmt.Println(styles.Info.Render(fmt.Sprintf("📄 Created: %s", promptFilePath)))
	
	return nil
}

// generateDotpromptContent generates the .prompt file content for an agent using multi-role format
func (h *AgentHandler) generateDotpromptContent(agent *models.Agent, tools []*models.AgentToolWithDetails, environment string) string {
	var content strings.Builder

	// Get configured model from Station config, fallback to default
	modelName := "gemini-1.5-flash" // default fallback
	// TODO: Add proper config service access for CLI handlers

	// YAML frontmatter with multi-role support
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("model: \"%s\"\n", modelName))
	content.WriteString("config:\n")
	content.WriteString("  temperature: 0.3\n")
	content.WriteString("  max_tokens: 2000\n")
	
	// Input schema with automatic userInput variable
	content.WriteString("input:\n")
	content.WriteString("  schema:\n")
	content.WriteString("    userInput: string\n")
	
	// Add default output schema for GenKit UI compatibility
	content.WriteString("output:\n")
	content.WriteString("  schema:\n")
	content.WriteString("    response: string\n")
	
	content.WriteString("metadata:\n")
	content.WriteString(fmt.Sprintf("  name: \"%s\"\n", agent.Name))
	if agent.Description != "" {
		content.WriteString(fmt.Sprintf("  description: \"%s\"\n", agent.Description))
	}
	content.WriteString("  version: \"1.0.0\"\n")

	// Tools section
	if len(tools) > 0 {
		content.WriteString("tools:\n")
		for _, tool := range tools {
			content.WriteString(fmt.Sprintf("  - \"%s\"\n", tool.ToolName))
		}
	}

	// Station metadata
	content.WriteString("station:\n")
	content.WriteString("  execution_metadata:\n")
	if agent.MaxSteps > 0 {
		content.WriteString(fmt.Sprintf("    max_steps: %d\n", agent.MaxSteps))
	}
	content.WriteString(fmt.Sprintf("    environment: \"%s\"\n", environment))
	content.WriteString(fmt.Sprintf("    agent_id: %d\n", agent.ID))
	content.WriteString(fmt.Sprintf("    created_at: \"%s\"\n", agent.CreatedAt.Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("    updated_at: \"%s\"\n", agent.UpdatedAt.Format(time.RFC3339)))
	content.WriteString("---\n\n")

	// Multi-role prompt content
	// Check if agent prompt is already multi-role
	if h.isMultiRolePrompt(agent.Prompt) {
		// Already multi-role, use as-is
		content.WriteString(agent.Prompt)
	} else {
		// Convert single prompt to multi-role format
		content.WriteString("{{role \"system\"}}\n")
		content.WriteString(agent.Prompt)
		content.WriteString("\n\n{{role \"user\"}}\n")
		content.WriteString("{{userInput}}")
	}
	content.WriteString("\n")

	return content.String()
}

// isMultiRolePrompt checks if a prompt already contains role directives
func (h *AgentHandler) isMultiRolePrompt(prompt string) bool {
	return strings.Contains(prompt, "{{role \"") || strings.Contains(prompt, "{{role '")
}

// RunAgentImport imports agents from file-based configs
func (h *AgentHandler) RunAgentImport(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📥 Import Agents")
	fmt.Println(banner)

	var environment string
	if len(args) > 0 {
		environment = args[0]
	} else {
		environment = "default"
	}

	fmt.Println(styles.Info.Render(fmt.Sprintf("🏠 Importing agents from environment '%s'", environment)))
	return h.importAgentsLocal(environment)
}

// Agent Bundle Handler Methods

// RunAgentBundleCreate creates a new agent bundle
func (h *AgentHandler) RunAgentBundleCreate(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🎁 Create Agent Bundle")
	fmt.Println(banner)

	bundlePath := args[0]

	// Get flags
	name, _ := cmd.Flags().GetString("name")
	author, _ := cmd.Flags().GetString("author")
	description, _ := cmd.Flags().GetString("description")
	agentType, _ := cmd.Flags().GetString("type")
	tags, _ := cmd.Flags().GetStringSlice("tags")

	// Validate required flags
	if author == "" {
		return fmt.Errorf("--author flag is required")
	}
	if description == "" {
		return fmt.Errorf("--description flag is required")
	}

	fmt.Println(styles.Info.Render(fmt.Sprintf("📁 Creating bundle at: %s", bundlePath)))

	// Create bundle creator with mock registry
	fs := afero.NewOsFs()
	mockRegistry := &MockBundleRegistry{}
	bundleCreator := creator.New(fs, mockRegistry)

	// Set up creation options
	opts := agent_bundle.CreateOptions{
		Name:        name,
		Author:      author,
		Description: description,
		AgentType:   agentType,
		Tags:        tags,
	}

	if err := bundleCreator.Create(bundlePath, opts); err != nil {
		fmt.Println(styles.Error.Render("❌ Failed to create bundle: " + err.Error()))
		return err
	}

	fmt.Println(styles.Success.Render("✅ Agent bundle created successfully!"))
	fmt.Println(styles.Info.Render("📝 Next steps:"))
	fmt.Println("   • Edit agent.json to configure your agent")
	fmt.Println("   • Update variables.schema.json with required variables")
	fmt.Println("   • Add MCP dependencies to manifest.json")
	fmt.Println("   • Run 'stn agent bundle validate " + bundlePath + "' to validate")
	
	return nil
}

// RunAgentBundleValidate validates an agent bundle
func (h *AgentHandler) RunAgentBundleValidate(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("✅ Validate Agent Bundle")
	fmt.Println(banner)

	bundlePath := args[0]
	fmt.Println(styles.Info.Render(fmt.Sprintf("📁 Validating bundle at: %s", bundlePath)))

	// Create validator
	fs := afero.NewOsFs()
	bundleValidator := validator.New(fs)

	// Validate the bundle
	result, err := bundleValidator.Validate(bundlePath)
	if err != nil {
		fmt.Println(styles.Error.Render("❌ Validation failed: " + err.Error()))
		return err
	}

	// Display results
	if result.Valid {
		fmt.Println(styles.Success.Render("✅ Bundle is valid!"))
	} else {
		fmt.Println(styles.Error.Render("❌ Bundle validation failed"))
	}

	// Show statistics
	stats := result.Statistics
	fmt.Printf("\n📊 Bundle Statistics:\n")
	fmt.Printf("   Variables: %d total (%d required, %d optional)\n", 
		stats.TotalVariables, stats.RequiredVariables, stats.OptionalVariables)
	fmt.Printf("   MCP Dependencies: %d\n", stats.MCPDependencies)
	fmt.Printf("   Tools: %d required, %d optional\n", stats.RequiredTools, stats.OptionalTools)

	// Show errors
	if len(result.Errors) > 0 {
		fmt.Printf("\n❌ Errors (%d):\n", len(result.Errors))
		for _, err := range result.Errors {
			fmt.Printf("   • %s", err.Message)
			if err.Suggestion != "" {
				fmt.Printf(" (💡 %s)", err.Suggestion)
			}
			fmt.Println()
		}
	}

	// Show warnings
	if len(result.Warnings) > 0 {
		fmt.Printf("\n⚠️  Warnings (%d):\n", len(result.Warnings))
		for _, warning := range result.Warnings {
			fmt.Printf("   • %s", warning.Message)
			if warning.Suggestion != "" {
				fmt.Printf(" (💡 %s)", warning.Suggestion)
			}
			fmt.Println()
		}
	}

	if !result.Valid {
		return fmt.Errorf("bundle validation failed")
	}
	
	return nil
}

// RunAgentBundleInstall installs an agent bundle
func (h *AgentHandler) RunAgentBundleInstall(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📦 Install Agent Bundle")
	fmt.Println(banner)

	bundlePath := args[0]
	environment := "default"
	if len(args) > 1 {
		environment = args[1]
	}

	// Get flags
	envFlag, _ := cmd.Flags().GetString("env")
	if envFlag != "" {
		environment = envFlag
	}

	variables, _ := cmd.Flags().GetStringToString("vars")
	varsFile, _ := cmd.Flags().GetString("vars-file")
	interactive, _ := cmd.Flags().GetBool("interactive")

	fmt.Println(styles.Info.Render(fmt.Sprintf("📁 Installing bundle from: %s", bundlePath)))
	fmt.Println(styles.Info.Render(fmt.Sprintf("🌍 Target environment: %s", environment)))

	// Handle vars-file
	var fileVars map[string]interface{}
	if varsFile != "" {
		var err error
		fileVars, err = h.loadVariablesFromFile(varsFile)
		if err != nil {
			fmt.Println(styles.Error.Render("❌ Failed to load variables file: " + err.Error()))
			return err
		}
	}

	// Start with file variables (preserving types), then override with CLI variables
	variablesMap := make(map[string]interface{})
	
	// Add file variables first
	if fileVars != nil {
		for k, v := range fileVars {
			variablesMap[k] = v
		}
	}
	
	// Add/override with CLI variables (convert strings to interface{})
	for k, v := range variables {
		variablesMap[k] = v
	}

	// Handle interactive mode - prompt for missing variables
	if interactive {
		// Load bundle to get variable schema
		fs := afero.NewOsFs()
		bundleValidator := validator.New(fs)
		validationResult, err := bundleValidator.Validate(bundlePath)
		if err != nil || !validationResult.Valid {
			fmt.Println(styles.Error.Render("❌ Cannot use interactive mode with invalid bundle"))
			return fmt.Errorf("bundle validation failed")
		}

		// Load manifest to get variable schema
		manifest, err := h.loadManifestFromPath(bundlePath)
		if err != nil {
			fmt.Println(styles.Error.Render("❌ Failed to load bundle manifest: " + err.Error()))
			return err
		}

		// Prompt for missing variables
		updatedVars, err := h.promptForVariables(manifest.RequiredVariables, variablesMap, styles)
		if err != nil {
			fmt.Println(styles.Error.Render("❌ Interactive variable collection failed: " + err.Error()))
			return err
		}
		variablesMap = updatedVars
	}

	// Create manager with dependencies
	fs := afero.NewOsFs()
	bundleValidator := validator.New(fs)
	// Use mock resolver for now - in production, you'd create a real resolver with:
	// realResolver := resolver.New(toolRepo, bundleRegistry)
	mockResolver := &MockResolver{}
	bundleManager := manager.New(fs, bundleValidator, mockResolver)

	// Install the bundle
	result, err := bundleManager.Install(bundlePath, environment, variablesMap)
	if err != nil {
		fmt.Println(styles.Error.Render("❌ Installation failed: " + err.Error()))
		return err
	}

	if !result.Success {
		fmt.Println(styles.Error.Render("❌ Installation failed: " + result.Error))
		return fmt.Errorf("installation failed")
	}

	fmt.Println(styles.Success.Render("✅ Agent bundle installed successfully!"))
	fmt.Printf("🤖 Agent ID: %d\n", result.AgentID)
	fmt.Printf("📝 Agent Name: %s\n", result.AgentName)
	fmt.Printf("🌍 Environment: %s\n", result.Environment)
	fmt.Printf("🔧 Tools Installed: %d\n", result.ToolsInstalled)
	if len(result.MCPBundles) > 0 {
		fmt.Printf("📦 MCP Bundles: %v\n", result.MCPBundles)
	}

	return nil
}

// RunAgentBundleDuplicate duplicates an agent across environments
func (h *AgentHandler) RunAgentBundleDuplicate(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("👥 Duplicate Agent")
	fmt.Println(banner)

	agentIDStr := args[0]
	targetEnv := args[1]

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	// Get flags
	name, _ := cmd.Flags().GetString("name")
	variables, _ := cmd.Flags().GetStringToString("vars")
	varsFile, _ := cmd.Flags().GetString("vars-file")
	interactive, _ := cmd.Flags().GetBool("interactive")

	fmt.Printf("🤖 Duplicating agent %d to environment '%s'\n", agentID, targetEnv)

	// Handle vars-file
	var fileVars map[string]interface{}
	if varsFile != "" {
		var err error
		fileVars, err = h.loadVariablesFromFile(varsFile)
		if err != nil {
			fmt.Println(styles.Error.Render("❌ Failed to load variables file: " + err.Error()))
			return err
		}
	}

	// Start with file variables (preserving types), then override with CLI variables
	variablesMap := make(map[string]interface{})
	
	// Add file variables first
	if fileVars != nil {
		for k, v := range fileVars {
			variablesMap[k] = v
		}
	}
	
	// Add/override with CLI variables (convert strings to interface{})
	for k, v := range variables {
		variablesMap[k] = v
	}

	// Interactive mode for duplicate - load original agent's schema from database
	if interactive {
		// This would require database integration to:
		// 1. Load original agent configuration  
		// 2. Determine what template it was created from (if any)
		// 3. Load the variable schema for that template
		// 4. Prompt for variables that are different or missing
		
		fmt.Println(styles.Info.Render("🎯 Interactive Duplicate Mode"))
		fmt.Println(styles.Info.Render("Loading original agent configuration..."))
		
		// For now, create a basic variable schema based on common agent properties
		basicSchema := map[string]agent_bundle.VariableSpec{
			"AGENT_NAME": {
				Type:        "string",
				Description: "Name for the duplicated agent",
				Required:    true,
			},
			"AGENT_DESCRIPTION": {
				Type:        "string", 
				Description: "Description for the duplicated agent",
				Required:    false,
			},
			"ENVIRONMENT": {
				Type:        "string",
				Description: "Target environment name",
				Required:    false,
				Default:     targetEnv,
			},
		}
		
		// Merge existing variables with interactive input
		updatedVars, err := h.promptForVariables(basicSchema, variablesMap, styles)
		if err != nil {
			fmt.Println(styles.Error.Render("❌ Interactive variable collection failed: " + err.Error()))
			return err
		}
		variablesMap = updatedVars
		
		// Update the name if provided
		if newName, exists := variablesMap["AGENT_NAME"]; exists {
			if nameStr, ok := newName.(string); ok && nameStr != "" {
				name = nameStr
			}
		}
	}

	// Create manager with dependencies
	fs := afero.NewOsFs()
	bundleValidator := validator.New(fs)
	mockResolver := &MockResolver{}
	bundleManager := manager.New(fs, bundleValidator, mockResolver)

	// Set up duplication options
	opts := agent_bundle.DuplicateOptions{
		Name:      name,
		Variables: variablesMap,
	}

	// Duplicate the agent
	result, err := bundleManager.Duplicate(agentID, targetEnv, opts)
	if err != nil {
		fmt.Println(styles.Error.Render("❌ Duplication failed: " + err.Error()))
		return err
	}

	if !result.Success {
		fmt.Println(styles.Error.Render("❌ Duplication failed: " + result.Error))
		return fmt.Errorf("duplication failed")
	}

	fmt.Println(styles.Success.Render("✅ Agent duplicated successfully!"))
	fmt.Printf("🤖 New Agent ID: %d\n", result.AgentID)
	fmt.Printf("📝 Agent Name: %s\n", result.AgentName)
	fmt.Printf("🌍 Environment: %s\n", result.Environment)

	return nil
}

// RunAgentBundleExport exports an agent as a template bundle
func (h *AgentHandler) RunAgentBundleExport(cmd *cobra.Command, args []string) error {
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📤 Export Agent Bundle")
	fmt.Println(banner)

	agentIDStr := args[0]
	outputPath := args[1]

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	// Get flags
	sourceEnv, _ := cmd.Flags().GetString("env")
	includeDeps, _ := cmd.Flags().GetBool("include-deps")
	includeExamples, _ := cmd.Flags().GetBool("include-examples")
	analyzeVars, _ := cmd.Flags().GetBool("analyze-vars")

	fmt.Printf("🤖 Exporting agent %d to: %s\n", agentID, outputPath)
	if sourceEnv != "" {
		fmt.Printf("🌍 Source environment: %s\n", sourceEnv)
	}

	// Create bundle creator with mock registry
	fs := afero.NewOsFs()
	mockRegistry := &MockBundleRegistry{}
	bundleCreator := creator.New(fs, mockRegistry)

	// Set up export options
	opts := agent_bundle.ExportOptions{
		IncludeDependencies: includeDeps,
		IncludeExamples:    includeExamples,
		VariableAnalysis:   analyzeVars,
		OutputFormat:       "bundle",
	}

	// Export the agent (this will require database integration)
	err = bundleCreator.ExportFromAgent(agentID, outputPath, opts)
	if err != nil {
		fmt.Println(styles.Error.Render("❌ Export failed: " + err.Error()))
		return err
	}

	fmt.Println(styles.Success.Render("✅ Agent exported successfully!"))
	fmt.Printf("📁 Bundle created at: %s\n", outputPath)
	fmt.Println(styles.Info.Render("📝 Next steps:"))
	fmt.Println("   • Review and customize the bundle files")
	fmt.Println("   • Run 'stn agent bundle validate " + outputPath + "' to validate")
	fmt.Println("   • Share or install the bundle in other environments")

	return nil
}

// MockResolver is a temporary mock for dependency resolution
type MockResolver struct{}

func (r *MockResolver) Resolve(ctx context.Context, deps []agent_bundle.MCPBundleDependency, env string) (*agent_bundle.ResolutionResult, error) {
	return &agent_bundle.ResolutionResult{
		Success: true,
		ResolvedBundles: []agent_bundle.MCPBundleRef{
			{Name: "filesystem-tools", Version: "1.0.0", Source: "registry"},
		},
		MissingBundles: []agent_bundle.MCPBundleDependency{},
		Conflicts:      []agent_bundle.ToolConflict{},
		InstallOrder:   []string{"filesystem-tools"},
	}, nil
}

func (r *MockResolver) InstallMCPBundles(ctx context.Context, bundles []agent_bundle.MCPBundleRef, env string) error {
	return nil
}

func (r *MockResolver) ValidateToolAvailability(ctx context.Context, tools []agent_bundle.ToolRequirement, env string) error {
	return nil
}

func (r *MockResolver) ResolveConflicts(conflicts []agent_bundle.ToolConflict) (*agent_bundle.ConflictResolution, error) {
	return &agent_bundle.ConflictResolution{
		Strategy:    "auto",
		Resolutions: make(map[string]string),
		Warnings:    []string{},
	}, nil
}

// MockBundleRegistry is a temporary mock for bundle registry
type MockBundleRegistry struct{}

func (r *MockBundleRegistry) List(ctx context.Context, opts bundle.ListOptions) ([]bundle.BundleManifest, error) {
	return []bundle.BundleManifest{}, nil
}

func (r *MockBundleRegistry) Get(ctx context.Context, name, version string) (*bundle.BundleManifest, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *MockBundleRegistry) Download(ctx context.Context, name, version string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *MockBundleRegistry) GetVersions(ctx context.Context, name string) ([]string, error) {
	return []string{}, nil
}

// Helper functions for interactive and file-based variable handling

// loadVariablesFromFile loads variables from JSON or YAML file
func (h *AgentHandler) loadVariablesFromFile(filePath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read variables file: %w", err)
	}

	variables := make(map[string]interface{})

	// Try JSON first, then YAML
	if err := json.Unmarshal(data, &variables); err != nil {
		if err := yaml.Unmarshal(data, &variables); err != nil {
			return nil, fmt.Errorf("failed to parse variables file as JSON or YAML: %w", err)
		}
	}

	return variables, nil
}

// loadManifestFromPath loads the agent bundle manifest from a path
func (h *AgentHandler) loadManifestFromPath(bundlePath string) (*agent_bundle.AgentBundleManifest, error) {
	manifestPath := fmt.Sprintf("%s/manifest.json", bundlePath)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest.json: %w", err)
	}

	var manifest agent_bundle.AgentBundleManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	return &manifest, nil
}

// promptForVariables interactively prompts user for missing or undefined variables
func (h *AgentHandler) promptForVariables(schema map[string]agent_bundle.VariableSpec, existingVars map[string]interface{}, styles common.CLIStyles) (map[string]interface{}, error) {
	fmt.Println(styles.Info.Render("🎯 Interactive Variable Collection"))
	fmt.Println()

	result := make(map[string]interface{})
	// Copy existing variables
	for k, v := range existingVars {
		result[k] = v
	}

	reader := bufio.NewReader(os.Stdin)

	// Sort variables by required first, then alphabetically
	var requiredVars, optionalVars []string
	for varName, spec := range schema {
		if spec.Required {
			requiredVars = append(requiredVars, varName)
		} else {
			optionalVars = append(optionalVars, varName)
		}
	}

	// Process required variables first
	if len(requiredVars) > 0 {
		fmt.Println(styles.Info.Render("📋 Required Variables:"))
		for _, varName := range requiredVars {
			spec := schema[varName]
			if _, exists := result[varName]; !exists {
				value, err := h.promptForSingleVariable(varName, spec, reader, styles)
				if err != nil {
					return nil, err
				}
				result[varName] = value
			} else {
				fmt.Printf("   ✅ %s: %v (already provided)\n", varName, h.formatVariableValue(result[varName], spec.Sensitive))
			}
		}
		fmt.Println()
	}

	// Process optional variables
	if len(optionalVars) > 0 {
		fmt.Println(styles.Info.Render("🔧 Optional Variables (press Enter to use default):"))
		for _, varName := range optionalVars {
			spec := schema[varName]
			if _, exists := result[varName]; !exists {
				value, err := h.promptForSingleVariable(varName, spec, reader, styles)
				if err != nil {
					return nil, err
				}
				if value != nil {
					result[varName] = value
				} else if spec.Default != nil {
					result[varName] = spec.Default
					fmt.Printf("   📌 %s: %v (using default)\n", varName, h.formatVariableValue(spec.Default, spec.Sensitive))
				}
			} else {
				fmt.Printf("   ✅ %s: %v (already provided)\n", varName, h.formatVariableValue(result[varName], spec.Sensitive))
			}
		}
	}

	fmt.Println(styles.Success.Render("✅ All variables collected!"))
	return result, nil
}

// promptForSingleVariable prompts for a single variable with proper input handling
func (h *AgentHandler) promptForSingleVariable(varName string, spec agent_bundle.VariableSpec, reader *bufio.Reader, styles common.CLIStyles) (interface{}, error) {
	// Build prompt
	prompt := fmt.Sprintf("   %s", varName)
	if spec.Description != "" {
		prompt += fmt.Sprintf(" (%s)", spec.Description)
	}
	if spec.Default != nil && !spec.Required {
		prompt += fmt.Sprintf(" [default: %v]", spec.Default)
	}
	prompt += ": "

	fmt.Print(prompt)

	var input string
	var err error

	// Handle sensitive variables (passwords, API keys)
	if spec.Sensitive || strings.Contains(strings.ToLower(varName), "password") || strings.Contains(strings.ToLower(varName), "secret") || strings.Contains(strings.ToLower(varName), "key") {
		// Read password without echo
		fmt.Print("[hidden] ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return nil, fmt.Errorf("failed to read sensitive input: %w", err)
		}
		input = string(bytePassword)
		fmt.Println() // Add newline after hidden input
	} else {
		// Read normal input
		input, err = reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.TrimSpace(input)
	}

	// Handle empty input for optional variables
	if input == "" && !spec.Required {
		return nil, nil // Will use default value
	}

	// Validate required variables
	if input == "" && spec.Required {
		fmt.Println(styles.Error.Render("   ❌ This variable is required"))
		return h.promptForSingleVariable(varName, spec, reader, styles)
	}

	// Type conversion based on variable spec
	return h.convertVariableValue(input, spec)
}

// convertVariableValue converts string input to appropriate type
func (h *AgentHandler) convertVariableValue(input string, spec agent_bundle.VariableSpec) (interface{}, error) {
	switch spec.Type {
	case "string", "secret":
		return input, nil
	case "number":
		if strings.Contains(input, ".") {
			return strconv.ParseFloat(input, 64)
		}
		return strconv.ParseInt(input, 10, 64)
	case "boolean":
		return strconv.ParseBool(input)
	default:
		return input, nil
	}
}

// formatVariableValue formats a variable value for display (masking sensitive ones)
func (h *AgentHandler) formatVariableValue(value interface{}, sensitive bool) string {
	if sensitive {
		return "••••••••"
	}
	return fmt.Sprintf("%v", value)
}