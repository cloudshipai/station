package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"station/internal/services"
)

// registerMCPManagementRoutes registers MCP server management routes
func (h *APIHandlers) registerMCPManagementRoutes(envGroup *gin.RouterGroup) {
	// MCP server management routes
	mcpGroup := envGroup.Group("/:env_id/mcp-servers")
	mcpGroup.GET("", h.listMCPServersForEnvironment)
	mcpGroup.POST("", h.addMCPServerToEnvironment)
	mcpGroup.PUT("/:server_name", h.updateMCPServerInEnvironment)
	mcpGroup.DELETE("/:server_name", h.deleteMCPServerFromEnvironment)
	mcpGroup.GET("/:server_name/config", h.getServerConfig)
	mcpGroup.PUT("/:server_name/config", h.updateServerConfig)

	// Raw MCP config routes
	configGroup := envGroup.Group("/:env_id/config")
	configGroup.GET("/raw", h.getRawMCPConfig)
	configGroup.PUT("/raw", h.updateRawMCPConfig)
	configGroup.GET("/files", h.getEnvironmentFileConfig)
	configGroup.PUT("/files/:filename", h.updateEnvironmentFileConfig)
}

// listMCPServersForEnvironment lists all MCP servers for an environment
func (h *APIHandlers) listMCPServersForEnvironment(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	// Get environment to get the name
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	mcpService := services.NewMCPServerManagementService(h.repos)
	servers, err := mcpService.GetMCPServersForEnvironment(env.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get MCP servers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"environment": env.Name,
		"servers":     servers,
		"count":       len(servers),
	})
}

// addMCPServerToEnvironment adds an MCP server to an environment
func (h *APIHandlers) addMCPServerToEnvironment(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	var req struct {
		ServerName string                            `json:"server_name" binding:"required"`
		Config     services.MCPServerConfig          `json:"config" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get environment to get the name
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	mcpService := services.NewMCPServerManagementService(h.repos)
	result := mcpService.AddMCPServerToEnvironment(env.Name, req.ServerName, req.Config)

	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to add MCP server",
			"result": result,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": result.Message,
		"result":  result,
	})
}

// updateMCPServerInEnvironment updates an MCP server in an environment
func (h *APIHandlers) updateMCPServerInEnvironment(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	serverName := c.Param("server_name")
	if serverName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Server name is required"})
		return
	}

	var req struct {
		Config services.MCPServerConfig `json:"config" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get environment to get the name
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	mcpService := services.NewMCPServerManagementService(h.repos)
	result := mcpService.UpdateMCPServerInEnvironment(env.Name, serverName, req.Config)

	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to update MCP server",
			"result": result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": result.Message,
		"result":  result,
	})
}

// deleteMCPServerFromEnvironment deletes an MCP server from an environment
func (h *APIHandlers) deleteMCPServerFromEnvironment(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	serverName := c.Param("server_name")
	if serverName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Server name is required"})
		return
	}

	// Get environment to get the name
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	mcpService := services.NewMCPServerManagementService(h.repos)
	result := mcpService.DeleteMCPServerFromEnvironment(env.Name, serverName)

	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to delete MCP server",
			"result": result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": result.Message,
		"result":  result,
	})
}

// getRawMCPConfig gets the raw template.json content for an environment
func (h *APIHandlers) getRawMCPConfig(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	// Get environment to get the name
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	mcpService := services.NewMCPServerManagementService(h.repos)
	content, err := mcpService.GetRawMCPConfig(env.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get raw MCP config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"environment": env.Name,
		"content":     content,
	})
}

// updateRawMCPConfig updates the raw template.json content for an environment
func (h *APIHandlers) updateRawMCPConfig(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get environment to get the name
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	mcpService := services.NewMCPServerManagementService(h.repos)
	err = mcpService.UpdateRawMCPConfig(env.Name, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update raw MCP config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Raw MCP config updated successfully",
		"environment": env.Name,
	})
}

// getEnvironmentFileConfig gets all file-based config for an environment
func (h *APIHandlers) getEnvironmentFileConfig(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	// Get environment to get the name
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	envService := services.NewEnvironmentManagementService(h.repos)
	config, err := envService.GetEnvironmentFileConfig(env.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get environment file config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"environment": env.Name,
		"config":      config,
	})
}

// updateEnvironmentFileConfig updates a specific file in environment config
func (h *APIHandlers) updateEnvironmentFileConfig(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filename is required"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get environment to get the name
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	envService := services.NewEnvironmentManagementService(h.repos)
	err = envService.UpdateEnvironmentFileConfig(env.Name, filename, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update environment file config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Environment file config updated successfully",
		"environment": env.Name,
		"filename":    filename,
	})
}

// getServerConfig gets the configuration for a specific MCP server
func (h *APIHandlers) getServerConfig(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	serverName := c.Param("server_name")
	if serverName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Server name is required"})
		return
	}

	// Get environment to get the name
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	mcpService := services.NewMCPServerManagementService(h.repos)
	servers, err := mcpService.GetMCPServersForEnvironment(env.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get MCP servers"})
		return
	}

	// Check if server exists
	serverConfig, exists := servers[serverName]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"server_name": serverName,
		"environment": env.Name,
		"config":      serverConfig,
	})
}

// updateServerConfig updates the configuration for a specific MCP server
func (h *APIHandlers) updateServerConfig(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	serverName := c.Param("server_name")
	if serverName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Server name is required"})
		return
	}

	var req struct {
		Config services.MCPServerConfig `json:"config" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get environment to get the name
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	mcpService := services.NewMCPServerManagementService(h.repos)
	result := mcpService.UpdateMCPServerInEnvironment(env.Name, serverName, req.Config)

	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to update MCP server config",
			"result": result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": result.Message,
		"result":  result,
	})
}