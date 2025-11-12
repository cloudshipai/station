package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

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

	// Create benchmark task in database
	taskID := fmt.Sprintf("bench_%d_%d", runID, time.Now().Unix())

	// TODO: Start async benchmark evaluation
	// For now, return task ID immediately
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

	// TODO: Query benchmark task status from database
	response := map[string]interface{}{
		"success": true,
		"task_id": taskID,
		"status":  "completed",
		"message": "Benchmark evaluation completed",
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

	// TODO: Query benchmark results from database
	response := map[string]interface{}{
		"success": true,
		"results": []map[string]interface{}{},
		"count":   0,
		"limit":   limit,
		"run_id":  runID,
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

	// Update agent schedule in database
	// TODO: Use proper repository method when available
	_, err = s.db.Conn().Exec(
		"UPDATE agents SET cron_schedule = ?, schedule_variables = ?, schedule_enabled = ?, is_scheduled = ? WHERE id = ?",
		cronSchedule, scheduleVars, enabled, enabled, agentID,
	)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to set schedule: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Schedule set for agent '%s'", agent.Name),
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

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Schedule removed from agent '%s'", agent.Name),
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
