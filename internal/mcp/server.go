package mcp

import (
	"context"
	"fmt"
	"log"
	"time"

	"station/internal/auth"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/lighthouse"
	"station/internal/schemas"
	"station/internal/services"
	"station/internal/telemetry"

	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	mcpServer          *server.MCPServer
	httpServer         *server.StreamableHTTPServer
	db                 db.Database
	toolDiscoverySvc   *ToolDiscoveryService
	agentService       services.AgentServiceInterface
	authService        *auth.AuthService
	repos              *repositories.Repositories
	config             *config.Config
	localMode          bool
	agentExportService *services.AgentExportService
	bundleHandler      *UnifiedBundleHandler
	telemetryService   *telemetry.TelemetryService
	lighthouseClient   *lighthouse.LighthouseClient // For surgical telemetry integration
	schemaRegistry     *schemas.SchemaRegistry
	schedulerService   *services.SchedulerService // Cron-based agent scheduler
	benchmarkService   *services.BenchmarkService // Async benchmark evaluation
}

func NewServer(database db.Database, agentService services.AgentServiceInterface, repos *repositories.Repositories, cfg *config.Config, localMode bool) *Server {
	// Create MCP server using the official mcp-go library
	mcpServer := server.NewMCPServer(
		"Station MCP Server",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithRecovery(),
	)

	toolDiscoverySvc := NewToolDiscoveryService(database, repos)
	authService := auth.NewAuthService(repos)

	// Create streamable HTTP server (no OAuth - management MCP is for local use only)
	httpServer := server.NewStreamableHTTPServer(mcpServer)

	log.Printf("MCP Server configured with streamable HTTP transport")

	// Initialize scheduler service
	schedulerService := services.NewSchedulerService(database, repos, agentService)

	// Initialize benchmark service
	benchmarkService, err := services.NewBenchmarkService(database.Conn(), cfg)
	if err != nil {
		log.Printf("Warning: failed to initialize benchmark service: %v", err)
		benchmarkService = nil
	}

	server := &Server{
		mcpServer:          mcpServer,
		httpServer:         httpServer,
		db:                 database,
		toolDiscoverySvc:   toolDiscoverySvc,
		agentService:       agentService,
		authService:        authService,
		repos:              repos,
		config:             cfg,
		localMode:          localMode,
		agentExportService: services.NewAgentExportService(repos),
		bundleHandler:      NewUnifiedBundleHandler(),
		schemaRegistry:     schemas.NewSchemaRegistry(),
		schedulerService:   schedulerService,
		benchmarkService:   benchmarkService,
	}

	// Setup the server capabilities
	server.setupTools()
	server.setupResources()
	server.setupToolSuggestion()

	// Setup the enhanced tools server for advanced functionality
	NewToolsServer(repos, mcpServer, agentService, localMode)

	log.Printf("MCP Server setup complete - Resources vs Tools architecture implemented")
	log.Printf("📄 Resources: Read-only data access (GET-like operations)")
	log.Printf("🛠️  Tools: Operations with side effects (POST-like operations)")

	return server
}

func (s *Server) Start(ctx context.Context, port int) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("Starting MCP server using streamable HTTP transport on %s", addr)
	log.Printf("MCP endpoint will be available at http://localhost:%d/mcp", port)

	// Start scheduler service for cron-based agent execution
	if s.schedulerService != nil {
		if err := s.schedulerService.Start(); err != nil {
			return fmt.Errorf("failed to start scheduler service: %w", err)
		}
	}

	if err := s.httpServer.Start(addr); err != nil {
		return fmt.Errorf("MCP server error: %w", err)
	}

	return nil
}

// StartStdio starts the MCP server using stdio transport
func (s *Server) StartStdio(ctx context.Context) error {
	log.Printf("Starting MCP server using stdio transport")

	// Start scheduler service for cron-based agent execution
	if s.schedulerService != nil {
		if err := s.schedulerService.Start(); err != nil {
			return fmt.Errorf("failed to start scheduler service: %w", err)
		}
	}

	// Use the mcp-go ServeStdio convenience function
	if err := server.ServeStdio(s.mcpServer); err != nil {
		return fmt.Errorf("MCP stdio server error: %w", err)
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("MCP server shutting down...")

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
	}

	// Stop scheduler service
	if s.schedulerService != nil {
		s.schedulerService.Stop()
	}

	if s.httpServer != nil {
		log.Println("MCP HTTP server shutdown complete")
	}

	log.Println("MCP server shutdown complete")
	return nil
}

func (s *Server) requireAuthInServerMode(ctx context.Context) error {
	if s.localMode {
		return nil
	}

	user, err := auth.GetUserFromHTTPContext(ctx)
	if err != nil {
		return fmt.Errorf("authentication required: %w", err)
	}

	if user == nil {
		return fmt.Errorf("no authenticated user found")
	}

	return nil
}

// SetLighthouseClient sets the lighthouse client for surgical telemetry integration
func (s *Server) SetLighthouseClient(client *lighthouse.LighthouseClient) {
	s.lighthouseClient = client
}
