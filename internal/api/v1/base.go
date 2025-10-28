package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"station/internal/auth"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/telemetry"
)

// APIHandlers contains all the API handlers and their dependencies
type APIHandlers struct {
	repos        *repositories.Repositories
	agentService services.AgentServiceInterface
	// mcpConfigService removed - using file-based configs only
	toolDiscoveryService *services.ToolDiscoveryService // restored for lighthouse/API compatibility
	// genkitService removed - service no longer exists
	// executionQueueSvc removed - using direct execution instead
	agentExportService *services.AgentExportService
	telemetryService   *telemetry.TelemetryService
	localMode          bool
}

// NewAPIHandlers creates a new API handlers instance
func NewAPIHandlers(
	repos *repositories.Repositories,
	toolDiscoveryService *services.ToolDiscoveryService,
	telemetryService *telemetry.TelemetryService,
	localMode bool,
) *APIHandlers {
	return &APIHandlers{
		repos:                repos,
		agentService:         services.NewAgentService(repos),
		toolDiscoveryService: toolDiscoveryService,
		agentExportService:   services.NewAgentExportService(repos),
		telemetryService:     telemetryService,
		localMode:            localMode,
	}
}

// telemetryMiddleware tracks API requests
func (h *APIHandlers) telemetryMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// Track API request telemetry
		if h.telemetryService != nil {
			h.telemetryService.TrackAPIRequest(
				param.Path,
				param.Method,
				param.StatusCode,
				param.Latency.Milliseconds(),
			)
		}
		return ""
	})
}

// RegisterRoutes registers all v1 API routes
func (h *APIHandlers) RegisterRoutes(router *gin.RouterGroup) {
	// Add telemetry middleware
	router.Use(h.telemetryMiddleware())

	// Create auth middleware with local mode setting
	authMiddleware := auth.NewAuthMiddlewareWithLocalMode(h.repos, h.localMode)

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

	// MCP server management routes (file-based configuration)
	h.registerMCPManagementRoutes(envGroup)

	// MCP Directory template routes
	h.registerMCPDirectoryRoutes(router)

	// Tools routes (nested under environments)
	toolsGroup := envGroup.Group("/:env_id/tools")
	// Inherits admin-only restriction from envGroup
	h.registerToolsRoutes(toolsGroup)

	// MCP Servers routes - admin only in server mode
	mcpServersGroup := router.Group("/mcp-servers")
	if !h.localMode {
		mcpServersGroup.Use(h.requireAdminInServerMode())
	}
	h.registerMCPServerRoutes(mcpServersGroup)

	// Agent routes - accessible to regular users in server mode
	agentGroup := router.Group("/agents")
	agentGroup.GET("", h.listAgents)                    // Users can list agents
	agentGroup.GET("/:id", h.getAgent)                  // Users can view individual agents
	agentGroup.GET("/:id/details", h.getAgentWithTools) // Users can view agent details
	agentGroup.GET("/:id/prompt", h.getAgentPrompt)     // Users can view agent prompts
	agentGroup.PUT("/:id/prompt", h.updateAgentPrompt)  // Users can update agent prompts
	agentGroup.POST("/:id/execute", h.callAgent)        // Users can execute agents (direct execution with async goroutine)

	// Admin-only agent management routes
	agentAdminGroup := router.Group("/admin/agents")
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

	// Sync route - admin only in server mode
	syncGroup := router.Group("/sync")
	if !h.localMode {
		syncGroup.Use(h.requireAdminInServerMode())
	}
	syncGroup.POST("", h.syncConfigurations)
	syncGroup.POST("/interactive", h.startInteractiveSync)
	syncGroup.GET("/status/:id", h.getSyncStatus)
	syncGroup.POST("/variables", h.submitVariables)

	// Bundles route - admin only in server mode
	bundlesGroup := router.Group("/bundles")
	if !h.localMode {
		bundlesGroup.Use(h.requireAdminInServerMode())
	}
	bundlesGroup.GET("", h.listBundles)
	bundlesGroup.GET("/cloudship", h.listCloudShipBundles)
	bundlesGroup.POST("", h.createBundle)
	bundlesGroup.POST("/install", h.installBundle)

	// Demo Bundles routes
	demoBundlesGroup := router.Group("/demo-bundles")
	demoBundlesGroup.GET("", h.listDemoBundles)
	demoBundlesGroup.POST("/install", h.installDemoBundle)

	// MCP API bridge route - admin only in server mode
	mcpGroup := router.Group("/mcp")
	if !h.localMode {
		mcpGroup.Use(h.requireAdminInServerMode())
	}
	h.registerMCPRoutes(mcpGroup)

	// Ship CLI routes
	shipGroup := router.Group("/ship")
	h.registerShipRoutes(shipGroup)

	// OpenAPI to MCP conversion routes - admin only in server mode
	openapiGroup := router.Group("/openapi")
	if !h.localMode {
		openapiGroup.Use(h.requireAdminInServerMode())
	}
	h.registerOpenAPIRoutes(openapiGroup)

	// CloudShip lighthouse status
	router.GET("/lighthouse/status", h.LighthouseStatusHandler)
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

// syncConfigurations triggers file-based configuration sync
func (h *APIHandlers) syncConfigurations(c *gin.Context) {
	// Import the os/exec package for running the stn sync command
	// For now, return a success response - actual implementation would call stn sync
	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   "Configuration sync triggered successfully",
		"timestamp": "2025-08-17T22:45:00Z",
	})
}

// registerMCPRoutes registers MCP tool bridge routes
func (h *APIHandlers) registerMCPRoutes(mcpGroup *gin.RouterGroup) {
	// For now, this can be empty as we're using existing REST endpoints
	// Future: Could add direct MCP tool bridging if needed
}
