package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"station/internal/api"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/lighthouse"
	lighthouseServices "station/internal/lighthouse/services"
	"station/internal/logging"
	"station/internal/mcp"
	"station/internal/services"

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
	stdioCmd.Flags().Bool("dev", false, "Enable development mode with GenKit reflection server (default: disabled)")
	stdioCmd.Flags().Bool("core", false, "Run in core mode - MCP server only, no API server or ports (ideal for containers)")
	stdioCmd.Flags().Bool("jaeger", true, "Auto-launch Jaeger for distributed tracing (default: true)")
	rootCmd.AddCommand(stdioCmd)
}

func runStdioServer(cmd *cobra.Command, args []string) error {
	// Set GenKit environment based on --dev flag
	devMode, _ := cmd.Flags().GetBool("dev")
	coreMode, _ := cmd.Flags().GetBool("core")
	if !devMode && os.Getenv("GENKIT_ENV") == "" {
		os.Setenv("GENKIT_ENV", "prod") // Disable reflection server by default
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize Jaeger if enabled (default: true)
	jaegerCtx := context.Background()
	var jaegerSvc *services.JaegerService
	enableJaeger, _ := cmd.Flags().GetBool("jaeger")
	if enableJaeger || os.Getenv("STATION_AUTO_JAEGER") == "true" {
		jaegerSvc = services.NewJaegerService(&services.JaegerConfig{})
		if err := jaegerSvc.Start(jaegerCtx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "⚠️  Warning: Failed to start Jaeger: %v\n", err)
		} else {
			// Set OTEL endpoint for automatic trace export
			os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", jaegerSvc.GetOTLPEndpoint())
			_, _ = fmt.Fprintf(os.Stderr, "🔍 Jaeger UI: %s\n", jaegerSvc.GetUIURL())
			_, _ = fmt.Fprintf(os.Stderr, "🔍 OTLP endpoint: %s\n", jaegerSvc.GetOTLPEndpoint())
		}
	}

	// Setup debug logging to file if in dev mode
	if devMode {
		if logFile, err := os.OpenFile("/tmp/station-stdio-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
			log.SetOutput(logFile)
			log.Printf("=== Station stdio debug session started ===")

			// Initialize internal logging system with debug enabled and file output
			logging.Initialize(true)
		}
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

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

	// Initialize minimal services for API server only
	// Use separate contexts: one for long-lived services (management channel), one for MCP server
	longLivedCtx := context.Background()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Genkit with configured AI provider
	_, err = initializeGenkit(ctx, cfg)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: Failed to initialize Genkit: %v (agent execution will be limited)\n", err)
	}

	// Initialize Lighthouse client for CloudShip integration (same as server mode)
	mode := lighthouse.DetectModeFromCommand()
	lighthouseClient, err := lighthouse.InitializeLighthouseFromConfig(cfg, mode)
	if err != nil {
		log.Printf("Warning: Failed to initialize Lighthouse client: %v", err)
	}

	// Initialize agent service with Lighthouse integration (same as server mode)
	agentSvc := services.NewAgentServiceWithLighthouse(repos, lighthouseClient)

	// Initialize remote control service for bidirectional management (same as server mode)
	var remoteControlSvc *lighthouseServices.RemoteControlService
	if lighthouseClient != nil && (lighthouseClient.GetMode() == lighthouse.ModeServe || lighthouseClient.GetMode() == lighthouse.ModeStdio) {
		log.Printf("🌐 Initializing stdio mode remote control via CloudShip")
		remoteControlSvc = lighthouseServices.NewRemoteControlService(
			lighthouseClient,
			agentSvc,
			repos,
			cfg.CloudShip.RegistrationKey,
			"default", // TODO: use actual environment name
		)

		// Start remote control service with long-lived context to keep management channel active
		if err := remoteControlSvc.Start(longLivedCtx); err != nil {
			log.Printf("Warning: Failed to start remote control service: %v", err)
		} else {
			log.Printf("✅ Stdio mode remote control active - CloudShip can manage this Station")
		}
	}

	// Check if we're in local mode
	localMode := viper.GetBool("local_mode")

	// Create MCP server for stdio communication
	mcpServer := mcp.NewServer(database, agentSvc, repos, cfg, localMode)

	// Set lighthouse client for surgical telemetry integration
	if lighthouseClient != nil {
		mcpServer.SetLighthouseClient(lighthouseClient)
		log.Printf("✅ Lighthouse client configured for MCP server telemetry")
	}

	// Try to start API server if port is available (avoid conflicts with other stdio instances)
	// Skip API server entirely in core mode
	var apiServer *api.Server
	var apiCtx context.Context
	var apiCancel context.CancelFunc
	var wg sync.WaitGroup

	if !coreMode && isPortAvailable(cfg.APIPort) {
		_, _ = fmt.Fprintf(os.Stderr, "🚀 Starting API server on port %d in stdio mode\n", cfg.APIPort)

		apiServer = api.New(cfg, database, localMode, nil)
		apiCtx, apiCancel = context.WithCancel(ctx)

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := apiServer.Start(apiCtx); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "⚠️  API server error: %v\n", err)
			}
		}()
	} else if coreMode {
		_, _ = fmt.Fprintf(os.Stderr, "⚙️  Core mode: running MCP server only (no API server)\n")
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "⚠️  Port %d already in use, skipping API server (another Station instance running?)\n", cfg.APIPort)
	}

	// Log startup message to stderr (so it doesn't interfere with stdio protocol)
	_, _ = fmt.Fprintf(os.Stderr, "🚀 Station MCP Server starting in stdio mode\n")
	_, _ = fmt.Fprintf(os.Stderr, "Local mode: %t\n", localMode)
	if agentSvc != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Agent execution: enabled\n")
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "Agent execution: limited (Genkit initialization failed)\n")
	}
	_, _ = fmt.Fprintf(os.Stderr, "Ready for MCP communication via stdin/stdout\n")

	// Start MCP server in stdio mode in a separate goroutine to keep management channel alive
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := mcpServer.StartStdio(ctx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "⚠️  MCP stdio server error: %v (management channel remains active)\n", err)
		}
	}()

	// Keep the main process alive to maintain the management channel for CloudShip control
	// This ensures persistent bidirectional communication even when no MCP client is connected
	_, _ = fmt.Fprintf(os.Stderr, "🌐 Management channel active - Station remains available for CloudShip control\n")
	_, _ = fmt.Fprintf(os.Stderr, "📡 Station will continue running until terminated (Ctrl+C)\n")

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Block until signal received
	<-sigChan
	_, _ = fmt.Fprintf(os.Stderr, "\n🛑 Received termination signal, shutting down...\n")

	// Cancel context to trigger cleanup
	cancel()

	// Clean shutdown of services when terminating
	if apiCancel != nil {
		_, _ = fmt.Fprintf(os.Stderr, "🛑 Shutting down API server...\n")
		apiCancel()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Clean shutdown of remote control service for CloudShip management
	if remoteControlSvc != nil {
		_, _ = fmt.Fprintf(os.Stderr, "🛑 Shutting down remote control service...\n")
		if err := remoteControlSvc.Stop(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "⚠️  Error stopping remote control service: %v\n", err)
		}
	}

	// Keep Jaeger running across stdio sessions
	// Jaeger container persists for continuous tracing across connections
	if jaegerSvc != nil && jaegerSvc.IsRunning() {
		_, _ = fmt.Fprintf(os.Stderr, "🔍 Jaeger container will continue running for next session\n")
		_, _ = fmt.Fprintf(os.Stderr, "💡 To stop Jaeger: docker stop station-jaeger\n")
	}

	// Note: Lighthouse client cleanup happens automatically via context cancellation

	return nil
}

// isPortAvailable checks if a port is available on localhost
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
