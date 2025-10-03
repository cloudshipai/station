package v1

import (
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
)

// checkShipInstalled checks if Ship CLI is installed
func (h *APIHandlers) checkShipInstalled(c *gin.Context) {
	// Try to run 'ship --version' to check if Ship is installed
	cmd := exec.Command("ship", "--version")
	err := cmd.Run()

	installed := err == nil

	c.JSON(http.StatusOK, gin.H{
		"installed": installed,
	})
}

// registerShipRoutes registers Ship-related routes
func (h *APIHandlers) registerShipRoutes(router *gin.RouterGroup) {
	router.GET("/installed", h.checkShipInstalled)
}
