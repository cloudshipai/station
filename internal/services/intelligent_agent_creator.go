package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/telemetry"
	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	compat_oai "github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/firebase/genkit/go/plugins/mcp"
	"github.com/firebase/genkit/go/plugins/ollama"
	"github.com/openai/openai-go/option"
)

// IntelligentAgentCreator uses a Genkit agent with Station's own MCP server to intelligently create agents
type IntelligentAgentCreator struct {
	repos        *repositories.Repositories
	agentService AgentServiceInterface
	// mcpConfigSvc removed - using file-based configs only
	genkitApp    *genkit.Genkit
	mcpClient    *mcp.GenkitMCPClient
}

// AgentCreationRequest represents a request for intelligent agent creation
type AgentCreationRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	UserIntent  string `json:"user_intent"`
	Domain      string `json:"domain,omitempty"`
	Schedule    string `json:"schedule,omitempty"`
}

// AgentCreationPlan represents the intelligent agent creation plan
type AgentCreationPlan struct {
	AgentName        string   `json:"agent_name"`
	AgentDescription string   `json:"agent_description"`
	SystemPrompt     string   `json:"system_prompt"`
	RecommendedEnv   string   `json:"recommended_environment"`
	CoreTools        []string `json:"core_tools"`
	MaxSteps         int      `json:"max_steps"`
	Schedule         string   `json:"schedule"`
	Rationale        string   `json:"rationale"`
	SuccessCriteria  string   `json:"success_criteria"`
}

func NewIntelligentAgentCreator(repos *repositories.Repositories, agentService AgentServiceInterface) *IntelligentAgentCreator {
	return &IntelligentAgentCreator{
		repos:        repos,
		agentService: agentService,
	}
}

// initializeGenkit initializes Genkit with the configured AI provider and our MCP server
func (iac *IntelligentAgentCreator) initializeGenkit(ctx context.Context) error {
	if iac.genkitApp != nil {
		return nil // Already initialized
	}

	// Load Station configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize Genkit with the configured AI provider
	var genkitApp *genkit.Genkit
	switch strings.ToLower(cfg.AIProvider) {
	case "openai":
		// Validate API key for OpenAI
		if cfg.AIAPIKey == "" {
			return fmt.Errorf("STN_AI_API_KEY is required for OpenAI provider")
		}
		openaiPlugin := &compat_oai.OpenAI{
			APIKey: cfg.AIAPIKey,
		}
		// Set custom base URL if provided
		if cfg.AIBaseURL != "" {
			openaiPlugin.Opts = []option.RequestOption{
				option.WithBaseURL(cfg.AIBaseURL),
			}
		}
		genkitApp, err = genkit.Init(ctx, genkit.WithPlugins(openaiPlugin))
		if err != nil {
			return fmt.Errorf("failed to initialize Genkit with OpenAI: %w", err)
		}
	case "gemini":
		// Validate API key for Gemini
		if cfg.AIAPIKey == "" {
			return fmt.Errorf("STN_AI_API_KEY (or GOOGLE_API_KEY) is required for Gemini provider")
		}
		geminiPlugin := &googlegenai.GoogleAI{
			APIKey: cfg.AIAPIKey,
		}
		genkitApp, err = genkit.Init(ctx, genkit.WithPlugins(geminiPlugin))
		if err != nil {
			return fmt.Errorf("failed to initialize Genkit with Gemini: %w", err)
		}
	case "ollama":
		ollamaBaseURL := cfg.AIBaseURL
		if ollamaBaseURL == "" {
			ollamaBaseURL = "http://127.0.0.1:11434" // Default Ollama server
		}
		ollamaPlugin := &ollama.Ollama{
			ServerAddress: ollamaBaseURL,
		}
		genkitApp, err = genkit.Init(ctx, genkit.WithPlugins(ollamaPlugin))
		if err != nil {
			return fmt.Errorf("failed to initialize Genkit with Ollama: %w", err)
		}
	default:
		return fmt.Errorf("unsupported AI provider: %s (supported: openai, gemini, ollama)", cfg.AIProvider)
	}
	iac.genkitApp = genkitApp

	// Create MCP client to connect to our own stdio server
	mcpClient, err := mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
		Name:    "station-mcp",
		Version: "1.0.0",
		Stdio: &mcp.StdioConfig{
			Command: "./stn", // Use our own binary
			Args:    []string{"stdio"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create MCP client: %w", err)
	}
	iac.mcpClient = mcpClient

	log.Printf("🤖 Initialized Genkit with Station MCP client")
	return nil
}

// CreateIntelligentAgent uses Genkit with our own MCP server to analyze requirements and create an optimized agent
func (iac *IntelligentAgentCreator) CreateIntelligentAgent(ctx context.Context, req AgentCreationRequest) (*models.Agent, error) {
	log.Printf("🤖 Starting intelligent agent creation for: %s", req.Name)

	// Step 1: Initialize Genkit with our MCP server
	err := iac.initializeGenkit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Genkit: %w", err)
	}

	// Step 2: Use Genkit agent to analyze requirements and generate creation plan
	plan, err := iac.generateAgentPlanWithGenkit(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate agent plan: %w", err)
	}

	log.Printf("📋 Generated plan for agent '%s' with %d tools", plan.AgentName, len(plan.CoreTools))

	// Step 3: Find or create the recommended environment
	environmentID, err := iac.ensureEnvironment(plan.RecommendedEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure environment: %w", err)
	}

	// Step 4: Create the agent with intelligent configuration
	// Parse schedule from plan if provided
	var cronSchedule *string
	scheduleEnabled := false
	if plan.Schedule != "" && plan.Schedule != "on-demand" {
		cronSchedule = &plan.Schedule
		scheduleEnabled = true
	}
	
	agent, err := iac.repos.Agents.Create(
		plan.AgentName,
		plan.AgentDescription,
		plan.SystemPrompt,
		int64(plan.MaxSteps),
		environmentID,
		1, // Default user ID for now
		cronSchedule,
		scheduleEnabled,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Step 5: Assign tools to the agent
	assignedCount := 0
	if len(plan.CoreTools) > 0 {
		assignedCount = iac.assignToolsToAgent(agent.ID, plan.CoreTools, environmentID)
	}

	// Step 6: Handle scheduling if enabled
	if agent.IsScheduled && agent.CronSchedule != nil {
		log.Printf("📅 Agent '%s' has schedule '%s' - will be handled by scheduler service", agent.Name, *agent.CronSchedule)
		// The scheduler service will pick up this agent automatically on next restart,
		// or we can implement a notification mechanism here if needed
	}

	// Step 7: Cleanup
	if iac.mcpClient != nil {
		iac.mcpClient.Disconnect()
	}

	log.Printf("✅ Successfully created intelligent agent '%s' (ID: %d) with %d tools%s",
		agent.Name, agent.ID, assignedCount, 
		func() string {
			if agent.IsScheduled && agent.CronSchedule != nil {
				return fmt.Sprintf(" (scheduled: %s)", *agent.CronSchedule)
			}
			return ""
		}())

	return agent, nil
}

// generateAgentPlanWithGenkit uses Genkit with our MCP server to intelligently analyze requirements
func (iac *IntelligentAgentCreator) generateAgentPlanWithGenkit(ctx context.Context, req AgentCreationRequest) (*AgentCreationPlan, error) {
	log.Printf("🔍 Using Genkit to analyze agent requirements...")

	// Get available tools from our MCP server
	tools, err := iac.mcpClient.GetActiveTools(ctx, iac.genkitApp)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP tools: %w", err)
	}

	log.Printf("📋 Found %d available MCP tools for analysis", len(tools))

	// Convert tools to tool references
	var toolRefs []ai.ToolRef
	for _, tool := range tools {
		toolRefs = append(toolRefs, tool)
	}

	// Create prompt for agent analysis
	prompt := fmt.Sprintf(`You are an expert AI agent architect. Your task is to analyze the following agent requirements and create an optimal configuration.

Agent Requirements:
- Name: %s
- Description: %s
- User Intent: %s
- Domain: %s
- Schedule: %s

Available MCP Tools: You have access to %d MCP tools through the Station platform including file operations, directory management, search capabilities, and system information tools.

Your task:
1. Analyze the requirements and determine what tools this agent would need
2. Generate an appropriate system prompt for the agent
3. Determine optimal max steps (1-25 based on complexity)
4. Recommend an environment (default, development, production, staging)
5. Select 2-5 most relevant tools from the available MCP tools
6. Set appropriate schedule based on task requirements:
   - "on-demand" for interactive tasks
   - "0 0 0 * * *" for daily tasks (midnight) - 6-field format with seconds
   - "0 0 9 * * 1" for weekly tasks (Monday 9am) - 6-field format with seconds
   - "0 0 */6 * * *" for every 6 hours - 6-field format with seconds
   - Use 6-field cron format: "second minute hour day month weekday"

Please respond with a JSON object in this exact format:
{
  "agent_name": "%s",
  "agent_description": "%s", 
  "system_prompt": "detailed system prompt for the agent...",
  "recommended_environment": "environment_name",
  "core_tools": ["tool1", "tool2", "tool3"],
  "max_steps": 5,
  "schedule": "cron_expression_or_on_demand",
  "rationale": "explanation of decisions made",
  "success_criteria": "how to measure success"
}

Be intelligent about tool selection - only choose tools that are actually needed for the described task.`,
		req.Name, req.Description, req.UserIntent, req.Domain, req.Schedule, len(tools), req.Name, req.Description)

	// Get model name based on provider (reload config to get latest values)
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config for model selection: %w", err)
	}

	var modelName string
	switch strings.ToLower(cfg.AIProvider) {
	case "openai":
		// Use configured model or default
		if cfg.AIModel != "" {
			modelName = fmt.Sprintf("openai/%s", cfg.AIModel)
		} else {
			modelName = "openai/gpt-4o"
		}
	case "gemini":
		// Use configured model or default
		if cfg.AIModel != "" {
			modelName = fmt.Sprintf("googleai/%s", cfg.AIModel)
		} else {
			modelName = "googleai/gemini-pro"
		}
	case "ollama":
		// Use configured model or default
		if cfg.AIModel != "" {
			modelName = fmt.Sprintf("ollama/%s", cfg.AIModel)
		} else {
			modelName = "ollama/llama3"
		}
	default:
		modelName = "openai/gpt-4o" // Default fallback
	}

	// Use Genkit to generate the agent plan
	response, err := genkit.Generate(ctx, iac.genkitApp,
		ai.WithModelName(modelName),
		ai.WithPrompt(prompt),
		ai.WithTools(toolRefs...),
		ai.WithToolChoice(ai.ToolChoiceAuto),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate agent plan: %w", err)
	}

	// Parse the JSON response
	var plan AgentCreationPlan
	responseText := response.Text()
	log.Printf("🤖 Genkit response: %s", responseText)

	// Try to extract JSON from the response (might be wrapped in markdown)
	jsonStart := strings.Index(responseText, "{")
	jsonEnd := strings.LastIndex(responseText, "}") + 1
	if jsonStart == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	jsonStr := responseText[jsonStart:jsonEnd]
	err = json.Unmarshal([]byte(jsonStr), &plan)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent plan JSON: %w", err)
	}

	log.Printf("📋 Generated intelligent plan for '%s': %d tools, %d max steps",
		plan.AgentName, len(plan.CoreTools), plan.MaxSteps)

	return &plan, nil
}

// ensureEnvironment finds an existing environment or creates one if needed
func (iac *IntelligentAgentCreator) ensureEnvironment(envName string) (int64, error) {
	// First try to find existing environment
	environments, err := iac.repos.Environments.List()
	if err != nil {
		return 0, fmt.Errorf("failed to list environments: %w", err)
	}

	for _, env := range environments {
		if strings.EqualFold(env.Name, envName) {
			return env.ID, nil
		}
	}

	// If not found, use the first available environment (default)
	if len(environments) > 0 {
		log.Printf("Environment '%s' not found, using '%s' instead", envName, environments[0].Name)
		return environments[0].ID, nil
	}

	return 0, fmt.Errorf("no environments available")
}

// assignToolsToAgent assigns the specified tools to the agent using the repository API
func (iac *IntelligentAgentCreator) assignToolsToAgent(agentID int64, toolNames []string, environmentID int64) int {
	assignedCount := 0
	for _, toolName := range toolNames {
		// Find tool by name in the agent's environment
		tool, err := iac.repos.MCPTools.FindByNameInEnvironment(environmentID, toolName)
		if err != nil {
			log.Printf("Warning: Failed to find tool '%s' in environment %d: %v", toolName, environmentID, err)
			continue
		}
		
		// Add the tool assignment to the agent
		_, err = iac.repos.AgentTools.AddAgentTool(agentID, tool.ID)
		if err != nil {
			log.Printf("Warning: Failed to assign tool '%s' (ID: %d) to agent %d: %v", toolName, tool.ID, agentID, err)
			continue
		}
		
		assignedCount++
		log.Printf("✓ Successfully assigned tool '%s' (ID: %d) to agent %d", toolName, tool.ID, agentID)
	}
	return assignedCount
}

// TestStdioMCPConnection tests the connection to our stdio MCP server
func (iac *IntelligentAgentCreator) TestStdioMCPConnection(ctx context.Context) error {
	// Initialize Genkit if not already done
	err := iac.initializeGenkit(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize Genkit for stdio MCP test: %w", err)
	}

	// Test MCP connection by getting available tools
	tools, err := iac.mcpClient.GetActiveTools(ctx, iac.genkitApp)
	if err != nil {
		return fmt.Errorf("failed to connect to stdio MCP server: %w", err)
	}

	log.Printf("✅ Stdio MCP connection successful - found %d tools", len(tools))
	return nil
}

// AgentExecutionResult represents the result of agent execution via stdio MCP
type AgentExecutionResult struct {
	Response       string            `json:"response"`
	StepsTaken     int64             `json:"steps_taken"`
	ToolCalls      *models.JSONArray `json:"tool_calls,omitempty"`
	ExecutionSteps *models.JSONArray `json:"execution_steps,omitempty"`
}

// ExecuteAgentViaStdioMCP executes an agent using self-bootstrapping stdio MCP architecture
func (iac *IntelligentAgentCreator) ExecuteAgentViaStdioMCP(ctx context.Context, agent *models.Agent, task string, runID int64) (*AgentExecutionResult, error) {
	startTime := time.Now()
	log.Printf("🤖 Starting stdio MCP agent execution for agent '%s'", agent.Name)

	// Initialize Genkit + MCP if not already done
	err := iac.initializeGenkit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Genkit for agent execution: %w", err)
	}

	// Get tools assigned to this specific agent
	assignedTools, err := iac.repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assigned tools for agent %d: %w", agent.ID, err)
	}

	log.Printf("📋 Agent has %d assigned tools for execution", len(assignedTools))

	// Get MCP tools from agent's environment instead of Station stdio
	allTools, err := iac.getEnvironmentMCPTools(ctx, agent.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment MCP tools for agent %d: %w", agent.ID, err)
	}

	// Filter to only include tools assigned to this agent
	var tools []ai.ToolRef
	for _, assignedTool := range assignedTools {
		for _, mcpTool := range allTools {
			// Match by tool name - try multiple methods to get tool name  
			var toolName string
			if named, ok := mcpTool.(interface{ Name() string }); ok {
				toolName = named.Name()
			} else if stringer, ok := mcpTool.(interface{ String() string }); ok {
				toolName = stringer.String()
			} else {
				// Could not extract tool name - skip this tool
				continue
			}
			
			// Handle prefixed tool names from MCP servers (e.g., "filesystem-server_create_directory")
			baseName := toolName
			if strings.Contains(toolName, "_") {
				parts := strings.SplitN(toolName, "_", 2)
				if len(parts) == 2 {
					baseName = parts[1] // Extract "create_directory" from "filesystem-server_create_directory"
				}
			}
			
			if baseName == assignedTool.ToolName {
				// Convert ai.Tool to ai.ToolRef
				tools = append(tools, ai.ToolRef(mcpTool))
				log.Printf("🔧 Including assigned tool: %s", assignedTool.ToolName)
				break
			}
		}
	}

	log.Printf("📋 Filtered to %d assigned tools for agent execution", len(tools))

	// Tools are already ai.ToolRef, use them directly
	toolRefs := tools

	// Create execution prompt that incorporates the agent's system prompt and task
	executionPrompt := fmt.Sprintf(`You are %s, an AI agent with the following configuration:

System Prompt: %s

Your task is to: %s

You have access to MCP tools through the Station platform. Use these tools as needed to complete the task effectively.

Please execute this task step by step, using available tools when necessary. Provide a detailed response about what you accomplished.

Available Tools: You have access to %d MCP tools including file operations, directory management, search capabilities, and system information tools.

Execute the task now:`,
		agent.Name,
		agent.Prompt,
		task,
		len(tools))

	// Get model name based on provider configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config for model selection: %w", err)
	}

	var modelName string
	switch strings.ToLower(cfg.AIProvider) {
	case "openai":
		// Use configured model or default
		if cfg.AIModel != "" {
			modelName = fmt.Sprintf("openai/%s", cfg.AIModel)
		} else {
			modelName = "openai/gpt-4o"
		}
	case "gemini":
		// Use configured model or default
		if cfg.AIModel != "" {
			modelName = fmt.Sprintf("googleai/%s", cfg.AIModel)
		} else {
			modelName = "googleai/gemini-pro"
		}
	case "ollama":
		// Use configured model or default
		if cfg.AIModel != "" {
			modelName = fmt.Sprintf("ollama/%s", cfg.AIModel)
		} else {
			modelName = "ollama/llama3"
		}
	default:
		modelName = "openai/gpt-4o" // Default fallback
	}

	log.Printf("🔍 Executing agent with model: %s", modelName)

	// Use Genkit to execute the agent with assigned tools only
	var generateOptions []ai.GenerateOption
	generateOptions = append(generateOptions, ai.WithModelName(modelName))
	generateOptions = append(generateOptions, ai.WithPrompt(executionPrompt))
	generateOptions = append(generateOptions, ai.WithMaxTurns(25)) // Increase from default 5 to handle complex tasks
	
	// Only add tools and tool choice if we have tools available
	if len(toolRefs) > 0 {
		generateOptions = append(generateOptions, ai.WithTools(toolRefs...))
		generateOptions = append(generateOptions, ai.WithToolChoice(ai.ToolChoiceAuto))
		log.Printf("🔧 Executing with %d assigned tools", len(toolRefs))
	} else {
		log.Printf("⚠️ No tools available - executing in reasoning-only mode")
	}
	
	response, err := genkit.Generate(ctx, iac.genkitApp, generateOptions...)
	if err != nil {
		// Track execution failure
		executionTime := time.Since(startTime).Milliseconds()
		if cfg, cfgErr := config.Load(); cfgErr == nil && cfg.TelemetryEnabled {
			telemetryService := telemetry.NewTelemetryService(true)
			defer telemetryService.Close()
			
			telemetryService.TrackAgentExecuted(
				agent.ID,
				executionTime,
				false, // failure
				0,     // no steps completed
			)
			
			telemetryService.TrackError("agent_execution_failed", err.Error(), map[string]interface{}{
				"agent_id":         agent.ID,
				"agent_name":      agent.Name,
				"run_id":          runID,
				"execution_time_ms": executionTime,
				"execution_mode":  "stdio_mcp",
				"model_name":      modelName,
			})
		}
		return nil, fmt.Errorf("failed to execute agent via stdio MCP: %w", err)
	}

	// Extract execution results
	responseText := response.Text()
	log.Printf("🤖 Agent execution completed via stdio MCP")

	// Parse actual execution steps and tool calls from Genkit response
	stepsTaken := int64(1) // Default to 1 step for basic reasoning
	var toolCalls *models.JSONArray
	var executionSteps *models.JSONArray

	// Count tool usage patterns in the response to estimate execution steps
	// This is a text-based approach since Genkit's response structure doesn't 
	// expose detailed execution metadata directly
	toolCallCount := 0
	responseLower := strings.ToLower(responseText)
	
	// Look for tool execution patterns in the response
	toolCallPatterns := []string{
		"executing", "execute", "using tool", "tool:",
		"directory_tree", "list_allowed_directories", "create_directory",
		"edit_file", "get_file_info", "search_files", "read_text_file",
	}
	
	detectedTools := []string{}
	for _, pattern := range toolCallPatterns {
		if count := strings.Count(responseLower, pattern); count > 0 {
			toolCallCount += count
			detectedTools = append(detectedTools, pattern)
		}
	}
	
	// Count step-by-step patterns that indicate multi-step thinking
	stepPatterns := []string{
		"step 1", "step 2", "step 3", "step 4", "step 5",
		"action 1", "action 2", "action 3", "action 4", "action 5",
		"first", "second", "third", "fourth", "fifth", "next",
	}
	
	detectedSteps := []string{}
	stepIndicators := 0
	for _, pattern := range stepPatterns {
		if strings.Contains(responseLower, pattern) {
			stepIndicators++
			detectedSteps = append(detectedSteps, pattern)
		}
	}
	
	// Calculate steps based on tool calls and reasoning patterns
	if toolCallCount > 0 || stepIndicators > 2 {
		// If we found tool calls or multiple step indicators, estimate steps
		estimatedSteps := max(toolCallCount+1, stepIndicators)
		stepsTaken = int64(min(estimatedSteps, 25)) // Cap at max steps
	}
	
	// Create structured tool calls data for display
	if len(detectedTools) > 0 {
		toolCallsData := make([]interface{}, 0)
		for i, tool := range detectedTools {
			toolCallsData = append(toolCallsData, map[string]interface{}{
				"call_id":    i + 1,
				"tool_name":  tool,
				"detected":   true,
				"timestamp":  time.Now().Format(time.RFC3339),
				"context":    "pattern_detection",
			})
		}
		toolCallsArray := models.JSONArray(toolCallsData)
		toolCalls = &toolCallsArray
	}
	
	// Create structured execution steps data for display
	executionStepsData := make([]interface{}, 0)
	
	// Add initial reasoning step
	executionStepsData = append(executionStepsData, map[string]interface{}{
		"step":        1,
		"type":        "reasoning",
		"description": "Initial task analysis and planning",
		"timestamp":   time.Now().Format(time.RFC3339),
		"status":      "completed",
	})
	
	// Add detected steps
	for i, stepPattern := range detectedSteps {
		executionStepsData = append(executionStepsData, map[string]interface{}{
			"step":        i + 2,
			"type":        "execution",
			"description": fmt.Sprintf("Executed step: %s", stepPattern),
			"timestamp":   time.Now().Format(time.RFC3339),
			"status":      "completed",
		})
	}
	
	// Add detected tool usage steps
	for i, tool := range detectedTools {
		executionStepsData = append(executionStepsData, map[string]interface{}{
			"step":        len(detectedSteps) + i + 2,
			"type":        "tool_usage",
			"description": fmt.Sprintf("Used tool: %s", tool),
			"tool":        tool,
			"timestamp":   time.Now().Format(time.RFC3339),
			"status":      "completed",
		})
	}
	
	if len(executionStepsData) > 0 {
		executionStepsArray := models.JSONArray(executionStepsData)
		executionSteps = &executionStepsArray
	}
	
	log.Printf("🔍 Multi-step execution analysis: %d tool patterns, %d step indicators → %d total steps", 
		toolCallCount, stepIndicators, stepsTaken)
	log.Printf("🔧 Detected tools: %v", detectedTools)
	log.Printf("📋 Detected steps: %v", detectedSteps)

	result := &AgentExecutionResult{
		Response:       responseText,
		StepsTaken:     stepsTaken,
		ToolCalls:      toolCalls,
		ExecutionSteps: executionSteps,
	}

	log.Printf("✅ Stdio MCP agent execution completed: %d steps taken", stepsTaken)

	// Track agent execution with PostHog telemetry
	executionTime := time.Since(startTime).Milliseconds()
	
	// Load telemetry service and track if enabled
	if cfg, cfgErr := config.Load(); cfgErr == nil && cfg.TelemetryEnabled {
		telemetryService := telemetry.NewTelemetryService(true)
		defer telemetryService.Close()
		
		telemetryService.TrackAgentExecuted(
			agent.ID,
			executionTime,
			true, // success
			int(stepsTaken),
		)
		
		// Also track detailed execution metadata
		telemetryService.TrackEvent("agent_execution_detailed", map[string]interface{}{
			"agent_id":         agent.ID,
			"agent_name":      agent.Name,
			"run_id":          runID,
			"execution_time_ms": executionTime,
			"steps_taken":     stepsTaken,
			"tools_detected":  len(detectedTools),
			"step_indicators": len(detectedSteps),
			"task_length":     len(task),
			"response_length": len(responseText),
			"execution_mode":  "stdio_mcp",
		})
	}

	log.Printf("✅ Environment MCP agent execution completed successfully")
	return result, nil
}

// getEnvironmentMCPTools connects to MCP servers in the agent's environment and gets available tools
func (iac *IntelligentAgentCreator) getEnvironmentMCPTools(ctx context.Context, environmentID int64) ([]ai.Tool, error) {
	// Get file-based MCP configurations for this environment
	environment, err := iac.repos.Environments.GetByID(environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment %d: %w", environmentID, err)
	}

	log.Printf("🌍 Getting MCP tools for environment: %s (ID: %d)", environment.Name, environmentID)

	// Check if staging environment has filesystem MCP server
	if environment.Name == "staging" {
		// Create MCP client for filesystem server
		fsClient, err := mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
			Name:    "filesystem-server",
			Version: "1.0.0",
			Stdio: &mcp.StdioConfig{
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/home/epuerta/projects/hack/station"},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create filesystem MCP client: %w", err)
		}

		// Get tools from filesystem server
		tools, err := fsClient.GetActiveTools(ctx, iac.genkitApp)
		if err != nil {
			return nil, fmt.Errorf("failed to get tools from filesystem server: %w", err)
		}

		log.Printf("🗂️ Found %d tools from filesystem MCP server", len(tools))
		return tools, nil
	}

	// Fallback to Station stdio server for other environments
	log.Printf("⚠️ Environment '%s' not configured for file-based MCP, using Station stdio fallback", environment.Name)
	return iac.mcpClient.GetActiveTools(ctx, iac.genkitApp)
}
