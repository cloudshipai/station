package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"station/internal/config"
	"station/internal/db/repositories"
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
	mcpConfigSvc *MCPConfigService
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

func NewIntelligentAgentCreator(repos *repositories.Repositories, agentService AgentServiceInterface, mcpConfigSvc *MCPConfigService) *IntelligentAgentCreator {
	return &IntelligentAgentCreator{
		repos:        repos,
		agentService: agentService,
		mcpConfigSvc: mcpConfigSvc,
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
	agent, err := iac.repos.Agents.Create(
		plan.AgentName,
		plan.AgentDescription,
		plan.SystemPrompt,
		environmentID,
		int64(plan.MaxSteps),
		1,   // Default user ID for now
		nil, // Tool assignments handled separately
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Step 5: Assign tools to the agent
	assignedCount := 0
	if len(plan.CoreTools) > 0 {
		assignedCount = iac.assignToolsToAgent(agent.ID, plan.CoreTools, environmentID)
	}

	// Step 6: Cleanup
	if iac.mcpClient != nil {
		iac.mcpClient.Disconnect()
	}

	log.Printf("✅ Successfully created intelligent agent '%s' (ID: %d) with %d tools",
		agent.Name, agent.ID, assignedCount)

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

Please respond with a JSON object in this exact format:
{
  "agent_name": "%s",
  "agent_description": "%s", 
  "system_prompt": "detailed system prompt for the agent...",
  "recommended_environment": "environment_name",
  "core_tools": ["tool1", "tool2", "tool3"],
  "max_steps": 5,
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
		_, err := iac.repos.AgentTools.Add(agentID, toolName, environmentID)
		if err != nil {
			log.Printf("Warning: Failed to assign tool '%s' to agent %d: %v", toolName, agentID, err)
			// Continue with other tools even if one fails
		} else {
			assignedCount++
			log.Printf("✓ Successfully assigned tool '%s' to agent %d", toolName, agentID)
		}
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
	log.Printf("🤖 Starting stdio MCP agent execution for agent '%s'", agent.Name)

	// Initialize Genkit + MCP if not already done
	err := iac.initializeGenkit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Genkit for agent execution: %w", err)
	}

	// Get available tools from our MCP server
	tools, err := iac.mcpClient.GetActiveTools(ctx, iac.genkitApp)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP tools for execution: %w", err)
	}

	log.Printf("📋 Found %d available MCP tools for agent execution", len(tools))

	// Convert tools to tool references
	var toolRefs []ai.ToolRef
	for _, tool := range tools {
		toolRefs = append(toolRefs, tool)
	}

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

	// Use Genkit to execute the agent with MCP tools
	response, err := genkit.Generate(ctx, iac.genkitApp,
		ai.WithModelName(modelName),
		ai.WithPrompt(executionPrompt),
		ai.WithTools(toolRefs...),
		ai.WithToolChoice(ai.ToolChoiceAuto),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute agent via stdio MCP: %w", err)
	}

	// Extract execution results
	responseText := response.Text()
	log.Printf("🤖 Agent execution completed via stdio MCP")

	// For now, we'll create basic execution result
	// In a full implementation, we would parse tool calls and execution steps from the response
	result := &AgentExecutionResult{
		Response:   responseText,
		StepsTaken: 1, // Would be calculated based on actual execution
		// ToolCalls and ExecutionSteps would be extracted from the actual response
	}

	// TODO: Parse actual tool calls and execution steps from the Genkit response
	// This would involve extracting the tool usage information from the response

	log.Printf("✅ Stdio MCP agent execution completed successfully")
	return result, nil
}
