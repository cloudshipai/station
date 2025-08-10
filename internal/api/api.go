package api

import (
	"context"
	"fmt"
	"net/http"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/spf13/afero"
	
	"station/internal/api/v1"
	internalconfig "station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/filesystem"
	"station/internal/services"
	"station/internal/template"
	"station/internal/variables"
	"station/pkg/config"
	// "station/pkg/crypto" // Removed - no longer needed for file-based configs
)

type Server struct {
	cfg                  *internalconfig.Config
	db                   db.Database
	httpServer           *http.Server
	repos                *repositories.Repositories
	// mcpConfigService removed - using file-based configs only
	fileConfigService    *services.FileConfigService
	// hybridConfigService removed - using file-based configs only
	toolDiscoveryService *services.ToolDiscoveryService
	// genkitService removed - service no longer exists
	webhookService       *services.WebhookService
	executionQueueSvc    *services.ExecutionQueueService
	localMode            bool
}

func New(cfg *internalconfig.Config, database db.Database, localMode bool) *Server {
	repos := repositories.New(database)
	// keyManager removed - no longer needed for file-based configs
	
	// Initialize services (MCPConfigService removed - using file-based configs only)
	webhookService := services.NewWebhookService(repos)
	
	// Initialize file config components
	fs := afero.NewOsFs()
	fileSystem := filesystem.NewConfigFileSystem(fs, "./config", "./config/vars")
	templateEngine := template.NewGoTemplateEngine()
	variableStore := variables.NewEnvVariableStore(fs)
	
	// Create file config options with default paths
	fileConfigOptions := config.FileConfigOptions{
		ConfigDir:       "./config",        // Default config directory
		VariablesDir:    "./config/vars",   // Default variables directory
		Strategy:        config.StrategyTemplateFirst,
		AutoCreate:      true,
		BackupOnChange:  false,
		ValidateOnLoad:  true,
	}
	
	// Create file config manager
	fileConfigManager := internalconfig.NewFileConfigManager(
		fileSystem,
		templateEngine, 
		variableStore,
		fileConfigOptions,
		repos.Environments,
	)
	
	// Initialize tool discovery service (updated for file-based configs)
	toolDiscoveryService := services.NewToolDiscoveryService(repos)
	
	// Initialize file config service
	fileConfigService := services.NewFileConfigService(
		fileConfigManager,
		toolDiscoveryService,
		repos,
	)
	
	// Hybrid config service removed - using file-based configs only
	
	return &Server{
		cfg:                  cfg,
		db:                   database,
		repos:                repos,
		fileConfigService:    fileConfigService,
		toolDiscoveryService: toolDiscoveryService,
		webhookService:       webhookService,
		localMode:            localMode,
	}
}

// SetServices allows setting optional services after creation (simplified for file-based configs)
func (s *Server) SetServices(toolDiscoveryService *services.ToolDiscoveryService, executionQueueSvc *services.ExecutionQueueService) {
	s.toolDiscoveryService = toolDiscoveryService
	s.executionQueueSvc = executionQueueSvc
}

func (s *Server) Start(ctx context.Context) error {
	// Set Gin to release mode for production
	gin.SetMode(gin.ReleaseMode)
	
	// Create Gin router with minimal middleware
	router := gin.New()
	router.Use(gin.Recovery())
	
	// Enable CORS for API endpoints
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})
	
	// Health check endpoint
	router.GET("/health", s.healthCheck)
	
	// API v1 routes
	v1Group := router.Group("/api/v1")
	apiHandlers := v1.NewAPIHandlers(
		s.repos,
		s.toolDiscoveryService,
		s.webhookService,
		s.executionQueueSvc,
		s.localMode,
	)
	apiHandlers.RegisterRoutes(v1Group)
	
	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.APIPort),
		Handler: router,
	}
	
	// Start server in goroutine
	go func() {
		fmt.Printf("🚀 API server starting on port %d\n", s.cfg.APIPort)
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