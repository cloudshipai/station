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
	"station/internal/services"
	"station/internal/ssh"
	"station/pkg/crypto"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/firebase/genkit/go/genkit"
	oai "github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/openai/openai-go/option"
	"github.com/spf13/viper"
)

// genkitSetup holds the initialized Genkit components
type genkitSetup struct {
	app          *genkit.Genkit
	openaiPlugin *oai.OpenAI
}

// initializeGenkit initializes Genkit with configured AI provider
func initializeGenkit(ctx context.Context, cfg *config.Config) (*genkitSetup, error) {
	// Initialize AI provider plugin based on configuration
	var genkitApp *genkit.Genkit
	var openaiPlugin *oai.OpenAI
	var err error
	
	switch strings.ToLower(cfg.AIProvider) {
	case "openai":
		// Validate API key for OpenAI
		if cfg.AIAPIKey == "" {
			return nil, fmt.Errorf("STN_AI_API_KEY is required for OpenAI provider")
		}
		openaiPlugin = &oai.OpenAI{
			APIKey: cfg.AIAPIKey,
		}
		// Set custom base URL if provided
		if cfg.AIBaseURL != "" {
			openaiPlugin.Opts = []option.RequestOption{
				option.WithBaseURL(cfg.AIBaseURL),
			}
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
		// For now, main server only supports OpenAI - Gemini support will be added
		return nil, fmt.Errorf("Gemini provider not yet supported in main server (use OpenAI for now)")
	case "ollama":
		// For now, main server only supports OpenAI - Ollama support will be added
		return nil, fmt.Errorf("Ollama provider not yet supported in main server (use OpenAI for now)")
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s (supported: openai, gemini, ollama)", cfg.AIProvider)
	}
	
	return &genkitSetup{
		app:          genkitApp,
		openaiPlugin: openaiPlugin,
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
	keyManager, err := crypto.NewKeyManagerFromEnv()
	if err != nil {
		return fmt.Errorf("failed to initialize key manager: %w", err)
	}

	// Initialize all required services
	mcpConfigSvc := services.NewMCPConfigService(repos, keyManager)
	webhookSvc := services.NewWebhookService(repos)
	
	// Create default environment if none exists
	if err := ensureDefaultEnvironment(ctx, repos); err != nil {
		log.Printf("Warning: Failed to create default environment: %v", err)
	}

	
	// Initialize Genkit with configured AI provider
	genkit, err := initializeGenkit(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize Genkit: %w", err)
	}
	
	agentSvc := services.NewGenkitService(
		genkit.app,
		genkit.openaiPlugin,
		repos.Agents,
		repos.AgentRuns,
		repos.MCPConfigs,
		repos.MCPTools,
		repos.AgentTools,
		repos.Environments,
		mcpConfigSvc,
		webhookSvc,
		telemetryService,
	)
	
	// Initialize MCP for the agent service
	if err := agentSvc.InitializeMCP(ctx); err != nil {
		log.Printf("Warning: Failed to initialize MCP for agent service: %v", err)
	}
	
	// Initialize execution queue service for async agent execution
	executionQueueSvc := services.NewExecutionQueueService(repos, agentSvc, webhookSvc, 5) // 5 workers
	
	// Start execution queue service
	if err := executionQueueSvc.Start(); err != nil {
		return fmt.Errorf("failed to start execution queue service: %w", err)
	}
	defer executionQueueSvc.Stop()
	
	// Initialize scheduler service for cron-based agent execution
	schedulerSvc := services.NewSchedulerService(database, executionQueueSvc)
	
	// Start scheduler service
	if err := schedulerSvc.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler service: %w", err)
	}
	defer schedulerSvc.Stop()
	
	// Start webhook retry processor
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("🪝 Starting webhook retry processor...")
		
		ticker := time.NewTicker(1 * time.Minute) // Process retries every minute
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				if err := webhookSvc.ProcessRetries(ctx); err != nil {
					log.Printf("Webhook retry processing error: %v", err)
				}
			case <-ctx.Done():
				log.Printf("🪝 Webhook retry processor stopping...")
				return
			}
		}
	}()

	// Check if we're in local mode
	localMode := viper.GetBool("local_mode")
	
	sshServer := ssh.New(cfg, database, executionQueueSvc, agentSvc, localMode)
	mcpServer := mcp.NewServer(database, mcpConfigSvc, agentSvc, repos, localMode)
	apiServer := api.New(cfg, database, localMode)
	
	// Initialize ToolDiscoveryService for API config uploads
	toolDiscoveryService := services.NewToolDiscoveryService(repos, mcpConfigSvc)
	
	// Set services for the API server
	apiServer.SetServices(toolDiscoveryService, agentSvc, executionQueueSvc)

	wg.Add(4) // SSH, MCP, API, and webhook retry processor

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
		log.Printf("🚀 Starting API server on port %d", cfg.APIPort)
		if err := apiServer.Start(ctx); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	fmt.Printf("\n✅ Station is running!\n")
	fmt.Printf("🔗 SSH Admin: ssh admin@localhost -p %d\n", cfg.SSHPort)
	fmt.Printf("🔧 MCP Server: http://localhost:%d/mcp\n", cfg.MCPPort)
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
		// Force cleanup critical resources immediately
		if agentSvc != nil {
			forcedCtx, forceCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			agentSvc.Close(forcedCtx)
			forceCancel()
		}
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