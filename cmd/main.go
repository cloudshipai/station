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

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		log.Fatal("Failed to run database migrations:", err)
	}

	var wg sync.WaitGroup

	sshServer := ssh.New(cfg, database)
	
	// Create repositories and services
	repos := repositories.New(database)
	keyManager, err := crypto.NewKeyManagerFromEnv()
	if err != nil {
		log.Fatal("Failed to initialize key manager:", err)
	}

	// Initialize all required services
	mcpConfigSvc := services.NewMCPConfigService(repos, keyManager)
	modelProviderSvc, err := services.NewModelProviderBootService(repos, keyManager)
	if err != nil {
		log.Fatal("Failed to initialize model provider service:", err)
	}

	// Load model providers on startup
	if err := modelProviderSvc.LoadAndSyncProvidersOnBoot(ctx); err != nil {
		log.Fatal("Failed to load model providers:", err)
	}

	toolDiscoverySvc := services.NewToolDiscoveryService(repos, mcpConfigSvc)
	mcpClientSvc := services.NewMCPClientService(repos, mcpConfigSvc, toolDiscoverySvc)
	agentSvc := services.NewEinoAgentService(repos, mcpClientSvc, toolDiscoverySvc)
	
	mcpServer := mcp.NewServer(database, mcpConfigSvc, agentSvc, repos)
	apiServer := api.New(cfg, database)

	wg.Add(3)

	go func() {
		defer wg.Done()
		if err := sshServer.Start(ctx); err != nil {
			log.Printf("SSH server error: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := mcpServer.Start(ctx, cfg.MCPPort); err != nil {
			log.Printf("MCP server error: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := apiServer.Start(ctx); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	fmt.Println("\nReceived shutdown signal, gracefully shutting down...")

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
		fmt.Println("All servers stopped gracefully")
	case <-shutdownCtx.Done():
		fmt.Println("Shutdown timeout exceeded, forcing exit")
	}
}