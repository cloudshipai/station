package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/lighthouse"
	"station/internal/logging"
	dotprompt "station/pkg/dotprompt"
	"station/pkg/models"
	"station/pkg/schema"
	"station/pkg/types"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/plugins/mcp"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v2"
)

// AgentExecutionResult contains the result of an agent execution
type AgentExecutionResult struct {
	Success        bool                   `json:"success"`
	Response       string                 `json:"response"`
	ToolCalls      *models.JSONArray      `json:"tool_calls"`
	Steps          []interface{}          `json:"steps"`
	ExecutionSteps *models.JSONArray      `json:"execution_steps"` // For database storage
	Duration       time.Duration          `json:"duration"`
	TokenUsage     map[string]interface{} `json:"token_usage,omitempty"`
	ModelName      string                 `json:"model_name"`
	StepsUsed      int                    `json:"steps_used"`
	StepsTaken     int64                  `json:"steps_taken"` // For database storage
	ToolsUsed      int                    `json:"tools_used"`
	Error          string                 `json:"error,omitempty"`
	// Metadata from dotprompt for data ingestion classification
	App     string `json:"app,omitempty"`      // CloudShip data ingestion app classification
	AppType string `json:"app_type,omitempty"` // CloudShip data ingestion app_type classification
}

// ExecutionOptions contains optional parameters for agent execution
type ExecutionOptions struct {
	// SkipLighthouse if true, skips sending to Lighthouse (used by management channel which handles its own SendRun)
	SkipLighthouse bool
	// CloudShipRunID is the run ID provided by CloudShip for management channel executions
	// This is used for telemetry correlation - when set, spans will include cloudship.run_id attribute
	CloudShipRunID string
}

// AgentExecutionEngine handles the execution of agents using GenKit and MCP
type AgentExecutionEngine struct {
	repos                    *repositories.Repositories
	agentService             AgentServiceInterface
	genkitProvider           *GenKitProvider
	mcpConnManager           *MCPConnectionManager
	telemetryService         *TelemetryService            // For creating spans
	lighthouseClient         *lighthouse.LighthouseClient // For CloudShip integration
	deploymentContextService *DeploymentContextService    // For gathering deployment context
	activeMCPClients         []*mcp.GenkitMCPClient       // Store active connections for cleanup after execution
	memoryClient             *lighthouse.MemoryClient     // For CloudShip memory integration (serve/stdio mode)
	memoryAPIClient          *lighthouse.MemoryAPIClient  // For CloudShip memory integration (CLI mode - direct API)
	sandboxService           *SandboxService              // For sandbox execution (Dagger-based isolated compute)
	unifiedSandboxFactory    *UnifiedSandboxFactory       // For creating sandbox tools (supports both compute and code modes)
	sessionManager           *SessionManager              // For managing persistent code mode sandbox sessions
	codingToolFactory        *CodingToolFactory           // For creating AI coding backend tools (OpenCode)
}

// NewAgentExecutionEngine creates a new agent execution engine
func NewAgentExecutionEngine(repos *repositories.Repositories, agentService AgentServiceInterface) *AgentExecutionEngine {
	return NewAgentExecutionEngineWithLighthouse(repos, agentService, nil)
}

// NewAgentExecutionEngineWithLighthouse creates a new agent execution engine with optional Lighthouse integration
func NewAgentExecutionEngineWithLighthouse(repos *repositories.Repositories, agentService AgentServiceInterface, lighthouseClient *lighthouse.LighthouseClient) *AgentExecutionEngine {
	mcpConnManager := NewMCPConnectionManager(repos, nil)
	mcpConnManager.agentService = agentService

	if os.Getenv("STATION_MCP_POOLING") == "true" {
		mcpConnManager.EnableConnectionPooling()
		logging.Info("🏊 MCP connection pooling enabled via STATION_MCP_POOLING environment variable")
	}

	sandboxCfg := DefaultSandboxConfig()
	codeModeConfig := DefaultCodeModeConfig()

	if cfg := config.GetLoadedConfig(); cfg != nil {
		sandboxCfg.Enabled = cfg.Sandbox.Enabled
		codeModeConfig.Enabled = cfg.Sandbox.CodeModeEnabled
		if cfg.Sandbox.IdleTimeoutMinutes > 0 {
			codeModeConfig.IdleTimeout = time.Duration(cfg.Sandbox.IdleTimeoutMinutes) * time.Minute
		}
		if cfg.Sandbox.CleanupIntervalMinutes > 0 {
			codeModeConfig.CleanupInterval = time.Duration(cfg.Sandbox.CleanupIntervalMinutes) * time.Minute
		}
	}
	sandboxService := NewSandboxService(sandboxCfg)
	var sessionManager *SessionManager
	if codeModeConfig.Enabled {
		dockerBackend, err := NewDockerBackend(codeModeConfig)
		if err != nil {
			logging.Info("Failed to initialize Docker backend for code mode sandbox: %v (code mode will be disabled)", err)
			codeModeConfig.Enabled = false
		} else {
			sessionManager = NewSessionManager(dockerBackend)
			logging.Info("Sandbox code mode enabled with Docker backend")
		}
	}
	unifiedSandboxFactory := NewUnifiedSandboxFactory(sandboxService, sessionManager, codeModeConfig)

	// Initialize coding tool factory for AI coding backend integration
	var codingToolFactory *CodingToolFactory
	if cfg := config.GetLoadedConfig(); cfg != nil && cfg.Coding.Backend != "" {
		codingToolFactory = NewCodingToolFactory(cfg.Coding)
		if codingToolFactory.IsEnabled() {
			logging.Info("Coding tool factory initialized with %s backend", cfg.Coding.Backend)
		}
	}

	return &AgentExecutionEngine{
		repos:                    repos,
		agentService:             agentService,
		genkitProvider:           NewGenKitProvider(),
		mcpConnManager:           mcpConnManager,
		lighthouseClient:         lighthouseClient,
		deploymentContextService: NewDeploymentContextService(),
		telemetryService:         nil,
		sandboxService:           sandboxService,
		unifiedSandboxFactory:    unifiedSandboxFactory,
		sessionManager:           sessionManager,
		codingToolFactory:        codingToolFactory,
	}
}

// SetTelemetryService sets the OTEL telemetry service for distributed tracing
func (aee *AgentExecutionEngine) SetTelemetryService(ts *TelemetryService) {
	aee.telemetryService = ts
	logging.Debug("OTEL telemetry service configured for agent execution engine")
}

// SetMemoryClient sets the CloudShip memory client for memory integration
// This should be called when the ManagementChannel connects to CloudShip
func (aee *AgentExecutionEngine) SetMemoryClient(mc *lighthouse.MemoryClient) {
	aee.memoryClient = mc
	if mc != nil {
		logging.Info("CloudShip memory client configured for agent execution engine")
	}
}

// SetMemoryAPIClient sets the direct API memory client for CLI mode
// This should be called for CLI executions where there's no management channel
func (aee *AgentExecutionEngine) SetMemoryAPIClient(apiClient *lighthouse.MemoryAPIClient) {
	aee.memoryAPIClient = apiClient
	if apiClient != nil {
		logging.Info("CloudShip memory API client configured for agent execution engine (CLI mode)")
	}
}

// ExecuteAgent executes an agent using the unified execution architecture
func (aee *AgentExecutionEngine) ExecuteAgent(ctx context.Context, agent *models.Agent, task string, runID int64) (*AgentExecutionResult, error) {
	// Default to empty user variables for backward compatibility
	return aee.Execute(ctx, agent, task, runID, map[string]interface{}{})
}

// Execute executes an agent with optional user variables for dotprompt rendering
// skipLighthouse: if true, skips sending to Lighthouse (used by management channel which handles its own SendRun)
func (aee *AgentExecutionEngine) Execute(ctx context.Context, agent *models.Agent, task string, runID int64, userVariables map[string]interface{}) (*AgentExecutionResult, error) {
	return aee.ExecuteWithOptions(ctx, agent, task, runID, userVariables, ExecutionOptions{SkipLighthouse: false})
}

// ExecuteWithOptions executes an agent with options to control Lighthouse integration
func (aee *AgentExecutionEngine) ExecuteWithOptions(ctx context.Context, agent *models.Agent, task string, runID int64, userVariables map[string]interface{}, opts ExecutionOptions) (*AgentExecutionResult, error) {
	// Nil check to prevent panic
	if agent == nil {
		return nil, fmt.Errorf("agent cannot be nil")
	}

	// Add execution timeout at top level (15 minutes default)
	// This ensures ALL agent executions have a maximum time limit
	timeout := 15 * time.Minute

	// Check if context already has a deadline and use the shorter timeout
	if deadline, ok := ctx.Deadline(); ok {
		remainingTime := time.Until(deadline)
		if remainingTime > 0 && remainingTime < timeout {
			timeout = remainingTime
		}
	}

	// Create context with timeout for the entire execution
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startTime := time.Now()

	// Generate run UUID early so it can be used for both telemetry spans and SendRun
	// For management channel executions: use CloudShip's run ID (opts.CloudShipRunID)
	// For local executions: generate a new UUID
	var runUUID string
	if opts.CloudShipRunID != "" {
		runUUID = opts.CloudShipRunID
	} else {
		runUUID = uuid.New().String()
	}

	logging.Info("🎬 [EXECUTION TRACKER] ExecuteWithOptions STARTED - agent_id=%d agent_name='%s' station_run_id=%d run_uuid=%s skipLighthouse=%v timeout=%v", agent.ID, agent.Name, runID, runUUID, opts.SkipLighthouse, timeout)
	logging.Debug("Execute called for agent %s (ID: %d), skipLighthouse=%v, timeout=%v", agent.Name, agent.ID, opts.SkipLighthouse, timeout)
	logging.Info("Starting unified dotprompt execution for agent '%s'", agent.Name)

	// Use execCtx instead of ctx for all subsequent operations
	ctx = execCtx

	// Create telemetry span if telemetry service is available
	var span trace.Span
	if aee.telemetryService != nil {
		// Get parent run ID from context for hierarchical tracing
		parentRunID := GetParentRunIDFromContext(ctx)
		spanAttributes := []attribute.KeyValue{
			attribute.String("agent.name", agent.Name),
			attribute.Int64("agent.id", agent.ID),
			attribute.Int64("run.id", runID),
			attribute.String("run.uuid", runUUID), // UUID that will be sent to CloudShip
			attribute.Int("user_variables.count", len(userVariables)),
		}

		// Add parent run ID to span if available for hierarchical tracing
		if parentRunID != nil {
			spanAttributes = append(spanAttributes, attribute.Int64("run.parent_id", *parentRunID))
			spanAttributes = append(spanAttributes, attribute.Bool("run.is_child_execution", true))
		} else {
			spanAttributes = append(spanAttributes, attribute.Bool("run.is_child_execution", false))
		}

		ctx, span = aee.telemetryService.StartSpan(ctx, "agent_execution_engine.execute",
			trace.WithAttributes(spanAttributes...),
		)
		defer span.End()
	}

	// Log execution start
	err := aee.repos.AgentRuns.AppendDebugLog(ctx, runID, map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     "info",
		"message":   fmt.Sprintf("Starting execution for agent '%s'", agent.Name),
		"details": map[string]interface{}{
			"agent_id": agent.ID,
			"task":     task,
		},
	})
	if err != nil {
		logging.Debug("Failed to log execution start: %v", err)
	}

	// All agents now use unified dotprompt execution system

	// Note: MCP cleanup will happen after dotprompt execution completes
	// Do NOT defer cleanup here as it would disconnect connections while LLM is still using tools

	// Get agent tools for the new dotprompt system
	agentTools, err := aee.repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to get agent tools")
		}
		return nil, fmt.Errorf("failed to get agent tools for dotprompt execution: %w", err)
	}

	// Get child agents from agent_agents table (for agents: frontmatter section)
	childAgents, err := aee.repos.AgentAgents.ListChildAgents(agent.ID)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to get child agents")
		}
		return nil, fmt.Errorf("failed to get child agents for dotprompt execution: %w", err)
	}
	logging.Info("🔍 [CHILD AGENTS] Loaded %d child agents for agent %s (ID: %d)", len(childAgents), agent.Name, agent.ID)
	for _, ca := range childAgents {
		logging.Info("   - Child: %s (ID: %d)", ca.ChildAgent.Name, ca.ChildAgentID)
	}

	if span != nil {
		span.SetAttributes(
			attribute.Int("agent.tools_count", len(agentTools)),
			attribute.Int("agent.child_agents_count", len(childAgents)),
		)
	}

	// Get GenKit app for dotprompt execution
	genkitApp, err := aee.genkitProvider.GetApp(ctx)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to get genkit app")
		}
		return nil, fmt.Errorf("failed to get genkit app for dotprompt execution: %w", err)
	}

	// Update MCP connection manager with GenKit app (same as traditional)
	aee.mcpConnManager.genkitApp = genkitApp

	// Initialize server pool for THIS environment only (performance fix)
	// CRITICAL: Only load servers for agent's environment, not ALL environments
	if err := aee.mcpConnManager.InitializeServerPool(ctx, agent.EnvironmentID); err != nil {
		logging.Info("Warning: Failed to initialize MCP server pool for environment %d: %v", agent.EnvironmentID, err)
	}

	// Load MCP tools for dotprompt execution (reuse the same logic as traditional execution)
	var mcpLoadSpan trace.Span
	if span != nil {
		ctx, mcpLoadSpan = aee.telemetryService.StartSpan(ctx, "mcp.load_tools",
			trace.WithAttributes(
				attribute.Int64("environment.id", agent.EnvironmentID),
			),
		)
		defer mcpLoadSpan.End()
	}

	logging.Debug("About to call GetEnvironmentMCPTools for env %d", agent.EnvironmentID)
	allMCPTools, mcpClients, err := aee.mcpConnManager.GetEnvironmentMCPTools(ctx, agent.EnvironmentID)
	fmt.Printf("✅ [ENGINE] GetEnvironmentMCPTools returned %d tools, %d clients, err=%v\n", len(allMCPTools), len(mcpClients), err != nil)
	logging.Debug("GetEnvironmentMCPTools returned %d tools, %d clients, err=%v", len(allMCPTools), len(mcpClients), err != nil)
	if err != nil {
		if mcpLoadSpan != nil {
			mcpLoadSpan.RecordError(err)
			mcpLoadSpan.SetStatus(codes.Error, "failed to load MCP tools")
		}
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to get environment MCP tools")
		}
		return nil, fmt.Errorf("failed to get environment MCP tools for dotprompt execution: %w", err)
	}

	if mcpLoadSpan != nil {
		mcpLoadSpan.SetAttributes(
			attribute.Int("mcp.tools_loaded", len(allMCPTools)),
			attribute.Int("mcp.clients_connected", len(mcpClients)),
		)
	}

	// Store clients for cleanup after execution
	aee.activeMCPClients = mcpClients

	// Filter to only include tools assigned to this agent
	// Special handling: Include ALL agent tools since they're not stored in DB but created at runtime
	fmt.Printf("DEBUG FILTER: Agent %s has %d assigned tools from DB, %d available MCP tools\n", agent.Name, len(agentTools), len(allMCPTools))
	logging.Debug("Filtering %d assigned tools from %d available MCP tools", len(agentTools), len(allMCPTools))
	var mcpTools []ai.ToolRef

	// Create a map of assigned tool names for quick lookup
	assignedToolNames := make(map[string]bool)
	for _, assignedTool := range agentTools {
		assignedToolNames[assignedTool.ToolName] = true
		fmt.Printf("DEBUG FILTER: Assigned tool from DB: %s\n", assignedTool.ToolName)
	}

	// Add child agents as tool names (convert agent name to __agent_* format)
	// This matches the naming convention in mcp_connection_manager.go:getAgentToolsForEnvironment
	for _, childAgent := range childAgents {
		// Normalize agent name to tool name format
		normalizedName := strings.ToLower(childAgent.ChildAgent.Name)
		// Replace all special characters with underscores
		replacements := []string{" ", "-", ".", "!", "@", "#", "$", "%", "^", "&", "*", "(", ")", "+", "=", "[", "]", "{", "}", "|", "\\", ":", ";", "\"", "'", "<", ">", ",", "?", "/"}
		for _, char := range replacements {
			normalizedName = strings.ReplaceAll(normalizedName, char, "_")
		}
		// Remove multiple consecutive underscores
		for strings.Contains(normalizedName, "__") {
			normalizedName = strings.ReplaceAll(normalizedName, "__", "_")
		}
		// Trim leading/trailing underscores
		normalizedName = strings.Trim(normalizedName, "_")

		childToolName := fmt.Sprintf("__agent_%s", normalizedName)
		assignedToolNames[childToolName] = true
		fmt.Printf("DEBUG FILTER: Assigned child agent tool: %s (from agent: %s)\n", childToolName, childAgent.ChildAgent.Name)
		logging.Debug("Added child agent as tool: %s (agent: %s, id: %d)", childToolName, childAgent.ChildAgent.Name, childAgent.ChildAgentID)
	}

	logging.Debug("Starting tool filtering with %d assigned tools", len(agentTools))
	for j, mcpTool := range allMCPTools {
		// Get tool name
		var toolName string
		if named, ok := mcpTool.(interface{ Name() string }); ok {
			toolName = named.Name()
		} else if stringer, ok := mcpTool.(interface{ String() string }); ok {
			toolName = stringer.String()
		} else {
			toolName = fmt.Sprintf("%T", mcpTool)
		}

		if j < 35 {
			isAgentTool := strings.HasPrefix(toolName, "__agent_")
			fmt.Printf("DEBUG FILTER: Checking tool[%d]: %s (is_agent_tool=%v)\n", j, toolName, isAgentTool)
		}

		// Include tool ONLY if it's in the assigned tools list
		// Child agent tools are already added to assignedToolNames map (lines 280-301)
		// so we don't need a separate check for agent tools
		isAssigned := assignedToolNames[toolName]

		if isAssigned {
			isAgentTool := strings.HasPrefix(toolName, "__agent_")
			fmt.Printf("DEBUG FILTER: MATCHED tool %s (assigned=%v, agent_tool=%v)\n", toolName, isAssigned, isAgentTool)
			logging.Debug("MATCHED! Adding tool %s (assigned=%v, agent_tool=%v)", toolName, isAssigned, isAgentTool)
			mcpTools = append(mcpTools, mcpTool)
		}
	}
	fmt.Printf("DEBUG FILTER: Filtering complete - found %d matching tools\n", len(mcpTools))
	logging.Debug("Tool filtering completed - found %d matching tools (from %d assigned + agent tools)", len(mcpTools), len(agentTools))

	logging.Debug("Dotprompt execution using %d tools (filtered from %d available)", len(mcpTools), len(allMCPTools))

	if span != nil {
		span.SetAttributes(attribute.Int("agent.filtered_tools_count", len(mcpTools)))
	}

	logging.Debug("Creating dotprompt executor")
	executor := dotprompt.NewGenKitExecutor()

	// Create a logging callback for real-time progress updates
	logCallback := func(logEntry map[string]interface{}) {
		// Store all logs in database for UI display (filtering handled by UI layer if needed)
		err := aee.repos.AgentRuns.AppendDebugLog(ctx, runID, logEntry)
		if err != nil {
			logging.Debug("Failed to append debug log: %v", err)
		}
	}

	// Set the logging callback on the OpenAI plugin for detailed API call logging
	aee.genkitProvider.SetOpenAILogCallback(logCallback)

	logging.Debug("About to call dotprompt executor - agent: %s", agent.Name)

	// Create execution span
	var execSpan trace.Span
	if span != nil {
		ctx, execSpan = aee.telemetryService.StartSpan(ctx, "dotprompt.execute",
			trace.WithAttributes(
				attribute.String("task.preview", func() string {
					if len(task) > 200 {
						return task[:200] + "..."
					}
					return task
				}()),
			),
		)
		defer execSpan.End()
	}

	environment, err := aee.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment (ID: %d) for agent %s: %w", agent.EnvironmentID, agent.Name, err)
	}

	// Extract workflow run ID from user variables if agent is running in workflow context
	var workflowRunID string
	if runIDRaw, ok := userVariables["_runID"]; ok {
		if runIDStr, ok := runIDRaw.(string); ok {
			workflowRunID = runIDStr
			logging.Info("Agent %s running in workflow context (workflowRunID=%s)", agent.Name, workflowRunID)
		}
	}

	sandboxConfig := aee.parseSandboxConfigFromAgent(agent, environment.Name)
	toolExecCtx := ExecutionContext{
		WorkflowRunID:      workflowRunID,
		AgentRunID:         fmt.Sprintf("%d", runID),
		SandboxSessionName: "",
	}
	if aee.unifiedSandboxFactory.ShouldAddTools(sandboxConfig) {
		sandboxTools := aee.unifiedSandboxFactory.GetSandboxTools(sandboxConfig, toolExecCtx)
		for _, tool := range sandboxTools {
			mcpTools = append(mcpTools, tool)
		}
		if aee.unifiedSandboxFactory.IsCodeMode(sandboxConfig) {
			logging.Info("Sandbox code mode enabled for agent %s (%d tools)", agent.Name, len(sandboxTools))
		} else {
			logging.Info("Sandbox compute mode enabled for agent %s (runtime: %s)", agent.Name, sandboxConfig.Runtime)
		}
	}

	codingConfig := aee.parseCodingConfigFromAgent(agent, environment.Name)
	logging.Info("[CODING DEBUG] Agent %s: codingConfig=%+v, factory=%v", agent.Name, codingConfig, aee.codingToolFactory != nil)
	if aee.codingToolFactory != nil && aee.codingToolFactory.ShouldAddTools(codingConfig) {
		codingTools := aee.codingToolFactory.GetCodingTools(codingConfig, toolExecCtx)
		for _, tool := range codingTools {
			mcpTools = append(mcpTools, tool)
		}
		logging.Info("Coding tools enabled for agent %s (%d tools: coding_open, code, coding_close)", agent.Name, len(codingTools))
	} else {
		logging.Info("[CODING DEBUG] Coding tools NOT enabled for agent %s: factoryNil=%v, shouldAdd=%v",
			agent.Name, aee.codingToolFactory == nil,
			aee.codingToolFactory != nil && aee.codingToolFactory.ShouldAddTools(codingConfig))
	}

	ctx = WithParentRunID(ctx, runID)
	ctx = WithParentRunID(ctx, runID)

	// Add current run ID to context for trace correlation
	ctx = WithCurrentRunID(ctx, runID)

	// Inject CloudShip memory context if agent has memory configured
	logging.Info("[MEMORY DEBUG] Agent '%s' (ID: %d) MemoryTopicKey=%v", agent.Name, agent.ID, agent.MemoryTopicKey)
	if agent.MemoryTopicKey != nil && *agent.MemoryTopicKey != "" {
		logging.Info("[MEMORY DEBUG] Processing memory for topic '%s'", *agent.MemoryTopicKey)
		maxTokens := 2000 // Default max tokens
		if agent.MemoryMaxTokens != nil && *agent.MemoryMaxTokens > 0 {
			maxTokens = *agent.MemoryMaxTokens
		}

		// Create memory service with CloudShip integration if memory client is available
		var memoryService *MemoryService
		if aee.memoryClient != nil {
			memoryService = NewMemoryServiceWithClient(aee.memoryClient)
		} else {
			memoryService = NewMemoryService()
		}
		// Also set API client for CLI mode direct API calls
		if aee.memoryAPIClient != nil {
			memoryService.SetMemoryAPIClient(aee.memoryAPIClient)
		}
		memoryCtx := memoryService.GetMemoryContext(ctx, *agent.MemoryTopicKey, maxTokens)

		if memoryCtx.Error != nil {
			logging.Info("Warning: Failed to get memory context for topic '%s': %v", *agent.MemoryTopicKey, memoryCtx.Error)
		} else if memoryCtx.Content != "" {
			// Initialize userVariables if nil
			if userVariables == nil {
				userVariables = make(map[string]interface{})
			}
			userVariables["cloudship_memory"] = memoryCtx.Content
			logging.Info("Injected memory context for topic '%s' (%d tokens, source: %s)",
				memoryCtx.TopicKey, memoryCtx.TokenCount, memoryCtx.Source)
		} else {
			// Memory topic exists but no content yet - inject empty string
			if userVariables == nil {
				userVariables = make(map[string]interface{})
			}
			userVariables["cloudship_memory"] = ""
			logging.Debug("Memory topic '%s' exists but has no content yet", *agent.MemoryTopicKey)
		}
	}

	// Use clean, unified dotprompt.Execute() execution path with context for timeout protection
	response, err := executor.ExecuteAgent(ctx, *agent, agentTools, genkitApp, mcpTools, task, logCallback, environment.Name, userVariables)

	// Clean up MCP connections after execution is complete
	aee.mcpConnManager.CleanupConnections(aee.activeMCPClients)
	aee.activeMCPClients = nil

	// Clean up sandbox sessions (for code mode) after execution
	if aee.sessionManager != nil && aee.unifiedSandboxFactory.IsCodeMode(sandboxConfig) {
		sessionKey := ResolveSessionKey("", fmt.Sprintf("%d", runID), "")
		if session, getErr := aee.sessionManager.GetSession(ctx, sessionKey); getErr == nil {
			if closeErr := aee.sessionManager.CloseSession(ctx, session.ID); closeErr != nil {
				logging.Debug("Failed to cleanup sandbox session for agent %s: %v", agent.Name, closeErr)
			}
		}
	}

	if err != nil {
		// Check if this was a timeout
		if ctx.Err() == context.DeadlineExceeded {
			timeoutErr := fmt.Errorf("agent execution timed out after %v: agent %s (ID: %d) exceeded maximum execution time",
				timeout, agent.Name, agent.ID)
			logging.Error("⏱️ TIMEOUT: %v", timeoutErr)

			if execSpan != nil {
				execSpan.RecordError(timeoutErr)
				execSpan.SetStatus(codes.Error, "execution timeout")
				execSpan.SetAttributes(
					attribute.String("error.type", "timeout"),
					attribute.Float64("timeout.duration_seconds", timeout.Seconds()),
				)
			}
			if span != nil {
				span.RecordError(timeoutErr)
				span.SetStatus(codes.Error, "execution timeout")
			}

			// Log timeout for debugging
			if logCallback != nil {
				logCallback(map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339),
					"level":     "error",
					"message":   "Agent execution timed out",
					"details": map[string]interface{}{
						"error":           timeoutErr.Error(),
						"timeout":         timeout.String(),
						"duration":        time.Since(startTime).String(),
						"steps_completed": 0, // Will be updated if we have step data
					},
				})
			}

			return nil, timeoutErr
		}

		// Regular execution error (not timeout)
		// Record error in spans
		if execSpan != nil {
			execSpan.RecordError(err)
			execSpan.SetStatus(codes.Error, "dotprompt execution failed")
		}
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "agent execution failed")
		}

		// Log the execution failure for debugging
		if logCallback != nil {
			logCallback(map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
				"level":     "error",
				"message":   "Agent execution failed",
				"details": map[string]interface{}{
					"error":    err.Error(),
					"duration": time.Since(startTime).String(),
				},
			})
		}
		return nil, fmt.Errorf("dotprompt execution failed: %w", err)
	}

	// Add success metrics to spans
	duration := time.Since(startTime)
	if execSpan != nil {
		execSpan.SetAttributes(
			attribute.Bool("execution.success", response.Success),
			attribute.String("execution.model", response.ModelName),
			attribute.Float64("execution.duration_seconds", duration.Seconds()),
			attribute.Int("execution.steps_used", response.StepsUsed),
			attribute.Int("execution.tools_used", response.ToolsUsed),
		)
	}
	if span != nil {
		span.SetAttributes(
			attribute.Bool("execution.success", response.Success),
			attribute.String("execution.model", response.ModelName),
			attribute.Float64("execution.duration_seconds", duration.Seconds()),
			attribute.Int("execution.steps_used", response.StepsUsed),
			attribute.Int("execution.tools_used", response.ToolsUsed),
		)
	}

	// Convert ExecutionResponse to AgentExecutionResult
	logging.Debug("🔍 ENGINE: Converting dotprompt response to AgentExecutionResult for agent %d, run %d", agent.ID, runID)
	logging.Debug("🔍 ENGINE: Response.App='%s', Response.AppType='%s'", response.App, response.AppType)

	result := &AgentExecutionResult{
		Success:        response.Success,
		Response:       response.Response,
		Duration:       duration,
		ModelName:      response.ModelName,
		StepsUsed:      response.StepsUsed,
		StepsTaken:     int64(response.StepsUsed), // Map StepsUsed to StepsTaken for database
		ToolsUsed:      response.ToolsUsed,
		Error:          response.Error,
		TokenUsage:     response.TokenUsage,     // ✅ Pass through token usage from dotprompt
		ToolCalls:      response.ToolCalls,      // ✅ Pass through tool calls
		ExecutionSteps: response.ExecutionSteps, // ✅ Pass through execution steps
		App:            response.App,            // ✅ Pass through app classification for CloudShip data ingestion
		AppType:        response.AppType,        // ✅ Pass through app_type classification for CloudShip data ingestion
	}

	logging.Debug("🔍 ENGINE: AgentExecutionResult created - result.App='%s', result.AppType='%s'", result.App, result.AppType)

	// 🚀 Lighthouse Integration: Send run data to CloudShip (async, non-blocking)
	// Send to CloudShip Lighthouse (dual flow: SendRun always + IngestData conditionally)
	// Skip if management channel is handling the SendRun (they use their own run_id)
	if !opts.SkipLighthouse {
		logging.Debug("🔍 DEBUG: About to call sendToLighthouse for agent %d, run %d, runUUID %s", agent.ID, runID, runUUID)
		aee.sendToLighthouse(agent, task, runID, runUUID, startTime, result)
		logging.Debug("🔍 DEBUG: Returned from sendToLighthouse for agent %d, run %d", agent.ID, runID)
	} else {
		logging.Debug("🔍 DEBUG: Skipping sendToLighthouse for agent %d, run %d (management channel will handle SendRun)", agent.ID, runID)
	}

	return result, nil
}

// GetGenkitProvider returns the genkit provider for external access
func (aee *AgentExecutionEngine) GetGenkitProvider() *GenKitProvider {
	return aee.genkitProvider
}

// sendToLighthouse sends agent run data to CloudShip Lighthouse (async, non-blocking)
// runUUID is the UUID that will be used as the run ID in CloudShip (matches span.run.uuid attribute)
func (aee *AgentExecutionEngine) sendToLighthouse(agent *models.Agent, task string, runID int64, runUUID string, startTime time.Time, result *AgentExecutionResult) {
	logging.Info("🚀 [SENDRUN TRACKER] sendToLighthouse CALLED - agent_id=%d agent_name='%s' station_run_id=%d run_uuid=%s", agent.ID, agent.Name, runID, runUUID)
	logging.Debug("🔍 DEBUG: sendToLighthouse called for agent %d, run %d", agent.ID, runID)
	logging.Debug("🔍 DEBUG: lighthouseClient nil? %v", aee.lighthouseClient == nil)
	if aee.lighthouseClient != nil {
		logging.Debug("🔍 DEBUG: lighthouseClient.IsRegistered()? %v", aee.lighthouseClient.IsRegistered())
	}

	// Skip if no Lighthouse client configured
	if aee.lighthouseClient == nil || !aee.lighthouseClient.IsRegistered() {
		logging.Debug("🔍 DEBUG: Skipping Lighthouse - client nil or not registered")
		return // Graceful degradation - no cloud integration
	}

	logging.Debug("🔍 DEBUG: Proceeding with Lighthouse integration")

	// Convert AgentExecutionResult to types.AgentRun for Lighthouse
	agentRun := aee.convertToAgentRun(agent, task, runID, runUUID, startTime, result)
	logging.Info("🆔 [SENDRUN TRACKER] Using run_uuid=%s for station_run_id=%d agent='%s' memory_topic_key='%s'", agentRun.ID, runID, agent.Name, agentRun.MemoryTopicKey)

	// Determine deployment mode and send appropriate data
	mode := aee.lighthouseClient.GetMode()
	logging.Info("📡 [SENDRUN TRACKER] Detected mode=%s for station_run_id=%d run_uuid=%s", mode, runID, agentRun.ID)
	logging.Debug("Lighthouse client mode detected: %v (comparing with ModeCLI: %v)", mode, lighthouse.ModeCLI)
	switch mode {
	case lighthouse.ModeStdio:
		// stdio mode: Local development context
		context := aee.deploymentContextService.GatherContextForMode("stdio")
		logging.Info("📤 [SENDRUN TRACKER] Calling SendRun for STDIO mode - run_uuid=%s station_run_id=%d agent='%s'", agentRun.ID, runID, agent.Name)
		aee.lighthouseClient.SendRun(agentRun, "default", context.ToLabelsMap())
		logging.Info("✅ [SENDRUN TRACKER] SendRun completed for STDIO mode - run_uuid=%s", agentRun.ID)
		// Dual flow: Send structured data if conditions are met (pass run UUID for lineage)
		logging.Debug("🚀 [LIGHTHOUSE DEBUG] About to call sendStructuredDataIfEligible for stdio mode (run_id: %d, run_uuid: %s)", runID, agentRun.ID)
		aee.sendStructuredDataIfEligible(agent, result, runID, agentRun.ID, context.ToLabelsMap())

	case lighthouse.ModeServe:
		// serve mode: Server deployment context
		context := aee.deploymentContextService.GatherContextForMode("serve")
		logging.Info("📤 [SENDRUN TRACKER] Calling SendRun for SERVE mode - run_uuid=%s station_run_id=%d agent='%s'", agentRun.ID, runID, agent.Name)
		aee.lighthouseClient.SendRun(agentRun, "default", context.ToLabelsMap())
		logging.Info("✅ [SENDRUN TRACKER] SendRun completed for SERVE mode - run_uuid=%s", agentRun.ID)
		// Dual flow: Send structured data if conditions are met (pass run UUID for lineage)
		aee.sendStructuredDataIfEligible(agent, result, runID, agentRun.ID, context.ToLabelsMap())

	case lighthouse.ModeCLI:
		// CLI mode: Rich execution context (may include CI/CD)
		context := aee.deploymentContextService.GatherContextForMode("cli")
		logging.Info("📤 [SENDRUN TRACKER] Calling SendRun for CLI mode - run_uuid=%s station_run_id=%d agent='%s'", agentRun.ID, runID, agent.Name)
		aee.lighthouseClient.SendRun(agentRun, "default", context.ToLabelsMap())
		logging.Info("✅ [SENDRUN TRACKER] SendRun completed for CLI mode - run_uuid=%s", agentRun.ID)
		// Dual flow: Send structured data if conditions are met (pass run UUID for lineage)
		aee.sendStructuredDataIfEligible(agent, result, runID, agentRun.ID, context.ToLabelsMap())
		logging.Info("Successfully sent CLI run data with deployment context for run_id: %d", runID)

	default:
		// Unknown mode - send basic run data
		logging.Info("📤 [SENDRUN TRACKER] Calling SendRun for UNKNOWN mode - run_uuid=%s station_run_id=%d agent='%s'", agentRun.ID, runID, agent.Name)
		aee.lighthouseClient.SendRun(agentRun, "unknown", map[string]string{
			"mode": "unknown",
		})
		logging.Info("✅ [SENDRUN TRACKER] SendRun completed for UNKNOWN mode - run_uuid=%s", agentRun.ID)
		// Dual flow: Send structured data if conditions are met (pass run UUID for lineage)
		aee.sendStructuredDataIfEligible(agent, result, runID, agentRun.ID, map[string]string{"mode": "unknown"})
	}

	logging.Info("🏁 [SENDRUN TRACKER] Completed sendToLighthouse for station_run_id=%d run_uuid=%s mode=%s", runID, agentRun.ID, mode)
	logging.Debug("Completed CloudShip Lighthouse integration (run_id: %d, mode: %s)", runID, mode)
}

// sendStructuredDataIfEligible checks if agent qualifies for structured data ingestion and sends it
func (aee *AgentExecutionEngine) sendStructuredDataIfEligible(agent *models.Agent, result *AgentExecutionResult, runID int64, runUUID string, contextLabels map[string]string) {
	logging.Debug("🔍 [LIGHTHOUSE DEBUG] sendStructuredDataIfEligible called for agent %d (name: %s, run_id: %d, run_uuid: %s)", agent.ID, agent.Name, runID, runUUID)

	// Log lighthouse client state
	if aee.lighthouseClient == nil {
		logging.Debug("❌ [LIGHTHOUSE DEBUG] Lighthouse client is nil - cannot send structured data")
		return
	} else {
		logging.Debug("✅ [LIGHTHOUSE DEBUG] Lighthouse client exists and is connected")
	}

	// Check if agent has app/app_type metadata (from dotprompt or database)
	app := result.App
	appType := result.AppType

	logging.Debug("📊 [LIGHTHOUSE DEBUG] Agent %d initial metadata - result.App: '%s', result.AppType: '%s', agent.OutputSchemaPreset: %v",
		agent.ID, app, appType, func() string {
			if agent.OutputSchemaPreset != nil {
				return *agent.OutputSchemaPreset
			}
			return "nil"
		}())
	hasUserDefinedSchema := false
	hasPreset := false

	// Check for user-defined output schema + app/app_type
	if (app != "" && appType != "") && (agent.OutputSchema != nil && *agent.OutputSchema != "") {
		hasUserDefinedSchema = true
		logging.Debug("✅ User-defined schema agent %d for data ingestion (app: %s, app_type: %s)", agent.ID, app, appType)
	}

	// Fallback: Check if agent has preset-based app/app_type
	if app == "" && appType == "" && agent.OutputSchemaPreset != nil && *agent.OutputSchemaPreset != "" {
		if presetInfo, exists := schema.GetPresetInfo(*agent.OutputSchemaPreset); exists {
			app = presetInfo.App
			appType = presetInfo.AppType
			hasPreset = true
			logging.Debug("🔄 Identified preset '%s' for agent %d (app: %s, app_type: %s)", *agent.OutputSchemaPreset, agent.ID, app, appType)
		} else {
			logging.Debug("⚠️ Unknown preset '%s' for agent %d", *agent.OutputSchemaPreset, agent.ID)
		}
	}

	// Skip if no app/app_type identified
	if app == "" || appType == "" {
		logging.Debug("❌ No app/app_type metadata found for agent %d, skipping structured data ingestion", agent.ID)
		return
	}

	// Validation: Require either user-defined schema OR preset
	if !hasUserDefinedSchema && !hasPreset {
		logging.Debug("❌ Agent %d has app/app_type but no valid schema source (needs output_schema OR preset), skipping data ingestion", agent.ID)
		return
	}

	// Log what type of agent we're processing
	if hasUserDefinedSchema {
		logging.Debug("📋 Processing user-defined schema agent %d with explicit output schema", agent.ID)
	} else if hasPreset {
		logging.Debug("🎯 Processing preset-based agent %d (preset: %s)", agent.ID, *agent.OutputSchemaPreset)
	}

	// Skip if agent execution failed (no meaningful structured data)
	if !result.Success {
		logging.Debug("Agent execution failed for agent %d, skipping structured data ingestion", agent.ID)
		return
	}

	// Attempt to parse the response as structured JSON
	var structuredData map[string]interface{}
	if err := json.Unmarshal([]byte(result.Response), &structuredData); err != nil {
		logging.Debug("Agent response is not valid JSON for agent %d, skipping structured data ingestion: %v", agent.ID, err)
		return
	}

	// Prepare metadata for ingestion
	metadata := make(map[string]string)
	for k, v := range contextLabels {
		metadata[k] = v
	}
	metadata["agent_id"] = fmt.Sprintf("%d", agent.ID)
	metadata["agent_name"] = agent.Name
	metadata["run_id"] = fmt.Sprintf("%d", runID)
	metadata["output_schema_preset"] = func() string {
		if agent.OutputSchemaPreset != nil {
			return *agent.OutputSchemaPreset
		}
		return ""
	}()
	metadata["execution_success"] = fmt.Sprintf("%t", result.Success)
	metadata["duration_ms"] = fmt.Sprintf("%d", result.Duration.Milliseconds())

	// Send structured data to CloudShip Data Ingestion service
	// Use UUID for correlation to prevent collisions across multiple stations
	correlationID := uuid.New().String()
	agentIDStr := fmt.Sprintf("%d", agent.ID)

	logging.Debug("🚀 Attempting IngestData call to CloudShip (app: %s, app_type: %s, run_id: %d, run_uuid: %s, agent: %s, correlation_id: %s)",
		app, appType, runID, runUUID, agent.Name, correlationID)

	// Pass run_uuid, agent_name, and agent_id for lineage tracing
	if err := aee.lighthouseClient.IngestData(app, appType, structuredData, metadata, correlationID, runUUID, agent.Name, agentIDStr); err != nil {
		logging.Debug("❌ Failed to send structured data to CloudShip: %v", err)
		// Don't fail the execution - this is supplementary data
	} else {
		logging.Debug("✅ Successfully sent structured data to CloudShip (app: %s, app_type: %s, run_id: %d, run_uuid: %s, agent: %s)",
			app, appType, runID, runUUID, agent.Name)
	}
}

// convertToAgentRun converts Station models to Lighthouse types
// runUUID is the pre-generated UUID that matches the span.run.uuid telemetry attribute
func (aee *AgentExecutionEngine) convertToAgentRun(agent *models.Agent, task string, runID int64, runUUID string, startTime time.Time, result *AgentExecutionResult) *types.AgentRun {
	status := "completed"
	if !result.Success {
		status = "failed"
	}

	// Use the provided runUUID (generated early in ExecuteWithOptions for telemetry correlation)
	return &types.AgentRun{
		ID:             runUUID,
		AgentID:        fmt.Sprintf("agent_%d", agent.ID),
		AgentName:      agent.Name,
		Task:           task,
		Response:       result.Response,
		Status:         status,
		DurationMs:     result.Duration.Milliseconds(),
		ModelName:      result.ModelName,
		StartedAt:      startTime,
		CompletedAt:    startTime.Add(result.Duration),
		ToolCalls:      aee.convertToolCalls(result.ToolCalls),
		ExecutionSteps: aee.convertExecutionSteps(result.ExecutionSteps),
		TokenUsage:     aee.convertTokenUsage(result.TokenUsage),
		OutputSchema: func() string {
			if agent.OutputSchema != nil {
				return *agent.OutputSchema
			}
			return ""
		}(),
		OutputSchemaPreset: func() string {
			if agent.OutputSchemaPreset != nil {
				return *agent.OutputSchemaPreset
			}
			return ""
		}(),
		MemoryTopicKey: func() string {
			if agent.MemoryTopicKey != nil {
				return *agent.MemoryTopicKey
			}
			return ""
		}(),
		Metadata: map[string]string{
			"steps_used":     fmt.Sprintf("%d", result.StepsUsed),
			"tools_used":     fmt.Sprintf("%d", result.ToolsUsed),
			"run_id":         fmt.Sprintf("%d", runID),
			"run_uuid":       runUUID,
			"agent_id":       fmt.Sprintf("%d", agent.ID),
			"station_run_id": fmt.Sprintf("%d", runID), // Keep local DB ID for correlation
		},
	}
}

// convertToolCalls converts Station tool calls to Lighthouse format
func (aee *AgentExecutionEngine) convertToolCalls(toolCalls *models.JSONArray) []types.ToolCall {
	if toolCalls == nil {
		return nil
	}

	// Convert JSONArray slice to ToolCall types
	var lighthouseCalls []types.ToolCall
	for _, item := range *toolCalls {
		if toolCallMap, ok := item.(map[string]interface{}); ok {
			toolCall := types.ToolCall{
				Timestamp: time.Now(), // Default timestamp
			}

			if name, exists := toolCallMap["tool_name"]; exists {
				if nameStr, ok := name.(string); ok {
					toolCall.ToolName = nameStr
				}
			}

			if params, exists := toolCallMap["parameters"]; exists {
				toolCall.Parameters = params
			}

			if result, exists := toolCallMap["result"]; exists {
				if resultStr, ok := result.(string); ok {
					toolCall.Result = resultStr
				} else {
					// Convert non-string results to JSON
					if jsonBytes, err := json.Marshal(result); err == nil {
						toolCall.Result = string(jsonBytes)
					}
				}
			}

			if duration, exists := toolCallMap["duration_ms"]; exists {
				if durationFloat, ok := duration.(float64); ok {
					toolCall.DurationMs = int64(durationFloat)
				}
			}

			if success, exists := toolCallMap["success"]; exists {
				if successBool, ok := success.(bool); ok {
					toolCall.Success = successBool
				}
			}

			lighthouseCalls = append(lighthouseCalls, toolCall)
		}
	}

	return lighthouseCalls
}

// convertExecutionSteps converts Station execution steps to Lighthouse format
func (aee *AgentExecutionEngine) convertExecutionSteps(steps *models.JSONArray) []types.ExecutionStep {
	if steps == nil {
		return nil
	}

	// Convert JSONArray slice to ExecutionStep types
	var lighthouseSteps []types.ExecutionStep
	for _, item := range *steps {
		if stepMap, ok := item.(map[string]interface{}); ok {
			step := types.ExecutionStep{
				Timestamp: time.Now(), // Default timestamp
			}

			if stepNum, exists := stepMap["step_number"]; exists {
				if stepNumFloat, ok := stepNum.(float64); ok {
					step.StepNumber = int(stepNumFloat)
				}
			}

			if desc, exists := stepMap["description"]; exists {
				if descStr, ok := desc.(string); ok {
					step.Description = descStr
				}
			}

			if stepType, exists := stepMap["type"]; exists {
				if typeStr, ok := stepType.(string); ok {
					step.Type = typeStr
				}
			}

			if duration, exists := stepMap["duration_ms"]; exists {
				if durationFloat, ok := duration.(float64); ok {
					step.DurationMs = int64(durationFloat)
				}
			}

			lighthouseSteps = append(lighthouseSteps, step)
		}
	}

	return lighthouseSteps
}

// convertTokenUsage converts Station token usage to Lighthouse format
func (aee *AgentExecutionEngine) convertTokenUsage(usage map[string]interface{}) *types.TokenUsage {
	if usage == nil {
		return nil
	}

	tokenUsage := &types.TokenUsage{}

	if val, ok := usage["prompt_tokens"]; ok {
		if intVal, ok := val.(int); ok {
			tokenUsage.PromptTokens = intVal
		}
	}

	if val, ok := usage["completion_tokens"]; ok {
		if intVal, ok := val.(int); ok {
			tokenUsage.CompletionTokens = intVal
		}
	}

	if val, ok := usage["total_tokens"]; ok {
		if intVal, ok := val.(int); ok {
			tokenUsage.TotalTokens = intVal
		}
	}

	if val, ok := usage["cost_usd"]; ok {
		if floatVal, ok := val.(float64); ok {
			tokenUsage.CostUSD = floatVal
		}
	}

	return tokenUsage
}

type sandboxFrontmatter struct {
	Sandbox *dotprompt.SandboxConfig `yaml:"sandbox"`
}

type codingFrontmatter struct {
	Coding *dotprompt.CodingConfig `yaml:"coding"`
}

func (aee *AgentExecutionEngine) parseSandboxConfigFromAgent(agent *models.Agent, environmentName string) *dotprompt.SandboxConfig {
	promptPath := config.GetAgentPromptPath(environmentName, agent.Name)

	content, err := os.ReadFile(promptPath)
	if err != nil {
		logging.Debug("Failed to read dotprompt file for sandbox config: %v", err)
		return nil
	}

	parts := strings.Split(string(content), "---")
	if len(parts) < 3 {
		return nil
	}

	yamlContent := strings.TrimSpace(parts[1])
	if yamlContent == "" {
		return nil
	}

	var fm sandboxFrontmatter
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		logging.Debug("Failed to parse sandbox config from dotprompt: %v", err)
		return nil
	}

	return fm.Sandbox
}

func (aee *AgentExecutionEngine) parseCodingConfigFromAgent(agent *models.Agent, environmentName string) *dotprompt.CodingConfig {
	promptPath := config.GetAgentPromptPath(environmentName, agent.Name)

	content, err := os.ReadFile(promptPath)
	if err != nil {
		logging.Debug("Failed to read dotprompt file for coding config: %v", err)
		return nil
	}

	parts := strings.Split(string(content), "---")
	if len(parts) < 3 {
		return nil
	}

	yamlContent := strings.TrimSpace(parts[1])
	if yamlContent == "" {
		return nil
	}

	var fm codingFrontmatter
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		logging.Debug("Failed to parse coding config from dotprompt: %v", err)
		return nil
	}

	return fm.Coding
}
