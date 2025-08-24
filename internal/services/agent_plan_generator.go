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
	repos             *repositories.Repositories
	agentService      AgentServiceInterface
	genkitProvider    *GenKitProvider
	mcpConnManager    *MCPConnectionManager
	mcpClient         *mcp.GenkitMCPClient
	agentExportService *AgentExportService
}

// NewAgentPlanGenerator creates a new agent plan generator
func NewAgentPlanGenerator(repos *repositories.Repositories, agentService AgentServiceInterface) *AgentPlanGenerator {
	return &AgentPlanGenerator{
		repos:             repos,
		agentService:      agentService,
		genkitProvider:    NewGenKitProvider(),
		mcpConnManager:    NewMCPConnectionManager(repos, nil),
		agentExportService: NewAgentExportService(repos),
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
		nil, // input_schema - not set in plan generator
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

	// Step 5.5: Automatically export agent to file-based config after successful DB save
	if err := apg.agentExportService.ExportAgentAfterSave(agent.ID); err != nil {
		// Log the error but don't fail the agent creation - the agent was successfully created in DB
		logging.Info("Failed to export agent %d after creation: %v", agent.ID, err)
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

	// Convert tools to tool references and extract tool names with descriptions
	var toolRefs []ai.Tool
	var toolDescriptions []string
	
	for _, tool := range tools {
		// Create tool reference for GenKit
		toolRefs = append(toolRefs, tool)
		
		// Extract tool name and description for intelligent analysis
		if named, ok := tool.(interface{ Name() string }); ok {
			toolName := named.Name()
			toolDesc := "No description available"
			
			// Try to get description from tool definition
			if definable, ok := tool.(interface{ Definition() *ai.ToolDefinition }); ok {
				if def := definable.Definition(); def != nil && def.Description != "" {
					toolDesc = def.Description
				}
			}
			
			// Format as "name: description" for AI analysis
			toolDescriptions = append(toolDescriptions, fmt.Sprintf("- %s: %s", toolName, toolDesc))
		}
	}
	
	if len(toolDescriptions) == 0 {
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

	// Create the AI prompt for agent planning with enhanced tool analysis
	promptText := fmt.Sprintf(`You are an expert AI Agent Architect and Tool Selection Specialist. Your job is to analyze the user's requirements and intelligently select the optimal tools from the available toolkit to create a highly effective agent.

USER REQUEST ANALYSIS:
Name: %s
Description: %s  
User Intent: %s
Domain: %s
Target Environment: %s (ID: %d)

AVAILABLE TOOLS FOR ANALYSIS (%d tools):
%s

TOOL SELECTION METHODOLOGY:
1. UNDERSTAND THE GOAL: Carefully analyze what the user wants to accomplish
2. SCAN ALL TOOLS: Review every available tool and its description/capabilities
3. IDENTIFY KEY ACTIONS: Break down the user's goal into specific actions needed
4. MAP TOOLS TO ACTIONS: Match each required action to the most appropriate tool(s)
5. OPTIMIZE SELECTION: Choose the minimum viable set that maximizes capability

TOOL SELECTION GUIDELINES:
- Dashboard tasks → Look for: search_dashboards, get_dashboard_by_uid, list_datasources
- Data analysis → Look for: query_prometheus, query_loki_logs, query_loki_stats
- Monitoring/alerts → Look for: list_alert_rules, get_alert_rule_by_uid, list_incidents
- AWS tasks → Look for: __call_aws, __suggest_aws_commands
- Investigation → Look for: find_error_pattern_logs, find_slow_requests
- Management → Look for: create_incident, add_activity_to_incident
- Discovery → Look for: list_* tools for finding resources
- Analysis → Look for: get_* tools for detailed information

SCHEDULING LOGIC:
- One-time analysis/search tasks → "on-demand"
- Monitoring/health checks → "*/5 * * * *" (every 5 minutes)  
- Daily reports/cleanup → "0 9 * * *" (daily at 9am)
- Incident response → "on-demand" (human-triggered)

STEP CALCULATION:
- Simple lookup/search: 2-3 steps
- Analysis with multiple data sources: 4-6 steps  
- Complex workflows with incident creation: 6-8 steps
- Investigation and remediation: 8-10 steps

You MUST analyze the tool descriptions carefully and select tools that directly support the user's stated goals. DO NOT default to generic incident management tools unless the user specifically mentions incident response.

Respond with a JSON object in this exact format:
{
  "agent_name": "string",
  "agent_description": "string", 
  "system_prompt": "string",
  "recommended_env": "%s (ID: %d)",
  "core_tools": ["tool1", "tool2", "tool3"],
  "max_steps": number,
  "schedule": "string",
  "rationale": "string - explain WHY you selected each tool and how it supports the user's goal",
  "success_criteria": "string - specific measurable outcomes"
}

CRITICAL: Read through ALL available tools before making your selection. Match tools to the user's actual intent, not generic patterns. If the user mentions "dashboard", "search", "analyze", "monitor", etc., find the specific tools that do those things.`,
		req.Name, req.Description, req.UserIntent, req.Domain,
		environmentName, targetEnvironmentID, len(toolDescriptions),
		strings.Join(toolDescriptions, "\n"),
		environmentName, targetEnvironmentID)

	// Generate response using GenKit with proper provider-specific model selection
	var modelName string
	switch strings.ToLower(cfg.AIProvider) {
	case "openai":
		// Station's OpenAI plugin registers models with station-openai provider prefix
		modelName = fmt.Sprintf("station-openai/%s", cfg.AIModel)
	case "googlegenai", "gemini":
		// Gemini models need the googleai provider prefix
		modelName = fmt.Sprintf("googleai/%s", cfg.AIModel)
	default:
		modelName = fmt.Sprintf("%s/%s", cfg.AIProvider, cfg.AIModel)
	}
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

	// Extract available tool names from descriptions for validation
	var availableToolNames []string
	for _, tool := range tools {
		if named, ok := tool.(interface{ Name() string }); ok {
			availableToolNames = append(availableToolNames, named.Name())
		}
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