package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"station/internal/services"
)

// versionService is a package-level instance for version checking
var versionService = services.NewVersionService()

// registerVersionRoutes registers version-related API routes
func (h *APIHandlers) registerVersionRoutes(router *gin.RouterGroup) {
	router.GET("", h.getCurrentVersion)
	router.GET("/check", h.checkForUpdates)
	router.POST("/update", h.performUpdate)
}

// getCurrentVersion returns the current version information
// @Summary Get current version
// @Description Returns the current Station version and build information
// @Tags Version
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /version [get]
func (h *APIHandlers) getCurrentVersion(c *gin.Context) {
	info := versionService.GetCurrentVersion()
	c.JSON(http.StatusOK, info)
}

// checkForUpdates checks GitHub for available updates
// @Summary Check for updates
// @Description Checks GitHub releases for a newer version of Station
// @Tags Version
// @Produce json
// @Success 200 {object} services.VersionInfo
// @Failure 500 {object} map[string]string
// @Router /version/check [get]
func (h *APIHandlers) checkForUpdates(c *gin.Context) {
	info, err := versionService.CheckForUpdates(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, info)
}

// performUpdate runs the install script to update Station
// @Summary Perform update
// @Description Downloads and runs the install script to update Station to the latest version
// @Tags Version
// @Produce json
// @Success 200 {object} services.UpdateResult
// @Failure 500 {object} map[string]string
// @Router /version/update [post]
func (h *APIHandlers) performUpdate(c *gin.Context) {
	result, err := versionService.PerformUpdate(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if !result.Success {
		c.JSON(http.StatusOK, result)
		return
	}

	c.JSON(http.StatusOK, result)
}
