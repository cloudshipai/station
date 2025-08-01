package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"station/internal/auth"
	"station/internal/db/repositories"
	"station/internal/services"
)

// APIHandlers contains all the API handlers and their dependencies
type APIHandlers struct {
	repos                *repositories.Repositories
	mcpConfigService     *services.MCPConfigService
	toolDiscoveryService *services.ToolDiscoveryService
	genkitService        *services.GenkitService
	webhookService       *services.WebhookService
	localMode            bool
}

// NewAPIHandlers creates a new API handlers instance
func NewAPIHandlers(
	repos *repositories.Repositories,
	mcpConfigService *services.MCPConfigService,
	toolDiscoveryService *services.ToolDiscoveryService,
	genkitService *services.GenkitService,
	webhookService *services.WebhookService,
	localMode bool,
) *APIHandlers {
	return &APIHandlers{
		repos:                repos,
		mcpConfigService:     mcpConfigService,
		toolDiscoveryService: toolDiscoveryService,
		genkitService:        genkitService,
		webhookService:       webhookService,
		localMode:            localMode,
	}
}

// RegisterRoutes registers all v1 API routes
func (h *APIHandlers) RegisterRoutes(router *gin.RouterGroup) {
	// Create auth middleware
	authMiddleware := auth.NewAuthMiddleware(h.repos)
	
	// In server mode, all routes require authentication
	if !h.localMode {
		router.Use(authMiddleware.Authenticate())
	}
	
	// Environment routes
	envGroup := router.Group("/environments")
	// In server mode, only admins can manage environments  
	if !h.localMode {
		envGroup.Use(h.requireAdminInServerMode())
	}
	h.registerEnvironmentRoutes(envGroup)

	// MCP Config routes (nested under environments)
	mcpGroup := envGroup.Group("/:env_id/mcp-configs")
	// Inherits admin-only restriction from envGroup
	h.registerMCPConfigRoutes(mcpGroup)

	// Tools routes (nested under environments)
	toolsGroup := envGroup.Group("/:env_id/tools")
	// Inherits admin-only restriction from envGroup
	h.registerToolsRoutes(toolsGroup)
	
	// Agent routes - accessible to regular users in server mode
	agentGroup := router.Group("/agents")
	agentGroup.GET("", h.listAgents)           // Users can list agents
	agentGroup.POST("/:id/execute", h.callAgent) // Users can call agents
	
	// Admin-only agent management routes
	agentAdminGroup := router.Group("/agents")
	if !h.localMode {
		agentAdminGroup.Use(h.requireAdminInServerMode())
	}
	h.registerAgentAdminRoutes(agentAdminGroup)
	
	// Agent runs routes - accessible to regular users in server mode
	runsGroup := router.Group("/runs")
	h.registerAgentRunRoutes(runsGroup)

	// Settings routes - admin only
	settingsGroup := router.Group("/settings")
	if !h.localMode {
		settingsGroup.Use(h.requireAdminInServerMode())
	}
	h.registerSettingsRoutes(settingsGroup)

	// Webhook routes - admin only
	webhookGroup := router.Group("/webhooks")
	if !h.localMode {
		webhookGroup.Use(h.requireAdminInServerMode())
	}
	h.registerWebhookRoutes(webhookGroup)

	// Webhook deliveries routes - admin only
	deliveriesGroup := webhookGroup.Group("/:id/deliveries")
	deliveriesGroup.GET("", h.listWebhookDeliveries)
	
	// All webhook deliveries route
	allDeliveriesGroup := router.Group("/webhook-deliveries")
	if !h.localMode {
		allDeliveriesGroup.Use(h.requireAdminInServerMode())
	}
	allDeliveriesGroup.GET("", h.listAllWebhookDeliveries)
}

// requireAdminInServerMode is a middleware that requires admin privileges in server mode
func (h *APIHandlers) requireAdminInServerMode() gin.HandlerFunc {
	return func(c *gin.Context) {
		// In local mode, no admin check needed
		if h.localMode {
			c.Next()
			return
		}
		
		// Get user from context (should be set by auth middleware)
		user, exists := auth.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}
		
		// Check if user is admin
		if !user.IsAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
			c.Abort()
			return
		}
		
		c.Next()
	}
}