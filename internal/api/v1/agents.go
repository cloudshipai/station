package v1

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/afero"
	"station/internal/auth"
	"station/pkg/models"
	agent_bundle "station/pkg/agent-bundle"
	"station/pkg/agent-bundle/manager"
	"station/pkg/agent-bundle/validator"

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
	
	// Agent template installation endpoint
	group.POST("/templates/install", h.installAgentTemplate)
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

	// For direct execution (not queued), we'll use a simplified approach
	// In a production system, this would:
	// 1. Load agent configuration and tools
	// 2. Set up MCP connections
	// 3. Execute the task with the Claude API
	// 4. Stream results back to client

	// For now, return a placeholder response indicating the execution was received
	c.JSON(http.StatusAccepted, gin.H{
		"message":    "Agent execution initiated (direct mode)",
		"agent_id":   agentID,
		"agent_name": agent.Name,
		"task":       req.Task,
		"status":     "executing",
		"note":       "Direct execution is simplified - use queue endpoint for full execution with streaming",
	})
}

func (h *APIHandlers) queueAgent(c *gin.Context) {
	log.Printf("🔍 queueAgent: Handler called!")
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

	// Check if execution queue service is available
	log.Printf("🔍 queueAgent: executionQueueSvc is nil: %t", h.executionQueueSvc == nil)
	if h.executionQueueSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Execution queue service not available"})
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

	// Create metadata for the execution
	metadata := map[string]interface{}{
		"source":       "api_execution",
		"triggered_by": "cli",
		"api_endpoint": c.Request.URL.Path,
	}

	// Queue the execution
	runID, err := h.executionQueueSvc.QueueExecution(agentID, userID, req.Task, metadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to queue execution: %v", err)})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"agent_id": agentID,
		"task":     req.Task,
		"run_id":   runID,
		"status":   "queued",
		"message":  "Agent execution queued successfully",
	})
}

// DEBUG: Test if executionQueueSvc is available
func (h *APIHandlers) debugService(c *gin.Context) {
	log.Printf("🔍 debugService: Handler called!")
	c.JSON(http.StatusOK, gin.H{
		"executionQueueSvc_nil": h.executionQueueSvc == nil,
		"message": "Debug service check",
	})
}

func (h *APIHandlers) createAgent(c *gin.Context) {
	var req struct {
		Name          string   `json:"name" binding:"required"`
		Description   string   `json:"description" binding:"required"`
		Prompt        string   `json:"prompt" binding:"required"`
		EnvironmentID int64    `json:"environment_id" binding:"required"`
		MaxSteps      int64    `json:"max_steps"`
		AssignedTools []string `json:"assigned_tools"`
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

	// Validate environment exists
	environment, err := h.repos.Environments.GetByID(req.EnvironmentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Environment not found"})
		return
	}

	// Create agent directly in database (simplified implementation)
	// In a production system, you might want to:
	// 1. Validate tool assignments
	// 2. Check user permissions for the environment
	// 3. Create associated tool mappings

	// Create the agent using the repository
	agent, err := h.repos.Agents.Create(
		req.Name,
		req.Description,
		req.Prompt,
		req.MaxSteps,
		req.EnvironmentID,
		createdBy,
		nil,   // input_schema - not set in API v1
		nil,   // cronSchedule - no schedule initially
		false, // scheduleEnabled - disabled initially
	)
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
	if err := h.agentExportService.ExportAgentAfterSave(agent.ID); err != nil {
		// Log the error but don't fail the request - the agent was successfully created in DB
		log.Printf("Failed to export agent %d after creation: %v", agent.ID, err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Agent created successfully",
		"agent_id":    agent.ID,
		"agent_name":  agent.Name,
		"environment": environment.Name,
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

	// Get agent with tools using the new query
	rows, err := h.repos.Agents.GetAgentWithTools(context.Background(), agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get agent details"})
		return
	}

	if len(rows) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// Structure the response data
	firstRow := rows[0]
	
	// Extract agent info (same for all rows)
	agent := gin.H{
		"id":               firstRow.AgentID,
		"name":            firstRow.AgentName,
		"description":     firstRow.AgentDescription,
		"prompt":          firstRow.AgentPrompt,
		"max_steps":       firstRow.AgentMaxSteps,
		"environment_id":  firstRow.AgentEnvironmentID,
		"created_by":      firstRow.AgentCreatedBy,
		"is_scheduled":    firstRow.AgentIsScheduled.Bool,
		"schedule_enabled": firstRow.AgentScheduleEnabled.Bool,
		"input_schema":    firstRow.AgentInputSchema.String,
		"created_at":      firstRow.AgentCreatedAt,
		"updated_at":      firstRow.AgentUpdatedAt,
	}

	// Group tools by MCP server
	mcpServers := make(map[int64]gin.H)
	
	for _, row := range rows {
		// Skip rows without tools
		if !row.McpServerID.Valid {
			continue
		}
		
		serverID := row.McpServerID.Int64
		
		// Initialize MCP server if not exists
		if _, exists := mcpServers[serverID]; !exists {
			mcpServers[serverID] = gin.H{
				"id":    serverID,
				"name":  row.McpServerName.String,
				"tools": []gin.H{},
			}
		}
		
		// Add tool to server
		if row.ToolID.Valid {
			tool := gin.H{
				"id":           row.ToolID.Int64,
				"name":         row.ToolName.String,
				"description":  row.ToolDescription.String,
				"input_schema": row.ToolInputSchema.String,
			}
			
			server := mcpServers[serverID]
			tools := server["tools"].([]gin.H)
			server["tools"] = append(tools, tool)
			mcpServers[serverID] = server
		}
	}

	// Convert map to slice
	mcpServersList := make([]gin.H, 0, len(mcpServers))
	for _, server := range mcpServers {
		mcpServersList = append(mcpServersList, server)
	}

	c.JSON(http.StatusOK, gin.H{
		"agent":       agent,
		"mcp_servers": mcpServersList,
	})
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
		err = h.repos.Agents.Update(agentID, req.Name, req.Description, req.Prompt, req.MaxSteps, nil, nil, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agent"})
			return
		}

		// Automatically export agent to file-based config after successful DB update
		if err := h.agentExportService.ExportAgentAfterSave(agentID); err != nil {
			// Log the error but don't fail the request - the agent was successfully updated in DB
			log.Printf("Failed to export agent %d after update: %v", agentID, err)
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

// installAgentTemplate installs an agent from a template bundle
func (h *APIHandlers) installAgentTemplate(c *gin.Context) {
	var req struct {
		BundlePath  string                 `json:"bundle_path" binding:"required"`
		Environment string                 `json:"environment"`
		Variables   map[string]interface{} `json:"variables"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set default environment if not provided
	if req.Environment == "" {
		req.Environment = "default"
	}

	// Validate that environment exists
	envs, err := h.repos.Environments.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list environments"})
		return
	}

	var foundEnv *models.Environment
	for _, env := range envs {
		if env.Name == req.Environment {
			foundEnv = env
			break
		}
	}

	if foundEnv == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Environment '%s' not found", req.Environment)})
		return
	}

	// Create manager with dependencies (using mock for now - TODO: implement real resolver)
	fs := afero.NewOsFs()
	bundleValidator := validator.New(fs)
	mockResolver := &MockResolver{}
	bundleManager := manager.New(fs, bundleValidator, mockResolver)

	// Install the bundle
	result, err := bundleManager.Install(req.BundlePath, req.Environment, req.Variables)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to install agent template",
			"details": err.Error(),
		})
		return
	}

	if !result.Success {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Agent template installation failed",
			"details": result.Error,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":        "Agent template installed successfully",
		"agent_id":       result.AgentID,
		"agent_name":     result.AgentName,
		"environment":    result.Environment,
		"tools_installed": result.ToolsInstalled,
		"mcp_bundles":    result.MCPBundles,
	})
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

	c.JSON(http.StatusOK, gin.H{
		"message":     "Prompt file updated successfully",
		"path":        promptFilePath,
		"agent_id":    agentID,
		"environment": environment.Name,
		"sync_command": fmt.Sprintf("stn sync %s", environment.Name),
	})
}

// MockResolver for template installation API (reused from CLI handler)
type MockResolver struct{}

func (r *MockResolver) Resolve(ctx context.Context, deps []agent_bundle.MCPBundleDependency, env string) (*agent_bundle.ResolutionResult, error) {
	return &agent_bundle.ResolutionResult{
		Success: true,
		ResolvedBundles: []agent_bundle.MCPBundleRef{
			{Name: "filesystem-tools", Version: "1.0.0", Source: "registry"},
		},
		MissingBundles: []agent_bundle.MCPBundleDependency{},
		Conflicts:      []agent_bundle.ToolConflict{},
		InstallOrder:   []string{"filesystem-tools"},
	}, nil
}

func (r *MockResolver) InstallMCPBundles(ctx context.Context, bundles []agent_bundle.MCPBundleRef, env string) error {
	return nil
}

func (r *MockResolver) ValidateToolAvailability(ctx context.Context, tools []agent_bundle.ToolRequirement, env string) error {
	return nil
}

func (r *MockResolver) ResolveConflicts(conflicts []agent_bundle.ToolConflict) (*agent_bundle.ConflictResolution, error) {
	return &agent_bundle.ConflictResolution{
		Strategy:    "auto",
		Resolutions: make(map[string]string),
		Warnings:    []string{},
	}, nil
}
