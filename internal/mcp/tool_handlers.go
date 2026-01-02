package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"station/pkg/benchmark"
	"station/pkg/models"

	"github.com/mark3labs/mcp-go/mcp"
)

// Tool Management Handlers
// Handles tool operations: discover, list, add to agent, remove from agent

func (s *Server) handleDiscoverTools(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get all available tools
	tools, err := s.repos.MCPTools.GetAllWithDetails()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to discover tools: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"tools":   tools,
		"count":   len(tools),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListTools(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract pagination parameters
	limit := request.GetInt("limit", 50)
	offset := request.GetInt("offset", 0)

	// Extract optional filters
	environmentID := request.GetString("environment_id", "")
	search := request.GetString("search", "")

	tools, err := s.repos.MCPTools.GetAllWithDetails()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list tools: %v", err)), nil
	}

	// Apply search filter if provided
	if search != "" {
		filteredTools := make([]*models.MCPToolWithDetails, 0)
		searchLower := strings.ToLower(search)
		for _, tool := range tools {
			if strings.Contains(strings.ToLower(tool.Name), searchLower) ||
				strings.Contains(strings.ToLower(tool.Description), searchLower) {
				filteredTools = append(filteredTools, tool)
			}
		}
		tools = filteredTools
	}

	// Apply environment filter if provided
	if environmentID != "" {
		filteredTools := make([]*models.MCPToolWithDetails, 0)
		for _, tool := range tools {
			if fmt.Sprintf("%d", tool.EnvironmentID) == environmentID {
				filteredTools = append(filteredTools, tool)
			}
		}
		tools = filteredTools
	}

	totalCount := len(tools)

	// Apply pagination
	start := offset
	if start > totalCount {
		start = totalCount
	}

	end := start + limit
	if end > totalCount {
		end = totalCount
	}

	paginatedTools := tools[start:end]

	response := map[string]interface{}{
		"success": true,
		"tools":   paginatedTools,
		"pagination": map[string]interface{}{
			"count":       len(paginatedTools),
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

func (s *Server) handleAddTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	toolName, err := request.RequireString("tool_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'tool_name' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get agent to verify it exists and get environment
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Find tool by name in the agent's environment
	tool, err := s.repos.MCPTools.FindByNameInEnvironment(agent.EnvironmentID, toolName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Tool '%s' not found in environment: %v", toolName, err)), nil
	}

	// Check if tool is already assigned
	existingTools, err := s.repos.AgentTools.ListAgentTools(agentID)
	if err == nil {
		for _, existingTool := range existingTools {
			if existingTool.ToolName == toolName {
				return mcp.NewToolResultError(fmt.Sprintf("Tool '%s' is already assigned to agent '%s'", toolName, agent.Name)), nil
			}
		}
	}

	// Add tool to agent
	_, err = s.repos.AgentTools.AddAgentTool(agentID, tool.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to add tool to agent: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Successfully added tool '%s' to agent '%s'", toolName, agent.Name),
		"agent": map[string]interface{}{
			"id":   agent.ID,
			"name": agent.Name,
		},
		"tool": map[string]interface{}{
			"name": toolName,
			"id":   tool.ID,
		},
	}

	// Auto-export agent to keep file config in sync (Database → Config)
	if s.agentExportService != nil {
		if err := s.agentExportService.ExportAgentAfterSave(agentID); err != nil {
			// Add export error info to response for user awareness
			response["export_warning"] = fmt.Sprintf("Tool added but export failed: %v. Use 'stn agent export %s' to export manually.", err, agent.Name)
		}
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleRemoveTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	toolName, err := request.RequireString("tool_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'tool_name' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get agent to verify it exists
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Find tool by name in the agent's environment
	tool, err := s.repos.MCPTools.FindByNameInEnvironment(agent.EnvironmentID, toolName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Tool '%s' not found in environment: %v", toolName, err)), nil
	}

	// Remove tool from agent
	err = s.repos.AgentTools.RemoveAgentTool(agentID, tool.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to remove tool from agent: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Successfully removed tool '%s' from agent '%s'", toolName, agent.Name),
		"agent": map[string]interface{}{
			"id":   agent.ID,
			"name": agent.Name,
		},
		"tool": map[string]interface{}{
			"name": toolName,
			"id":   tool.ID,
		},
	}

	// Auto-export agent to keep file config in sync (Database → Config)
	if s.agentExportService != nil {
		if err := s.agentExportService.ExportAgentAfterSave(agentID); err != nil {
			// Add export error info to response for user awareness
			response["export_warning"] = fmt.Sprintf("Tool removed but export failed: %v. Use 'stn agent export %s' to export manually.", err, agent.Name)
		}
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleAddAgentAsTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	parentAgentIDStr, err := request.RequireString("parent_agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'parent_agent_id' parameter: %v", err)), nil
	}

	childAgentIDStr, err := request.RequireString("child_agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'child_agent_id' parameter: %v", err)), nil
	}

	parentAgentID, err := strconv.ParseInt(parentAgentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid parent_agent_id format: %v", err)), nil
	}

	childAgentID, err := strconv.ParseInt(childAgentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid child_agent_id format: %v", err)), nil
	}

	// Get both agents to verify they exist
	parentAgent, err := s.repos.Agents.GetByID(parentAgentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Parent agent not found: %v", err)), nil
	}

	childAgent, err := s.repos.Agents.GetByID(childAgentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Child agent not found: %v", err)), nil
	}

	// Verify both agents are in the same environment
	if parentAgent.EnvironmentID != childAgent.EnvironmentID {
		return mcp.NewToolResultError(fmt.Sprintf("Agents must be in the same environment. Parent is in environment %d, child is in environment %d", parentAgent.EnvironmentID, childAgent.EnvironmentID)), nil
	}

	// Add relationship to database
	_, err = s.repos.AgentAgents.AddChildAgent(parentAgentID, childAgentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to add child agent: %v", err)), nil
	}

	// Export agent to update .prompt file with agents: section
	if s.agentExportService != nil {
		if err := s.agentExportService.ExportAgentAfterSave(parentAgentID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to export agent: %v", err)), nil
		}
	}

	// Get environment for sync
	env, err := s.repos.Environments.GetByID(parentAgent.EnvironmentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get environment: %v", err)), nil
	}

	// Run sync to apply changes immediately
	syncCmd := exec.CommandContext(ctx, "stn", "sync", env.Name)
	syncOutput, err := syncCmd.CombinedOutput()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to sync environment: %v\nOutput: %s", err, string(syncOutput))), nil
	}

	// Create normalized tool name for response (same logic as runtime)
	normalizedName := strings.ToLower(childAgent.Name)
	replacements := []string{" ", "-", ".", "!", "@", "#", "$", "%", "^", "&", "*", "(", ")", "+", "=", "[", "]", "{", "}", "|", "\\", ":", ";", "\"", "'", "<", ">", ",", "?", "/"}
	for _, char := range replacements {
		normalizedName = strings.ReplaceAll(normalizedName, char, "_")
	}
	for strings.Contains(normalizedName, "__") {
		normalizedName = strings.ReplaceAll(normalizedName, "__", "_")
	}
	normalizedName = strings.Trim(normalizedName, "_")
	agentToolName := fmt.Sprintf("__agent_%s", normalizedName)

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Successfully added agent '%s' as tool '%s' to agent '%s'", childAgent.Name, agentToolName, parentAgent.Name),
		"parent_agent": map[string]interface{}{
			"id":   parentAgent.ID,
			"name": parentAgent.Name,
		},
		"child_agent": map[string]interface{}{
			"id":   childAgent.ID,
			"name": childAgent.Name,
		},
		"tool_name": agentToolName,
		"note":      "Child agent added to database and exported to agents: section in .prompt file",
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleRemoveAgentAsTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	parentAgentIDStr, err := request.RequireString("parent_agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'parent_agent_id' parameter: %v", err)), nil
	}

	childAgentIDStr, err := request.RequireString("child_agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'child_agent_id' parameter: %v", err)), nil
	}

	parentAgentID, err := strconv.ParseInt(parentAgentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid parent_agent_id format: %v", err)), nil
	}

	childAgentID, err := strconv.ParseInt(childAgentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid child_agent_id format: %v", err)), nil
	}

	// Get both agents to verify they exist
	parentAgent, err := s.repos.Agents.GetByID(parentAgentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Parent agent not found: %v", err)), nil
	}

	childAgent, err := s.repos.Agents.GetByID(childAgentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Child agent not found: %v", err)), nil
	}

	// Remove relationship from database
	err = s.repos.AgentAgents.RemoveChildAgent(parentAgentID, childAgentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to remove child agent: %v", err)), nil
	}

	// Export agent to update .prompt file (removes from agents: section)
	if s.agentExportService != nil {
		if err := s.agentExportService.ExportAgentAfterSave(parentAgentID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to export agent: %v", err)), nil
		}
	}

	// Get environment for sync
	env, err := s.repos.Environments.GetByID(parentAgent.EnvironmentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get environment: %v", err)), nil
	}

	// Run sync to apply changes immediately
	syncCmd := exec.CommandContext(ctx, "stn", "sync", env.Name)
	syncOutput, err := syncCmd.CombinedOutput()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to sync environment: %v\nOutput: %s", err, string(syncOutput))), nil
	}

	// Create normalized tool name for response
	normalizedName := strings.ToLower(childAgent.Name)
	replacements := []string{" ", "-", ".", "!", "@", "#", "$", "%", "^", "&", "*", "(", ")", "+", "=", "[", "]", "{", "}", "|", "\\", ":", ";", "\"", "'", "<", ">", ",", "?", "/"}
	for _, char := range replacements {
		normalizedName = strings.ReplaceAll(normalizedName, char, "_")
	}
	for strings.Contains(normalizedName, "__") {
		normalizedName = strings.ReplaceAll(normalizedName, "__", "_")
	}
	normalizedName = strings.Trim(normalizedName, "_")
	agentToolName := fmt.Sprintf("__agent_%s", normalizedName)

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Successfully removed agent '%s' (tool '%s') from agent '%s'", childAgent.Name, agentToolName, parentAgent.Name),
		"parent_agent": map[string]interface{}{
			"id":   parentAgent.ID,
			"name": parentAgent.Name,
		},
		"child_agent": map[string]interface{}{
			"id":   childAgent.ID,
			"name": childAgent.Name,
		},
		"tool_name": agentToolName,
		"note":      "Child agent removed from database and agents: section in .prompt file",
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// Benchmark Tools

func (s *Server) handleEvaluateBenchmark(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runIDStr, err := request.RequireString("run_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'run_id' parameter: %v", err)), nil
	}

	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid run_id format: %v", err)), nil
	}

	// Create benchmark task ID
	taskID := fmt.Sprintf("bench_%d_%d", runID, time.Now().Unix())

	// Start async benchmark evaluation
	if s.benchmarkService == nil {
		return mcp.NewToolResultError("Benchmark service not available"), nil
	}

	if err := s.benchmarkService.EvaluateAsync(ctx, runID, taskID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start benchmark: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Benchmark evaluation started",
		"task_id": taskID,
		"run_id":  runID,
		"status":  "pending",
		"note":    "Use get_benchmark_status to check progress",
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleGetBenchmarkStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, err := request.RequireString("task_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'task_id' parameter: %v", err)), nil
	}

	if s.benchmarkService == nil {
		return mcp.NewToolResultError("Benchmark service not available"), nil
	}

	// Query benchmark task status
	task, err := s.benchmarkService.GetTaskStatus(taskID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Task not found: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"task_id": taskID,
		"run_id":  task.RunID,
		"status":  task.Status,
	}

	if task.Status == "completed" && task.Result != nil {
		response["message"] = "Benchmark evaluation completed"
		response["result"] = task.Result
	} else if task.Status == "failed" {
		response["message"] = "Benchmark evaluation failed"
		if task.Error != nil {
			response["error"] = task.Error.Error()
		}
	} else if task.Status == "running" {
		response["message"] = "Benchmark evaluation in progress"
	} else {
		response["message"] = "Benchmark evaluation pending"
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListBenchmarkResults(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := int64(10)
	limitStr := request.GetString("limit", "")
	if limitStr != "" {
		if parsed, err := strconv.ParseInt(limitStr, 10, 64); err == nil {
			limit = parsed
		}
	}

	var runID *int64
	runIDStr := request.GetString("run_id", "")
	if runIDStr != "" {
		if parsed, err := strconv.ParseInt(runIDStr, 10, 64); err == nil {
			runID = &parsed
		}
	}

	if s.benchmarkService == nil {
		return mcp.NewToolResultError("Benchmark service not available"), nil
	}

	// Query benchmark results
	tasks, err := s.benchmarkService.ListResults(int(limit), runID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list results: %v", err)), nil
	}

	// Convert tasks to response format
	results := make([]map[string]interface{}, 0, len(tasks))
	for _, task := range tasks {
		result := map[string]interface{}{
			"task_id": task.TaskID,
			"run_id":  task.RunID,
			"status":  task.Status,
		}
		if task.Result != nil {
			result["result"] = task.Result
		}
		results = append(results, result)
	}

	response := map[string]interface{}{
		"success": true,
		"results": results,
		"count":   len(results),
		"limit":   limit,
	}
	if runID != nil {
		response["run_id"] = *runID
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleEvaluateDataset(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get dataset path
	datasetPath, err := request.RequireString("dataset_path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'dataset_path' parameter: %v", err)), nil
	}

	if s.benchmarkService == nil {
		return mcp.NewToolResultError("Benchmark service not available"), nil
	}

	// Load dataset.json file
	datasetFile := filepath.Join(datasetPath, "dataset.json")
	datasetBytes, err := os.ReadFile(datasetFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read dataset file: %v", err)), nil
	}

	// Parse dataset
	var datasetInput benchmark.DatasetEvaluationInput
	if err := json.Unmarshal(datasetBytes, &datasetInput); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse dataset: %v", err)), nil
	}

	// Set dataset ID from path
	datasetInput.DatasetID = filepath.Base(datasetPath)

	// Evaluate dataset
	result, err := s.benchmarkService.EvaluateDataset(ctx, &datasetInput)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Dataset evaluation failed: %v", err)), nil
	}

	// Save result to llm_evaluation.json
	evaluationFile := filepath.Join(datasetPath, "llm_evaluation.json")
	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal evaluation result: %v", err)), nil
	}

	if err := os.WriteFile(evaluationFile, resultBytes, 0644); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to save evaluation result: %v", err)), nil
	}

	// Return summary
	response := map[string]interface{}{
		"success":          true,
		"message":          "Dataset evaluation completed",
		"dataset_id":       result.DatasetID,
		"runs_evaluated":   result.RunsEvaluated,
		"overall_score":    result.OverallScore,
		"production_ready": result.ProductionReady,
		"recommendation":   result.Recommendation,
		"aggregate_scores": result.AggregateScores,
		"pass_rates":       result.PassRates,
		"key_strengths":    result.KeyStrengths,
		"key_weaknesses":   result.KeyWeaknesses,
		"output_file":      evaluationFile,
		"evaluation_cost":  fmt.Sprintf("$%.4f", result.TotalJudgeCost),
		"evaluation_time":  fmt.Sprintf("%.2fs", float64(result.EvaluationTimeMS)/1000),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// Schedule Management Tools

func (s *Server) handleSetSchedule(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	cronSchedule, err := request.RequireString("cron_schedule")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'cron_schedule' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get agent to verify it exists
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Get optional parameters
	scheduleVars := request.GetString("schedule_variables", "")
	enabled := true
	enabledStr := request.GetString("enabled", "true")
	if enabledStr == "false" {
		enabled = false
	}

	// Validate schedule variables against agent schema
	var warnings []string
	if scheduleVars != "" {
		// Parse schedule variables
		var scheduleVarsMap map[string]interface{}
		if err := json.Unmarshal([]byte(scheduleVars), &scheduleVarsMap); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid schedule_variables JSON: %v", err)), nil
		}

		// Get agent's expected variables from schema
		expectedVars := []string{"userInput"} // Default variable always available
		if agent.InputSchema != nil && *agent.InputSchema != "" {
			// Parse custom schema to get additional variable names
			var customSchema map[string]interface{}
			if err := json.Unmarshal([]byte(*agent.InputSchema), &customSchema); err == nil {
				for varName := range customSchema {
					expectedVars = append(expectedVars, varName)
				}
			}
		}

		// Check if schedule variables contain at least one expected variable
		foundMatch := false
		for _, expectedVar := range expectedVars {
			if _, exists := scheduleVarsMap[expectedVar]; exists {
				foundMatch = true
				break
			}
		}

		if !foundMatch {
			warnings = append(warnings, fmt.Sprintf("WARNING: Schedule variables don't contain any expected fields. Agent expects: %v", expectedVars))
		}
	}

	// Update agent schedule in database
	// TODO: Use proper repository method when available
	_, err = s.db.Conn().Exec(
		"UPDATE agents SET cron_schedule = ?, schedule_variables = ?, schedule_enabled = ?, is_scheduled = ? WHERE id = ?",
		cronSchedule, scheduleVars, enabled, enabled, agentID,
	)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to set schedule: %v", err)), nil
	}

	// Update the agent object with new schedule
	agent.CronSchedule = &cronSchedule
	agent.ScheduleEnabled = enabled
	agent.IsScheduled = enabled

	// Add/update schedule in the running scheduler service
	if s.schedulerService != nil && enabled {
		if err := s.schedulerService.ScheduleAgent(agent); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to activate schedule in scheduler: %v", err)), nil
		}
	} else if s.schedulerService != nil && !enabled {
		// If disabled, remove from scheduler
		s.schedulerService.UnscheduleAgent(agentID)
	}

	if err := s.agentExportService.ExportAgentAfterSave(agentID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to export agent after schedule update: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Schedule set for agent '%s' and activated in scheduler", agent.Name),
		"agent": map[string]interface{}{
			"id":   agent.ID,
			"name": agent.Name,
		},
		"schedule": map[string]interface{}{
			"cron_expression":    cronSchedule,
			"enabled":            enabled,
			"schedule_variables": scheduleVars,
		},
	}

	// Add warnings if any
	if len(warnings) > 0 {
		response["warnings"] = warnings
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleRemoveSchedule(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get agent to verify it exists
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Clear schedule in database
	_, err = s.db.Conn().Exec(
		"UPDATE agents SET cron_schedule = NULL, schedule_variables = NULL, schedule_enabled = 0, is_scheduled = 0 WHERE id = ?",
		agentID,
	)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to remove schedule: %v", err)), nil
	}

	// Remove from the running scheduler service
	if s.schedulerService != nil {
		s.schedulerService.UnscheduleAgent(agentID)
	}

	if err := s.agentExportService.ExportAgentAfterSave(agentID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to export agent after schedule removal: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Schedule removed from agent '%s' and deactivated in scheduler", agent.Name),
		"agent": map[string]interface{}{
			"id":   agent.ID,
			"name": agent.Name,
		},
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleGetSchedule(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get agent with schedule info
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Query schedule details
	var cronSchedule, scheduleVars sql.NullString
	var scheduleEnabled, isScheduled bool
	var lastRun, nextRun sql.NullTime

	err = s.db.Conn().QueryRow(
		"SELECT cron_schedule, schedule_variables, schedule_enabled, is_scheduled, last_scheduled_run, next_scheduled_run FROM agents WHERE id = ?",
		agentID,
	).Scan(&cronSchedule, &scheduleVars, &scheduleEnabled, &isScheduled, &lastRun, &nextRun)

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get schedule: %v", err)), nil
	}

	scheduleInfo := map[string]interface{}{
		"enabled":      scheduleEnabled,
		"is_scheduled": isScheduled,
	}

	if cronSchedule.Valid {
		scheduleInfo["cron_expression"] = cronSchedule.String
	}
	if scheduleVars.Valid {
		scheduleInfo["schedule_variables"] = scheduleVars.String
	}
	if lastRun.Valid {
		scheduleInfo["last_run"] = lastRun.Time.Format(time.RFC3339)
	}
	if nextRun.Valid {
		scheduleInfo["next_run"] = nextRun.Time.Format(time.RFC3339)
	}

	response := map[string]interface{}{
		"success": true,
		"agent": map[string]interface{}{
			"id":   agent.ID,
			"name": agent.Name,
		},
		"schedule": scheduleInfo,
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}
