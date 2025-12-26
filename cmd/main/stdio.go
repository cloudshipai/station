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
	"time"

	"station/internal/api"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/lighthouse"
	lighthouseServices "station/internal/lighthouse/services"
	"station/internal/logging"
	"station/internal/mcp"
	"station/internal/services"
	"station/internal/workflows"
	"station/internal/workflows/runtime"

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
	// Jaeger removed - run separately if needed. Tracing configured via config.yaml otel_endpoint
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

	// Apply smart telemetry defaults for stdio mode (local development)
	// - Always uses local Jaeger (localhost:4318) unless explicitly overridden
	cfg.ApplyTelemetryDefaults(true) // true = stdio mode

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

	// Initialize remote control service for bidirectional management (serve mode only)
	// Note: stdio mode does NOT connect to CloudShip platform - it's for local MCP integration only
	var remoteControlSvc *lighthouseServices.RemoteControlService
	if lighthouseClient != nil && lighthouseClient.GetMode() == lighthouse.ModeServe {
		log.Printf("🌐 Initializing server mode remote control via CloudShip")

		// Use v2 config if station name is provided
		remoteControlConfig := lighthouseServices.RemoteControlConfig{
			RegistrationKey: cfg.CloudShip.RegistrationKey,
			Environment:     "default", // TODO: use actual environment name
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

		// Start remote control service with long-lived context to keep management channel active
		if err := remoteControlSvc.Start(longLivedCtx); err != nil {
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

	// Check if we're in local mode
	localMode := viper.GetBool("local_mode")

	workflowOpts := runtime.EnvOptions()
	workflowEngine, err := runtime.NewEngine(workflowOpts)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: Failed to initialize workflow engine: %v (workflow execution disabled)\n", err)
	}

	var workflowService *services.WorkflowService
	if workflowEngine != nil {
		workflowService = services.NewWorkflowServiceWithEngine(repos, workflowEngine)
		_, _ = fmt.Fprintf(os.Stderr, "✅ Workflow engine initialized (embedded NATS)\n")
	} else {
		workflowService = services.NewWorkflowService(repos)
	}

	var workflowConsumer *runtime.WorkflowConsumer
	if workflowEngine != nil {
		workflowConsumer = startStdioWorkflowConsumer(ctx, repos, workflowEngine, agentSvc)
		if workflowConsumer != nil {
			_, _ = fmt.Fprintf(os.Stderr, "✅ Workflow consumer started\n")
		}
	}

	mcpServer := mcp.NewServer(database, agentSvc, repos, cfg, localMode)
	mcpServer.SetWorkflowService(workflowService)

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

	if remoteControlSvc != nil {
		_, _ = fmt.Fprintf(os.Stderr, "🛑 Shutting down remote control service...\n")
		if err := remoteControlSvc.Stop(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "⚠️  Error stopping remote control service: %v\n", err)
		}
	}

	if workflowConsumer != nil {
		_, _ = fmt.Fprintf(os.Stderr, "🛑 Shutting down workflow consumer...\n")
		workflowConsumer.Stop()
	}

	if workflowEngine != nil {
		_, _ = fmt.Fprintf(os.Stderr, "🛑 Shutting down workflow engine...\n")
		workflowEngine.Close()
	}

	return nil
}

func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func startStdioWorkflowConsumer(ctx context.Context, repos *repositories.Repositories, engine *runtime.NATSEngine, agentService services.AgentServiceInterface) *runtime.WorkflowConsumer {
	registry := runtime.NewExecutorRegistry()
	registry.Register(runtime.NewInjectExecutor())
	registry.Register(runtime.NewSwitchExecutor())
	registry.Register(runtime.NewAgentRunExecutor(&stdioAgentExecutorAdapter{agentService: agentService, repos: repos}))
	registry.Register(runtime.NewHumanApprovalExecutor(&stdioApprovalExecutorAdapter{repos: repos}))
	registry.Register(runtime.NewCustomExecutor(nil))
	registry.Register(runtime.NewCronExecutor())
	registry.Register(runtime.NewTimerExecutor())
	registry.Register(runtime.NewTryCatchExecutor(registry))
	registry.Register(runtime.NewTransformExecutor())

	stepAdapter := &stdioRegistryStepExecutorAdapter{registry: registry}
	registry.Register(runtime.NewParallelExecutor(stepAdapter))
	registry.Register(runtime.NewForeachExecutor(stepAdapter))

	adapter := runtime.NewWorkflowServiceAdapter(repos, engine)

	consumer := runtime.NewWorkflowConsumer(engine, registry, adapter, adapter, adapter)
	consumer.SetPendingRunProvider(adapter)

	if err := consumer.Start(ctx); err != nil {
		log.Printf("Workflow consumer: failed to start: %v", err)
		return nil
	}

	log.Println("Workflow consumer started for stdio mode")
	return consumer
}

type stdioAgentExecutorAdapter struct {
	agentService services.AgentServiceInterface
	repos        *repositories.Repositories
}

func (a *stdioAgentExecutorAdapter) GetAgentByID(ctx context.Context, id int64) (runtime.AgentInfo, error) {
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

func (a *stdioAgentExecutorAdapter) GetAgentByNameAndEnvironment(ctx context.Context, name string, environmentID int64) (runtime.AgentInfo, error) {
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

func (a *stdioAgentExecutorAdapter) GetAgentByNameGlobal(ctx context.Context, name string) (runtime.AgentInfo, error) {
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

func (a *stdioAgentExecutorAdapter) GetEnvironmentIDByName(ctx context.Context, name string) (int64, error) {
	env, err := a.repos.Environments.GetByName(name)
	if err != nil {
		return 0, err
	}
	return env.ID, nil
}

func (a *stdioAgentExecutorAdapter) ExecuteAgent(ctx context.Context, agentID int64, task string, variables map[string]interface{}) (runtime.AgentExecutionResult, error) {
	userID := int64(1)

	agentRun, err := a.repos.AgentRuns.Create(ctx, agentID, userID, task, "", 0, nil, nil, "running", nil)
	if err != nil {
		log.Printf("❌ Workflow agent step: Failed to create agent run: %v", err)
		return runtime.AgentExecutionResult{}, err
	}

	result, err := a.agentService.ExecuteAgentWithRunID(ctx, agentID, task, agentRun.ID, variables)
	if err != nil {
		log.Printf("❌ Workflow agent step: Execution failed for run %d: %v", agentRun.ID, err)
		completedAt := time.Now()
		errorMsg := err.Error()
		a.repos.AgentRuns.UpdateCompletionWithMetadata(
			ctx, agentRun.ID, errorMsg, 0, nil, nil, "failed", &completedAt,
			nil, nil, nil, nil, nil, nil, &errorMsg,
		)
		return runtime.AgentExecutionResult{}, err
	}

	log.Printf("✅ Workflow agent step: Completed run %d for agent %d", agentRun.ID, agentID)

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

type stdioApprovalExecutorAdapter struct {
	repos *repositories.Repositories
}

func (a *stdioApprovalExecutorAdapter) CreateApproval(ctx context.Context, params runtime.CreateApprovalParams) (runtime.ApprovalInfo, error) {
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

func (a *stdioApprovalExecutorAdapter) GetApproval(ctx context.Context, approvalID string) (runtime.ApprovalInfo, error) {
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

type stdioRegistryStepExecutorAdapter struct {
	registry *runtime.ExecutorRegistry
}

func (a *stdioRegistryStepExecutorAdapter) ExecuteStep(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (runtime.StepResult, error) {
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
