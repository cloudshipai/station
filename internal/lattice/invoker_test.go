package lattice

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"station/internal/config"
)

type MockAgentExecutor struct {
	mu           sync.Mutex
	executions   []MockExecution
	returnResult string
	returnCalls  int
	returnError  error
}

type MockExecution struct {
	AgentID   string
	AgentName string
	Task      string
	Timestamp time.Time
}

func NewMockAgentExecutor() *MockAgentExecutor {
	return &MockAgentExecutor{
		returnResult: "mock result",
		returnCalls:  1,
	}
}

func (m *MockAgentExecutor) SetResult(result string, calls int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.returnResult = result
	m.returnCalls = calls
	m.returnError = err
}

func (m *MockAgentExecutor) GetExecutions() []MockExecution {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MockExecution{}, m.executions...)
}

func (m *MockAgentExecutor) ExecuteAgentByID(ctx context.Context, agentID string, task string) (string, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executions = append(m.executions, MockExecution{
		AgentID:   agentID,
		Task:      task,
		Timestamp: time.Now(),
	})
	return m.returnResult, m.returnCalls, m.returnError
}

func (m *MockAgentExecutor) ExecuteAgentByName(ctx context.Context, agentName string, task string) (string, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executions = append(m.executions, MockExecution{
		AgentName: agentName,
		Task:      task,
		Timestamp: time.Now(),
	})
	return m.returnResult, m.returnCalls, m.returnError
}

type MockWorkflowExecutor struct {
	mu           sync.Mutex
	executions   []MockWorkflowExecution
	returnRunID  string
	returnStatus string
	returnError  error
}

type MockWorkflowExecution struct {
	WorkflowID string
	Input      map[string]string
	Timestamp  time.Time
}

func NewMockWorkflowExecutor() *MockWorkflowExecutor {
	return &MockWorkflowExecutor{
		returnRunID:  "run-123",
		returnStatus: "started",
	}
}

func (m *MockWorkflowExecutor) SetResult(runID, status string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.returnRunID = runID
	m.returnStatus = status
	m.returnError = err
}

func (m *MockWorkflowExecutor) GetExecutions() []MockWorkflowExecution {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MockWorkflowExecution{}, m.executions...)
}

func (m *MockWorkflowExecutor) ExecuteWorkflow(ctx context.Context, workflowID string, input map[string]string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executions = append(m.executions, MockWorkflowExecution{
		WorkflowID: workflowID,
		Input:      input,
		Timestamp:  time.Now(),
	})
	return m.returnRunID, m.returnStatus, m.returnError
}

func TestInvoker_RemoteAgentExecution(t *testing.T) {
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

	targetCfg := config.LatticeConfig{
		StationID:   "target-station",
		StationName: "Target Station",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	targetClient, err := NewClient(targetCfg)
	if err != nil {
		t.Fatalf("Failed to create target client: %v", err)
	}
	if err := targetClient.Connect(); err != nil {
		t.Fatalf("Failed to connect target: %v", err)
	}
	defer targetClient.Close()

	mockExecutor := NewMockAgentExecutor()
	mockExecutor.SetResult("Hello from target agent!", 3, nil)

	targetInvoker := NewInvoker(targetClient, "target-station", mockExecutor)
	ctx := context.Background()
	if err := targetInvoker.Start(ctx); err != nil {
		t.Fatalf("Failed to start target invoker: %v", err)
	}
	defer targetInvoker.Stop()

	callerCfg := config.LatticeConfig{
		StationID:   "caller-station",
		StationName: "Caller Station",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	callerClient, err := NewClient(callerCfg)
	if err != nil {
		t.Fatalf("Failed to create caller client: %v", err)
	}
	if err := callerClient.Connect(); err != nil {
		t.Fatalf("Failed to connect caller: %v", err)
	}
	defer callerClient.Close()

	callerInvoker := NewInvoker(callerClient, "caller-station", nil)

	t.Run("InvokeByAgentID", func(t *testing.T) {
		req := InvokeAgentRequest{
			AgentID: "42",
			Task:    "What is the meaning of life?",
		}

		resp, err := callerInvoker.InvokeRemoteAgent(ctx, "target-station", req)
		if err != nil {
			t.Fatalf("Remote invocation failed: %v", err)
		}

		if resp.Status != "success" {
			t.Errorf("Status = %v, want success", resp.Status)
		}

		if resp.Result != "Hello from target agent!" {
			t.Errorf("Result = %v, want 'Hello from target agent!'", resp.Result)
		}

		if resp.ToolCalls != 3 {
			t.Errorf("ToolCalls = %d, want 3", resp.ToolCalls)
		}

		if resp.StationID != "target-station" {
			t.Errorf("StationID = %v, want target-station", resp.StationID)
		}

		executions := mockExecutor.GetExecutions()
		if len(executions) != 1 {
			t.Fatalf("Expected 1 execution, got %d", len(executions))
		}

		if executions[0].AgentID != "42" {
			t.Errorf("Executed AgentID = %v, want 42", executions[0].AgentID)
		}

		if executions[0].Task != "What is the meaning of life?" {
			t.Errorf("Executed Task = %v, want 'What is the meaning of life?'", executions[0].Task)
		}
	})

	t.Run("InvokeByAgentName", func(t *testing.T) {
		req := InvokeAgentRequest{
			AgentName: "coder-agent",
			Task:      "Write hello world",
		}

		resp, err := callerInvoker.InvokeRemoteAgent(ctx, "target-station", req)
		if err != nil {
			t.Fatalf("Remote invocation failed: %v", err)
		}

		if resp.Status != "success" {
			t.Errorf("Status = %v, want success", resp.Status)
		}

		executions := mockExecutor.GetExecutions()
		lastExec := executions[len(executions)-1]
		if lastExec.AgentName != "coder-agent" {
			t.Errorf("Executed AgentName = %v, want coder-agent", lastExec.AgentName)
		}
	})

	t.Run("InvokeWithError", func(t *testing.T) {
		mockExecutor.SetResult("", 0, fmt.Errorf("agent crashed"))

		req := InvokeAgentRequest{
			AgentID: "99",
			Task:    "This will fail",
		}

		resp, err := callerInvoker.InvokeRemoteAgent(ctx, "target-station", req)
		if err != nil {
			t.Fatalf("Request should succeed even if agent fails: %v", err)
		}

		if resp.Status != "error" {
			t.Errorf("Status = %v, want error", resp.Status)
		}

		if resp.Error == "" {
			t.Error("Error message should not be empty")
		}

		mockExecutor.SetResult("Hello from target agent!", 3, nil)
	})
}

func TestInvoker_RemoteWorkflowExecution(t *testing.T) {
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

	targetCfg := config.LatticeConfig{
		StationID:   "workflow-target",
		StationName: "Workflow Target",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	targetClient, err := NewClient(targetCfg)
	if err != nil {
		t.Fatalf("Failed to create target client: %v", err)
	}
	if err := targetClient.Connect(); err != nil {
		t.Fatalf("Failed to connect target: %v", err)
	}
	defer targetClient.Close()

	mockAgentExecutor := NewMockAgentExecutor()
	mockWorkflowExecutor := NewMockWorkflowExecutor()
	mockWorkflowExecutor.SetResult("run-abc-123", "running", nil)

	targetInvoker := NewInvoker(targetClient, "workflow-target", mockAgentExecutor)
	targetInvoker.SetWorkflowExecutor(mockWorkflowExecutor)

	ctx := context.Background()
	if err := targetInvoker.Start(ctx); err != nil {
		t.Fatalf("Failed to start target invoker: %v", err)
	}
	defer targetInvoker.Stop()

	callerCfg := config.LatticeConfig{
		StationID:   "workflow-caller",
		StationName: "Workflow Caller",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	callerClient, err := NewClient(callerCfg)
	if err != nil {
		t.Fatalf("Failed to create caller client: %v", err)
	}
	if err := callerClient.Connect(); err != nil {
		t.Fatalf("Failed to connect caller: %v", err)
	}
	defer callerClient.Close()

	callerInvoker := NewInvoker(callerClient, "workflow-caller", nil)

	t.Run("InvokeWorkflow", func(t *testing.T) {
		req := RunWorkflowRequest{
			WorkflowID: "deploy-pipeline",
			Input: map[string]string{
				"environment": "production",
				"version":     "1.2.3",
			},
		}

		resp, err := callerInvoker.InvokeRemoteWorkflow(ctx, "workflow-target", req)
		if err != nil {
			t.Fatalf("Remote workflow invocation failed: %v", err)
		}

		if resp.Status != "running" {
			t.Errorf("Status = %v, want running", resp.Status)
		}

		if resp.RunID != "run-abc-123" {
			t.Errorf("RunID = %v, want run-abc-123", resp.RunID)
		}

		if resp.StationID != "workflow-target" {
			t.Errorf("StationID = %v, want workflow-target", resp.StationID)
		}

		executions := mockWorkflowExecutor.GetExecutions()
		if len(executions) != 1 {
			t.Fatalf("Expected 1 workflow execution, got %d", len(executions))
		}

		if executions[0].WorkflowID != "deploy-pipeline" {
			t.Errorf("WorkflowID = %v, want deploy-pipeline", executions[0].WorkflowID)
		}

		if executions[0].Input["environment"] != "production" {
			t.Errorf("Input[environment] = %v, want production", executions[0].Input["environment"])
		}
	})

	t.Run("InvokeWorkflowWithError", func(t *testing.T) {
		mockWorkflowExecutor.SetResult("", "", fmt.Errorf("workflow validation failed"))

		req := RunWorkflowRequest{
			WorkflowID: "invalid-workflow",
		}

		resp, err := callerInvoker.InvokeRemoteWorkflow(ctx, "workflow-target", req)
		if err != nil {
			t.Fatalf("Request should succeed even if workflow fails: %v", err)
		}

		if resp.Status != "error" {
			t.Errorf("Status = %v, want error", resp.Status)
		}

		if resp.Error == "" {
			t.Error("Error message should not be empty")
		}
	})
}

func TestInvoker_ConcurrentRequests(t *testing.T) {
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

	targetCfg := config.LatticeConfig{
		StationID:   "concurrent-target",
		StationName: "Concurrent Target",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	targetClient, err := NewClient(targetCfg)
	if err != nil {
		t.Fatalf("Failed to create target client: %v", err)
	}
	if err := targetClient.Connect(); err != nil {
		t.Fatalf("Failed to connect target: %v", err)
	}
	defer targetClient.Close()

	mockExecutor := NewMockAgentExecutor()
	targetInvoker := NewInvoker(targetClient, "concurrent-target", mockExecutor)

	ctx := context.Background()
	if err := targetInvoker.Start(ctx); err != nil {
		t.Fatalf("Failed to start target invoker: %v", err)
	}
	defer targetInvoker.Stop()

	callerCfg := config.LatticeConfig{
		StationID:   "concurrent-caller",
		StationName: "Concurrent Caller",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	callerClient, err := NewClient(callerCfg)
	if err != nil {
		t.Fatalf("Failed to create caller client: %v", err)
	}
	if err := callerClient.Connect(); err != nil {
		t.Fatalf("Failed to connect caller: %v", err)
	}
	defer callerClient.Close()

	callerInvoker := NewInvoker(callerClient, "concurrent-caller", nil)

	numRequests := 10
	var wg sync.WaitGroup
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			req := InvokeAgentRequest{
				AgentID: fmt.Sprintf("agent-%d", idx),
				Task:    fmt.Sprintf("Task %d", idx),
			}

			resp, err := callerInvoker.InvokeRemoteAgent(ctx, "concurrent-target", req)
			if err != nil {
				errors <- fmt.Errorf("request %d failed: %w", idx, err)
				return
			}

			if resp.Status != "success" {
				errors <- fmt.Errorf("request %d got status %s: %s", idx, resp.Status, resp.Error)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	executions := mockExecutor.GetExecutions()
	if len(executions) != numRequests {
		t.Errorf("Expected %d executions, got %d", numRequests, len(executions))
	}
}

func TestInvoker_InvalidRequests(t *testing.T) {
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

	targetCfg := config.LatticeConfig{
		StationID:   "invalid-target",
		StationName: "Invalid Target",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	targetClient, err := NewClient(targetCfg)
	if err != nil {
		t.Fatalf("Failed to create target client: %v", err)
	}
	if err := targetClient.Connect(); err != nil {
		t.Fatalf("Failed to connect target: %v", err)
	}
	defer targetClient.Close()

	mockExecutor := NewMockAgentExecutor()
	targetInvoker := NewInvoker(targetClient, "invalid-target", mockExecutor)

	ctx := context.Background()
	if err := targetInvoker.Start(ctx); err != nil {
		t.Fatalf("Failed to start target invoker: %v", err)
	}
	defer targetInvoker.Stop()

	t.Run("MissingAgentIDAndName", func(t *testing.T) {
		subject := fmt.Sprintf("lattice.station.%s.agent.invoke", "invalid-target")
		req := InvokeAgentRequest{
			Task: "No agent specified",
		}

		data, _ := json.Marshal(req)
		resp, err := targetClient.Request(subject, data, 5*time.Second)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		var response InvokeAgentResponse
		if err := json.Unmarshal(resp.Data, &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Status != "error" {
			t.Errorf("Status = %v, want error", response.Status)
		}

		if response.Error == "" {
			t.Error("Expected error message")
		}
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		subject := fmt.Sprintf("lattice.station.%s.agent.invoke", "invalid-target")

		resp, err := targetClient.Request(subject, []byte("not valid json"), 5*time.Second)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		var response InvokeAgentResponse
		if err := json.Unmarshal(resp.Data, &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Status != "error" {
			t.Errorf("Status = %v, want error", response.Status)
		}
	})

	t.Run("NoWorkflowExecutor", func(t *testing.T) {
		subject := fmt.Sprintf("lattice.station.%s.workflow.run", "invalid-target")
		req := RunWorkflowRequest{
			WorkflowID: "some-workflow",
		}

		data, _ := json.Marshal(req)
		resp, err := targetClient.Request(subject, data, 5*time.Second)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		var response RunWorkflowResponse
		if err := json.Unmarshal(resp.Data, &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response.Status != "error" {
			t.Errorf("Status = %v, want error", response.Status)
		}

		if response.Error == "" {
			t.Error("Expected error about workflow executor not configured")
		}
	})
}
