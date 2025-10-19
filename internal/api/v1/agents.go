package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"station/internal/auth"
	"station/internal/config"
	"station/internal/services"
	"station/pkg/models"

	"github.com/gin-gonic/gin"
)

// registerAgentAdminRoutes registers admin-only agent management routes
func (h *APIHandlers) registerAgentAdminRoutes(group *gin.RouterGroup) {
	group.POST("", h.createAgent)
	group.GET("/:id", h.getAgent)
	group.GET("/:id/details", h.getAgentWithTools)  // New endpoint for detailed view
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

	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"count":  len(agents),
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
		ctx := context.Background()

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
			// Update run with completion response and status
			finalResponse := ""
			if response != nil {
				finalResponse = response.Content
			}
			// Mark run as completed with final response
			completedAt := time.Now()
			h.repos.AgentRuns.UpdateCompletion(ctx, agentRun.ID, finalResponse, 0, nil, nil, "completed", &completedAt)
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
		Task string `json:"task" binding:"required"`
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
		"", // final_response (will be updated)
		0,  // steps_taken
		nil, // tool_calls 
		nil, // execution_steps
		"running", // status
		nil, // completed_at
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
		"message":     "Agent created successfully",
		"agent_id":    agent.ID,
		"agent_name":  agent.Name,
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

	// Parse YAML frontmatter to extract tools
	lines := strings.Split(string(content), "\n")
	inFrontmatter := false
	var tools []string
	
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
		
		// Look for tools section
		if strings.HasPrefix(line, "tools:") {
			continue
		}
		
		// Parse tool entries (- "toolname" or - "__toolname")
		if strings.HasPrefix(line, "- \"") && strings.HasSuffix(line, "\"") {
			tool := strings.Trim(line[2:], "\"")
			tools = append(tools, tool)
		} else if strings.HasPrefix(line, "- \"__") && strings.HasSuffix(line, "\"") {
			tool := strings.Trim(line[2:], "\"")
			tools = append(tools, tool)
		} else if strings.HasPrefix(line, "- __") {
			// Handle unquoted tool names like: - "__toolname"
			tool := strings.TrimSpace(line[2:])
			if strings.HasPrefix(tool, "\"") && strings.HasSuffix(tool, "\"") {
				tool = strings.Trim(tool, "\"")
			}
			tools = append(tools, tool)
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
		Name        string `json:"name"`
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
		MaxSteps    int64  `json:"max_steps"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update agent fields if provided
	if req.Name != "" || req.Description != "" || req.Prompt != "" || req.MaxSteps > 0 {
		config := &services.AgentConfig{
			Name:        req.Name,
			Description: req.Description,
			Prompt:      req.Prompt,
			MaxSteps:    req.MaxSteps,
		}
		
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

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get home directory"})
		return
	}

	// Construct prompt file path
	promptFileName := fmt.Sprintf("%s.prompt", agent.Name)
	promptFilePath := filepath.Join(homeDir, ".config", "station", "environments", environment.Name, "agents", promptFileName)

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

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get home directory"})
		return
	}

	// Construct prompt file path
	promptFileName := fmt.Sprintf("%s.prompt", agent.Name)
	agentsDir := filepath.Join(homeDir, ".config", "station", "environments", environment.Name, "agents")
	promptFilePath := filepath.Join(agentsDir, promptFileName)

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
		"message":     "Prompt file updated successfully",
		"path":        promptFilePath,
		"agent_id":    agentID,
		"environment": environment.Name,
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

