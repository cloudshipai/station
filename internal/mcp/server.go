package mcp

import (
	"context"
	"fmt"
	"log"
	"time"

	"station/internal/auth"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"

	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	mcpServer        *server.MCPServer
	httpServer       *server.StreamableHTTPServer
	db               db.Database
	mcpConfigSvc     *services.MCPConfigService
	toolDiscoverySvc *ToolDiscoveryService
	agentService     services.AgentServiceInterface
	authService      *auth.AuthService
	repos            *repositories.Repositories
	localMode        bool
}

func NewServer(database db.Database, mcpConfigSvc *services.MCPConfigService, agentService services.AgentServiceInterface, repos *repositories.Repositories, localMode bool) *Server {
	// Create MCP server using the official mcp-go library
	mcpServer := server.NewMCPServer(
		"Station MCP Server",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithRecovery(),
	)

	toolDiscoverySvc := NewToolDiscoveryService(database, mcpConfigSvc, repos)
	authService := auth.NewAuthService(repos)

	// Create streamable HTTP server
	httpServer := server.NewStreamableHTTPServer(mcpServer)
	
	log.Printf("MCP Server configured with streamable HTTP transport")

	server := &Server{
		mcpServer:        mcpServer,
		httpServer:       httpServer,
		db:               database,
		mcpConfigSvc:     mcpConfigSvc,
		toolDiscoverySvc: toolDiscoverySvc,
		agentService:     agentService,
		authService:      authService,
		repos:            repos,
		localMode:        localMode,
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
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting MCP server using streamable HTTP transport on %s", addr)
	log.Printf("MCP endpoint will be available at http://localhost:%d/mcp", port)
	
	if err := s.httpServer.Start(addr); err != nil {
		return fmt.Errorf("MCP server error: %w", err)
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
	
	if s.httpServer != nil {
		log.Println("MCP HTTP server shutdown complete")
	}
	
	log.Println("MCP server shutdown complete")
	return nil
}

func (s *Server) isLocalMode() bool {
	return s.localMode
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

func (s *Server) requireAdminInServerMode(ctx context.Context) error {
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