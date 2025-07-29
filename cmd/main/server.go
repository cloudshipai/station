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
)

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

	sshServer := ssh.New(cfg, database)
	
	// Create repositories and services
	repos := repositories.New(database)
	keyManager, err := crypto.NewKeyManagerFromEnv()
	if err != nil {
		return fmt.Errorf("failed to initialize key manager: %w", err)
	}

	// Initialize all required services
	mcpConfigSvc := services.NewMCPConfigService(repos, keyManager)
	modelProviderSvc, err := services.NewModelProviderBootService(repos, keyManager)
	if err != nil {
		return fmt.Errorf("failed to initialize model provider service: %w", err)
	}

	// Load model providers on startup
	if err := modelProviderSvc.LoadAndSyncProvidersOnBoot(ctx); err != nil {
		log.Printf("Warning: Failed to load model providers: %v", err)
	}
	
	// Create default environment if none exists
	if err := ensureDefaultEnvironment(ctx, repos); err != nil {
		log.Printf("Warning: Failed to create default environment: %v", err)
	}

	toolDiscoverySvc := services.NewToolDiscoveryService(repos, mcpConfigSvc)
	mcpClientSvc := services.NewMCPClientService(repos, mcpConfigSvc, toolDiscoverySvc)
	agentSvc := services.NewEinoAgentService(repos, mcpClientSvc, toolDiscoverySvc)
	
	mcpServer := mcp.NewServer(database, mcpConfigSvc, agentSvc, repos)
	apiServer := api.New(cfg, database)

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