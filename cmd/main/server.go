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
	"station/internal/mcp"
	"station/internal/mcp_agents"
	"station/internal/services"
	"station/internal/ssh"
	"station/pkg/cloudshipai"
	"station/pkg/crypto"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/firebase/genkit/go/genkit"
	stationGenkit "station/internal/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/spf13/viper"
)

// genkitSetup holds the initialized Genkit components
type genkitSetup struct {
	app           *genkit.Genkit
	openaiPlugin  *stationGenkit.StationOpenAI
	geminiPlugin  *googlegenai.GoogleAI
}

// initializeGenkit initializes Genkit with configured AI provider
func initializeGenkit(ctx context.Context, cfg *config.Config) (*genkitSetup, error) {
	// Initialize AI provider plugin based on configuration
	var genkitApp *genkit.Genkit
	var openaiPlugin *stationGenkit.StationOpenAI
	var geminiPlugin *googlegenai.GoogleAI
	var err error
	
	switch strings.ToLower(cfg.AIProvider) {
	case "openai":
		// Validate API key for OpenAI
		if cfg.AIAPIKey == "" {
			return nil, fmt.Errorf("STN_AI_API_KEY is required for OpenAI provider")
		}
		openaiPlugin = &stationGenkit.StationOpenAI{
			APIKey: cfg.AIAPIKey,
		}
		// Set custom base URL if provided
		if cfg.AIBaseURL != "" {
			openaiPlugin.BaseURL = cfg.AIBaseURL
		}
		genkitApp, err = genkit.Init(ctx, genkit.WithPlugins(openaiPlugin))
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Genkit with OpenAI: %w", err)
		}
	case "gemini":
		// Validate API key for Gemini
		if cfg.AIAPIKey == "" {
			return nil, fmt.Errorf("STN_AI_API_KEY (or GOOGLE_API_KEY) is required for Gemini provider")
		}
		geminiPlugin = &googlegenai.GoogleAI{
			APIKey: cfg.AIAPIKey,
		}
		genkitApp, err = genkit.Init(ctx, genkit.WithPlugins(geminiPlugin))
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Genkit with Gemini: %w", err)
		}
	case "ollama":
		// For now, main server only supports OpenAI - Ollama support will be added
		return nil, fmt.Errorf("Ollama provider not yet supported in main server (use OpenAI for now)")
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s (supported: openai, gemini, ollama)", cfg.AIProvider)
	}
	
	return &genkitSetup{
		app:           genkitApp,
		openaiPlugin:  openaiPlugin,
		geminiPlugin:  geminiPlugin,
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
	defer database.Close()

	if err := database.Migrate(); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	var wg sync.WaitGroup
	
	// Create repositories and services
	repos := repositories.New(database)
	_, err = crypto.NewKeyManagerFromEnv()
	if err != nil {
		return fmt.Errorf("failed to initialize key manager: %w", err)
	}

	// Initialize required services
	
	// Create default environment if none exists
	if err := ensureDefaultEnvironment(ctx, repos); err != nil {
		log.Printf("Warning: Failed to create default environment: %v", err)
	}

	
	// Initialize Genkit with configured AI provider
	_, err = initializeGenkit(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize Genkit: %w", err)
	}
	
	// Initialize agent service with AgentExecutionEngine
	agentSvc := services.NewAgentService(repos)
	
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
	

	// Check if we're in local mode
	localMode := viper.GetBool("local_mode")
	
	// Get environment name from viper config (defaults to "default")
	environmentName := viper.GetString("serve_environment")
	if environmentName == "" {
		environmentName = "default"
	}
	log.Printf("🤖 Serving agents from environment: %s", environmentName)
	
	sshServer := ssh.New(cfg, database, repos, agentSvc, localMode)
	mcpServer := mcp.NewServer(database, agentSvc, repos, cfg, localMode)
	dynamicAgentServer := mcp_agents.NewDynamicAgentServer(repos, agentSvc, localMode, environmentName)
	apiServer := api.New(cfg, database, localMode, telemetryService)
	
	// Initialize ToolDiscoveryService for API config uploads
	toolDiscoveryService := services.NewToolDiscoveryService(repos)
	
	// Set services for the API server  
	apiServer.SetServices(toolDiscoveryService)

	// Initialize CloudShip AI client
	cloudshipaiClient := cloudshipai.NewClient()
	if err := cloudshipaiClient.Start(); err != nil {
		log.Printf("Warning: Failed to start CloudShip AI client: %v", err)
	}

	wg.Add(5) // SSH, MCP, Dynamic Agent MCP, API, and webhook retry processor

	go func() {
		defer wg.Done()
		log.Printf("🌐 Starting SSH server on port %d", cfg.SSHPort)
		if err := sshServer.Start(ctx); err != nil {
			log.Printf("SSH server error: %v", err)
		}
	}()

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
		log.Printf("🤖 Starting Dynamic Agent MCP server on port %d", cfg.MCPPort+1)
		if err := dynamicAgentServer.Start(ctx, cfg.MCPPort+1); err != nil {
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
		log.Printf("🚀 Starting API server on port %d", cfg.APIPort)
		if err := apiServer.Start(ctx); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	// Track server startup telemetry
	if telemetryService != nil {
		telemetryService.TrackServerModeStarted(cfg.APIPort, cfg.MCPPort, cfg.SSHPort)
	}
	
	fmt.Printf("\n✅ Station is running!\n")
	fmt.Printf("🔗 SSH Admin: ssh admin@localhost -p %d\n", cfg.SSHPort)
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
	
	// Stop CloudShip AI client
	cloudshipaiClient.Stop()

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