package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/workflows"
	"station/internal/workflows/runtime"
)

// Workflow command definitions
var (
	workflowCmd = &cobra.Command{
		Use:   "workflow",
		Short: "Manage workflows",
		Long:  "List, show, and run workflows",
	}

	workflowListCmd = &cobra.Command{
		Use:   "list",
		Short: "List workflows",
		Long:  "List all available workflow definitions",
		RunE:  runWorkflowList,
	}

	workflowShowCmd = &cobra.Command{
		Use:   "show <workflow-id>",
		Short: "Show workflow details",
		Long:  "Show detailed information about a workflow by ID",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowShow,
	}

	workflowRunCmd = &cobra.Command{
		Use:   "run <workflow-id>",
		Short: "Run a workflow",
		Long:  "Execute a workflow by ID with optional input JSON",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowRun,
	}

	workflowRunsCmd = &cobra.Command{
		Use:   "runs [workflow-id]",
		Short: "List workflow runs",
		Long:  "List workflow runs, optionally filtered by workflow ID",
		RunE:  runWorkflowRuns,
	}

	workflowInspectCmd = &cobra.Command{
		Use:   "inspect <run-id>",
		Short: "Inspect a workflow run",
		Long:  "Show detailed information about a workflow run including steps",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowInspect,
	}

	workflowDebugExpressionCmd = &cobra.Command{
		Use:   "debug-expression <expression>",
		Short: "Evaluate a Starlark expression",
		Long:  "Evaluate a Starlark expression against an optional JSON context. Useful for testing switch conditions and transforms.",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowDebugExpression,
	}
)

func runWorkflowDebugExpression(cmd *cobra.Command, args []string) error {
	expression := args[0]
	contextJSON, _ := cmd.Flags().GetString("context")
	runID, _ := cmd.Flags().GetString("run-id")
	dataPath, _ := cmd.Flags().GetString("data-path")

	var data map[string]interface{}
	if runID != "" {
		// Load context from a specific run
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		database, err := db.New(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer func() { _ = database.Close() }()
		repos := repositories.New(database)
		run, err := repos.WorkflowRuns.Get(context.Background(), runID)
		if err != nil {
			return fmt.Errorf("run not found: %w", err)
		}
		if err := json.Unmarshal(run.Context, &data); err != nil {
			return fmt.Errorf("failed to parse run context: %w", err)
		}
		fmt.Printf("Using context from run: %s\n", runID)
	} else if contextJSON != "" {
		if err := json.Unmarshal([]byte(contextJSON), &data); err != nil {
			return fmt.Errorf("invalid context JSON: %w", err)
		}
	} else {
		data = make(map[string]interface{})
	}

	// Apply dataPath if specified
	evalData := data
	if dataPath != "" && dataPath != "$" && dataPath != "$. " {
		val, ok := runtime.GetNestedValue(data, dataPath)
		if !ok {
			return fmt.Errorf("data path not found: %s", dataPath)
		}
		if m, ok := val.(map[string]interface{}); ok {
			evalData = m
		} else {
			// If it's not a map, we wrap it so hasattr works on the 'data' variable or similar
			// But usually Switch/Transform expect a map-like context.
			// For simplicity, we'll just use it as is if it's a map.
			fmt.Printf("Warning: data path returned %T, expected map for full compatibility\n", val)
			evalData = map[string]interface{}{"data": val}
		}
		fmt.Printf("Extracted data from path: %s\n", dataPath)
	}

	evaluator := runtime.NewStarlarkEvaluator()
	result, err := evaluator.EvaluateExpression(expression, evalData)
	if err != nil {
		// Try evaluating as condition if expression fails (might be a boolean expression)
		condResult, condErr := evaluator.EvaluateCondition(expression, evalData)
		if condErr == nil {
			fmt.Printf("Result (Condition): %v\n", condResult)
			return nil
		}
		return fmt.Errorf("evaluation failed: %w", err)
	}

	fmt.Printf("Result: %v\n", result)
	if resultDict, ok := result.(map[string]interface{}); ok {
		pretty, _ := json.MarshalIndent(resultDict, "", "  ")
		fmt.Printf("JSON: %s\n", string(pretty))
	}

	return nil
}

func runWorkflowList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)
	workflowService := services.NewWorkflowService(repos)

	ctx := context.Background()
	workflows, err := workflowService.ListWorkflows(ctx)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	if len(workflows) == 0 {
		fmt.Println("No workflows found.")
		fmt.Println("Create workflow files in ~/.config/station/environments/<env>/workflows/")
		fmt.Println("Then run 'stn sync <env>' to load them.")
		return nil
	}

	fmt.Printf("Workflows (%d):\n\n", len(workflows))
	for _, wf := range workflows {
		statusIcon := "✅"
		if wf.Status != "active" {
			statusIcon = "⏸️"
		}
		fmt.Printf("%s %s (v%d)\n", statusIcon, wf.WorkflowID, wf.Version)
		if wf.Name != "" && wf.Name != wf.WorkflowID {
			fmt.Printf("   Name: %s\n", wf.Name)
		}
		if wf.Description != nil && *wf.Description != "" {
			fmt.Printf("   Description: %s\n", *wf.Description)
		}
		fmt.Printf("   Status: %s\n", wf.Status)
		fmt.Printf("   Updated: %s\n\n", wf.UpdatedAt.Format(time.RFC3339))
	}

	return nil
}

func runWorkflowShow(cmd *cobra.Command, args []string) error {
	workflowID := args[0]
	version, _ := cmd.Flags().GetInt64("version")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)
	workflowService := services.NewWorkflowService(repos)

	ctx := context.Background()
	wf, err := workflowService.GetWorkflow(ctx, workflowID, version)
	if err != nil {
		return fmt.Errorf("workflow not found: %w", err)
	}

	fmt.Printf("📋 Workflow: %s\n", wf.WorkflowID)
	fmt.Printf("   Version: %d\n", wf.Version)
	if wf.Name != "" {
		fmt.Printf("   Name: %s\n", wf.Name)
	}
	if wf.Description != nil && *wf.Description != "" {
		fmt.Printf("   Description: %s\n", *wf.Description)
	}
	fmt.Printf("   Status: %s\n", wf.Status)
	fmt.Printf("   Created: %s\n", wf.CreatedAt.Format(time.RFC3339))
	fmt.Printf("   Updated: %s\n", wf.UpdatedAt.Format(time.RFC3339))

	// Pretty print definition
	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		fmt.Printf("\nDefinition:\n")
		var prettyDef interface{}
		if err := json.Unmarshal(wf.Definition, &prettyDef); err == nil {
			pretty, _ := json.MarshalIndent(prettyDef, "", "  ")
			fmt.Println(string(pretty))
		} else {
			fmt.Println(string(wf.Definition))
		}
	}

	return nil
}

func runWorkflowRun(cmd *cobra.Command, args []string) error {
	workflowID := args[0]
	inputJSON, _ := cmd.Flags().GetString("input")
	version, _ := cmd.Flags().GetInt64("version")
	wait, _ := cmd.Flags().GetBool("wait")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)

	opts := runtime.EnvOptions()
	engine, err := runtime.NewEngine(opts)
	if err != nil {
		return fmt.Errorf("failed to create workflow engine: %w", err)
	}
	if engine == nil {
		return fmt.Errorf("NATS engine not available")
	}
	defer engine.Close()

	ctx := context.Background()

	agentService := services.NewAgentService(repos)
	consumer := startCLIWorkflowConsumer(ctx, repos, engine, agentService)
	if consumer != nil {
		defer consumer.Stop()
	}

	workflowService := services.NewWorkflowServiceWithEngine(repos, engine)

	// Get default environment ID
	var environmentID int64 = 1
	if env, err := repos.Environments.GetByName("default"); err == nil {
		environmentID = env.ID
	}

	// Parse input JSON
	var input json.RawMessage
	if inputJSON != "" {
		input = json.RawMessage(inputJSON)
		// Validate it's valid JSON
		var test interface{}
		if err := json.Unmarshal(input, &test); err != nil {
			return fmt.Errorf("invalid input JSON: %w", err)
		}
	}

	fmt.Printf("🚀 Starting workflow: %s\n", workflowID)
	if inputJSON != "" {
		fmt.Printf("   Input: %s\n", inputJSON)
	}

	run, validation, err := workflowService.StartRun(ctx, services.StartWorkflowRunRequest{
		WorkflowID:    workflowID,
		Version:       version,
		EnvironmentID: environmentID,
		Input:         input,
	})
	if err != nil {
		if len(validation.Errors) > 0 {
			fmt.Printf("\n❌ Validation errors:\n")
			for _, ve := range validation.Errors {
				fmt.Printf("   • %s: %s\n", ve.Path, ve.Message)
				if ve.Hint != "" {
					fmt.Printf("     💡 %s\n", ve.Hint)
				}
			}
		}
		return fmt.Errorf("failed to start workflow: %w", err)
	}

	if len(validation.Warnings) > 0 {
		fmt.Printf("\n⚠️  Warnings:\n")
		for _, vw := range validation.Warnings {
			fmt.Printf("   • %s: %s\n", vw.Path, vw.Message)
		}
	}

	fmt.Printf("\n✅ Workflow run started!\n")
	fmt.Printf("   Run ID: %s\n", run.RunID)
	fmt.Printf("   Status: %s\n", run.Status)
	if run.CurrentStep != nil {
		fmt.Printf("   Current Step: %s\n", *run.CurrentStep)
	}
	fmt.Printf("   Started: %s\n", run.StartedAt.Format(time.RFC3339))

	// If wait flag is set, poll for completion
	if wait {
		fmt.Printf("\n⏳ Waiting for workflow to complete (timeout: %s)...\n", timeout)

		deadline := time.Now().Add(timeout)
		pollInterval := 2 * time.Second

		for time.Now().Before(deadline) {
			time.Sleep(pollInterval)

			run, err = workflowService.GetRun(ctx, run.RunID)
			if err != nil {
				return fmt.Errorf("failed to get run status: %w", err)
			}

			switch run.Status {
			case "completed":
				fmt.Printf("\n✅ Workflow completed!\n")
				if run.Result != nil && len(run.Result) > 0 {
					fmt.Printf("\nResult:\n")
					var prettyResult interface{}
					if err := json.Unmarshal(run.Result, &prettyResult); err == nil {
						pretty, _ := json.MarshalIndent(prettyResult, "", "  ")
						fmt.Println(string(pretty))
					} else {
						fmt.Println(string(run.Result))
					}
				}
				if run.CompletedAt != nil {
					duration := run.CompletedAt.Sub(run.StartedAt)
					fmt.Printf("\nDuration: %s\n", duration)
				}
				return nil

			case "failed":
				fmt.Printf("\n❌ Workflow failed!\n")
				if run.Error != nil {
					fmt.Printf("   Error: %s\n", *run.Error)
				}
				return fmt.Errorf("workflow failed")

			case "canceled":
				fmt.Printf("\n⏹️ Workflow was canceled\n")
				return fmt.Errorf("workflow canceled")

			default:
				// Still running, show progress
				step := "starting"
				if run.CurrentStep != nil {
					step = *run.CurrentStep
				}
				fmt.Printf("   Status: %s (step: %s)\n", run.Status, step)
			}
		}

		return fmt.Errorf("workflow did not complete within timeout (%s)", timeout)
	}

	fmt.Printf("\n💡 Use 'stn workflow inspect %s' to check progress\n", run.RunID)
	return nil
}

func runWorkflowRuns(cmd *cobra.Command, args []string) error {
	var workflowID string
	if len(args) > 0 {
		workflowID = args[0]
	}
	limit, _ := cmd.Flags().GetInt64("limit")
	status, _ := cmd.Flags().GetString("status")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)
	workflowService := services.NewWorkflowService(repos)

	ctx := context.Background()
	runs, err := workflowService.ListRuns(ctx, workflowID, status, limit)
	if err != nil {
		return fmt.Errorf("failed to list runs: %w", err)
	}

	if len(runs) == 0 {
		fmt.Println("No workflow runs found.")
		return nil
	}

	fmt.Printf("Workflow Runs (%d):\n\n", len(runs))
	for _, run := range runs {
		statusIcon := getStatusIcon(run.Status)
		fmt.Printf("%s %s\n", statusIcon, run.RunID)
		fmt.Printf("   Workflow: %s (v%d)\n", run.WorkflowID, run.WorkflowVersion)
		fmt.Printf("   Status: %s\n", run.Status)
		if run.CurrentStep != nil {
			fmt.Printf("   Current Step: %s\n", *run.CurrentStep)
		}
		fmt.Printf("   Started: %s\n", run.StartedAt.Format(time.RFC3339))
		if run.CompletedAt != nil {
			duration := run.CompletedAt.Sub(run.StartedAt)
			fmt.Printf("   Completed: %s (took %s)\n", run.CompletedAt.Format(time.RFC3339), duration)
		}
		if run.Error != nil && *run.Error != "" {
			fmt.Printf("   Error: %s\n", *run.Error)
		}
		fmt.Println()
	}

	return nil
}

func runWorkflowInspect(cmd *cobra.Command, args []string) error {
	runID := args[0]
	verbose, _ := cmd.Flags().GetBool("verbose")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)
	workflowService := services.NewWorkflowService(repos)

	ctx := context.Background()
	run, err := workflowService.GetRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("run not found: %w", err)
	}

	statusIcon := getStatusIcon(run.Status)
	fmt.Printf("%s Workflow Run: %s\n", statusIcon, run.RunID)
	fmt.Printf("   Workflow: %s (v%d)\n", run.WorkflowID, run.WorkflowVersion)
	fmt.Printf("   Status: %s\n", run.Status)
	if run.CurrentStep != nil {
		fmt.Printf("   Current Step: %s\n", *run.CurrentStep)
	}
	fmt.Printf("   Started: %s\n", run.StartedAt.Format(time.RFC3339))
	if run.CompletedAt != nil {
		duration := run.CompletedAt.Sub(run.StartedAt)
		fmt.Printf("   Completed: %s (took %s)\n", run.CompletedAt.Format(time.RFC3339), duration)
	}
	if run.Error != nil && *run.Error != "" {
		fmt.Printf("   Error: %s\n", *run.Error)
	}

	// Show input
	if run.Input != nil && len(run.Input) > 0 {
		fmt.Printf("\nInput:\n")
		var prettyInput interface{}
		if err := json.Unmarshal(run.Input, &prettyInput); err == nil {
			pretty, _ := json.MarshalIndent(prettyInput, "", "  ")
			fmt.Println(string(pretty))
		}
	}

	// Show result
	if run.Result != nil && len(run.Result) > 0 {
		fmt.Printf("\nResult:\n")
		var prettyResult interface{}
		if err := json.Unmarshal(run.Result, &prettyResult); err == nil {
			pretty, _ := json.MarshalIndent(prettyResult, "", "  ")
			fmt.Println(string(pretty))
		}
	}

	// Show steps
	steps, err := workflowService.ListSteps(ctx, runID)
	if err == nil && len(steps) > 0 {
		fmt.Printf("\nSteps (%d):\n", len(steps))
		for _, step := range steps {
			stepIcon := getStatusIcon(step.Status)
			fmt.Printf("  %s %s\n", stepIcon, step.StepID)
			fmt.Printf("     Status: %s\n", step.Status)
			if !step.StartedAt.IsZero() {
				fmt.Printf("     Started: %s\n", step.StartedAt.Format(time.RFC3339))
			}
			if step.CompletedAt != nil && !step.StartedAt.IsZero() {
				duration := step.CompletedAt.Sub(step.StartedAt)
				fmt.Printf("     Duration: %s\n", duration)
			}
			if step.Error != nil && *step.Error != "" {
				fmt.Printf("     Error: %s\n", *step.Error)
			}
			if verbose && step.Output != nil && len(step.Output) > 0 {
				fmt.Printf("     Output:\n")
				var prettyOutput interface{}
				if err := json.Unmarshal(step.Output, &prettyOutput); err == nil {
					pretty, _ := json.MarshalIndent(prettyOutput, "", "       ")
					fmt.Println(string(pretty))
				}
			}
		}
	}

	// Show context if verbose
	if verbose && run.Context != nil && len(run.Context) > 0 {
		fmt.Printf("\nContext:\n")
		var prettyContext interface{}
		if err := json.Unmarshal(run.Context, &prettyContext); err == nil {
			pretty, _ := json.MarshalIndent(prettyContext, "", "  ")
			fmt.Println(string(pretty))
		}
	}

	return nil
}

func getStatusIcon(status string) string {
	switch status {
	case "completed":
		return "✅"
	case "failed":
		return "❌"
	case "running", "pending":
		return "🔄"
	case "waiting_approval":
		return "⏸️"
	case "canceled":
		return "⏹️"
	default:
		return "❓"
	}
}

func startCLIWorkflowConsumer(ctx context.Context, repos *repositories.Repositories, engine *runtime.NATSEngine, agentService services.AgentServiceInterface) *runtime.WorkflowConsumer {
	registry := runtime.NewExecutorRegistry()
	registry.Register(runtime.NewInjectExecutor())
	registry.Register(runtime.NewSwitchExecutor())
	registry.Register(runtime.NewAgentRunExecutor(&cliAgentExecutorAdapter{agentService: agentService, repos: repos}))
	registry.Register(runtime.NewHumanApprovalExecutor(&cliApprovalExecutorAdapter{repos: repos}))
	registry.Register(runtime.NewCustomExecutor(nil))
	registry.Register(runtime.NewCronExecutor())
	registry.Register(runtime.NewTimerExecutor())
	registry.Register(runtime.NewTryCatchExecutor(registry))
	registry.Register(runtime.NewTransformExecutor())

	stepAdapter := &cliRegistryStepExecutorAdapter{registry: registry}
	registry.Register(runtime.NewParallelExecutor(stepAdapter))
	registry.Register(runtime.NewForeachExecutor(stepAdapter))

	adapter := runtime.NewWorkflowServiceAdapter(repos, engine)

	consumer := runtime.NewWorkflowConsumer(engine, registry, adapter, adapter, adapter)
	consumer.SetPendingRunProvider(adapter)

	if err := consumer.Start(ctx); err != nil {
		log.Printf("Workflow consumer: failed to start: %v", err)
		return nil
	}

	log.Println("Workflow consumer started for CLI")
	return consumer
}

type cliAgentExecutorAdapter struct {
	agentService services.AgentServiceInterface
	repos        *repositories.Repositories
}

func (a *cliAgentExecutorAdapter) GetAgentByID(ctx context.Context, id int64) (runtime.AgentInfo, error) {
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

func (a *cliAgentExecutorAdapter) GetAgentByNameAndEnvironment(ctx context.Context, name string, environmentID int64) (runtime.AgentInfo, error) {
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

func (a *cliAgentExecutorAdapter) GetAgentByNameGlobal(ctx context.Context, name string) (runtime.AgentInfo, error) {
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

func (a *cliAgentExecutorAdapter) GetEnvironmentIDByName(ctx context.Context, name string) (int64, error) {
	env, err := a.repos.Environments.GetByName(name)
	if err != nil {
		return 0, err
	}
	return env.ID, nil
}

func (a *cliAgentExecutorAdapter) ExecuteAgent(ctx context.Context, agentID int64, task string, variables map[string]interface{}) (runtime.AgentExecutionResult, error) {
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

type cliApprovalExecutorAdapter struct {
	repos *repositories.Repositories
}

func (a *cliApprovalExecutorAdapter) CreateApproval(ctx context.Context, params runtime.CreateApprovalParams) (runtime.ApprovalInfo, error) {
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

func (a *cliApprovalExecutorAdapter) GetApproval(ctx context.Context, approvalID string) (runtime.ApprovalInfo, error) {
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

type cliRegistryStepExecutorAdapter struct {
	registry *runtime.ExecutorRegistry
}

func (a *cliRegistryStepExecutorAdapter) ExecuteStep(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (runtime.StepResult, error) {
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
