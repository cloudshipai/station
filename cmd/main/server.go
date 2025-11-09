package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"station/internal/api"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/lighthouse"
	lighthouseServices "station/internal/lighthouse/services"
	"station/internal/mcp"
	"station/internal/mcp_agents"
	"station/internal/services"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/openai/openai-go/option"
	"github.com/spf13/viper"
)

// genkitSetup holds the initialized Genkit components
type genkitSetup struct {
	app          *genkit.Genkit
	openaiPlugin *openai.OpenAI // Official GenKit v1.0.1 OpenAI plugin
	geminiPlugin *googlegenai.GoogleAI
}

// initializeGenkit initializes Genkit with configured AI provider
func initializeGenkit(ctx context.Context, cfg *config.Config) (*genkitSetup, error) {
	// Initialize AI provider plugin based on configuration
	var genkitApp *genkit.Genkit
	var openaiPlugin *openai.OpenAI
	var geminiPlugin *googlegenai.GoogleAI

	switch strings.ToLower(cfg.AIProvider) {
	case "openai":
		// Validate API key for OpenAI
		if cfg.AIAPIKey == "" {
			return nil, fmt.Errorf("STN_AI_API_KEY is required for OpenAI provider")
		}

		// Build request options for official plugin
		var opts []option.RequestOption
		if cfg.AIBaseURL != "" {
			opts = append(opts, option.WithBaseURL(cfg.AIBaseURL))
		}

		openaiPlugin = &openai.OpenAI{
			APIKey: cfg.AIAPIKey,
			Opts:   opts,
		}
		genkitApp = genkit.Init(ctx, genkit.WithPlugins(openaiPlugin))
		// GenKit v1.0.1 Init doesn't return error
	case "gemini":
		// Validate API key for Gemini
		if cfg.AIAPIKey == "" {
			return nil, fmt.Errorf("STN_AI_API_KEY (or GOOGLE_API_KEY) is required for Gemini provider")
		}
		geminiPlugin = &googlegenai.GoogleAI{
			APIKey: cfg.AIAPIKey,
		}
		genkitApp = genkit.Init(ctx, genkit.WithPlugins(geminiPlugin))
		// GenKit v1.0.1 Init doesn't return error
	case "ollama":
		// For now, main server only supports OpenAI - Ollama support will be added
		return nil, fmt.Errorf("Ollama provider not yet supported in main server (use OpenAI for now)")
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s (supported: openai, gemini, ollama)", cfg.AIProvider)
	}

	return &genkitSetup{
		app:          genkitApp,
		openaiPlugin: openaiPlugin,
		geminiPlugin: geminiPlugin,
	}, nil
}

func runMainServer() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() { _ = database.Close() }()

	if err := database.Migrate(); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	var wg sync.WaitGroup

	// Create repositories and services
	repos := repositories.New(database)

	// Get environment name from viper config (defaults to "default")
	environmentName := viper.GetString("serve_environment")
	if environmentName == "" {
		environmentName = "default"
	}

	// Initialize required services

	// Create default environment if none exists
	if err := ensureDefaultEnvironment(ctx, repos); err != nil {
		log.Printf("Warning: Failed to create default environment: %v", err)
	}

	// Automatic DeclarativeSync on startup for Docker container deployment
	log.Printf("🔄 Running automatic sync for environment: %s", environmentName)
	syncer := services.NewDeclarativeSync(repos, cfg)
	syncResult, err := syncer.SyncEnvironment(ctx, environmentName, services.SyncOptions{
		DryRun:      false,
		Validate:    false,
		Interactive: false, // Non-interactive for Docker containers
		Verbose:     true,  // Verbose for debugging
		Confirm:     false,
	})
	if err != nil {
		// Log sync failure but don't crash the server - cleanup already ran
		// This allows the server to start even if MCP configs are broken
		log.Printf("⚠️  WARNING: Environment sync failed for '%s': %v", environmentName, err)
		log.Printf("⚠️  Broken MCP servers have been cleaned up from database")
		log.Printf("⚠️  Please fix or remove broken config files and re-sync via UI or CLI")
		log.Printf("⚠️  Server will continue starting with no MCP tools available")
	} else {
		log.Printf("✅ Sync completed - Agents: %d processed, %d synced | MCP Servers: %d processed, %d connected",
			syncResult.AgentsProcessed, syncResult.AgentsSynced, syncResult.MCPServersProcessed, syncResult.MCPServersConnected)
	}

	// Initialize Genkit with configured AI provider
	_, err = initializeGenkit(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize Genkit: %w", err)
	}

	// Initialize Lighthouse client for CloudShip integration
	mode := lighthouse.DetectModeFromCommand()
	lighthouseClient, err := lighthouse.InitializeLighthouseFromConfig(cfg, mode)
	if err != nil {
		log.Printf("Warning: Failed to initialize Lighthouse client: %v", err)
	}

	// Initialize agent service with AgentExecutionEngine and Lighthouse integration
	agentSvc := services.NewAgentServiceWithLighthouse(repos, lighthouseClient)

	// Initialize MCP for the agent service
	if err := agentSvc.InitializeMCP(ctx); err != nil {
		log.Printf("Warning: Failed to initialize MCP for agent service: %v", err)
	}

	// Initialize scheduler service for cron-based agent execution (using direct execution)
	schedulerSvc := services.NewSchedulerService(database, agentSvc)

	// Start scheduler service
	if err := schedulerSvc.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler service: %w", err)
	}
	defer schedulerSvc.Stop()

	// Initialize remote control service for server mode CloudShip integration
	var remoteControlSvc *lighthouseServices.RemoteControlService
	if lighthouseClient != nil && lighthouseClient.GetMode() == lighthouse.ModeServe {
		log.Printf("🌐 Initializing server mode remote control via CloudShip")
		remoteControlSvc = lighthouseServices.NewRemoteControlService(
			lighthouseClient,
			agentSvc,
			repos,
			cfg.CloudShip.RegistrationKey,
			environmentName, // Use actual environment name
		)

		// Start remote control service
		if err := remoteControlSvc.Start(ctx); err != nil {
			log.Printf("Warning: Failed to start remote control service: %v", err)
		} else {
			log.Printf("✅ Server mode remote control active - CloudShip can manage this Station")
		}
	}

	// Check if we're in local mode
	localMode := viper.GetBool("local_mode")

	log.Printf("🤖 Serving agents from environment: %s", environmentName)

	mcpServer := mcp.NewServer(database, agentSvc, repos, cfg, localMode)
	// Set lighthouse client for IngestData dual flow
	if lighthouseClient != nil {
		mcpServer.SetLighthouseClient(lighthouseClient)
		log.Printf("✅ Lighthouse client configured for MCP server IngestData dual flow")
	}
	dynamicAgentServer := mcp_agents.NewDynamicAgentServer(repos, agentSvc, localMode, environmentName)
	apiServer := api.New(cfg, database, localMode, nil)

	// Initialize ToolDiscoveryService for lighthouse and API compatibility
	toolDiscoveryService := services.NewToolDiscoveryService(repos)

	// Set services for the API server
	apiServer.SetServices(toolDiscoveryService)

	wg.Add(4) // MCP, Dynamic Agent MCP, API, and webhook retry processor

	// SSH server removed - not needed

	go func() {
		defer wg.Done()
		log.Printf("🔧 Starting MCP server on port %d", cfg.MCPPort)
		if err := mcpServer.Start(ctx, cfg.MCPPort); err != nil {
			log.Printf("MCP server error: %v", err)
		}

		// Wait for context cancellation, then shutdown fast
		<-ctx.Done()

		// Very aggressive timeout - 1s for MCP shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer shutdownCancel()

		log.Printf("🔧 Shutting down MCP server...")
		if err := mcpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("MCP server shutdown error: %v", err)
		} else {
			log.Printf("🔧 MCP server stopped gracefully")
		}
	}()

	go func() {
		defer wg.Done()
		dynamicAgentPort := 3030
		log.Printf("🤖 Starting Dynamic Agent MCP server on port %d", dynamicAgentPort)
		if err := dynamicAgentServer.Start(ctx, dynamicAgentPort); err != nil {
			log.Printf("Dynamic Agent MCP server error: %v", err)
		}

		// Wait for context cancellation, then shutdown fast
		<-ctx.Done()

		// Very aggressive timeout - 1s for dynamic MCP shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer shutdownCancel()

		log.Printf("🤖 Shutting down Dynamic Agent MCP server...")
		if err := dynamicAgentServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("Dynamic Agent MCP server shutdown error: %v", err)
		} else {
			log.Printf("🤖 Dynamic Agent MCP server stopped gracefully")
		}
	}()

	go func() {
		defer wg.Done()
		if err := apiServer.Start(ctx); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	// Remove telemetry tracking

	fmt.Printf("\n✅ Station is running!\n")
	fmt.Printf("🔧 MCP Server: http://localhost:%d/mcp\n", cfg.MCPPort)
	fmt.Printf("🤖 Dynamic Agent MCP: http://localhost:%d/mcp (environment: %s)\n", cfg.MCPPort+1, environmentName)
	fmt.Printf("🌐 API Server: http://localhost:%d\n", cfg.APIPort)
	fmt.Printf("\nPress Ctrl+C to stop\n\n")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	fmt.Println("\n🛑 Received shutdown signal, gracefully shutting down...")

	// Much faster timeout - 3s for clean shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()

	// Signal all goroutines to start shutdown immediately
	cancel()

	// Stop remote control service
	if remoteControlSvc != nil {
		if err := remoteControlSvc.Stop(); err != nil {
			log.Printf("Error stopping remote control service: %v", err)
		} else {
			log.Printf("🌐 Remote control service stopped gracefully")
		}
	}

	// Stop Lighthouse client
	if lighthouseClient != nil {
		if err := lighthouseClient.Close(); err != nil {
			log.Printf("Error stopping Lighthouse client: %v", err)
		}
	}

	// Create done channel with aggressive timeout handling
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("✅ All servers stopped gracefully")
	case <-shutdownCtx.Done():
		fmt.Println("⏰ Shutdown timeout exceeded (3s), forcing exit")
	}

	return nil
}

// ensureDefaultEnvironment creates a default environment if none exists
func ensureDefaultEnvironment(_ context.Context, repos *repositories.Repositories) error {
	// Check if any environments exist
	envs, err := repos.Environments.List()
	if err != nil {
		return fmt.Errorf("failed to check existing environments: %w", err)
	}

	// If environments exist, nothing to do
	if len(envs) > 0 {
		log.Printf("✅ Found %d existing environments", len(envs))
		return nil
	}

	// Create default environment
	description := "Default environment for MCP configurations"

	// Get console user for created_by field
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		return fmt.Errorf("failed to get console user: %w", err)
	}

	defaultEnv, err := repos.Environments.Create("default", &description, consoleUser.ID)
	if err != nil {
		// Check if it's a unique constraint error (environment already exists)
		if err.Error() == "UNIQUE constraint failed: environments.name" {
			log.Printf("✅ Default environment already exists")
			return nil
		}
		return fmt.Errorf("failed to create default environment: %w", err)
	}

	log.Printf("✅ Created default environment (ID: %d)", defaultEnv.ID)
	return nil
}
