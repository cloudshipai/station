// Package api provides the HTTP API server for Station
//
// @title Station API
// @version 1.0
// @description Station is a secure, self-hosted platform for creating intelligent multi-environment MCP agents
// @description
// @description ## Features
// @description - Agent management and execution
// @description - MCP server configuration
// @description - OpenAPI to MCP conversion
// @description - Environment management
// @description - Tool discovery and assignment
//
// @contact.name Station Support
// @contact.url https://github.com/cloudshipai/station
//
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
//
// @host localhost:8585
// @BasePath /api/v1
// @schemes http https
//
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @description Bearer token authentication
package api

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"station/internal/api/v1"
	internalconfig "station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/telemetry"
	"station/internal/ui"
	// "station/pkg/crypto" // Removed - no longer needed for file-based configs
)

type Server struct {
	cfg        *internalconfig.Config
	db         db.Database
	httpServer *http.Server
	repos      *repositories.Repositories
	// mcpConfigService removed - using file-based configs only
	// FileConfigService removed - using DeclarativeSync directly
	toolDiscoveryService *services.ToolDiscoveryService // restored for lighthouse/API compatibility
	telemetryService     *telemetry.TelemetryService
	// genkitService removed - service no longer exists
	// executionQueueSvc removed - using direct execution instead
	localMode bool
}

func New(cfg *internalconfig.Config, database db.Database, localMode bool, telemetryService *telemetry.TelemetryService) *Server {
	repos := repositories.New(database)
	// keyManager removed - no longer needed for file-based configs

	// Initialize services (MCPConfigService removed - using file-based configs only)

	// Initialize tool discovery service for lighthouse and API compatibility
	toolDiscoveryService := services.NewToolDiscoveryService(repos)

	// FileConfigService and FileConfigManager removed - using DeclarativeSync directly when needed

	return &Server{
		cfg:                  cfg,
		db:                   database,
		repos:                repos,
		toolDiscoveryService: toolDiscoveryService,
		telemetryService:     telemetryService,
		localMode:            localMode,
	}
}

// SetServices allows setting optional services after creation
func (s *Server) SetServices(toolDiscoveryService *services.ToolDiscoveryService) {
	s.toolDiscoveryService = toolDiscoveryService
}

func (s *Server) Start(ctx context.Context) error {
	// Set Gin to release mode for production
	gin.SetMode(gin.ReleaseMode)

	// Create Gin router with minimal middleware
	router := gin.New()
	router.Use(gin.Recovery())

	// Debug middleware to log all requests
	router.Use(func(c *gin.Context) {
		c.Next()
	})

	// Enable CORS for API endpoints only
	router.Use(func(c *gin.Context) {
		// Skip CORS headers for UI routes
		if !strings.HasPrefix(c.Request.URL.Path, "/ui") {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}
		}

		c.Next()
	})

	// Health check endpoint
	router.GET("/health", s.healthCheck)

	// Debug route
	router.GET("/debug", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"debug": "working"})
	})

	// API v1 routes
	v1Group := router.Group("/api/v1")
	apiHandlers := v1.NewAPIHandlers(
		s.repos,
		s.toolDiscoveryService,
		s.telemetryService,
		s.localMode,
	)
	apiHandlers.RegisterRoutes(v1Group)

	// UI routes - serve embedded UI files when available
	s.setupUIRoutes(router)

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.APIPort),
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("API server error: %v\n", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown with aggressive timeout
	fmt.Println("🛑 Shutting down API server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(shutdownCtx)
}

// Health check endpoint
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "station-api",
		"version": "1.0.0",
	})
}

// setupUIRoutes configures routes for serving the embedded UI from root
func (s *Server) setupUIRoutes(router *gin.Engine) {

	if !ui.IsEmbedded() {
		log.Println("🔍 UI not embedded, skipping UI routes")
		return
	}

	// Get embedded UI filesystem
	uiFS, err := ui.GetFileSystem()
	if err != nil {
		return
	}

	log.Println("🔍 UI filesystem loaded successfully, serving from root")

	// Handle static assets manually to avoid redirect issues
	router.GET("/assets/*filepath", func(c *gin.Context) {
		filepath := c.Param("filepath")
		if len(filepath) > 0 && filepath[0] == '/' {
			filepath = filepath[1:]
		}
		actualPath := "assets/" + filepath

		file, err := uiFS.Open(actualPath)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
			return
		}
		defer func() { _ = file.Close() }()

		content, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read asset"})
			return
		}

		// Set appropriate content type
		if strings.HasSuffix(actualPath, ".js") {
			c.Header("Content-Type", "application/javascript")
		} else if strings.HasSuffix(actualPath, ".css") {
			c.Header("Content-Type", "text/css")
		}

		c.Data(http.StatusOK, c.GetHeader("Content-Type"), content)
	})

	// Handle vite.svg
	router.GET("/vite.svg", func(c *gin.Context) {
		file, err := uiFS.Open("vite.svg")
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "vite.svg not found"})
			return
		}
		defer func() { _ = file.Close() }()

		content, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read vite.svg"})
			return
		}

		c.Header("Content-Type", "image/svg+xml")
		c.Data(http.StatusOK, "image/svg+xml", content)
	})

	// Serve index.html for all other routes (SPA catch-all)
	router.NoRoute(func(c *gin.Context) {
		// Skip API routes
		if strings.HasPrefix(c.Request.URL.Path, "/api/") ||
			strings.HasPrefix(c.Request.URL.Path, "/health") ||
			strings.HasPrefix(c.Request.URL.Path, "/debug") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}

		// Handle PNG images at root level (for screenshots in Getting Started)
		if strings.HasSuffix(c.Request.URL.Path, ".png") && !strings.Contains(c.Request.URL.Path[1:], "/") {
			filename := strings.TrimPrefix(c.Request.URL.Path, "/")

			file, err := uiFS.Open(filename)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
				return
			}
			defer func() { _ = file.Close() }()

			content, err := io.ReadAll(file)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read image"})
				return
			}

			c.Header("Content-Type", "image/png")
			c.Data(http.StatusOK, "image/png", content)
			return
		}

		// Read index.html manually to prevent redirect loops
		file, err := uiFS.Open("index.html")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load UI"})
			return
		}
		defer func() { _ = file.Close() }()

		content, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read UI"})
			return
		}

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})
}
