package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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

	// Create agent tool name with __agent_ prefix
	// CRITICAL: Normalize name same way as mcp_connection_manager.go:getAgentToolsForEnvironment
	normalizedName := strings.ToLower(childAgent.Name)
	// Replace all special characters with underscores (same normalization as agent tool creation)
	replacements := []string{" ", "-", ".", "!", "@", "#", "$", "%", "^", "&", "*", "(", ")", "+", "=", "[", "]", "{", "}", "|", "\\", ":", ";", "\"", "'", "<", ">", ",", "?", "/"}
	for _, char := range replacements {
		normalizedName = strings.ReplaceAll(normalizedName, char, "_")
	}
	// Remove multiple consecutive underscores
	for strings.Contains(normalizedName, "__") {
		normalizedName = strings.ReplaceAll(normalizedName, "__", "_")
	}
	// Trim leading/trailing underscores
	normalizedName = strings.Trim(normalizedName, "_")

	agentToolName := fmt.Sprintf("__agent_%s", normalizedName)

	// Get environment to build file path
	env, err := s.repos.Environments.GetByID(parentAgent.EnvironmentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get environment: %v", err)), nil
	}

	// Build path to agent .prompt file
	promptFilePath := fmt.Sprintf("%s/.config/station/environments/%s/agents/%s.prompt",
		os.Getenv("HOME"), env.Name, parentAgent.Name)

	// Check if file exists, if not export it first
	if _, err := os.Stat(promptFilePath); os.IsNotExist(err) {
		if s.agentExportService == nil {
			return mcp.NewToolResultError("Agent export service not available"), nil
		}
		if err := s.agentExportService.ExportAgentAfterSave(parentAgentID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to export agent: %v", err)), nil
		}
	}

	// Read current file content
	content, err := os.ReadFile(promptFilePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read agent file: %v", err)), nil
	}

	// Parse YAML frontmatter to add tools
	contentStr := string(content)

	// Find the tools: section or add it after max_steps
	if !strings.Contains(contentStr, "tools:") {
		// Add tools section after max_steps line
		contentStr = strings.Replace(contentStr,
			fmt.Sprintf("max_steps: %d\n", parentAgent.MaxSteps),
			fmt.Sprintf("max_steps: %d\ntools:\n  - \"%s\"\n", parentAgent.MaxSteps, agentToolName),
			1)
	} else {
		// Append to existing tools list
		lines := strings.Split(contentStr, "\n")
		newLines := make([]string, 0, len(lines)+1)
		for i, line := range lines {
			newLines = append(newLines, line)
			if strings.HasPrefix(strings.TrimSpace(line), "tools:") {
				// Found tools section, now find where it ends
				j := i + 1
				for j < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[j]), "-") {
					newLines = append(newLines, lines[j])
					j++
				}
				// Add new tool after last tool item
				newLines = append(newLines, fmt.Sprintf("  - \"%s\"", agentToolName))
				// Append rest of file
				newLines = append(newLines, lines[j:]...)
				break
			}
		}
		contentStr = strings.Join(newLines, "\n")
	}

	// Write modified content back
	if err := os.WriteFile(promptFilePath, []byte(contentStr), 0644); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to write agent file: %v", err)), nil
	}

	// Run sync to apply changes immediately
	syncCmd := exec.CommandContext(ctx, "stn", "sync", env.Name)
	syncOutput, err := syncCmd.CombinedOutput()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to sync environment: %v\nOutput: %s", err, string(syncOutput))), nil
	}

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
		"note":      "Agent tools are created dynamically at runtime. Run 'stn sync' to apply changes.",
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}
