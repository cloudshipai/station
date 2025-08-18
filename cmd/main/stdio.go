package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	"station/internal/api"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/mcp"
	"station/internal/services"
	"station/pkg/crypto"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var stdioCmd = &cobra.Command{
	Use:   "stdio",
	Short: "Start Station MCP server in stdio mode",
	Long: `Start the Station MCP server using stdio transport for direct communication.
This mode is useful for integrating Station as an MCP server with other tools
that communicate via standard input/output streams.

All the same tools and resources available in the HTTP mode are available here,
including agent management, file operations, and system resources.`,
	RunE: runStdioServer,
}

func init() {
	rootCmd.AddCommand(stdioCmd)
}

func runStdioServer(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	// Run database migrations
	if err := database.Migrate(); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	// Initialize repositories
	repos := repositories.New(database)

	// Ensure default environment exists
	if err := ensureDefaultEnvironment(context.Background(), repos); err != nil {
		return fmt.Errorf("failed to ensure default environment: %w", err)
	}

	// Initialize key manager using config
	_, err = crypto.NewKeyManagerFromConfig(cfg.EncryptionKey)
	if err != nil {
		return fmt.Errorf("failed to initialize key manager: %w", err)
	}

	// Initialize services  
	// TODO: Replace with file-based config service
	// fileConfigSvc := services.NewFileConfigService(configManager, toolDiscovery, repos)
	_ = services.NewWebhookService(repos) // webhookSvc declared but not used

	// Initialize Genkit and agent service (needed for agent tools)
	ctx := context.Background()
	
	// Initialize Genkit with configured AI provider (minimal setup for stdio)
	_, err = initializeGenkit(ctx, cfg)
	if err != nil {
		log.Printf("Warning: Failed to initialize Genkit: %v (agent execution will be limited)", err)
	}

	// Initialize agent service with IntelligentAgentCreator (same as server mode)
	agentSvc := services.NewAgentService(repos)
	fmt.Fprintf(os.Stderr, "DEBUG: Agent service created: %v\n", agentSvc != nil)
	
	// Note: Skip InitializeMCP in stdio mode - MCP tools are available directly
	// The agent service is initialized but doesn't need the MCP connection test

	// Check if we're in local mode
	localMode := viper.GetBool("local_mode")

	// Create MCP server with the same functionality as HTTP mode
	// Note: stdio mode doesn't use execution queue, pass nil for direct execution
	mcpServer := mcp.NewServer(database, agentSvc, nil, repos, cfg, localMode)

	// Try to start API server if port is available (avoid conflicts with other stdio instances)
	var apiServer *api.Server
	var apiCtx context.Context
	var apiCancel context.CancelFunc
	var wg sync.WaitGroup

	if isPortAvailable(cfg.APIPort) {
		fmt.Fprintf(os.Stderr, "🚀 Starting API server on port %d in stdio mode\n", cfg.APIPort)
		
		apiServer = api.New(cfg, database, localMode)
		apiCtx, apiCancel = context.WithCancel(ctx)
		
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := apiServer.Start(apiCtx); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  API server error: %v\n", err)
			}
		}()
	} else {
		fmt.Fprintf(os.Stderr, "⚠️  Port %d already in use, skipping API server (another Station instance running?)\n", cfg.APIPort)
	}

	// Log startup message to stderr (so it doesn't interfere with stdio protocol)
	fmt.Fprintf(os.Stderr, "🚀 Station MCP Server starting in stdio mode\n")
	fmt.Fprintf(os.Stderr, "Local mode: %t\n", localMode)
	if agentSvc != nil {
		fmt.Fprintf(os.Stderr, "Agent execution: enabled\n")
	} else {
		fmt.Fprintf(os.Stderr, "Agent execution: limited (Genkit initialization failed)\n")
	}
	fmt.Fprintf(os.Stderr, "Ready for MCP communication via stdin/stdout\n")

	// Start MCP server in stdio mode (this blocks until stdin closes)
	if err := mcpServer.StartStdio(ctx); err != nil {
		// Clean shutdown of API server if it was started
		if apiCancel != nil {
			fmt.Fprintf(os.Stderr, "🛑 Shutting down API server...\n")
			apiCancel()
			wg.Wait()
		}
		return fmt.Errorf("failed to start MCP stdio server: %w", err)
	}

	// Clean shutdown of API server when stdio closes
	if apiCancel != nil {
		fmt.Fprintf(os.Stderr, "🛑 Shutting down API server...\n")
		apiCancel()
		wg.Wait()
	}

	return nil
}

// isPortAvailable checks if a port is available on localhost
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}