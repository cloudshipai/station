package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"station/internal/config"
	"station/internal/lattice"
)

var (
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
  stn lattice agent exec <name>      Execute an agent (local or remote)`,
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

	execOnStation string
)

func init() {
	latticeCmd.AddCommand(latticeStatusCmd)
	latticeCmd.AddCommand(latticeAgentsCmd)
	latticeCmd.AddCommand(latticeAgentCmd)
	latticeAgentCmd.AddCommand(latticeAgentExecCmd)
	latticeAgentExecCmd.Flags().StringVar(&execOnStation, "station", "", "Execute on specific station")
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
