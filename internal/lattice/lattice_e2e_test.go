package lattice_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"station/internal/config"
	"station/internal/lattice"
	"station/internal/lattice/work"
)

type TestStation struct {
	ID          string
	Name        string
	Server      *lattice.EmbeddedServer
	Client      *lattice.Client
	Registry    *lattice.Registry
	Presence    *lattice.Presence
	Invoker     *lattice.Invoker
	WorkStore   *work.WorkStore
	Manifest    lattice.StationManifest
	Executor    *MockAgentExecutor
	TempDir     string
	NATSPort    int
	HTTPPort    int
	IsLeaf      bool
	LeafNATSURL string
}

type MockAgentExecutor struct {
	mu           sync.Mutex
	invocations  []AgentInvocation
	responses    map[string]MockResponse
	defaultDelay time.Duration
}

type AgentInvocation struct {
	AgentID   string
	AgentName string
	Task      string
	Timestamp time.Time
	StationID string
}

type MockResponse struct {
	Result    string
	ToolCalls int
	Error     error
	Delay     time.Duration
}

func NewMockAgentExecutor() *MockAgentExecutor {
	return &MockAgentExecutor{
		responses:    make(map[string]MockResponse),
		defaultDelay: 100 * time.Millisecond,
	}
}

func (m *MockAgentExecutor) SetResponse(agentName string, resp MockResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[agentName] = resp
}

func (m *MockAgentExecutor) GetInvocations() []AgentInvocation {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]AgentInvocation, len(m.invocations))
	copy(result, m.invocations)
	return result
}

func (m *MockAgentExecutor) ExecuteAgentByID(ctx context.Context, agentID, task string) (string, int, error) {
	return m.execute(ctx, agentID, "", task)
}

func (m *MockAgentExecutor) ExecuteAgentByName(ctx context.Context, agentName, task string) (string, int, error) {
	return m.execute(ctx, "", agentName, task)
}

func (m *MockAgentExecutor) execute(ctx context.Context, agentID, agentName, task string) (string, int, error) {
	m.mu.Lock()
	m.invocations = append(m.invocations, AgentInvocation{
		AgentID:   agentID,
		AgentName: agentName,
		Task:      task,
		Timestamp: time.Now(),
	})

	key := agentName
	if key == "" {
		key = agentID
	}
	resp, ok := m.responses[key]
	m.mu.Unlock()

	delay := m.defaultDelay
	if ok && resp.Delay > 0 {
		delay = resp.Delay
	}

	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return "", 0, ctx.Err()
	}

	if ok {
		return resp.Result, resp.ToolCalls, resp.Error
	}

	return fmt.Sprintf("Mock result for %s: %s", key, task), 1, nil
}

func createTestStation(t *testing.T, name string, isOrchestrator bool, natsURL string) *TestStation {
	t.Helper()

	tempDir := t.TempDir()

	station := &TestStation{
		ID:       fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
		Name:     name,
		TempDir:  tempDir,
		IsLeaf:   !isOrchestrator,
		Executor: NewMockAgentExecutor(),
	}

	var natsPort, httpPort int
	if isOrchestrator {
		natsPort = 14222 + int(time.Now().UnixNano()%1000)
		httpPort = 18222 + int(time.Now().UnixNano()%1000)
		station.NATSPort = natsPort
		station.HTTPPort = httpPort

		embeddedCfg := config.LatticeEmbeddedNATSConfig{
			Port:     natsPort,
			HTTPPort: httpPort,
			StoreDir: filepath.Join(tempDir, "nats"),
		}

		server := lattice.NewEmbeddedServer(embeddedCfg)
		if err := server.Start(); err != nil {
			t.Fatalf("Failed to start embedded NATS: %v", err)
		}
		station.Server = server
		natsURL = fmt.Sprintf("nats://127.0.0.1:%d", natsPort)
	}
	station.LeafNATSURL = natsURL

	latticeCfg := config.LatticeConfig{
		StationID:   station.ID,
		StationName: station.Name,
		NATS: config.LatticeNATSConfig{
			URL: natsURL,
		},
	}

	client, err := lattice.NewClient(latticeCfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	station.Client = client

	registry := lattice.NewRegistry(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := registry.Initialize(ctx); err != nil {
		t.Fatalf("Failed to init registry: %v", err)
	}
	station.Registry = registry

	station.Manifest = lattice.StationManifest{
		StationID:   station.ID,
		StationName: station.Name,
		Status:      lattice.StatusOnline,
		LastSeen:    time.Now(),
		Agents:      []lattice.AgentInfo{},
		Workflows:   []lattice.WorkflowInfo{},
	}

	presence := lattice.NewPresence(client, registry, station.Manifest, 5)
	station.Presence = presence

	invoker := lattice.NewInvoker(client, station.ID, station.Executor)
	station.Invoker = invoker

	js := client.JetStream()
	if js != nil {
		workStore, err := work.NewWorkStore(js, work.DefaultWorkStoreConfig())
		if err == nil {
			station.WorkStore = workStore
		}
	}

	return station
}

func (s *TestStation) AddAgent(name, description string, capabilities []string) {
	s.Manifest.Agents = append(s.Manifest.Agents, lattice.AgentInfo{
		ID:           fmt.Sprintf("agent-%s-%d", name, len(s.Manifest.Agents)),
		Name:         name,
		Description:  description,
		Capabilities: capabilities,
	})
	s.Presence.UpdateManifest(s.Manifest)
}

func (s *TestStation) Start(ctx context.Context) error {
	if err := s.Presence.Start(ctx); err != nil {
		return fmt.Errorf("failed to start presence: %w", err)
	}
	if err := s.Invoker.Start(ctx); err != nil {
		return fmt.Errorf("failed to start invoker: %w", err)
	}
	return nil
}

func (s *TestStation) Stop() {
	if s.Presence != nil {
		s.Presence.Stop()
	}
	if s.Invoker != nil {
		s.Invoker.Stop()
	}
	if s.Client != nil {
		s.Client.Close()
	}
	if s.Server != nil {
		s.Server.Shutdown()
	}
}

func TestLattice_UseCase1_ParallelAgentExecution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	orchestrator := createTestStation(t, "orchestrator", true, "")
	defer orchestrator.Stop()

	sre := createTestStation(t, "sre-station", false, orchestrator.LeafNATSURL)
	defer sre.Stop()
	sre.AddAgent("K8sHealthChecker", "Checks K8s health", []string{"kubernetes", "monitoring"})
	sre.AddAgent("LogAnalyzer", "Analyzes logs", []string{"logging", "analysis"})

	security := createTestStation(t, "security-station", false, orchestrator.LeafNATSURL)
	defer security.Stop()
	security.AddAgent("VulnScanner", "Scans vulnerabilities", []string{"security", "scanning"})
	security.AddAgent("NetworkAudit", "Audits network", []string{"security", "network"})

	sre.Executor.SetResponse("K8sHealthChecker", MockResponse{
		Result:    "All 15 pods healthy in production namespace",
		ToolCalls: 2,
		Delay:     200 * time.Millisecond,
	})
	sre.Executor.SetResponse("LogAnalyzer", MockResponse{
		Result:    "Found 3 ERROR entries in last hour",
		ToolCalls: 1,
		Delay:     150 * time.Millisecond,
	})
	security.Executor.SetResponse("VulnScanner", MockResponse{
		Result:    "2 HIGH, 5 MEDIUM vulnerabilities found",
		ToolCalls: 3,
		Delay:     300 * time.Millisecond,
	})
	security.Executor.SetResponse("NetworkAudit", MockResponse{
		Result:    "Port 22 exposed, firewall rule missing",
		ToolCalls: 2,
		Delay:     250 * time.Millisecond,
	})

	if err := orchestrator.Start(ctx); err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	if err := sre.Start(ctx); err != nil {
		t.Fatalf("Failed to start SRE: %v", err)
	}
	if err := security.Start(ctx); err != nil {
		t.Fatalf("Failed to start Security: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	router := lattice.NewAgentRouter(orchestrator.Registry, orchestrator.ID)
	agents, err := router.ListAllAgents(ctx)
	if err != nil {
		t.Fatalf("Failed to list agents: %v", err)
	}
	t.Logf("Discovered %d agents in lattice", len(agents))

	if len(agents) < 4 {
		t.Fatalf("Expected at least 4 agents, got %d", len(agents))
	}

	var wg sync.WaitGroup
	results := make(chan struct {
		agent  string
		result string
		err    error
	}, 4)

	invokeAgent := func(agentName, task string) {
		defer wg.Done()

		location, err := router.FindBestAgent(ctx, agentName, "")
		if err != nil {
			results <- struct {
				agent  string
				result string
				err    error
			}{agentName, "", err}
			return
		}

		req := lattice.InvokeAgentRequest{
			AgentName: agentName,
			Task:      task,
		}

		var invoker *lattice.Invoker
		if location.StationID == sre.ID {
			invoker = lattice.NewInvoker(sre.Client, sre.ID, nil)
		} else {
			invoker = lattice.NewInvoker(security.Client, security.ID, nil)
		}

		resp, err := invoker.InvokeRemoteAgent(ctx, location.StationID, req)
		if err != nil {
			results <- struct {
				agent  string
				result string
				err    error
			}{agentName, "", err}
			return
		}

		results <- struct {
			agent  string
			result string
			err    error
		}{agentName, resp.Result, nil}
	}

	startTime := time.Now()

	wg.Add(4)
	go invokeAgent("K8sHealthChecker", "Check pod health")
	go invokeAgent("LogAnalyzer", "Analyze recent logs")
	go invokeAgent("VulnScanner", "Scan for vulnerabilities")
	go invokeAgent("NetworkAudit", "Audit network config")

	wg.Wait()
	close(results)

	elapsed := time.Since(startTime)
	t.Logf("Parallel execution completed in %v", elapsed)

	if elapsed > 1*time.Second {
		t.Errorf("Parallel execution took too long: %v (should be < 1s for parallel)", elapsed)
	}

	successCount := 0
	for r := range results {
		if r.err != nil {
			t.Errorf("Agent %s failed: %v", r.agent, r.err)
		} else {
			t.Logf("Agent %s result: %s", r.agent, r.result)
			successCount++
		}
	}

	if successCount < 4 {
		t.Errorf("Expected 4 successful invocations, got %d", successCount)
	}
}

func TestLattice_UseCase2_AgentFailureAndRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	orchestrator := createTestStation(t, "orchestrator", true, "")
	defer orchestrator.Stop()

	worker := createTestStation(t, "worker", false, orchestrator.LeafNATSURL)
	defer worker.Stop()
	worker.AddAgent("UnreliableAgent", "Sometimes fails", []string{"testing"})
	worker.AddAgent("ReliableAgent", "Always works", []string{"testing"})

	worker.Executor.SetResponse("UnreliableAgent", MockResponse{
		Result: "",
		Error:  fmt.Errorf("simulated failure"),
		Delay:  50 * time.Millisecond,
	})
	worker.Executor.SetResponse("ReliableAgent", MockResponse{
		Result:    "Success!",
		ToolCalls: 1,
		Delay:     50 * time.Millisecond,
	})

	if err := orchestrator.Start(ctx); err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	router := lattice.NewAgentRouter(orchestrator.Registry, orchestrator.ID)

	location, _ := router.FindBestAgent(ctx, "UnreliableAgent", "")
	if location != nil {
		invoker := lattice.NewInvoker(worker.Client, worker.ID, nil)
		resp, err := invoker.InvokeRemoteAgent(ctx, location.StationID, lattice.InvokeAgentRequest{
			AgentName: "UnreliableAgent",
			Task:      "Do something risky",
		})

		if err == nil && resp.Status == "error" {
			t.Logf("UnreliableAgent failed as expected: %s", resp.Error)
		}
	}

	location, _ = router.FindBestAgent(ctx, "ReliableAgent", "")
	if location != nil {
		invoker := lattice.NewInvoker(worker.Client, worker.ID, nil)
		resp, err := invoker.InvokeRemoteAgent(ctx, location.StationID, lattice.InvokeAgentRequest{
			AgentName: "ReliableAgent",
			Task:      "Do something safe",
		})

		if err != nil {
			t.Errorf("ReliableAgent should not fail: %v", err)
		} else if resp.Status != "success" {
			t.Errorf("ReliableAgent should succeed, got status: %s", resp.Status)
		} else {
			t.Logf("ReliableAgent succeeded: %s", resp.Result)
		}
	}
}

func TestLattice_UseCase3_WorkQueueAndAsync(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	orchestrator := createTestStation(t, "orchestrator", true, "")
	defer orchestrator.Stop()

	if orchestrator.WorkStore == nil {
		t.Skip("WorkStore not available")
	}

	if err := orchestrator.Start(ctx); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	workID := fmt.Sprintf("work-%d", time.Now().UnixNano())
	record := &work.WorkRecord{
		WorkID:        workID,
		AgentName:     "TestAgent",
		Task:          "Process data batch",
		TargetStation: "worker-station",
		AssignedAt:    time.Now(),
	}

	if err := orchestrator.WorkStore.Assign(ctx, record); err != nil {
		t.Fatalf("Failed to assign work: %v", err)
	}
	t.Logf("Work assigned: %s", workID)

	retrieved, err := orchestrator.WorkStore.Get(ctx, workID)
	if err != nil {
		t.Fatalf("Failed to get work: %v", err)
	}
	if retrieved.Status != work.StatusAssigned {
		t.Errorf("Expected status 'assigned', got '%s'", retrieved.Status)
	}

	if err := orchestrator.WorkStore.UpdateStatus(ctx, workID, work.StatusAccepted, nil); err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	result := &work.WorkResult{
		Result:     "Processed 1000 records",
		ToolCalls:  5,
		DurationMs: 1234.5,
	}
	if err := orchestrator.WorkStore.UpdateStatus(ctx, workID, work.StatusComplete, result); err != nil {
		t.Fatalf("Failed to complete work: %v", err)
	}

	final, err := orchestrator.WorkStore.Get(ctx, workID)
	if err != nil {
		t.Fatalf("Failed to get final work: %v", err)
	}

	if final.Status != work.StatusComplete {
		t.Errorf("Expected status 'complete', got '%s'", final.Status)
	}
	if final.Result != "Processed 1000 records" {
		t.Errorf("Unexpected result: %s", final.Result)
	}
	t.Logf("Work completed: %s -> %s (%.2fms)", workID, final.Result, final.DurationMs)

	history, err := orchestrator.WorkStore.GetHistory(ctx, workID)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	t.Logf("Work history has %d entries", len(history))
	for i, h := range history {
		t.Logf("  [%d] Status: %s", i, h.Status)
	}
}

func TestLattice_UseCase4_StationDisconnectReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	orchestrator := createTestStation(t, "orchestrator", true, "")
	defer orchestrator.Stop()

	worker := createTestStation(t, "worker", false, orchestrator.LeafNATSURL)
	worker.AddAgent("FlappingAgent", "Agent on unstable station", []string{"testing"})

	if err := orchestrator.Start(ctx); err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	if err := worker.Start(ctx); err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	router := lattice.NewAgentRouter(orchestrator.Registry, orchestrator.ID)
	agents1, _ := router.ListAllAgents(ctx)
	t.Logf("Before disconnect: %d agents", len(agents1))

	worker.Stop()
	t.Log("Worker disconnected")

	time.Sleep(200 * time.Millisecond)

	worker2 := createTestStation(t, "worker", false, orchestrator.LeafNATSURL)
	defer worker2.Stop()
	worker2.AddAgent("FlappingAgent", "Agent on unstable station", []string{"testing"})
	worker2.AddAgent("NewAgent", "New agent after reconnect", []string{"testing"})

	if err := worker2.Start(ctx); err != nil {
		t.Fatalf("Failed to restart worker: %v", err)
	}
	t.Log("Worker reconnected")

	time.Sleep(300 * time.Millisecond)

	agents2, _ := router.ListAllAgents(ctx)
	t.Logf("After reconnect: %d agents", len(agents2))

	if len(agents2) < 1 {
		t.Error("Expected agents to be available after reconnect")
	}
}

func TestLattice_UseCase5_NATSMonitoringEndpoints(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	orchestrator := createTestStation(t, "orchestrator", true, "")
	defer orchestrator.Stop()

	if err := orchestrator.Start(ctx); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	monitorURL := fmt.Sprintf("http://127.0.0.1:%d", orchestrator.HTTPPort)

	endpoints := []struct {
		path string
		desc string
	}{
		{"/varz", "Server variables"},
		{"/connz", "Connection info"},
		{"/subsz", "Subscriptions"},
		{"/jsz", "JetStream info"},
	}

	for _, ep := range endpoints {
		resp, err := http.Get(monitorURL + ep.path)
		if err != nil {
			t.Errorf("Failed to fetch %s: %v", ep.path, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s returned status %d", ep.path, resp.StatusCode)
			continue
		}

		var data map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			t.Errorf("Failed to decode %s response: %v", ep.path, err)
			continue
		}

		t.Logf("✓ %s (%s) - %d fields", ep.path, ep.desc, len(data))
	}

	resp, err := http.Get(monitorURL + "/jsz?streams=true")
	if err == nil {
		defer resp.Body.Close()
		var jsData map[string]interface{}
		if json.NewDecoder(resp.Body).Decode(&jsData) == nil {
			if streams, ok := jsData["streams"].([]interface{}); ok {
				t.Logf("JetStream streams: %d", len(streams))
				for _, s := range streams {
					if stream, ok := s.(map[string]interface{}); ok {
						t.Logf("  - %v", stream["name"])
					}
				}
			}
		}
	}
}

func TestLattice_UseCase6_OrchestratorContextTracking(t *testing.T) {
	rootCtx := work.NewRootContext("orchestrator-station", "trace-test-123")

	if rootCtx.RunID == "" {
		t.Error("RunID should be auto-generated")
	}
	t.Logf("Root context: RunID=%s, TraceID=%s", rootCtx.RunID, rootCtx.TraceID)

	child1Ctx := rootCtx.NewChildContext()
	child2Ctx := rootCtx.NewChildContext()

	if child1Ctx.ParentRunID != rootCtx.RunID {
		t.Errorf("Child1 parent should be root RunID")
	}
	if child2Ctx.ParentRunID != rootCtx.RunID {
		t.Errorf("Child2 parent should be root RunID")
	}
	if child1Ctx.TraceID != rootCtx.TraceID {
		t.Errorf("Child should inherit TraceID")
	}

	t.Logf("Child1: RunID=%s, ParentRunID=%s", child1Ctx.RunID, child1Ctx.ParentRunID)
	t.Logf("Child2: RunID=%s, ParentRunID=%s", child2Ctx.RunID, child2Ctx.ParentRunID)

	grandchild := child1Ctx.NewChildContext()
	if grandchild.ParentRunID != child1Ctx.RunID {
		t.Errorf("Grandchild parent should be child1 RunID")
	}
	if grandchild.RootRunID != rootCtx.RunID {
		t.Errorf("Grandchild should track root RunID")
	}
	t.Logf("Grandchild: RunID=%s, ParentRunID=%s, RootRunID=%s",
		grandchild.RunID, grandchild.ParentRunID, grandchild.RootRunID)

	assignment := rootCtx.ToWorkAssignment("TestAgent", "Do something", "target-station", 60)
	if assignment.OrchestratorRunID == "" {
		t.Errorf("Assignment should have OrchestratorRunID")
	}
	t.Logf("Work assignment: WorkID=%s, OrchestratorRunID=%s",
		assignment.WorkID, assignment.OrchestratorRunID)
}

func TestLattice_UseCase7_HighVolumeWorkProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high volume test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	orchestrator := createTestStation(t, "orchestrator", true, "")
	defer orchestrator.Stop()

	if orchestrator.WorkStore == nil {
		t.Skip("WorkStore not available")
	}

	if err := orchestrator.Start(ctx); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	numWorkers := 5
	workPerWorker := 20
	totalWork := numWorkers * workPerWorker

	var wg sync.WaitGroup
	var successCount int32
	var errorCount int32

	startTime := time.Now()

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for i := 0; i < workPerWorker; i++ {
				workID := fmt.Sprintf("work-w%d-i%d-%d", workerID, i, time.Now().UnixNano())
				record := &work.WorkRecord{
					WorkID:        workID,
					AgentName:     fmt.Sprintf("Agent%d", workerID),
					Task:          fmt.Sprintf("Task %d from worker %d", i, workerID),
					TargetStation: fmt.Sprintf("station-%d", workerID),
				}

				if err := orchestrator.WorkStore.Assign(ctx, record); err != nil {
					atomic.AddInt32(&errorCount, 1)
					continue
				}

				result := &work.WorkResult{
					Result:     fmt.Sprintf("Result for %s", workID),
					ToolCalls:  1,
					DurationMs: 50,
				}
				if err := orchestrator.WorkStore.UpdateStatus(ctx, workID, work.StatusComplete, result); err != nil {
					atomic.AddInt32(&errorCount, 1)
					continue
				}

				atomic.AddInt32(&successCount, 1)
			}
		}(w)
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	t.Logf("High volume test: %d/%d successful in %v", successCount, totalWork, elapsed)
	t.Logf("Throughput: %.2f work/sec", float64(successCount)/elapsed.Seconds())
	t.Logf("Errors: %d", errorCount)

	if successCount < int32(totalWork*8/10) {
		t.Errorf("Expected at least 80%% success rate, got %d/%d", successCount, totalWork)
	}
}

func TestLattice_UseCase8_AgentCapabilityDiscovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	orchestrator := createTestStation(t, "orchestrator", true, "")
	defer orchestrator.Stop()

	sre := createTestStation(t, "sre", false, orchestrator.LeafNATSURL)
	defer sre.Stop()
	sre.AddAgent("K8sMonitor", "Monitors K8s", []string{"kubernetes", "monitoring", "alerting"})
	sre.AddAgent("PrometheusAgent", "Prometheus integration", []string{"monitoring", "metrics", "prometheus"})

	security := createTestStation(t, "security", false, orchestrator.LeafNATSURL)
	defer security.Stop()
	security.AddAgent("WAFManager", "WAF management", []string{"security", "waf", "firewall"})
	security.AddAgent("SecretScanner", "Scans for secrets", []string{"security", "secrets", "compliance"})

	devops := createTestStation(t, "devops", false, orchestrator.LeafNATSURL)
	defer devops.Stop()
	devops.AddAgent("CICDRunner", "Runs CI/CD", []string{"cicd", "deployment", "automation"})
	devops.AddAgent("TerraformAgent", "Terraform automation", []string{"infrastructure", "terraform", "automation"})

	if err := orchestrator.Start(ctx); err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	if err := sre.Start(ctx); err != nil {
		t.Fatalf("Failed to start SRE: %v", err)
	}
	if err := security.Start(ctx); err != nil {
		t.Fatalf("Failed to start Security: %v", err)
	}
	if err := devops.Start(ctx); err != nil {
		t.Fatalf("Failed to start DevOps: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	router := lattice.NewAgentRouter(orchestrator.Registry, orchestrator.ID)

	allAgents, err := router.ListAllAgents(ctx)
	if err != nil {
		t.Fatalf("Failed to list agents: %v", err)
	}
	t.Logf("Total agents in lattice: %d", len(allAgents))

	capabilityTests := []struct {
		capability  string
		expectedMin int
	}{
		{"monitoring", 2},
		{"security", 2},
		{"automation", 2},
		{"kubernetes", 1},
	}

	for _, tc := range capabilityTests {
		agents, err := orchestrator.Registry.FindAgentsByCapability(ctx, tc.capability)
		if err != nil {
			t.Errorf("Failed to find agents with capability '%s': %v", tc.capability, err)
			continue
		}

		t.Logf("Capability '%s': %d agents found", tc.capability, len(agents))
		for _, a := range agents {
			t.Logf("  - %s", a.Name)
		}

		if len(agents) < tc.expectedMin {
			t.Errorf("Expected at least %d agents with capability '%s', got %d",
				tc.expectedMin, tc.capability, len(agents))
		}
	}
}

func init() {
	os.Setenv("OTEL_SDK_DISABLED", "true")
}
