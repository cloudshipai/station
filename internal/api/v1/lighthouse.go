package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"station/internal/config"
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

// CloudShipStatusResponse represents the response for CloudShip API status
type CloudShipStatusResponse struct {
	Authenticated bool   `json:"authenticated"`
	HasAPIKey     bool   `json:"has_api_key"`
	APIKeyMasked  string `json:"api_key_masked,omitempty"`
	APIURL        string `json:"api_url"`
	BundleCount   int    `json:"bundle_count"`
	Organization  string `json:"organization,omitempty"`
	Error         string `json:"error,omitempty"`
}

// CloudShipStatusHandler returns the CloudShip API key authentication status
func (h *APIHandlers) CloudShipStatusHandler(c *gin.Context) {
	cfg, err := config.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, CloudShipStatusResponse{
			Authenticated: false,
			Error:         "Failed to load config: " + err.Error(),
		})
		return
	}

	response := CloudShipStatusResponse{
		HasAPIKey: cfg.CloudShip.APIKey != "",
		APIURL:    cfg.CloudShip.APIURL,
	}

	if response.APIURL == "" {
		response.APIURL = "https://api.cloudshipai.com"
	}

	// Mask the API key for display
	if cfg.CloudShip.APIKey != "" {
		key := cfg.CloudShip.APIKey
		if len(key) > 8 {
			response.APIKeyMasked = key[:4] + "..." + key[len(key)-4:]
		} else {
			response.APIKeyMasked = "****"
		}
	}

	// If we have an API key, try to validate it by fetching bundles
	if cfg.CloudShip.APIKey != "" {
		apiURL := response.APIURL
		listURL := fmt.Sprintf("%s/api/public/bundles/", strings.TrimSuffix(apiURL, "/"))

		req, err := http.NewRequest("GET", listURL, nil)
		if err != nil {
			response.Error = "Failed to create request: " + err.Error()
			c.JSON(http.StatusOK, response)
			return
		}

		// Use API key for authentication
		req.Header.Set("Authorization", "Bearer "+cfg.CloudShip.APIKey)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			response.Error = "Failed to connect to CloudShip: " + err.Error()
			c.JSON(http.StatusOK, response)
			return
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			response.Authenticated = true

			// Try to parse bundle count and organization
			var bundleResp struct {
				Bundles []struct {
					Organization string `json:"organization"`
				} `json:"bundles"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&bundleResp); err == nil {
				response.BundleCount = len(bundleResp.Bundles)
				if len(bundleResp.Bundles) > 0 {
					response.Organization = bundleResp.Bundles[0].Organization
				}
			}
		} else if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			response.Authenticated = false
			response.Error = "API key is invalid or expired"
		} else {
			response.Error = fmt.Sprintf("CloudShip returned HTTP %d", resp.StatusCode)
		}
	}

	c.JSON(http.StatusOK, response)
}
