package v1

import (
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
)

// checkShipInstalled checks if Ship CLI is installed and returns version if available
func (h *APIHandlers) checkShipInstalled(c *gin.Context) {
	// Try to run 'ship help' to check if Ship is installed
	cmd := exec.Command("ship", "help")
	err := cmd.Run()

	installed := err == nil
	version := ""

	if installed {
		// Try to get version with 'ship version' command
		versionCmd := exec.Command("ship", "version")
		output, err := versionCmd.CombinedOutput()
		if err == nil {
			version = string(output)
		} else {
			// If 'ship version' doesn't work, try --version
			versionCmd = exec.Command("ship", "--version")
			output, err = versionCmd.CombinedOutput()
			if err == nil {
				version = string(output)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"installed": installed,
		"version":   version,
	})
}

// registerShipRoutes registers Ship-related routes
func (h *APIHandlers) registerShipRoutes(router *gin.RouterGroup) {
	router.GET("/installed", h.checkShipInstalled)
}
