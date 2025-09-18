package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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