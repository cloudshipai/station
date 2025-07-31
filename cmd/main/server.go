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
	"sync"
	"syscall"
	"time"

	"github.com/firebase/genkit/go/genkit"
	oai "github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/spf13/viper"
)

// genkitSetup holds the initialized Genkit components
type genkitSetup struct {
	app          *genkit.Genkit
	openaiPlugin *oai.OpenAI
}

// initializeGenkit initializes Genkit with OpenAI plugin
func initializeGenkit(ctx context.Context, apiKey string) (*genkitSetup, error) {
	// Initialize OpenAI plugin
	openaiPlugin := &oai.OpenAI{APIKey: apiKey}
	
	// Initialize Genkit with OpenAI plugin
	app, err := genkit.Init(ctx, genkit.WithPlugins(openaiPlugin))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Genkit: %w", err)
	}
	
	return &genkitSetup{
		app:          app,
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
	
	// Create default environment if none exists
	if err := ensureDefaultEnvironment(ctx, repos); err != nil {
		log.Printf("Warning: Failed to create default environment: %v", err)
	}

	
	// Create GenkitService with OpenAI plugin
	// TODO: Make API key configurable
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		log.Printf("Warning: OPENAI_API_KEY not set, agent execution may fail")
	}
	
	// Initialize Genkit with OpenAI plugin
	genkit, err := initializeGenkit(ctx, openaiAPIKey)
	if err != nil {
		return fmt.Errorf("failed to initialize Genkit: %w", err)
	}
	
	agentSvc := services.NewGenkitService(
		genkit.app,
		genkit.openaiPlugin,
		repos.Agents,
		repos.AgentRuns,
		repos.MCPConfigs,
		repos.AgentTools,
		repos.AgentEnvironments,
		repos.Environments,
		mcpConfigSvc,
	)
	
	// Initialize MCP for the agent service
	if err := agentSvc.InitializeMCP(ctx); err != nil {
		log.Printf("Warning: Failed to initialize MCP for agent service: %v", err)
	}
	
	// Initialize execution queue service for async agent execution
	executionQueueSvc := services.NewExecutionQueueService(repos, agentSvc, 5) // 5 workers
	
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

	sshServer := ssh.New(cfg, database, executionQueueSvc, agentSvc)
	
	// Check if we're in local mode
	localMode := viper.GetBool("local_mode")
	mcpServer := mcp.NewServer(database, mcpConfigSvc, agentSvc, repos, localMode)
	apiServer := api.New(cfg, database, localMode)
	
	// Initialize ToolDiscoveryService for API config uploads
	toolDiscoveryService := services.NewToolDiscoveryService(repos, mcpConfigSvc)
	
	// Set services for the API server
	apiServer.SetServices(toolDiscoveryService, agentSvc)

	wg.Add(3)

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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("✅ All servers stopped gracefully")
	case <-shutdownCtx.Done():
		fmt.Println("⏰ Shutdown timeout exceeded, forcing exit")
	}

	return nil
}

// ensureDefaultEnvironment creates a default environment if none exists
func ensureDefaultEnvironment(ctx context.Context, repos *repositories.Repositories) error {
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
	defaultEnv, err := repos.Environments.Create("default", &description)
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