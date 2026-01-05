package lattice

import (
	"context"
	"testing"

	"station/internal/config"
)

type MockLocalCollector struct {
	agents []AgentInfo
	err    error
}

func (m *MockLocalCollector) CollectAgents(ctx context.Context) ([]AgentInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.agents, nil
}

func TestAgentDiscovery_ListAvailableAgents_LocalOnly(t *testing.T) {
	localAgents := []AgentInfo{
		{
			ID:           "1",
			Name:         "coder",
			Description:  "A coding agent",
			Capabilities: []string{"coding", "debugging"},
			InputSchema:  `{"type": "string"}`,
			OutputSchema: `{"type": "string"}`,
		},
		{
			ID:           "2",
			Name:         "reviewer",
			Description:  "A code review agent",
			Capabilities: []string{"review", "security"},
		},
	}

	mockCollector := &MockLocalCollector{agents: localAgents}
	discovery := NewAgentDiscoveryWithMock(nil, mockCollector, "local-station", "Local Station")

	ctx := context.Background()
	agents, err := discovery.ListAvailableAgents(ctx, "")
	if err != nil {
		t.Fatalf("ListAvailableAgents() error = %v", err)
	}

	if len(agents) != 2 {
		t.Fatalf("ListAvailableAgents() returned %d agents, want 2", len(agents))
	}

	if agents[0].Name != "coder" {
		t.Errorf("agents[0].Name = %v, want coder", agents[0].Name)
	}
	if !agents[0].IsLocal {
		t.Error("agents[0].IsLocal = false, want true")
	}
	if agents[0].Location != "local" {
		t.Errorf("agents[0].Location = %v, want local", agents[0].Location)
	}
	if agents[0].InputSchema != `{"type": "string"}` {
		t.Errorf("agents[0].InputSchema = %v, want schema", agents[0].InputSchema)
	}
}

func TestAgentDiscovery_ListAvailableAgents_FilterByCapability(t *testing.T) {
	localAgents := []AgentInfo{
		{
			ID:           "1",
			Name:         "coder",
			Description:  "A coding agent",
			Capabilities: []string{"coding", "debugging"},
		},
		{
			ID:           "2",
			Name:         "security-scanner",
			Description:  "A security scanner",
			Capabilities: []string{"security", "audit"},
		},
		{
			ID:           "3",
			Name:         "reviewer",
			Description:  "A code review agent",
			Capabilities: []string{"review", "security"},
		},
	}

	mockCollector := &MockLocalCollector{agents: localAgents}
	discovery := NewAgentDiscoveryWithMock(nil, mockCollector, "local-station", "Local Station")

	ctx := context.Background()
	agents, err := discovery.ListAvailableAgents(ctx, "security")
	if err != nil {
		t.Fatalf("ListAvailableAgents(security) error = %v", err)
	}

	if len(agents) != 2 {
		t.Fatalf("ListAvailableAgents(security) returned %d agents, want 2", len(agents))
	}

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}

	if !names["security-scanner"] {
		t.Error("Expected security-scanner in results")
	}
	if !names["reviewer"] {
		t.Error("Expected reviewer in results")
	}
	if names["coder"] {
		t.Error("Did not expect coder in security-filtered results")
	}
}

func TestAgentDiscovery_ListAvailableAgents_WithRemote(t *testing.T) {
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
	defer server.Shutdown()

	clientCfg := config.LatticeConfig{
		StationID:   "discovery-station",
		StationName: "Discovery Station",
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

	remoteManifest := StationManifest{
		StationID:   "remote-station",
		StationName: "Remote Station",
		Agents: []AgentInfo{
			{
				ID:           "r1",
				Name:         "remote-coder",
				Description:  "Remote coding agent",
				Capabilities: []string{"coding"},
				InputSchema:  `{"type": "object"}`,
			},
		},
	}
	if err := registry.RegisterStation(ctx, remoteManifest); err != nil {
		t.Fatalf("Failed to register remote station: %v", err)
	}

	localAgents := []AgentInfo{
		{
			ID:           "1",
			Name:         "local-agent",
			Description:  "Local agent",
			Capabilities: []string{"local"},
		},
	}

	mockCollector := &MockLocalCollector{agents: localAgents}
	discovery := NewAgentDiscoveryWithMock(registry, mockCollector, "discovery-station", "Discovery Station")

	agents, err := discovery.ListAvailableAgents(ctx, "")
	if err != nil {
		t.Fatalf("ListAvailableAgents() error = %v", err)
	}

	if len(agents) != 2 {
		t.Fatalf("ListAvailableAgents() returned %d agents, want 2", len(agents))
	}

	var hasLocal, hasRemote bool
	for _, a := range agents {
		if a.Name == "local-agent" && a.IsLocal {
			hasLocal = true
		}
		if a.Name == "remote-coder" && !a.IsLocal {
			hasRemote = true
			if a.StationName != "Remote Station" {
				t.Errorf("remote agent StationName = %v, want Remote Station", a.StationName)
			}
		}
	}

	if !hasLocal {
		t.Error("Expected local-agent in results")
	}
	if !hasRemote {
		t.Error("Expected remote-coder in results")
	}
}

func TestAgentDiscovery_ListAvailableAgents_ExcludesOwnStation(t *testing.T) {
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
	defer server.Shutdown()

	clientCfg := config.LatticeConfig{
		StationID:   "my-station",
		StationName: "My Station",
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

	ownManifest := StationManifest{
		StationID:   "my-station",
		StationName: "My Station",
		Agents: []AgentInfo{
			{ID: "1", Name: "my-agent", Description: "My agent"},
		},
	}
	if err := registry.RegisterStation(ctx, ownManifest); err != nil {
		t.Fatalf("Failed to register own station: %v", err)
	}

	otherManifest := StationManifest{
		StationID:   "other-station",
		StationName: "Other Station",
		Agents: []AgentInfo{
			{ID: "2", Name: "other-agent", Description: "Other agent"},
		},
	}
	if err := registry.RegisterStation(ctx, otherManifest); err != nil {
		t.Fatalf("Failed to register other station: %v", err)
	}

	discovery := NewAgentDiscoveryWithMock(registry, nil, "my-station", "My Station")

	agents, err := discovery.ListAvailableAgents(ctx, "")
	if err != nil {
		t.Fatalf("ListAvailableAgents() error = %v", err)
	}

	if len(agents) != 1 {
		t.Fatalf("ListAvailableAgents() returned %d agents, want 1", len(agents))
	}

	if agents[0].Name != "other-agent" {
		t.Errorf("agents[0].Name = %v, want other-agent", agents[0].Name)
	}
}

func TestAgentDiscovery_GetAgentSchema_LocalAgent(t *testing.T) {
	localAgents := []AgentInfo{
		{
			ID:           "1",
			Name:         "coder",
			Description:  "A coding agent for writing code",
			Capabilities: []string{"coding"},
			InputSchema:  `{"type": "string", "description": "The task to perform"}`,
			OutputSchema: `{"type": "string", "description": "The code output"}`,
			Examples:     []string{"Write a hello world program", "Debug this function"},
		},
	}

	mockCollector := &MockLocalCollector{agents: localAgents}
	discovery := NewAgentDiscoveryWithMock(nil, mockCollector, "local-station", "Local Station")

	ctx := context.Background()
	schema, err := discovery.GetAgentSchema(ctx, "coder")
	if err != nil {
		t.Fatalf("GetAgentSchema() error = %v", err)
	}

	if schema.Name != "coder" {
		t.Errorf("schema.Name = %v, want coder", schema.Name)
	}
	if schema.Description != "A coding agent for writing code" {
		t.Errorf("schema.Description = %v, want 'A coding agent for writing code'", schema.Description)
	}
	if schema.InputSchema != `{"type": "string", "description": "The task to perform"}` {
		t.Errorf("schema.InputSchema mismatch")
	}
	if schema.OutputSchema != `{"type": "string", "description": "The code output"}` {
		t.Errorf("schema.OutputSchema mismatch")
	}
	if len(schema.Examples) != 2 {
		t.Errorf("len(schema.Examples) = %d, want 2", len(schema.Examples))
	}
	if schema.Location != "local" {
		t.Errorf("schema.Location = %v, want local", schema.Location)
	}
	if schema.StationID != "local-station" {
		t.Errorf("schema.StationID = %v, want local-station", schema.StationID)
	}
}

func TestAgentDiscovery_GetAgentSchema_RemoteAgent(t *testing.T) {
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
	defer server.Shutdown()

	clientCfg := config.LatticeConfig{
		StationID:   "query-station",
		StationName: "Query Station",
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

	remoteManifest := StationManifest{
		StationID:   "remote-station",
		StationName: "Remote Station",
		Agents: []AgentInfo{
			{
				ID:           "r1",
				Name:         "remote-analyzer",
				Description:  "Analyzes code for issues",
				Capabilities: []string{"analysis"},
				InputSchema:  `{"type": "object", "properties": {"code": {"type": "string"}}}`,
				OutputSchema: `{"type": "array", "items": {"type": "string"}}`,
				Examples:     []string{"Analyze this Python code"},
			},
		},
	}
	if err := registry.RegisterStation(ctx, remoteManifest); err != nil {
		t.Fatalf("Failed to register remote station: %v", err)
	}

	discovery := NewAgentDiscoveryWithMock(registry, nil, "query-station", "Query Station")

	schema, err := discovery.GetAgentSchema(ctx, "remote-analyzer")
	if err != nil {
		t.Fatalf("GetAgentSchema() error = %v", err)
	}

	if schema.Name != "remote-analyzer" {
		t.Errorf("schema.Name = %v, want remote-analyzer", schema.Name)
	}
	if schema.Location != "Remote Station" {
		t.Errorf("schema.Location = %v, want Remote Station", schema.Location)
	}
	if schema.StationID != "remote-station" {
		t.Errorf("schema.StationID = %v, want remote-station", schema.StationID)
	}
	if len(schema.Examples) != 1 {
		t.Errorf("len(schema.Examples) = %d, want 1", len(schema.Examples))
	}
}

func TestAgentDiscovery_GetAgentSchema_NotFound(t *testing.T) {
	mockCollector := &MockLocalCollector{agents: []AgentInfo{}}
	discovery := NewAgentDiscoveryWithMock(nil, mockCollector, "local-station", "Local Station")

	ctx := context.Background()
	_, err := discovery.GetAgentSchema(ctx, "nonexistent-agent")
	if err == nil {
		t.Error("GetAgentSchema() expected error for nonexistent agent")
	}

	if err.Error() != "agent 'nonexistent-agent' not found" {
		t.Errorf("error message = %v, want 'agent 'nonexistent-agent' not found'", err.Error())
	}
}

func TestAgentDiscovery_BuildAssignWorkDescription(t *testing.T) {
	localAgents := []AgentInfo{
		{
			ID:           "1",
			Name:         "coder",
			Description:  "A coding agent that writes high-quality, tested code",
			Capabilities: []string{"coding"},
		},
		{
			ID:           "2",
			Name:         "reviewer",
			Description:  "Reviews code for bugs and security issues",
			Capabilities: []string{"review"},
		},
	}

	mockCollector := &MockLocalCollector{agents: localAgents}
	discovery := NewAgentDiscoveryWithMock(nil, mockCollector, "local-station", "Local Station")

	ctx := context.Background()
	description := discovery.BuildAssignWorkDescription(ctx)

	if description == "" {
		t.Error("BuildAssignWorkDescription() returned empty string")
	}

	if !containsString(description, "coder") {
		t.Error("Description should mention 'coder' agent")
	}
	if !containsString(description, "reviewer") {
		t.Error("Description should mention 'reviewer' agent")
	}
	if !containsString(description, "local") {
		t.Error("Description should indicate local agents")
	}
	if !containsString(description, "list_available_agents") {
		t.Error("Description should include usage instructions")
	}
}

func TestAgentDiscovery_BuildAssignWorkDescription_NoAgents(t *testing.T) {
	mockCollector := &MockLocalCollector{agents: []AgentInfo{}}
	discovery := NewAgentDiscoveryWithMock(nil, mockCollector, "local-station", "Local Station")

	ctx := context.Background()
	description := discovery.BuildAssignWorkDescription(ctx)

	if !containsString(description, "no agents available") {
		t.Error("Description should indicate no agents available")
	}
}

func TestAgentDiscovery_BuildAssignWorkDescription_WithRemote(t *testing.T) {
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
	defer server.Shutdown()

	clientCfg := config.LatticeConfig{
		StationID:   "local-station",
		StationName: "Local Station",
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

	remoteManifest := StationManifest{
		StationID:   "remote-station",
		StationName: "Remote Station",
		Agents: []AgentInfo{
			{ID: "r1", Name: "remote-agent", Description: "A remote agent"},
		},
	}
	if err := registry.RegisterStation(ctx, remoteManifest); err != nil {
		t.Fatalf("Failed to register remote station: %v", err)
	}

	localAgents := []AgentInfo{
		{ID: "1", Name: "local-agent", Description: "A local agent"},
	}

	mockCollector := &MockLocalCollector{agents: localAgents}
	discovery := NewAgentDiscoveryWithMock(registry, mockCollector, "local-station", "Local Station")

	description := discovery.BuildAssignWorkDescription(ctx)

	if !containsString(description, "local-agent") {
		t.Error("Description should mention 'local-agent'")
	}
	if !containsString(description, "remote-agent") {
		t.Error("Description should mention 'remote-agent'")
	}
	if !containsString(description, "@Remote Station") {
		t.Error("Description should indicate remote station location")
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestHasCapability(t *testing.T) {
	tests := []struct {
		name         string
		capabilities []string
		target       string
		want         bool
	}{
		{"exact match", []string{"coding", "debugging"}, "coding", true},
		{"case insensitive", []string{"Coding", "Debugging"}, "coding", true},
		{"partial match", []string{"security-audit"}, "security", true},
		{"no match", []string{"coding", "debugging"}, "security", false},
		{"empty capabilities", []string{}, "coding", false},
		{"empty target matches all", []string{"coding"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasCapability(tt.capabilities, tt.target)
			if got != tt.want {
				t.Errorf("hasCapability(%v, %q) = %v, want %v",
					tt.capabilities, tt.target, got, tt.want)
			}
		})
	}
}

func TestTruncateDescription(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very short max", "hello", 3, "hel"},
		{"unicode safe", "hello", 4, "h..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateDescription(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateDescription(%q, %d) = %q, want %q",
					tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
