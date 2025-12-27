package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/workflows"
	"station/internal/workflows/runtime"
	"station/pkg/models"
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

	workflowExportCmd = &cobra.Command{
		Use:   "export <workflow-id>",
		Short: "Export workflow to YAML file",
		Long:  "Export a workflow from the database to a YAML file in the environment's workflows directory. This enables the workflow to be included in bundles.",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowExport,
	}

	workflowApprovalsCmd = &cobra.Command{
		Use:   "approvals",
		Short: "Manage workflow approvals",
		Long:  "List, approve, or reject pending workflow approvals",
	}

	workflowApprovalsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List pending approvals",
		Long:  "List all pending workflow approvals awaiting decision",
		RunE:  runWorkflowApprovalsList,
	}

	workflowApprovalsApproveCmd = &cobra.Command{
		Use:   "approve <approval-id>",
		Short: "Approve a workflow step",
		Long:  "Approve a pending workflow step, allowing the workflow to continue",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowApprovalsApprove,
	}

	workflowApprovalsRejectCmd = &cobra.Command{
		Use:   "reject <approval-id>",
		Short: "Reject a workflow step",
		Long:  "Reject a pending workflow step, causing the workflow to fail",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowApprovalsReject,
	}

	workflowDeleteCmd = &cobra.Command{
		Use:   "delete [workflow-id...]",
		Short: "Delete workflows permanently",
		Long:  "Permanently delete workflows from the database. Use --all to delete all workflows.",
		RunE:  runWorkflowDelete,
	}

	workflowValidateCmd = &cobra.Command{
		Use:   "validate <workflow-file>",
		Short: "Validate a workflow definition",
		Long: `Validate a workflow YAML file against the Station workflow schema.

Checks for:
- Required fields (id, states, types)
- Valid step transitions
- Starlark expression syntax
- Duplicate step IDs`,
		Args: cobra.ExactArgs(1),
		RunE: runWorkflowValidate,
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

func runWorkflowExport(cmd *cobra.Command, args []string) error {
	workflowID := args[0]
	version, _ := cmd.Flags().GetInt64("version")
	envName, _ := cmd.Flags().GetString("environment")
	outputPath, _ := cmd.Flags().GetString("output")

	if envName == "" {
		envName = "default"
	}

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

	var defMap map[string]interface{}
	if err := json.Unmarshal(wf.Definition, &defMap); err != nil {
		return fmt.Errorf("failed to parse workflow definition: %w", err)
	}

	yamlBytes, err := yaml.Marshal(defMap)
	if err != nil {
		return fmt.Errorf("failed to convert to YAML: %w", err)
	}

	if outputPath == "" {
		outputPath = config.GetWorkflowFilePath(envName, workflowID)
	}

	workflowsDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return fmt.Errorf("failed to create workflows directory: %w", err)
	}

	if err := os.WriteFile(outputPath, yamlBytes, 0644); err != nil {
		return fmt.Errorf("failed to write workflow file: %w", err)
	}

	fmt.Printf("✅ Exported workflow '%s' (v%d) to:\n", wf.WorkflowID, wf.Version)
	fmt.Printf("   %s\n", outputPath)
	fmt.Printf("\n💡 This workflow will now be included in 'stn bundle' for environment '%s'\n", envName)

	return nil
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

func runWorkflowApprovalsList(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")

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
	ctx := context.Background()

	approvals, err := repos.WorkflowApprovals.ListPending(ctx, 100)
	if err != nil {
		return fmt.Errorf("failed to list approvals: %w", err)
	}

	if !all {
		var pending []*models.WorkflowApproval
		for _, a := range approvals {
			if a.Status == "pending" {
				pending = append(pending, a)
			}
		}
		approvals = pending
	}

	if len(approvals) == 0 {
		fmt.Println("No pending approvals.")
		return nil
	}

	fmt.Printf("Pending Approvals (%d):\n\n", len(approvals))
	for _, a := range approvals {
		fmt.Printf("⏸️  %s\n", a.ApprovalID)
		fmt.Printf("   Run: %s\n", a.RunID)
		fmt.Printf("   Step: %s\n", a.StepID)
		fmt.Printf("   Message: %s\n", a.Message)
		if a.Approvers != nil && *a.Approvers != "" {
			fmt.Printf("   Approvers: %s\n", *a.Approvers)
		}
		if a.TimeoutAt != nil {
			remaining := time.Until(*a.TimeoutAt)
			if remaining > 0 {
				fmt.Printf("   Timeout: %s (in %s)\n", a.TimeoutAt.Format(time.RFC3339), remaining.Round(time.Second))
			} else {
				fmt.Printf("   Timeout: %s (expired)\n", a.TimeoutAt.Format(time.RFC3339))
			}
		}
		fmt.Printf("   Created: %s\n\n", a.CreatedAt.Format(time.RFC3339))
	}

	fmt.Println("💡 Use 'stn workflow approvals approve <id>' or 'stn workflow approvals reject <id>' to decide")
	return nil
}

func runWorkflowApprovalsApprove(cmd *cobra.Command, args []string) error {
	approvalID := args[0]
	comment, _ := cmd.Flags().GetString("comment")

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
	if engine != nil {
		defer engine.Close()
	}

	workflowService := services.NewWorkflowServiceWithEngine(repos, engine)

	ctx := context.Background()
	approval, err := workflowService.ApproveWorkflowStep(ctx, services.ApproveWorkflowStepRequest{
		ApprovalID: approvalID,
		ApproverID: "cli-user",
		Comment:    comment,
	})
	if err != nil {
		return fmt.Errorf("failed to approve: %w", err)
	}

	fmt.Printf("✅ Approved: %s\n", approval.ApprovalID)
	fmt.Printf("   Run: %s\n", approval.RunID)
	fmt.Printf("   Step: %s\n", approval.StepID)
	fmt.Printf("   Status: %s\n", approval.Status)
	fmt.Println("\n🚀 Workflow will resume automatically")
	return nil
}

func runWorkflowApprovalsReject(cmd *cobra.Command, args []string) error {
	approvalID := args[0]
	reason, _ := cmd.Flags().GetString("reason")

	if reason == "" {
		reason = "Rejected via CLI"
	}

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
	if engine != nil {
		defer engine.Close()
	}

	workflowService := services.NewWorkflowServiceWithEngine(repos, engine)

	ctx := context.Background()
	approval, err := workflowService.RejectWorkflowStep(ctx, services.RejectWorkflowStepRequest{
		ApprovalID: approvalID,
		RejecterID: "cli-user",
		Reason:     reason,
	})
	if err != nil {
		return fmt.Errorf("failed to reject: %w", err)
	}

	fmt.Printf("❌ Rejected: %s\n", approval.ApprovalID)
	fmt.Printf("   Run: %s\n", approval.RunID)
	fmt.Printf("   Step: %s\n", approval.StepID)
	fmt.Printf("   Reason: %s\n", reason)
	fmt.Println("\n⏹️  Workflow has been stopped")
	return nil
}

func runWorkflowDelete(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")
	force, _ := cmd.Flags().GetBool("force")

	if !all && len(args) == 0 {
		return fmt.Errorf("specify workflow IDs to delete, or use --all to delete all workflows")
	}

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

	if all {
		workflowList, err := workflowService.ListWorkflows(ctx)
		if err != nil {
			return fmt.Errorf("failed to list workflows: %w", err)
		}

		if len(workflowList) == 0 {
			fmt.Println("No workflows to delete.")
			return nil
		}

		if !force {
			fmt.Printf("⚠️  This will permanently delete %d workflow(s):\n", len(workflowList))
			for _, wf := range workflowList {
				fmt.Printf("   • %s (v%d)\n", wf.WorkflowID, wf.Version)
			}
			fmt.Print("\nType 'yes' to confirm: ")
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		count, err := workflowService.DeleteWorkflows(ctx, services.DeleteWorkflowsRequest{All: true})
		if err != nil {
			return fmt.Errorf("failed to delete workflows: %w", err)
		}

		fmt.Printf("✅ Deleted %d workflow(s)\n", count)
		return nil
	}

	if !force {
		fmt.Printf("⚠️  This will permanently delete %d workflow(s):\n", len(args))
		for _, id := range args {
			fmt.Printf("   • %s\n", id)
		}
		fmt.Print("\nType 'yes' to confirm: ")
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	count, err := workflowService.DeleteWorkflows(ctx, services.DeleteWorkflowsRequest{WorkflowIDs: args})
	if err != nil {
		return fmt.Errorf("failed to delete workflows: %w", err)
	}

	fmt.Printf("✅ Deleted %d workflow(s)\n", count)
	return nil
}

func runWorkflowValidate(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	formatOutput, _ := cmd.Flags().GetString("format")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	def, result, validationErr := workflows.ValidateDefinition(data)

	if formatOutput == "json" {
		output := map[string]interface{}{
			"valid":    validationErr == nil,
			"errors":   result.Errors,
			"warnings": result.Warnings,
		}
		if def != nil {
			output["workflow_id"] = def.ID
			output["state_count"] = len(def.States)
		}
		jsonBytes, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(jsonBytes))
		if validationErr != nil {
			os.Exit(1)
		}
		return nil
	}

	fmt.Printf("\n📋 Validating: %s\n", filePath)

	if len(result.Errors) > 0 {
		fmt.Printf("\n❌ %d Validation Error(s):\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("   [%s] %s: %s\n", e.Code, e.Path, e.Message)
			if e.Hint != "" {
				fmt.Printf("         💡 %s\n", e.Hint)
			}
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("\n⚠️  %d Warning(s):\n", len(result.Warnings))
		for _, w := range result.Warnings {
			fmt.Printf("   [%s] %s: %s\n", w.Code, w.Path, w.Message)
			if w.Hint != "" {
				fmt.Printf("         💡 %s\n", w.Hint)
			}
		}
	}

	if validationErr != nil {
		fmt.Printf("\n❌ Validation failed with %d error(s)\n", len(result.Errors))
		return fmt.Errorf("validation failed")
	}

	fmt.Printf("\n✅ Workflow is valid!\n")
	if def != nil {
		fmt.Printf("   ID: %s\n", def.ID)
		fmt.Printf("   States: %d\n", len(def.States))
		fmt.Printf("   Start: %s\n", def.Start)
	}

	return nil
}
