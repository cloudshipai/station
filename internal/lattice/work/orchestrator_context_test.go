//go:build integration

package work_test

import (
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"station/internal/config"
	"station/internal/lattice"
	"station/internal/lattice/work"

	"github.com/google/uuid"
)

func getTestFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

// MockExecutorWithContext extends MockExecutor with context-aware methods
type MockExecutorWithContext struct {
	mu                      sync.Mutex
	executions              []ExecutionRecord
	result                  string
	toolCalls               int
	err                     error
	delay                   time.Duration
	lastOrchestratorContext *work.OrchestratorContext
}

func NewMockExecutorWithContext(result string, toolCalls int) *MockExecutorWithContext {
	return &MockExecutorWithContext{
		result:    result,
		toolCalls: toolCalls,
	}
}

func (m *MockExecutorWithContext) ExecuteAgentByID(ctx context.Context, agentID, task string) (string, int, error) {
	m.mu.Lock()
	m.executions = append(m.executions, ExecutionRecord{
		AgentID:   agentID,
		Task:      task,
		Timestamp: time.Now(),
	})
	result := m.result
	toolCalls := m.toolCalls
	err := m.err
	delay := m.delay
	m.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	return result, toolCalls, err
}

func (m *MockExecutorWithContext) ExecuteAgentByName(ctx context.Context, agentName, task string) (string, int, error) {
	m.mu.Lock()
	m.executions = append(m.executions, ExecutionRecord{
		AgentName: agentName,
		Task:      task,
		Timestamp: time.Now(),
	})
	result := m.result
	toolCalls := m.toolCalls
	err := m.err
	delay := m.delay
	m.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	return result, toolCalls, err
}

func (m *MockExecutorWithContext) ExecuteAgentByIDWithContext(ctx context.Context, agentID string, task string, orchCtx *work.OrchestratorContext) (string, int64, int, error) {
	m.mu.Lock()
	m.lastOrchestratorContext = orchCtx
	m.executions = append(m.executions, ExecutionRecord{
		AgentID:   agentID,
		Task:      task,
		Timestamp: time.Now(),
	})
	result := m.result
	toolCalls := m.toolCalls
	err := m.err
	delay := m.delay
	m.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	return result, 123, toolCalls, err
}

func (m *MockExecutorWithContext) ExecuteAgentByNameWithContext(ctx context.Context, agentName string, task string, orchCtx *work.OrchestratorContext) (string, int64, int, error) {
	m.mu.Lock()
	m.lastOrchestratorContext = orchCtx
	m.executions = append(m.executions, ExecutionRecord{
		AgentName: agentName,
		Task:      task,
		Timestamp: time.Now(),
	})
	result := m.result
	toolCalls := m.toolCalls
	err := m.err
	delay := m.delay
	m.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	return result, 456, toolCalls, err
}

func (m *MockExecutorWithContext) GetLastOrchestratorContext() *work.OrchestratorContext {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastOrchestratorContext
}

func TestOrchestratorContext_NewRootContext(t *testing.T) {
	ctx := work.NewRootContext("station-1", "trace-abc123")

	if ctx.RunID == "" {
		t.Error("Expected non-empty RunID")
	}

	if _, err := uuid.Parse(ctx.RunID); err != nil {
		t.Errorf("RunID should be valid UUID, got: %s", ctx.RunID)
	}

	if ctx.RootRunID != ctx.RunID {
		t.Error("RootRunID should equal RunID for root context")
	}

	if ctx.ParentRunID != "" {
		t.Error("ParentRunID should be empty for root context")
	}

	if ctx.OriginatingStation != "station-1" {
		t.Errorf("Expected OriginatingStation 'station-1', got '%s'", ctx.OriginatingStation)
	}

	if ctx.TraceID != "trace-abc123" {
		t.Errorf("Expected TraceID 'trace-abc123', got '%s'", ctx.TraceID)
	}

	if ctx.Depth != 0 {
		t.Errorf("Expected Depth 0, got %d", ctx.Depth)
	}
}

func TestOrchestratorContext_NewChildContext(t *testing.T) {
	root := work.NewRootContext("station-1", "trace-abc123")
	child := root.NewChildContext()

	if child.ParentRunID != root.RunID {
		t.Error("Child ParentRunID should equal parent RunID")
	}

	if child.RootRunID != root.RootRunID {
		t.Error("Child RootRunID should equal root's RootRunID")
	}

	if child.Depth != 1 {
		t.Errorf("Expected child depth 1, got %d", child.Depth)
	}

	if child.OriginatingStation != root.OriginatingStation {
		t.Error("Child should inherit OriginatingStation")
	}

	if child.TraceID != root.TraceID {
		t.Error("Child should inherit TraceID")
	}

	if !strings.HasPrefix(child.RunID, root.RootRunID) {
		t.Errorf("Child RunID should start with root UUID, got: %s", child.RunID)
	}

	if !strings.Contains(child.RunID, "-1") {
		t.Errorf("Child RunID should contain index suffix, got: %s", child.RunID)
	}
}

func TestOrchestratorContext_MultipleChildren(t *testing.T) {
	root := work.NewRootContext("station-1", "trace-abc123")

	child1 := root.NewChildContext()
	child2 := root.NewChildContext()
	child3 := root.NewChildContext()

	if child1.RunID == child2.RunID {
		t.Error("Siblings should have different RunIDs")
	}

	if child2.RunID == child3.RunID {
		t.Error("Siblings should have different RunIDs")
	}

	grandchild := child1.NewChildContext()
	if grandchild.Depth != 2 {
		t.Errorf("Expected grandchild depth 2, got %d", grandchild.Depth)
	}

	if grandchild.ParentRunID != child1.RunID {
		t.Error("Grandchild ParentRunID should be child1 RunID")
	}
}

func TestOrchestratorContext_WithWorkID(t *testing.T) {
	root := work.NewRootContext("station-1", "trace-abc123")
	withWork := root.WithWorkID("work-xyz789")

	if withWork.WorkID != "work-xyz789" {
		t.Errorf("Expected WorkID 'work-xyz789', got '%s'", withWork.WorkID)
	}

	if withWork.RunID != root.RunID {
		t.Error("Should preserve RunID")
	}

	if withWork.TraceID != root.TraceID {
		t.Error("Should preserve TraceID")
	}
}

func TestOrchestratorContext_ToWorkAssignment(t *testing.T) {
	root := work.NewRootContext("station-1", "trace-abc123")
	root = root.WithWorkID("parent-work-id")

	assignment := root.ToWorkAssignment("TestAgent", "Do something", "target-station", 60000)

	if assignment.AgentName != "TestAgent" {
		t.Errorf("Expected AgentName 'TestAgent', got '%s'", assignment.AgentName)
	}

	if assignment.Task != "Do something" {
		t.Errorf("Expected Task 'Do something', got '%s'", assignment.Task)
	}

	if assignment.TargetStation != "target-station" {
		t.Errorf("Expected TargetStation 'target-station', got '%s'", assignment.TargetStation)
	}

	if assignment.ParentWorkID != "parent-work-id" {
		t.Errorf("Expected ParentWorkID 'parent-work-id', got '%s'", assignment.ParentWorkID)
	}

	if assignment.TraceID != "trace-abc123" {
		t.Errorf("Expected TraceID 'trace-abc123', got '%s'", assignment.TraceID)
	}

	if assignment.WorkID == "" {
		t.Error("WorkID should be generated")
	}

	if _, err := uuid.Parse(assignment.WorkID); err != nil {
		t.Errorf("WorkID should be valid UUID, got: %s", assignment.WorkID)
	}

	if !strings.HasPrefix(assignment.OrchestratorRunID, root.RootRunID) {
		t.Errorf("OrchestratorRunID should be child run ID, got: %s", assignment.OrchestratorRunID)
	}
}

func TestOrchestratorContext_GoRoutineContextPropagation(t *testing.T) {
	root := work.NewRootContext("station-1", "trace-abc123")
	ctx := work.WithContext(context.Background(), root)

	retrieved := work.FromContext(ctx)
	if retrieved == nil {
		t.Fatal("Expected to retrieve context from Go context")
	}

	if retrieved.RunID != root.RunID {
		t.Error("Retrieved context should match original")
	}

	noContext := work.FromContext(context.Background())
	if noContext != nil {
		t.Error("Expected nil from context without OrchestratorContext")
	}
}

func TestIntegration_OrchestratorContextPassedToExecutor(t *testing.T) {
	port := getTestFreePort(t)
	httpPort := getTestFreePort(t)

	serverCfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	server := lattice.NewEmbeddedServer(serverCfg)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start embedded server: %v", err)
	}
	defer server.Shutdown()

	clientCfg := config.LatticeConfig{
		StationID:   "context-test-station",
		StationName: "Context Test Station",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	client, err := lattice.NewClient(clientCfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	executor := NewMockExecutorWithContext("context result", 2)

	hook := work.NewHook(client, executor, "context-test-station")
	ctx := context.Background()
	if err := hook.Start(ctx); err != nil {
		t.Fatalf("Failed to start hook: %v", err)
	}
	defer hook.Stop()

	dispatcher := work.NewDispatcher(client, "context-test-station")
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	time.Sleep(100 * time.Millisecond)

	rootContext := work.NewRootContext("orchestrator-station", "trace-e2e-test")
	assignment := rootContext.ToWorkAssignment("ContextTestAgent", "Test task", "context-test-station", 10000)
	assignment.Timeout = 10 * time.Second

	workID, err := dispatcher.AssignWork(ctx, assignment)
	if err != nil {
		t.Fatalf("AssignWork failed: %v", err)
	}

	result, err := dispatcher.AwaitWork(ctx, workID)
	if err != nil {
		t.Fatalf("AwaitWork failed: %v", err)
	}

	if result.Type != work.MsgWorkComplete {
		t.Errorf("Expected WORK_COMPLETE, got %s (error: %s)", result.Type, result.Error)
	}

	orchCtx := executor.GetLastOrchestratorContext()
	if orchCtx == nil {
		t.Fatal("Expected OrchestratorContext to be passed to executor")
	}

	if orchCtx.RunID != assignment.OrchestratorRunID {
		t.Errorf("Expected RunID '%s', got '%s'", assignment.OrchestratorRunID, orchCtx.RunID)
	}

	if orchCtx.TraceID != "trace-e2e-test" {
		t.Errorf("Expected TraceID 'trace-e2e-test', got '%s'", orchCtx.TraceID)
	}

	if orchCtx.WorkID != assignment.WorkID {
		t.Errorf("Expected WorkID '%s', got '%s'", assignment.WorkID, orchCtx.WorkID)
	}

	if result.LocalRunID == 0 {
		t.Log("Note: LocalRunID not set (mock executor returns 456)")
	}
}

func TestIntegration_OrchestratorRunIDInResponse(t *testing.T) {
	port := getTestFreePort(t)
	httpPort := getTestFreePort(t)

	serverCfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	server := lattice.NewEmbeddedServer(serverCfg)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start embedded server: %v", err)
	}
	defer server.Shutdown()

	clientCfg := config.LatticeConfig{
		StationID:   "runid-test-station",
		StationName: "RunID Test Station",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	client, err := lattice.NewClient(clientCfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	executor := NewMockExecutorWithContext("runid result", 1)

	hook := work.NewHook(client, executor, "runid-test-station")
	ctx := context.Background()
	if err := hook.Start(ctx); err != nil {
		t.Fatalf("Failed to start hook: %v", err)
	}
	defer hook.Stop()

	dispatcher := work.NewDispatcher(client, "runid-test-station")
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	time.Sleep(100 * time.Millisecond)

	orchestratorRunID := "550e8400-e29b-41d4-a716-446655440000-1"
	assignment := &work.WorkAssignment{
		AgentName:         "RunIDAgent",
		Task:              "Check run ID propagation",
		OrchestratorRunID: orchestratorRunID,
		TraceID:           "trace-runid-test",
		Timeout:           10 * time.Second,
	}

	workID, err := dispatcher.AssignWork(ctx, assignment)
	if err != nil {
		t.Fatalf("AssignWork failed: %v", err)
	}

	result, err := dispatcher.AwaitWork(ctx, workID)
	if err != nil {
		t.Fatalf("AwaitWork failed: %v", err)
	}

	if result.Type != work.MsgWorkComplete {
		t.Errorf("Expected WORK_COMPLETE, got %s", result.Type)
	}

	if result.OrchestratorRunID != orchestratorRunID {
		t.Errorf("Expected OrchestratorRunID '%s' in response, got '%s'", orchestratorRunID, result.OrchestratorRunID)
	}

	orchCtx := executor.GetLastOrchestratorContext()
	if orchCtx == nil {
		t.Fatal("Expected OrchestratorContext in executor")
	}

	if orchCtx.RunID != orchestratorRunID {
		t.Errorf("Expected RunID '%s' in executor context, got '%s'", orchestratorRunID, orchCtx.RunID)
	}
}

func TestIntegration_DistributedRunChain(t *testing.T) {
	port := getTestFreePort(t)
	httpPort := getTestFreePort(t)

	serverCfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	server := lattice.NewEmbeddedServer(serverCfg)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start embedded server: %v", err)
	}
	defer server.Shutdown()

	station1Cfg := config.LatticeConfig{
		StationID:   "chain-station-1",
		StationName: "Chain Station 1",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}
	client1, _ := lattice.NewClient(station1Cfg)
	client1.Connect()
	defer client1.Close()

	station2Cfg := config.LatticeConfig{
		StationID:   "chain-station-2",
		StationName: "Chain Station 2",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}
	client2, _ := lattice.NewClient(station2Cfg)
	client2.Connect()
	defer client2.Close()

	executor1 := NewMockExecutorWithContext("result from station 1", 1)
	executor2 := NewMockExecutorWithContext("result from station 2", 2)

	hook1 := work.NewHook(client1, executor1, "chain-station-1")
	hook2 := work.NewHook(client2, executor2, "chain-station-2")
	ctx := context.Background()

	hook1.Start(ctx)
	hook2.Start(ctx)
	defer hook1.Stop()
	defer hook2.Stop()

	dispatcher := work.NewDispatcher(client1, "chain-station-1")
	dispatcher.Start(ctx)
	defer dispatcher.Stop()

	time.Sleep(100 * time.Millisecond)

	rootCtx := work.NewRootContext("orchestrator", "trace-chain-test")
	rootRunID := rootCtx.RootRunID

	assignment1 := rootCtx.ToWorkAssignment("Agent1", "First task", "chain-station-1", 10000)
	assignment1.Timeout = 10 * time.Second

	workID1, err := dispatcher.AssignWork(ctx, assignment1)
	if err != nil {
		t.Fatalf("First assignment failed: %v", err)
	}

	result1, err := dispatcher.AwaitWork(ctx, workID1)
	if err != nil {
		t.Fatalf("First await failed: %v", err)
	}

	if result1.Type != work.MsgWorkComplete {
		t.Fatalf("First task failed: %s", result1.Error)
	}

	childCtx := rootCtx.NewChildContext()
	assignment2 := childCtx.ToWorkAssignment("Agent2", "Second task", "chain-station-2", 10000)
	assignment2.Timeout = 10 * time.Second

	workID2, err := dispatcher.AssignWork(ctx, assignment2)
	if err != nil {
		t.Fatalf("Second assignment failed: %v", err)
	}

	result2, err := dispatcher.AwaitWork(ctx, workID2)
	if err != nil {
		t.Fatalf("Second await failed: %v", err)
	}

	if result2.Type != work.MsgWorkComplete {
		t.Fatalf("Second task failed: %s", result2.Error)
	}

	ctx1 := executor1.GetLastOrchestratorContext()
	ctx2 := executor2.GetLastOrchestratorContext()

	if ctx1 == nil || ctx2 == nil {
		t.Fatal("Expected both executors to receive context")
	}

	if !strings.HasPrefix(ctx1.RunID, rootRunID) {
		t.Errorf("Station 1 RunID should start with root UUID: %s", ctx1.RunID)
	}

	if !strings.HasPrefix(ctx2.RunID, rootRunID) {
		t.Errorf("Station 2 RunID should start with root UUID: %s", ctx2.RunID)
	}

	if ctx1.TraceID != "trace-chain-test" || ctx2.TraceID != "trace-chain-test" {
		t.Error("Both should have same TraceID")
	}

	t.Logf("Root RunID: %s", rootRunID)
	t.Logf("Station 1 RunID: %s", ctx1.RunID)
	t.Logf("Station 2 RunID: %s", ctx2.RunID)
}
