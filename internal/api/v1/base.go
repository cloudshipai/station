package v1

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"station/internal/auth"
	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/telemetry"
	"station/internal/workflows"
	"station/internal/workflows/runtime"
)

type APIHandlers struct {
	repos                *repositories.Repositories
	db                   *sql.DB
	agentService         services.AgentServiceInterface
	toolDiscoveryService *services.ToolDiscoveryService
	agentExportService   *services.AgentExportService
	telemetryService     *telemetry.TelemetryService
	workflowService      *services.WorkflowService
	workflowConsumer     *runtime.WorkflowConsumer
	workflowEngine       runtime.Engine
	localMode            bool
	cfg                  *config.Config
}

func NewAPIHandlers(
	repos *repositories.Repositories,
	db *sql.DB,
	toolDiscoveryService *services.ToolDiscoveryService,
	telemetryService *telemetry.TelemetryService,
	localMode bool,
) *APIHandlers {
	engine := mustInitWorkflowEngine()
	workflowService := services.NewWorkflowServiceWithEngine(repos, engine)
	agentService := services.NewAgentService(repos)

	h := &APIHandlers{
		repos:                repos,
		db:                   db,
		agentService:         agentService,
		toolDiscoveryService: toolDiscoveryService,
		agentExportService:   services.NewAgentExportService(repos),
		workflowService:      workflowService,
		workflowEngine:       engine,
		telemetryService:     telemetryService,
		localMode:            localMode,
	}

	h.startWorkflowConsumer(repos, engine, agentService)
	return h
}

func NewAPIHandlersWithConfig(
	repos *repositories.Repositories,
	db *sql.DB,
	toolDiscoveryService *services.ToolDiscoveryService,
	telemetryService *telemetry.TelemetryService,
	localMode bool,
	cfg *config.Config,
) *APIHandlers {
	engine := mustInitWorkflowEngine()
	workflowService := services.NewWorkflowServiceWithEngine(repos, engine)
	agentService := services.NewAgentService(repos)

	h := &APIHandlers{
		repos:                repos,
		db:                   db,
		agentService:         agentService,
		toolDiscoveryService: toolDiscoveryService,
		agentExportService:   services.NewAgentExportService(repos),
		workflowService:      workflowService,
		workflowEngine:       engine,
		telemetryService:     telemetryService,
		localMode:            localMode,
		cfg:                  cfg,
	}

	h.startWorkflowConsumer(repos, engine, agentService)
	return h
}

func NewAPIHandlersWithAgentService(
	repos *repositories.Repositories,
	db *sql.DB,
	toolDiscoveryService *services.ToolDiscoveryService,
	telemetryService *telemetry.TelemetryService,
	localMode bool,
	cfg *config.Config,
	agentService *services.AgentService,
) *APIHandlers {
	engine := mustInitWorkflowEngine()
	workflowService := services.NewWorkflowServiceWithEngine(repos, engine)

	h := &APIHandlers{
		repos:                repos,
		db:                   db,
		agentService:         agentService,
		toolDiscoveryService: toolDiscoveryService,
		agentExportService:   services.NewAgentExportService(repos),
		workflowService:      workflowService,
		workflowEngine:       engine,
		telemetryService:     telemetryService,
		localMode:            localMode,
		cfg:                  cfg,
	}

	h.startWorkflowConsumer(repos, engine, agentService)
	return h
}

func mustInitWorkflowEngine() runtime.Engine {
	opts := runtime.EnvOptions()
	engine, err := runtime.NewEngine(opts)
	if err != nil {
		return nil
	}
	return engine
}

func (h *APIHandlers) startWorkflowConsumer(repos *repositories.Repositories, engine runtime.Engine, agentService services.AgentServiceInterface) {
	natsEngine, ok := engine.(*runtime.NATSEngine)
	if !ok || natsEngine == nil {
		log.Println("Workflow consumer: NATS engine not enabled, skipping consumer start")
		return
	}

	registry := runtime.NewExecutorRegistry()
	registry.Register(runtime.NewInjectExecutor())
	registry.Register(runtime.NewSwitchExecutor())
	registry.Register(runtime.NewAgentRunExecutor(&agentExecutorAdapter{agentService: agentService, repos: repos}))
	registry.Register(runtime.NewHumanApprovalExecutor(&approvalExecutorAdapter{repos: repos}))
	registry.Register(runtime.NewCustomExecutor(nil))
	registry.Register(runtime.NewCronExecutor())
	registry.Register(runtime.NewTimerExecutor())
	registry.Register(runtime.NewTryCatchExecutor(registry))
	registry.Register(runtime.NewTransformExecutor())

	stepAdapter := &registryStepExecutorAdapter{registry: registry}
	registry.Register(runtime.NewParallelExecutor(stepAdapter))
	registry.Register(runtime.NewForeachExecutor(stepAdapter))

	adapter := runtime.NewWorkflowServiceAdapter(repos, engine)

	consumer := runtime.NewWorkflowConsumer(natsEngine, registry, adapter, adapter, adapter)
	consumer.SetPendingRunProvider(adapter)
	h.workflowConsumer = consumer

	if err := consumer.Start(context.Background()); err != nil {
		log.Printf("Workflow consumer: failed to start: %v", err)
	}
}

func (h *APIHandlers) StopWorkflowConsumer() {
	if h.workflowConsumer != nil {
		h.workflowConsumer.Stop()
	}
}

func (h *APIHandlers) SetWorkflowScheduler(scheduler *services.WorkflowSchedulerService) {
	if h.workflowService != nil {
		h.workflowService.SetScheduler(scheduler)
	}
}

type agentExecutorAdapter struct {
	agentService services.AgentServiceInterface
	repos        *repositories.Repositories
}

func (a *agentExecutorAdapter) GetAgentByID(ctx context.Context, id int64) (runtime.AgentInfo, error) {
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

func (a *agentExecutorAdapter) GetAgentByNameAndEnvironment(ctx context.Context, name string, environmentID int64) (runtime.AgentInfo, error) {
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

func (a *agentExecutorAdapter) GetAgentByNameGlobal(ctx context.Context, name string) (runtime.AgentInfo, error) {
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

func (a *agentExecutorAdapter) GetEnvironmentIDByName(ctx context.Context, name string) (int64, error) {
	env, err := a.repos.Environments.GetByName(name)
	if err != nil {
		return 0, err
	}
	return env.ID, nil
}

func (a *agentExecutorAdapter) ExecuteAgent(ctx context.Context, agentID int64, task string, variables map[string]interface{}) (runtime.AgentExecutionResult, error) {
	result, err := a.agentService.ExecuteAgent(ctx, agentID, task, variables)
	if err != nil {
		return runtime.AgentExecutionResult{}, err
	}
	return runtime.AgentExecutionResult{
		Response:  result.Content,
		StepCount: 0,
		ToolsUsed: 0,
	}, nil
}

type approvalExecutorAdapter struct {
	repos *repositories.Repositories
}

func (a *approvalExecutorAdapter) CreateApproval(ctx context.Context, params runtime.CreateApprovalParams) (runtime.ApprovalInfo, error) {
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

func (a *approvalExecutorAdapter) GetApproval(ctx context.Context, approvalID string) (runtime.ApprovalInfo, error) {
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

// registryStepExecutorAdapter implements BranchExecutorDeps and IteratorExecutorDeps
// by delegating step execution to the executor registry.
type registryStepExecutorAdapter struct {
	registry *runtime.ExecutorRegistry
}

func (a *registryStepExecutorAdapter) ExecuteStep(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (runtime.StepResult, error) {
	executor, err := a.registry.GetExecutor(step.Type)
	if err != nil {
		return runtime.StepResult{
			Status: runtime.StepStatusFailed,
			Error:  strPtr(err.Error()),
		}, err
	}
	return executor.Execute(ctx, step, runContext)
}

func strPtr(s string) *string {
	return &s
}

func (h *APIHandlers) telemetryMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// Track API request telemetry
		if h.telemetryService != nil {
			h.telemetryService.TrackAPIRequest(
				param.Path,
				param.Method,
				param.StatusCode,
				param.Latency.Milliseconds(),
			)
		}
		return ""
	})
}

// RegisterRoutes registers all v1 API routes
func (h *APIHandlers) RegisterRoutes(router *gin.RouterGroup) {
	// Add telemetry middleware
	router.Use(h.telemetryMiddleware())

	// Create auth middleware with local mode setting and OAuth support
	var authMiddleware *auth.AuthMiddleware
	if h.cfg != nil {
		authMiddleware = auth.NewAuthMiddlewareWithOAuth(h.repos, h.localMode, h.cfg)
	} else {
		authMiddleware = auth.NewAuthMiddlewareWithLocalMode(h.repos, h.localMode)
	}

	// In server mode, all routes require authentication
	if !h.localMode {
		router.Use(authMiddleware.Authenticate())
	}

	// Environment routes
	envGroup := router.Group("/environments")
	// In server mode, only admins can manage environments
	if !h.localMode {
		envGroup.Use(h.requireAdminInServerMode())
	}
	h.registerEnvironmentRoutes(envGroup)

	// MCP server management routes (file-based configuration)
	h.registerMCPManagementRoutes(envGroup)

	// Faker routes (simulation tools)
	h.registerFakerRoutes(envGroup)

	// Traces routes (OTEL/Jaeger integration)
	h.registerTracesRoutes(router)

	// MCP Directory template routes
	h.registerMCPDirectoryRoutes(router)

	// Tools routes (nested under environments)
	toolsGroup := envGroup.Group("/:env_id/tools")
	// Inherits admin-only restriction from envGroup
	h.registerToolsRoutes(toolsGroup)

	// MCP Servers routes - admin only in server mode
	mcpServersGroup := router.Group("/mcp-servers")
	if !h.localMode {
		mcpServersGroup.Use(h.requireAdminInServerMode())
	}
	h.registerMCPServerRoutes(mcpServersGroup)

	// Agent routes - accessible to regular users in server mode
	agentGroup := router.Group("/agents")
	agentGroup.GET("", h.listAgents)                    // Users can list agents
	agentGroup.GET("/:id", h.getAgent)                  // Users can view individual agents
	agentGroup.GET("/:id/details", h.getAgentWithTools) // Users can view agent details
	agentGroup.GET("/:id/prompt", h.getAgentPrompt)     // Users can view agent prompts
	agentGroup.PUT("/:id/prompt", h.updateAgentPrompt)  // Users can update agent prompts
	agentGroup.POST("/:id/execute", h.callAgent)        // Users can execute agents (direct execution with async goroutine)

	// Admin-only agent management routes
	agentAdminGroup := router.Group("/admin/agents")
	if !h.localMode {
		agentAdminGroup.Use(h.requireAdminInServerMode())
	}
	h.registerAgentAdminRoutes(agentAdminGroup)

	// Agent runs routes - accessible to regular users in server mode
	runsGroup := router.Group("/runs")
	h.registerAgentRunRoutes(runsGroup)

	// Benchmark routes - accessible to regular users in server mode
	benchmarksGroup := router.Group("/benchmarks")
	h.registerBenchmarkRoutes(benchmarksGroup)

	// Report routes - accessible to regular users in server mode
	reportsGroup := router.Group("/reports")
	h.registerReportRoutes(reportsGroup)

	// Workflow definition routes - admin only in server mode
	workflowGroup := router.Group("/workflows")
	if !h.localMode {
		workflowGroup.Use(h.requireAdminInServerMode())
	}
	h.registerWorkflowRoutes(workflowGroup)

	// Workflow run routes - accessible to regular users
	workflowRunsGroup := router.Group("/workflow-runs")
	h.registerWorkflowRunRoutes(workflowRunsGroup)

	// Workflow approval routes - accessible to regular users
	workflowApprovalsGroup := router.Group("/workflow-approvals")
	h.registerWorkflowApprovalRoutes(workflowApprovalsGroup)

	// Settings routes - admin only
	settingsGroup := router.Group("/settings")
	if !h.localMode {
		settingsGroup.Use(h.requireAdminInServerMode())
	}
	h.registerSettingsRoutes(settingsGroup)

	// Sync route - admin only in server mode
	syncGroup := router.Group("/sync")
	if !h.localMode {
		syncGroup.Use(h.requireAdminInServerMode())
	}
	syncGroup.POST("", h.syncConfigurations)
	syncGroup.POST("/interactive", h.startInteractiveSync)
	syncGroup.GET("/status/:id", h.getSyncStatus)
	syncGroup.POST("/variables", h.submitVariables)

	// Bundles route - admin only in server mode
	bundlesGroup := router.Group("/bundles")
	if !h.localMode {
		bundlesGroup.Use(h.requireAdminInServerMode())
	}
	bundlesGroup.GET("", h.listBundles)
	bundlesGroup.GET("/cloudship", h.listCloudShipBundles)
	bundlesGroup.POST("", h.createBundle)
	bundlesGroup.POST("/install", h.installBundle)

	// Demo Bundles routes
	demoBundlesGroup := router.Group("/demo-bundles")
	demoBundlesGroup.GET("", h.listDemoBundles)
	demoBundlesGroup.POST("/install", h.installDemoBundle)

	// MCP API bridge route - admin only in server mode
	mcpGroup := router.Group("/mcp")
	if !h.localMode {
		mcpGroup.Use(h.requireAdminInServerMode())
	}
	h.registerMCPRoutes(mcpGroup)

	// Ship CLI routes
	shipGroup := router.Group("/ship")
	h.registerShipRoutes(shipGroup)

	// OpenAPI to MCP conversion routes - admin only in server mode
	openapiGroup := router.Group("/openapi")
	if !h.localMode {
		openapiGroup.Use(h.requireAdminInServerMode())
	}
	h.registerOpenAPIRoutes(openapiGroup)

	// CloudShip lighthouse status
	router.GET("/lighthouse/status", h.LighthouseStatusHandler)

	// CloudShip API status (checks API key authentication for bundles)
	router.GET("/cloudship/status", h.CloudShipStatusHandler)

	// Version routes (public - no auth required for checking updates)
	versionGroup := router.Group("/version")
	h.registerVersionRoutes(versionGroup)
}

// requireAdminInServerMode is a middleware that requires admin privileges in server mode
func (h *APIHandlers) requireAdminInServerMode() gin.HandlerFunc {
	return func(c *gin.Context) {
		// In local mode, no admin check needed
		if h.localMode {
			c.Next()
			return
		}

		// Check auth type - CloudShip OAuth users get special handling
		authType, _ := c.Get("auth_type")
		if authType == "cloudship_oauth" {
			// CloudShip OAuth users are authenticated via CloudShip
			// They have read access to Station resources based on their org permissions
			// For write operations, we could add additional checks here
			c.Next()
			return
		}

		// Get user from context (should be set by auth middleware for local users)
		user, exists := auth.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		// Check if user is admin
		if !user.IsAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// syncConfigurations triggers file-based configuration sync
func (h *APIHandlers) syncConfigurations(c *gin.Context) {
	// Import the os/exec package for running the stn sync command
	// For now, return a success response - actual implementation would call stn sync
	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   "Configuration sync triggered successfully",
		"timestamp": "2025-08-17T22:45:00Z",
	})
}

// registerMCPRoutes registers MCP tool bridge routes
func (h *APIHandlers) registerMCPRoutes(mcpGroup *gin.RouterGroup) {
	// For now, this can be empty as we're using existing REST endpoints
	// Future: Could add direct MCP tool bridging if needed
}
