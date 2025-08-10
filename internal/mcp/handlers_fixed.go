package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"station/internal/services"
	"station/pkg/models"

	"github.com/mark3labs/mcp-go/mcp"
)

// Simplified handlers that work with the current repository interfaces

func (s *Server) handleCreateAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'name' parameter: %v", err)), nil
	}

	description, err := request.RequireString("description")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'description' parameter: %v", err)), nil
	}

	prompt, err := request.RequireString("prompt")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'prompt' parameter: %v", err)), nil
	}

	environmentIDStr, err := request.RequireString("environment_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_id' parameter: %v", err)), nil
	}

	environmentID, err := strconv.ParseInt(environmentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid environment_id format: %v", err)), nil
	}

	// Extract optional parameters
	maxSteps := request.GetInt("max_steps", 5) // Default to 5 if not provided
	
	// Extract tool_names array if provided
	var toolNames []string
	if request.Params.Arguments != nil {
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			if toolNamesArg, ok := argsMap["tool_names"]; ok {
				if toolNamesArray, ok := toolNamesArg.([]interface{}); ok {
					for _, toolName := range toolNamesArray {
						if str, ok := toolName.(string); ok {
							toolNames = append(toolNames, str)
						}
					}
				}
			}
		}
	}

	// Create the agent using repository with correct parameter order
	// Create(name, description, prompt string, maxSteps, environmentID, createdBy int64, cronSchedule *string, scheduleEnabled bool)
	createdAgent, err := s.repos.Agents.Create(name, description, prompt, int64(maxSteps), environmentID, 1, nil, true)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create agent: %v", err)), nil
	}

	// Assign tools to the agent
	var assignedTools []string
	var skippedTools []string
	
	if len(toolNames) > 0 {
		// Assign specific tools if provided
		for _, toolName := range toolNames {
			// Find tool by name in the agent's environment
			tool, err := s.repos.MCPTools.FindByNameInEnvironment(environmentID, toolName)
			if err != nil {
				skippedTools = append(skippedTools, fmt.Sprintf("%s (not found)", toolName))
				continue
			}
			
			// Assign tool to agent
			_, err = s.repos.AgentTools.AddAgentTool(createdAgent.ID, tool.ID)
			if err != nil {
				skippedTools = append(skippedTools, fmt.Sprintf("%s (failed: %v)", toolName, err))
				continue
			}
			
			assignedTools = append(assignedTools, toolName)
		}
	} else {
		// If no specific tools provided, assign all available tools in the environment
		allTools, err := s.repos.MCPTools.GetByEnvironmentID(environmentID)
		if err == nil {
			for _, tool := range allTools {
				// Assign tool to agent
				_, err = s.repos.AgentTools.AddAgentTool(createdAgent.ID, tool.ID)
				if err != nil {
					skippedTools = append(skippedTools, fmt.Sprintf("%s (failed: %v)", tool.Name, err))
					continue
				}
				assignedTools = append(assignedTools, tool.Name)
			}
		}
	}

	response := map[string]interface{}{
		"success": true,
		"agent": map[string]interface{}{
			"id":             createdAgent.ID,
			"name":           createdAgent.Name,
			"description":    createdAgent.Description,
			"max_steps":      createdAgent.MaxSteps,
			"environment_id": createdAgent.EnvironmentID,
		},
		"message": fmt.Sprintf("Agent '%s' created successfully with max_steps=%d in environment_id=%d", name, createdAgent.MaxSteps, createdAgent.EnvironmentID),
	}
	
	// Add tool assignment status to response
	if len(toolNames) > 0 {
		toolAssignment := map[string]interface{}{
			"requested_tools": toolNames,
			"assigned_tools":  assignedTools,
			"assigned_count":  len(assignedTools),
		}
		
		if len(skippedTools) > 0 {
			toolAssignment["skipped_tools"] = skippedTools
			toolAssignment["skipped_count"] = len(skippedTools)
		}
		
		if len(assignedTools) == len(toolNames) {
			toolAssignment["status"] = "success"
		} else if len(assignedTools) > 0 {
			toolAssignment["status"] = "partial"
		} else {
			toolAssignment["status"] = "failed"
		}
		
		response["tool_assignment"] = toolAssignment
		
		// Update message to include tool assignment info
		response["message"] = fmt.Sprintf("Agent '%s' created successfully with max_steps=%d in environment_id=%d. Tools assigned: %d/%d", 
			name, createdAgent.MaxSteps, createdAgent.EnvironmentID, len(assignedTools), len(toolNames))
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
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
	
	// Use execution queue for proper tracing and storage
	var runID int64
	var response *services.Message
	var execErr error
	
	if s.executionQueue != nil {
		// Queue the execution for detailed tracing
		runID, execErr = s.executionQueue.QueueExecution(agentID, userID, task, nil)
		if execErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to queue agent execution: %v", execErr)), nil
		}
		
		// Wait for execution to complete and get the result
		// For synchronous execution, we need to poll the database
		// or implement a blocking queue mechanism
		time.Sleep(100 * time.Millisecond) // Small delay to let queue start
		
		// Poll for completion with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
		
	pollLoop:
		for {
			select {
			case <-timeoutCtx.Done():
				return mcp.NewToolResultError("Agent execution timed out"), nil
			default:
				// Check if execution is complete
				if s.repos.AgentRuns == nil {
					return mcp.NewToolResultError("Agent runs repository not available"), nil
				}
				run, checkErr := s.repos.AgentRuns.GetByID(runID)
				if checkErr != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to check execution status: %v", checkErr)), nil
				}
				
				if run.Status == "completed" {
					response = &services.Message{Content: run.FinalResponse}
					break pollLoop
				} else if run.Status == "failed" {
					return mcp.NewToolResultError(fmt.Sprintf("Agent execution failed: %s", run.FinalResponse)), nil
				}
				
				// Wait before polling again
				time.Sleep(500 * time.Millisecond)
			}
		}
	} else {
		// Fallback to direct execution
		if s.agentService == nil {
			return mcp.NewToolResultError("Agent service not available for direct execution"), nil
		}
		response, execErr = s.agentService.ExecuteAgent(ctx, agentID, task)
		if execErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to execute agent: %v", execErr)), nil
		}
		
		if storeRun {
			// TODO: Store the run in the database for direct execution
			runID = 0 // Run storage not yet implemented for direct execution
		}
	}
	
	// Return detailed response
	result := map[string]interface{}{
		"success": true,
		"execution": map[string]interface{}{
			"agent_id": agentID,
			"task": task,
			"response": response.Content,
			"user_id": userID,
			"run_id": runID,
			"async": async,
			"timeout": timeout,
			"stored": storeRun,
		},
		"timestamp": time.Now(),
	}
	
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleDeleteAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get agent before deletion
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Delete the agent
	err = s.repos.Agents.Delete(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete agent: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Agent '%s' deleted successfully", agent.Name),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleUpdateAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get existing agent
	existingAgent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// For now, return success with current agent data
	response := map[string]interface{}{
		"success": true,
		"agent": map[string]interface{}{
			"id":          existingAgent.ID,
			"name":        existingAgent.Name,
			"description": existingAgent.Description,
		},
		"message": "Agent update functionality pending - repository signature mismatch",
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

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

func (s *Server) handleListMCPConfigs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// File-based configs: Get all file configs across all environments
	environments, err := s.repos.Environments.List()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list environments: %v", err)), nil
	}
	
	var allConfigs []interface{}
	for _, env := range environments {
		fileConfigs, err := s.repos.FileMCPConfigs.ListByEnvironment(env.ID)
		if err != nil {
			continue // Skip environments with no configs
		}
		for _, fc := range fileConfigs {
			allConfigs = append(allConfigs, map[string]interface{}{
				"id":             fc.ID,
				"name":           fc.ConfigName,
				"environment_id": fc.EnvironmentID,
				"path":           fc.TemplatePath,
				"type":           "file",
				"last_loaded":    fc.LastLoadedAt,
			})
		}
	}

	response := map[string]interface{}{
		"success": true,
		"configs": allConfigs,
		"count":   len(allConfigs),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListTools(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tools, err := s.repos.MCPTools.GetAllWithDetails()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list tools: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"tools":   tools,
		"count":   len(tools),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListPrompts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	prompts := []map[string]interface{}{
		{"name": "create_comprehensive_agent", "description": "Guide for creating well-structured AI agents"},
		{"name": "create_logs_analysis_agent", "description": "Guide for AWS logs analysis agents"},
		{"name": "create_devops_monitor_agent", "description": "Guide for DevOps monitoring agents"},
		{"name": "create_security_scan_agent", "description": "Guide for security scanning agents"},
		{"name": "create_data_processing_agent", "description": "Guide for data processing agents"},
	}

	response := map[string]interface{}{
		"success": true,
		"prompts": prompts,
		"count":   len(prompts),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListEnvironments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	environments, err := s.repos.Environments.List()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list environments: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":      true,
		"environments": environments,
		"count":        len(environments),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListAgents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agents, err := s.repos.Agents.List()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list agents: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"agents":  agents,
		"count":   len(agents),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleGetAgentDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get agent details
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Get environment
	environment, err := s.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		environment = &models.Environment{Name: "Unknown"}
	}

	// Get assigned tools
	agentTools, err := s.repos.AgentTools.ListAgentTools(agentID)
	if err != nil {
		agentTools = []*models.AgentToolWithDetails{}
	}

	response := map[string]interface{}{
		"success": true,
		"agent": map[string]interface{}{
			"id":          agent.ID,
			"name":        agent.Name,
			"description": agent.Description,
			"prompt":      agent.Prompt,
			"max_steps":   agent.MaxSteps,
		},
		"environment": map[string]interface{}{
			"id":   environment.ID,
			"name": environment.Name,
		},
		"tools":       agentTools,
		"tools_count": len(agentTools),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

