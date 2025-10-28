package v1

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// registerSettingsRoutes registers settings routes
func (h *APIHandlers) registerSettingsRoutes(group *gin.RouterGroup) {
	group.GET("", h.listSettings)
	group.GET("/:key", h.getSetting)
	group.PUT("/:key", h.updateSetting)
	group.DELETE("/:key", h.deleteSetting)

	// Config file routes
	group.GET("/config/file", h.getConfigFile)
	group.PUT("/config/file", h.updateConfigFile)
}

// UpdateSettingRequest represents the request body for updating a setting
type UpdateSettingRequest struct {
	Value       string `json:"value" binding:"required"`
	Description string `json:"description"`
}

// Settings handlers

func (h *APIHandlers) listSettings(c *gin.Context) {
	settings, err := h.repos.Settings.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"settings": settings,
		"count":    len(settings),
	})
}

func (h *APIHandlers) getSetting(c *gin.Context) {
	key := c.Param("key")

	setting, err := h.repos.Settings.GetByKey(key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Setting not found"})
		return
	}

	c.JSON(http.StatusOK, setting)
}

func (h *APIHandlers) updateSetting(c *gin.Context) {
	key := c.Param("key")

	var req UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.repos.Settings.Set(key, req.Value, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update setting"})
		return
	}

	// Return the updated setting
	setting, err := h.repos.Settings.GetByKey(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve updated setting"})
		return
	}

	c.JSON(http.StatusOK, setting)
}

func (h *APIHandlers) deleteSetting(c *gin.Context) {
	key := c.Param("key")

	err := h.repos.Settings.Delete(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete setting"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Setting deleted successfully"})
}

// getConfigFile returns the config.yaml file content
func (h *APIHandlers) getConfigFile(c *gin.Context) {
	// Get config file path from viper
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Config file not found"})
		return
	}

	// Read file content
	content, err := os.ReadFile(configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read config file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content": string(content),
		"path":    configPath,
	})
}

// updateConfigFile updates the config.yaml file content
func (h *APIHandlers) updateConfigFile(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate YAML syntax
	var test map[string]interface{}
	if err := yaml.Unmarshal([]byte(req.Content), &test); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid YAML syntax: %v", err)})
		return
	}

	// Get config file path from viper
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Config file not found"})
		return
	}

	// Write config file
	if err := os.WriteFile(configPath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write config file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Config file updated successfully. Restart Station to apply changes.",
		"path":    configPath,
	})
}
