package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/harness"
	"station/pkg/harness/tools"
	"station/pkg/harness/workspace"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
)

// Harness agent templates for scaffolding
var harnessTemplates = map[string]HarnessTemplate{
	"coding": {
		Name:        "coding",
		Description: "Multi-turn coding assistant with file operations",
		MaxSteps:    50,
		Timeout:     "30m",
		Tools:       []string{"read", "write", "edit", "bash", "glob", "grep"},
		SandboxMode: "host",
		Prompt: `You are an expert coding assistant. You help users with software engineering tasks including:
- Writing and modifying code
- Debugging issues
- Refactoring and improving code quality
- Explaining code and concepts

Use the available tools to read, write, and edit files. Always read existing files before making changes.
Be concise and focused on the task at hand.`,
	},
	"sre": {
		Name:        "sre",
		Description: "Site reliability engineering agent for system operations",
		MaxSteps:    30,
		Timeout:     "15m",
		Tools:       []string{"bash", "read", "glob", "grep"},
		SandboxMode: "host",
		Prompt: `You are an SRE (Site Reliability Engineer) assistant. You help with:
- System monitoring and health checks
- Log analysis and troubleshooting
- Infrastructure automation
- Performance optimization

Use bash commands to inspect system state. Be careful with destructive operations.
Always explain what commands you're running and why.`,
	},
	"security": {
		Name:        "security",
		Description: "Security scanning and analysis agent",
		MaxSteps:    40,
		Timeout:     "20m",
		Tools:       []string{"read", "glob", "grep", "bash"},
		SandboxMode: "docker",
		DockerImage: "python:3.11-slim",
		Prompt: `You are a security analyst. You help with:
- Code security review and vulnerability detection
- Configuration security audits
- Dependency scanning
- Security best practices recommendations

Scan code for common vulnerabilities like SQL injection, XSS, and hardcoded secrets.
Provide actionable remediation guidance.`,
	},
	"data": {
		Name:        "data",
		Description: "Data analysis and processing agent",
		MaxSteps:    25,
		Timeout:     "10m",
		Tools:       []string{"read", "write", "bash", "glob"},
		SandboxMode: "docker",
		DockerImage: "python:3.11",
		Prompt: `You are a data analysis assistant. You help with:
- Data exploration and profiling
- Data transformation and cleaning
- Statistical analysis
- Generating reports and visualizations

Use Python in the sandbox for data processing. Be mindful of large datasets.`,
	},
	"minimal": {
		Name:        "minimal",
		Description: "Minimal harness agent template",
		MaxSteps:    10,
		Timeout:     "5m",
		Tools:       []string{"read", "glob"},
		SandboxMode: "host",
		Prompt: `You are a helpful assistant with file access.
Use the available tools to help the user with their request.`,
	},
}

// HarnessTemplate defines a harness agent template
type HarnessTemplate struct {
	Name        string
	Description string
	MaxSteps    int
	Timeout     string
	Tools       []string
	SandboxMode string
	DockerImage string
	Prompt      string
}

var harnessInitCmd = &cobra.Command{
	Use:   "init <agent-name>",
	Short: "Scaffold a new harness agent",
	Long: `Create a new harness agent with a pre-configured template.

Templates available:
  coding   - Multi-turn coding assistant with file operations
  sre      - Site reliability engineering agent
  security - Security scanning and analysis agent
  data     - Data analysis and processing agent
  minimal  - Minimal template for customization

EXAMPLES:
  # Create a coding agent in default environment
  stn harness init my-coder --template coding

  # Create with custom sandbox mode
  stn harness init code-runner --template coding --sandbox docker --image python:3.11

  # Create in specific environment
  stn harness init my-agent --template minimal --env production

  # Interactive mode (prompts for options)
  stn harness init`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHarnessInit,
}

var harnessRunCmd = &cobra.Command{
	Use:   "run <agent-name> <task>",
	Short: "Execute a harness agent with a task",
	Long: `Execute a harness agent with a task and wait for completion.

This is a one-shot execution mode - the agent runs until completion or timeout.
For interactive development, use 'stn harness repl' instead.

EXAMPLES:
  # Run a coding task
  stn harness run my-coder "Fix the bug in main.go"

  # Run with custom workspace
  stn harness run my-coder "Review the code" --workspace ./my-project

  # Run with variables
  stn harness run my-agent "Analyze {{path}}" --var path=./src

  # Stream output in real-time
  stn harness run my-coder "Write tests" --stream

  # Resume from a checkpoint
  stn harness run my-coder --resume session_abc123`,
	Args: cobra.MinimumNArgs(1),
	RunE: runHarnessRun,
}

var harnessInspectCmd = &cobra.Command{
	Use:   "inspect <run-id>",
	Short: "Inspect a harness agent run",
	Long: `Display detailed information about a harness agent run.

Shows execution timeline, tool calls, token usage, and results.

EXAMPLES:
  # Inspect a specific run
  stn harness inspect run_abc123

  # Show verbose output with tool arguments
  stn harness inspect run_abc123 -v`,
	Args: cobra.ExactArgs(1),
	RunE: runHarnessInspect,
}

var harnessListCmd = &cobra.Command{
	Use:   "runs",
	Short: "List harness agent runs",
	Long: `List recent harness agent runs with status and summary.

EXAMPLES:
  # List recent runs
  stn harness runs

  # Filter by agent
  stn harness runs --agent my-coder

  # Limit results
  stn harness runs --limit 10`,
	RunE: runHarnessListRuns,
}

var harnessTemplatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "List available harness agent templates",
	Long:  `Show all available templates for scaffolding harness agents.`,
	RunE:  runHarnessTemplates,
}

func init() {
	// harness init flags
	harnessInitCmd.Flags().StringP("template", "t", "minimal", "Template to use (coding, sre, security, data, minimal)")
	harnessInitCmd.Flags().StringP("env", "e", "default", "Environment to create agent in")
	harnessInitCmd.Flags().String("sandbox", "", "Sandbox mode (host, docker)")
	harnessInitCmd.Flags().String("image", "", "Docker image for sandbox")
	harnessInitCmd.Flags().Int("max-steps", 0, "Maximum execution steps (0 = use template default)")
	harnessInitCmd.Flags().String("timeout", "", "Execution timeout (e.g., 30m, 1h)")
	harnessInitCmd.Flags().StringSlice("tools", nil, "Additional tools to include")
	harnessInitCmd.Flags().Bool("no-sync", false, "Skip syncing environment after creation")

	// harness run flags
	harnessRunCmd.Flags().StringP("env", "e", "default", "Environment name")
	harnessRunCmd.Flags().String("workspace", "", "Workspace directory (default: current directory)")
	harnessRunCmd.Flags().StringToString("var", nil, "Variables for the task (key=value)")
	harnessRunCmd.Flags().Bool("stream", false, "Stream output in real-time")
	harnessRunCmd.Flags().String("resume", "", "Session ID to resume")
	harnessRunCmd.Flags().Int("max-steps", 0, "Override max steps (0 = use agent config)")
	harnessRunCmd.Flags().String("timeout", "", "Override timeout (e.g., 30m)")
	harnessRunCmd.Flags().String("sandbox", "", "Override sandbox mode")
	harnessRunCmd.Flags().Bool("json", false, "Output result as JSON")

	// harness inspect flags
	harnessInspectCmd.Flags().BoolP("verbose", "v", false, "Show detailed tool call arguments")
	harnessInspectCmd.Flags().Bool("json", false, "Output as JSON")

	// harness runs flags
	harnessListCmd.Flags().String("agent", "", "Filter by agent name")
	harnessListCmd.Flags().Int("limit", 20, "Maximum number of runs to show")
	harnessListCmd.Flags().String("status", "", "Filter by status (success, error, running)")

	// Add subcommands to harness command
	harnessCmd.AddCommand(harnessInitCmd)
	harnessCmd.AddCommand(harnessRunCmd)
	harnessCmd.AddCommand(harnessInspectCmd)
	harnessCmd.AddCommand(harnessListCmd)
	harnessCmd.AddCommand(harnessTemplatesCmd)
}

func runHarnessInit(cmd *cobra.Command, args []string) error {
	templateName, _ := cmd.Flags().GetString("template")
	envName, _ := cmd.Flags().GetString("env")
	sandboxMode, _ := cmd.Flags().GetString("sandbox")
	dockerImage, _ := cmd.Flags().GetString("image")
	maxSteps, _ := cmd.Flags().GetInt("max-steps")
	timeout, _ := cmd.Flags().GetString("timeout")
	extraTools, _ := cmd.Flags().GetStringSlice("tools")
	noSync, _ := cmd.Flags().GetBool("no-sync")

	// Get agent name
	var agentName string
	if len(args) > 0 {
		agentName = args[0]
	} else {
		// Interactive mode - prompt for name
		fmt.Print("Agent name: ")
		fmt.Scanln(&agentName)
		if agentName == "" {
			return fmt.Errorf("agent name is required")
		}
	}

	// Get template
	tmpl, ok := harnessTemplates[templateName]
	if !ok {
		return fmt.Errorf("unknown template: %s (available: coding, sre, security, data, minimal)", templateName)
	}

	// Apply overrides
	if sandboxMode != "" {
		tmpl.SandboxMode = sandboxMode
	}
	if dockerImage != "" {
		tmpl.DockerImage = dockerImage
	}
	if maxSteps > 0 {
		tmpl.MaxSteps = maxSteps
	}
	if timeout != "" {
		tmpl.Timeout = timeout
	}
	if len(extraTools) > 0 {
		tmpl.Tools = append(tmpl.Tools, extraTools...)
	}

	// Get environment path
	envPath := config.GetEnvironmentDir(envName)
	agentsDir := filepath.Join(envPath, "agents")

	// Ensure agents directory exists
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}

	// Generate .prompt file
	promptPath := filepath.Join(agentsDir, agentName+".prompt")
	if _, err := os.Stat(promptPath); err == nil {
		return fmt.Errorf("agent already exists: %s", promptPath)
	}

	// Build harness config
	harnessConfig := map[string]interface{}{
		"max_steps": tmpl.MaxSteps,
	}

	// Parse timeout
	if tmpl.Timeout != "" {
		harnessConfig["timeout"] = tmpl.Timeout
	}

	// Build sandbox config
	if tmpl.SandboxMode != "" && tmpl.SandboxMode != "host" {
		sandboxConfig := map[string]interface{}{
			"mode": tmpl.SandboxMode,
		}
		if tmpl.DockerImage != "" {
			sandboxConfig["image"] = tmpl.DockerImage
		}
		harnessConfig["sandbox"] = sandboxConfig
	}

	// Create prompt content
	promptContent := generatePromptFile(agentName, tmpl, harnessConfig)

	// Write prompt file
	if err := os.WriteFile(promptPath, []byte(promptContent), 0644); err != nil {
		return fmt.Errorf("failed to write prompt file: %w", err)
	}

	fmt.Printf("✓ Created harness agent: %s\n", promptPath)
	fmt.Printf("  Template:    %s\n", templateName)
	fmt.Printf("  Max steps:   %d\n", tmpl.MaxSteps)
	fmt.Printf("  Timeout:     %s\n", tmpl.Timeout)
	fmt.Printf("  Sandbox:     %s\n", tmpl.SandboxMode)
	fmt.Printf("  Tools:       %s\n", strings.Join(tmpl.Tools, ", "))

	// Sync environment unless --no-sync
	if !noSync {
		fmt.Println("\nSyncing environment...")
		// We'd call sync here, but for simplicity just print instructions
		fmt.Printf("Run 'stn sync %s' to activate the agent.\n", envName)
	}

	return nil
}

func generatePromptFile(name string, tmpl HarnessTemplate, harnessConfig map[string]interface{}) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString("metadata:\n")
	sb.WriteString(fmt.Sprintf("  name: %q\n", name))
	sb.WriteString(fmt.Sprintf("  description: %q\n", tmpl.Description))
	sb.WriteString("  tags:\n")
	sb.WriteString("    - harness\n")
	sb.WriteString(fmt.Sprintf("    - %s\n", tmpl.Name))
	sb.WriteString("model: gpt-4o-mini\n")
	sb.WriteString(fmt.Sprintf("max_steps: %d\n", tmpl.MaxSteps))

	// Write harness config
	sb.WriteString("harness:\n")
	sb.WriteString(fmt.Sprintf("  max_steps: %d\n", tmpl.MaxSteps))
	if tmpl.Timeout != "" {
		sb.WriteString(fmt.Sprintf("  timeout: %s\n", tmpl.Timeout))
	}
	if tmpl.SandboxMode != "" && tmpl.SandboxMode != "host" {
		sb.WriteString("  sandbox:\n")
		sb.WriteString(fmt.Sprintf("    mode: %s\n", tmpl.SandboxMode))
		if tmpl.DockerImage != "" {
			sb.WriteString(fmt.Sprintf("    image: %s\n", tmpl.DockerImage))
		}
	}

	// Write tools
	sb.WriteString("tools:\n")
	for _, tool := range tmpl.Tools {
		sb.WriteString(fmt.Sprintf("  - %s\n", tool))
	}

	sb.WriteString("---\n\n")
	sb.WriteString("{{role \"system\"}}\n")
	sb.WriteString(tmpl.Prompt)
	sb.WriteString("\n\n")
	sb.WriteString("{{role \"user\"}}\n")
	sb.WriteString("{{userInput}}\n")

	return sb.String()
}

func runHarnessRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	agentName := args[0]
	var task string
	if len(args) > 1 {
		task = strings.Join(args[1:], " ")
	}

	envName, _ := cmd.Flags().GetString("env")
	workspacePath, _ := cmd.Flags().GetString("workspace")
	vars, _ := cmd.Flags().GetStringToString("var")
	stream, _ := cmd.Flags().GetBool("stream")
	resumeSession, _ := cmd.Flags().GetString("resume")
	maxStepsOverride, _ := cmd.Flags().GetInt("max-steps")
	timeoutOverride, _ := cmd.Flags().GetString("timeout")
	outputJSON, _ := cmd.Flags().GetBool("json")

	// Set default workspace to current directory
	if workspacePath == "" {
		var err error
		workspacePath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Apply variable substitution to task
	if len(vars) > 0 {
		tmpl, err := template.New("task").Parse(task)
		if err != nil {
			return fmt.Errorf("invalid task template: %w", err)
		}
		var sb strings.Builder
		if err := tmpl.Execute(&sb, vars); err != nil {
			return fmt.Errorf("failed to apply variables: %w", err)
		}
		task = sb.String()
	}

	if task == "" && resumeSession == "" {
		return fmt.Errorf("task is required (or use --resume to continue a session)")
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)

	// Get environment
	env, err := repos.Environments.GetByName(envName)
	if err != nil {
		return fmt.Errorf("environment '%s' not found: %w", envName, err)
	}

	// Get agent
	agents, err := repos.Agents.ListByEnvironment(env.ID)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	var agent *struct {
		ID       int64
		Name     string
		Prompt   string
		MaxSteps int
	}
	for _, a := range agents {
		if a.Name == agentName {
			agent = &struct {
				ID       int64
				Name     string
				Prompt   string
				MaxSteps int
			}{
				ID:       a.ID,
				Name:     a.Name,
				Prompt:   a.Prompt,
				MaxSteps: int(a.MaxSteps),
			}
			break
		}
	}

	if agent == nil {
		return fmt.Errorf("agent '%s' not found in environment '%s'", agentName, envName)
	}

	// Print header
	if !outputJSON {
		fmt.Println()
		fmt.Printf("╭─ Harness Run ──────────────────────────────────╮\n")
		fmt.Printf("│  Agent:     %-35s │\n", agent.Name)
		fmt.Printf("│  Workspace: %-35s │\n", truncatePath(workspacePath, 35))
		fmt.Printf("│  Env:       %-35s │\n", envName)
		fmt.Printf("╰────────────────────────────────────────────────╯\n")
		fmt.Println()
	}

	// Initialize harness execution
	os.Setenv("OTEL_SDK_DISABLED", "true")
	promptDir := filepath.Join(workspacePath, ".harness", "prompts")
	os.MkdirAll(promptDir, 0755)

	// Get API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY environment variable is required")
	}

	genkitApp := genkit.Init(ctx,
		genkit.WithPlugins(&openai.OpenAI{APIKey: apiKey}),
		genkit.WithPromptDir(promptDir))

	// Create workspace
	ws := workspace.NewHostWorkspace(workspacePath)
	if err := ws.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize workspace: %w", err)
	}

	// Create tool registry
	toolRegistry := tools.NewToolRegistry(genkitApp, workspacePath)
	if err := toolRegistry.RegisterBuiltinTools(); err != nil {
		return fmt.Errorf("failed to register tools: %w", err)
	}

	// Configure harness
	harnessConfig := harness.DefaultHarnessConfig()

	maxSteps := agent.MaxSteps
	if maxStepsOverride > 0 {
		maxSteps = maxStepsOverride
	}
	if maxSteps == 0 {
		maxSteps = 50
	}

	timeout := 30 * time.Minute
	if timeoutOverride != "" {
		if parsed, err := time.ParseDuration(timeoutOverride); err == nil {
			timeout = parsed
		}
	}

	agentConfig := &harness.AgentHarnessConfig{
		MaxSteps:          maxSteps,
		DoomLoopThreshold: maxSteps / 2,
		Timeout:           timeout,
	}

	// Create executor
	executor := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		agentConfig,
		harness.WithWorkspace(ws),
		harness.WithModelName("openai/gpt-4o-mini"),
	)

	// Execute with streaming if requested
	startTime := time.Now()

	if stream && !outputJSON {
		fmt.Printf("Task: %s\n\n", task)
		fmt.Println("─────────────────────────────────────────────────")
	}

	result, err := executor.Execute(ctx, agentName, task, toolRegistry.All())

	duration := time.Since(startTime)

	if outputJSON {
		output := map[string]interface{}{
			"success":       result.Success,
			"response":      result.Response,
			"total_steps":   result.TotalSteps,
			"total_tokens":  result.TotalTokens,
			"finish_reason": result.FinishReason,
			"duration_ms":   duration.Milliseconds(),
		}
		if err != nil {
			output["error"] = err.Error()
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	if err != nil {
		fmt.Printf("\n❌ Execution failed: %v\n", err)
		return err
	}

	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────")
	fmt.Println(result.Response)
	fmt.Println("─────────────────────────────────────────────────")
	fmt.Printf("\n✓ Completed [Steps: %d | Tokens: %d | %s | %s]\n",
		result.TotalSteps, result.TotalTokens, result.FinishReason, duration.Round(time.Millisecond))

	return nil
}

func runHarnessInspect(cmd *cobra.Command, args []string) error {
	runIDStr := args[0]
	verbose, _ := cmd.Flags().GetBool("verbose")
	outputJSON, _ := cmd.Flags().GetBool("json")

	// Parse run ID
	var runID int64
	if _, err := fmt.Sscanf(runIDStr, "%d", &runID); err != nil {
		return fmt.Errorf("invalid run ID: %s", runIDStr)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)

	// Get run details
	run, err := repos.AgentRuns.GetByIDWithDetails(cmd.Context(), runID)
	if err != nil {
		return fmt.Errorf("run not found: %w", err)
	}

	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(run)
	}

	// Pretty print
	fmt.Println()
	fmt.Printf("╭─ Run: %d ─────────────────────────────────────╮\n", run.ID)
	fmt.Printf("│ Agent:    %-37s │\n", run.AgentName)
	fmt.Printf("│ Status:   %-37s │\n", run.Status)
	if run.CompletedAt != nil {
		duration := run.CompletedAt.Sub(run.StartedAt)
		fmt.Printf("│ Duration: %-37s │\n", duration.Round(time.Millisecond))
	}
	fmt.Printf("│ Steps:    %-37d │\n", run.StepsTaken)
	fmt.Printf("╰───────────────────────────────────────────────╯\n")

	if verbose {
		fmt.Println("\nTask:")
		fmt.Printf("  %s\n", run.Task)

		if run.FinalResponse != "" {
			fmt.Println("\nResponse:")
			fmt.Printf("  %s\n", run.FinalResponse)
		}
	}

	fmt.Println()
	return nil
}

func runHarnessListRuns(cmd *cobra.Command, args []string) error {
	agentFilter, _ := cmd.Flags().GetString("agent")
	limit, _ := cmd.Flags().GetInt("limit")
	statusFilter, _ := cmd.Flags().GetString("status")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)

	// Get runs
	runs, err := repos.AgentRuns.ListRecent(cmd.Context(), int64(limit))
	if err != nil {
		return fmt.Errorf("failed to list runs: %w", err)
	}

	fmt.Printf("\n%-8s %-25s %-10s %-6s %-12s\n", "ID", "AGENT", "STATUS", "STEPS", "DURATION")
	fmt.Println(strings.Repeat("-", 70))

	for _, run := range runs {
		// Apply filters
		if statusFilter != "" && run.Status != statusFilter {
			continue
		}
		if agentFilter != "" && run.AgentName != agentFilter {
			continue
		}

		duration := "-"
		if run.CompletedAt != nil {
			d := run.CompletedAt.Sub(run.StartedAt)
			duration = fmt.Sprintf("%.1fs", d.Seconds())
		}

		fmt.Printf("%-8d %-25s %-10s %-6d %-12s\n",
			run.ID,
			shortenStr(run.AgentName, 25),
			run.Status,
			run.StepsTaken,
			duration,
		)
	}

	fmt.Println()
	return nil
}

func runHarnessTemplates(cmd *cobra.Command, args []string) error {
	fmt.Println("\nAvailable Harness Templates:")
	fmt.Println(strings.Repeat("=", 60))

	for name, tmpl := range harnessTemplates {
		fmt.Printf("\n%s\n", name)
		fmt.Println(strings.Repeat("-", len(name)))
		fmt.Printf("  Description: %s\n", tmpl.Description)
		fmt.Printf("  Max Steps:   %d\n", tmpl.MaxSteps)
		fmt.Printf("  Timeout:     %s\n", tmpl.Timeout)
		fmt.Printf("  Sandbox:     %s\n", tmpl.SandboxMode)
		fmt.Printf("  Tools:       %s\n", strings.Join(tmpl.Tools, ", "))
	}

	fmt.Println("\nUsage: stn harness init <name> --template <template>")
	fmt.Println()
	return nil
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

func shortenStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
