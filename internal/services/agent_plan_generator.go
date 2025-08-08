package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
)

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

// AgentPlanGenerator handles intelligent agent creation and planning
type AgentPlanGenerator struct {
	repos          *repositories.Repositories
	agentService   AgentServiceInterface
	genkitProvider *GenKitProvider
	mcpConnManager *MCPConnectionManager
	mcpClient      *mcp.GenkitMCPClient
}

// NewAgentPlanGenerator creates a new agent plan generator
func NewAgentPlanGenerator(repos *repositories.Repositories, agentService AgentServiceInterface) *AgentPlanGenerator {
	return &AgentPlanGenerator{
		repos:          repos,
		agentService:   agentService,
		genkitProvider: NewGenKitProvider(),
		mcpConnManager: NewMCPConnectionManager(repos, nil),
	}
}

// CreateIntelligentAgent creates a new agent using AI-powered planning
func (apg *AgentPlanGenerator) CreateIntelligentAgent(ctx context.Context, req AgentCreationRequest) (*models.Agent, error) {
	logging.Info("Starting intelligent agent creation for: %s", req.Name)

	// Step 1: Initialize Genkit
	genkitApp, err := apg.genkitProvider.GetApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Genkit: %w", err)
	}
	
	// Update MCP connection manager with GenKit app
	apg.mcpConnManager.genkitApp = genkitApp

	// Step 2: Determine target environment (use specified or validate recommended)
	targetEnvironmentID := req.TargetEnvironmentID
	if targetEnvironmentID == 0 {
		// No target specified, use default
		environments, err := apg.repos.Environments.List()
		if err != nil {
			return nil, fmt.Errorf("failed to list environments: %w", err)
		}
		if len(environments) == 0 {
			return nil, fmt.Errorf("no environments available")
		}
		targetEnvironmentID = environments[0].ID
	}

	// Step 3: Use Genkit agent to analyze requirements and generate creation plan for target environment
	plan, err := apg.generateAgentPlanWithGenkit(ctx, req, targetEnvironmentID, genkitApp)
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
	
	agent, err := apg.repos.Agents.Create(
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
		assignedCount = apg.assignToolsToAgent(agent.ID, plan.CoreTools, targetEnvironmentID)
		logging.Debug("Agent assigned %d tools from %d selected by AI", assignedCount, len(plan.CoreTools))
	}

	// Step 6: Handle scheduling if enabled
	if agent.IsScheduled && agent.CronSchedule != nil {
		logging.Info("Agent '%s' has schedule '%s' - will be handled by scheduler service", agent.Name, *agent.CronSchedule)
		// The scheduler service will pick up this agent automatically on next restart,
		// or we can implement a notification mechanism here if needed
	}

	// Step 7: Cleanup
	if apg.mcpClient != nil {
		apg.mcpClient.Disconnect()
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
func (apg *AgentPlanGenerator) generateAgentPlanWithGenkit(ctx context.Context, req AgentCreationRequest, targetEnvironmentID int64, genkitApp *genkit.Genkit) (*AgentCreationPlan, error) {
	logging.Info("Using Genkit to analyze agent requirements for environment ID %d...", targetEnvironmentID)

	// Get available tools from the target environment
	tools, clients, err := apg.mcpConnManager.GetEnvironmentMCPTools(ctx, targetEnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment MCP tools: %w", err)
	}
	
	// Cleanup connections when done
	defer apg.mcpConnManager.CleanupConnections(clients)

	logging.Debug("Found %d available MCP tools in target environment for analysis", len(tools))

	// Convert tools to tool references and extract tool names
	var toolRefs []ai.Tool
	var availableToolNames []string
	
	for _, tool := range tools {
		// Create tool reference for GenKit
		toolRefs = append(toolRefs, tool)
		
		// Extract tool name for the prompt
		if named, ok := tool.(interface{ Name() string }); ok {
			availableToolNames = append(availableToolNames, named.Name())
		}
	}
	
	if len(availableToolNames) == 0 {
		return nil, fmt.Errorf("no tools available in environment %d for agent planning", targetEnvironmentID)
	}

	// Get environment name for context
	environment, err := apg.repos.Environments.GetByID(targetEnvironmentID)
	if err != nil {
		logging.Debug("Failed to get environment name for ID %d: %v", targetEnvironmentID, err)
	}
	
	environmentName := "unknown"
	if environment != nil {
		environmentName = environment.Name
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create the AI prompt for agent planning
	promptText := fmt.Sprintf(`You are an expert AI agent architect. Create an intelligent agent plan based on these requirements.

USER REQUEST:
Name: %s
Description: %s
User Intent: %s
Domain: %s
Target Environment: %s (ID: %d)

AVAILABLE TOOLS IN THIS ENVIRONMENT (%d tools):
%s

REQUIREMENTS:
1. Analyze the user's intent and select the minimum viable set of tools needed
2. Create a focused system prompt that guides the agent to accomplish the user's goals
3. Recommend appropriate max_steps (1-10, default 5)
4. Suggest a schedule if this should be a recurring task (use cron format, or "on-demand")
5. Provide rationale for your tool selection and configuration

Respond with a JSON object in this exact format:
{
  "agent_name": "string",
  "agent_description": "string", 
  "system_prompt": "string",
  "recommended_env": "string",
  "core_tools": ["tool1", "tool2"],
  "max_steps": number,
  "schedule": "string",
  "rationale": "string",
  "success_criteria": "string"
}

Be specific and practical. Only select tools that are actually needed for the task.`,
		req.Name, req.Description, req.UserIntent, req.Domain,
		environmentName, targetEnvironmentID, len(availableToolNames),
		strings.Join(availableToolNames, "\n"))

	// Generate response using GenKit
	modelName := fmt.Sprintf("%s/%s", cfg.AIProvider, cfg.AIModel)
	response, err := genkit.Generate(ctx, genkitApp,
		ai.WithModelName(modelName),
		ai.WithPrompt(promptText),
		// ai.WithTools(toolRefs...), // Skip tools for agent planning for now
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate agent plan with Genkit: %w", err)
	}

	// Extract text response
	responseText := response.Text()
	
	if responseText == "" {
		return nil, fmt.Errorf("empty response from Genkit")
	}

	logging.Debug("Raw Genkit response for agent planning: %s", responseText)

	// Parse JSON response
	var plan AgentCreationPlan
	// Clean the response to handle potential markdown formatting
	cleanResponse := strings.TrimSpace(responseText)
	if strings.HasPrefix(cleanResponse, "```json") {
		cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
		cleanResponse = strings.TrimSuffix(cleanResponse, "```")
		cleanResponse = strings.TrimSpace(cleanResponse)
	}
	
	if err := json.Unmarshal([]byte(cleanResponse), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse agent plan JSON: %w\nResponse was: %s", err, cleanResponse)
	}

	// Validate and set defaults
	if plan.AgentName == "" {
		plan.AgentName = req.Name
	}
	if plan.MaxSteps == 0 {
		plan.MaxSteps = 5
	}
	if plan.Schedule == "" {
		plan.Schedule = "on-demand"
	}

	// Filter tools to only include ones that actually exist in the environment
	var validTools []string
	for _, toolName := range plan.CoreTools {
		for _, availTool := range availableToolNames {
			if toolName == availTool {
				validTools = append(validTools, toolName)
				break
			}
		}
	}
	plan.CoreTools = validTools

	return &plan, nil
}

// assignToolsToAgent assigns tools to an agent and returns the count of successfully assigned tools
func (apg *AgentPlanGenerator) assignToolsToAgent(agentID int64, toolNames []string, environmentID int64) int {
	assignedCount := 0
	
	for _, toolName := range toolNames {
		// Try to find the tool in the MCP tools table
		tool, err := apg.repos.MCPTools.FindByNameInEnvironment(environmentID, toolName)
		if err != nil {
			// Tool doesn't exist in MCP tools table, create it
			logging.Debug("Creating new MCP tool entry for: %s", toolName)
			mcpTool := &models.MCPTool{
				Name: toolName,
				Description: fmt.Sprintf("Auto-discovered tool: %s", toolName),
				// Add other required fields based on models.MCPTool
			}
			toolID, err := apg.repos.MCPTools.Create(mcpTool)
			if err != nil {
				logging.Debug("Failed to create MCP tool %s: %v", toolName, err)
				continue
			}
			tool = &models.MCPTool{ID: toolID, Name: toolName}
		}
		
		// Assign the tool to the agent
		_, err = apg.repos.AgentTools.AddAgentTool(agentID, tool.ID)
		if err != nil {
			logging.Debug("Failed to assign tool %s to agent %d: %v", toolName, agentID, err)
			continue
		}
		
		assignedCount++
		logging.Debug("Assigned tool '%s' (ID: %d) to agent %d", toolName, tool.ID, agentID)
	}
	
	return assignedCount
}

// ensureEnvironment ensures an environment exists, creating it if necessary
func (apg *AgentPlanGenerator) ensureEnvironment(envName string) (int64, error) {
	// Try to find existing environment
	environments, err := apg.repos.Environments.List()
	if err != nil {
		return 0, fmt.Errorf("failed to list environments: %w", err)
	}
	
	for _, env := range environments {
		if env.Name == envName {
			return env.ID, nil
		}
	}
	
	// Environment doesn't exist, create it
	desc := fmt.Sprintf("Auto-created environment for: %s", envName)
	env, err := apg.repos.Environments.Create(envName, &desc, 1)
	if err != nil {
		return 0, fmt.Errorf("failed to create environment %s: %w", envName, err)
	}
	
	return env.ID, nil
}