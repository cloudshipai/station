package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"station/internal/auth"
	"station/internal/config"
	"station/internal/services"
	"station/pkg/models"

	"github.com/gin-gonic/gin"
)

// Global semaphore for concurrent agent execution (max 100 concurrent goroutines)
var agentExecutionSem = make(chan struct{}, 100)

// registerAgentAdminRoutes registers admin-only agent management routes
func (h *APIHandlers) registerAgentAdminRoutes(group *gin.RouterGroup) {
	group.POST("", h.createAgent)
	group.GET("/:id", h.getAgent)
	group.GET("/:id/details", h.getAgentWithTools) // New endpoint for detailed view
	group.PUT("/:id", h.updateAgent)
	group.DELETE("/:id", h.deleteAgent)

	// Agent prompt file management
	group.GET("/:id/prompt", h.getAgentPrompt)
	group.PUT("/:id/prompt", h.updateAgentPrompt)

}

// Agent handlers

func (h *APIHandlers) listAgents(c *gin.Context) {
	// Check for environment filter parameter
	envFilter := c.Query("environment_id")

	var agents []*models.Agent
	var err error

	if envFilter != "" {
		envID, parseErr := strconv.ParseInt(envFilter, 10, 64)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment_id"})
			return
		}
		agents, err = h.agentService.ListAgentsByEnvironment(c.Request.Context(), envID)
	} else {
		// For now, list all agents (environment_id = 0 means all environments)
		agents, err = h.agentService.ListAgentsByEnvironment(c.Request.Context(), 0)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list agents"})
		return
	}

	// Filter by environment if specified
	if envFilter != "" {
		// Try to parse as environment ID or name
		var targetEnvID int64 = -1

		// Try as ID first
		if envID, err := strconv.ParseInt(envFilter, 10, 64); err == nil {
			targetEnvID = envID
		} else {
			// Try as environment name
			envs, err := h.repos.Environments.List()
			if err == nil {
				for _, env := range envs {
					if env.Name == envFilter {
						targetEnvID = env.ID
						break
					}
				}
			}
		}

		if targetEnvID == -1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Environment with ID '%s' not found", envFilter)})
			return
		}

		// Filter agents by environment
		var filteredAgents []*models.Agent
		for _, agent := range agents {
			if agent.EnvironmentID == targetEnvID {
				filteredAgents = append(filteredAgents, agent)
			}
		}
		agents = filteredAgents
	}

	// Enrich agents with child agent relationships
	enrichedAgents := make([]map[string]interface{}, len(agents))
	for i, agent := range agents {
		agentMap := map[string]interface{}{
			"id":               agent.ID,
			"name":             agent.Name,
			"description":      agent.Description,
			"environment_id":   agent.EnvironmentID,
			"max_steps":        agent.MaxSteps,
			"created_at":       agent.CreatedAt,
			"updated_at":       agent.UpdatedAt,
			"prompt":           agent.Prompt,
			"schedule_enabled": agent.ScheduleEnabled,
			"cron_schedule":    agent.CronSchedule,
			"is_scheduled":     agent.IsScheduled,
		}

		// Add child agents if any exist
		if h.repos != nil && h.repos.AgentAgents != nil {
			childAgents, err := h.repos.AgentAgents.ListChildAgents(agent.ID)
			if err == nil && len(childAgents) > 0 {
				// Convert to simple format for UI
				children := make([]map[string]interface{}, len(childAgents))
				for j, child := range childAgents {
					children[j] = map[string]interface{}{
						"id":          child.ChildAgentID,
						"name":        child.ChildAgent.Name,
						"description": child.ChildAgent.Description,
					}
				}
				agentMap["child_agents"] = children
			} else {
				agentMap["child_agents"] = []interface{}{} // Empty array if no children
			}
		}

		enrichedAgents[i] = agentMap
	}

	c.JSON(http.StatusOK, gin.H{
		"agents": enrichedAgents,
		"count":  len(enrichedAgents),
	})
}

func (h *APIHandlers) callAgent(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	var req struct {
		Task string `json:"task" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate agent exists
	agent, err := h.agentService.GetAgent(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// Execute agent in background goroutine to avoid blocking the API response
	go func() {
		// Acquire semaphore to limit concurrent executions (max 100)
		agentExecutionSem <- struct{}{}
		defer func() { <-agentExecutionSem }()

		ctx := context.Background()

		// Skip execution in test mode (when repos is nil)
		if h.repos == nil || h.repos.AgentRuns == nil {
			log.Printf("⚠️  Agent execution skipped (test mode)")
			return
		}

		// Create run record
		userID := int64(1) // Console user for local execution

		// Based on working examples in the codebase, use the full signature
		agentRun, err := h.repos.AgentRuns.Create(ctx, agentID, userID, req.Task, "", 0, nil, nil, "running", nil)
		if err != nil {
			log.Printf("❌ Failed to create agent run: %v", err)
			return
		}

		// Execute the agent using the full agent execution engine with proper signature
		response, err := h.agentService.ExecuteAgentWithRunID(ctx, agentID, req.Task, agentRun.ID, nil)
		if err != nil {
			log.Printf("❌ Agent execution failed: %v", err)
			// Update run status to failed with error message
			completedAt := time.Now()
			errorMsg := fmt.Sprintf("API execution failed: %v", err)
			h.repos.AgentRuns.UpdateCompletionWithMetadata(
				ctx, agentRun.ID, errorMsg, 0, nil, nil, "failed", &completedAt,
				nil, nil, nil, nil, nil, nil, &errorMsg,
			)
		} else {
			log.Printf("✅ Agent execution completed successfully for run ID: %d", agentRun.ID)

			// Extract metadata from response.Extra (matches CLI pattern in cmd/main/handlers/agent/local.go:594-663)
			finalResponse := ""
			if response != nil {
				finalResponse = response.Content
			}

			completedAt := time.Now()

			// Extract execution metadata from Extra field
			var inputTokens, outputTokens, totalTokens *int64
			var durationSeconds *float64
			var modelName *string
			var toolCalls, executionSteps *models.JSONArray
			var stepsTaken int64
			var toolsUsed *int64

			if response != nil && response.Extra != nil {
				log.Printf("DEBUG API: response.Extra keys: %+v", response.Extra)
				log.Printf("DEBUG API: token_usage type: %T, value: %+v", response.Extra["token_usage"], response.Extra["token_usage"])

				// Extract token usage
				if tokenUsage, ok := response.Extra["token_usage"].(map[string]interface{}); ok {
					log.Printf("DEBUG API: Successfully cast token_usage to map[string]interface{}")
					log.Printf("DEBUG API: input_tokens type: %T, value: %v", tokenUsage["input_tokens"], tokenUsage["input_tokens"])

					// Try int first, then float64
					if val, ok := tokenUsage["input_tokens"].(int); ok {
						v := int64(val)
						inputTokens = &v
						log.Printf("DEBUG API: Set input_tokens from int: %d", v)
					} else if val, ok := tokenUsage["input_tokens"].(float64); ok {
						v := int64(val)
						inputTokens = &v
						log.Printf("DEBUG API: Set input_tokens from float64: %d", v)
					} else {
						log.Printf("DEBUG API: Failed to extract input_tokens, type was: %T", tokenUsage["input_tokens"])
					}
					if val, ok := tokenUsage["output_tokens"].(int); ok {
						v := int64(val)
						outputTokens = &v
					} else if val, ok := tokenUsage["output_tokens"].(float64); ok {
						v := int64(val)
						outputTokens = &v
					}
					if val, ok := tokenUsage["total_tokens"].(int); ok {
						v := int64(val)
						totalTokens = &v
					} else if val, ok := tokenUsage["total_tokens"].(float64); ok {
						v := int64(val)
						totalTokens = &v
					}
				}

				// Extract duration
				if val, ok := response.Extra["duration"].(float64); ok {
					durationSeconds = &val
				}

				// Extract model name
				if val, ok := response.Extra["model_name"].(string); ok {
					modelName = &val
				}

				// Extract tool calls
				if val, ok := response.Extra["tool_calls"].(*models.JSONArray); ok {
					toolCalls = val
				}

				// Extract execution steps
				if val, ok := response.Extra["execution_steps"].(*models.JSONArray); ok {
					executionSteps = val
				}

				// Extract steps taken
				if val, ok := response.Extra["steps_taken"].(int64); ok {
					stepsTaken = val
				} else if val, ok := response.Extra["steps_taken"].(float64); ok {
					stepsTaken = int64(val)
				}

				// Extract tools used count
				if val, ok := response.Extra["tools_used"].(int); ok {
					v := int64(val)
					toolsUsed = &v
				} else if val, ok := response.Extra["tools_used"].(int64); ok {
					toolsUsed = &val
				}
			}

			// Update run with all metadata (matches CLI pattern)
			h.repos.AgentRuns.UpdateCompletionWithMetadata(
				ctx, agentRun.ID, finalResponse, stepsTaken, toolCalls, executionSteps,
				"completed", &completedAt,
				inputTokens, outputTokens, totalTokens, durationSeconds, modelName, toolsUsed, nil,
			)
		}
	}()

	// Return immediate response
	c.JSON(http.StatusAccepted, gin.H{
		"message":    "Agent execution initiated (async mode)",
		"agent_id":   agentID,
		"agent_name": agent.Name,
		"task":       req.Task,
		"status":     "running",
		"note":       "Agent is executing in background - check /runs endpoint for progress",
	})
}

func (h *APIHandlers) queueAgent(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	var req struct {
		Task      string                 `json:"task" binding:"required"`
		Variables map[string]interface{} `json:"variables"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user ID for tracking (use console user for local mode)
	var userID int64 = 1 // Default console user
	if !h.localMode {
		user, exists := auth.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}
		userID = user.ID
	}

	// Get agent details
	agent, err := h.agentService.GetAgent(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// Create agent run record (same as CLI approach)
	agentRun, err := h.repos.AgentRuns.Create(
		c.Request.Context(),
		agentID,
		userID,
		req.Task,
		"",        // final_response (will be updated)
		0,         // steps_taken
		nil,       // tool_calls
		nil,       // execution_steps
		"running", // status
		nil,       // completed_at
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create agent run: %v", err)})
		return
	}

	// Execute agent directly (same as CLI) - async with goroutine for API responsiveness
	go func() {
		ctx := context.Background()
		metadata := map[string]interface{}{
			"source":       "api_execution",
			"triggered_by": "api",
			"api_endpoint": c.Request.URL.Path,
		}

		// Merge variables into metadata
		if req.Variables != nil {
			for k, v := range req.Variables {
				metadata[k] = v
			}
		}

		_, err := h.agentService.ExecuteAgentWithRunID(ctx, agent.ID, req.Task, agentRun.ID, metadata)
		if err != nil {
			log.Printf("Agent execution failed (Run ID: %d): %v", agentRun.ID, err)
		} else {
			log.Printf("Agent execution completed successfully (Run ID: %d)", agentRun.ID)
		}

	}()

	c.JSON(http.StatusAccepted, gin.H{
		"agent_id": agentID,
		"task":     req.Task,
		"run_id":   agentRun.ID,
		"status":   "running",
		"message":  "Agent execution started",
	})
}

func (h *APIHandlers) createAgent(c *gin.Context) {
	var req struct {
		Name          string      `json:"name" binding:"required"`
		Description   string      `json:"description" binding:"required"`
		Prompt        string      `json:"prompt" binding:"required"`
		EnvironmentID int64       `json:"environment_id" binding:"required"`
		MaxSteps      int64       `json:"max_steps"`
		AssignedTools []string    `json:"assigned_tools"`
		InputSchema   interface{} `json:"input_schema,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user for created_by field
	var createdBy int64 = 1 // Default for local mode
	if !h.localMode {
		user, exists := auth.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}
		createdBy = user.ID
	}

	// Set default max steps if not provided
	if req.MaxSteps == 0 {
		req.MaxSteps = 25
	}

	// Process input schema if provided
	var inputSchemaJSON *string
	if req.InputSchema != nil {
		schemaBytes, err := json.Marshal(req.InputSchema)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input schema format"})
			return
		}
		schemaStr := string(schemaBytes)
		inputSchemaJSON = &schemaStr
	}

	// Create AgentConfig for unified service
	config := &services.AgentConfig{
		EnvironmentID: req.EnvironmentID,
		Name:          req.Name,
		Description:   req.Description,
		Prompt:        req.Prompt,
		AssignedTools: req.AssignedTools,
		MaxSteps:      req.MaxSteps,
		CreatedBy:     createdBy,
		InputSchema:   inputSchemaJSON,
	}

	// Create the agent using the unified service
	agent, err := h.agentService.CreateAgent(c.Request.Context(), config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create agent"})
		return
	}

	// TODO: Handle tool assignments
	// For now, we'll skip tool assignment - this would require:
	// 1. Validating that tools exist in the environment
	// 2. Creating AgentTool records in the database

	if len(req.AssignedTools) > 0 {
		// Placeholder for tool assignment
		// h.assignToolsToAgent(agentID, req.AssignedTools, req.EnvironmentID)
	}

	// Automatically export agent to file-based config after successful DB save
	if h.agentExportService != nil {
		if err := h.agentExportService.ExportAgentAfterSave(agent.ID); err != nil {
			// Log the error but don't fail the request - the agent was successfully created in DB
			log.Printf("Failed to export agent %d after creation: %v", agent.ID, err)
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":        "Agent created successfully",
		"agent_id":       agent.ID,
		"agent_name":     agent.Name,
		"environment_id": config.EnvironmentID,
	})
}

func (h *APIHandlers) getAgent(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	agent, err := h.agentService.GetAgent(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agent": agent})
}

// getAgentWithTools returns agent details including tools and MCP server relationships
func (h *APIHandlers) getAgentWithTools(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	// Get agent first
	agent, err := h.agentService.GetAgent(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// Get environment to find MCP configs
	environment, err := h.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get agent environment"})
		return
	}

	// Get MCP servers and tools assigned to this specific agent
	mcpServers, err := h.getAgentAssignedTools(agent, environment.Name)
	if err != nil {
		log.Printf("Failed to get agent assigned tools: %v", err)
		mcpServers = []gin.H{} // Return empty array on error
	}

	// Ensure we never return nil - always return at least an empty array
	if mcpServers == nil {
		mcpServers = []gin.H{}
	}

	c.JSON(http.StatusOK, gin.H{
		"agent":       agent,
		"mcp_servers": mcpServers,
	})
}

// getAgentAssignedTools gets only the tools assigned to this specific agent
func (h *APIHandlers) getAgentAssignedTools(agent *models.Agent, environmentName string) ([]gin.H, error) {
	// First, get the agent's assigned tools from its prompt file
	assignedTools, err := h.getAgentToolsFromPrompt(agent, environmentName)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent tools: %w", err)
	}

	if len(assignedTools) == 0 {
		return []gin.H{}, nil
	}

	// Map each assigned tool to its MCP server
	mcpServers := make(map[string][]gin.H)
	toolID := 1

	for _, toolName := range assignedTools {
		serverName := h.getToolMCPServer(toolName)
		if serverName == "" {
			continue // Skip tools that don't map to known MCP servers
		}

		tool := gin.H{
			"id":           toolID,
			"name":         toolName,
			"description":  fmt.Sprintf("Tool: %s", toolName),
			"input_schema": "{}",
		}

		mcpServers[serverName] = append(mcpServers[serverName], tool)
		toolID++
	}

	// Convert to the expected format
	result := []gin.H{} // Initialize as empty array instead of nil
	serverID := 1
	for serverName, tools := range mcpServers {
		server := gin.H{
			"id":    serverID,
			"name":  serverName,
			"tools": tools,
		}
		result = append(result, server)
		serverID++
	}

	return result, nil
}

// getAgentToolsFromPrompt extracts the tools list from the agent's prompt file
func (h *APIHandlers) getAgentToolsFromPrompt(agent *models.Agent, environmentName string) ([]string, error) {
	promptFile := config.GetAgentPromptPath(environmentName, agent.Name)

	content, err := ioutil.ReadFile(promptFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read prompt file: %w", err)
	}

	// Parse YAML frontmatter to extract tools and child agents
	lines := strings.Split(string(content), "\n")
	inFrontmatter := false
	var tools []string
	inToolsSection := false
	inAgentsSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			} else {
				break // End of frontmatter
			}
		}

		if !inFrontmatter {
			continue
		}

		// Check for section headers
		if strings.HasPrefix(line, "tools:") {
			inToolsSection = true
			inAgentsSection = false
			continue
		} else if strings.HasPrefix(line, "agents:") {
			inAgentsSection = true
			inToolsSection = false
			continue
		} else if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "-") {
			// New section started, exit both
			inToolsSection = false
			inAgentsSection = false
			continue
		}

		// Parse tool entries from tools: section
		if inToolsSection && strings.HasPrefix(line, "-") {
			// Extract tool name (handle quoted and unquoted)
			tool := strings.TrimSpace(line[1:]) // Remove leading dash
			tool = strings.Trim(tool, "\"")     // Remove quotes if present
			if tool != "" {
				tools = append(tools, tool)
			}
		}

		// Parse agent entries from agents: section and convert to __agent_* format
		if inAgentsSection && strings.HasPrefix(line, "-") {
			// Extract agent name
			agentName := strings.TrimSpace(line[1:])  // Remove leading dash
			agentName = strings.Trim(agentName, "\"") // Remove quotes if present
			if agentName != "" {
				// Normalize agent name to tool name format (same as runtime)
				normalizedName := strings.ToLower(agentName)
				replacements := []string{" ", "-", ".", "!", "@", "#", "$", "%", "^", "&", "*", "(", ")", "+", "=", "[", "]", "{", "}", "|", "\\", ":", ";", "\"", "'", "<", ">", ",", "?", "/"}
				for _, char := range replacements {
					normalizedName = strings.ReplaceAll(normalizedName, char, "_")
				}
				for strings.Contains(normalizedName, "__") {
					normalizedName = strings.ReplaceAll(normalizedName, "__", "_")
				}
				normalizedName = strings.Trim(normalizedName, "_")
				agentToolName := fmt.Sprintf("__agent_%s", normalizedName)
				tools = append(tools, agentToolName)
			}
		}
	}

	return tools, nil
}

// getToolMCPServer dynamically looks up which MCP server hosts the given tool
func (h *APIHandlers) getToolMCPServer(toolName string) string {
	// Query database through repositories to find which MCP server has this tool
	serverName, err := h.repos.MCPTools.GetServerNameForTool(toolName)
	if err != nil {
		// Tool not found in any MCP server
		return ""
	}

	return serverName
}

func (h *APIHandlers) updateAgent(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	var req struct {
		Name              string  `json:"name"`
		Description       string  `json:"description"`
		Prompt            string  `json:"prompt"`
		MaxSteps          int64   `json:"max_steps"`
		CronSchedule      *string `json:"cron_schedule"`
		ScheduleEnabled   *bool   `json:"schedule_enabled"`
		ScheduleVariables *string `json:"schedule_variables"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Fetch current agent to preserve fields not being updated
	currentAgent, err := h.agentService.GetAgent(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// Build config with current values as defaults, overriding with provided values
	config := &services.AgentConfig{
		Name:              currentAgent.Name,
		Description:       currentAgent.Description,
		Prompt:            currentAgent.Prompt,
		MaxSteps:          currentAgent.MaxSteps,
		CronSchedule:      req.CronSchedule,
		ScheduleEnabled:   currentAgent.ScheduleEnabled,
		ScheduleVariables: req.ScheduleVariables,
	}

	// Override with provided values only if non-empty
	if req.Name != "" {
		config.Name = req.Name
	}
	if req.Description != "" {
		config.Description = req.Description
	}
	if req.Prompt != "" {
		config.Prompt = req.Prompt
	}
	if req.MaxSteps > 0 {
		config.MaxSteps = req.MaxSteps
	}
	if req.ScheduleEnabled != nil {
		config.ScheduleEnabled = *req.ScheduleEnabled
	}

	// Update agent fields if any changes were requested
	if req.Name != "" || req.Description != "" || req.Prompt != "" || req.MaxSteps > 0 || req.CronSchedule != nil || req.ScheduleEnabled != nil || req.ScheduleVariables != nil {

		_, err = h.agentService.UpdateAgent(c.Request.Context(), agentID, config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agent"})
			return
		}

		// Automatically export agent to file-based config after successful DB update
		if h.agentExportService != nil {
			if err := h.agentExportService.ExportAgentAfterSave(agentID); err != nil {
				// Log the error but don't fail the request - the agent was successfully updated in DB
				log.Printf("Failed to export agent %d after update: %v", agentID, err)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Agent updated successfully"})
}

func (h *APIHandlers) deleteAgent(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	err = h.agentService.DeleteAgent(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Agent deleted successfully"})
}

// getAgentPrompt returns the agent's prompt file content
func (h *APIHandlers) getAgentPrompt(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	// Get agent to verify it exists and get its environment
	agent, err := h.agentService.GetAgent(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// Get environment to find the prompt file
	environment, err := h.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent environment not found"})
		return
	}

	// Construct prompt file path using config helpers to respect workspace
	promptFilePath := config.GetAgentPromptPath(environment.Name, agent.Name)

	// Check if file exists
	if _, err := os.Stat(promptFilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Prompt file not found",
			"path":  promptFilePath,
		})
		return
	}

	// Read file content
	content, err := ioutil.ReadFile(promptFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read prompt file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content":  string(content),
		"path":     promptFilePath,
		"agent_id": agentID,
	})
}

// updateAgentPrompt updates the agent's prompt file content
func (h *APIHandlers) updateAgentPrompt(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get agent to verify it exists and get its environment
	agent, err := h.agentService.GetAgent(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// Get environment to find the prompt file
	environment, err := h.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent environment not found"})
		return
	}

	// Construct prompt file path using config helpers to respect workspace
	promptFilePath := config.GetAgentPromptPath(environment.Name, agent.Name)
	agentsDir := config.GetAgentsDir(environment.Name)

	// Create agents directory if it doesn't exist
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create agents directory"})
		return
	}

	// Write content to file
	if err := ioutil.WriteFile(promptFilePath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write prompt file"})
		return
	}

	// Extract the system prompt from the file content and update the database
	systemPrompt := extractSystemPromptFromContent(req.Content)
	if systemPrompt != "" {
		err = h.agentService.UpdateAgentPrompt(c.Request.Context(), agentID, systemPrompt)
		if err != nil {
			log.Printf("Failed to update agent prompt in database: %v", err)
			// Don't fail the request since file was saved successfully
		} else {
		}
	} else {
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Prompt file updated successfully",
		"path":         promptFilePath,
		"agent_id":     agentID,
		"environment":  environment.Name,
		"sync_command": fmt.Sprintf("stn sync %s", environment.Name),
	})
}

// extractSystemPromptFromContent extracts the system prompt content from the full prompt file
func extractSystemPromptFromContent(content string) string {
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	frontmatterEnded := false
	var promptLines []string

	for _, line := range lines {
		// Check for YAML frontmatter boundaries
		if strings.TrimSpace(line) == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			} else {
				frontmatterEnded = true
				continue
			}
		}

		// Skip frontmatter content
		if inFrontmatter && !frontmatterEnded {
			continue
		}

		// Once we're past the frontmatter, collect the prompt content
		if frontmatterEnded {
			promptLines = append(promptLines, line)
		}
	}

	// Join the prompt lines and clean up
	prompt := strings.Join(promptLines, "\n")
	prompt = strings.TrimSpace(prompt)

	return prompt
}
