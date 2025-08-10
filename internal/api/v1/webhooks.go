package v1

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"station/internal/auth"
	"station/pkg/models"
)

// registerWebhookRoutes registers webhook routes
func (h *APIHandlers) registerWebhookRoutes(group *gin.RouterGroup) {
	group.GET("", h.listWebhooks)
	group.POST("", h.createWebhook)
	group.GET("/:id", h.getWebhook)
	group.PUT("/:id", h.updateWebhook)
	group.DELETE("/:id", h.deleteWebhook)
	group.POST("/:id/enable", h.enableWebhook)
	group.POST("/:id/disable", h.disableWebhook)
}

// CreateWebhookRequest represents the request body for creating a webhook
type CreateWebhookRequest struct {
	Name           string            `json:"name" binding:"required"`
	URL            string            `json:"url" binding:"required,url"`
	Secret         string            `json:"secret"`
	Events         []string          `json:"events" binding:"required"`
	Headers        map[string]string `json:"headers"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	RetryAttempts  int               `json:"retry_attempts"`
}

// UpdateWebhookRequest represents the request body for updating a webhook
type UpdateWebhookRequest struct {
	Name           string            `json:"name"`
	URL            string            `json:"url" binding:"url"`
	Secret         string            `json:"secret"`
	Events         []string          `json:"events"`
	Headers        map[string]string `json:"headers"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	RetryAttempts  int               `json:"retry_attempts"`
}

// Webhook handlers

func (h *APIHandlers) createWebhook(c *gin.Context) {
	var req CreateWebhookRequest
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

	// Convert events to JSON
	eventsJSON, err := json.Marshal(req.Events)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid events format"})
		return
	}

	// Convert headers to JSON
	var headersJSON string
	if len(req.Headers) > 0 {
		headersBytes, err := json.Marshal(req.Headers)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid headers format"})
			return
		}
		headersJSON = string(headersBytes)
	}

	// Set defaults
	if req.TimeoutSeconds <= 0 {
		req.TimeoutSeconds = 30
	}
	if req.RetryAttempts <= 0 {
		req.RetryAttempts = 3
	}

	webhook := &models.Webhook{
		Name:           req.Name,
		URL:            req.URL,
		Secret:         req.Secret,
		Enabled:        true,
		Events:         string(eventsJSON),
		Headers:        headersJSON,
		TimeoutSeconds: req.TimeoutSeconds,
		RetryAttempts:  req.RetryAttempts,
		CreatedBy:      createdBy,
	}

	created, err := h.repos.Webhooks.Create(webhook)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create webhook"})
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *APIHandlers) listWebhooks(c *gin.Context) {
	webhooks, err := h.repos.Webhooks.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list webhooks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"webhooks": webhooks,
		"count":    len(webhooks),
	})
}

func (h *APIHandlers) getWebhook(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	webhook, err := h.repos.Webhooks.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
		return
	}

	c.JSON(http.StatusOK, webhook)
}

func (h *APIHandlers) updateWebhook(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	var req UpdateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing webhook
	existing, err := h.repos.Webhooks.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
		return
	}

	// Update fields if provided
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.URL != "" {
		existing.URL = req.URL
	}
	existing.Secret = req.Secret // Allow clearing secret

	if len(req.Events) > 0 {
		eventsJSON, err := json.Marshal(req.Events)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid events format"})
			return
		}
		existing.Events = string(eventsJSON)
	}

	if req.Headers != nil {
		if len(req.Headers) > 0 {
			headersBytes, err := json.Marshal(req.Headers)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid headers format"})
				return
			}
			existing.Headers = string(headersBytes)
		} else {
			existing.Headers = ""
		}
	}

	if req.TimeoutSeconds > 0 {
		existing.TimeoutSeconds = req.TimeoutSeconds
	}
	if req.RetryAttempts > 0 {
		existing.RetryAttempts = req.RetryAttempts
	}

	err = h.repos.Webhooks.Update(id, existing)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update webhook"})
		return
	}

	c.JSON(http.StatusOK, existing)
}

func (h *APIHandlers) deleteWebhook(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	err = h.repos.Webhooks.Delete(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete webhook"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Webhook deleted successfully"})
}

func (h *APIHandlers) enableWebhook(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	err = h.repos.Webhooks.Enable(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable webhook"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Webhook enabled successfully"})
}

func (h *APIHandlers) disableWebhook(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	err = h.repos.Webhooks.Disable(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable webhook"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Webhook disabled successfully"})
}

// Webhook delivery handlers

func (h *APIHandlers) listWebhookDeliveries(c *gin.Context) {
	webhookID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 200 {
			limit = parsedLimit
		}
	}

	deliveries, err := h.repos.WebhookDeliveries.ListByWebhook(webhookID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list webhook deliveries"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deliveries": deliveries,
		"count":      len(deliveries),
		"webhook_id": webhookID,
	})
}

func (h *APIHandlers) listAllWebhookDeliveries(c *gin.Context) {
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 200 {
			limit = parsedLimit
		}
	}

	deliveries, err := h.repos.WebhookDeliveries.List(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list webhook deliveries"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deliveries": deliveries,
		"count":      len(deliveries),
	})
}