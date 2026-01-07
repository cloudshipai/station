package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/storage"
	"station/internal/workflows"
	"station/internal/workflows/runtime"
	"station/pkg/models"
)

// mockAgentService implements services.AgentServiceInterface for testing
type mockAgentService struct {
	mu           sync.Mutex
	executeCalls []mockExecuteCall
	agents       map[int64]*models.Agent
}

type mockExecuteCall struct {
	AgentID   int64
	AgentName string
	Task      string
	Variables map[string]interface{}
}

func newMockAgentService() *mockAgentService {
	return &mockAgentService{
		executeCalls: make([]mockExecuteCall, 0),
		agents:       make(map[int64]*models.Agent),
	}
}

func (m *mockAgentService) ExecuteAgent(ctx context.Context, agentID int64, task string, userVariables map[string]interface{}) (*services.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[agentID]
	agentName := ""
	if exists {
		agentName = agent.Name
	}

	m.executeCalls = append(m.executeCalls, mockExecuteCall{
		AgentID:   agentID,
		AgentName: agentName,
		Task:      task,
		Variables: userVariables,
	})

	return &services.Message{
		Content: fmt.Sprintf("Agent %d (%s) executed task: %s", agentID, agentName, task),
		Role:    services.RoleAssistant,
	}, nil
}

func (m *mockAgentService) ExecuteAgentWithRunID(ctx context.Context, agentID int64, task string, runID int64, userVariables map[string]interface{}) (*services.Message, error) {
	return m.ExecuteAgent(ctx, agentID, task, userVariables)
}

func (m *mockAgentService) CreateAgent(ctx context.Context, config *services.AgentConfig) (*models.Agent, error) {
	return nil, fmt.Errorf("not implemented in mock")
}

func (m *mockAgentService) GetAgent(ctx context.Context, agentID int64) (*models.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent not found: %d", agentID)
	}
	return agent, nil
}

func (m *mockAgentService) ListAgentsByEnvironment(ctx context.Context, environmentID int64) ([]*models.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []*models.Agent
	for _, agent := range m.agents {
		if agent.EnvironmentID == environmentID {
			result = append(result, agent)
		}
	}
	return result, nil
}

func (m *mockAgentService) UpdateAgent(ctx context.Context, agentID int64, config *services.AgentConfig) (*models.Agent, error) {
	return nil, fmt.Errorf("not implemented in mock")
}

func (m *mockAgentService) UpdateAgentPrompt(ctx context.Context, agentID int64, prompt string) error {
	return fmt.Errorf("not implemented in mock")
}

func (m *mockAgentService) DeleteAgent(ctx context.Context, agentID int64) error {
	return fmt.Errorf("not implemented in mock")
}

func (m *mockAgentService) SetFileStore(store storage.FileStore) {}

func (m *mockAgentService) SetSessionStore(store services.SessionStore) {}

func (m *mockAgentService) registerAgent(agent *models.Agent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[agent.ID] = agent
}

func (m *mockAgentService) getExecuteCalls() []mockExecuteCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]mockExecuteCall, len(m.executeCalls))
	copy(result, m.executeCalls)
	return result
}

// TestTwoAgentWorkflowE2E tests a workflow that executes two agents by name
func TestTwoAgentWorkflowE2E(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup test database
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()
	repos := repositories.New(testDB)

	// Create embedded NATS engine
	engine, err := runtime.NewEmbeddedEngineForTests()
	if err != nil {
		t.Fatalf("failed to start embedded engine: %v", err)
	}
	defer engine.Close()

	// Create test environment
	env, err := repos.Environments.Create("test-env", nil, 1)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	// Create two agents in the database
	triageAgent, err := repos.Agents.Create(
		"triage-agent",
		"Triage agent for incident analysis",
		"You are a triage agent",
		5, env.ID, 1, nil, nil, false, nil, nil, "", "",
	)
	if err != nil {
		t.Fatalf("failed to create triage agent: %v", err)
	}

	remediationAgent, err := repos.Agents.Create(
		"remediation-agent",
		"Remediation agent for fixing issues",
		"You are a remediation agent",
		5, env.ID, 1, nil, nil, false, nil, nil, "", "",
	)
	if err != nil {
		t.Fatalf("failed to create remediation agent: %v", err)
	}

	// Create mock agent service and register agents
	mockService := newMockAgentService()
	mockService.registerAgent(triageAgent)
	mockService.registerAgent(remediationAgent)

	// Create workflow service with engine
	workflowService := services.NewWorkflowServiceWithEngine(repos, engine)

	// Create minimal API handler directly (don't use NewAPIHandlers which starts its own consumer)
	handler := &APIHandlers{
		repos:                repos,
		db:                   testDB.Conn(),
		agentService:         mockService,
		toolDiscoveryService: services.NewToolDiscoveryService(repos),
		agentExportService:   services.NewAgentExportService(repos),
		workflowService:      workflowService,
		workflowEngine:       engine,
		localMode:            true,
	}

	// Start workflow consumer with mock agent service
	startTestWorkflowConsumer(t, engine, repos, mockService)

	router := gin.New()
	apiGroup := router.Group("/api/v1")
	handler.RegisterRoutes(apiGroup)

	// Subscribe to workflow completion events
	completionCh := make(chan string, 10)
	sub, err := engine.SubscribeDurable("workflow.run.*.event", "test-completion-consumer", func(msg *nats.Msg) {
		completionCh <- string(msg.Data)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer sub.Unsubscribe()

	// Create workflow definition with two agent steps referencing agents by name
	workflowDef := map[string]interface{}{
		"workflowId": "two-agent-workflow",
		"name":       "Two Agent Workflow",
		"definition": map[string]interface{}{
			"id":    "two-agent-workflow",
			"start": "triage",
			"states": []map[string]interface{}{
				{
					"id":   "triage",
					"type": "operation",
					"input": map[string]interface{}{
						"task":       "agent.run",
						"agent":      "triage-agent", // Reference by name
						"agent_task": "Analyze the incident and provide initial assessment",
					},
					"transition": "remediate",
				},
				{
					"id":   "remediate",
					"type": "operation",
					"input": map[string]interface{}{
						"task":       "agent.run",
						"agent":      "remediation-agent", // Reference by name
						"agent_task": "Apply the fix based on triage assessment",
					},
				},
			},
		},
	}

	// Create workflow via API
	createBody, _ := json.Marshal(workflowDef)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewBuffer(createBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	// Start workflow run with environment ID
	startPayload := map[string]interface{}{
		"workflowId":    "two-agent-workflow",
		"environmentId": env.ID,
	}
	startBody, _ := json.Marshal(startPayload)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/workflow-runs", bytes.NewBuffer(startBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("start run failed: status=%d body=%s", w.Code, w.Body.String())
	}

	var runResp struct {
		Run struct {
			RunID  string `json:"run_id"`
			Status string `json:"status"`
		} `json:"run"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("failed to parse run response: %v", err)
	}

	runID := runResp.Run.RunID
	if runID == "" {
		t.Fatalf("run id should not be empty")
	}

	// Wait for workflow completion (both steps should execute)
	timeout := time.After(10 * time.Second)
	completed := false

	for !completed {
		select {
		case eventData := <-completionCh:
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(eventData), &event); err == nil {
				if eventType, ok := event["type"].(string); ok && eventType == "run_completed" {
					completed = true
				}
			}
		case <-timeout:
			// Check run status directly
			w = httptest.NewRecorder()
			req, _ = http.NewRequest(http.MethodGet, "/api/v1/workflow-runs/"+runID, nil)
			router.ServeHTTP(w, req)

			var statusResp struct {
				Run struct {
					Status string `json:"status"`
				} `json:"run"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &statusResp)

			if statusResp.Run.Status == "completed" {
				completed = true
			} else if statusResp.Run.Status == "failed" {
				t.Fatalf("workflow failed: %s", w.Body.String())
			} else {
				t.Fatalf("timeout waiting for workflow completion, current status: %s", statusResp.Run.Status)
			}
		}
	}

	// Verify both agents were called
	calls := mockService.getExecuteCalls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 agent calls, got %d: %+v", len(calls), calls)
	}

	// Verify first call was to triage agent
	if calls[0].AgentID != triageAgent.ID {
		t.Errorf("first call should be to triage agent (ID %d), got %d", triageAgent.ID, calls[0].AgentID)
	}
	if calls[0].AgentName != "triage-agent" {
		t.Errorf("first call agent name should be 'triage-agent', got %s", calls[0].AgentName)
	}

	// Verify second call was to remediation agent
	if calls[1].AgentID != remediationAgent.ID {
		t.Errorf("second call should be to remediation agent (ID %d), got %d", remediationAgent.ID, calls[1].AgentID)
	}
	if calls[1].AgentName != "remediation-agent" {
		t.Errorf("second call agent name should be 'remediation-agent', got %s", calls[1].AgentName)
	}

	t.Logf("Successfully executed 2-agent workflow:")
	t.Logf("  - Agent 1: %s (ID: %d) - Task: %s", calls[0].AgentName, calls[0].AgentID, calls[0].Task)
	t.Logf("  - Agent 2: %s (ID: %d) - Task: %s", calls[1].AgentName, calls[1].AgentID, calls[1].Task)
}

// startTestWorkflowConsumer creates and starts a workflow consumer with mock agent service
func startTestWorkflowConsumer(t *testing.T, engine runtime.Engine, repos *repositories.Repositories, mockService *mockAgentService) {
	natsEngine, ok := engine.(*runtime.NATSEngine)
	if !ok || natsEngine == nil {
		t.Fatal("expected NATS engine for test")
	}

	registry := runtime.NewExecutorRegistry()
	registry.Register(runtime.NewInjectExecutor())
	registry.Register(runtime.NewSwitchExecutor())
	registry.Register(runtime.NewAgentRunExecutor(&testAgentExecutorAdapter{
		mockService: mockService,
		repos:       repos,
	}))
	registry.Register(runtime.NewHumanApprovalExecutor(&approvalExecutorAdapter{repos: repos}))
	registry.Register(runtime.NewCustomExecutor(nil))
	registry.Register(runtime.NewCronExecutor())
	registry.Register(runtime.NewTimerExecutor())
	registry.Register(runtime.NewTryCatchExecutor(registry))
	registry.Register(runtime.NewTransformExecutor())

	stepAdapter := &testRegistryStepExecutorAdapter{registry: registry}
	registry.Register(runtime.NewParallelExecutor(stepAdapter))
	registry.Register(runtime.NewForeachExecutor(stepAdapter))

	adapter := runtime.NewWorkflowServiceAdapter(repos, engine)
	consumer := runtime.NewWorkflowConsumer(natsEngine, registry, adapter, adapter, adapter)

	if err := consumer.Start(context.Background()); err != nil {
		t.Fatalf("failed to start workflow consumer: %v", err)
	}

	t.Cleanup(func() {
		consumer.Stop()
	})
}

type testRegistryStepExecutorAdapter struct {
	registry *runtime.ExecutorRegistry
}

func (a *testRegistryStepExecutorAdapter) ExecuteStep(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (runtime.StepResult, error) {
	executor, err := a.registry.GetExecutor(step.Type)
	if err != nil {
		errStr := err.Error()
		return runtime.StepResult{
			Status: runtime.StepStatusFailed,
			Error:  &errStr,
		}, err
	}
	return executor.Execute(ctx, step, runContext)
}

// testAgentExecutorAdapter implements runtime.AgentExecutorDeps using mock service
type testAgentExecutorAdapter struct {
	mockService *mockAgentService
	repos       *repositories.Repositories
}

func (a *testAgentExecutorAdapter) GetAgentByID(ctx context.Context, id int64) (runtime.AgentInfo, error) {
	agent, err := a.mockService.GetAgent(ctx, id)
	if err != nil {
		return runtime.AgentInfo{}, err
	}
	return runtime.AgentInfo{
		ID:           agent.ID,
		Name:         agent.Name,
		InputSchema:  agent.InputSchema,
		OutputSchema: agent.OutputSchema,
	}, nil
}

func (a *testAgentExecutorAdapter) GetAgentByNameAndEnvironment(ctx context.Context, name string, environmentID int64) (runtime.AgentInfo, error) {
	agent, err := a.repos.Agents.GetByNameAndEnvironment(name, environmentID)
	if err != nil {
		return runtime.AgentInfo{}, err
	}

	a.mockService.registerAgent(agent)

	return runtime.AgentInfo{
		ID:           agent.ID,
		Name:         agent.Name,
		InputSchema:  agent.InputSchema,
		OutputSchema: agent.OutputSchema,
	}, nil
}

func (a *testAgentExecutorAdapter) GetAgentByNameGlobal(ctx context.Context, name string) (runtime.AgentInfo, error) {
	agent, err := a.repos.Agents.GetByNameGlobal(name)
	if err != nil {
		return runtime.AgentInfo{}, err
	}

	a.mockService.registerAgent(agent)

	return runtime.AgentInfo{
		ID:           agent.ID,
		Name:         agent.Name,
		InputSchema:  agent.InputSchema,
		OutputSchema: agent.OutputSchema,
	}, nil
}

func (a *testAgentExecutorAdapter) GetEnvironmentIDByName(ctx context.Context, name string) (int64, error) {
	env, err := a.repos.Environments.GetByName(name)
	if err != nil {
		return 0, err
	}
	return env.ID, nil
}

func (a *testAgentExecutorAdapter) ExecuteAgent(ctx context.Context, agentID int64, task string, variables map[string]interface{}) (runtime.AgentExecutionResult, error) {
	result, err := a.mockService.ExecuteAgent(ctx, agentID, task, variables)
	if err != nil {
		return runtime.AgentExecutionResult{}, err
	}
	return runtime.AgentExecutionResult{
		Response:  result.Content,
		StepCount: 1,
		ToolsUsed: 0,
	}, nil
}
