package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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

	case *proto.ManagementMessage_GetSystemStatusRequest:
		response, err := mhs.handleGetSystemStatus(ctx, request.GetSystemStatusRequest)
		if err != nil {
			logging.Error("Failed to get system status: %v", err)
			resp.Success = false
			resp.Message = &proto.ManagementMessage_Error{
				Error: &proto.ManagementError{
					Code:    proto.ErrorCode_UNKNOWN_ERROR,
					Message: err.Error(),
				},
			}
		} else {
			resp.Message = &proto.ManagementMessage_GetSystemStatusResponse{
				GetSystemStatusResponse: response,
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

	case *proto.ManagementMessage_ListActiveRunsRequest:
		response, err := mhs.handleListActiveRuns(ctx, request.ListActiveRunsRequest)
		if err != nil {
			logging.Error("Failed to list active runs: %v", err)
			resp.Success = false
			resp.Message = &proto.ManagementMessage_Error{
				Error: &proto.ManagementError{
					Code:    proto.ErrorCode_UNKNOWN_ERROR,
					Message: err.Error(),
				},
			}
		} else {
			resp.Message = &proto.ManagementMessage_ListActiveRunsResponse{
				ListActiveRunsResponse: response,
			}
		}

	case *proto.ManagementMessage_CancelExecutionRequest:
		response, err := mhs.handleCancelExecution(ctx, request.CancelExecutionRequest)
		if err != nil {
			logging.Error("Failed to cancel execution: %v", err)
			resp.Success = false
			resp.Message = &proto.ManagementMessage_Error{
				Error: &proto.ManagementError{
					Code:    proto.ErrorCode_EXECUTION_ERROR,
					Message: err.Error(),
				},
			}
		} else {
			resp.Message = &proto.ManagementMessage_CancelExecutionResponse{
				CancelExecutionResponse: response,
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

		protoAgent := &proto.AgentInfo{
			Id:              fmt.Sprintf("%d", agent.ID),
			Name:            agent.Name,
			Description:     agent.Description,
			Prompt:          agent.Prompt,
			ModelName:       "gpt-4o-mini", // Default model
			MaxSteps:        int32(agent.MaxSteps),
			AssignedTools:   toolNames,
			EnvironmentName: environmentName,
			ScheduleEnabled: scheduleEnabled,
			CronSchedule:    cronSchedule,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
			Status:          proto.AgentStatus_AGENT_STATUS_ACTIVE, // Default to active
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

	// For now, return a placeholder response
	// TODO: Implement actual tool listing from MCP servers
	return &proto.ListToolsManagementResponse{
		Tools:           []*proto.ToolInfo{},
		EnvironmentName: environmentName,
		TotalTools:      0,
		McpServers:      []string{},
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

// handleGetSystemStatus processes get system status requests
func (mhs *ManagementHandlerService) handleGetSystemStatus(ctx context.Context, req *proto.GetSystemStatusRequest) (*proto.GetSystemStatusResponse, error) {
	logging.Info("Getting system status")

	// TODO: Get actual system metrics and active executions
	return &proto.GetSystemStatusResponse{
		Health: proto.SystemHealth_SYSTEM_HEALTHY,
		Metrics: &proto.SystemMetrics{
			CpuUsagePercent:    25.0,
			MemoryUsagePercent: 45.0,
			ActiveConnections:  1,
			ActiveRuns:         0,
		},
		ActiveExecutions:     []*proto.ActiveExecution{},
		StationVersion:       "v0.11.0",
		Uptime:               fmt.Sprintf("%.0f minutes", time.Since(time.Now().Add(-30*time.Minute)).Minutes()),
		LighthouseConnection: proto.ConnectionStatus_CONNECTION_CONNECTED,
	}, nil
}

// handleExecuteAgent processes execute agent requests using unified execution flow (same as MCP/CLI)
func (mhs *ManagementHandlerService) handleExecuteAgent(ctx context.Context, originalRequestId string, req *proto.ExecuteAgentManagementRequest) (*proto.ExecuteAgentManagementResponse, error) {
	logging.Info("Executing agent %s with task: %s", req.AgentId, req.Task)

	// Convert agent ID to int64
	agentID, err := strconv.ParseInt(req.AgentId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID: %s", req.AgentId)
	}

	// Use CloudShip's original request ID as the execution ID for status updates
	// This ensures CloudShip can match status updates to the run they created
	executionID := originalRequestId

	// Send QUEUED status update to CloudShip
	mhs.sendStatusUpdate(originalRequestId, executionID, proto.ExecutionStatus_EXECUTION_QUEUED, "Agent execution queued", 0)

	// Send RUNNING status update to CloudShip
	mhs.sendStatusUpdate(originalRequestId, executionID, proto.ExecutionStatus_EXECUTION_RUNNING, "Agent execution started", 1)

	// Use CloudShip's provided run_id for correlation tracking
	// Store it for status updates and telemetry correlation
	var userID int64 = 1 // Default user ID for CloudShip executions
	
	// Parse CloudShip's run_id to int64 for consistency with our database
	cloudShipRunID, err := strconv.ParseInt(req.RunId, 10, 64)
	if err != nil {
		mhs.sendStatusUpdate(originalRequestId, executionID, proto.ExecutionStatus_EXECUTION_FAILED, fmt.Sprintf("Invalid run_id format: %v", err), 1)
		return nil, fmt.Errorf("invalid run_id format: %v", err)
	}
	
	// Create local agent run (we'll correlate with CloudShip's ID in SendRun)
	run, err := mhs.repos.AgentRuns.Create(ctx, agentID, userID, req.Task, "", 0, nil, nil, "running", nil)
	if err != nil {
		mhs.sendStatusUpdate(originalRequestId, executionID, proto.ExecutionStatus_EXECUTION_FAILED, fmt.Sprintf("Failed to create agent run: %v", err), 1)
		return nil, fmt.Errorf("failed to create agent run: %v", err)
	}
	runID := run.ID
	logging.Info("Using CloudShip's run ID: %d (local ID: %d) for execution tracking", cloudShipRunID, runID)
	
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
	
	// Extract token usage from result
	var inputTokens, outputTokens, totalTokens *int64
	if result != nil && result.TokenUsage != nil {
		inputTokens = extractInt64FromTokenUsage(result.TokenUsage["inputTokens"])
		outputTokens = extractInt64FromTokenUsage(result.TokenUsage["outputTokens"])
		totalTokens = extractInt64FromTokenUsage(result.TokenUsage["totalTokens"])
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
			// Send run data to CloudShip via ManagementChannel
			if mhs.managementChannel != nil {
				tags := map[string]string{
					"execution_id": executionID,
					"source":       "cloudship_management",
					"environment":  "cloudship",
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
					mhs.lighthouseClient.SendRun(typesAgentRun, "cloudship", map[string]string{
						"execution_id": executionID,
						"source":       "cloudship_management",
						"environment":  "cloudship",
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
		ToolCalls:       []*proto.ToolCall{},
		Timestamp:       timestamppb.Now(),
	}, nil
}

// handleListActiveRuns processes list active runs requests
func (mhs *ManagementHandlerService) handleListActiveRuns(ctx context.Context, req *proto.ListActiveRunsRequest) (*proto.ListActiveRunsResponse, error) {
	logging.Info("Listing active runs")

	// TODO: Get actual active runs from execution queue or similar service
	return &proto.ListActiveRunsResponse{
		ActiveRuns:  []*proto.ActiveExecution{},
		TotalActive: 0,
	}, nil
}

// handleCancelExecution processes cancel execution requests
func (mhs *ManagementHandlerService) handleCancelExecution(ctx context.Context, req *proto.CancelExecutionRequest) (*proto.CancelExecutionResponse, error) {
	logging.Info("Cancelling execution: %s", req.ExecutionId)

	// TODO: Implement actual execution cancellation
	return &proto.CancelExecutionResponse{
		Cancelled: false,
		Message:   "Execution cancellation not yet implemented",
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
func (mhs *ManagementHandlerService) convertRunDetailsToProtoAgentRun(runDetails *models.AgentRunWithDetails) *proto.AgentRunData {
	if runDetails == nil {
		return nil
	}

	// Calculate duration in milliseconds
	var durationMs int64
	if runDetails.DurationSeconds != nil {
		durationMs = int64(*runDetails.DurationSeconds * 1000)
	}

	// Handle optional fields
	var completedAt *timestamppb.Timestamp
	if runDetails.CompletedAt != nil {
		completedAt = timestamppb.New(*runDetails.CompletedAt)
	}

	startedAt := timestamppb.New(runDetails.StartedAt)

	modelName := ""
	if runDetails.ModelName != nil {
		modelName = *runDetails.ModelName
	}

	// Create token usage data if available
	var tokenUsage *proto.TokenUsage
	if runDetails.InputTokens != nil || runDetails.OutputTokens != nil || runDetails.TotalTokens != nil {
		tokenUsage = &proto.TokenUsage{}
		
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

	// Create proto AgentRunData
	return &proto.AgentRunData{
		RunId:       fmt.Sprintf("run_%d", runDetails.ID),
		AgentId:     fmt.Sprintf("agent_%d", runDetails.AgentID),
		AgentName:   runDetails.AgentName,
		Task:        runDetails.Task,
		Response:    runDetails.FinalResponse,
		TokenUsage:  tokenUsage,
		DurationMs:  durationMs,
		ModelName:   modelName,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Metadata: map[string]string{
			"run_id":      fmt.Sprintf("%d", runDetails.ID),
			"agent_id":    fmt.Sprintf("%d", runDetails.AgentID),
			"steps_taken": fmt.Sprintf("%d", runDetails.StepsTaken),
			"user_id":     fmt.Sprintf("%d", runDetails.UserID),
			"username":    runDetails.Username,
		},
	}
}
