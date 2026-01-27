package harness

import (
	"context"
	"fmt"
	"strings"
	"time"

	"station/pkg/harness/memory"
	"station/pkg/harness/sandbox"
	"station/pkg/harness/skills"
	"station/pkg/harness/stream"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("station.harness")

type ExecutionResult struct {
	Success      bool                   `json:"success"`
	Response     string                 `json:"response"`
	Error        string                 `json:"error,omitempty"`
	TotalSteps   int                    `json:"total_steps"`
	TotalTokens  int                    `json:"total_tokens"`
	FinishReason string                 `json:"finish_reason"`
	Duration     time.Duration          `json:"duration"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	History      []*ai.Message          `json:"-"` // Full message history for session persistence
}

type AgenticExecutor struct {
	genkitApp        *genkit.Genkit
	config           *HarnessConfig
	agentConfig      *AgentHarnessConfig
	hooks            *HookRegistry
	doomDetector     *DoomLoopDetector
	compactor        *Compactor
	workspace        WorkspaceManager
	gitManager       GitManager
	promptBuilder    *PromptBuilder
	modelName        string
	systemPrompt     string
	streamCtx        *stream.StreamContext
	publisher        stream.Publisher
	sandbox          sandbox.Sandbox
	sandboxConfig    *sandbox.Config
	skillsMiddleware *skills.SkillsMiddleware
	memoryMiddleware *memory.MemoryMiddleware
	initialHistory   []*ai.Message
}

type ExecutorOption func(*AgenticExecutor)

func WithHooks(hooks *HookRegistry) ExecutorOption {
	return func(e *AgenticExecutor) {
		e.hooks = hooks
	}
}

func WithCompactor(compactor *Compactor) ExecutorOption {
	return func(e *AgenticExecutor) {
		e.compactor = compactor
	}
}

func WithWorkspace(workspace WorkspaceManager) ExecutorOption {
	return func(e *AgenticExecutor) {
		e.workspace = workspace
	}
}

func WithGitManager(gitManager GitManager) ExecutorOption {
	return func(e *AgenticExecutor) {
		e.gitManager = gitManager
	}
}

func WithModelName(modelName string) ExecutorOption {
	return func(e *AgenticExecutor) {
		e.modelName = modelName
	}
}

func WithSystemPrompt(systemPrompt string) ExecutorOption {
	return func(e *AgenticExecutor) {
		e.systemPrompt = systemPrompt
	}
}

func WithStreamPublisher(publisher stream.Publisher) ExecutorOption {
	return func(e *AgenticExecutor) {
		e.publisher = publisher
	}
}

func WithSandbox(sb sandbox.Sandbox) ExecutorOption {
	return func(e *AgenticExecutor) {
		e.sandbox = sb
	}
}

func WithSandboxConfig(cfg *sandbox.Config) ExecutorOption {
	return func(e *AgenticExecutor) {
		e.sandboxConfig = cfg
	}
}

func WithSkillsMiddleware(sm *skills.SkillsMiddleware) ExecutorOption {
	return func(e *AgenticExecutor) {
		e.skillsMiddleware = sm
	}
}

func WithMemoryMiddleware(mm *memory.MemoryMiddleware) ExecutorOption {
	return func(e *AgenticExecutor) {
		e.memoryMiddleware = mm
	}
}

// WithInitialHistory sets the initial message history for session persistence.
// This allows REPL-style conversations where the agent can continue from
// previous interactions with full context.
func WithInitialHistory(history []*ai.Message) ExecutorOption {
	return func(e *AgenticExecutor) {
		e.initialHistory = history
	}
}

func NewAgenticExecutor(
	genkitApp *genkit.Genkit,
	config *HarnessConfig,
	agentConfig *AgentHarnessConfig,
	opts ...ExecutorOption,
) *AgenticExecutor {
	if config == nil {
		config = DefaultHarnessConfig()
	}
	if agentConfig == nil {
		agentConfig = DefaultAgentHarnessConfig()
	}

	e := &AgenticExecutor{
		genkitApp:    genkitApp,
		config:       config,
		agentConfig:  agentConfig.Merge(config),
		hooks:        NewHookRegistry(),
		doomDetector: NewDoomLoopDetector(agentConfig.DoomLoopThreshold),
		modelName:    "openai/gpt-4o-mini",
	}

	e.hooks.RegisterDoomLoopDetection(e.doomDetector)
	e.hooks.RegisterPermissionCheck(config.Permissions)

	for _, opt := range opts {
		opt(e)
	}

	return e
}

type ExecuteOptions struct {
	StationRunID  string
	RunUUID       string
	WorkflowRunID string
	SessionID     string
	AgentName     string
	StationID     string
}

func (e *AgenticExecutor) Execute(
	ctx context.Context,
	agentID string,
	task string,
	tools []ai.Tool,
) (*ExecutionResult, error) {
	return e.ExecuteWithOptions(ctx, agentID, task, tools, ExecuteOptions{})
}

func (e *AgenticExecutor) ExecuteWithOptions(
	ctx context.Context,
	agentID string,
	task string,
	tools []ai.Tool,
	opts ExecuteOptions,
) (*ExecutionResult, error) {
	ctx, span := tracer.Start(ctx, "agent_execution",
		trace.WithAttributes(
			attribute.String("agent_id", agentID),
			attribute.String("harness", "agentic"),
			attribute.Int("max_steps", e.agentConfig.MaxSteps),
		),
	)
	defer span.End()

	startTime := time.Now()

	e.initializeStreaming(ctx, agentID, task, opts, span)

	setupCtx, setupSpan := tracer.Start(ctx, "harness_setup")
	if err := e.setup(setupCtx, agentID, task); err != nil {
		setupSpan.RecordError(err)
		setupSpan.End()
		result := &ExecutionResult{
			Success:      false,
			Error:        fmt.Sprintf("setup failed: %v", err),
			FinishReason: "setup_error",
			Duration:     time.Since(startTime),
		}
		e.emitRunComplete(ctx, result, err)
		return result, err
	}
	setupSpan.End()

	loopCtx, loopSpan := tracer.Start(ctx, "agentic_loop")
	result, err := e.runLoop(loopCtx, task, tools)
	if err != nil {
		loopSpan.RecordError(err)
	}
	loopSpan.SetAttributes(
		attribute.Int("total_steps", result.TotalSteps),
		attribute.Int("total_tokens", result.TotalTokens),
		attribute.String("finish_reason", result.FinishReason),
	)
	loopSpan.End()

	cleanupCtx, cleanupSpan := tracer.Start(ctx, "harness_cleanup")
	if cleanupErr := e.cleanup(cleanupCtx, agentID, result); cleanupErr != nil {
		cleanupSpan.RecordError(cleanupErr)
	}
	cleanupSpan.End()

	result.Duration = time.Since(startTime)
	span.SetAttributes(
		attribute.Bool("success", result.Success),
		attribute.Int64("duration_ms", result.Duration.Milliseconds()),
	)

	e.emitRunComplete(ctx, result, err)

	return result, err
}

func (e *AgenticExecutor) initializeStreaming(ctx context.Context, agentID, task string, opts ExecuteOptions, span trace.Span) {
	if e.publisher == nil {
		return
	}

	runUUID := opts.RunUUID
	if runUUID == "" {
		runUUID = fmt.Sprintf("%s-%d", agentID, time.Now().UnixNano())
	}

	agentName := opts.AgentName
	if agentName == "" {
		agentName = agentID
	}

	ids := stream.StreamIdentifiers{
		StationRunID:  opts.StationRunID,
		RunUUID:       runUUID,
		WorkflowRunID: opts.WorkflowRunID,
		SessionID:     opts.SessionID,
		AgentID:       agentID,
		AgentName:     agentName,
		StationID:     opts.StationID,
	}

	e.streamCtx = stream.NewStreamContext(ids, e.publisher)

	if err := e.streamCtx.EmitRunStart(ctx, agentID, agentName, task, e.agentConfig.MaxSteps); err != nil {
		span.RecordError(err)
	}
}

func (e *AgenticExecutor) emitRunComplete(ctx context.Context, result *ExecutionResult, err error) {
	if e.streamCtx != nil {
		e.streamCtx.EmitRunComplete(ctx, result.Success, result.TotalSteps, result.TotalTokens, result.Duration.Milliseconds(), result.FinishReason, err)
	}
}

func (e *AgenticExecutor) emitStepComplete(ctx context.Context, stepNum int, totalTokens int, resp *ai.ModelResponse) {
	if e.streamCtx == nil {
		return
	}
	inputTokens := 0
	outputTokens := 0
	if resp.Usage != nil {
		inputTokens = resp.Usage.InputTokens
		outputTokens = resp.Usage.OutputTokens
	}
	finishReason := "tool_use"
	if len(resp.ToolRequests()) == 0 {
		finishReason = "stop"
	}
	e.streamCtx.EmitStepComplete(ctx, stepNum, totalTokens, inputTokens, outputTokens, finishReason)
}

func (e *AgenticExecutor) setup(ctx context.Context, agentID string, task string) error {
	_, span := tracer.Start(ctx, "harness_setup")
	defer span.End()

	// Check for context cancellation early
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := e.initializeSandbox(ctx); err != nil {
		span.RecordError(err)
		return fmt.Errorf("sandbox init failed: %w", err)
	}

	if e.workspace != nil {
		_, wsSpan := tracer.Start(ctx, "workspace_init")
		if err := e.workspace.Initialize(ctx); err != nil {
			wsSpan.RecordError(err)
			wsSpan.End()
			return fmt.Errorf("workspace init failed: %w", err)
		}
		wsSpan.SetAttributes(
			attribute.String("workspace_path", e.workspace.Path()),
			attribute.String("workspace_mode", e.config.Workspace.Mode),
		)
		wsSpan.End()
	}

	if e.gitManager != nil && e.config.Git.AutoBranch {
		_, gitSpan := tracer.Start(ctx, "git_setup")
		branchName, err := e.gitManager.CreateBranch(ctx, task, agentID)
		if err != nil {
			gitSpan.RecordError(err)
			gitSpan.End()
			return fmt.Errorf("git branch creation failed: %w", err)
		}
		gitSpan.SetAttributes(attribute.String("branch", branchName))
		gitSpan.End()
	}

	return nil
}

func (e *AgenticExecutor) initializeSandbox(ctx context.Context) error {
	if e.sandbox != nil {
		return nil
	}

	if e.sandboxConfig == nil {
		return nil
	}

	_, span := tracer.Start(ctx, "sandbox_init")
	defer span.End()

	factory := sandbox.NewFactory(sandbox.DefaultConfig())
	sb, err := factory.Create(*e.sandboxConfig)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to create sandbox: %w", err)
	}

	if err := sb.Create(ctx); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to initialize sandbox: %w", err)
	}

	e.sandbox = sb
	span.SetAttributes(
		attribute.String("sandbox_mode", string(e.sandboxConfig.Mode)),
		attribute.String("sandbox_id", sb.ID()),
	)

	return nil
}

func (e *AgenticExecutor) cleanup(ctx context.Context, agentID string, result *ExecutionResult) error {
	if e.gitManager != nil && e.config.Git.AutoCommit && result.Success {
		_, span := tracer.Start(ctx, "git_commit")
		commitSHA, err := e.gitManager.Commit(ctx, result.Response)
		if err != nil {
			span.RecordError(err)
			span.End()
			return err
		}
		span.SetAttributes(attribute.String("commit_sha", commitSHA))
		span.End()
	}

	if err := e.destroySandbox(ctx); err != nil {
		return fmt.Errorf("sandbox cleanup failed: %w", err)
	}

	return nil
}

func (e *AgenticExecutor) destroySandbox(ctx context.Context) error {
	if e.sandbox == nil {
		return nil
	}

	_, span := tracer.Start(ctx, "sandbox_destroy")
	defer span.End()

	sandboxID := e.sandbox.ID()
	if err := e.sandbox.Destroy(ctx); err != nil {
		span.RecordError(err)
		return err
	}

	span.SetAttributes(attribute.String("sandbox_id", sandboxID))
	e.sandbox = nil
	return nil
}

func (e *AgenticExecutor) Sandbox() sandbox.Sandbox {
	return e.sandbox
}

func (e *AgenticExecutor) runLoop(
	ctx context.Context,
	task string,
	tools []ai.Tool,
) (*ExecutionResult, error) {
	// Initialize history from previous session if provided
	var history []*ai.Message
	if len(e.initialHistory) > 0 {
		history = make([]*ai.Message, len(e.initialHistory))
		copy(history, e.initialHistory)
	}
	totalTokens := 0

	toolRefs := make([]ai.ToolRef, len(tools))
	for i, t := range tools {
		toolRefs[i] = t
	}

	toolMap := make(map[string]ai.Tool)
	for _, t := range tools {
		toolMap[t.Name()] = t
	}

	for step := 1; step <= e.agentConfig.MaxSteps; step++ {
		select {
		case <-ctx.Done():
			return &ExecutionResult{
				Success:      false,
				Error:        "context cancelled",
				TotalSteps:   step - 1,
				TotalTokens:  totalTokens,
				FinishReason: "cancelled",
				History:      history,
			}, ctx.Err()
		default:
		}

		stepCtx, stepSpan := tracer.Start(ctx, "step",
			trace.WithAttributes(attribute.Int("step", step)),
		)

		if e.compactor != nil && len(history) > 0 {
			compactedHistory, compacted, err := e.compactor.CompactIfNeeded(stepCtx, history)
			if err != nil {
				stepSpan.RecordError(err)
			} else if compacted {
				history = compactedHistory
				stepSpan.AddEvent("compaction_applied")
			}
		}

		genCtx, genSpan := tracer.Start(stepCtx, "llm_generate")

		// Build enhanced system prompt with skills injection
		enhancedPrompt := e.buildEnhancedSystemPrompt()

		generateOpts := []ai.GenerateOption{
			ai.WithModelName(e.modelName),
			ai.WithMessages(history...),
			ai.WithTools(toolRefs...),
		}

		if enhancedPrompt != "" {
			generateOpts = append(generateOpts, ai.WithSystem(enhancedPrompt))
		}

		if step == 1 {
			generateOpts = append(generateOpts, ai.WithPrompt(task))
		}

		resp, err := genkit.Generate(genCtx, e.genkitApp, generateOpts...)

		if err != nil {
			genSpan.RecordError(err)
			genSpan.End()
			stepSpan.End()
			return &ExecutionResult{
				Success:      false,
				Error:        fmt.Sprintf("generation failed: %v", err),
				TotalSteps:   step,
				TotalTokens:  totalTokens,
				FinishReason: "error",
				History:      history,
			}, err
		}

		if resp.Usage != nil {
			totalTokens += resp.Usage.InputTokens + resp.Usage.OutputTokens
			genSpan.SetAttributes(
				attribute.Int("input_tokens", resp.Usage.InputTokens),
				attribute.Int("output_tokens", resp.Usage.OutputTokens),
			)
		}
		genSpan.End()

		toolReqs := resp.ToolRequests()
		if len(toolReqs) == 0 {
			// Add final response to history for persistence
			history = append(history, resp.Message)
			stepSpan.SetAttributes(attribute.String("finish_reason", "agent_done"))
			stepSpan.End()
			return &ExecutionResult{
				Success:      true,
				Response:     resp.Text(),
				TotalSteps:   step,
				TotalTokens:  totalTokens,
				FinishReason: "agent_done",
				History:      history,
			}, nil
		}

		var toolParts []*ai.Part
		for _, req := range toolReqs {
			toolResult, toolErr := e.executeTool(stepCtx, req, toolMap)
			if toolErr != nil {
				toolParts = append(toolParts, ai.NewToolResponsePart(&ai.ToolResponse{
					Name:   req.Name,
					Ref:    req.Ref,
					Output: map[string]string{"error": toolErr.Error()},
				}))
				continue
			}
			toolParts = append(toolParts, ai.NewToolResponsePart(&ai.ToolResponse{
				Name:   req.Name,
				Ref:    req.Ref,
				Output: toolResult,
			}))
		}

		history = append(history, resp.Message)
		history = append(history, ai.NewMessage(ai.RoleTool, nil, toolParts...))

		e.emitStepComplete(stepCtx, step, totalTokens, resp)

		stepSpan.SetAttributes(attribute.Int("tool_count", len(toolReqs)))
		stepSpan.End()
	}

	return &ExecutionResult{
		Success:      false,
		Error:        "max steps exceeded",
		TotalSteps:   e.agentConfig.MaxSteps,
		TotalTokens:  totalTokens,
		FinishReason: "max_steps",
		History:      history,
	}, nil
}

func (e *AgenticExecutor) executeTool(
	ctx context.Context,
	req *ai.ToolRequest,
	toolMap map[string]ai.Tool,
) (interface{}, error) {
	// Check for context cancellation before executing tool
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	_, span := tracer.Start(ctx, "tool_execution",
		trace.WithAttributes(
			attribute.String("tool", req.Name),
		),
	)
	defer span.End()

	toolID := req.Ref
	if toolID == "" {
		toolID = fmt.Sprintf("%s-%d", req.Name, time.Now().UnixNano())
	}

	e.emitToolStart(ctx, req.Name, toolID, req.Input)

	preCtx, preSpan := tracer.Start(ctx, "pre_tool_hook")
	hookResult, hookMsg := e.hooks.RunPreHooks(preCtx, req)
	preSpan.SetAttributes(
		attribute.String("result", string(hookResult)),
	)
	preSpan.End()

	switch hookResult {
	case HookBlock:
		span.SetAttributes(attribute.Bool("blocked", true))
		err := fmt.Errorf("tool blocked: %s", hookMsg)
		e.emitToolResult(ctx, req.Name, toolID, nil, 0, err)
		return nil, err
	case HookInterrupt:
		span.SetAttributes(attribute.Bool("interrupted", true))
		err := fmt.Errorf("tool requires approval: %s", hookMsg)
		e.emitToolResult(ctx, req.Name, toolID, nil, 0, err)
		return nil, err
	}

	tool, exists := toolMap[req.Name]
	if !exists {
		err := fmt.Errorf("tool not found: %s", req.Name)
		e.emitToolResult(ctx, req.Name, toolID, nil, 0, err)
		return nil, err
	}

	runCtx, runSpan := tracer.Start(ctx, "tool_run")
	startTime := time.Now()

	output, err := tool.RunRaw(runCtx, req.Input)

	durationMs := time.Since(startTime).Milliseconds()
	runSpan.SetAttributes(
		attribute.Int64("duration_ms", durationMs),
		attribute.Bool("success", err == nil),
	)
	runSpan.End()

	e.emitToolResult(ctx, req.Name, toolID, output, durationMs, err)

	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	postCtx, postSpan := tracer.Start(ctx, "post_tool_hook")
	e.hooks.RunPostHooks(postCtx, req, output)
	postSpan.End()

	e.doomDetector.Record(req.Name, req.Input)

	return output, nil
}

func (e *AgenticExecutor) emitToolStart(ctx context.Context, toolName, toolID string, input any) {
	if e.streamCtx != nil {
		e.streamCtx.EmitToolStart(ctx, toolName, toolID, input)
	}
}

func (e *AgenticExecutor) emitToolResult(ctx context.Context, toolName, toolID string, output any, durationMs int64, err error) {
	if e.streamCtx != nil {
		e.streamCtx.EmitToolResult(ctx, toolName, toolID, output, durationMs, err)
	}
}

// buildEnhancedSystemPrompt combines the base system prompt with skills and memory sections
func (e *AgenticExecutor) buildEnhancedSystemPrompt() string {
	var parts []string

	// Base agent prompt
	if e.systemPrompt != "" {
		parts = append(parts, e.systemPrompt)
	}

	// Skills injection (progressive disclosure - only names and descriptions)
	if e.skillsMiddleware != nil {
		skillsList, err := e.skillsMiddleware.LoadSkills()
		if err == nil && len(skillsList) > 0 {
			skillsSection := e.skillsMiddleware.FormatSystemPromptSection(skillsList)
			if skillsSection != "" {
				parts = append(parts, skillsSection)
			}
		}
	}

	// Memory injection (always loaded, unlike skills)
	if e.memoryMiddleware != nil {
		memorySection, err := e.memoryMiddleware.FormatSystemPromptSection()
		if err == nil && memorySection != "" {
			parts = append(parts, memorySection)
		}
	}

	return strings.Join(parts, "\n")
}

type WorkspaceManager interface {
	Initialize(ctx context.Context) error
	Path() string
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, path string, data []byte) error
	Close(ctx context.Context) error
}

type GitManager interface {
	CreateBranch(ctx context.Context, task string, agentID string) (string, error)
	Commit(ctx context.Context, message string) (string, error)
	Push(ctx context.Context) error
	GetCurrentBranch(ctx context.Context) (string, error)
}

type PromptBuilder struct {
	config *HarnessConfig
}
