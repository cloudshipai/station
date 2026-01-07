package harness

import (
	"context"
	"fmt"
	"time"

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
}

type AgenticExecutor struct {
	genkitApp     *genkit.Genkit
	config        *HarnessConfig
	agentConfig   *AgentHarnessConfig
	hooks         *HookRegistry
	doomDetector  *DoomLoopDetector
	compactor     *Compactor
	workspace     WorkspaceManager
	gitManager    GitManager
	promptBuilder *PromptBuilder
	modelName     string
	systemPrompt  string
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

func (e *AgenticExecutor) Execute(
	ctx context.Context,
	agentID string,
	task string,
	tools []ai.Tool,
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

	setupCtx, setupSpan := tracer.Start(ctx, "harness_setup")
	if err := e.setup(setupCtx, agentID, task); err != nil {
		setupSpan.RecordError(err)
		setupSpan.End()
		return &ExecutionResult{
			Success:      false,
			Error:        fmt.Sprintf("setup failed: %v", err),
			FinishReason: "setup_error",
			Duration:     time.Since(startTime),
		}, err
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

	return result, err
}

func (e *AgenticExecutor) setup(ctx context.Context, agentID string, task string) error {
	_, span := tracer.Start(ctx, "workspace_init")
	defer span.End()

	if e.workspace != nil {
		if err := e.workspace.Initialize(ctx); err != nil {
			return fmt.Errorf("workspace init failed: %w", err)
		}
		span.SetAttributes(
			attribute.String("workspace_path", e.workspace.Path()),
			attribute.String("workspace_mode", e.config.Workspace.Mode),
		)
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

	return nil
}

func (e *AgenticExecutor) runLoop(
	ctx context.Context,
	task string,
	tools []ai.Tool,
) (*ExecutionResult, error) {
	var history []*ai.Message
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

		generateOpts := []ai.GenerateOption{
			ai.WithModelName(e.modelName),
			ai.WithMessages(history...),
			ai.WithTools(toolRefs...),
		}

		if e.systemPrompt != "" {
			generateOpts = append(generateOpts, ai.WithSystem(e.systemPrompt))
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
			stepSpan.SetAttributes(attribute.String("finish_reason", "agent_done"))
			stepSpan.End()
			return &ExecutionResult{
				Success:      true,
				Response:     resp.Text(),
				TotalSteps:   step,
				TotalTokens:  totalTokens,
				FinishReason: "agent_done",
			}, nil
		}

		var toolParts []*ai.Part
		for _, req := range toolReqs {
			toolResult, err := e.executeTool(stepCtx, req, toolMap)
			if err != nil {
				toolParts = append(toolParts, ai.NewToolResponsePart(&ai.ToolResponse{
					Name:   req.Name,
					Ref:    req.Ref,
					Output: map[string]string{"error": err.Error()},
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

		stepSpan.SetAttributes(attribute.Int("tool_count", len(toolReqs)))
		stepSpan.End()
	}

	return &ExecutionResult{
		Success:      false,
		Error:        "max steps exceeded",
		TotalSteps:   e.agentConfig.MaxSteps,
		TotalTokens:  totalTokens,
		FinishReason: "max_steps",
	}, nil
}

func (e *AgenticExecutor) executeTool(
	ctx context.Context,
	req *ai.ToolRequest,
	toolMap map[string]ai.Tool,
) (interface{}, error) {
	_, span := tracer.Start(ctx, "tool_execution",
		trace.WithAttributes(
			attribute.String("tool", req.Name),
		),
	)
	defer span.End()

	preCtx, preSpan := tracer.Start(ctx, "pre_tool_hook")
	hookResult, hookMsg := e.hooks.RunPreHooks(preCtx, req)
	preSpan.SetAttributes(
		attribute.String("result", string(hookResult)),
	)
	preSpan.End()

	switch hookResult {
	case HookBlock:
		span.SetAttributes(attribute.Bool("blocked", true))
		return nil, fmt.Errorf("tool blocked: %s", hookMsg)
	case HookInterrupt:
		span.SetAttributes(attribute.Bool("interrupted", true))
		return nil, fmt.Errorf("tool requires approval: %s", hookMsg)
	}

	tool, exists := toolMap[req.Name]
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", req.Name)
	}

	runCtx, runSpan := tracer.Start(ctx, "tool_run")
	startTime := time.Now()

	output, err := tool.RunRaw(runCtx, req.Input)

	runSpan.SetAttributes(
		attribute.Int64("duration_ms", time.Since(startTime).Milliseconds()),
		attribute.Bool("success", err == nil),
	)
	runSpan.End()

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
