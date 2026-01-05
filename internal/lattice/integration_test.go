package lattice

import (
	"context"
	"net"
	"testing"
	"time"

	"station/internal/config"
)

func getFreePortForTest(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

func TestIntegration_FullLatticeFlow(t *testing.T) {
	port := getFreePortForTest(t)
	httpPort := getFreePortForTest(t)

	serverCfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	server := NewEmbeddedServer(serverCfg)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start embedded server: %v", err)
	}
	defer server.Shutdown()

	clientCfg := config.LatticeConfig{
		StationID:   "test-station-1",
		StationName: "Test Station 1",
		NATS: config.LatticeNATSConfig{
			URL: server.ClientURL(),
		},
	}

	client, err := NewClient(clientCfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Fatal("Client should be connected")
	}

	registry := NewRegistry(client)
	ctx := context.Background()

	if err := registry.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	manifest := StationManifest{
		StationID:   "test-station-1",
		StationName: "Test Station 1",
		Agents: []AgentInfo{
			{ID: "1", Name: "test-agent", Description: "Test agent", Capabilities: []string{"testing"}},
			{ID: "2", Name: "k8s-agent", Description: "K8s agent", Capabilities: []string{"k8s.read", "k8s.write"}},
		},
		Workflows: []WorkflowInfo{
			{ID: "wf-1", Name: "test-workflow", Description: "Test workflow"},
		},
	}

	if err := registry.RegisterStation(ctx, manifest); err != nil {
		t.Fatalf("Failed to register station: %v", err)
	}

	retrieved, err := registry.GetStation(ctx, "test-station-1")
	if err != nil {
		t.Fatalf("Failed to get station: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved station should not be nil")
	}

	if retrieved.StationName != "Test Station 1" {
		t.Errorf("StationName = %v, want Test Station 1", retrieved.StationName)
	}

	if len(retrieved.Agents) != 2 {
		t.Errorf("Agents count = %d, want 2", len(retrieved.Agents))
	}

	if len(retrieved.Workflows) != 1 {
		t.Errorf("Workflows count = %d, want 1", len(retrieved.Workflows))
	}

	stations, err := registry.ListStations(ctx)
	if err != nil {
		t.Fatalf("Failed to list stations: %v", err)
	}

	if len(stations) != 1 {
		t.Errorf("Stations count = %d, want 1", len(stations))
	}

	router := NewAgentRouter(registry, "test-station-1")

	agents, err := router.ListAllAgents(ctx)
	if err != nil {
		t.Fatalf("Failed to list agents: %v", err)
	}

	if len(agents) != 2 {
		t.Errorf("Router agents count = %d, want 2", len(agents))
	}

	k8sAgents, err := router.FindAgentByCapability(ctx, "k8s.read")
	if err != nil {
		t.Fatalf("Failed to find agents by capability: %v", err)
	}

	if len(k8sAgents) != 1 {
		t.Errorf("K8s agents count = %d, want 1", len(k8sAgents))
	}

	if k8sAgents[0].AgentName != "k8s-agent" {
		t.Errorf("K8s agent name = %v, want k8s-agent", k8sAgents[0].AgentName)
	}

	workflows, err := router.ListAllWorkflows(ctx)
	if err != nil {
		t.Fatalf("Failed to list workflows: %v", err)
	}

	if len(workflows) != 1 {
		t.Errorf("Workflows count = %d, want 1", len(workflows))
	}

	bestAgent, err := router.FindBestAgent(ctx, "test-agent", "")
	if err != nil {
		t.Fatalf("Failed to find best agent: %v", err)
	}

	if bestAgent == nil {
		t.Fatal("Best agent should not be nil")
	}

	if !bestAgent.IsLocal {
		t.Error("Best agent should be local")
	}

	if err := registry.UnregisterStation(ctx, "test-station-1"); err != nil {
		t.Fatalf("Failed to unregister station: %v", err)
	}

	stations, err = registry.ListStations(ctx)
	if err != nil {
		t.Fatalf("Failed to list stations after unregister: %v", err)
	}

	if len(stations) != 0 {
		t.Errorf("Stations count after unregister = %d, want 0", len(stations))
	}
}

func TestIntegration_MultipleStations(t *testing.T) {
	port := getFreePortForTest(t)
	httpPort := getFreePortForTest(t)

	serverCfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	server := NewEmbeddedServer(serverCfg)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start embedded server: %v", err)
	}
	defer server.Shutdown()

	client1Cfg := config.LatticeConfig{
		StationID:   "station-1",
		StationName: "Station 1",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	client1, err := NewClient(client1Cfg)
	if err != nil {
		t.Fatalf("Failed to create client1: %v", err)
	}
	if err := client1.Connect(); err != nil {
		t.Fatalf("Failed to connect client1: %v", err)
	}
	defer client1.Close()

	client2Cfg := config.LatticeConfig{
		StationID:   "station-2",
		StationName: "Station 2",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	client2, err := NewClient(client2Cfg)
	if err != nil {
		t.Fatalf("Failed to create client2: %v", err)
	}
	if err := client2.Connect(); err != nil {
		t.Fatalf("Failed to connect client2: %v", err)
	}
	defer client2.Close()

	registry := NewRegistry(client1)
	ctx := context.Background()

	if err := registry.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	manifest1 := StationManifest{
		StationID:   "station-1",
		StationName: "Station 1",
		Agents: []AgentInfo{
			{ID: "1", Name: "coder", Description: "Coding agent", Capabilities: []string{"coding"}},
		},
	}

	manifest2 := StationManifest{
		StationID:   "station-2",
		StationName: "Station 2",
		Agents: []AgentInfo{
			{ID: "1", Name: "k8s-admin", Description: "K8s agent", Capabilities: []string{"k8s"}},
		},
	}

	if err := registry.RegisterStation(ctx, manifest1); err != nil {
		t.Fatalf("Failed to register station 1: %v", err)
	}

	if err := registry.RegisterStation(ctx, manifest2); err != nil {
		t.Fatalf("Failed to register station 2: %v", err)
	}

	stations, err := registry.ListStations(ctx)
	if err != nil {
		t.Fatalf("Failed to list stations: %v", err)
	}

	if len(stations) != 2 {
		t.Errorf("Stations count = %d, want 2", len(stations))
	}

	router1 := NewAgentRouter(registry, "station-1")

	coderLocs, err := router1.FindAgentByName(ctx, "coder")
	if err != nil {
		t.Fatalf("Failed to find coder: %v", err)
	}

	if len(coderLocs) != 1 {
		t.Errorf("Coder locations = %d, want 1", len(coderLocs))
	}

	if !coderLocs[0].IsLocal {
		t.Error("Coder should be local to station-1")
	}

	k8sLocs, err := router1.FindAgentByName(ctx, "k8s-admin")
	if err != nil {
		t.Fatalf("Failed to find k8s-admin: %v", err)
	}

	if len(k8sLocs) != 1 {
		t.Errorf("K8s locations = %d, want 1", len(k8sLocs))
	}

	if k8sLocs[0].IsLocal {
		t.Error("K8s-admin should NOT be local to station-1")
	}

	if k8sLocs[0].StationID != "station-2" {
		t.Errorf("K8s-admin station = %v, want station-2", k8sLocs[0].StationID)
	}
}

func TestIntegration_PresenceHeartbeat(t *testing.T) {
	port := getFreePortForTest(t)
	httpPort := getFreePortForTest(t)

	serverCfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	server := NewEmbeddedServer(serverCfg)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start embedded server: %v", err)
	}
	defer server.Shutdown()

	clientCfg := config.LatticeConfig{
		StationID:   "presence-test-station",
		StationName: "Presence Test Station",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	client, err := NewClient(clientCfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	registry := NewRegistry(client)
	ctx := context.Background()

	if err := registry.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	manifest := StationManifest{
		StationID:   "presence-test-station",
		StationName: "Presence Test Station",
		Agents:      []AgentInfo{{ID: "1", Name: "test-agent"}},
	}
	presence := NewPresence(client, registry, manifest, 1)

	presenceCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := presence.Start(presenceCtx); err != nil {
		t.Fatalf("Failed to start presence: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	presence.Stop()
}
