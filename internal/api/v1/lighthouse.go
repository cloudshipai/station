package v1

import (
	"net/http"

	"station/internal/lighthouse"

	"github.com/gin-gonic/gin"
)

// LighthouseStatusHandler returns the current lighthouse status
func (h *APIHandlers) LighthouseStatusHandler(c *gin.Context) {
	status := lighthouse.GetStatus()

	// Add summary fields for UI convenience
	response := map[string]interface{}{
		"status":          status,
		"is_healthy":      status.Connected && status.Registered,
		"has_error":       status.LastError != "",
		"summary_message": getLighthouseSummaryMessage(status),
	}

	c.JSON(http.StatusOK, response)
}

// getLighthouseSummaryMessage provides a human-readable status message
func getLighthouseSummaryMessage(status lighthouse.LighthouseStatus) string {
	if !status.Connected {
		return "CloudShip not connected"
	}

	if !status.Registered {
		return "CloudShip connected but not authenticated"
	}

	if status.LastError != "" {
		return "CloudShip error: " + status.LastError
	}

	if status.TelemetrySent > 0 {
		return "CloudShip connected and active"
	}

	return "CloudShip connected"
}
