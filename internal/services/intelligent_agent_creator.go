package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"station/internal/config"
	"station/internal/db/repositories"
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
	genkitApp    *genkit.Genkit
	mcpClient    *mcp.GenkitMCPClient
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

	// Set up OpenTelemetry export to Jaeger using Genkit's RegisterSpanProcessor
	if err := iac.setupOpenTelemetryExport(ctx, genkitApp); err != nil {
		log.Printf("⚠️ Failed to set up OpenTelemetry export: %v", err)
		// Don't fail initialization, just log the warning
	} else {
		log.Printf("✅ OpenTelemetry telemetry configured successfully")
	}

	// Create MCP client to connect to our own stdio server
	mcpClient, err := mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
		Name:    "s", // Short name to minimize tool call IDs in OpenAI
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

	log.Printf("📋 Generated plan for agent '%s' with %d tools for environment ID %d", plan.AgentName, len(plan.CoreTools), targetEnvironmentID)

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
		log.Printf("📋 Agent assigned %d tools from %d selected by AI", assignedCount, len(plan.CoreTools))
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

// generateAgentPlanWithGenkit uses Genkit with environment-specific MCP tools to intelligently analyze requirements
func (iac *IntelligentAgentCreator) generateAgentPlanWithGenkit(ctx context.Context, req AgentCreationRequest, targetEnvironmentID int64) (*AgentCreationPlan, error) {
	log.Printf("🔍 Using Genkit to analyze agent requirements for environment ID %d...", targetEnvironmentID)

	// Get available tools from the target environment
	tools, err := iac.getEnvironmentMCPTools(ctx, targetEnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment MCP tools: %w", err)
	}

	log.Printf("📋 Found %d available MCP tools in target environment for analysis", len(tools))

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
	log.Printf("🔍 DEBUG: cfg.AIProvider='%s', cfg.AIModel='%s'", cfg.AIProvider, cfg.AIModel)
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
	log.Printf("🤖 Agent creation response: %s", responseText)
	if response.Usage != nil {
		log.Printf("🔍 Agent creation usage - Input tokens: %d, Output tokens: %d, Total tokens: %d", 
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
		log.Printf("🔍 Agent creation took %d turns, %d total messages in conversation", 
			modelMessages, len(response.Request.Messages))
	}

	// Try to extract JSON from the response (might be wrapped in markdown)
	jsonStart := strings.Index(responseText, "{")
	jsonEnd := strings.LastIndex(responseText, "}") + 1
	if jsonStart == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no valid JSON found in response: %s", responseText)
	}

	jsonStr := responseText[jsonStart:jsonEnd]
	log.Printf("🔍 Extracted JSON: %s", jsonStr)
	
	err = json.Unmarshal([]byte(jsonStr), &plan)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent plan JSON: %w. JSON was: %s", err, jsonStr)
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
		// AI analysis returns prefixed names but database stores clean names
		// Strip prefix to match database storage: "f_read_text_file" -> "read_text_file"
		dbToolName := toolName
		if strings.Contains(toolName, "_") {
			parts := strings.SplitN(toolName, "_", 2)
			if len(parts) == 2 {
				dbToolName = parts[1] // Remove prefix for database lookup
				log.Printf("🔄 Mapping AI tool name for database: %s -> %s", toolName, dbToolName)
			}
		}
		
		tool, err := iac.repos.MCPTools.FindByNameInEnvironment(environmentID, dbToolName)
		if err != nil {
			log.Printf("Warning: Failed to find tool '%s' in environment %d: %v", toolName, environmentID, err)
			continue
		}
		
		// Add the tool assignment to the agent
		_, err = iac.repos.AgentTools.AddAgentTool(agentID, tool.ID)
		if err != nil {
			log.Printf("Warning: Failed to assign tool '%s' (ID: %d) to agent %d: %v", tool.Name, tool.ID, agentID, err)
			continue
		}
		
		assignedCount++
		log.Printf("✓ Successfully assigned tool '%s' (ID: %d) to agent %d", tool.Name, tool.ID, agentID)
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
	log.Printf("🤖 Starting stdio MCP agent execution for agent '%s'", agent.Name)

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

	log.Printf("📋 Agent has %d assigned tools for execution", len(assignedTools))

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
				log.Printf("✅ Tool match found: MCP '%s' matches assigned '%s'", toolName, assignedTool.ToolName)
				
				// DEBUG: Log what Station passes to Genkit
				toolRef := ai.ToolRef(mcpTool)
				log.Printf("🔍 DEBUG: Station creating ToolRef with Name='%s' (length=%d)", toolRef.Name(), len(toolRef.Name()))
				
				tools = append(tools, toolRef)
				log.Printf("🔧 Including assigned tool: %s", toolName)
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
	log.Printf("🔍 DEBUG: cfg.AIProvider='%s', cfg.AIModel='%s'", cfg.AIProvider, cfg.AIModel)
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

	// Note: Genkit automatically captures all telemetry data:
	// - Token usage (input/output tokens)  
	// - Tool calls (requests/responses with parameters)
	// - Prompts and responses (complete I/O logging)
	// - Performance metrics (latency, request counts)
	// - OpenTelemetry traces automatically exported to Jaeger

	// Use Genkit to execute the agent with assigned tools only
	var generateOptions []ai.GenerateOption
	generateOptions = append(generateOptions, ai.WithModelName(modelName))
	generateOptions = append(generateOptions, ai.WithPrompt(executionPrompt))
	generateOptions = append(generateOptions, ai.WithMaxTurns(25)) // Support multi-step workflows
	
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
		log.Printf("❌ Agent execution failed after %dms (Genkit captured error telemetry automatically)", executionTime)
		return nil, fmt.Errorf("failed to execute agent via stdio MCP: %w", err)
	}

	// Extract execution results
	responseText := response.Text()
	log.Printf("🤖 Agent execution completed via stdio MCP")
	
	// DEBUG: Log comprehensive response details to understand multi-turn behavior
	log.Printf("🔍 DEBUG: Response text length: %d characters", len(responseText))
	log.Printf("🔍 DEBUG: Response type: %T", response)
	log.Printf("🔍 DEBUG: Final response text: %s", responseText)
	
	// Try to access usage or turn information if available
	if response.Usage != nil {
		log.Printf("🔍 DEBUG: Usage - Input tokens: %d, Output tokens: %d, Total tokens: %d", 
			response.Usage.InputTokens, response.Usage.OutputTokens, 
			response.Usage.InputTokens + response.Usage.OutputTokens)
	}
	
	// Log additional response metadata
	log.Printf("🔍 DEBUG: Finish reason: %s", response.FinishReason)
	if response.FinishMessage != "" {
		log.Printf("🔍 DEBUG: Finish message: %s", response.FinishMessage)
	}
	log.Printf("🔍 DEBUG: Latency: %.2fms", response.LatencyMs)
	
	// Check if response has any turn/step information
	log.Printf("🔍 DEBUG: Looking for multi-turn indicators in response...")

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
			log.Printf("🔍 DEBUG: Found %d model messages in conversation history", modelMessages)
		}
		log.Printf("🔍 DEBUG: Message count breakdown - User: %d, Model: %d, Tool: %d, System: %d, Total: %d", 
			userMessages, modelMessages, toolMessages, systemMessages, len(response.Request.Messages))
		
		// Debug: Log detailed conversation flow with content preview
		log.Printf("🔍 DEBUG: === CONVERSATION FLOW ===")
		for i, msg := range response.Request.Messages {
			contentPreview := ""
			if len(msg.Content) > 0 {
				// Get text content from the first part if available
				if msg.Content[0] != nil && msg.Content[0].Text != "" {
					contentPreview = msg.Content[0].Text
					if len(contentPreview) > 100 {
						contentPreview = contentPreview[:100] + "..."
					}
				}
			}
			log.Printf("🔍 DEBUG: Step %d - Role: %s, Content: %s", i+1, msg.Role, contentPreview)
			
			// Log tool calls if this is a model message with tool requests
			if msg.Role == ai.RoleModel && len(msg.Content) > 0 {
				for j, part := range msg.Content {
					if part.ToolRequest != nil {
						toolCallPreview := fmt.Sprintf("Tool: %s", part.ToolRequest.Name)
						if part.ToolRequest.Input != nil {
							if inputBytes, err := json.Marshal(part.ToolRequest.Input); err == nil {
								inputStr := string(inputBytes)
								if len(inputStr) > 100 {
									inputStr = inputStr[:100] + "..."
								}
								toolCallPreview += fmt.Sprintf(", Input: %s", inputStr)
							}
						}
						log.Printf("🔍 DEBUG:   Part %d - %s", j+1, toolCallPreview)
					}
				}
			}
			
			// Log tool responses if this is a tool message
			if msg.Role == ai.RoleTool && len(msg.Content) > 0 {
				for j, part := range msg.Content {
					if part.ToolResponse != nil {
						toolRespPreview := fmt.Sprintf("Tool Response: %s", part.ToolResponse.Name)
						if part.ToolResponse.Output != nil {
							if outputBytes, err := json.Marshal(part.ToolResponse.Output); err == nil {
								outputStr := string(outputBytes)
								if len(outputStr) > 100 {
									outputStr = outputStr[:100] + "..."
								}
								toolRespPreview += fmt.Sprintf(", Output: %s", outputStr)
							}
						}
						log.Printf("🔍 DEBUG:   Part %d - %s", j+1, toolRespPreview)
					}
				}
			}
		}
		log.Printf("🔍 DEBUG: === END CONVERSATION FLOW ===")
	}
	var toolCalls *models.JSONArray
	var executionSteps *models.JSONArray

	// Update basic run information (Genkit captures detailed telemetry automatically)
	if runID > 0 {
		// Fallback: Create basic execution data when telemetry is not available
		log.Printf("📊 No telemetry client - using basic execution data")
		
		// Create basic tool calls data
		basicToolCalls := []interface{}{
			map[string]interface{}{
				"type":        "generation",
				"model":       modelName,
				"timestamp":   time.Now().Format(time.RFC3339),
				"status":      "completed",
				"tools_available": len(toolRefs),
			},
		}
		toolCallsArray := models.JSONArray(basicToolCalls)
		toolCalls = &toolCallsArray
		
		// Create basic execution steps data
		basicSteps := []interface{}{
			map[string]interface{}{
				"step":        1,
				"type":        "agent_execution",
				"description": fmt.Sprintf("Executed agent '%s' with %d available tools", agent.Name, len(toolRefs)),
				"timestamp":   time.Now().Format(time.RFC3339),
				"status":      "completed",
				"model":       modelName,
			},
		}
		executionStepsArray := models.JSONArray(basicSteps)
		executionSteps = &executionStepsArray
		
		log.Printf("📊 Basic execution data created (Genkit provides comprehensive telemetry automatically)")
	}

	result := &AgentExecutionResult{
		Response:       responseText,
		StepsTaken:     stepsTaken,
		ToolCalls:      toolCalls,
		ExecutionSteps: executionSteps,
	}

	log.Printf("✅ Stdio MCP agent execution completed: %d steps taken", stepsTaken)

	// Track agent execution with PostHog telemetry
	executionTime := time.Since(startTime).Milliseconds()
	
	log.Printf("✅ Agent execution completed in %dms with %d steps (Genkit captured comprehensive telemetry automatically)", executionTime, stepsTaken)

	log.Printf("✅ Environment MCP agent execution completed successfully")
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
	
	log.Printf("📊 OpenTelemetry configured to export traces to Jaeger at localhost:4317 (service: station-agent)")
	log.Printf("🔍 Rich telemetry data (tokens, usage, costs) will appear in span attributes")
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

	log.Printf("🌍 Getting MCP tools for environment: %s (ID: %d)", environment.Name, environmentID)

	// Get file configs for this environment
	log.Printf("🔍 Querying database for file configs with environment ID: %d", environmentID)
	fileConfigs, err := iac.repos.FileMCPConfigs.ListByEnvironment(environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file configs for environment %d: %w", environmentID, err)
	}

	log.Printf("📋 Database query returned %d file configs for environment %d", len(fileConfigs), environmentID)
	for i, config := range fileConfigs {
		log.Printf("  🗂️ Config %d: %s (ID: %d, Template: %s)", i+1, config.ConfigName, config.ID, config.TemplatePath)
	}

	var allTools []ai.Tool

	// Connect to each MCP server from file configs and get their tools
	for _, fileConfig := range fileConfigs {
		log.Printf("📁 Processing file config: %s (ID: %d), template path: %s", fileConfig.ConfigName, fileConfig.ID, fileConfig.TemplatePath)
		
		// Make template path absolute (relative to ~/.config/station/)
		configDir := os.ExpandEnv("$HOME/.config/station")
		absolutePath := fmt.Sprintf("%s/%s", configDir, fileConfig.TemplatePath)
		
		log.Printf("📂 Reading file config from: %s", absolutePath)
		
		// Read the actual file content from template path
		content, err := os.ReadFile(absolutePath)
		if err != nil {
			log.Printf("⚠️ Failed to read file config %s from %s: %v", fileConfig.ConfigName, absolutePath, err)
			continue
		}

		log.Printf("📄 File config content loaded: %d bytes", len(content))

		// Parse the file config content to get server configurations
		// The JSON files use "mcpServers" but the struct expects "servers" - handle both
		var rawConfig map[string]interface{}
		if err := json.Unmarshal(content, &rawConfig); err != nil {
			log.Printf("⚠️ Failed to parse file config %s: %v", fileConfig.ConfigName, err)
			continue
		}

		// Extract servers from either "mcpServers" or "servers" field
		var serversData map[string]interface{}
		if mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{}); ok {
			serversData = mcpServers
		} else if servers, ok := rawConfig["servers"].(map[string]interface{}); ok {
			serversData = servers
		} else {
			log.Printf("⚠️ No 'mcpServers' or 'servers' field found in config %s", fileConfig.ConfigName)
			continue
		}

		log.Printf("🔍 Parsed config data with %d servers", len(serversData))

		// Process each server in the config
		for serverName, serverConfigRaw := range serversData {
			// Convert the server config to proper structure
			serverConfigBytes, err := json.Marshal(serverConfigRaw)
			if err != nil {
				log.Printf("⚠️ Failed to marshal server config for %s: %v", serverName, err)
				continue
			}
			
			var serverConfig models.MCPServerConfig
			if err := json.Unmarshal(serverConfigBytes, &serverConfig); err != nil {
				log.Printf("⚠️ Failed to unmarshal server config for %s: %v", serverName, err)
				continue
			}
			// Determine transport type based on config fields
			var mcpClient *mcp.GenkitMCPClient
			if serverConfig.URL != "" {
				// HTTP-based MCP server
				log.Printf("🔌 Connecting to HTTP MCP server: %s (URL: %s)", serverName, serverConfig.URL)
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
				log.Printf("🔌 Connecting to Stdio MCP server: %s (command: %s, args: %v)", serverName, serverConfig.Command, serverConfig.Args)
				
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
				log.Printf("⚠️ Invalid MCP server config for %s: missing both URL and Command fields", serverName)
				continue
			}
			if err != nil {
				log.Printf("⚠️ Failed to create MCP client for %s: %v", serverName, err)
				continue
			}

			// Get tools from this MCP server with timeout and panic recovery
			log.Printf("🔍 Attempting to get tools from MCP server: %s", serverName)
			
			// Create a timeout context for the tool discovery
			toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			
			var serverTools []ai.Tool
			func() {
				// Recover from potential panics in the MCP client
				defer func() {
					if r := recover(); r != nil {
						log.Printf("🚨 Panic recovered while getting tools from %s: %v", serverName, r)
						err = fmt.Errorf("panic in MCP client: %v", r)
					}
				}()
				
				serverTools, err = mcpClient.GetActiveTools(toolCtx, iac.genkitApp)
			}()
			
			if err != nil {
				log.Printf("⚠️ Failed to get tools from %s: %v", serverName, err)
				if serverConfig.URL != "" {
					log.Printf("🔍 HTTP MCP server details - Name: %s, URL: %s", serverName, serverConfig.URL)
				} else {
					log.Printf("🔍 Stdio MCP server details - Name: %s, Command: %s, Args: %v", serverName, serverConfig.Command, serverConfig.Args)
				}
				continue
			}

			log.Printf("🔧 Found %d tools from %s", len(serverTools), serverName)
			for i, tool := range serverTools {
				if named, ok := tool.(interface{ Name() string }); ok {
					log.Printf("  📋 Tool %d: %s", i+1, named.Name())
				} else {
					log.Printf("  📋 Tool %d: %T (no Name method)", i+1, tool)
				}
			}
			allTools = append(allTools, serverTools...)
		}
	}

	log.Printf("🗂️ Total tools from all file config servers: %d", len(allTools))
	return allTools, nil
}
