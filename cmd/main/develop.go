package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	stationGenkit "station/internal/genkit"
	"station/internal/logging"
	"station/internal/services"
)

// runDevelop implements the "stn develop" command
func runDevelop(cmd *cobra.Command, args []string) error {
	// Get command flags
	environment, _ := cmd.Flags().GetString("env")
	port, _ := cmd.Flags().GetInt("port")
	aiModel, _ := cmd.Flags().GetString("ai-model")
	aiProvider, _ := cmd.Flags().GetString("ai-provider")
	verbose, _ := cmd.Flags().GetBool("verbose")

	// Show banner
	styles := getCLIStyles(themeManager)
	banner := styles.Banner.Render("🧪 Station Development Playground")
	fmt.Println(banner)

	fmt.Printf("🌍 Environment: %s\n", environment)
	fmt.Printf("🚀 Starting development server on port %d...\n", port)
	fmt.Printf("🤖 AI Provider: %s, Model: %s\n", aiProvider, aiModel)
	fmt.Printf("🔧 Verbose: %v\n", verbose)
	
	ctx := context.Background()
	
	// Initialize database and services
	databasePath := viper.GetString("database_url")
	if databasePath == "" {
		configDir := getWorkspacePath()
		databasePath = filepath.Join(configDir, "station.db")
	}
	
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()
	
	repos := repositories.New(database)
	
	// Get environment ID
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found: %w", environment, err)
	}
	
	fmt.Printf("📁 Loading agents and MCP configs from environment: %s (ID: %d)\n", env.Name, env.ID)
	
	// Initialize GenKit with environment-specific agents directory for automatic dotprompt loading
	workspacePath := getWorkspacePath()
	agentsDir := filepath.Join(workspacePath, "environments", environment, "agents")
	
	// Check if agents directory exists
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		fmt.Printf("⚠️  Agents directory does not exist: %s\n", agentsDir)
		fmt.Printf("💡 Create some agents first using: stn agent create\n")
		fmt.Printf("📖 Or export existing agents using: stn agent export <id>\n")
	}
	
	genkitApp, err := initializeGenKitWithPromptDir(ctx, agentsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize GenKit with prompt directory: %w", err)
	}
	
	fmt.Printf("📁 GenKit initialized with prompt directory: %s\n", agentsDir)
	
	// Load MCP tools
	mcpManager := services.NewMCPConnectionManager(repos, genkitApp)
	mcpTools, mcpClients, err := mcpManager.GetEnvironmentMCPTools(ctx, env.ID)
	if err != nil {
		return fmt.Errorf("failed to load MCP tools: %w", err)
	}
	defer mcpManager.CleanupConnections(mcpClients)
	
	fmt.Printf("🔧 Loaded %d MCP tools from %d servers\n", len(mcpTools), len(mcpClients))
	fmt.Printf("🤖 Agent prompts automatically loaded from: %s\n", agentsDir)
	
	// List loaded MCP tools
	for _, tool := range mcpTools {
		// MCP tools are already registered in GenKit by the MCP plugin
		fmt.Printf("   ✅ MCP Tool: %s\n", tool.Name())
	}
	
	fmt.Println()
	fmt.Println("🎉 Station Development Playground is ready!")
	fmt.Printf("📖 To start the Genkit developer UI, run:\n")
	fmt.Printf("   genkit start -o -- stn develop --env %s --port %d\n", environment, port)
	fmt.Println()
	fmt.Println("🧪 This will start the interactive testing UI at http://localhost:4000")
	fmt.Println("🔧 All your agents and MCP tools will be available for testing")
	fmt.Println("✨ Agent input schemas from .prompt files will be properly loaded")
	fmt.Println()
	fmt.Println("For now, Station development playground setup is complete.")
	fmt.Println("Your agents and tools are loaded in Genkit and ready to use.")
	
	// Keep the process alive to maintain MCP connections
	fmt.Println()
	fmt.Println("Press Ctrl+C to exit and cleanup MCP connections...")
	
	// Block indefinitely until interrupted
	select {}
	
	return nil
}

// initializeGenKitWithPromptDir initializes GenKit with a custom prompt directory
// This allows automatic loading of .prompt files with proper schema parsing
func initializeGenKitWithPromptDir(ctx context.Context, promptDir string) (*genkit.Genkit, error) {
	// Load Station configuration for AI provider setup
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	logging.Info("Initializing GenKit for development with prompt directory: %s", promptDir)
	logging.Info("AI Provider: %s, Model: %s", cfg.AIProvider, cfg.AIModel)

	// Initialize GenKit with AI provider and prompt directory
	var genkitApp *genkit.Genkit
	switch strings.ToLower(cfg.AIProvider) {
	case "openai":
		logging.Debug("Setting up Station's OpenAI plugin for development")
		
		stationOpenAI := &stationGenkit.StationOpenAI{
			APIKey: cfg.AIAPIKey,
		}
		
		if cfg.AIBaseURL != "" {
			stationOpenAI.BaseURL = cfg.AIBaseURL
			logging.Debug("Using custom OpenAI base URL: %s", cfg.AIBaseURL)
		}
		
		genkitApp = genkit.Init(ctx, 
			genkit.WithPlugins(stationOpenAI),
			genkit.WithPromptDir(promptDir),
		)
		err = nil // GenKit v1.0.1 Init doesn't return error
		
	case "googlegenai", "gemini":
		logging.Debug("Setting up Google AI plugin for development")
		
		geminiPlugin := &googlegenai.GoogleAI{}
		
		genkitApp = genkit.Init(ctx, 
			genkit.WithPlugins(geminiPlugin),
			genkit.WithPromptDir(promptDir),
		)
		err = nil // GenKit v1.0.1 Init doesn't return error
		
	default:
		return nil, fmt.Errorf("unsupported AI provider for development: %s", cfg.AIProvider)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GenKit for development: %w", err)
	}
	
	logging.Info("Prompts automatically loaded from directory: %s", promptDir)
	
	return genkitApp, nil
}