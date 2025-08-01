package v1

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"station/internal/auth"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/models"
)

// APIHandlers contains all the API handlers and their dependencies
type APIHandlers struct {
	repos             *repositories.Repositories
	mcpConfigService  *services.MCPConfigService
	toolDiscoveryService *services.ToolDiscoveryService
	genkitService     *services.GenkitService
	localMode         bool
}

// NewAPIHandlers creates a new API handlers instance
func NewAPIHandlers(
	repos *repositories.Repositories,
	mcpConfigService *services.MCPConfigService,
	toolDiscoveryService *services.ToolDiscoveryService,
	genkitService *services.GenkitService,
	localMode bool,
) *APIHandlers {
	return &APIHandlers{
		repos:                repos,
		mcpConfigService:     mcpConfigService,
		toolDiscoveryService: toolDiscoveryService,
		genkitService:        genkitService,
		localMode:            localMode,
	}
}

// RegisterRoutes registers all v1 API routes
func (h *APIHandlers) RegisterRoutes(router *gin.RouterGroup) {
	// Create auth middleware
	authMiddleware := auth.NewAuthMiddleware(h.repos)
	
	// In server mode, all routes require authentication
	if !h.localMode {
		router.Use(authMiddleware.Authenticate())
	}
	
	// Environment routes
	envGroup := router.Group("/environments")
	// In server mode, only admins can manage environments  
	if !h.localMode {
		envGroup.Use(h.requireAdminInServerMode())
	}
	envGroup.GET("", h.listEnvironments)
	envGroup.POST("", h.createEnvironment)
	envGroup.GET("/:env_id", h.getEnvironment)
	envGroup.PUT("/:env_id", h.updateEnvironment)
	envGroup.DELETE("/:env_id", h.deleteEnvironment)

	// MCP Config routes (nested under environments)
	mcpGroup := envGroup.Group("/:env_id/mcp-configs")
	// Inherits admin-only restriction from envGroup
	mcpGroup.GET("", h.listMCPConfigs)
	mcpGroup.POST("", h.uploadMCPConfig)
	mcpGroup.GET("/latest", h.getLatestMCPConfig)
	mcpGroup.GET("/:config_id", h.getMCPConfig)
	mcpGroup.DELETE("/:config_id", h.deleteMCPConfig)

	// Tools routes (nested under environments)
	toolsGroup := envGroup.Group("/:env_id/tools")
	// Inherits admin-only restriction from envGroup
	toolsGroup.GET("", h.listTools)
	
	// Agent routes - accessible to regular users in server mode
	agentGroup := router.Group("/agents")
	agentGroup.GET("", h.listAgents)           // Users can list agents
	agentGroup.POST("/:id/execute", h.callAgent) // Users can call agents
	
	// Admin-only agent management routes
	agentAdminGroup := router.Group("/agents")
	if !h.localMode {
		agentAdminGroup.Use(h.requireAdminInServerMode())
	}
	agentAdminGroup.POST("", h.createAgent)
	agentAdminGroup.GET("/:id", h.getAgent)
	agentAdminGroup.PUT("/:id", h.updateAgent)
	agentAdminGroup.DELETE("/:id", h.deleteAgent)
	
	// Agent runs routes - accessible to regular users in server mode
	runsGroup := router.Group("/runs")
	runsGroup.GET("", h.listRuns)              // Users can list runs
	runsGroup.GET("/:id", h.getRun)            // Users can get run details
	runsGroup.GET("/agent/:agent_id", h.listRunsByAgent) // Users can list runs by agent
}

// requireAdminInServerMode is a middleware that requires admin privileges in server mode
func (h *APIHandlers) requireAdminInServerMode() gin.HandlerFunc {
	return func(c *gin.Context) {
		// In local mode, no admin check needed
		if h.localMode {
			c.Next()
			return
		}
		
		// Get user from context (should be set by auth middleware)
		user, exists := auth.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}
		
		// Check if user is admin
		if !user.IsAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// Agent handlers

func (h *APIHandlers) listAgents(c *gin.Context) {
	agents, err := h.repos.Agents.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list agents"})
		return
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

	// Execute agent using the genkit service
	if h.genkitService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Agent execution service not available"})
		return
	}

	response, err := h.genkitService.ExecuteAgent(c.Request.Context(), agentID, req.Task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to execute agent: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agent_id": agentID,
		"task":     req.Task,
		"response": response.Content,
		"success":  true,
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
		req.MaxSteps = 5
	}

	// Create agent using genkit service
	if h.genkitService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Agent service not available"})
		return
	}

	agentConfig := &services.AgentConfig{
		EnvironmentID: req.EnvironmentID,
		Name:          req.Name,
		Description:   req.Description,
		Prompt:        req.Prompt,
		AssignedTools: req.AssignedTools,
		MaxSteps:      req.MaxSteps,
		CreatedBy:     createdBy,
	}

	agent, err := h.genkitService.CreateAgent(c.Request.Context(), agentConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create agent: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"agent": agent})
}

func (h *APIHandlers) getAgent(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	agent, err := h.repos.Agents.GetByID(agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agent": agent})
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
		err = h.repos.Agents.Update(agentID, req.Name, req.Description, req.Prompt, req.MaxSteps, nil, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agent"})
			return
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

	err = h.repos.Agents.Delete(agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Agent deleted successfully"})
}

// Agent runs handlers

func (h *APIHandlers) listRuns(c *gin.Context) {
	// Get limit parameter, default to 50
	limit := int64(50)
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.ParseInt(limitStr, 10, 64); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	runs, err := h.repos.AgentRuns.ListRecent(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list runs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"runs":  runs,
		"count": len(runs),
		"limit": limit,
	})
}

func (h *APIHandlers) getRun(c *gin.Context) {
	runID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid run ID"})
		return
	}

	run, err := h.repos.AgentRuns.GetByIDWithDetails(runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Run not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"run": run})
}

func (h *APIHandlers) listRunsByAgent(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("agent_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	runs, err := h.repos.AgentRuns.ListByAgent(agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list runs for agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"runs":     runs,
		"count":    len(runs),
		"agent_id": agentID,
	})
}

// Environment handlers

func (h *APIHandlers) listEnvironments(c *gin.Context) {
	environments, err := h.repos.Environments.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list environments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"environments": environments,
		"count":        len(environments),
	})
}

func (h *APIHandlers) createEnvironment(c *gin.Context) {
	var req struct {
		Name        string  `json:"name" binding:"required"`
		Description *string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	env, err := h.repos.Environments.Create(req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create environment"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"environment": env})
}

func (h *APIHandlers) getEnvironment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	env, err := h.repos.Environments.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"environment": env})
}

func (h *APIHandlers) updateEnvironment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.repos.Environments.Update(id, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update environment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Environment updated successfully"})
}

func (h *APIHandlers) deleteEnvironment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	err = h.repos.Environments.Delete(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete environment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Environment deleted successfully"})
}

// MCP Config handlers

func (h *APIHandlers) listMCPConfigs(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	configs, err := h.repos.MCPConfigs.ListByEnvironment(envID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list MCP configurations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"configs": configs,
		"count":   len(configs),
	})
}

func (h *APIHandlers) uploadMCPConfig(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	var req struct {
		Name    string                            `json:"name" binding:"required"`
		Servers map[string]models.MCPServerConfig `json:"servers" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create config data
	configData := &models.MCPConfigData{
		Name:    req.Name,
		Servers: req.Servers,
	}

	// Upload and encrypt config
	savedConfig, err := h.mcpConfigService.UploadConfig(envID, configData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save MCP configuration"})
		return
	}

	// Start tool discovery in background
	go func() {
		if h.toolDiscoveryService != nil {
			h.toolDiscoveryService.ReplaceToolsWithTransaction(envID, req.Name)
		}
	}()

	// Reinitialize MCP manager if available
	if h.genkitService != nil {
		go h.genkitService.ReinitializeMCP(c.Request.Context())
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "MCP configuration uploaded successfully",
		"config":  savedConfig,
	})
}

func (h *APIHandlers) getLatestMCPConfig(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	config, err := h.repos.MCPConfigs.GetLatest(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No MCP configuration found"})
		return
	}

	// Decrypt config for response
	decryptedConfig, err := h.mcpConfigService.GetDecryptedConfig(config.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decrypt configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config":           config,
		"decrypted_config": decryptedConfig,
	})
}

func (h *APIHandlers) getMCPConfig(c *gin.Context) {
	configID, err := strconv.ParseInt(c.Param("config_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
		return
	}

	config, err := h.repos.MCPConfigs.GetByID(configID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP configuration not found"})
		return
	}

	// Decrypt config for response
	decryptedConfig, err := h.mcpConfigService.GetDecryptedConfig(configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decrypt configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config":           config,
		"decrypted_config": decryptedConfig,
	})
}

func (h *APIHandlers) deleteMCPConfig(c *gin.Context) {
	configID, err := strconv.ParseInt(c.Param("config_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
		return
	}

	err = h.repos.MCPConfigs.Delete(configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete MCP configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "MCP configuration deleted successfully"})
}

// Tools handlers

func (h *APIHandlers) listTools(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	// Get filter parameter
	filter := c.Query("filter")

	tools, err := h.repos.MCPTools.GetByEnvironmentID(envID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tools"})
		return
	}

	// Convert to MCPToolWithDetails - for now just create a simple version
	toolsWithDetails := make([]*models.MCPToolWithDetails, len(tools))
	for i, tool := range tools {
		toolsWithDetails[i] = &models.MCPToolWithDetails{
			MCPTool:           *tool,
			ConfigName:        "unknown", // Would need to join with config
			ConfigVersion:     1,         // Would need to join with config
			ServerName:        "unknown", // Would need to join with server
			EnvironmentName:   "unknown", // Would need to join with environment
		}
	}

	// Apply filter if provided
	if filter != "" {
		filteredTools := make([]*models.MCPToolWithDetails, 0)
		filterLower := strings.ToLower(filter)
		
		for _, tool := range toolsWithDetails {
			if strings.Contains(strings.ToLower(tool.Name), filterLower) ||
				strings.Contains(strings.ToLower(tool.Description), filterLower) {
				filteredTools = append(filteredTools, tool)
			}
		}
		toolsWithDetails = filteredTools
	}

	c.JSON(http.StatusOK, gin.H{
		"tools": toolsWithDetails,
		"count": len(toolsWithDetails),
		"filter": filter,
	})
}