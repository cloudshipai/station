package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"station/pkg/models"
)

// registerMCPConfigRoutes registers MCP configuration routes
func (h *APIHandlers) registerMCPConfigRoutes(group *gin.RouterGroup) {
	group.GET("", h.listMCPConfigs)
	group.POST("", h.uploadMCPConfig)
	group.GET("/latest", h.getLatestMCPConfig)
	group.GET("/:config_id", h.getMCPConfig)
	group.DELETE("/:config_id", h.deleteMCPConfig)
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