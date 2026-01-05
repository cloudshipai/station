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
	"station/internal/genkit/anthropic_oauth"
	"station/internal/lattice"
	"station/internal/lighthouse"
	lighthouseServices "station/internal/lighthouse/services"
	"station/internal/mcp"
	"station/internal/mcp_agents"
	"station/internal/notifications"
	"station/internal/services"
	"station/internal/workflows"
	"station/internal/workflows/runtime"
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
	app             *genkit.Genkit
	openaiPlugin    *openai.OpenAI // Official GenKit v1.0.1 OpenAI plugin
	geminiPlugin    *googlegenai.GoogleAI
	anthropicPlugin *anthropic_oauth.AnthropicOAuth
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
	case "anthropic":
		log.Printf("Setting up Anthropic plugin with model: %s, auth_type: %s", cfg.AIModel, cfg.AIAuthType)

		var anthropicPlugin *anthropic_oauth.AnthropicOAuth

		if cfg.AIAuthType == "oauth" && cfg.AIOAuthToken != "" {
			log.Printf("Using native Anthropic plugin with OAuth authentication (full tool support)")
			anthropicPlugin = &anthropic_oauth.AnthropicOAuth{
				OAuthToken: cfg.AIOAuthToken,
			}
		} else if cfg.AIAPIKey != "" {
			log.Printf("Using native Anthropic plugin with API key authentication (full tool support)")
			anthropicPlugin = &anthropic_oauth.AnthropicOAuth{
				APIKey: cfg.AIAPIKey,
			}
		} else {
			return nil, fmt.Errorf("Anthropic provider requires either OAuth token (ai_oauth_token) or API key (ANTHROPIC_API_KEY)")
		}

		genkitApp = genkit.Init(ctx, genkit.WithPlugins(anthropicPlugin))
		return &genkitSetup{
			app:             genkitApp,
			anthropicPlugin: anthropicPlugin,
		}, nil
	case "ollama":
		// For now, main server only supports OpenAI - Ollama support will be added
		return nil, fmt.Errorf("Ollama provider not yet supported in main server (use OpenAI for now)")
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s (supported: openai, gemini, anthropic, ollama)", cfg.AIProvider)
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

	// Apply smart telemetry defaults based on CloudShip connection status
	// - With CloudShip registration key: use telemetry.cloudshipai.com
	// - Without CloudShip: use local Jaeger (localhost:4318)
	cfg.ApplyTelemetryDefaults(false) // false = serve mode, not stdio

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

	// Bundle bootstrap: If STN_BUNDLE_ID is set, download and install from CloudShip
	// This runs BEFORE DeclarativeSync so the bundle files are in place for sync
	if bundleBootstrapped, err := services.CheckAndBootstrap(ctx, cfg, repos); err != nil {
		return fmt.Errorf("bundle bootstrap failed: %w", err)
	} else if bundleBootstrapped {
		log.Printf("🎯 Bundle bootstrap completed, continuing with server startup")
	}

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
	// Always runs sync to ensure agents and MCP servers are loaded from file configs
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

	// Initialize workflow engine FIRST for NATS-based workflow execution
	// IMPORTANT: This must happen before AgentService creation because the
	// CodingToolFactory needs the embedded NATS server to be running
	workflowOpts := runtime.EnvOptions()
	workflowEngine, err := runtime.NewEngine(workflowOpts)
	if err != nil {
		log.Printf("Warning: Failed to initialize workflow engine: %v (workflow execution disabled)", err)
	}

	workflowTelemetry, _ := runtime.NewWorkflowTelemetry()

	var workflowService *services.WorkflowService
	if workflowEngine != nil {
		workflowService = services.NewWorkflowServiceWithEngine(repos, workflowEngine)
		workflowService.SetTelemetry(workflowTelemetry)
		log.Printf("✅ Workflow engine initialized (embedded NATS)")
	} else {
		workflowService = services.NewWorkflowService(repos)
		log.Printf("⚠️  Workflow engine not available - workflows will not execute")
	}

	// Initialize agent service with AgentExecutionEngine and Lighthouse integration
	// Pass the global otelTelemetryService to avoid creating a duplicate TracerProvider
	// This ensures org_id from CloudShip auth is properly added to all spans
	agentSvc := services.NewAgentServiceWithLighthouse(repos, lighthouseClient, otelTelemetryService)

	// Initialize MCP for the agent service
	if err := agentSvc.InitializeMCP(ctx); err != nil {
		log.Printf("Warning: Failed to initialize MCP for agent service: %v", err)
	}

	schedulerSvc := services.NewSchedulerService(database, repos, agentSvc)
	if err := schedulerSvc.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler service: %w", err)
	}
	defer schedulerSvc.Stop()

	workflowSchedulerSvc := services.NewWorkflowSchedulerService(repos, workflowService)
	if err := workflowSchedulerSvc.Start(ctx); err != nil {
		return fmt.Errorf("failed to start workflow scheduler service: %w", err)
	}
	defer workflowSchedulerSvc.Stop()

	// Start workflow consumer to process workflow steps
	var workflowConsumer *runtime.WorkflowConsumer
	if workflowEngine != nil {
		workflowConsumer = startServerWorkflowConsumer(ctx, repos, workflowEngine, agentSvc, workflowTelemetry, cfg, database)
		if workflowConsumer != nil {
			log.Printf("✅ Workflow consumer started")
		}
	}

	syncer.SetWorkflowScheduler(workflowSchedulerSvc)

	// Initialize remote control service for server mode CloudShip integration
	var remoteControlSvc *lighthouseServices.RemoteControlService
	if lighthouseClient != nil && lighthouseClient.GetMode() == lighthouse.ModeServe {
		log.Printf("🌐 Initializing server mode remote control via CloudShip")

		// Use v2 config if station name is provided
		remoteControlConfig := lighthouseServices.RemoteControlConfig{
			RegistrationKey: cfg.CloudShip.RegistrationKey,
			Environment:     environmentName,
			StationName:     cfg.CloudShip.Name,
			StationTags:     cfg.CloudShip.Tags,
		}

		if cfg.CloudShip.Name != "" {
			log.Printf("🚀 Using v2 auth flow: station_name=%s tags=%v", cfg.CloudShip.Name, cfg.CloudShip.Tags)
		}

		remoteControlSvc = lighthouseServices.NewRemoteControlServiceWithConfig(
			lighthouseClient,
			agentSvc,
			repos,
			remoteControlConfig,
		)

		// Wire up auth callback to update telemetry with CloudShip info (org_id, station_id)
		// This ensures traces are tagged with org/station for multi-tenant filtering
		// NOTE: We update BOTH the global otelTelemetryService AND the agentSvc's internal telemetry
		remoteControlSvc.SetOnAuthSuccess(func(stationID, stationName, orgID string) {
			// Update global telemetry service (used by non-agent spans)
			if otelTelemetryService != nil {
				otelTelemetryService.SetCloudShipInfo(stationID, stationName, orgID)
			}
			// Update agent service's telemetry (used by agent execution spans)
			agentSvc.SetTelemetryCloudShipInfo(stationID, stationName, orgID)
		})
		log.Printf("✅ Telemetry auth callback configured for CloudShip trace filtering")

		// Start remote control service
		if err := remoteControlSvc.Start(ctx); err != nil {
			log.Printf("Warning: Failed to start remote control service: %v", err)
		} else {
			log.Printf("✅ Server mode remote control active - CloudShip can manage this Station")

			// Wire up CloudShip memory client for memory integration
			if memoryClient := remoteControlSvc.GetMemoryClient(); memoryClient != nil {
				agentSvc.SetMemoryClient(memoryClient)
				log.Printf("✅ CloudShip memory integration configured")
			}
		}
	}

	localMode := viper.GetBool("local_mode")

	var latticeEmbedded *lattice.EmbeddedServer
	var latticeClient *lattice.Client
	var latticeRegistry *lattice.Registry
	var latticePresence *lattice.Presence
	var latticeInvoker *lattice.Invoker

	latticeOrchestration := viper.GetBool("lattice_orchestration")
	latticeURL := viper.GetString("lattice_url")

	if latticeOrchestration || latticeURL != "" {
		log.Printf("🔗 Initializing Station Lattice mesh network...")

		var natsURL string

		if latticeOrchestration {
			natsPort := viper.GetInt("lattice.orchestrator.embedded_nats.port")
			if natsPort == 0 {
				natsPort = 4222
			}
			natsHTTPPort := viper.GetInt("lattice.orchestrator.embedded_nats.http_port")
			if natsHTTPPort == 0 {
				natsHTTPPort = 8222
			}

			embeddedCfg := config.LatticeEmbeddedNATSConfig{
				Port:     natsPort,
				HTTPPort: natsHTTPPort,
			}
			latticeEmbedded = lattice.NewEmbeddedServer(embeddedCfg)
			if err := latticeEmbedded.Start(); err != nil {
				log.Printf("⚠️  Failed to start embedded NATS server: %v", err)
				log.Printf("⚠️  Lattice mesh network disabled")
			} else {
				natsURL = latticeEmbedded.ClientURL()
				log.Printf("✅ Lattice orchestrator mode: embedded NATS on port %d", natsPort)
			}
		} else if latticeURL != "" {
			natsURL = latticeURL
			log.Printf("✅ Lattice client mode: connecting to %s", latticeURL)
		}

		if natsURL != "" {
			latticeCfg := config.LatticeConfig{
				StationName: cfg.CloudShip.Name,
				NATS: config.LatticeNATSConfig{
					URL: natsURL,
				},
			}

			var err error
			latticeClient, err = lattice.NewClient(latticeCfg)
			if err != nil {
				log.Printf("⚠️  Failed to create lattice client: %v", err)
			} else {
				if err := latticeClient.Connect(); err != nil {
					log.Printf("⚠️  Failed to connect to lattice NATS: %v", err)
					latticeClient = nil
				} else {
					log.Printf("✅ Connected to lattice NATS at %s (station ID: %s)", natsURL, latticeClient.StationID())

					latticeRegistry = lattice.NewRegistry(latticeClient)
					if err := latticeRegistry.Initialize(ctx); err != nil {
						log.Printf("⚠️  Failed to initialize lattice registry: %v", err)
					} else {
						log.Printf("✅ Lattice registry initialized")

						manifestCollector := lattice.NewManifestCollector(database.Conn())
						manifest, err := manifestCollector.CollectFullManifest(ctx, latticeClient.StationID(), latticeClient.StationName())
						if err != nil {
							log.Printf("⚠️  Failed to collect station manifest: %v", err)
						} else {
							if err := latticeRegistry.RegisterStation(ctx, *manifest); err != nil {
								log.Printf("⚠️  Failed to register station: %v", err)
							} else {
								log.Printf("✅ Station registered with %d agents and %d workflows", len(manifest.Agents), len(manifest.Workflows))

								latticePresence = lattice.NewPresence(latticeClient, latticeRegistry, *manifest, 10)
								if err := latticePresence.Start(ctx); err != nil {
									log.Printf("⚠️  Failed to start lattice presence: %v", err)
								} else {
									log.Printf("✅ Lattice presence heartbeat started")
								}

								executorAdapter := lattice.NewExecutorAdapter(agentSvc, repos, database.Conn())
								latticeInvoker = lattice.NewInvoker(latticeClient, latticeClient.StationID(), executorAdapter)

								if workflowService != nil {
									workflowExecutorAdapter := lattice.NewWorkflowExecutorAdapter(workflowService, repos)
									latticeInvoker.SetWorkflowExecutor(workflowExecutorAdapter)
									log.Printf("✅ Lattice workflow executor configured")
								}

								if err := latticeInvoker.Start(ctx); err != nil {
									log.Printf("⚠️  Failed to start lattice invoker: %v", err)
								} else {
									log.Printf("✅ Lattice invoker listening for remote agent/workflow requests")
								}
							}
						}
					}
				}
			}
		}
	}

	log.Printf("🤖 Serving agents from environment: %s", environmentName)

	mcpServer := mcp.NewServer(database, agentSvc, repos, cfg, localMode)
	// Set lighthouse client for IngestData dual flow
	if lighthouseClient != nil {
		mcpServer.SetLighthouseClient(lighthouseClient)
		log.Printf("✅ Lighthouse client configured for MCP server IngestData dual flow")
	}
	dynamicAgentServer := mcp_agents.NewDynamicAgentServerWithConfig(repos, agentSvc, localMode, environmentName, cfg)
	dynamicAgentServer.SetWorkflowService(workflowService)
	apiServer := api.New(cfg, database, localMode, nil)

	// Initialize ToolDiscoveryService for lighthouse and API compatibility
	toolDiscoveryService := services.NewToolDiscoveryService(repos)

	// Set services for the API server
	apiServer.SetServices(toolDiscoveryService)
	// Share agent service with API server so CloudShip telemetry info propagates to traces
	apiServer.SetAgentService(agentSvc)
	// Share workflow components so APIHandlers uses the SAME engine (avoids duplicate engines)
	if workflowEngine != nil {
		apiServer.SetWorkflowComponents(workflowService, workflowEngine, workflowTelemetry)
	}

	apiServer.InitializeHandlers()
	apiServer.SetWorkflowScheduler(workflowSchedulerSvc)

	devMode := viper.GetBool("dev_mode")

	// Conditional goroutine count based on dev mode
	if devMode {
		wg.Add(4) // MCP, Dynamic Agent MCP, API, and webhook retry processor
	} else {
		wg.Add(3) // MCP, Dynamic Agent MCP, and webhook retry processor (no API)
	}

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
		// Dynamic Agent MCP port is Management MCP port + 1 (e.g., 8586 + 1 = 8587)
		dynamicAgentPort := cfg.MCPPort + 1
		log.Printf("🤖 Starting Dynamic Agent MCP server on port %d", dynamicAgentPort)
		if err := dynamicAgentServer.Start(ctx, dynamicAgentPort); err != nil {
			log.Printf("Dynamic Agent MCP server error: %v", err)
		}
	}()

	// Only start API/UI server in dev mode
	if devMode {
		go func() {
			defer wg.Done()
			if err := apiServer.Start(ctx); err != nil {
				log.Printf("API server error: %v", err)
			}
		}()
	}

	// Remove telemetry tracking

	fmt.Printf("\n✅ Station is running!\n")
	fmt.Printf("🔧 MCP Server: http://localhost:%d/mcp\n", cfg.MCPPort)
	fmt.Printf("🤖 Dynamic Agent MCP: http://localhost:%d/mcp (environment: %s)\n", cfg.MCPPort+1, environmentName)

	if devMode {
		fmt.Printf("🌐 API Server: http://localhost:%d (DEV MODE)\n", cfg.APIPort)
	} else {
		fmt.Printf("🔒 API Server: Disabled (production mode)\n")
		fmt.Printf("💡 Set STN_DEV_MODE=true to enable management UI\n")
	}

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

	if remoteControlSvc != nil {
		if err := remoteControlSvc.Stop(); err != nil {
			log.Printf("Error stopping remote control service: %v", err)
		} else {
			log.Printf("🌐 Remote control service stopped gracefully")
		}
	}

	if latticeInvoker != nil {
		latticeInvoker.Stop()
		log.Printf("🔗 Lattice invoker stopped")
	}

	if latticePresence != nil {
		latticePresence.Stop()
		log.Printf("🔗 Lattice presence stopped")
	}

	if latticeClient != nil {
		latticeClient.Close()
		log.Printf("🔗 Lattice client disconnected")
	}

	if latticeEmbedded != nil {
		latticeEmbedded.Shutdown()
		log.Printf("🔗 Lattice embedded NATS server stopped")
	}

	if workflowConsumer != nil {
		log.Printf("🛑 Shutting down workflow consumer...")
		workflowConsumer.Stop()
	}

	if workflowEngine != nil {
		log.Printf("🛑 Shutting down workflow engine...")
		workflowEngine.Close()
	}

	// Stop Lighthouse client
	if lighthouseClient != nil {
		if err := lighthouseClient.Close(); err != nil {
			log.Printf("Error stopping Lighthouse client: %v", err)
		}
	}

	// Jaeger/tracing managed externally via config.yaml otel_endpoint

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

func startServerWorkflowConsumer(
	ctx context.Context,
	repos *repositories.Repositories,
	engine *runtime.NATSEngine,
	agentService services.AgentServiceInterface,
	telemetry *runtime.WorkflowTelemetry,
	cfg *config.Config,
	database *db.DB,
) *runtime.WorkflowConsumer {
	registry := runtime.NewExecutorRegistry()
	registry.Register(runtime.NewInjectExecutor())
	registry.Register(runtime.NewSwitchExecutor())
	registry.Register(runtime.NewAgentRunExecutor(&serverAgentExecutorAdapter{agentService: agentService, repos: repos}))

	approvalExecutor := runtime.NewHumanApprovalExecutor(&serverApprovalExecutorAdapter{repos: repos})
	if cfg != nil && database != nil {
		notifier := notifications.NewWebhookNotifier(cfg, database.Conn())
		approvalExecutor.SetNotifier(notifier)
	}
	registry.Register(approvalExecutor)
	registry.Register(runtime.NewCustomExecutor(nil))
	registry.Register(runtime.NewCronExecutor())
	registry.Register(runtime.NewTimerExecutor())
	registry.Register(runtime.NewTryCatchExecutor(registry))
	registry.Register(runtime.NewTransformExecutor())

	stepAdapter := &serverRegistryStepExecutorAdapter{registry: registry}
	registry.Register(runtime.NewParallelExecutor(stepAdapter))
	registry.Register(runtime.NewForeachExecutor(stepAdapter))

	adapter := runtime.NewWorkflowServiceAdapter(repos, engine)
	if telemetry != nil {
		adapter.SetTelemetry(telemetry)
	}

	consumer := runtime.NewWorkflowConsumer(engine, registry, adapter, adapter, adapter)
	consumer.SetPendingRunProvider(adapter)
	if telemetry != nil {
		consumer.SetTelemetry(telemetry)
	}

	if err := consumer.Start(ctx); err != nil {
		log.Printf("Workflow consumer: failed to start: %v", err)
		return nil
	}

	log.Println("Workflow consumer started for server mode")
	return consumer
}

type serverAgentExecutorAdapter struct {
	agentService services.AgentServiceInterface
	repos        *repositories.Repositories
}

func (a *serverAgentExecutorAdapter) GetAgentByID(ctx context.Context, id int64) (runtime.AgentInfo, error) {
	agent, err := a.agentService.GetAgent(ctx, id)
	if err != nil {
		return runtime.AgentInfo{}, err
	}
	return runtime.AgentInfo{
		ID:           agent.ID,
		Name:         agent.Name,
		InputSchema:  agent.InputSchema,
		OutputSchema: agent.OutputSchema,
	}, nil
}

func (a *serverAgentExecutorAdapter) GetAgentByNameAndEnvironment(ctx context.Context, name string, environmentID int64) (runtime.AgentInfo, error) {
	agent, err := a.repos.Agents.GetByNameAndEnvironment(name, environmentID)
	if err != nil {
		return runtime.AgentInfo{}, err
	}
	return runtime.AgentInfo{
		ID:           agent.ID,
		Name:         agent.Name,
		InputSchema:  agent.InputSchema,
		OutputSchema: agent.OutputSchema,
	}, nil
}

func (a *serverAgentExecutorAdapter) GetAgentByNameGlobal(ctx context.Context, name string) (runtime.AgentInfo, error) {
	agent, err := a.repos.Agents.GetByNameGlobal(name)
	if err != nil {
		return runtime.AgentInfo{}, err
	}
	return runtime.AgentInfo{
		ID:           agent.ID,
		Name:         agent.Name,
		InputSchema:  agent.InputSchema,
		OutputSchema: agent.OutputSchema,
	}, nil
}

func (a *serverAgentExecutorAdapter) GetEnvironmentIDByName(ctx context.Context, name string) (int64, error) {
	env, err := a.repos.Environments.GetByName(name)
	if err != nil {
		return 0, err
	}
	return env.ID, nil
}

func (a *serverAgentExecutorAdapter) ExecuteAgent(ctx context.Context, agentID int64, task string, variables map[string]interface{}) (runtime.AgentExecutionResult, error) {
	userID := int64(1)

	agentRun, err := a.repos.AgentRuns.Create(ctx, agentID, userID, task, "", 0, nil, nil, "running", nil)
	if err != nil {
		log.Printf("Workflow agent step: Failed to create agent run: %v", err)
		return runtime.AgentExecutionResult{}, err
	}

	result, err := a.agentService.ExecuteAgentWithRunID(ctx, agentID, task, agentRun.ID, variables)
	if err != nil {
		log.Printf("Workflow agent step: Execution failed for run %d: %v", agentRun.ID, err)
		completedAt := time.Now()
		errorMsg := err.Error()
		a.repos.AgentRuns.UpdateCompletionWithMetadata(
			ctx, agentRun.ID, errorMsg, 0, nil, nil, "failed", &completedAt,
			nil, nil, nil, nil, nil, nil, &errorMsg,
		)
		return runtime.AgentExecutionResult{}, err
	}

	log.Printf("Workflow agent step: Completed run %d for agent %d", agentRun.ID, agentID)

	completedAt := time.Now()
	var inputTokens, outputTokens, totalTokens *int64
	var durationSeconds *float64
	var modelName *string
	var stepsTaken int64

	if result.Extra != nil {
		if tokenUsage, ok := result.Extra["token_usage"].(map[string]interface{}); ok {
			if val, ok := tokenUsage["input_tokens"].(float64); ok {
				v := int64(val)
				inputTokens = &v
			}
			if val, ok := tokenUsage["output_tokens"].(float64); ok {
				v := int64(val)
				outputTokens = &v
			}
			if val, ok := tokenUsage["total_tokens"].(float64); ok {
				v := int64(val)
				totalTokens = &v
			}
		}
		if dur, ok := result.Extra["duration_seconds"].(float64); ok {
			durationSeconds = &dur
		}
		if model, ok := result.Extra["model_name"].(string); ok {
			modelName = &model
		}
		if steps, ok := result.Extra["steps_taken"].(int64); ok {
			stepsTaken = steps
		} else if steps, ok := result.Extra["steps_taken"].(float64); ok {
			stepsTaken = int64(steps)
		}
	}

	a.repos.AgentRuns.UpdateCompletionWithMetadata(
		ctx, agentRun.ID, result.Content, stepsTaken, nil, nil, "completed", &completedAt,
		inputTokens, outputTokens, totalTokens, durationSeconds, modelName, nil, nil,
	)

	return runtime.AgentExecutionResult{
		Response:  result.Content,
		StepCount: stepsTaken,
		ToolsUsed: 0,
	}, nil
}

type serverApprovalExecutorAdapter struct {
	repos *repositories.Repositories
}

func (a *serverApprovalExecutorAdapter) CreateApproval(ctx context.Context, params runtime.CreateApprovalParams) (runtime.ApprovalInfo, error) {
	var summaryPath *string
	if params.SummaryPath != "" {
		summaryPath = &params.SummaryPath
	}

	var approvers *string
	if len(params.Approvers) > 0 {
		joined := ""
		for i, ap := range params.Approvers {
			if i > 0 {
				joined += ","
			}
			joined += ap
		}
		approvers = &joined
	}

	var timeoutAt *time.Time
	if params.TimeoutSecs > 0 {
		t := time.Now().Add(time.Duration(params.TimeoutSecs) * time.Second)
		timeoutAt = &t
	}

	approval, err := a.repos.WorkflowApprovals.Create(ctx, repositories.CreateWorkflowApprovalParams{
		ApprovalID:  params.ApprovalID,
		RunID:       params.RunID,
		StepID:      params.StepID,
		Message:     params.Message,
		SummaryPath: summaryPath,
		Approvers:   approvers,
		TimeoutAt:   timeoutAt,
	})
	if err != nil {
		return runtime.ApprovalInfo{}, err
	}

	return runtime.ApprovalInfo{
		ID:     approval.ApprovalID,
		Status: approval.Status,
	}, nil
}

func (a *serverApprovalExecutorAdapter) GetApproval(ctx context.Context, approvalID string) (runtime.ApprovalInfo, error) {
	approval, err := a.repos.WorkflowApprovals.Get(ctx, approvalID)
	if err != nil {
		return runtime.ApprovalInfo{}, err
	}

	info := runtime.ApprovalInfo{
		ID:     approval.ApprovalID,
		Status: approval.Status,
	}
	if approval.DecidedBy != nil {
		info.DecidedBy = *approval.DecidedBy
	}
	if approval.DecisionReason != nil {
		info.DecisionReason = *approval.DecisionReason
	}
	return info, nil
}

type serverRegistryStepExecutorAdapter struct {
	registry *runtime.ExecutorRegistry
}

func (a *serverRegistryStepExecutorAdapter) ExecuteStep(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (runtime.StepResult, error) {
	executor, err := a.registry.GetExecutor(step.Type)
	if err != nil {
		errStr := err.Error()
		return runtime.StepResult{
			Status: runtime.StepStatusFailed,
			Error:  &errStr,
		}, err
	}
	return executor.Execute(ctx, step, runContext)
}
