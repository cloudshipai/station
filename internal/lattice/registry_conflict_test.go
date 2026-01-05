package lattice

import (
	"context"
	"testing"
	"time"

	"station/internal/config"
)

func setupTestRegistryWithServer(t *testing.T) (*Registry, *EmbeddedServer, func()) {
	t.Helper()

	port := getFreePort(t)
	httpPort := getFreePort(t)

	serverCfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	server := NewEmbeddedServer(serverCfg)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start embedded server: %v", err)
	}

	latticeCfg := config.LatticeConfig{
		StationID: "test-station",
		NATS: config.LatticeNATSConfig{
			URL: server.ClientURL(),
		},
	}

	client, err := NewClient(latticeCfg)
	if err != nil {
		server.Shutdown()
		t.Fatalf("Failed to create client: %v", err)
	}

	if err := client.Connect(); err != nil {
		server.Shutdown()
		t.Fatalf("Failed to connect client: %v", err)
	}

	registry := NewRegistry(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := registry.Initialize(ctx); err != nil {
		client.Close()
		server.Shutdown()
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	cleanup := func() {
		client.Close()
		server.Shutdown()
	}

	return registry, server, cleanup
}

func TestRegisterStationWithConflictCheck_DetectsConflicts(t *testing.T) {
	registry, _, cleanup := setupTestRegistryWithServer(t)
	defer cleanup()

	ctx := context.Background()

	// Register first station with agent "SecurityScanner"
	station1 := StationManifest{
		StationID:   "station-1",
		StationName: "security-station",
		Agents: []AgentInfo{
			{ID: "1", Name: "SecurityScanner", Description: "Scans for vulnerabilities"},
		},
	}
	if err := registry.RegisterStation(ctx, station1); err != nil {
		t.Fatalf("Failed to register station1: %v", err)
	}

	// Try to register second station with same agent name
	station2 := StationManifest{
		StationID:   "station-2",
		StationName: "ops-station",
		Agents: []AgentInfo{
			{ID: "1", Name: "SecurityScanner", Description: "Different scanner"},
			{ID: "2", Name: "Deployer", Description: "Deploys apps"},
		},
	}

	result, err := registry.RegisterStationWithConflictCheck(ctx, station2)
	if err != nil {
		t.Fatalf("RegisterStationWithConflictCheck failed: %v", err)
	}

	// Should detect conflict for SecurityScanner
	if len(result.ConflictingAgents) != 1 {
		t.Errorf("Expected 1 conflict, got %d", len(result.ConflictingAgents))
	}

	if result.ConflictingAgents[0].AgentName != "SecurityScanner" {
		t.Errorf("Expected conflict for 'SecurityScanner', got '%s'", result.ConflictingAgents[0].AgentName)
	}

	if result.ConflictingAgents[0].ExistingStation != "security-station" {
		t.Errorf("Expected existing station 'security-station', got '%s'", result.ConflictingAgents[0].ExistingStation)
	}
}

func TestRegisterStationWithConflictCheck_AllowsNonConflictingAgents(t *testing.T) {
	registry, _, cleanup := setupTestRegistryWithServer(t)
	defer cleanup()

	ctx := context.Background()

	// Register first station
	station1 := StationManifest{
		StationID:   "station-1",
		StationName: "security-station",
		Agents: []AgentInfo{
			{ID: "1", Name: "SecurityScanner", Description: "Scans for vulnerabilities"},
		},
	}
	if err := registry.RegisterStation(ctx, station1); err != nil {
		t.Fatalf("Failed to register station1: %v", err)
	}

	// Register second station with different agents
	station2 := StationManifest{
		StationID:   "station-2",
		StationName: "ops-station",
		Agents: []AgentInfo{
			{ID: "1", Name: "Deployer", Description: "Deploys apps"},
			{ID: "2", Name: "Monitor", Description: "Monitors services"},
		},
	}

	result, err := registry.RegisterStationWithConflictCheck(ctx, station2)
	if err != nil {
		t.Fatalf("RegisterStationWithConflictCheck failed: %v", err)
	}

	// Should have no conflicts
	if len(result.ConflictingAgents) != 0 {
		t.Errorf("Expected 0 conflicts, got %d", len(result.ConflictingAgents))
	}

	// Should have registered both agents
	if len(result.RegisteredAgents) != 2 {
		t.Errorf("Expected 2 registered agents, got %d", len(result.RegisteredAgents))
	}
}

func TestRegisterStationWithConflictCheck_ExcludesConflictingAgentsFromManifest(t *testing.T) {
	registry, _, cleanup := setupTestRegistryWithServer(t)
	defer cleanup()

	ctx := context.Background()

	// Register first station
	station1 := StationManifest{
		StationID:   "station-1",
		StationName: "security-station",
		Agents: []AgentInfo{
			{ID: "1", Name: "SecurityScanner", Description: "Scans for vulnerabilities"},
		},
	}
	if err := registry.RegisterStation(ctx, station1); err != nil {
		t.Fatalf("Failed to register station1: %v", err)
	}

	// Register second station with one conflicting and one unique agent
	station2 := StationManifest{
		StationID:   "station-2",
		StationName: "ops-station",
		Agents: []AgentInfo{
			{ID: "1", Name: "SecurityScanner", Description: "Different scanner"}, // conflict
			{ID: "2", Name: "Deployer", Description: "Deploys apps"},             // unique
		},
	}

	result, err := registry.RegisterStationWithConflictCheck(ctx, station2)
	if err != nil {
		t.Fatalf("RegisterStationWithConflictCheck failed: %v", err)
	}

	// Should have registered only Deployer
	if len(result.RegisteredAgents) != 1 {
		t.Errorf("Expected 1 registered agent, got %d", len(result.RegisteredAgents))
	}
	if result.RegisteredAgents[0] != "Deployer" {
		t.Errorf("Expected registered agent 'Deployer', got '%s'", result.RegisteredAgents[0])
	}

	// Verify the stored manifest only has Deployer
	storedStation, err := registry.GetStation(ctx, "station-2")
	if err != nil {
		t.Fatalf("Failed to get station: %v", err)
	}
	if len(storedStation.Agents) != 1 {
		t.Errorf("Expected stored manifest to have 1 agent, got %d", len(storedStation.Agents))
	}
	if storedStation.Agents[0].Name != "Deployer" {
		t.Errorf("Expected stored agent 'Deployer', got '%s'", storedStation.Agents[0].Name)
	}
}

func TestFindAgentNameConflicts_ReturnsEmptyForFirstStation(t *testing.T) {
	registry, _, cleanup := setupTestRegistryWithServer(t)
	defer cleanup()

	ctx := context.Background()

	// First station should have no conflicts
	station1 := StationManifest{
		StationID:   "station-1",
		StationName: "first-station",
		Agents: []AgentInfo{
			{ID: "1", Name: "Agent1", Description: "First agent"},
			{ID: "2", Name: "Agent2", Description: "Second agent"},
		},
	}

	result, err := registry.RegisterStationWithConflictCheck(ctx, station1)
	if err != nil {
		t.Fatalf("RegisterStationWithConflictCheck failed: %v", err)
	}

	if len(result.ConflictingAgents) != 0 {
		t.Errorf("Expected 0 conflicts for first station, got %d", len(result.ConflictingAgents))
	}

	if len(result.RegisteredAgents) != 2 {
		t.Errorf("Expected 2 registered agents, got %d", len(result.RegisteredAgents))
	}
}

func TestFindAgentNameConflicts_DetectsDuplicateNamesAcrossStations(t *testing.T) {
	registry, _, cleanup := setupTestRegistryWithServer(t)
	defer cleanup()

	ctx := context.Background()

	// Register first station
	station1 := StationManifest{
		StationID:   "station-1",
		StationName: "station-alpha",
		Agents: []AgentInfo{
			{ID: "1", Name: "SharedAgent", Description: "Shared agent on alpha"},
			{ID: "2", Name: "UniqueAlpha", Description: "Unique to alpha"},
		},
	}
	if err := registry.RegisterStation(ctx, station1); err != nil {
		t.Fatalf("Failed to register station1: %v", err)
	}

	// Register second station
	station2 := StationManifest{
		StationID:   "station-2",
		StationName: "station-beta",
		Agents: []AgentInfo{
			{ID: "1", Name: "SharedAgent", Description: "Shared agent on beta"}, // duplicate
			{ID: "2", Name: "UniqueBeta", Description: "Unique to beta"},
		},
	}

	result, err := registry.RegisterStationWithConflictCheck(ctx, station2)
	if err != nil {
		t.Fatalf("RegisterStationWithConflictCheck failed: %v", err)
	}

	// Should detect SharedAgent conflict
	if len(result.ConflictingAgents) != 1 {
		t.Errorf("Expected 1 conflict, got %d", len(result.ConflictingAgents))
	}

	// Should register UniqueBeta
	if len(result.RegisteredAgents) != 1 {
		t.Errorf("Expected 1 registered agent, got %d", len(result.RegisteredAgents))
	}
	if result.RegisteredAgents[0] != "UniqueBeta" {
		t.Errorf("Expected 'UniqueBeta' to be registered, got '%s'", result.RegisteredAgents[0])
	}
}

func TestCheckAgentNameAvailable_ReturnsTrueForUniqueNames(t *testing.T) {
	registry, _, cleanup := setupTestRegistryWithServer(t)
	defer cleanup()

	ctx := context.Background()

	// Register a station
	station1 := StationManifest{
		StationID:   "station-1",
		StationName: "test-station",
		Agents: []AgentInfo{
			{ID: "1", Name: "ExistingAgent", Description: "Already exists"},
		},
	}
	if err := registry.RegisterStation(ctx, station1); err != nil {
		t.Fatalf("Failed to register station1: %v", err)
	}

	// Check for a unique name
	available, existingStation := registry.CheckAgentNameAvailable(ctx, "NewAgent", "")
	if !available {
		t.Error("Expected 'NewAgent' to be available")
	}
	if existingStation != nil {
		t.Error("Expected existingStation to be nil for unique name")
	}
}

func TestCheckAgentNameAvailable_ReturnsFalseAndStationForDuplicates(t *testing.T) {
	registry, _, cleanup := setupTestRegistryWithServer(t)
	defer cleanup()

	ctx := context.Background()

	// Register a station
	station1 := StationManifest{
		StationID:   "station-1",
		StationName: "test-station",
		Agents: []AgentInfo{
			{ID: "1", Name: "ExistingAgent", Description: "Already exists"},
		},
	}
	if err := registry.RegisterStation(ctx, station1); err != nil {
		t.Fatalf("Failed to register station1: %v", err)
	}

	// Check for a duplicate name
	available, existingStation := registry.CheckAgentNameAvailable(ctx, "ExistingAgent", "")
	if available {
		t.Error("Expected 'ExistingAgent' to NOT be available")
	}
	if existingStation == nil {
		t.Fatal("Expected existingStation to be non-nil for duplicate name")
	}
	if existingStation.StationName != "test-station" {
		t.Errorf("Expected existing station 'test-station', got '%s'", existingStation.StationName)
	}
}

func TestCheckAgentNameAvailable_ExcludesOwnStation(t *testing.T) {
	registry, _, cleanup := setupTestRegistryWithServer(t)
	defer cleanup()

	ctx := context.Background()

	// Register a station
	station1 := StationManifest{
		StationID:   "station-1",
		StationName: "test-station",
		Agents: []AgentInfo{
			{ID: "1", Name: "MyAgent", Description: "My agent"},
		},
	}
	if err := registry.RegisterStation(ctx, station1); err != nil {
		t.Fatalf("Failed to register station1: %v", err)
	}

	// Check for own agent name - should be available when excluding own station
	available, existingStation := registry.CheckAgentNameAvailable(ctx, "MyAgent", "station-1")
	if !available {
		t.Error("Expected 'MyAgent' to be available when excluding own station")
	}
	if existingStation != nil {
		t.Error("Expected existingStation to be nil when excluding own station")
	}
}

func TestAgentNameConflictError_ErrorMessage(t *testing.T) {
	err := &AgentNameConflictError{
		AgentName:        "TestAgent",
		ExistingStation:  "existing-station",
		AttemptedStation: "new-station",
	}

	expected := "agent name 'TestAgent' already registered by station 'existing-station', cannot register from 'new-station'"
	if err.Error() != expected {
		t.Errorf("Error message mismatch:\ngot:  %s\nwant: %s", err.Error(), expected)
	}
}

func TestRegistrationResult_SuccessWithNoConflicts(t *testing.T) {
	registry, _, cleanup := setupTestRegistryWithServer(t)
	defer cleanup()

	ctx := context.Background()

	station := StationManifest{
		StationID:   "station-1",
		StationName: "test-station",
		Agents: []AgentInfo{
			{ID: "1", Name: "Agent1", Description: "First"},
			{ID: "2", Name: "Agent2", Description: "Second"},
		},
	}

	result, err := registry.RegisterStationWithConflictCheck(ctx, station)
	if err != nil {
		t.Fatalf("RegisterStationWithConflictCheck failed: %v", err)
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}
	if len(result.RegisteredAgents) != 2 {
		t.Errorf("Expected 2 registered agents, got %d", len(result.RegisteredAgents))
	}
	if len(result.ConflictingAgents) != 0 {
		t.Errorf("Expected 0 conflicting agents, got %d", len(result.ConflictingAgents))
	}
}

func TestRegisterStationWithConflictCheck_MultipleConflicts(t *testing.T) {
	registry, _, cleanup := setupTestRegistryWithServer(t)
	defer cleanup()

	ctx := context.Background()

	// Register first station with multiple agents
	station1 := StationManifest{
		StationID:   "station-1",
		StationName: "primary-station",
		Agents: []AgentInfo{
			{ID: "1", Name: "AgentA", Description: "Agent A"},
			{ID: "2", Name: "AgentB", Description: "Agent B"},
			{ID: "3", Name: "AgentC", Description: "Agent C"},
		},
	}
	if err := registry.RegisterStation(ctx, station1); err != nil {
		t.Fatalf("Failed to register station1: %v", err)
	}

	// Try to register second station with multiple conflicts
	station2 := StationManifest{
		StationID:   "station-2",
		StationName: "secondary-station",
		Agents: []AgentInfo{
			{ID: "1", Name: "AgentA", Description: "Duplicate A"}, // conflict
			{ID: "2", Name: "AgentB", Description: "Duplicate B"}, // conflict
			{ID: "3", Name: "AgentD", Description: "Unique D"},    // unique
		},
	}

	result, err := registry.RegisterStationWithConflictCheck(ctx, station2)
	if err != nil {
		t.Fatalf("RegisterStationWithConflictCheck failed: %v", err)
	}

	// Should have 2 conflicts
	if len(result.ConflictingAgents) != 2 {
		t.Errorf("Expected 2 conflicts, got %d", len(result.ConflictingAgents))
	}

	// Should have registered 1 agent
	if len(result.RegisteredAgents) != 1 {
		t.Errorf("Expected 1 registered agent, got %d", len(result.RegisteredAgents))
	}
	if result.RegisteredAgents[0] != "AgentD" {
		t.Errorf("Expected 'AgentD' to be registered, got '%s'", result.RegisteredAgents[0])
	}
}
