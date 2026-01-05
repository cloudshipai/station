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
  stn lattice status                 Show lattice connection status and discovered stations`,
	}

	latticeStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show lattice status and discovered stations",
		Long:  "Display the current lattice connection status, mode, and list of discovered stations.",
		RunE:  runLatticeStatus,
	}
)

func init() {
	latticeCmd.AddCommand(latticeStatusCmd)
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
