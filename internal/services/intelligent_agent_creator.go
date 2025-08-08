package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	compat_oai "github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/firebase/genkit/go/plugins/mcp"
	
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"github.com/firebase/genkit/go/plugins/ollama"
	"github.com/openai/openai-go/option"
)

// IntelligentAgentCreator uses a Genkit agent with Station's own MCP server to intelligently create agents
type IntelligentAgentCreator struct {
	repos        *repositories.Repositories
	agentService AgentServiceInterface
	// mcpConfigSvc removed - using file-based configs only
	genkitApp       *genkit.Genkit
	mcpClient       *mcp.GenkitMCPClient
	currentProvider string // Track current AI provider to detect changes
	currentAPIKey   string // Track current API key to detect changes
	currentBaseURL  string // Track current base URL to detect changes
}

// AgentCreationRequest represents a request for intelligent agent creation
type AgentCreationRequest struct {
	Name                string `json:"name"`
	Description         string `json:"description"`
	UserIntent          string `json:"user_intent"`
	Domain              string `json:"domain,omitempty"`
	Schedule            string `json:"schedule,omitempty"`
	TargetEnvironmentID int64  `json:"target_environment_id,omitempty"`
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

// initializeGenkit initializes Genkit with the configured AI provider and telemetry
func (iac *IntelligentAgentCreator) initializeGenkit(ctx context.Context) error {
	// Load Station configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if configuration has changed
	configChanged := iac.currentProvider != strings.ToLower(cfg.AIProvider) ||
		iac.currentAPIKey != cfg.AIAPIKey ||
		iac.currentBaseURL != cfg.AIBaseURL

	// If already initialized with same config, return early
	if iac.genkitApp != nil && !configChanged {
		return nil
	}

	// Configuration changed or first initialization - reinitialize GenKit
	if configChanged && iac.genkitApp != nil {
		logging.Info("AI provider configuration changed from %s to %s, reinitializing GenKit...", 
			iac.currentProvider, strings.ToLower(cfg.AIProvider))
		// Note: GenKit doesn't provide a clean shutdown method, so we'll just replace the instance
		iac.genkitApp = nil
	}

	// Update tracked configuration
	iac.currentProvider = strings.ToLower(cfg.AIProvider)
	iac.currentAPIKey = cfg.AIAPIKey
	iac.currentBaseURL = cfg.AIBaseURL

	// Initialize Genkit with the configured AI provider
	logging.Info("Initializing GenKit with provider: %s, model: %s", cfg.AIProvider, cfg.AIModel)
	
	var genkitApp *genkit.Genkit
	switch strings.ToLower(cfg.AIProvider) {
	case "openai":
		// Validate API key for OpenAI
		if cfg.AIAPIKey == "" {
			return fmt.Errorf("API key is required for OpenAI provider (set STN_AI_API_KEY or OPENAI_API_KEY)")
		}
		logging.Debug("Setting up OpenAI plugin with model: %s", cfg.AIModel)
		
		openaiPlugin := &compat_oai.OpenAI{
			APIKey: cfg.AIAPIKey,
		}
		// Set custom base URL if provided (for OpenAI-compatible providers)
		if cfg.AIBaseURL != "" {
			logging.Debug("Using custom OpenAI-compatible endpoint: %s", cfg.AIBaseURL)
			openaiPlugin.Opts = []option.RequestOption{
				option.WithBaseURL(cfg.AIBaseURL),
			}
		}
		genkitApp, err = genkit.Init(ctx, genkit.WithPlugins(openaiPlugin))
		if err != nil {
			return fmt.Errorf("failed to initialize Genkit with OpenAI: %w", err)
		}
		logging.Debug("OpenAI plugin initialized successfully")
	case "gemini":
		// Validate API key for Gemini
		if cfg.AIAPIKey == "" {
			return fmt.Errorf("API key is required for Gemini provider (set STN_AI_API_KEY or GOOGLE_API_KEY)")
		}
		logging.Debug("Setting up Gemini plugin with model: %s", cfg.AIModel)
		
		geminiPlugin := &googlegenai.GoogleAI{
			APIKey: cfg.AIAPIKey,
		}
		genkitApp, err = genkit.Init(ctx, genkit.WithPlugins(geminiPlugin))
		if err != nil {
			return fmt.Errorf("failed to initialize Genkit with Gemini: %w", err)
		}
		logging.Debug("Gemini plugin initialized successfully")
		
	case "ollama":
		ollamaBaseURL := cfg.AIBaseURL
		if ollamaBaseURL == "" {
			ollamaBaseURL = "http://127.0.0.1:11434" // Default Ollama server
		}
		logging.Debug("Setting up Ollama plugin with server: %s, model: %s", ollamaBaseURL, cfg.AIModel)
		
		ollamaPlugin := &ollama.Ollama{
			ServerAddress: ollamaBaseURL,
		}
		genkitApp, err = genkit.Init(ctx, genkit.WithPlugins(ollamaPlugin))
		if err != nil {
			return fmt.Errorf("failed to initialize Genkit with Ollama: %w", err)
		}
		logging.Debug("Ollama plugin initialized successfully")
		
	default:
		return fmt.Errorf("unsupported AI provider: %s\n\nSupported providers:\n"+
			"  • openai: OpenAI GPT models (gpt-4o, gpt-4, gpt-3.5-turbo)\n"+
			"  • gemini: Google Gemini models (gemini-2.0-flash-exp, gemini-pro)\n"+
			"  • ollama: Local Ollama models (llama3, mistral, etc)\n\n"+
			"For OpenAI-compatible providers, use 'openai' with custom STN_AI_BASE_URL", 
			cfg.AIProvider)
	}
	iac.genkitApp = genkitApp

	// Set up OpenTelemetry export to Jaeger using Genkit's RegisterSpanProcessor
	if err := iac.setupOpenTelemetryExport(ctx, genkitApp); err != nil {
		logging.Debug("Failed to set up OpenTelemetry export: %v", err)
		// Don't fail initialization, just log the warning
	} else {
		logging.Debug("OpenTelemetry telemetry configured successfully")
	}

	// Create MCP client to connect to our own stdio server
	mcpClient, err := mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
		Name:    "s", // Short name to minimize tool call IDs in OpenAI
		Version: "1.0.0",
		Stdio: &mcp.StdioConfig{
			Command: "stn", // Use globally installed binary
			Args:    []string{"stdio"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create MCP client: %w", err)
	}
	iac.mcpClient = mcpClient

	logging.Debug("Initialized Genkit with Station MCP client")
	return nil
}

// CreateIntelligentAgent uses Genkit with our own MCP server to analyze requirements and create an optimized agent
func (iac *IntelligentAgentCreator) CreateIntelligentAgent(ctx context.Context, req AgentCreationRequest) (*models.Agent, error) {
	logging.Info("Starting intelligent agent creation for: %s", req.Name)

	// Step 1: Initialize Genkit with our MCP server
	err := iac.initializeGenkit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Genkit: %w", err)
	}

	// Step 2: Determine target environment (use specified or validate recommended)
	targetEnvironmentID := req.TargetEnvironmentID
	if targetEnvironmentID == 0 {
		// No target specified, use default
		environments, err := iac.repos.Environments.List()
		if err != nil {
			return nil, fmt.Errorf("failed to list environments: %w", err)
		}
		if len(environments) == 0 {
			return nil, fmt.Errorf("no environments available")
		}
		targetEnvironmentID = environments[0].ID
	}

	// Step 3: Use Genkit agent to analyze requirements and generate creation plan for target environment
	plan, err := iac.generateAgentPlanWithGenkit(ctx, req, targetEnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate agent plan: %w", err)
	}

	logging.Info("Generated plan for agent '%s' with %d tools for environment ID %d", plan.AgentName, len(plan.CoreTools), targetEnvironmentID)

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
		targetEnvironmentID,
		1, // Default user ID for now
		cronSchedule,
		scheduleEnabled,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Step 5: Assign selected tools to the agent in database (for filtering during execution)
	assignedCount := 0
	if len(plan.CoreTools) > 0 {
		assignedCount = iac.assignToolsToAgent(agent.ID, plan.CoreTools, targetEnvironmentID)
		logging.Debug("Agent assigned %d tools from %d selected by AI", assignedCount, len(plan.CoreTools))
	}

	// Step 6: Handle scheduling if enabled
	if agent.IsScheduled && agent.CronSchedule != nil {
		logging.Info("Agent '%s' has schedule '%s' - will be handled by scheduler service", agent.Name, *agent.CronSchedule)
		// The scheduler service will pick up this agent automatically on next restart,
		// or we can implement a notification mechanism here if needed
	}

	// Step 7: Cleanup
	if iac.mcpClient != nil {
		iac.mcpClient.Disconnect()
	}

	logging.Info("Successfully created intelligent agent '%s' (ID: %d) with %d tools%s",
		agent.Name, agent.ID, assignedCount, 
		func() string {
			if agent.IsScheduled && agent.CronSchedule != nil {
				return fmt.Sprintf(" (scheduled: %s)", *agent.CronSchedule)
			}
			return ""
		}())

	return agent, nil
}

// generateAgentPlanWithGenkit uses Genkit with environment-specific MCP tools to intelligently analyze requirements
func (iac *IntelligentAgentCreator) generateAgentPlanWithGenkit(ctx context.Context, req AgentCreationRequest, targetEnvironmentID int64) (*AgentCreationPlan, error) {
	logging.Info("Using Genkit to analyze agent requirements for environment ID %d...", targetEnvironmentID)

	// Get available tools from the target environment
	tools, err := iac.getEnvironmentMCPTools(ctx, targetEnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment MCP tools: %w", err)
	}

	logging.Debug("Found %d available MCP tools in target environment for analysis", len(tools))

	// Convert tools to tool references and extract tool names
	var toolRefs []ai.ToolRef
	var availableToolNames []string
	for _, tool := range tools {
		toolRefs = append(toolRefs, tool)
		
		// Extract tool name for prompt
		var toolName string
		if named, ok := tool.(interface{ Name() string }); ok {
			toolName = named.Name()
		} else if stringer, ok := tool.(interface{ String() string }); ok {
			toolName = stringer.String()
		}
		
		if toolName != "" {
			availableToolNames = append(availableToolNames, toolName)
		}
	}

	// Get environment name for context
	environment, err := iac.repos.Environments.GetByID(targetEnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	// Create prompt for intelligent agent analysis with actual available tools
	prompt := fmt.Sprintf(`IMPORTANT: You must respond with ONLY a valid JSON object. No explanations, no conversational text, no markdown formatting. Only JSON.

Analyze these agent requirements and create an optimal configuration:

Agent Requirements:
- Name: %s
- Description: %s
- User Intent: %s
- Domain: %s
- Schedule: %s
- Target Environment: %s

Available Tools in Environment '%s': %d MCP tools
%s

Based on the requirements, intelligently select 2-5 most relevant tools from the available tools listed above.

RESPOND WITH ONLY THIS JSON STRUCTURE (populate with appropriate values based on the requirements):
{
  "agent_name": "intelligent_agent_name",
  "agent_description": "detailed_description_of_agent_capabilities", 
  "system_prompt": "system_prompt_for_the_agent",
  "recommended_environment": "%s",
  "core_tools": ["tool1", "tool2", "tool3"],
  "max_steps": 10,
  "schedule": "on-demand",
  "rationale": "explanation_of_selections",
  "success_criteria": "how_to_measure_success"
}`,
		req.Name, req.Description, req.UserIntent, req.Domain, req.Schedule, environment.Name, 
		environment.Name, len(tools), 
		func() string {
			if len(availableToolNames) > 0 {
				result := "Available Tools:\n"
				for _, toolName := range availableToolNames {
					result += fmt.Sprintf("- %s\n", toolName)
				}
				return result
			}
			return "No tools available in this environment."
		}(),
		environment.Name)

	// Get model name based on provider (reload config to get latest values)
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config for model selection: %w", err)
	}

	var modelName string
	// Using provider configuration
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

	// Use Genkit to generate the agent plan with multi-turn support
	response, err := genkit.Generate(ctx, iac.genkitApp,
		ai.WithModelName(modelName),
		ai.WithPrompt(prompt),
		ai.WithTools(toolRefs...),
		ai.WithToolChoice(ai.ToolChoiceAuto),
		ai.WithMaxTurns(10), // Allow multi-step agent planning
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate agent plan: %w", err)
	}

	// Parse the JSON response with enhanced debugging
	var plan AgentCreationPlan
	responseText := response.Text()
	
	// DEBUG: Log comprehensive response details for agent creation
	logging.Debug("Agent creation response: %s", responseText)
	if response.Usage != nil {
		logging.Debug("Agent creation usage - Input tokens: %d, Output tokens: %d, Total tokens: %d", 
			response.Usage.InputTokens, response.Usage.OutputTokens, 
			response.Usage.InputTokens + response.Usage.OutputTokens)
	}
	
	// Count turns taken during agent creation
	if response.Request != nil && len(response.Request.Messages) > 0 {
		modelMessages := 0
		for _, msg := range response.Request.Messages {
			if msg.Role == ai.RoleModel {
				modelMessages++
			}
		}
		logging.Debug("Agent creation took %d turns, %d total messages in conversation", 
			modelMessages, len(response.Request.Messages))
	}

	// Try to extract JSON from the response (might be wrapped in markdown)
	jsonStart := strings.Index(responseText, "{")
	jsonEnd := strings.LastIndex(responseText, "}") + 1
	if jsonStart == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no valid JSON found in response: %s", responseText)
	}

	jsonStr := responseText[jsonStart:jsonEnd]
	logging.Debug("Extracted JSON: %s", jsonStr)
	
	err = json.Unmarshal([]byte(jsonStr), &plan)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent plan JSON: %w. JSON was: %s", err, jsonStr)
	}

	logging.Info("Generated intelligent plan for '%s': %d tools, %d max steps",
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
		logging.Info("Environment '%s' not found, using '%s' instead", envName, environments[0].Name)
		return environments[0].ID, nil
	}

	return 0, fmt.Errorf("no environments available")
}

// assignToolsToAgent assigns the specified tools to the agent using the repository API
func (iac *IntelligentAgentCreator) assignToolsToAgent(agentID int64, toolNames []string, environmentID int64) int {
	assignedCount := 0
	for _, toolName := range toolNames {
		// AI analysis returns prefixed names but database stores clean names
		// Strip prefix to match database storage: "f_read_text_file" -> "read_text_file"
		dbToolName := toolName
		if strings.Contains(toolName, "_") {
			parts := strings.SplitN(toolName, "_", 2)
			if len(parts) == 2 {
				dbToolName = parts[1] // Remove prefix for database lookup
				logging.Debug("Mapping AI tool name for database: %s -> %s", toolName, dbToolName)
			}
		}
		
		tool, err := iac.repos.MCPTools.FindByNameInEnvironment(environmentID, dbToolName)
		if err != nil {
			logging.Debug("Warning: Failed to find tool '%s' in environment %d: %v", toolName, environmentID, err)
			continue
		}
		
		// Add the tool assignment to the agent
		_, err = iac.repos.AgentTools.AddAgentTool(agentID, tool.ID)
		if err != nil {
			logging.Debug("Warning: Failed to assign tool '%s' (ID: %d) to agent %d: %v", tool.Name, tool.ID, agentID, err)
			continue
		}
		
		assignedCount++
		logging.Debug("Successfully assigned tool '%s' (ID: %d) to agent %d", tool.Name, tool.ID, agentID)
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

	logging.Debug("Stdio MCP connection successful - found %d tools", len(tools))
	return nil
}

// AgentExecutionResult represents the result of agent execution via stdio MCP
type AgentExecutionResult struct {
	Response       string            `json:"response"`
	StepsTaken     int64             `json:"steps_taken"`
	ToolCalls      *models.JSONArray `json:"tool_calls,omitempty"`
	ExecutionSteps *models.JSONArray `json:"execution_steps,omitempty"`
	TokenUsage     *TokenUsage       `json:"token_usage,omitempty"`
}

// TokenUsage represents token usage statistics for an execution
type TokenUsage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	LatencyMs    float64 `json:"latency_ms,omitempty"`
}

// generateShortToolID creates a short unique ID for OpenAI tool calls (max 40 chars)
func generateShortToolID(toolName string) string {
	// Create a hash of the tool name
	hash := sha256.Sum256([]byte(toolName))
	// Take first 8 bytes (16 hex chars) and prefix with "tool_"
	shortID := "tool_" + hex.EncodeToString(hash[:8])
	// Result: "tool_" + 16 chars = 21 chars total (well under 40 char limit)
	return shortID
}

// cleanNameTool wraps an MCP tool to provide a clean name and short ID for OpenAI compatibility
type cleanNameTool struct {
	originalTool ai.Tool
	cleanName    string
	shortID      string
}

func (ct *cleanNameTool) Name() string {
	return ct.cleanName
}

// GetShortID returns the short hash-based ID for OpenAI tool call compatibility
func (ct *cleanNameTool) GetShortID() string {
	return ct.shortID
}

// Delegate all other methods to the original tool
func (ct *cleanNameTool) Definition() *ai.ToolDefinition {
	def := ct.originalTool.Definition()
	// Update the definition to use the clean name (ultra-short for OpenAI compatibility)
	if def != nil {
		cleanDef := *def
		cleanDef.Name = ct.cleanName
		// Keep original description so user knows what the tool does
		return &cleanDef
	}
	return def
}

func (ct *cleanNameTool) RunRaw(ctx context.Context, input any) (any, error) {
	return ct.originalTool.RunRaw(ctx, input)
}

func (ct *cleanNameTool) Register(r any) {
	// Delegate to original tool if it supports Register
	if registrable, ok := ct.originalTool.(interface{ Register(any) }); ok {
		registrable.Register(r)
	}
}

func (ct *cleanNameTool) Respond(toolReq *ai.Part, outputData any, opts *ai.RespondOptions) *ai.Part {
	return ct.originalTool.Respond(toolReq, outputData, opts)
}

// ExecuteAgentViaStdioMCP executes an agent using self-bootstrapping stdio MCP architecture
func (iac *IntelligentAgentCreator) ExecuteAgentViaStdioMCP(ctx context.Context, agent *models.Agent, task string, runID int64) (*AgentExecutionResult, error) {
	startTime := time.Now()
	logging.Info("Starting stdio MCP agent execution for agent '%s'", agent.Name)

	// Initialize Genkit + MCP if not already done
	err := iac.initializeGenkit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Genkit for agent execution: %w", err)
	}

	// Let Genkit handle tracing automatically - no need for custom span wrapping
	// Get tools assigned to this specific agent
	assignedTools, err := iac.repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assigned tools for agent %d: %w", agent.ID, err)
	}

	logging.Debug("Agent has %d assigned tools for execution", len(assignedTools))

	// Get MCP tools from agent's environment instead of Station stdio
	allTools, err := iac.getEnvironmentMCPTools(ctx, agent.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment MCP tools for agent %d: %w", agent.ID, err)
	}

	// Filter to only include tools assigned to this agent and ensure clean tool names
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
			
			// Match prefixed runtime tool names with clean database names
			// Runtime tools have prefixes like "f_list_directory", database has "list_directory"
			var matchFound bool
			if strings.Contains(toolName, "_") {
				parts := strings.SplitN(toolName, "_", 2)
				if len(parts) == 2 {
					cleanToolName := parts[1] // Remove prefix: "f_list_directory" -> "list_directory"  
					if cleanToolName == assignedTool.ToolName {
						matchFound = true
					}
				}
			}
			// Also try direct match in case of different naming schemes
			if toolName == assignedTool.ToolName {
				matchFound = true
			}
			
			if matchFound {
				logging.Debug("Tool match found: MCP '%s' matches assigned '%s'", toolName, assignedTool.ToolName)
				
				// DEBUG: Log what Station passes to Genkit
				toolRef := ai.ToolRef(mcpTool)
				// Tool reference created
				
				tools = append(tools, toolRef)
				// Including assigned tool
				break
			}
		}
	}

	logging.Debug("Filtered to %d assigned tools for agent execution", len(tools))

	// Tools are already ai.ToolRef, use them directly
	toolRefs := tools

	// Create execution prompt that incorporates the agent's system prompt and task
	executionPrompt := fmt.Sprintf(`You are %s, an AI agent with the following configuration:

System Prompt: %s

Your task is to: %s

You have access to MCP tools through the Station platform. Use these tools as needed to complete the task effectively.

IMPORTANT: Break down complex tasks into multiple steps. After each tool call, analyze the results and determine if you need to use additional tools. Do NOT try to complete everything in one response. Take multiple turns as needed.

Multi-Step Execution Guidelines:
1. Start with one tool call to gather initial information
2. Analyze the results from that tool call  
3. Determine what additional information or actions are needed
4. Make subsequent tool calls based on your analysis
5. Continue this process until the task is fully complete
6. Provide a comprehensive final summary

Available Tools: You have access to %d MCP tools including file operations, directory management, search capabilities, and system information tools.

Begin by making your first tool call:`,
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
	// Using provider configuration
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

	// Executing agent

	// Note: Genkit automatically captures all telemetry data:
	// - Token usage (input/output tokens)  
	// - Tool calls (requests/responses with parameters)
	// - Prompts and responses (complete I/O logging)
	// - Performance metrics (latency, request counts)
	// - OpenTelemetry traces automatically exported to Jaeger

	// Provider-aware tool call tracking
	var capturedToolCalls []map[string]interface{}
	var toolTrackingMiddleware func(ai.ModelFunc) ai.ModelFunc
	
	// Define providers that need middleware for tool call extraction
	// These providers use OpenAI-compatible format where GenKit's multi-turn orchestration hides tool calls
	openAICompatibleProviders := []string{
		"openai",
		"anthropic", 
		"groq",
		"deepseek",
		"together",
		"fireworks",
		"perplexity",
		"mistral",
		"cohere",
		"huggingface",
		"replicate",
		"anyscale",
		"local", // Local OpenAI-compatible servers
	}
	
	// Check if current provider needs middleware
	needsMiddleware := false
	for _, provider := range openAICompatibleProviders {
		if strings.HasPrefix(cfg.AIProvider, provider) {
			needsMiddleware = true
			break
		}
	}
	if needsMiddleware {
		toolTrackingMiddleware = func(next ai.ModelFunc) ai.ModelFunc {
			return func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
				resp, err := next(ctx, req, cb)
				if err != nil {
					return resp, err
				}
				
				// Capture OpenAI tool calls from response
				if resp.Message != nil {
					for _, part := range resp.Message.Content {
						if part.IsToolRequest() {
							toolReq := part.ToolRequest
							capturedToolCalls = append(capturedToolCalls, map[string]interface{}{
								"tool_name": toolReq.Name,
								"input":     toolReq.Input,
								"step":      len(capturedToolCalls) + 1,
							})
						}
					}
				}
				
				return resp, err
			}
		}
	}
	
	// Use Genkit to execute the agent with assigned tools only
	var generateOptions []ai.GenerateOption
	generateOptions = append(generateOptions, ai.WithModelName(modelName))
	generateOptions = append(generateOptions, ai.WithPrompt(executionPrompt))
	generateOptions = append(generateOptions, ai.WithMaxTurns(25)) // Support multi-step workflows
	
	// Apply middleware only for OpenAI-compatible providers  
	if needsMiddleware && toolTrackingMiddleware != nil {
		generateOptions = append(generateOptions, ai.WithMiddleware(toolTrackingMiddleware))
	}
	
	// Only add tools and tool choice if we have tools available
	if len(toolRefs) > 0 {
		generateOptions = append(generateOptions, ai.WithTools(toolRefs...))
		generateOptions = append(generateOptions, ai.WithToolChoice(ai.ToolChoiceAuto))
		// Tools available for execution
	} else {
		logging.Info("No tools available - executing in reasoning-only mode")
	}
	
	// EXPERIMENT: Add streaming callback to capture intermediate tool calls
	var capturedChunks []*ai.ModelResponseChunk
	_ = func(ctx context.Context, chunk *ai.ModelResponseChunk) error {
		capturedChunks = append(capturedChunks, chunk)
		logging.Debug("STREAM CHUNK: Role=%s, Content=%+v", chunk.Role, chunk.Content)
		return nil
	}
	
	response, err := genkit.Generate(ctx, iac.genkitApp, generateOptions...)
	// TODO: Also try with streaming callback
	// response, err := ai.GenerateWithRequest(ctx, iac.genkitApp.Registry(), &ai.GenerateActionOptions{...}, nil, streamCallback)
	if err != nil {
		// Track execution failure
		executionTime := time.Since(startTime).Milliseconds()
		logging.Debug("Agent execution failed after %dms", executionTime)
		return nil, fmt.Errorf("failed to execute agent via stdio MCP: %w", err)
	}

	// Extract execution results
	responseText := response.Text()
	logging.Debug("Agent execution completed via stdio MCP")
	
	// Log execution metrics
	if response.Usage != nil {
		logging.Debug("Token usage - Input: %d, Output: %d, Total: %d, Latency: %.2fms", 
			response.Usage.InputTokens, response.Usage.OutputTokens, response.Usage.TotalTokens, response.LatencyMs)
	}
	
	// Provider-specific tool call logging
	if needsMiddleware && len(capturedToolCalls) > 0 {
		logging.Debug("Middleware captured %d tool calls from %s", len(capturedToolCalls), cfg.AIProvider)
	}
	
	// Streaming data captured for analysis

	// Count actual steps/turns from Genkit response
	stepsTaken := int64(1) // Default to 1 step for basic reasoning
	if response.Request != nil && len(response.Request.Messages) > 0 {
		// Count the number of model messages (excluding the initial user message)
		modelMessages := 0
		toolMessages := 0
		userMessages := 0
		systemMessages := 0
		for _, msg := range response.Request.Messages {
			switch msg.Role {
			case ai.RoleModel:
				modelMessages++
			case ai.RoleTool:
				toolMessages++
			case ai.RoleUser:
				userMessages++
			case ai.RoleSystem:
				systemMessages++
			}
		}
		if modelMessages > 0 {
			stepsTaken = int64(modelMessages)
		}
	}
	var toolCalls *models.JSONArray
	var executionSteps *models.JSONArray

	// Always extract detailed execution data from GenKit response
	logging.Debug("Extracting detailed execution data from GenKit response...")
	
	// Unified tool call extraction: OpenAI-compatible from middleware, Gemini from native response
	if needsMiddleware && len(capturedToolCalls) > 0 {
		// Add tool outputs to captured OpenAI tool calls from conversation messages
		iac.addToolOutputsToCapturedCalls(capturedToolCalls, response)
		var toolCallsInterface []interface{}
		for _, toolCall := range capturedToolCalls {
			toolCallsInterface = append(toolCallsInterface, toolCall)
		}
		toolCallsArray := models.JSONArray(toolCallsInterface)
		toolCalls = &toolCallsArray
	} else if !needsMiddleware {
		// Use native GenKit tool call extraction for Gemini and other providers (works beautifully)
		toolCallsFromResponse := iac.extractToolCallsFromResponse(response, modelName)
		if len(toolCallsFromResponse) > 0 {
			toolCallsArray := models.JSONArray(toolCallsFromResponse)
			toolCalls = &toolCallsArray
		}
	}
	
	// Build detailed execution steps from conversation flow
	detailedSteps := iac.buildExecutionStepsFromResponse(response, agent, modelName, len(toolRefs))
	if len(detailedSteps) > 0 {
		executionStepsArray := models.JSONArray(detailedSteps)
		executionSteps = &executionStepsArray
		logging.Debug("Built %d execution steps from conversation", len(detailedSteps))
	} else {
		logging.Debug("No detailed execution steps generated")
	}
	
	logging.Debug("Detailed execution data extracted successfully")

	// Extract token usage from GenKit response
	var tokenUsage *TokenUsage
	if response.Usage != nil {
		tokenUsage = &TokenUsage{
			InputTokens:  response.Usage.InputTokens,
			OutputTokens: response.Usage.OutputTokens,
			TotalTokens:  response.Usage.InputTokens + response.Usage.OutputTokens,
			LatencyMs:    response.LatencyMs,
		}
		logging.Debug("Token usage - Input: %d, Output: %d, Total: %d, Latency: %.2fms", 
			tokenUsage.InputTokens, tokenUsage.OutputTokens, tokenUsage.TotalTokens, tokenUsage.LatencyMs)
	}

	result := &AgentExecutionResult{
		Response:       responseText,
		StepsTaken:     stepsTaken,
		ToolCalls:      toolCalls,
		ExecutionSteps: executionSteps,
		TokenUsage:     tokenUsage,
	}

	logging.Info("Agent execution completed: %d steps taken", stepsTaken)

	// Track agent execution with PostHog telemetry
	executionTime := time.Since(startTime).Milliseconds()
	
	logging.Debug("Agent execution completed in %dms with %d steps", executionTime, stepsTaken)

	logging.Info("Environment MCP agent execution completed successfully")
	return result, nil
}

// setupOpenTelemetryExport configures OpenTelemetry to export traces to Jaeger
func (iac *IntelligentAgentCreator) setupOpenTelemetryExport(ctx context.Context, g *genkit.Genkit) error {
	// Create resource with proper service information
	_, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("station-agent"),
			semconv.ServiceVersion("1.0.0"),
			semconv.ServiceInstanceID("station-local"),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP gRPC trace exporter for Jaeger
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint("localhost:4317"),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	// Create adjusting trace exporter (similar to Google Cloud plugin)
	adjustedExporter := &stationTraceExporter{traceExporter}

	// Create batch span processor with proper resource
	spanProcessor := trace.NewBatchSpanProcessor(adjustedExporter,
		trace.WithBatchTimeout(time.Second*1),      // Export every 1 second
		trace.WithMaxExportBatchSize(10),           // Small batches for testing
		trace.WithExportTimeout(time.Second*10),    // 10 second timeout
	)
	
	// Register the span processor with Genkit
	genkit.RegisterSpanProcessor(g, spanProcessor)
	
	logging.Debug("OpenTelemetry configured to export traces to Jaeger at localhost:4317 (service: station-agent)")
	logging.Debug("Rich telemetry data (tokens, usage, costs) will appear in span attributes")
	return nil
}

// stationTraceExporter wraps the OTLP exporter to add proper resource information
type stationTraceExporter struct {
	exporter trace.SpanExporter
}

func (e *stationTraceExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	// TODO: Add any span adjustments if needed (like resource information)
	return e.exporter.ExportSpans(ctx, spans)
}

func (e *stationTraceExporter) Shutdown(ctx context.Context) error {
	return e.exporter.Shutdown(ctx)
}

// getEnvironmentMCPTools connects to the actual MCP servers from file configs and gets their tools
func (iac *IntelligentAgentCreator) getEnvironmentMCPTools(ctx context.Context, environmentID int64) ([]ai.Tool, error) {
	// Get file-based MCP configurations for this environment
	environment, err := iac.repos.Environments.GetByID(environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment %d: %w", environmentID, err)
	}

	logging.Info("Getting MCP tools for environment: %s (ID: %d)", environment.Name, environmentID)

	// Get file configs for this environment
	logging.Debug("Querying database for file configs with environment ID: %d", environmentID)
	fileConfigs, err := iac.repos.FileMCPConfigs.ListByEnvironment(environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file configs for environment %d: %w", environmentID, err)
	}

	logging.Debug("Database query returned %d file configs for environment %d", len(fileConfigs), environmentID)
	for i, config := range fileConfigs {
		logging.Debug("Config %d: %s (ID: %d, Template: %s)", i+1, config.ConfigName, config.ID, config.TemplatePath)
	}

	var allTools []ai.Tool

	// Connect to each MCP server from file configs and get their tools
	for _, fileConfig := range fileConfigs {
		logging.Debug("Processing file config: %s (ID: %d), template path: %s", fileConfig.ConfigName, fileConfig.ID, fileConfig.TemplatePath)
		
		// Make template path absolute (relative to ~/.config/station/)
		configDir := os.ExpandEnv("$HOME/.config/station")
		absolutePath := fmt.Sprintf("%s/%s", configDir, fileConfig.TemplatePath)
		
		logging.Debug("Reading file config from: %s", absolutePath)
		
		// Read the actual file content from template path
		rawContent, err := os.ReadFile(absolutePath)
		if err != nil {
			logging.Debug("Failed to read file config %s from %s: %v", fileConfig.ConfigName, absolutePath, err)
			continue
		}

		logging.Debug("File config content loaded: %d bytes", len(rawContent))

		// Process template variables using TemplateVariableService
		templateService := NewTemplateVariableService(os.ExpandEnv("$HOME/.config/station"), iac.repos)
		result, err := templateService.ProcessTemplateWithVariables(fileConfig.EnvironmentID, fileConfig.ConfigName, string(rawContent), false)
		if err != nil {
			logging.Debug("Failed to process template variables for %s: %v", fileConfig.ConfigName, err)
			continue
		}

		// Use rendered content with variables resolved
		content := result.RenderedContent
		logging.Debug("Template rendered: %d bytes, variables resolved: %v", len(content), result.AllResolved)

		// Parse the file config content to get server configurations
		// The JSON files use "mcpServers" but the struct expects "servers" - handle both
		var rawConfig map[string]interface{}
		if err := json.Unmarshal([]byte(content), &rawConfig); err != nil {
			logging.Debug("Failed to parse file config %s: %v", fileConfig.ConfigName, err)
			continue
		}

		// Extract servers from either "mcpServers" or "servers" field
		var serversData map[string]interface{}
		if mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{}); ok {
			serversData = mcpServers
		} else if servers, ok := rawConfig["servers"].(map[string]interface{}); ok {
			serversData = servers
		} else {
			logging.Debug("No 'mcpServers' or 'servers' field found in config %s", fileConfig.ConfigName)
			continue
		}

		logging.Debug("Parsed config data with %d servers", len(serversData))

		// Process each server in the config
		for serverName, serverConfigRaw := range serversData {
			// Convert the server config to proper structure
			serverConfigBytes, err := json.Marshal(serverConfigRaw)
			if err != nil {
				logging.Debug("Failed to marshal server config for %s: %v", serverName, err)
				continue
			}
			
			var serverConfig models.MCPServerConfig
			if err := json.Unmarshal(serverConfigBytes, &serverConfig); err != nil {
				logging.Debug("Failed to unmarshal server config for %s: %v", serverName, err)
				continue
			}
			// Determine transport type based on config fields
			var mcpClient *mcp.GenkitMCPClient
			if serverConfig.URL != "" {
				// HTTP-based MCP server
				logging.Debug("Connecting to HTTP MCP server: %s (URL: %s)", serverName, serverConfig.URL)
				mcpClient, err = mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
					Name:    "f", // Short name to minimize tool call IDs in OpenAI
					Version: "1.0.0",
					StreamableHTTP: &mcp.StreamableHTTPConfig{
						BaseURL: serverConfig.URL,
						Timeout: 30 * time.Second, // Add timeout to prevent hanging
					},
				})
			} else if serverConfig.Command != "" {
				// Stdio-based MCP server
				logging.Debug("Connecting to Stdio MCP server: %s (command: %s, args: %v)", serverName, serverConfig.Command, serverConfig.Args)
				
				// Convert env map to slice for Stdio config
				var envSlice []string
				for key, value := range serverConfig.Env {
					envSlice = append(envSlice, key+"="+value)
				}
				
				mcpClient, err = mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
					Name:    "f", // Short name to minimize tool call IDs in OpenAI
					Version: "1.0.0",
					Stdio: &mcp.StdioConfig{
						Command: serverConfig.Command,
						Args:    serverConfig.Args,
						Env:     envSlice,
					},
				})
			} else {
				logging.Debug("Invalid MCP server config for %s: missing both URL and Command fields", serverName)
				continue
			}
			if err != nil {
				logging.Debug("Failed to create MCP client for %s: %v", serverName, err)
				continue
			}

			// Get tools from this MCP server with timeout and panic recovery
			logging.Debug("Attempting to get tools from MCP server: %s", serverName)
			
			// Create a timeout context for the tool discovery - increased timeout for stdio servers
			timeout := 30 * time.Second
			if serverConfig.Command != "" {
				// Stdio servers (especially uvx-based) need more time for cold start
				timeout = 90 * time.Second
			}
			toolCtx, cancel := context.WithTimeout(ctx, timeout)
			
			var serverTools []ai.Tool
			func() {
				// Recover from potential panics in the MCP client
				defer func() {
					if r := recover(); r != nil {
						logging.Debug("Panic recovered while getting tools from %s: %v", serverName, r)
						err = fmt.Errorf("panic in MCP client: %v", r)
					}
				}()
				
				serverTools, err = mcpClient.GetActiveTools(toolCtx, iac.genkitApp)
			}()
			
			// Cancel immediately after the call returns to prevent resource leaks
			cancel()
			
			// Always disconnect the client after discovery to prevent subprocess leaks
			if mcpClient != nil {
				mcpClient.Disconnect()
			}
			
			if err != nil {
				// Enhanced error logging for timeouts and other failures
				if err == context.DeadlineExceeded {
					envKeys := make([]string, 0, len(serverConfig.Env))
					for k := range serverConfig.Env {
						envKeys = append(envKeys, k)
					}
					logging.Debug("Timeout discovering tools for %s (cmd=%s args=%v envKeys=%v timeout=%v)", 
						serverName, serverConfig.Command, serverConfig.Args, envKeys, timeout)
				} else {
					logging.Debug("Failed to get tools from %s: %v", serverName, err)
				}
				
				if serverConfig.URL != "" {
					logging.Debug("HTTP MCP server details - Name: %s, URL: %s", serverName, serverConfig.URL)
				} else {
					logging.Debug("Stdio MCP server details - Name: %s, Command: %s, Args: %v, Env: %v", 
						serverName, serverConfig.Command, serverConfig.Args, serverConfig.Env)
				}
				continue
			}

			logging.Debug("Found %d tools from %s", len(serverTools), serverName)
			for i, tool := range serverTools {
				if named, ok := tool.(interface{ Name() string }); ok {
					logging.Debug("  Tool %d: %s", i+1, named.Name())
				} else {
					logging.Debug("  Tool %d: %T (no Name method)", i+1, tool)
				}
			}
			allTools = append(allTools, serverTools...)
		}
	}

	logging.Info("Total tools from all file config servers: %d", len(allTools))
	return allTools, nil
}

// extractToolCallsFromResponse extracts actual tool calls from GenKit response messages
func (iac *IntelligentAgentCreator) extractToolCallsFromResponse(response *ai.ModelResponse, modelName string) []interface{} {
	var toolCalls []interface{}
	
	if response.Request == nil || len(response.Request.Messages) == 0 {
		return toolCalls
	}
	
	stepCounter := 0
	for _, msg := range response.Request.Messages {
		if msg.Role == ai.RoleModel && len(msg.Content) > 0 {
			for _, part := range msg.Content {
				if part.ToolRequest != nil {
					stepCounter++
					toolCall := map[string]interface{}{
						"step":        stepCounter,
						"type":        "tool_call",
						"tool_name":   part.ToolRequest.Name,
						"timestamp":   time.Now().Format(time.RFC3339),
						"model":       modelName,
					}
					
					// Add input parameters if available
					if part.ToolRequest.Input != nil {
						toolCall["input"] = part.ToolRequest.Input
					}
					
					// Find corresponding tool response
					for _, respMsg := range response.Request.Messages {
						if respMsg.Role == ai.RoleTool {
							for _, respPart := range respMsg.Content {
								if respPart.ToolResponse != nil && respPart.ToolResponse.Name == part.ToolRequest.Name {
									toolCall["output"] = respPart.ToolResponse.Output
									toolCall["status"] = "completed"
									break
								}
							}
						}
					}
					
					// Add token usage if available from response
					if response.Usage != nil {
						toolCall["tokens"] = map[string]interface{}{
							"input":  response.Usage.InputTokens,
							"output": response.Usage.OutputTokens,
							"total":  response.Usage.InputTokens + response.Usage.OutputTokens,
						}
					}
					
					toolCalls = append(toolCalls, toolCall)
				}
			}
		}
	}
	
	return toolCalls
}

// buildExecutionStepsFromResponse builds detailed execution steps from GenKit conversation flow
func (iac *IntelligentAgentCreator) buildExecutionStepsFromResponse(response *ai.ModelResponse, agent *models.Agent, modelName string, toolsAvailable int) []interface{} {
	var executionSteps []interface{}
	
	if response.Request == nil || len(response.Request.Messages) == 0 {
		// Fallback: Create basic step
		executionSteps = append(executionSteps, map[string]interface{}{
			"step":        1,
			"type":        "agent_execution",
			"description": fmt.Sprintf("Executed agent '%s' with %d available tools", agent.Name, toolsAvailable),
			"timestamp":   time.Now().Format(time.RFC3339),
			"status":      "completed",
			"model":       modelName,
		})
		return executionSteps
	}
	
	stepCounter := 0
	userMessages := 0
	modelMessages := 0
	toolMessages := 0
	
	// Count message types and build detailed steps
	for _, msg := range response.Request.Messages {
		switch msg.Role {
		case ai.RoleUser:
			userMessages++
			if userMessages == 1 { // Initial user message
				stepCounter++
				executionSteps = append(executionSteps, map[string]interface{}{
					"step":        stepCounter,
					"type":        "user_input",
					"description": "Initial task input received",
					"content":     iac.truncateContent(iac.extractMessageContent(msg), 200),
					"timestamp":   time.Now().Format(time.RFC3339),
					"status":      "completed",
				})
			}
			
		case ai.RoleModel:
			modelMessages++
			stepCounter++
			
			// Check if this model message contains tool calls
			hasToolCalls := false
			toolCallCount := 0
			for _, part := range msg.Content {
				if part.ToolRequest != nil {
					hasToolCalls = true
					toolCallCount++
				}
			}
			
			stepType := "reasoning"
			description := fmt.Sprintf("Model reasoning step %d", modelMessages)
			if hasToolCalls {
				stepType = "tool_planning"
				description = fmt.Sprintf("Model planning to use %d tools", toolCallCount)
			}
			
			executionSteps = append(executionSteps, map[string]interface{}{
				"step":         stepCounter,
				"type":         stepType,
				"description":  description,
				"content":      iac.truncateContent(iac.extractMessageContent(msg), 200),
				"tool_calls":   toolCallCount,
				"timestamp":    time.Now().Format(time.RFC3339),
				"status":       "completed",
				"model":        modelName,
			})
			
		case ai.RoleTool:
			toolMessages++
			stepCounter++
			
			// Get tool name from response
			toolName := "unknown"
			for _, part := range msg.Content {
				if part.ToolResponse != nil {
					toolName = part.ToolResponse.Name
					break
				}
			}
			
			executionSteps = append(executionSteps, map[string]interface{}{
				"step":        stepCounter,
				"type":        "tool_response",
				"description": fmt.Sprintf("Tool '%s' execution completed", toolName),
				"tool_name":   toolName,
				"content":     iac.truncateContent(iac.extractMessageContent(msg), 200),
				"timestamp":   time.Now().Format(time.RFC3339),
				"status":      "completed",
			})
		}
	}
	
	// Add summary step with token usage
	stepCounter++
	summaryStep := map[string]interface{}{
		"step":         stepCounter,
		"type":         "execution_summary",
		"description":  fmt.Sprintf("Execution completed: %d user msgs, %d model msgs, %d tool msgs", userMessages, modelMessages, toolMessages),
		"timestamp":    time.Now().Format(time.RFC3339),
		"status":       "completed",
		"model":        modelName,
		"total_steps":  stepCounter - 1,
	}
	
	// Add token usage and performance metrics
	if response.Usage != nil {
		summaryStep["tokens"] = map[string]interface{}{
			"input":  response.Usage.InputTokens,
			"output": response.Usage.OutputTokens,
			"total":  response.Usage.InputTokens + response.Usage.OutputTokens,
		}
	}
	
	if response.LatencyMs > 0 {
		summaryStep["latency_ms"] = response.LatencyMs
	}
	
	if response.FinishReason != "" {
		summaryStep["finish_reason"] = response.FinishReason
	}
	
	executionSteps = append(executionSteps, summaryStep)
	
	return executionSteps
}

// Helper method to extract content from message parts
func (iac *IntelligentAgentCreator) extractMessageContent(msg *ai.Message) string {
	var content strings.Builder
	
	for i, part := range msg.Content {
		if i > 0 {
			content.WriteString(" | ")
		}
		
		if part.Text != "" {
			content.WriteString(part.Text)
		} else if part.ToolRequest != nil {
			content.WriteString(fmt.Sprintf("[Tool Call: %s]", part.ToolRequest.Name))
		} else if part.ToolResponse != nil {
			content.WriteString(fmt.Sprintf("[Tool Response: %s]", part.ToolResponse.Name))
		}
	}
	
	return content.String()
}

// addToolOutputsToCapturedCalls finds matching tool responses in the conversation 
// and adds outputs to the captured tool calls for OpenAI-compatible providers
func (iac *IntelligentAgentCreator) addToolOutputsToCapturedCalls(capturedToolCalls []map[string]interface{}, response *ai.ModelResponse) {
	if response.Request == nil || len(response.Request.Messages) == 0 {
		logging.Debug("No conversation messages available for tool output matching")
		return
	}
	
	logging.Debug("Searching %d conversation messages for tool outputs", len(response.Request.Messages))
	
	// Build a map of tool responses by tool name for quick lookup
	toolResponses := make(map[string]interface{})
	for _, msg := range response.Request.Messages {
		if msg.Role == ai.RoleTool {
			for _, part := range msg.Content {
				if part.ToolResponse != nil {
					logging.Debug("Found tool response: %s", part.ToolResponse.Name)
					toolResponses[part.ToolResponse.Name] = part.ToolResponse.Output
				}
			}
		}
	}
	
	logging.Debug("Found %d tool responses to match with %d captured calls", len(toolResponses), len(capturedToolCalls))
	
	// Add outputs to captured tool calls
	for i, toolCall := range capturedToolCalls {
		if toolName, ok := toolCall["tool_name"].(string); ok {
			if output, found := toolResponses[toolName]; found {
				logging.Debug("Adding output to tool call: %s", toolName)
				capturedToolCalls[i]["output"] = output
				capturedToolCalls[i]["status"] = "completed"
			} else {
				logging.Debug("No output found for tool call: %s", toolName)
			}
		}
	}
}

// Helper method to truncate content to specified length
func (iac *IntelligentAgentCreator) truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen-3] + "..."
}
