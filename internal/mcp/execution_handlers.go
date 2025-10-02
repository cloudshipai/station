package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"station/internal/lighthouse"
	"station/internal/logging"
	"station/internal/services"
	"station/pkg/models"

	"github.com/mark3labs/mcp-go/mcp"
)

// Agent Execution Handlers
// Handles agent execution and runs management: call agent, list runs, inspect runs

// extractInt64FromTokenUsage safely extracts int64 from various numeric types in token usage
// (Same helper function as CLI uses)
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

func (s *Server) handleCallAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Validate required dependencies
	if s.repos == nil {
		return mcp.NewToolResultError("Server repositories not initialized"), nil
	}

	// Get user for agent execution (default user ID for local mode)
	var userID int64 = 1

	// Extract required parameters
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	task, err := request.RequireString("task")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'task' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError("Invalid agent_id format"), nil
	}

	// Extract optional parameters
	async := request.GetBool("async", false)
	timeout := request.GetInt("timeout", 300)
	storeRun := request.GetBool("store_run", true)

	// Extract variables for dotprompt rendering
	var userVariables map[string]interface{}
	if request.Params.Arguments != nil {
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			if variablesArg, ok := argsMap["variables"]; ok {
				if variables, ok := variablesArg.(map[string]interface{}); ok {
					userVariables = variables
				}
			}
		}
	}
	if userVariables == nil {
		userVariables = make(map[string]interface{}) // Default to empty map
	}

	// DEBUG: Temporary file logging to verify execution path
	debugFile := "/tmp/station-lighthouse-debug.log"
	debugLog := func(msg string) {
		os.WriteFile(debugFile, []byte(fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)), os.ModeAppend|0644)
	}
	debugLog(fmt.Sprintf("handleCallAgent called for agentID=%d, storeRun=%v", agentID, storeRun))

	// Use execution queue for proper tracing and storage
	var runID int64
	var response *services.Message
	var execErr error

	if storeRun {
		// Create agent run first to get a proper run ID
		run, err := s.repos.AgentRuns.Create(ctx, agentID, userID, task, "", 0, nil, nil, "running", nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create agent run: %v", err)), nil
		}
		runID = run.ID

		// Get agent details for unified execution flow
		agent, err := s.repos.Agents.GetByID(agentID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
		}

		// Create agent service to access execution engine with lighthouse client for dual flow
		agentService := services.NewAgentService(s.repos, s.lighthouseClient)

		// Use the same unified execution flow as CLI (working version)
		result, execErr := agentService.GetExecutionEngine().Execute(ctx, agent, task, runID, userVariables)
		if execErr != nil {
			// Update run as failed (same as CLI)
			completedAt := time.Now()
			errorMsg := fmt.Sprintf("MCP execution failed: %v", execErr)
			updateErr := s.repos.AgentRuns.UpdateCompletionWithMetadata(
				ctx, runID, errorMsg, 0, nil, nil, "failed", &completedAt,
				nil, nil, nil, nil, nil, nil,
			)
			if updateErr != nil {
				logging.Info("Warning: Failed to update failed run %d: %v", runID, updateErr)
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to execute agent: %v", execErr)), nil
		}

		// Update run as completed with full metadata (same as CLI)
		completedAt := time.Now()
		durationSeconds := result.Duration.Seconds()

		// Extract token usage from result using exact same logic as CLI
		var inputTokens, outputTokens, totalTokens *int64
		var toolsUsed *int64

		if result.TokenUsage != nil {
			// Use same field names and extraction logic as CLI
			if inputVal := extractInt64FromTokenUsage(result.TokenUsage["input_tokens"]); inputVal != nil {
				inputTokens = inputVal
			}
			if outputVal := extractInt64FromTokenUsage(result.TokenUsage["output_tokens"]); outputVal != nil {
				outputTokens = outputVal
			}
			if totalVal := extractInt64FromTokenUsage(result.TokenUsage["total_tokens"]); totalVal != nil {
				totalTokens = totalVal
			}
		}

		if result.StepsUsed > 0 {
			toolsUsedVal := int64(result.StepsUsed) // Using StepsUsed as proxy for tools used
			toolsUsed = &toolsUsedVal
		}

		// Determine status based on execution result success (same as CLI)
		status := "completed"
		if !result.Success {
			status = "failed"
		}

		// Update database with complete metadata (same as CLI)
		err = s.repos.AgentRuns.UpdateCompletionWithMetadata(
			ctx,
			runID,
			result.Response,       // final_response
			result.StepsTaken,     // steps_taken
			result.ToolCalls,      // tool_calls
			result.ExecutionSteps, // execution_steps
			status,                // status - now respects result.Success
			&completedAt,          // completed_at
			inputTokens,           // input_tokens
			outputTokens,          // output_tokens
			totalTokens,           // total_tokens
			&durationSeconds,      // duration_seconds
			&result.ModelName,     // model_name
			toolsUsed,             // tools_used
		)
		if err != nil {
			logging.Info("Warning: Failed to update run %d completion metadata: %v", runID, err)
		}

		// Convert AgentExecutionResult to Message for response
		response = &services.Message{
			Content: result.Response,
		}

		// 🚀 Surgical Lighthouse Integration: Send telemetry AFTER execution completes
		// This avoids GenKit conflicts by using a separate lighthouse client

		// DEBUG: Temporary file logging to verify telemetry
		debugFile := "/tmp/station-lighthouse-debug.log"
		debugLog := func(msg string) {
			os.WriteFile(debugFile, []byte(fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)), os.ModeAppend|0644)
		}

		debugLog(fmt.Sprintf("Lighthouse integration check for run %d", runID))
		debugLog(fmt.Sprintf("lighthouseClient != nil: %v", s.lighthouseClient != nil))

		if s.lighthouseClient != nil {
			isRegistered := s.lighthouseClient.IsRegistered()
			debugLog(fmt.Sprintf("lighthouseClient.IsRegistered(): %v", isRegistered))

			// Update lighthouse status
			lighthouse.SetConnected(true, "localhost:50051")
			lighthouse.SetRegistered(isRegistered, "GLm2oiyW_uI_ACfr8DYUYGkrEngvSxJXTWlyYqJTcq0") // TODO: Get from client

			if isRegistered {
				go func() {
					// Send lighthouse telemetry asynchronously (non-blocking)
					defer func() {
						if r := recover(); r != nil {
							debugLog(fmt.Sprintf("Lighthouse telemetry panic: %v", r))
							logging.Debug("Lighthouse telemetry error (non-critical): %v", r)
						}
					}()

					debugLog(fmt.Sprintf("Converting run %d to lighthouse format", runID))
					// Convert result to lighthouse format
					lighthouseRun := ConvertToLighthouseRun(agent, task, runID, result)
					debugLog(fmt.Sprintf("Lighthouse run created: ID=%s, AgentID=%s, Status=%s", lighthouseRun.ID, lighthouseRun.AgentID, lighthouseRun.Status))

					debugLog(fmt.Sprintf("Sending run %d to lighthouse - Data: AgentID=%s, Status=%s, Response length=%d",
						runID, lighthouseRun.AgentID, lighthouseRun.Status, len(lighthouseRun.Response)))

					// Send via lighthouse client
					s.lighthouseClient.SendRun(lighthouseRun, "default", map[string]string{
						"source": "mcp",
						"mode":   "stdio",
					})

					lighthouse.RecordSuccess()
					debugLog(fmt.Sprintf("Lighthouse telemetry sent successfully for MCP run %d", runID))
					logging.Debug("Lighthouse telemetry sent for MCP run %d", runID)
				}()
			} else {
				lighthouse.RecordError("Lighthouse client is not registered - registration key not found in database")
				debugLog("Lighthouse client is not registered")
			}
		} else {
			lighthouse.RecordError("Lighthouse client is not initialized")
			debugLog("Lighthouse client is nil")
		}
	} else {
		// Execute without run storage using simplified flow
		response, execErr = s.agentService.ExecuteAgent(ctx, agentID, task, userVariables)
		if execErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to execute agent: %v", execErr)), nil
		}
	}

	if response == nil {
		return mcp.NewToolResultError("Agent execution returned nil response"), nil
	}

	// Return detailed response
	result := map[string]interface{}{
		"success": true,
		"execution": map[string]interface{}{
			"agent_id": agentID,
			"task":     task,
			"response": response.Content,
			"user_id":  userID,
			"run_id":   runID,
			"async":    async,
			"timeout":  timeout,
			"stored":   storeRun,
		},
		"timestamp": time.Now(),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListRuns(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract pagination parameters
	limit := request.GetInt("limit", 50)
	offset := request.GetInt("offset", 0)

	// Extract optional filters
	agentIDStr := request.GetString("agent_id", "")
	status := request.GetString("status", "")

	// Get all runs - we'll filter and paginate manually since we need filtering capability
	var allRuns []*models.AgentRunWithDetails
	var err error

	if agentIDStr != "" {
		// Filter by specific agent
		agentID, parseErr := strconv.ParseInt(agentIDStr, 10, 64)
		if parseErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", parseErr)), nil
		}

		// Get basic runs for this agent, then convert to detailed format
		basicRuns, err := s.repos.AgentRuns.ListByAgent(context.Background(), agentID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list runs for agent: %v", err)), nil
		}

		// Convert to detailed format
		allRuns = make([]*models.AgentRunWithDetails, len(basicRuns))
		for i, run := range basicRuns {
			allRuns[i] = &models.AgentRunWithDetails{
				AgentRun:  *run,
				AgentName: "Unknown", // Could be enhanced to fetch agent name
				Username:  "Unknown", // Could be enhanced to fetch user email
			}
		}
	} else {
		// Get recent runs (no agent filter)
		allRuns, err = s.repos.AgentRuns.ListRecent(context.Background(), 1000) // Get large number, then filter/paginate
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list runs: %v", err)), nil
		}
	}

	// Apply status filter if provided
	if status != "" {
		filteredRuns := make([]*models.AgentRunWithDetails, 0)
		for _, run := range allRuns {
			if strings.ToLower(run.Status) == strings.ToLower(status) {
				filteredRuns = append(filteredRuns, run)
			}
		}
		allRuns = filteredRuns
	}

	totalCount := len(allRuns)

	// Apply pagination
	start := offset
	if start > totalCount {
		start = totalCount
	}

	end := start + limit
	if end > totalCount {
		end = totalCount
	}

	paginatedRuns := allRuns[start:end]

	response := map[string]interface{}{
		"success": true,
		"runs":    paginatedRuns,
		"pagination": map[string]interface{}{
			"count":       len(paginatedRuns),
			"total":       totalCount,
			"limit":       limit,
			"offset":      offset,
			"has_more":    end < totalCount,
			"next_offset": end,
		},
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleInspectRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runIDStr, err := request.RequireString("run_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'run_id' parameter: %v", err)), nil
	}

	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid run_id format: %v", err)), nil
	}

	verbose := request.GetBool("verbose", true)

	// Get detailed run information
	run, err := s.repos.AgentRuns.GetByIDWithDetails(context.Background(), runID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get run details: %v", err)), nil
	}

	if run == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Run with ID %d not found", runID)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"run":     run,
	}

	// Add detailed information if verbose is true
	if verbose {
		response["detailed"] = map[string]interface{}{
			"has_tool_calls":        run.ToolCalls != nil,
			"has_execution_steps":   run.ExecutionSteps != nil,
			"tool_calls_count":      0,
			"execution_steps_count": 0,
		}

		if run.ToolCalls != nil {
			// Count tool calls if available
			toolCalls := *run.ToolCalls
			response["detailed"].(map[string]interface{})["tool_calls_count"] = len(toolCalls)
		}

		if run.ExecutionSteps != nil {
			// Count execution steps if available
			execSteps := *run.ExecutionSteps
			response["detailed"].(map[string]interface{})["execution_steps_count"] = len(execSteps)
		}
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}