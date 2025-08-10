package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// registerSettingsRoutes registers settings routes
func (h *APIHandlers) registerSettingsRoutes(group *gin.RouterGroup) {
	group.GET("", h.listSettings)
	group.GET("/:key", h.getSetting)
	group.PUT("/:key", h.updateSetting)
	group.DELETE("/:key", h.deleteSetting)
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