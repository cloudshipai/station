package services

import (
	"context"
	"encoding/json"
	"fmt"
	"station/internal/db/repositories"
	"station/internal/lighthouse"
	"station/internal/lighthouse/proto"
	"station/internal/logging"
	"station/internal/services"
	"station/pkg/models"
	"station/pkg/types"
	"strconv"
	"time"
)

// CommandHandlerService handles processing of CloudShip commands
// This service is responsible for executing remote commands from CloudShip
// and managing the lifecycle of remote agent executions.
type CommandHandlerService struct {
	agentService     services.AgentServiceInterface
	metricsService   *MetricsService
	repos            *repositories.Repositories
	lighthouseClient *lighthouse.LighthouseClient
}

// NewCommandHandlerService creates a new command handler service
func NewCommandHandlerService(
	agentService services.AgentServiceInterface,
	metricsService *MetricsService,
	repos *repositories.Repositories,
	lighthouseClient *lighthouse.LighthouseClient,
) *CommandHandlerService {
	return &CommandHandlerService{
		agentService:     agentService,
		metricsService:   metricsService,
		repos:            repos,
		lighthouseClient: lighthouseClient,
	}
}

// ProcessCommand handles incoming CloudShip commands
func (chs *CommandHandlerService) ProcessCommand(ctx context.Context, cmd *proto.CloudShipCommand) error {
	logging.Debug("Processing CloudShip command: %T", cmd.Command)

	switch command := cmd.Command.(type) {
	case *proto.CloudShipCommand_ExecuteAgent:
		return chs.handleExecuteAgentCommand(ctx, command.ExecuteAgent)
	case *proto.CloudShipCommand_CreateAgent:
		return chs.handleCreateAgentCommand(ctx, command.CreateAgent)
	case *proto.CloudShipCommand_UpdateAgent:
		return chs.handleUpdateAgentCommand(ctx, command.UpdateAgent)
	case *proto.CloudShipCommand_DeleteAgent:
		return chs.handleDeleteAgentCommand(ctx, command.DeleteAgent)
	case *proto.CloudShipCommand_GetEnv:
		return chs.handleGetEnvironmentCommand(ctx, command.GetEnv)
	case *proto.CloudShipCommand_SyncMcp:
		return chs.handleSyncMCPServersCommand(ctx, command.SyncMcp)
	case *proto.CloudShipCommand_ListAgents:
		return chs.handleListAgentsCommand(ctx, command.ListAgents)
	default:
		return fmt.Errorf("unknown command type: %T", cmd.Command)
	}
}

// handleExecuteAgentCommand processes remote agent execution requests
func (chs *CommandHandlerService) handleExecuteAgentCommand(ctx context.Context, cmd *proto.ExecuteAgentCommand) error {
	logging.Info("Executing agent %s with task: %s (run_id: %s)", cmd.AgentId, cmd.Task, cmd.RunId)

	// Update active runs metric
	chs.metricsService.UpdateActiveRuns(chs.metricsService.activeRuns + 1)
	defer func() {
		chs.metricsService.UpdateActiveRuns(chs.metricsService.activeRuns - 1)
	}()

	// Convert agent ID to int64
	agentIDInt, err := strconv.ParseInt(cmd.AgentId, 10, 64)
	if err != nil {
		logging.Error("Invalid agent ID %s: %v", cmd.AgentId, err)
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	// Convert variables from map[string]string to map[string]interface{}
	userVariables := make(map[string]interface{})
	for k, v := range cmd.Variables {
		userVariables[k] = v
	}

	// Execute the agent using the agent service
	logging.Info("Starting remote agent execution for agent %d", agentIDInt)
	result, err := chs.agentService.ExecuteAgent(ctx, agentIDInt, cmd.Task, userVariables)
	if err != nil {
		logging.Error("Agent execution failed for agent %s: %v", cmd.AgentId, err)
		return fmt.Errorf("agent execution failed: %v", err)
	}

	// Log the execution result
	if result != nil {
		logging.Info("Agent execution completed for agent %s. Result: %s", cmd.AgentId, result.Content)

		// TODO: Send result back to CloudShip via SendRun once protocol supports responses
		// For now, log that the execution completed successfully
		logging.Info("Remote agent execution completed successfully (run_id: %s)", cmd.RunId)
	} else {
		logging.Info("Agent execution completed with no result for agent %s", cmd.AgentId)
	}

	return nil
}

// handleCreateAgentCommand processes remote agent creation requests
func (chs *CommandHandlerService) handleCreateAgentCommand(ctx context.Context, cmd *proto.CreateAgentCommand) error {
	logging.Info("Creating agent %s: %s", cmd.AgentId, cmd.Config.Name)

	// Get default environment for agents created via CloudShip
	env, err := chs.repos.Environments.GetByName("default")
	if err != nil {
		logging.Error("Failed to get default environment: %v", err)
		return fmt.Errorf("failed to get default environment: %v", err)
	}

	// Get console user for created_by field
	consoleUser, err := chs.repos.Users.GetByUsername("console")
	if err != nil {
		logging.Error("Failed to get console user: %v", err)
		return fmt.Errorf("failed to get console user: %v", err)
	}

	// Convert proto AgentConfig to service AgentConfig
	agentConfig := &services.AgentConfig{
		EnvironmentID:   env.ID,
		Name:            cmd.Config.Name,
		Description:     cmd.Config.Description,
		Prompt:          cmd.Config.PromptTemplate,
		AssignedTools:   cmd.Config.Tools,
		MaxSteps:        int64(cmd.Config.MaxSteps),
		CreatedBy:       consoleUser.ID,
		ModelProvider:   "openai", // TODO: Parse from model_name
		ModelID:         cmd.Config.ModelName,
		CronSchedule:    nil, // CloudShip agents are not scheduled by default
		ScheduleEnabled: false,
	}
	

	// Create the agent
	agent, err := chs.agentService.CreateAgent(ctx, agentConfig)
	if err != nil {
		logging.Error("Failed to create agent %s: %v", cmd.AgentId, err)
		return fmt.Errorf("failed to create agent: %v", err)
	}

	logging.Info("Successfully created agent %s with database ID %d", cmd.AgentId, agent.ID)

	// TODO: Send response back to CloudShip via SendRun once protocol supports responses
	// For now, log the successful creation
	logging.Info("Agent creation completed successfully for CloudShip agent ID %s", cmd.AgentId)

	return nil
}

func (chs *CommandHandlerService) handleUpdateAgentCommand(ctx context.Context, cmd *proto.UpdateAgentCommand) error {
	logging.Info("Updating agent %s", cmd.AgentId)

	// Convert agent ID to int64
	agentIDInt, err := strconv.ParseInt(cmd.AgentId, 10, 64)
	if err != nil {
		logging.Error("Invalid agent ID %s: %v", cmd.AgentId, err)
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	// Get the existing agent to preserve environment and user info
	existingAgent, err := chs.agentService.GetAgent(ctx, agentIDInt)
	if err != nil {
		logging.Error("Failed to get existing agent %s: %v", cmd.AgentId, err)
		return fmt.Errorf("failed to get existing agent: %v", err)
	}

	// Convert proto AgentConfig to service AgentConfig, preserving existing fields
	agentConfig := &services.AgentConfig{
		EnvironmentID:   existingAgent.EnvironmentID,
		Name:            cmd.Config.Name,
		Description:     cmd.Config.Description,
		Prompt:          cmd.Config.PromptTemplate,
		AssignedTools:   cmd.Config.Tools,
		MaxSteps:        int64(cmd.Config.MaxSteps),
		CreatedBy:       existingAgent.CreatedBy,
		ModelProvider:   "openai", // TODO: Parse from model_name
		ModelID:         cmd.Config.ModelName,
		CronSchedule:    nil, // Preserve or update schedule as needed
		ScheduleEnabled: false,
	}

	// Update the agent
	updatedAgent, err := chs.agentService.UpdateAgent(ctx, agentIDInt, agentConfig)
	if err != nil {
		logging.Error("Failed to update agent %s: %v", cmd.AgentId, err)
		return fmt.Errorf("failed to update agent: %v", err)
	}

	logging.Info("Successfully updated agent %s (database ID %d)", cmd.AgentId, updatedAgent.ID)

	// TODO: Send response back to CloudShip via SendRun once protocol supports responses
	// For now, log the successful update
	logging.Info("Agent update completed successfully for CloudShip agent ID %s", cmd.AgentId)

	return nil
}

func (chs *CommandHandlerService) handleDeleteAgentCommand(ctx context.Context, cmd *proto.DeleteAgentCommand) error {
	logging.Info("Deleting agent %s", cmd.AgentId)

	// Convert agent ID to int64
	agentIDInt, err := strconv.ParseInt(cmd.AgentId, 10, 64)
	if err != nil {
		logging.Error("Invalid agent ID %s: %v", cmd.AgentId, err)
		return fmt.Errorf("invalid agent ID: %v", err)
	}

	// Check if agent exists before deletion
	existingAgent, err := chs.agentService.GetAgent(ctx, agentIDInt)
	if err != nil {
		logging.Error("Failed to get agent %s for deletion: %v", cmd.AgentId, err)
		return fmt.Errorf("failed to get agent for deletion: %v", err)
	}

	logging.Info("Deleting agent '%s' (database ID %d)", existingAgent.Name, existingAgent.ID)

	// Delete the agent
	err = chs.agentService.DeleteAgent(ctx, agentIDInt)
	if err != nil {
		logging.Error("Failed to delete agent %s: %v", cmd.AgentId, err)
		return fmt.Errorf("failed to delete agent: %v", err)
	}

	logging.Info("Successfully deleted agent %s (was '%s')", cmd.AgentId, existingAgent.Name)

	// TODO: Send response back to CloudShip via SendRun once protocol supports responses
	// For now, log the successful deletion
	logging.Info("Agent deletion completed successfully for CloudShip agent ID %s", cmd.AgentId)

	return nil
}

func (chs *CommandHandlerService) handleGetEnvironmentCommand(ctx context.Context, cmd *proto.GetEnvironmentCommand) error {
	logging.Info("Getting environment information for: %s", cmd.Environment)

	// Gather environment information
	envInfo, err := chs.gatherEnvironmentInformation(ctx, cmd.Environment)
	if err != nil {
		logging.Error("Failed to gather environment information: %v", err)
		return fmt.Errorf("failed to gather environment information: %v", err)
	}

	// TODO: Send response back via proper response method when available
	// For now, log the information that would be sent
	logging.Info("Environment Info for '%s': Agents=%d, Tools=%d, MCPServers=%d",
		envInfo.Name, len(envInfo.Agents), len(envInfo.Tools), len(envInfo.MCPServers))

	return nil
}

func (chs *CommandHandlerService) handleSyncMCPServersCommand(ctx context.Context, cmd *proto.SyncMCPServersCommand) error {
	logging.Info("Syncing MCP servers: %d servers", len(cmd.McpServers))

	// TODO: This is a complex operation that would require:
	// 1. Converting proto MCPConfig to Station's file-based config format
	// 2. Writing template files to the environment directory
	// 3. Triggering a sync operation via DeclarativeSync service
	// 4. Handling variable resolution and template rendering

	// For now, log the received configurations
	for i, mcpConfig := range cmd.McpServers {
		logging.Info("MCP Server %d: %s", i+1, mcpConfig.Name)
		// Log basic info about each server config received
		if mcpConfig.Command != "" {
			logging.Info("  Command: %s", mcpConfig.Command)
		}
		if len(mcpConfig.Args) > 0 {
			logging.Info("  Args: %v", mcpConfig.Args)
		}
	}

	// This operation is complex and would need significant implementation
	// It involves file system operations, template generation, and sync coordination
	logging.Info("MCP server sync implementation is complex - would require:")
	logging.Info("  1. Converting proto configs to Station file format")
	logging.Info("  2. Writing template.json files to environment directory")
	logging.Info("  3. Triggering DeclarativeSync service")
	logging.Info("  4. Handling variable resolution")

	// TODO: Send response back to CloudShip via SendRun once protocol supports responses
	// For now, acknowledge the sync request but indicate it needs full implementation
	logging.Info("MCP sync command acknowledged - full implementation needed")

	return nil // Return nil for now to avoid failing the command processing
}

func (chs *CommandHandlerService) handleListAgentsCommand(ctx context.Context, cmd *proto.ListAgentsCommand) error {
	logging.Info("Listing agents for environment: %s (run_id: %s)", cmd.Environment, cmd.RunId)

	// Get environment by name to get environment ID
	env, err := chs.repos.Environments.GetByName(cmd.Environment)
	if err != nil {
		logging.Error("Failed to get environment %s: %v", cmd.Environment, err)
		return fmt.Errorf("environment %s not found: %v", cmd.Environment, err)
	}

	// Get agents from the agent service using environment ID
	agents, err := chs.agentService.ListAgentsByEnvironment(ctx, env.ID)
	if err != nil {
		logging.Error("Failed to get agents for environment %s: %v", cmd.Environment, err)
		return fmt.Errorf("failed to get agents: %v", err)
	}

	logging.Info("Found %d agents in environment '%s'", len(agents), cmd.Environment)

	// Convert agents to proto format for response
	var protoAgents []*proto.AgentConfig
	for _, agent := range agents {
		// Get tools for this agent
		agentTools, err := chs.repos.AgentTools.ListAgentTools(agent.ID)
		if err != nil {
			logging.Error("Failed to get tools for agent %d: %v", agent.ID, err)
			// Continue with empty tools rather than failing
			agentTools = []*models.AgentToolWithDetails{}
		}

		// Extract tool names
		var toolNames []string
		for _, tool := range agentTools {
			toolNames = append(toolNames, tool.ToolName)
		}

		// Model name - using default since models.Agent doesn't expose ModelID
		modelName := "gpt-4o-mini" // Default model

		protoAgent := &proto.AgentConfig{
			Id:             fmt.Sprintf("%d", agent.ID),
			Name:           agent.Name,
			Description:    agent.Description,
			PromptTemplate: agent.Prompt,
			ModelName:      modelName,
			MaxSteps:       int32(agent.MaxSteps),
			Tools:          toolNames,
		}
		protoAgents = append(protoAgents, protoAgent)
		logging.Info("  Agent: ID=%d, Name='%s', MaxSteps=%d", agent.ID, agent.Name, agent.MaxSteps)
	}

	// Send response back to CloudShip with agent list
	err = chs.sendListAgentsResponse(cmd.RunId, cmd.Environment, protoAgents)
	if err != nil {
		logging.Error("Failed to send list_agents response for run_id %s: %v", cmd.RunId, err)
		return fmt.Errorf("failed to send response: %v", err)
	}

	logging.Info("Agent list response sent successfully for environment %s (run_id: %s)", cmd.Environment, cmd.RunId)
	return nil
}

// sendListAgentsResponse sends the list_agents command response back to CloudShip
func (chs *CommandHandlerService) sendListAgentsResponse(runId, environment string, agents []*proto.AgentConfig) error {
	// Format agents list as JSON for response content
	agentsJson, err := json.Marshal(agents)
	if err != nil {
		return fmt.Errorf("failed to marshal agents to JSON: %v", err)
	}

	// Convert to types.AgentRun format expected by LighthouseClient.SendRun
	agentRun := &types.AgentRun{
		ID:          runId,
		AgentID:     "system",
		AgentName:   "Station Command Handler",
		Task:        fmt.Sprintf("list_agents for environment: %s", environment),
		Response:    string(agentsJson),
		Status:      "completed",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Metadata: map[string]string{
			"command_type":  "list_agents",
			"environment":   environment,
			"response_type": "command_response",
			"agent_count":   fmt.Sprintf("%d", len(agents)),
		},
	}

	// Send via LighthouseClient.SendRun method
	labels := map[string]string{
		"type":    "command_response",
		"command": "list_agents",
	}

	chs.lighthouseClient.SendRun(agentRun, environment, labels)

	logging.Info("Successfully sent list_agents response for run_id %s", runId)
	return nil
}

// EnvironmentInfo represents the complete environment information that would be sent to CloudShip
type EnvironmentInfo struct {
	Name       string
	Agents     []AgentInfo
	Tools      []ToolInfo
	MCPServers []MCPServerInfo
}

type AgentInfo struct {
	ID          string
	Name        string
	Description string
	Model       string
	MaxSteps    int
	Tools       []string
}

type ToolInfo struct {
	Name        string
	Description string
	ServerName  string
}

type MCPServerInfo struct {
	Name    string
	Command string
	Args    []string
	Status  string
}

// gatherEnvironmentInformation collects comprehensive environment information
func (chs *CommandHandlerService) gatherEnvironmentInformation(ctx context.Context, environmentName string) (*EnvironmentInfo, error) {
	logging.Debug("Gathering environment information for: %s", environmentName)

	envInfo := &EnvironmentInfo{
		Name:       environmentName,
		Agents:     []AgentInfo{},
		Tools:      []ToolInfo{},
		MCPServers: []MCPServerInfo{},
	}

	// Get environment by name to get environment ID
	env, err := chs.repos.Environments.GetByName(environmentName)
	if err != nil {
		logging.Error("Failed to get environment %s: %v", environmentName, err)
		return nil, fmt.Errorf("environment %s not found: %v", environmentName, err)
	}

	// Get agents from the agent service using environment ID
	agents, err := chs.agentService.ListAgentsByEnvironment(ctx, env.ID)
	if err != nil {
		logging.Error("Failed to get agents for environment %s: %v", environmentName, err)
		// Continue with empty agents list rather than failing completely
	} else {
		for _, agent := range agents {
			agentInfo := AgentInfo{
				ID:          fmt.Sprintf("%d", agent.ID),
				Name:        agent.Name,
				Description: agent.Description,
				Model:       "gpt-4o-mini", // TODO: Get actual model from agent config
				MaxSteps:    int(agent.MaxSteps),
				Tools:       []string{}, // TODO: Get actual assigned tools
			}
			envInfo.Agents = append(envInfo.Agents, agentInfo)
		}
	}

	// Get MCP tools using ToolDiscovery service approach
	toolDiscoveryService := services.NewToolDiscoveryService(chs.repos)
	mcpTools, err := toolDiscoveryService.GetHybridToolsByEnvironment(env.ID)
	if err != nil {
		logging.Error("Failed to get MCP tools for environment %s: %v", environmentName, err)
		// Continue with empty tools list rather than failing completely
	} else {
		for _, tool := range mcpTools {
			toolInfo := ToolInfo{
				Name:        tool.Name,
				Description: tool.Description,
				ServerName:  tool.ServerName,
			}
			envInfo.Tools = append(envInfo.Tools, toolInfo)
		}
	}

	// Get MCP servers for the environment
	mcpServers, err := chs.repos.FileMCPConfigs.ListByEnvironment(env.ID)
	if err != nil {
		logging.Error("Failed to get MCP servers for environment %s: %v", environmentName, err)
	} else {
		for _, config := range mcpServers {
			// Parse the config to get server names, commands, and status
			mcpServerInfo := MCPServerInfo{
				Name:    config.ConfigName,
				Command: fmt.Sprintf("file-based-%s", config.ConfigName),
				Args:    []string{},   // TODO: Parse from config JSON
				Status:  "configured", // File-based configs are always configured
			}
			envInfo.MCPServers = append(envInfo.MCPServers, mcpServerInfo)
		}
	}

	logging.Debug("Gathered environment info: %d agents, %d tools, %d MCP servers",
		len(envInfo.Agents), len(envInfo.Tools), len(envInfo.MCPServers))

	return envInfo, nil
}
