package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"station/internal/db/repositories"
	"station/internal/lighthouse"
	"station/internal/lighthouse/proto"
	"station/internal/logging"
	"station/internal/services"
	"station/pkg/models"
	"station/pkg/types"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// extractInt64FromTokenUsage safely extracts int64 from various numeric types in token usage
// (Same helper function as MCP handlers use)
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

// ManagementHandlerService handles management commands from CloudShip via ManagementChannel
type ManagementHandlerService struct {
	agentService      services.AgentServiceInterface
	repos             *repositories.Repositories
	lighthouseClient  *lighthouse.LighthouseClient
	registrationKey   string
	managementChannel *ManagementChannelService
}

// NewManagementHandlerService creates a new management handler service
func NewManagementHandlerService(
	agentService services.AgentServiceInterface,
	repos *repositories.Repositories,
	lighthouseClient *lighthouse.LighthouseClient,
	registrationKey string,
) *ManagementHandlerService {
	return &ManagementHandlerService{
		agentService:     agentService,
		repos:            repos,
		lighthouseClient: lighthouseClient,
		registrationKey:  registrationKey,
	}
}

// NewManagementHandlerServiceWithChannel creates a new management handler service with ManagementChannel reference for SendRun
func NewManagementHandlerServiceWithChannel(
	agentService services.AgentServiceInterface,
	repos *repositories.Repositories,
	lighthouseClient *lighthouse.LighthouseClient,
	registrationKey string,
	managementChannel *ManagementChannelService,
) *ManagementHandlerService {
	return &ManagementHandlerService{
		agentService:      agentService,
		repos:             repos,
		lighthouseClient:  lighthouseClient,
		registrationKey:   registrationKey,
		managementChannel: managementChannel,
	}
}

// ProcessManagementRequest processes incoming management requests and returns responses
func (mhs *ManagementHandlerService) ProcessManagementRequest(ctx context.Context, req *proto.ManagementMessage) (*proto.ManagementMessage, error) {
	if req.IsResponse {
		// This is a response, not a request - shouldn't be processed here
		return nil, fmt.Errorf("received response message in request processor")
	}

	logging.Debug("Processing management request: request_id=%s, station_id=%s", req.RequestId, req.RegistrationKey)

	// Create response message base
	resp := &proto.ManagementMessage{
		RequestId:       req.RequestId,
		RegistrationKey: req.RegistrationKey,
		IsResponse:      true,
		Success:         true,
	}

	// Process specific request type
	switch request := req.Message.(type) {
	case *proto.ManagementMessage_ListAgentsRequest:
		response, err := mhs.handleListAgents(ctx, request.ListAgentsRequest)
		if err != nil {
			logging.Error("Failed to list agents: %v", err)
			resp.Success = false
			resp.Message = &proto.ManagementMessage_Error{
				Error: &proto.ManagementError{
					Code:    proto.ErrorCode_UNKNOWN_ERROR,
					Message: err.Error(),
				},
			}
		} else {
			resp.Message = &proto.ManagementMessage_ListAgentsResponse{
				ListAgentsResponse: response,
			}
		}

	case *proto.ManagementMessage_ListToolsRequest:
		response, err := mhs.handleListTools(ctx, request.ListToolsRequest)
		if err != nil {
			logging.Error("Failed to list tools: %v", err)
			resp.Success = false
			resp.Message = &proto.ManagementMessage_Error{
				Error: &proto.ManagementError{
					Code:    proto.ErrorCode_UNKNOWN_ERROR,
					Message: err.Error(),
				},
			}
		} else {
			resp.Message = &proto.ManagementMessage_ListToolsResponse{
				ListToolsResponse: response,
			}
		}

	case *proto.ManagementMessage_GetEnvironmentsRequest:
		response, err := mhs.handleGetEnvironments(ctx, request.GetEnvironmentsRequest)
		if err != nil {
			logging.Error("Failed to get environments: %v", err)
			resp.Success = false
			resp.Message = &proto.ManagementMessage_Error{
				Error: &proto.ManagementError{
					Code:    proto.ErrorCode_UNKNOWN_ERROR,
					Message: err.Error(),
				},
			}
		} else {
			resp.Message = &proto.ManagementMessage_GetEnvironmentsResponse{
				GetEnvironmentsResponse: response,
			}
		}

	case *proto.ManagementMessage_ExecuteAgentRequest:
		response, err := mhs.handleExecuteAgent(ctx, req.RequestId, request.ExecuteAgentRequest)
		if err != nil {
			logging.Error("Failed to execute agent: %v", err)
			resp.Success = false
			resp.Message = &proto.ManagementMessage_Error{
				Error: &proto.ManagementError{
					Code:    proto.ErrorCode_EXECUTION_ERROR,
					Message: err.Error(),
				},
			}
		} else {
			resp.Message = &proto.ManagementMessage_ExecuteAgentResponse{
				ExecuteAgentResponse: response,
			}
		}

	case *proto.ManagementMessage_GetAgentDetailsRequest:
		response, err := mhs.handleGetAgentDetails(ctx, request.GetAgentDetailsRequest)
		if err != nil {
			logging.Error("Failed to get agent details: %v", err)
			resp.Success = false
			resp.Message = &proto.ManagementMessage_Error{
				Error: &proto.ManagementError{
					Code:    proto.ErrorCode_UNKNOWN_ERROR,
					Message: err.Error(),
				},
			}
		} else {
			resp.Message = &proto.ManagementMessage_GetAgentDetailsResponse{
				GetAgentDetailsResponse: response,
			}
		}

	case *proto.ManagementMessage_UpdateAgentPromptRequest:
		response, err := mhs.handleUpdateAgentPrompt(ctx, request.UpdateAgentPromptRequest)
		if err != nil {
			logging.Error("Failed to update agent prompt: %v", err)
			resp.Success = false
			resp.Message = &proto.ManagementMessage_Error{
				Error: &proto.ManagementError{
					Code:    proto.ErrorCode_UNKNOWN_ERROR,
					Message: err.Error(),
				},
			}
		} else {
			resp.Message = &proto.ManagementMessage_UpdateAgentPromptResponse{
				UpdateAgentPromptResponse: response,
			}
		}

	default:
		logging.Error("Unknown management request type: %T", request)
		resp.Success = false
		resp.Message = &proto.ManagementMessage_Error{
			Error: &proto.ManagementError{
				Code:    proto.ErrorCode_UNKNOWN_ERROR,
				Message: "Unknown request type",
			},
		}
	}

	logging.Debug("Management request processed: request_id=%s, success=%v", req.RequestId, resp.Success)
	return resp, nil
}

// handleListAgents processes list agents requests
func (mhs *ManagementHandlerService) handleListAgents(ctx context.Context, req *proto.ListAgentsManagementRequest) (*proto.ListAgentsManagementResponse, error) {
	logging.Info("Listing agents for environment: %s", req.Environment)

	// Default to "default" environment if not specified
	environmentName := req.Environment
	if environmentName == "" {
		environmentName = "default"
	}

	// Get environment by name to get environment ID
	env, err := mhs.repos.Environments.GetByName(environmentName)
	if err != nil {
		return nil, fmt.Errorf("environment %s not found: %v", environmentName, err)
	}

	// Get agents from the agent service using environment ID
	agents, err := mhs.agentService.ListAgentsByEnvironment(ctx, env.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agents: %v", err)
	}

	logging.Info("Found %d agents in environment '%s'", len(agents), environmentName)

	// Convert agents to proto format for response
	var protoAgents []*proto.AgentInfo
	for _, agent := range agents {
		// Get tools for this agent
		agentTools, err := mhs.repos.AgentTools.ListAgentTools(agent.ID)
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

		// Convert timestamps
		var createdAt, updatedAt *timestamppb.Timestamp
		if !agent.CreatedAt.IsZero() {
			createdAt = timestamppb.New(agent.CreatedAt)
		}
		if !agent.UpdatedAt.IsZero() {
			updatedAt = timestamppb.New(agent.UpdatedAt)
		}

		// Handle cron schedule
		cronSchedule := ""
		scheduleEnabled := false
		if agent.CronSchedule != nil {
			cronSchedule = *agent.CronSchedule
			scheduleEnabled = agent.ScheduleEnabled
		}

		// Get recent run statistics for this agent to determine status
		recentRuns, err := mhs.repos.AgentRuns.ListByAgent(ctx, agent.ID)
		agentStatus := proto.AgentStatus_AGENT_STATUS_ACTIVE

		if err == nil && len(recentRuns) > 0 {
			// Check recent runs to determine current status
			hasRunning := false
			hasRecentFailures := false
			recentRunCount := 0

			for _, run := range recentRuns {
				// Only consider recent runs (limit to last 10)
				if recentRunCount >= 10 {
					break
				}
				recentRunCount++

				switch run.Status {
				case "running":
					hasRunning = true
				case "failed", "timeout", "cancelled":
					hasRecentFailures = true
				}
			}

			// Determine agent status based on recent activity
			if hasRunning {
				agentStatus = proto.AgentStatus_AGENT_STATUS_RUNNING
			} else if hasRecentFailures {
				agentStatus = proto.AgentStatus_AGENT_STATUS_ERROR
			}
		}

		protoAgent := &proto.AgentInfo{
			Id:              fmt.Sprintf("%d", agent.ID),
			Name:            agent.Name,
			Description:     agent.Description,
			Prompt:          agent.Prompt,
			ModelName:       "gpt-4o-mini", // Default model - could be enhanced with dotprompt model info
			MaxSteps:        int32(agent.MaxSteps),
			AssignedTools:   toolNames,
			EnvironmentName: environmentName,
			ScheduleEnabled: scheduleEnabled,
			CronSchedule:    cronSchedule,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
			Status:          agentStatus,
		}
		protoAgents = append(protoAgents, protoAgent)
	}

	return &proto.ListAgentsManagementResponse{
		Agents:      protoAgents,
		TotalAgents: int32(len(protoAgents)),
	}, nil
}

// handleListTools processes list tools requests
func (mhs *ManagementHandlerService) handleListTools(ctx context.Context, req *proto.ListToolsManagementRequest) (*proto.ListToolsManagementResponse, error) {
	logging.Info("Listing tools for environment: %s", req.Environment)

	environmentName := req.Environment
	if environmentName == "" {
		environmentName = "default"
	}

	// Get environment by name to get environment ID
	env, err := mhs.repos.Environments.GetByName(environmentName)
	if err != nil {
		return nil, fmt.Errorf("environment %s not found: %v", environmentName, err)
	}

	// Get file-based MCP configs for this environment
	fileConfigs, err := mhs.repos.FileMCPConfigs.ListByEnvironment(env.ID)
	if err != nil {
		logging.Error("Failed to get MCP configs for environment %s: %v", environmentName, err)
		return &proto.ListToolsManagementResponse{
			Tools:           []*proto.ToolInfo{},
			EnvironmentName: environmentName,
			TotalTools:      0,
			McpServers:      []string{},
		}, nil
	}

	// Get MCP tools with file config info for this environment
	mcpTools, err := mhs.repos.MCPTools.GetToolsWithFileConfigInfo(env.ID)
	if err != nil {
		logging.Error("Failed to get MCP tools for environment %s: %v", environmentName, err)
		mcpTools = []*models.MCPToolWithFileConfig{} // Continue with empty list
	}

	// Convert tools to proto format
	var protoTools []*proto.ToolInfo
	mcpServerNames := make(map[string]bool)

	for _, tool := range mcpTools {
		protoTool := &proto.ToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
			McpServer:   tool.ServerName, // Server name from the joined query
			Categories:  []string{},      // TODO: Add categories support
		}
		protoTools = append(protoTools, protoTool)
		mcpServerNames[tool.ServerName] = true
	}

	// Add MCP server names from file configs even if no tools discovered yet
	for _, fileConfig := range fileConfigs {
		mcpServerNames[fileConfig.ConfigName] = true
	}

	// Extract unique MCP server names
	var servers []string
	for serverName := range mcpServerNames {
		servers = append(servers, serverName)
	}

	logging.Info("Found %d tools from %d MCP servers in environment '%s'", len(protoTools), len(servers), environmentName)

	return &proto.ListToolsManagementResponse{
		Tools:           protoTools,
		EnvironmentName: environmentName,
		TotalTools:      int32(len(protoTools)),
		McpServers:      servers,
	}, nil
}

// handleGetEnvironments processes get environments requests
func (mhs *ManagementHandlerService) handleGetEnvironments(ctx context.Context, req *proto.GetEnvironmentsRequest) (*proto.GetEnvironmentsResponse, error) {
	logging.Info("Getting all environments")

	environments, err := mhs.repos.Environments.List()
	if err != nil {
		return nil, fmt.Errorf("failed to get environments: %v", err)
	}

	var protoEnvs []*proto.EnvironmentInfo
	for _, env := range environments {
		// Get agent count for this environment
		agents, err := mhs.agentService.ListAgentsByEnvironment(ctx, env.ID)
		agentCount := 0
		if err == nil {
			agentCount = len(agents)
		}

		protoEnv := &proto.EnvironmentInfo{
			Name:       env.Name,
			AgentCount: int32(agentCount),
			ToolCount:  0,                   // TODO: Get actual tool count
			McpServers: []string{},          // TODO: Get actual MCP servers
			Variables:  map[string]string{}, // TODO: Get actual variables
			IsDefault:  env.Name == "default",
		}
		protoEnvs = append(protoEnvs, protoEnv)
	}

	return &proto.GetEnvironmentsResponse{
		Environments: protoEnvs,
	}, nil
}


// handleExecuteAgent processes execute agent requests using unified execution flow (same as MCP/CLI)
func (mhs *ManagementHandlerService) handleExecuteAgent(ctx context.Context, originalRequestId string, req *proto.ExecuteAgentManagementRequest) (*proto.ExecuteAgentManagementResponse, error) {
	logging.Info("Executing agent %s with task: %s", req.AgentId, req.Task)
	logging.Debug("DEBUG: CloudShip payload - request_id: %s, run_id: '%s', agent_id: %s", originalRequestId, req.RunId, req.AgentId)

	// Convert agent ID to int64
	agentID, err := strconv.ParseInt(req.AgentId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID: %s", req.AgentId)
	}

	// For management commands, CloudShip provides the run_id that we must use for all tracking
	// This is the primary identifier - NOT our internal database ID
	var cloudShipRunID string
	if req.RunId != "" {
		cloudShipRunID = req.RunId
	} else {
		// If CloudShip doesn't provide run_id, use the request_id as fallback
		cloudShipRunID = originalRequestId
	}

	// Use CloudShip's run_id as the execution ID for all status updates and tracking
	executionID := cloudShipRunID

	// Send QUEUED status update to CloudShip using their run_id
	mhs.sendStatusUpdate(originalRequestId, executionID, proto.ExecutionStatus_EXECUTION_QUEUED, "Agent execution queued", 0)

	// Send RUNNING status update to CloudShip using their run_id
	mhs.sendStatusUpdate(originalRequestId, executionID, proto.ExecutionStatus_EXECUTION_RUNNING, "Agent execution started", 1)

	var userID int64 = 1 // Default user ID for CloudShip executions
	
	// Create local agent run (we'll correlate with CloudShip's ID in SendRun)
	run, err := mhs.repos.AgentRuns.Create(ctx, agentID, userID, req.Task, "", 0, nil, nil, "running", nil)
	if err != nil {
		mhs.sendStatusUpdate(originalRequestId, executionID, proto.ExecutionStatus_EXECUTION_FAILED, fmt.Sprintf("Failed to create agent run: %v", err), 1)
		return nil, fmt.Errorf("failed to create agent run: %v", err)
	}
	runID := run.ID
	logging.Info("Using CloudShip's run ID: %s (local database ID: %d) for execution tracking", cloudShipRunID, runID)
	
	// Get agent details for unified execution flow
	agent, err := mhs.repos.Agents.GetByID(agentID)
	if err != nil {
		mhs.sendStatusUpdate(originalRequestId, executionID, proto.ExecutionStatus_EXECUTION_FAILED, fmt.Sprintf("Agent not found: %v", err), 1)
		return nil, fmt.Errorf("agent not found: %v", err)
	}
	
	// Create agent service to access execution engine (same as MCP)
	agentService := services.NewAgentService(mhs.repos)
	
	// Use the same unified execution flow as MCP and CLI with empty variables
	userVariables := make(map[string]interface{})
	result, execErr := agentService.GetExecutionEngine().Execute(ctx, agent, req.Task, runID, userVariables)
	
	if execErr != nil {
		// Update run as failed (same as MCP)
		completedAt := time.Now()
		errorMsg := fmt.Sprintf("CloudShip execution failed: %v", execErr)
		updateErr := mhs.repos.AgentRuns.UpdateCompletionWithMetadata(
			ctx, runID, errorMsg, 0, nil, nil, "failed", &completedAt,
			nil, nil, nil, nil, nil, nil,
		)
		if updateErr != nil {
			logging.Info("Warning: Failed to update failed run %d: %v", runID, updateErr)
		}
		
		// Send FAILED status update on error
		mhs.sendStatusUpdate(originalRequestId, executionID, proto.ExecutionStatus_EXECUTION_FAILED, fmt.Sprintf("Agent execution failed: %v", execErr), 1)
		return nil, fmt.Errorf("failed to execute agent: %v", execErr)
	}

	// Update run as successful (same as CLI and MCP)
	completedAt := time.Now()
	
	// Extract token usage from result (using same field names as MCP handlers)
	var inputTokens, outputTokens, totalTokens *int64
	if result != nil && result.TokenUsage != nil {
		inputTokens = extractInt64FromTokenUsage(result.TokenUsage["input_tokens"])
		outputTokens = extractInt64FromTokenUsage(result.TokenUsage["output_tokens"])
		totalTokens = extractInt64FromTokenUsage(result.TokenUsage["total_tokens"])
	}
	
	// Calculate duration in seconds
	var durationSeconds *float64
	if result != nil {
		dur := result.Duration.Seconds()
		durationSeconds = &dur
	}
	
	// Update completion with metadata (same as CLI)
	updateErr := mhs.repos.AgentRuns.UpdateCompletionWithMetadata(
		ctx, runID, result.Response, result.StepsTaken, result.ToolCalls, result.ExecutionSteps, 
		"completed", &completedAt, inputTokens, outputTokens, totalTokens, durationSeconds, 
		&result.ModelName, nil,
	)
	if updateErr != nil {
		logging.Info("Warning: Failed to update completed run %d: %v", runID, updateErr)
	}

	// Send COMPLETED status update on success
	mhs.sendStatusUpdate(originalRequestId, executionID, proto.ExecutionStatus_EXECUTION_COMPLETED, "Agent execution completed successfully", 1)

	// Send completed run data to CloudShip via SendRun
	runDetails, err := mhs.repos.AgentRuns.GetByIDWithDetails(ctx, runID)
	if err != nil {
		logging.Info("Warning: Failed to get run details for CloudShip SendRun (run_id: %d): %v", runID, err)
	} else {
		// Convert database run to proto.AgentRun for ManagementChannel
		protoAgentRun := mhs.convertRunDetailsToProtoAgentRun(runDetails)
		if protoAgentRun != nil {
			// CRITICAL: For management commands, override the RunId with CloudShip's original run_id
			// This ensures CloudShip can correlate the telemetry data properly
			protoAgentRun.RunId = cloudShipRunID

			// Send run data to CloudShip via ManagementChannel using CloudShip's run_id as primary identifier
			if mhs.managementChannel != nil {
				tags := map[string]string{
					"cloudship_run_id": cloudShipRunID,
					"execution_id":     executionID,
					"source":           "cloudship_management",
					"environment":      "cloudship",
					"local_run_id":     fmt.Sprintf("%d", runID),
				}
				if err := mhs.managementChannel.SendRun(protoAgentRun, tags); err != nil {
					logging.Error("Failed to send run data via ManagementChannel: %v", err)
				} else {
					logging.Debug("Sent completed run data to CloudShip via ManagementChannel (run_id: %d, execution_id: %s)", runID, executionID)
				}
			} else {
				logging.Error("ManagementChannel not available for SendRun - falling back to direct lighthouse client")
				// Convert to types.AgentRun for fallback
				typesAgentRun := mhs.convertRunDetailsToAgentRun(runDetails)
				if typesAgentRun != nil {
					// CRITICAL: For management commands, override the ID with CloudShip's original run_id
					typesAgentRun.ID = cloudShipRunID
					mhs.lighthouseClient.SendRun(typesAgentRun, "cloudship", map[string]string{
						"cloudship_run_id": cloudShipRunID,
						"execution_id":     executionID,
						"source":           "cloudship_management",
						"environment":      "cloudship",
						"local_run_id":     fmt.Sprintf("%d", runID),
					})
					logging.Debug("Sent completed run data to CloudShip via lighthouse client fallback (run_id: %d, execution_id: %s)", runID, executionID)
				}
			}
		}
	}

	return &proto.ExecuteAgentManagementResponse{
		ExecutionId:     executionID,
		Status:          proto.ExecutionStatus_EXECUTION_COMPLETED,
		StepNumber:      1,
		StepDescription: result.Response,
		ToolCalls:       []*proto.LighthouseToolCall{},
		Timestamp:       timestamppb.Now(),
	}, nil
}


// handleGetAgentDetails processes get agent details requests
func (mhs *ManagementHandlerService) handleGetAgentDetails(ctx context.Context, req *proto.GetAgentDetailsRequest) (*proto.GetAgentDetailsResponse, error) {
	logging.Info("Getting agent details: agent_id=%s, environment=%s", req.AgentId, req.Environment)

	// Convert agent ID to int64
	agentID, err := strconv.ParseInt(req.AgentId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID: %s", req.AgentId)
	}

	// Default to "default" environment if not specified
	environmentName := req.Environment
	if environmentName == "" {
		environmentName = "default"
	}

	// Get environment by name
	env, err := mhs.repos.Environments.GetByName(environmentName)
	if err != nil {
		return nil, fmt.Errorf("environment %s not found: %v", environmentName, err)
	}

	// Get agent by ID and verify it belongs to the correct environment
	agent, err := mhs.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %v", err)
	}

	if agent.EnvironmentID != env.ID {
		return nil, fmt.Errorf("agent %s does not exist in environment %s", req.AgentId, environmentName)
	}

	// Get tools for this agent
	agentTools, err := mhs.repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		logging.Error("Failed to get tools for agent %d: %v", agent.ID, err)
		agentTools = []*models.AgentToolWithDetails{} // Continue with empty tools
	}

	// Extract tool names
	var toolNames []string
	for _, tool := range agentTools {
		toolNames = append(toolNames, tool.ToolName)
	}

	// Generate dotprompt content (similar to export_handlers.go)
	dotpromptContent := mhs.generateDotpromptContent(agent, agentTools, environmentName)

	// Convert timestamps
	var createdAt, updatedAt *timestamppb.Timestamp
	if !agent.CreatedAt.IsZero() {
		createdAt = timestamppb.New(agent.CreatedAt)
	}
	if !agent.UpdatedAt.IsZero() {
		updatedAt = timestamppb.New(agent.UpdatedAt)
	}

	// Create AgentConfig proto message
	agentConfig := &proto.AgentConfig{
		Id:             fmt.Sprintf("%d", agent.ID),
		Name:           agent.Name,
		Description:    agent.Description,
		PromptTemplate: dotpromptContent,
		ModelName:      "gemini-2.5-flash", // Default model, could be enhanced
		MaxSteps:       int32(agent.MaxSteps),
		Tools:          toolNames,
		Variables:      map[string]string{}, // TODO: Add actual variables if needed
		Tags:           []string{},          // TODO: Add tags if available
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}

	return &proto.GetAgentDetailsResponse{
		Agent: agentConfig,
	}, nil
}

// generateDotpromptContent generates the complete .prompt file content for an agent
// (Similar to the method in export_handlers.go but adapted for management service)
func (mhs *ManagementHandlerService) generateDotpromptContent(agent *models.Agent, tools []*models.AgentToolWithDetails, environment string) string {
	var content strings.Builder

	// Default model name
	modelName := "gemini-2.5-flash"

	// YAML frontmatter
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("model: \"%s\"\n", modelName))

	// Input schema (basic default)
	content.WriteString("input:\n")
	content.WriteString("  userInput:\n")
	content.WriteString("    type: string\n")
	content.WriteString("    description: \"The user's input or task description\"\n")

	// Output schema if available
	if agent.OutputSchema != nil && *agent.OutputSchema != "" {
		content.WriteString("output:\n")
		content.WriteString("  schema: |\n")
		content.WriteString("    " + *agent.OutputSchema + "\n")
	}

	// Max steps
	if agent.MaxSteps > 0 {
		content.WriteString(fmt.Sprintf("max_steps: %d\n", agent.MaxSteps))
	}

	// Tools list
	if len(tools) > 0 {
		content.WriteString("tools:\n")
		for _, tool := range tools {
			content.WriteString(fmt.Sprintf("  - \"%s\"\n", tool.ToolName))
		}
	}

	content.WriteString("---\n\n")

	// Multi-role prompt content
	content.WriteString("{{role \"system\"}}\n")
	content.WriteString(agent.Prompt)
	content.WriteString("\n\n")

	// User role with variable substitution
	content.WriteString("{{role \"user\"}}\n")
	content.WriteString("{{userInput}}")

	return content.String()
}

// handleUpdateAgentPrompt processes update agent prompt requests
func (mhs *ManagementHandlerService) handleUpdateAgentPrompt(ctx context.Context, req *proto.UpdateAgentPromptRequest) (*proto.UpdateAgentPromptResponse, error) {
	logging.Info("Updating agent prompt: agent_id=%s, environment=%s", req.AgentId, req.Environment)

	// Convert agent ID to int64
	agentID, err := strconv.ParseInt(req.AgentId, 10, 64)
	if err != nil {
		return &proto.UpdateAgentPromptResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid agent ID: %s", req.AgentId),
		}, nil
	}

	// Default to "default" environment if not specified
	environmentName := req.Environment
	if environmentName == "" {
		environmentName = "default"
	}

	// Get environment by name
	env, err := mhs.repos.Environments.GetByName(environmentName)
	if err != nil {
		return &proto.UpdateAgentPromptResponse{
			Success: false,
			Message: fmt.Sprintf("Environment %s not found: %v", environmentName, err),
		}, nil
	}

	// Get agent by ID and verify it belongs to the correct environment
	agent, err := mhs.repos.Agents.GetByID(agentID)
	if err != nil {
		return &proto.UpdateAgentPromptResponse{
			Success: false,
			Message: fmt.Sprintf("Agent not found: %v", err),
		}, nil
	}

	if agent.EnvironmentID != env.ID {
		return &proto.UpdateAgentPromptResponse{
			Success: false,
			Message: fmt.Sprintf("Agent %s does not exist in environment %s", req.AgentId, environmentName),
		}, nil
	}

	// Parse and validate the dotprompt content
	// Basic YAML validation - check if it starts with --- and has valid structure
	promptContent := strings.TrimSpace(req.NewPrompt)
	if !strings.HasPrefix(promptContent, "---") {
		return &proto.UpdateAgentPromptResponse{
			Success: false,
			Message: "Invalid dotprompt format: must start with YAML frontmatter (---)",
		}, nil
	}

	// Find the end of YAML frontmatter
	lines := strings.Split(promptContent, "\n")
	frontmatterEndIndex := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			frontmatterEndIndex = i
			break
		}
	}

	if frontmatterEndIndex == -1 {
		return &proto.UpdateAgentPromptResponse{
			Success: false,
			Message: "Invalid dotprompt format: missing end of YAML frontmatter (---)",
		}, nil
	}

	// Extract system prompt content (everything after the frontmatter)
	var systemPrompt string
	if frontmatterEndIndex+1 < len(lines) {
		promptLines := lines[frontmatterEndIndex+1:]
		systemPrompt = strings.TrimSpace(strings.Join(promptLines, "\n"))

		// Remove role directives to get the clean system prompt
		systemPrompt = strings.ReplaceAll(systemPrompt, "{{role \"system\"}}", "")
		systemPrompt = strings.ReplaceAll(systemPrompt, "{{role \"user\"}}", "")
		systemPrompt = strings.ReplaceAll(systemPrompt, "{{userInput}}", "")
		systemPrompt = strings.TrimSpace(systemPrompt)
	}

	if systemPrompt == "" {
		return &proto.UpdateAgentPromptResponse{
			Success: false,
			Message: "Invalid dotprompt format: missing system prompt content",
		}, nil
	}

	// Update agent prompt in database
	err = mhs.repos.Agents.UpdatePrompt(agentID, systemPrompt)
	if err != nil {
		return &proto.UpdateAgentPromptResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update agent in database: %v", err),
		}, nil
	}

	// Get updated agent details
	updatedAgent, err := mhs.repos.Agents.GetByID(agentID)
	if err != nil {
		return &proto.UpdateAgentPromptResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to get updated agent: %v", err),
		}, nil
	}

	// Write the complete dotprompt file to filesystem (similar to export_handlers.go)
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		var homeErr error
		homeDir, homeErr = os.UserHomeDir()
		if homeErr != nil {
			return &proto.UpdateAgentPromptResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to get user home directory: %v", homeErr),
			}, nil
		}
	}

	// Construct the .prompt file path
	promptFilePath := fmt.Sprintf("%s/.config/station/environments/%s/agents/%s.prompt",
		homeDir, environmentName, agent.Name)

	// Ensure directory exists
	agentsDir := filepath.Dir(promptFilePath)
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return &proto.UpdateAgentPromptResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create agents directory: %v", err),
		}, nil
	}

	// Write the complete dotprompt content to file
	if err := os.WriteFile(promptFilePath, []byte(promptContent), 0644); err != nil {
		return &proto.UpdateAgentPromptResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to write dotprompt file: %v", err),
		}, nil
	}

	// Get updated agent details to return
	agentTools, err := mhs.repos.AgentTools.ListAgentTools(updatedAgent.ID)
	if err != nil {
		logging.Error("Failed to get tools for updated agent %d: %v", updatedAgent.ID, err)
		agentTools = []*models.AgentToolWithDetails{} // Continue with empty tools
	}

	// Extract tool names
	var toolNames []string
	for _, tool := range agentTools {
		toolNames = append(toolNames, tool.ToolName)
	}

	// Convert timestamps
	var createdAt, updatedAt *timestamppb.Timestamp
	if !updatedAgent.CreatedAt.IsZero() {
		createdAt = timestamppb.New(updatedAgent.CreatedAt)
	}
	if !updatedAgent.UpdatedAt.IsZero() {
		updatedAt = timestamppb.New(updatedAgent.UpdatedAt)
	}

	// Create updated AgentConfig proto message
	updatedAgentConfig := &proto.AgentConfig{
		Id:             fmt.Sprintf("%d", updatedAgent.ID),
		Name:           updatedAgent.Name,
		Description:    updatedAgent.Description,
		PromptTemplate: promptContent, // Return the complete dotprompt content
		ModelName:      "gemini-2.5-flash", // Default model
		MaxSteps:       int32(updatedAgent.MaxSteps),
		Tools:          toolNames,
		Variables:      map[string]string{},
		Tags:           []string{},
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}

	logging.Info("Successfully updated agent prompt: agent_id=%d, environment=%s, file=%s",
		agentID, environmentName, promptFilePath)

	return &proto.UpdateAgentPromptResponse{
		Success:      true,
		Message:      "Agent prompt updated successfully",
		UpdatedAgent: updatedAgentConfig,
	}, nil
}

// sendStatusUpdate sends a status update to CloudShip via ManagementChannel as response to original request
func (mhs *ManagementHandlerService) sendStatusUpdate(originalRequestId, executionID string, status proto.ExecutionStatus, description string, stepNumber int32) {
	if mhs.lighthouseClient == nil || !mhs.lighthouseClient.IsConnected() {
		logging.Debug("Cannot send status update - lighthouse client not connected")
		return
	}

	// Create status update message as ExecuteAgentManagementResponse with IsResponse: false
	statusMsg := &proto.ManagementMessage{
		RequestId:       originalRequestId,   // Use original request ID from ExecuteAgent request
		RegistrationKey: mhs.registrationKey, // Registration key is the source of truth for Station identity
		IsResponse:      true,                // CloudShip expects status updates as responses
		Success:         true,
		Message: &proto.ManagementMessage_ExecuteAgentResponse{
			ExecuteAgentResponse: &proto.ExecuteAgentManagementResponse{
				ExecutionId:     executionID,
				Status:          status,
				PartialResponse: description,
				StepNumber:      stepNumber,
				Timestamp:       timestamppb.Now(),
			},
		},
	}

	// Log the status update
	logging.Info("Status Update [%s]: %s - %s (step %d)", executionID, status.String(), description, stepNumber)
	logging.Debug("Status message prepared: %v", statusMsg)

	// FIXED: Actually send the status update via management channel
	if mhs.managementChannel != nil {
		if err := mhs.managementChannel.SendStatusUpdate(statusMsg); err != nil {
			logging.Error("Failed to send status update for execution %s: %v", executionID, err)
			return
		}
		logging.Debug("Successfully sent status update for execution %s: %s", executionID, status.String())
	} else {
		logging.Error("Cannot send status update - management channel not available")
	}
}

// convertRunDetailsToAgentRun converts database AgentRunWithDetails to types.AgentRun for CloudShip
func (mhs *ManagementHandlerService) convertRunDetailsToAgentRun(runDetails *models.AgentRunWithDetails) *types.AgentRun {
	if runDetails == nil {
		return nil
	}

	// Parse tool calls from JSON if available
	var toolCalls []types.ToolCall
	if runDetails.ToolCalls != nil {
		if toolCallsBytes, err := json.Marshal(*runDetails.ToolCalls); err == nil {
			var parsedToolCalls []types.ToolCall
			if err := json.Unmarshal(toolCallsBytes, &parsedToolCalls); err == nil {
				toolCalls = parsedToolCalls
			}
		}
	}

	// Parse execution steps from JSON if available  
	var executionSteps []types.ExecutionStep
	if runDetails.ExecutionSteps != nil {
		if stepsBytes, err := json.Marshal(*runDetails.ExecutionSteps); err == nil {
			var parsedSteps []types.ExecutionStep
			if err := json.Unmarshal(stepsBytes, &parsedSteps); err == nil {
				executionSteps = parsedSteps
			}
		}
	}

	// Calculate token usage if available
	var tokenUsage *types.TokenUsage
	if runDetails.InputTokens != nil && runDetails.OutputTokens != nil && runDetails.TotalTokens != nil {
		tokenUsage = &types.TokenUsage{
			PromptTokens:     int(*runDetails.InputTokens),
			CompletionTokens: int(*runDetails.OutputTokens),
			TotalTokens:      int(*runDetails.TotalTokens),
			CostUSD:          0.0, // Cost calculation would need to be added
		}
	}

	// Calculate duration in milliseconds
	var durationMs int64
	if runDetails.DurationSeconds != nil {
		durationMs = int64(*runDetails.DurationSeconds * 1000)
	}

	// Handle optional fields
	var completedAt time.Time
	if runDetails.CompletedAt != nil {
		completedAt = *runDetails.CompletedAt
	}

	startedAt := runDetails.StartedAt

	modelName := ""
	if runDetails.ModelName != nil {
		modelName = *runDetails.ModelName
	}

	return &types.AgentRun{
		ID:             fmt.Sprintf("run_%d", runDetails.ID),
		AgentID:        fmt.Sprintf("agent_%d", runDetails.AgentID),
		AgentName:      runDetails.AgentName,
		Task:           runDetails.Task,
		Response:       runDetails.FinalResponse,
		Status:         runDetails.Status,
		DurationMs:     durationMs,
		ModelName:      modelName,
		StartedAt:      startedAt,
		CompletedAt:    completedAt,
		ToolCalls:      toolCalls,
		ExecutionSteps: executionSteps,
		TokenUsage:     tokenUsage,
		Metadata: map[string]string{
			"run_id":      fmt.Sprintf("%d", runDetails.ID),
			"agent_id":    fmt.Sprintf("%d", runDetails.AgentID),
			"steps_taken": fmt.Sprintf("%d", runDetails.StepsTaken),
			"user_id":     fmt.Sprintf("%d", runDetails.UserID),
			"username":    runDetails.Username,
		},
	}
}

// convertRunDetailsToProtoAgentRun converts database AgentRunWithDetails to proto.AgentRunData for ManagementChannel SendRun
func (mhs *ManagementHandlerService) convertRunDetailsToProtoAgentRun(runDetails *models.AgentRunWithDetails) *proto.LighthouseAgentRunData {
	if runDetails == nil {
		return nil
	}

	// Calculate duration in milliseconds
	var durationMs int64
	if runDetails.DurationSeconds != nil {
		durationMs = int64(*runDetails.DurationSeconds * 1000)
	}

	// Create token usage data if available
	var tokenUsage *proto.TokenUsage
	if runDetails.InputTokens != nil || runDetails.OutputTokens != nil || runDetails.TotalTokens != nil {
		tokenUsage = &proto.TokenUsage{
			PromptTokens:     int32(0),
			CompletionTokens: int32(0),
			TotalTokens:      int32(0),
			CostUsd:          0.0,
		}

		if runDetails.InputTokens != nil {
			tokenUsage.PromptTokens = int32(*runDetails.InputTokens)
		}

		if runDetails.OutputTokens != nil {
			tokenUsage.CompletionTokens = int32(*runDetails.OutputTokens)
		}

		if runDetails.TotalTokens != nil {
			tokenUsage.TotalTokens = int32(*runDetails.TotalTokens)
		}
	}


	// Create proto LighthouseAgentRunData for management channel
	metadata := map[string]string{
		"run_id":      fmt.Sprintf("%d", runDetails.ID),
		"agent_id":    fmt.Sprintf("%d", runDetails.AgentID),
		"steps_taken": fmt.Sprintf("%d", runDetails.StepsTaken),
		"user_id":     fmt.Sprintf("%d", runDetails.UserID),
		"username":    runDetails.Username,
	}

	// Convert timestamps
	var completedAt *timestamppb.Timestamp
	if runDetails.CompletedAt != nil {
		completedAt = timestamppb.New(*runDetails.CompletedAt)
	}
	startedAt := timestamppb.New(runDetails.StartedAt)

	// Convert to lighthouse token usage format
	var lighthouseTokenUsage *proto.LighthouseTokenUsage
	if tokenUsage != nil {
		lighthouseTokenUsage = &proto.LighthouseTokenUsage{
			PromptTokens:     tokenUsage.PromptTokens,
			CompletionTokens: tokenUsage.CompletionTokens,
			TotalTokens:      tokenUsage.TotalTokens,
			CostUsd:          tokenUsage.CostUsd,
		}
	}

	// Convert tool calls to lighthouse format
	var lighthouseToolCalls []*proto.LighthouseToolCall
	// TODO: Convert toolCalls to lighthouse format if needed

	// Convert status to lighthouse format
	lighthouseStatus := proto.LighthouseRunStatus_LIGHTHOUSE_RUN_STATUS_UNSPECIFIED
	switch runDetails.Status {
	case "queued":
		lighthouseStatus = proto.LighthouseRunStatus_LIGHTHOUSE_RUN_STATUS_QUEUED
	case "running":
		lighthouseStatus = proto.LighthouseRunStatus_LIGHTHOUSE_RUN_STATUS_RUNNING
	case "completed":
		lighthouseStatus = proto.LighthouseRunStatus_LIGHTHOUSE_RUN_STATUS_COMPLETED
	case "failed":
		lighthouseStatus = proto.LighthouseRunStatus_LIGHTHOUSE_RUN_STATUS_FAILED
	case "cancelled":
		lighthouseStatus = proto.LighthouseRunStatus_LIGHTHOUSE_RUN_STATUS_CANCELLED
	}

	return &proto.LighthouseAgentRunData{
		RunId:          fmt.Sprintf("run_%d", runDetails.ID),
		AgentId:        fmt.Sprintf("agent_%d", runDetails.AgentID),
		AgentName:      runDetails.AgentName,
		Task:           runDetails.Task,
		Response:       runDetails.FinalResponse,
		ToolCalls:      lighthouseToolCalls,
		ExecutionSteps: []*proto.ExecutionStep{}, // TODO: Convert if needed
		TokenUsage:     lighthouseTokenUsage,
		DurationMs:     durationMs,
		ModelName:      *runDetails.ModelName,
		Status:         lighthouseStatus,
		StartedAt:      startedAt,
		CompletedAt:    completedAt,
		Metadata:       metadata,
		StationVersion: "v0.11.0",
	}
}
