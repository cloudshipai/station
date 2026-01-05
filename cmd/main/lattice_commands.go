package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"station/internal/config"
	"station/internal/lattice"
	"station/internal/lattice/work"
)

var (
	// latticeNatsURL allows CLI commands to connect to a running lattice without being a server
	latticeNatsURL string

	latticeCmd = &cobra.Command{
		Use:   "lattice",
		Short: "Manage Station Lattice (multi-station mesh network)",
		Long: `Station Lattice enables multiple Stations to discover and invoke agents across a mesh network.

Operating Modes:
  stn serve                          Standalone (default) - no lattice connectivity
  stn serve --orchestration          Orchestrator - embedded NATS hub, accepts member connections
  stn serve --lattice nats://host    Member - connects to an orchestrator's NATS

Commands:
  stn lattice status                 Show lattice connection status and discovered stations
  stn lattice agents                 List all agents across the lattice
  stn lattice agent exec <name>      Execute an agent (local or remote)

CLI Client Mode:
  Use --nats flag to connect to a running lattice from CLI without starting a server:
  stn lattice --nats nats://localhost:4222 status
  stn lattice --nats nats://localhost:4222 agents`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// If --nats flag is provided, set it as the lattice URL for client commands
			if latticeNatsURL != "" {
				viper.Set("lattice_url", latticeNatsURL)
			}
		},
	}

	latticeStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show lattice status and discovered stations",
		Long:  "Display the current lattice connection status, mode, and list of discovered stations.",
		RunE:  runLatticeStatus,
	}

	latticeAgentsCmd = &cobra.Command{
		Use:   "agents",
		Short: "List all agents across the lattice",
		Long:  "Display all agents available across all connected stations in the lattice.",
		RunE:  runLatticeAgents,
	}

	latticeAgentCmd = &cobra.Command{
		Use:   "agent",
		Short: "Agent operations",
		Long:  "Execute or manage agents across the lattice.",
	}

	latticeAgentExecCmd = &cobra.Command{
		Use:   "exec <agent-name> <task>",
		Short: "Execute an agent (local or remote)",
		Long: `Execute an agent by name. If the agent exists on a remote station, 
the task will be dispatched to that station for execution.`,
		Args: cobra.MinimumNArgs(2),
		RunE: runLatticeAgentExec,
	}

	latticeWorkflowsCmd = &cobra.Command{
		Use:   "workflows",
		Short: "List all workflows across the lattice",
		Long:  "Display all workflows available across all connected stations in the lattice.",
		RunE:  runLatticeWorkflows,
	}

	latticeWorkflowCmd = &cobra.Command{
		Use:   "workflow",
		Short: "Workflow operations",
		Long:  "Run or manage workflows across the lattice.",
	}

	latticeWorkflowRunCmd = &cobra.Command{
		Use:   "run <workflow-id>",
		Short: "Run a workflow (local or remote)",
		Long: `Run a workflow by ID or name. If the workflow exists on a remote station,
it will be dispatched to that station for execution.`,
		Args: cobra.ExactArgs(1),
		RunE: runLatticeWorkflowRun,
	}

	execOnStation     string
	workflowOnStation string
	workOnStation     string
	workTimeout       string
	agentsDiscover    bool
	agentsCapability  string
	agentsSchemaName  string
)

var (
	latticeDashboardCmd = &cobra.Command{
		Use:   "dashboard",
		Short: "Real-time work dashboard",
		Long:  "Display a real-time terminal dashboard showing active and completed work across the lattice.",
		RunE:  runLatticeDashboard,
	}
)

var (
	latticeWorkCmd = &cobra.Command{
		Use:   "work",
		Short: "Async work operations",
		Long:  "Assign and track async work across the lattice.",
	}

	latticeWorkAssignCmd = &cobra.Command{
		Use:   "assign <agent-name> <task>",
		Short: "Assign work to an agent (async)",
		Long: `Assign work to an agent asynchronously. Returns a work_id immediately.
Use 'stn lattice work await <work_id>' to wait for results.`,
		Args: cobra.MinimumNArgs(2),
		RunE: runLatticeWorkAssign,
	}

	latticeWorkAwaitCmd = &cobra.Command{
		Use:   "await <work_id>",
		Short: "Wait for work to complete",
		Long:  "Block until the specified work completes and return the result.",
		Args:  cobra.ExactArgs(1),
		RunE:  runLatticeWorkAwait,
	}

	latticeWorkCheckCmd = &cobra.Command{
		Use:   "check <work_id>",
		Short: "Check work status (non-blocking)",
		Long:  "Check the status of work without blocking.",
		Args:  cobra.ExactArgs(1),
		RunE:  runLatticeWorkCheck,
	}
)

var (
	latticeRunCmd = &cobra.Command{
		Use:   "run <task>",
		Short: "Submit a task to the lattice",
		Long: `Submit a natural language task to the lattice. An orchestrator agent will
analyze the task, discover available agents, and coordinate their execution.

Examples:
  stn lattice run "Analyze security of my infrastructure"
  stn lattice run "Why is my production pod crashing?"
  stn lattice run --orchestrator SRECoordinator "Check system health"`,
		Args: cobra.MinimumNArgs(1),
		RunE: runLatticeRun,
	}

	runOrchestrator string
	runTimeout      string
	runStream       bool
)

func init() {
	latticeCmd.PersistentFlags().StringVar(&latticeNatsURL, "nats", "", "NATS URL to connect to (e.g., nats://localhost:4222)")

	latticeCmd.AddCommand(latticeStatusCmd)
	latticeCmd.AddCommand(latticeAgentsCmd)
	latticeAgentsCmd.Flags().BoolVar(&agentsDiscover, "discover", false, "Show detailed agent info with schema")
	latticeAgentsCmd.Flags().StringVar(&agentsCapability, "capability", "", "Filter agents by capability")
	latticeAgentsCmd.Flags().StringVar(&agentsSchemaName, "schema", "", "Show full schema for a specific agent")
	latticeCmd.AddCommand(latticeAgentCmd)
	latticeAgentCmd.AddCommand(latticeAgentExecCmd)
	latticeAgentExecCmd.Flags().StringVar(&execOnStation, "station", "", "Execute on specific station")

	latticeCmd.AddCommand(latticeWorkflowsCmd)
	latticeCmd.AddCommand(latticeWorkflowCmd)
	latticeWorkflowCmd.AddCommand(latticeWorkflowRunCmd)
	latticeWorkflowRunCmd.Flags().StringVar(&workflowOnStation, "station", "", "Run on specific station")

	latticeCmd.AddCommand(latticeWorkCmd)
	latticeWorkCmd.AddCommand(latticeWorkAssignCmd)
	latticeWorkCmd.AddCommand(latticeWorkAwaitCmd)
	latticeWorkCmd.AddCommand(latticeWorkCheckCmd)
	latticeWorkAssignCmd.Flags().StringVar(&workOnStation, "station", "", "Assign to specific station")
	latticeWorkAssignCmd.Flags().StringVar(&workTimeout, "timeout", "5m", "Work timeout (e.g., 30s, 5m)")

	latticeCmd.AddCommand(latticeDashboardCmd)

	latticeCmd.AddCommand(latticeRunCmd)
	latticeRunCmd.Flags().StringVar(&runOrchestrator, "orchestrator", "", "Specific orchestrator agent to use (default: auto-detect)")
	latticeRunCmd.Flags().StringVar(&runTimeout, "timeout", "10m", "Overall task timeout (e.g., 5m, 30s)")
	latticeRunCmd.Flags().BoolVar(&runStream, "stream", true, "Stream progress to terminal")
}

func runLatticeStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	orchestrationMode := viper.GetBool("lattice_orchestration")
	latticeURL := viper.GetString("lattice_url")

	if !orchestrationMode && latticeURL == "" {
		fmt.Println("Lattice Status: STANDALONE")
		fmt.Println("")
		fmt.Println("This station is running in standalone mode (no lattice connectivity).")
		fmt.Println("")
		fmt.Println("To enable lattice:")
		fmt.Println("  stn serve --orchestration     Start as orchestrator with embedded NATS")
		fmt.Println("  stn serve --lattice <url>     Connect to an existing orchestrator")
		return nil
	}

	if orchestrationMode {
		fmt.Println("Lattice Status: ORCHESTRATOR")
		fmt.Println("")
		port := cfg.Lattice.Orchestrator.EmbeddedNATS.Port
		if port == 0 {
			port = 4222
		}
		httpPort := cfg.Lattice.Orchestrator.EmbeddedNATS.HTTPPort
		if httpPort == 0 {
			httpPort = 8222
		}
		fmt.Printf("NATS URL:     nats://0.0.0.0:%d\n", port)
		fmt.Printf("Monitoring:   http://0.0.0.0:%d\n", httpPort)
		fmt.Println("")

		client, err := lattice.NewClient(cfg.Lattice)
		if err != nil {
			fmt.Printf("Registry:     Error creating client: %v\n", err)
			return nil
		}

		cfg.Lattice.NATS.URL = fmt.Sprintf("nats://127.0.0.1:%d", port)
		if err := client.Connect(); err != nil {
			fmt.Printf("Registry:     Not connected (run 'stn serve --orchestration' first)\n")
			return nil
		}
		defer client.Close()

		registry := lattice.NewRegistry(client)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := registry.Initialize(ctx); err != nil {
			fmt.Printf("Registry:     Failed to initialize: %v\n", err)
			return nil
		}

		stations, err := registry.ListStations(ctx)
		if err != nil {
			fmt.Printf("Stations:     Error listing: %v\n", err)
			return nil
		}

		fmt.Printf("Stations:     %d connected\n", len(stations))
		if len(stations) > 0 {
			fmt.Println("")
			fmt.Println("Connected Stations:")
			for _, s := range stations {
				fmt.Printf("  - %s (%s) [%s] - %d agents\n",
					s.StationName, s.StationID[:8], s.Status, len(s.Agents))
			}
		}
		return nil
	}

	if latticeURL != "" {
		fmt.Println("Lattice Status: MEMBER")
		fmt.Println("")
		fmt.Printf("Orchestrator: %s\n", latticeURL)

		cfg.Lattice.NATS.URL = latticeURL
		client, err := lattice.NewClient(cfg.Lattice)
		if err != nil {
			fmt.Printf("Connection:   Error: %v\n", err)
			return nil
		}

		if err := client.Connect(); err != nil {
			fmt.Printf("Connection:   Disconnected (error: %v)\n", err)
			return nil
		}
		defer client.Close()

		fmt.Printf("Connection:   Connected\n")
		fmt.Printf("Station ID:   %s\n", client.StationID())
		fmt.Printf("Station Name: %s\n", client.StationName())
	}

	return nil
}

func runLatticeAgents(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	orchestrationMode := viper.GetBool("lattice_orchestration")
	latticeURL := viper.GetString("lattice_url")

	if !orchestrationMode && latticeURL == "" {
		fmt.Println("Error: Not connected to lattice")
		fmt.Println("Start with --orchestration or --lattice <url>")
		return nil
	}

	if orchestrationMode {
		cfg.Lattice.NATS.URL = fmt.Sprintf("nats://127.0.0.1:%d", cfg.Lattice.Orchestrator.EmbeddedNATS.Port)
		if cfg.Lattice.Orchestrator.EmbeddedNATS.Port == 0 {
			cfg.Lattice.NATS.URL = "nats://127.0.0.1:4222"
		}
	} else {
		cfg.Lattice.NATS.URL = latticeURL
	}

	client, err := lattice.NewClient(cfg.Lattice)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	registry := lattice.NewRegistry(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := registry.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize registry: %w", err)
	}

	discovery := lattice.NewAgentDiscovery(registry, nil, client.StationID(), client.StationName())

	if agentsSchemaName != "" {
		schema, err := discovery.GetAgentSchema(ctx, agentsSchemaName)
		if err != nil {
			return fmt.Errorf("failed to get schema: %w", err)
		}

		fmt.Printf("Agent: %s\n", schema.Name)
		fmt.Printf("Location: %s (%s)\n", schema.Location, schema.StationID)
		fmt.Println("--------------------------------------------")
		fmt.Printf("Description: %s\n", schema.Description)
		if schema.InputSchema != "" {
			fmt.Printf("\nInput Schema:\n%s\n", schema.InputSchema)
		}
		if schema.OutputSchema != "" {
			fmt.Printf("\nOutput Schema:\n%s\n", schema.OutputSchema)
		}
		if len(schema.Examples) > 0 {
			fmt.Println("\nExamples:")
			for _, ex := range schema.Examples {
				fmt.Printf("  - %s\n", ex)
			}
		}
		return nil
	}

	if agentsDiscover || agentsCapability != "" {
		agents, err := discovery.ListAvailableAgents(ctx, agentsCapability)
		if err != nil {
			return fmt.Errorf("failed to discover agents: %w", err)
		}

		if len(agents) == 0 {
			if agentsCapability != "" {
				fmt.Printf("No agents found with capability '%s'\n", agentsCapability)
			} else {
				fmt.Println("No agents found in lattice")
			}
			return nil
		}

		fmt.Printf("Discovered Agents (%d total)\n", len(agents))
		if agentsCapability != "" {
			fmt.Printf("Filtered by capability: %s\n", agentsCapability)
		}
		fmt.Println("============================================================")
		fmt.Printf("%-20s %-20s %-30s\n", "AGENT", "LOCATION", "CAPABILITIES")
		fmt.Println("------------------------------------------------------------")
		for _, agent := range agents {
			location := agent.Location
			if agent.IsLocal {
				location = "local (this)"
			}
			if len(location) > 18 {
				location = location[:15] + "..."
			}
			caps := ""
			if len(agent.Capabilities) > 0 {
				for i, cap := range agent.Capabilities {
					if i > 0 {
						caps += ", "
					}
					caps += cap
				}
			}
			if len(caps) > 28 {
				caps = caps[:25] + "..."
			}
			fmt.Printf("%-20s %-20s %-30s\n", agent.Name, location, caps)
		}

		fmt.Println("\nUse --schema <agent-name> for full schema details")
		return nil
	}

	router := lattice.NewAgentRouter(registry, client.StationID())
	agents, err := router.ListAllAgents(ctx)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	if len(agents) == 0 {
		fmt.Println("No agents found in lattice")
		return nil
	}

	fmt.Printf("Agents in Lattice (%d total)\n", len(agents))
	fmt.Println("============================================")
	fmt.Printf("%-20s %-20s %-10s\n", "AGENT", "STATION", "LOCAL")
	fmt.Println("--------------------------------------------")
	for _, agent := range agents {
		localStr := ""
		if agent.IsLocal {
			localStr = "(this)"
		}
		stationName := agent.StationName
		if len(stationName) > 18 {
			stationName = stationName[:15] + "..."
		}
		fmt.Printf("%-20s %-20s %-10s\n", agent.AgentName, stationName, localStr)
	}

	fmt.Println("\nTip: Use --discover for more details or --capability to filter")

	return nil
}

func runLatticeAgentExec(cmd *cobra.Command, args []string) error {
	agentName := args[0]
	task := args[1]
	for i := 2; i < len(args); i++ {
		task += " " + args[i]
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	orchestrationMode := viper.GetBool("lattice_orchestration")
	latticeURL := viper.GetString("lattice_url")

	if !orchestrationMode && latticeURL == "" {
		fmt.Println("Error: Not connected to lattice")
		fmt.Println("Start with --orchestration or --lattice <url>")
		return nil
	}

	if orchestrationMode {
		cfg.Lattice.NATS.URL = fmt.Sprintf("nats://127.0.0.1:%d", cfg.Lattice.Orchestrator.EmbeddedNATS.Port)
		if cfg.Lattice.Orchestrator.EmbeddedNATS.Port == 0 {
			cfg.Lattice.NATS.URL = "nats://127.0.0.1:4222"
		}
	} else {
		cfg.Lattice.NATS.URL = latticeURL
	}

	client, err := lattice.NewClient(cfg.Lattice)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	registry := lattice.NewRegistry(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := registry.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize registry: %w", err)
	}

	router := lattice.NewAgentRouter(registry, client.StationID())

	var targetStation string
	if execOnStation != "" {
		targetStation = execOnStation
	} else {
		location, err := router.FindBestAgent(ctx, agentName, "")
		if err != nil {
			return fmt.Errorf("failed to find agent: %w", err)
		}
		if location == nil {
			return fmt.Errorf("agent '%s' not found in lattice", agentName)
		}
		targetStation = location.StationID
		fmt.Printf("[routing to %s]\n\n", location.StationName)
	}

	invoker := lattice.NewInvoker(client, client.StationID(), nil)
	req := lattice.InvokeAgentRequest{
		AgentName: agentName,
		Task:      task,
	}

	start := time.Now()
	response, err := invoker.InvokeRemoteAgent(ctx, targetStation, req)
	if err != nil {
		return fmt.Errorf("invocation failed: %w", err)
	}

	if response.Status == "error" {
		fmt.Printf("Error: %s\n", response.Error)
		return nil
	}

	fmt.Println(response.Result)
	fmt.Printf("\nExecution completed in %.2fs (via %s)\n",
		time.Since(start).Seconds(), response.StationID)

	return nil
}

func runLatticeWorkflows(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	orchestrationMode := viper.GetBool("lattice_orchestration")
	latticeURL := viper.GetString("lattice_url")

	if !orchestrationMode && latticeURL == "" {
		fmt.Println("Error: Not connected to lattice")
		fmt.Println("Start with --orchestration or --lattice <url>")
		return nil
	}

	if orchestrationMode {
		cfg.Lattice.NATS.URL = fmt.Sprintf("nats://127.0.0.1:%d", cfg.Lattice.Orchestrator.EmbeddedNATS.Port)
		if cfg.Lattice.Orchestrator.EmbeddedNATS.Port == 0 {
			cfg.Lattice.NATS.URL = "nats://127.0.0.1:4222"
		}
	} else {
		cfg.Lattice.NATS.URL = latticeURL
	}

	client, err := lattice.NewClient(cfg.Lattice)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	registry := lattice.NewRegistry(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := registry.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize registry: %w", err)
	}

	router := lattice.NewAgentRouter(registry, client.StationID())
	workflows, err := router.ListAllWorkflows(ctx)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	if len(workflows) == 0 {
		fmt.Println("No workflows found in lattice")
		return nil
	}

	fmt.Printf("Workflows in Lattice (%d total)\n", len(workflows))
	fmt.Println("============================================================")
	fmt.Printf("%-20s %-20s %-30s\n", "WORKFLOW", "STATION", "DESCRIPTION")
	fmt.Println("------------------------------------------------------------")
	for _, wf := range workflows {
		localStr := ""
		if wf.IsLocal {
			localStr = " (this)"
		}
		stationName := wf.StationName + localStr
		if len(stationName) > 18 {
			stationName = stationName[:15] + "..."
		}
		desc := wf.Description
		if len(desc) > 28 {
			desc = desc[:25] + "..."
		}
		fmt.Printf("%-20s %-20s %-30s\n", wf.WorkflowName, stationName, desc)
	}

	return nil
}

func runLatticeWorkflowRun(cmd *cobra.Command, args []string) error {
	workflowID := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	orchestrationMode := viper.GetBool("lattice_orchestration")
	latticeURL := viper.GetString("lattice_url")

	if !orchestrationMode && latticeURL == "" {
		fmt.Println("Error: Not connected to lattice")
		fmt.Println("Start with --orchestration or --lattice <url>")
		return nil
	}

	if orchestrationMode {
		cfg.Lattice.NATS.URL = fmt.Sprintf("nats://127.0.0.1:%d", cfg.Lattice.Orchestrator.EmbeddedNATS.Port)
		if cfg.Lattice.Orchestrator.EmbeddedNATS.Port == 0 {
			cfg.Lattice.NATS.URL = "nats://127.0.0.1:4222"
		}
	} else {
		cfg.Lattice.NATS.URL = latticeURL
	}

	client, err := lattice.NewClient(cfg.Lattice)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	registry := lattice.NewRegistry(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := registry.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize registry: %w", err)
	}

	router := lattice.NewAgentRouter(registry, client.StationID())

	var targetStation string
	if workflowOnStation != "" {
		targetStation = workflowOnStation
	} else {
		location, err := router.FindBestWorkflow(ctx, workflowID)
		if err != nil {
			return fmt.Errorf("failed to find workflow: %w", err)
		}
		if location == nil {
			return fmt.Errorf("workflow '%s' not found in lattice", workflowID)
		}
		targetStation = location.StationID
		fmt.Printf("[routing to %s]\n\n", location.StationName)
	}

	invoker := lattice.NewInvoker(client, client.StationID(), nil)
	req := lattice.RunWorkflowRequest{
		WorkflowID: workflowID,
	}

	start := time.Now()
	response, err := invoker.InvokeRemoteWorkflow(ctx, targetStation, req)
	if err != nil {
		return fmt.Errorf("workflow invocation failed: %w", err)
	}

	if response.Status == "error" {
		fmt.Printf("Error: %s\n", response.Error)
		return nil
	}

	fmt.Printf("Run ID: %s\n", response.RunID)
	if response.Result != "" {
		fmt.Println(response.Result)
	}
	fmt.Printf("\nWorkflow completed in %.2fs (via %s)\n",
		time.Since(start).Seconds(), response.StationID)

	return nil
}

func runLatticeWorkAssign(cmd *cobra.Command, args []string) error {
	agentName := args[0]
	task := args[1]
	for i := 2; i < len(args); i++ {
		task += " " + args[i]
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	orchestrationMode := viper.GetBool("lattice_orchestration")
	latticeURL := viper.GetString("lattice_url")

	if !orchestrationMode && latticeURL == "" {
		fmt.Println("Error: Not connected to lattice")
		fmt.Println("Start with --orchestration or --lattice <url>")
		return nil
	}

	if orchestrationMode {
		cfg.Lattice.NATS.URL = fmt.Sprintf("nats://127.0.0.1:%d", cfg.Lattice.Orchestrator.EmbeddedNATS.Port)
		if cfg.Lattice.Orchestrator.EmbeddedNATS.Port == 0 {
			cfg.Lattice.NATS.URL = "nats://127.0.0.1:4222"
		}
	} else {
		cfg.Lattice.NATS.URL = latticeURL
	}

	client, err := lattice.NewClient(cfg.Lattice)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	timeout, err := time.ParseDuration(workTimeout)
	if err != nil {
		timeout = 5 * time.Minute
	}

	dispatcher := work.NewDispatcher(client, client.StationID())
	ctx := context.Background()
	if err := dispatcher.Start(ctx); err != nil {
		return fmt.Errorf("failed to start dispatcher: %w", err)
	}
	defer dispatcher.Stop()

	assignment := &work.WorkAssignment{
		TargetStation: workOnStation,
		AgentName:     agentName,
		Task:          task,
		Timeout:       timeout,
	}

	workID, err := dispatcher.AssignWork(ctx, assignment)
	if err != nil {
		return fmt.Errorf("failed to assign work: %w", err)
	}

	fmt.Printf("Work assigned: %s\n", workID)
	fmt.Printf("Agent: %s\n", agentName)
	if workOnStation != "" {
		fmt.Printf("Station: %s\n", workOnStation)
	}
	fmt.Printf("\nUse 'stn lattice work await %s' to wait for results\n", workID)

	return nil
}

func runLatticeWorkAwait(cmd *cobra.Command, args []string) error {
	workID := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	orchestrationMode := viper.GetBool("lattice_orchestration")
	latticeURL := viper.GetString("lattice_url")

	if !orchestrationMode && latticeURL == "" {
		fmt.Println("Error: Not connected to lattice")
		return nil
	}

	if orchestrationMode {
		cfg.Lattice.NATS.URL = fmt.Sprintf("nats://127.0.0.1:%d", cfg.Lattice.Orchestrator.EmbeddedNATS.Port)
		if cfg.Lattice.Orchestrator.EmbeddedNATS.Port == 0 {
			cfg.Lattice.NATS.URL = "nats://127.0.0.1:4222"
		}
	} else {
		cfg.Lattice.NATS.URL = latticeURL
	}

	client, err := lattice.NewClient(cfg.Lattice)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	dispatcher := work.NewDispatcher(client, client.StationID())
	ctx := context.Background()
	if err := dispatcher.Start(ctx); err != nil {
		return fmt.Errorf("failed to start dispatcher: %w", err)
	}
	defer dispatcher.Stop()

	fmt.Printf("Waiting for work %s...\n", workID)

	result, err := dispatcher.AwaitWork(ctx, workID)
	if err != nil {
		return fmt.Errorf("failed to await work: %w", err)
	}

	fmt.Printf("\nStatus: %s\n", result.Type)
	if result.Result != "" {
		fmt.Printf("Result: %s\n", result.Result)
	}
	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
	}
	fmt.Printf("Station: %s\n", result.StationID)
	if result.DurationMs > 0 {
		fmt.Printf("Duration: %.2fs\n", result.DurationMs/1000)
	}

	return nil
}

func runLatticeWorkCheck(cmd *cobra.Command, args []string) error {
	workID := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	orchestrationMode := viper.GetBool("lattice_orchestration")
	latticeURL := viper.GetString("lattice_url")

	if !orchestrationMode && latticeURL == "" {
		fmt.Println("Error: Not connected to lattice")
		return nil
	}

	if orchestrationMode {
		cfg.Lattice.NATS.URL = fmt.Sprintf("nats://127.0.0.1:%d", cfg.Lattice.Orchestrator.EmbeddedNATS.Port)
		if cfg.Lattice.Orchestrator.EmbeddedNATS.Port == 0 {
			cfg.Lattice.NATS.URL = "nats://127.0.0.1:4222"
		}
	} else {
		cfg.Lattice.NATS.URL = latticeURL
	}

	client, err := lattice.NewClient(cfg.Lattice)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	dispatcher := work.NewDispatcher(client, client.StationID())
	ctx := context.Background()
	if err := dispatcher.Start(ctx); err != nil {
		return fmt.Errorf("failed to start dispatcher: %w", err)
	}
	defer dispatcher.Stop()

	status, err := dispatcher.CheckWork(workID)
	if err != nil {
		return fmt.Errorf("failed to check work: %w", err)
	}

	fmt.Printf("Work ID: %s\n", status.WorkID)
	fmt.Printf("Status: %s\n", status.Status)

	return nil
}

func runLatticeDashboard(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	orchestrationMode := viper.GetBool("lattice_orchestration")
	latticeURL := viper.GetString("lattice_url")

	if !orchestrationMode && latticeURL == "" {
		fmt.Println("Error: Not connected to lattice")
		fmt.Println("Start with --orchestration or --lattice <url>")
		return nil
	}

	if orchestrationMode {
		cfg.Lattice.NATS.URL = fmt.Sprintf("nats://127.0.0.1:%d", cfg.Lattice.Orchestrator.EmbeddedNATS.Port)
		if cfg.Lattice.Orchestrator.EmbeddedNATS.Port == 0 {
			cfg.Lattice.NATS.URL = "nats://127.0.0.1:4222"
		}
	} else {
		cfg.Lattice.NATS.URL = latticeURL
	}

	client, err := lattice.NewClient(cfg.Lattice)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	js := client.JetStream()
	if js == nil {
		return fmt.Errorf("JetStream not available (lattice must be in orchestration mode)")
	}

	store, err := work.NewWorkStore(js, work.DefaultWorkStoreConfig())
	if err != nil {
		return fmt.Errorf("failed to create work store: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dashboard := work.NewDashboard(store, client.StationID())
	return dashboard.Run(ctx)
}

func runLatticeRun(cmd *cobra.Command, args []string) error {
	task := args[0]
	for i := 1; i < len(args); i++ {
		task += " " + args[i]
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	orchestrationMode := viper.GetBool("lattice_orchestration")
	latticeURL := viper.GetString("lattice_url")

	if !orchestrationMode && latticeURL == "" {
		fmt.Println("Error: Not connected to lattice")
		fmt.Println("Start with --orchestration or --lattice <url>")
		return nil
	}

	if orchestrationMode {
		cfg.Lattice.NATS.URL = fmt.Sprintf("nats://127.0.0.1:%d", cfg.Lattice.Orchestrator.EmbeddedNATS.Port)
		if cfg.Lattice.Orchestrator.EmbeddedNATS.Port == 0 {
			cfg.Lattice.NATS.URL = "nats://127.0.0.1:4222"
		}
	} else {
		cfg.Lattice.NATS.URL = latticeURL
	}

	client, err := lattice.NewClient(cfg.Lattice)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	registry := lattice.NewRegistry(client)
	ctx := context.Background()

	timeout, err := time.ParseDuration(runTimeout)
	if err != nil {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := registry.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize registry: %w", err)
	}

	orchestratorAgent, orchestratorStation, err := findOrchestratorAgent(ctx, registry, runOrchestrator)
	if err != nil {
		return fmt.Errorf("failed to find orchestrator agent: %w", err)
	}

	fmt.Printf("🚀 Submitting task to lattice\n")
	fmt.Printf("   Orchestrator: %s", orchestratorAgent)
	if orchestratorStation != "" && orchestratorStation != client.StationID() {
		fmt.Printf(" (on %s)", orchestratorStation)
	}
	fmt.Printf("\n")
	fmt.Printf("   Task: %s\n", task)
	fmt.Printf("   Timeout: %s\n\n", timeout)

	dispatcher := work.NewDispatcher(client, client.StationID())
	if err := dispatcher.Start(ctx); err != nil {
		return fmt.Errorf("failed to start dispatcher: %w", err)
	}
	defer dispatcher.Stop()

	time.Sleep(100 * time.Millisecond)

	rootContext := work.NewRootContext(client.StationID(), "")
	assignment := rootContext.ToWorkAssignment(orchestratorAgent, task, orchestratorStation, timeout.Milliseconds())
	assignment.Timeout = timeout

	workID, err := dispatcher.AssignWork(ctx, assignment)
	if err != nil {
		return fmt.Errorf("failed to assign work: %w", err)
	}

	fmt.Printf("📋 Work ID: %s\n", workID)
	fmt.Printf("⏳ Waiting for orchestrator to complete...\n\n")

	if runStream {
		progressChan, err := dispatcher.StreamProgress(workID)
		if err == nil {
			go func() {
				for update := range progressChan {
					if update.Type == work.MsgWorkProgress && update.ProgressMsg != "" {
						fmt.Printf("   [progress] %s\n", update.ProgressMsg)
					} else if update.Type == work.MsgWorkAccepted {
						fmt.Printf("   [accepted] Work picked up by %s\n", update.StationID)
					}
				}
			}()
		}
	}

	result, err := dispatcher.AwaitWork(ctx, workID)
	if err != nil {
		return fmt.Errorf("work failed: %w", err)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if result.Type == work.MsgWorkComplete {
		fmt.Printf("✅ Task completed successfully\n\n")
		fmt.Println(result.Result)
	} else if result.Type == work.MsgWorkFailed {
		fmt.Printf("❌ Task failed: %s\n", result.Error)
	} else if result.Type == work.MsgWorkEscalate {
		fmt.Printf("⚠️  Orchestrator escalated: %s\n", result.EscalationReason)
	}

	fmt.Println()
	fmt.Printf("Duration: %.2fs\n", result.DurationMs/1000)
	if result.ToolCalls > 0 {
		fmt.Printf("Tool calls: %d\n", result.ToolCalls)
	}

	return nil
}

func findOrchestratorAgent(ctx context.Context, registry *lattice.Registry, specified string) (string, string, error) {
	if specified != "" {
		router := lattice.NewAgentRouter(registry, "")
		location, err := router.FindBestAgent(ctx, specified, "")
		if err != nil {
			return "", "", fmt.Errorf("specified orchestrator '%s' not found: %w", specified, err)
		}
		if location != nil {
			return specified, location.StationID, nil
		}
		return specified, "", nil
	}

	stations, err := registry.ListStations(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to list stations: %w", err)
	}

	orchestratorKeywords := []string{"orchestrator", "coordinator", "manager", "conductor", "dispatcher"}

	for _, station := range stations {
		for _, agent := range station.Agents {
			agentNameLower := toLowerSimple(agent.Name)
			for _, keyword := range orchestratorKeywords {
				if containsSimple(agentNameLower, keyword) {
					return agent.Name, station.StationID, nil
				}
			}
		}
	}

	for _, station := range stations {
		for _, agent := range station.Agents {
			descLower := toLowerSimple(agent.Description)
			for _, keyword := range orchestratorKeywords {
				if containsSimple(descLower, keyword) {
					return agent.Name, station.StationID, nil
				}
			}
		}
	}

	for _, station := range stations {
		if len(station.Agents) > 0 {
			return station.Agents[0].Name, station.StationID, nil
		}
	}

	return "", "", fmt.Errorf("no agents found in lattice")
}

func toLowerSimple(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func containsSimple(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
