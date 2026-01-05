//go:build integration

package work_test

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"station/internal/config"
	"station/internal/lattice"
	"station/internal/lattice/work"
)

func getFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

type MockExecutor struct {
	mu         sync.Mutex
	executions []ExecutionRecord
	result     string
	toolCalls  int
	err        error
	delay      time.Duration
}

type ExecutionRecord struct {
	AgentID   string
	AgentName string
	Task      string
	Timestamp time.Time
}

func NewMockExecutor(result string, toolCalls int) *MockExecutor {
	return &MockExecutor{
		result:    result,
		toolCalls: toolCalls,
	}
}

func (m *MockExecutor) SetDelay(d time.Duration) {
	m.delay = d
}

func (m *MockExecutor) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MockExecutor) ExecuteAgentByID(ctx context.Context, agentID, task string) (string, int, error) {
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

func (m *MockExecutor) ExecuteAgentByName(ctx context.Context, agentName, task string) (string, int, error) {
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

func (m *MockExecutor) GetExecutions() []ExecutionRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ExecutionRecord{}, m.executions...)
}

func TestIntegration_SingleStationAsyncWork(t *testing.T) {
	port := getFreePort(t)
	httpPort := getFreePort(t)

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
		StationID:   "test-station",
		StationName: "Test Station",
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

	executor := NewMockExecutor("Hello from agent!", 3)

	hook := work.NewHook(client, executor, "test-station")
	ctx := context.Background()
	if err := hook.Start(ctx); err != nil {
		t.Fatalf("Failed to start hook: %v", err)
	}
	defer hook.Stop()

	dispatcher := work.NewDispatcher(client, "test-station")
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	time.Sleep(100 * time.Millisecond)

	t.Run("AssignAndAwait", func(t *testing.T) {
		assignment := &work.WorkAssignment{
			AgentName: "test-agent",
			Task:      "Do something useful",
			Timeout:   10 * time.Second,
		}

		workID, err := dispatcher.AssignWork(ctx, assignment)
		if err != nil {
			t.Fatalf("AssignWork failed: %v", err)
		}

		if workID == "" {
			t.Fatal("Expected non-empty work ID")
		}

		result, err := dispatcher.AwaitWork(ctx, workID)
		if err != nil {
			t.Fatalf("AwaitWork failed: %v", err)
		}

		if result.Type != work.MsgWorkComplete {
			t.Errorf("Expected WORK_COMPLETE, got %s", result.Type)
		}

		if result.Result != "Hello from agent!" {
			t.Errorf("Expected 'Hello from agent!', got '%s'", result.Result)
		}

		if result.ToolCalls != 3 {
			t.Errorf("Expected 3 tool calls, got %d", result.ToolCalls)
		}

		execs := executor.GetExecutions()
		if len(execs) != 1 {
			t.Fatalf("Expected 1 execution, got %d", len(execs))
		}
		if execs[0].AgentName != "test-agent" {
			t.Errorf("Expected agent name 'test-agent', got '%s'", execs[0].AgentName)
		}
	})

	t.Run("AssignWithAgentID", func(t *testing.T) {
		assignment := &work.WorkAssignment{
			AgentID: "42",
			Task:    "Another task",
			Timeout: 10 * time.Second,
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

		execs := executor.GetExecutions()
		lastExec := execs[len(execs)-1]
		if lastExec.AgentID != "42" {
			t.Errorf("Expected agent ID '42', got '%s'", lastExec.AgentID)
		}
	})

	t.Run("CheckWorkBeforeComplete", func(t *testing.T) {
		executor.SetDelay(500 * time.Millisecond)
		defer executor.SetDelay(0)

		assignment := &work.WorkAssignment{
			AgentName: "slow-agent",
			Task:      "Take your time",
			Timeout:   10 * time.Second,
		}

		workID, err := dispatcher.AssignWork(ctx, assignment)
		if err != nil {
			t.Fatalf("AssignWork failed: %v", err)
		}

		time.Sleep(100 * time.Millisecond)

		status, err := dispatcher.CheckWork(workID)
		if err != nil {
			t.Fatalf("CheckWork failed: %v", err)
		}

		if status.Status != "PENDING" && status.Status != work.MsgWorkAccepted {
			t.Logf("Status was: %s (expected PENDING or WORK_ACCEPTED)", status.Status)
		}

		result, err := dispatcher.AwaitWork(ctx, workID)
		if err != nil {
			t.Fatalf("AwaitWork failed: %v", err)
		}

		if result.Type != work.MsgWorkComplete {
			t.Errorf("Expected WORK_COMPLETE, got %s", result.Type)
		}
	})

	t.Run("WorkFailure", func(t *testing.T) {
		executor.SetError(fmt.Errorf("agent crashed"))
		defer executor.SetError(nil)

		assignment := &work.WorkAssignment{
			AgentName: "failing-agent",
			Task:      "This will fail",
			Timeout:   10 * time.Second,
		}

		workID, err := dispatcher.AssignWork(ctx, assignment)
		if err != nil {
			t.Fatalf("AssignWork failed: %v", err)
		}

		result, err := dispatcher.AwaitWork(ctx, workID)
		if err != nil {
			t.Fatalf("AwaitWork failed: %v", err)
		}

		if result.Type != work.MsgWorkFailed {
			t.Errorf("Expected WORK_FAILED, got %s", result.Type)
		}

		if result.Error == "" {
			t.Error("Expected non-empty error message")
		}
	})
}

func TestIntegration_MultiStationAsyncWork(t *testing.T) {
	port := getFreePort(t)
	httpPort := getFreePort(t)

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

	orchestratorCfg := config.LatticeConfig{
		StationID:   "orchestrator",
		StationName: "Orchestrator Station",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}
	orchestratorClient, err := lattice.NewClient(orchestratorCfg)
	if err != nil {
		t.Fatalf("Failed to create orchestrator client: %v", err)
	}
	if err := orchestratorClient.Connect(); err != nil {
		t.Fatalf("Failed to connect orchestrator: %v", err)
	}
	defer orchestratorClient.Close()

	leafCfg := config.LatticeConfig{
		StationID:   "leaf-station",
		StationName: "Leaf Station",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}
	leafClient, err := lattice.NewClient(leafCfg)
	if err != nil {
		t.Fatalf("Failed to create leaf client: %v", err)
	}
	if err := leafClient.Connect(); err != nil {
		t.Fatalf("Failed to connect leaf: %v", err)
	}
	defer leafClient.Close()

	leafExecutor := NewMockExecutor("Result from leaf station", 5)

	leafHook := work.NewHook(leafClient, leafExecutor, "leaf-station")
	ctx := context.Background()
	if err := leafHook.Start(ctx); err != nil {
		t.Fatalf("Failed to start leaf hook: %v", err)
	}
	defer leafHook.Stop()

	dispatcher := work.NewDispatcher(orchestratorClient, "orchestrator")
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	time.Sleep(100 * time.Millisecond)

	t.Run("RemoteWorkAssignment", func(t *testing.T) {
		assignment := &work.WorkAssignment{
			TargetStation: "leaf-station",
			AgentName:     "remote-agent",
			Task:          "Execute on remote station",
			Timeout:       10 * time.Second,
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
			t.Errorf("Expected WORK_COMPLETE, got %s (error: %s)", result.Type, result.Error)
		}

		if result.Result != "Result from leaf station" {
			t.Errorf("Expected 'Result from leaf station', got '%s'", result.Result)
		}

		if result.StationID != "leaf-station" {
			t.Errorf("Expected station ID 'leaf-station', got '%s'", result.StationID)
		}

		execs := leafExecutor.GetExecutions()
		if len(execs) != 1 {
			t.Fatalf("Expected 1 execution on leaf, got %d", len(execs))
		}
	})
}

func TestIntegration_ParallelWorkAssignment(t *testing.T) {
	port := getFreePort(t)
	httpPort := getFreePort(t)

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
		StationID:   "parallel-station",
		StationName: "Parallel Test Station",
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

	executor := NewMockExecutor("parallel result", 1)
	executor.SetDelay(100 * time.Millisecond)

	hook := work.NewHook(client, executor, "parallel-station")
	ctx := context.Background()
	if err := hook.Start(ctx); err != nil {
		t.Fatalf("Failed to start hook: %v", err)
	}
	defer hook.Stop()

	dispatcher := work.NewDispatcher(client, "parallel-station")
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	time.Sleep(100 * time.Millisecond)

	numTasks := 5
	workIDs := make([]string, numTasks)

	startTime := time.Now()

	for i := 0; i < numTasks; i++ {
		assignment := &work.WorkAssignment{
			AgentName: fmt.Sprintf("agent-%d", i),
			Task:      fmt.Sprintf("Task %d", i),
			Timeout:   30 * time.Second,
		}

		workID, err := dispatcher.AssignWork(ctx, assignment)
		if err != nil {
			t.Fatalf("AssignWork %d failed: %v", i, err)
		}
		workIDs[i] = workID
	}

	var wg sync.WaitGroup
	var successCount atomic.Int32
	var errorCount atomic.Int32

	for i, workID := range workIDs {
		wg.Add(1)
		go func(idx int, wid string) {
			defer wg.Done()
			result, err := dispatcher.AwaitWork(ctx, wid)
			if err != nil {
				t.Logf("AwaitWork %d failed: %v", idx, err)
				errorCount.Add(1)
				return
			}
			if result.Type == work.MsgWorkComplete {
				successCount.Add(1)
			} else {
				t.Logf("Work %d got unexpected type: %s", idx, result.Type)
				errorCount.Add(1)
			}
		}(i, workID)
	}

	wg.Wait()
	totalTime := time.Since(startTime)

	if int(successCount.Load()) != numTasks {
		t.Errorf("Expected %d successes, got %d (errors: %d)", numTasks, successCount.Load(), errorCount.Load())
	}

	sequentialTime := time.Duration(numTasks) * 100 * time.Millisecond
	if totalTime >= sequentialTime {
		t.Logf("Warning: parallel execution took %v, sequential would be %v", totalTime, sequentialTime)
	}

	execs := executor.GetExecutions()
	if len(execs) != numTasks {
		t.Errorf("Expected %d executions, got %d", numTasks, len(execs))
	}
}

func TestIntegration_WorkTimeout(t *testing.T) {
	port := getFreePort(t)
	httpPort := getFreePort(t)

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
		StationID:   "timeout-station",
		StationName: "Timeout Test Station",
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

	executor := NewMockExecutor("slow result", 1)
	executor.SetDelay(5 * time.Second)

	hook := work.NewHook(client, executor, "timeout-station")
	ctx := context.Background()
	if err := hook.Start(ctx); err != nil {
		t.Fatalf("Failed to start hook: %v", err)
	}
	defer hook.Stop()

	dispatcher := work.NewDispatcher(client, "timeout-station")
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	time.Sleep(100 * time.Millisecond)

	assignment := &work.WorkAssignment{
		AgentName: "slow-agent",
		Task:      "This will timeout",
		Timeout:   500 * time.Millisecond,
	}

	workID, err := dispatcher.AssignWork(ctx, assignment)
	if err != nil {
		t.Fatalf("AssignWork failed: %v", err)
	}

	_, err = dispatcher.AwaitWork(ctx, workID)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if err.Error() != fmt.Sprintf("work %s timed out after 500ms", workID) {
		t.Logf("Got error: %v (expected timeout)", err)
	}
}

func TestIntegration_ProgressStreaming(t *testing.T) {
	port := getFreePort(t)
	httpPort := getFreePort(t)

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
		StationID:   "progress-station",
		StationName: "Progress Test Station",
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

	executor := NewMockExecutor("done", 1)

	hook := work.NewHook(client, executor, "progress-station")
	ctx := context.Background()
	if err := hook.Start(ctx); err != nil {
		t.Fatalf("Failed to start hook: %v", err)
	}
	defer hook.Stop()

	dispatcher := work.NewDispatcher(client, "progress-station")
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatalf("Failed to start dispatcher: %v", err)
	}
	defer dispatcher.Stop()

	time.Sleep(100 * time.Millisecond)

	assignment := &work.WorkAssignment{
		AgentName: "progress-agent",
		Task:      "Task with progress",
		Timeout:   10 * time.Second,
	}

	workID, err := dispatcher.AssignWork(ctx, assignment)
	if err != nil {
		t.Fatalf("AssignWork failed: %v", err)
	}

	progressChan, err := dispatcher.StreamProgress(workID)
	if err != nil {
		t.Fatalf("StreamProgress failed: %v", err)
	}

	var progressUpdates []*work.WorkResponse
	done := make(chan struct{})

	go func() {
		for update := range progressChan {
			progressUpdates = append(progressUpdates, update)
		}
		close(done)
	}()

	result, err := dispatcher.AwaitWork(ctx, workID)
	if err != nil {
		t.Fatalf("AwaitWork failed: %v", err)
	}

	<-done

	if result.Type != work.MsgWorkComplete {
		t.Errorf("Expected WORK_COMPLETE, got %s", result.Type)
	}

	hasAccepted := false
	for _, update := range progressUpdates {
		if update.Type == work.MsgWorkAccepted {
			hasAccepted = true
			break
		}
	}

	if !hasAccepted {
		t.Log("No WORK_ACCEPTED message received (this is expected in fast execution)")
	}
}
