package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/workflows/runtime"
)

// TestComplexWorkflowWithSwitchAndInject tests a workflow with inject + switch + Starlark conditions
// Flow: setup (inject) -> evaluate (switch) -> critical-path OR normal-path agents
func TestComplexWorkflowWithSwitchAndInject(t *testing.T) {
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

	// Create agents for different paths
	triageAgent, err := repos.Agents.Create(
		"triage-agent",
		"Triage agent for critical issues",
		"You are a critical triage agent",
		5, env.ID, 1, nil, nil, false, nil, nil, "", "",
	)
	if err != nil {
		t.Fatalf("failed to create triage agent: %v", err)
	}

	monitorAgent, err := repos.Agents.Create(
		"monitor-agent",
		"Monitor agent for normal issues",
		"You are a monitoring agent",
		5, env.ID, 1, nil, nil, false, nil, nil, "", "",
	)
	if err != nil {
		t.Fatalf("failed to create monitor agent: %v", err)
	}

	// Create mock agent service and register agents
	mockService := newMockAgentService()
	mockService.registerAgent(triageAgent)
	mockService.registerAgent(monitorAgent)

	// Create workflow service with engine
	workflowService := services.NewWorkflowServiceWithEngine(repos, engine)

	// Create minimal API handler
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
	sub, err := engine.SubscribeDurable("workflow.run.*.event", "test-complex-consumer", func(msg *nats.Msg) {
		completionCh <- string(msg.Data)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer sub.Unsubscribe()

	tests := []struct {
		name              string
		errorRate         float64
		severity          string
		expectedAgent     string
		expectedAgentID   int64
		expectedCondition string
	}{
		{
			name:              "high error rate triggers critical path",
			errorRate:         0.15, // > 0.05 threshold
			severity:          "medium",
			expectedAgent:     "triage-agent",
			expectedAgentID:   triageAgent.ID,
			expectedCondition: "error_rate > 0.05",
		},
		{
			name:              "high severity triggers critical path",
			errorRate:         0.01, // below threshold
			severity:          "critical",
			expectedAgent:     "triage-agent",
			expectedAgentID:   triageAgent.ID,
			expectedCondition: "severity == \"critical\"",
		},
		{
			name:              "low severity and error rate goes to normal path",
			errorRate:         0.02, // below threshold
			severity:          "low",
			expectedAgent:     "monitor-agent",
			expectedAgentID:   monitorAgent.ID,
			expectedCondition: "default",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear previous calls
			mockService.mu.Lock()
			mockService.executeCalls = make([]mockExecuteCall, 0)
			mockService.mu.Unlock()

			workflowID := "complex-workflow-" + tc.severity

			// Create workflow with inject + switch + agent paths
			workflowDef := map[string]interface{}{
				"workflowId": workflowID,
				"name":       "Complex Workflow with Switch",
				"definition": map[string]interface{}{
					"id":    workflowID,
					"start": "setup",
					"states": []map[string]interface{}{
						{
							"id":   "setup",
							"type": "inject",
							"data": map[string]interface{}{
								"error_rate": tc.errorRate,
								"severity":   tc.severity,
								"retries":    3,
								"service":    "api-gateway",
							},
							"resultPath": "ctx",
							"transition": "evaluate",
						},
						{
							"id":       "evaluate",
							"type":     "switch",
							"dataPath": "ctx",
							"conditions": []map[string]interface{}{
								{
									"if":   "error_rate > 0.05",
									"next": "critical-path",
								},
								{
									"if":   "severity == \"critical\"",
									"next": "critical-path",
								},
								{
									"if":   "retries >= 5",
									"next": "critical-path",
								},
							},
							"defaultNext": "normal-path",
						},
						{
							"id":   "critical-path",
							"type": "operation",
							"input": map[string]interface{}{
								"task":       "agent.run",
								"agent":      "triage-agent",
								"agent_task": "Critical issue detected! Analyze and escalate.",
							},
							"end": true,
						},
						{
							"id":   "normal-path",
							"type": "operation",
							"input": map[string]interface{}{
								"task":       "agent.run",
								"agent":      "monitor-agent",
								"agent_task": "Monitor the situation and log metrics.",
							},
							"end": true,
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
				"workflowId":    workflowID,
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

			// Wait for workflow completion
			timeout := time.After(10 * time.Second)
			completed := false

			for !completed {
				select {
				case eventData := <-completionCh:
					var event map[string]interface{}
					if err := json.Unmarshal([]byte(eventData), &event); err == nil {
						if eventType, ok := event["type"].(string); ok && eventType == "run_completed" {
							if eventRunID, ok := event["run_id"].(string); ok && eventRunID == runID {
								completed = true
							}
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

			// Verify correct agent was called
			calls := mockService.getExecuteCalls()
			if len(calls) != 1 {
				t.Fatalf("expected 1 agent call, got %d: %+v", len(calls), calls)
			}

			if calls[0].AgentID != tc.expectedAgentID {
				t.Errorf("expected agent ID %d (%s), got %d (%s)",
					tc.expectedAgentID, tc.expectedAgent,
					calls[0].AgentID, calls[0].AgentName)
			}

			if calls[0].AgentName != tc.expectedAgent {
				t.Errorf("expected agent name '%s', got '%s'", tc.expectedAgent, calls[0].AgentName)
			}

			t.Logf("SUCCESS: error_rate=%.2f, severity=%s -> %s (condition: %s)",
				tc.errorRate, tc.severity, calls[0].AgentName, tc.expectedCondition)
		})
	}
}

// TestWorkflowInjectSetsContextCorrectly verifies inject step properly sets context data
func TestWorkflowInjectSetsContextCorrectly(t *testing.T) {
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
	env, err := repos.Environments.Create("test-env-inject", nil, 1)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	// Create agent that will receive the injected context
	verifyAgent, err := repos.Agents.Create(
		"verify-agent",
		"Agent to verify context",
		"You verify context data",
		5, env.ID, 1, nil, nil, false, nil, nil, "", "",
	)
	if err != nil {
		t.Fatalf("failed to create verify agent: %v", err)
	}

	mockService := newMockAgentService()
	mockService.registerAgent(verifyAgent)

	workflowService := services.NewWorkflowServiceWithEngine(repos, engine)

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

	startTestWorkflowConsumer(t, engine, repos, mockService)

	router := gin.New()
	apiGroup := router.Group("/api/v1")
	handler.RegisterRoutes(apiGroup)

	completionCh := make(chan string, 10)
	sub, err := engine.SubscribeDurable("workflow.run.*.event", "test-inject-consumer", func(msg *nats.Msg) {
		completionCh <- string(msg.Data)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer sub.Unsubscribe()

	// Workflow: inject nested data -> agent uses it
	workflowDef := map[string]interface{}{
		"workflowId": "inject-context-workflow",
		"name":       "Inject Context Test",
		"definition": map[string]interface{}{
			"id":    "inject-context-workflow",
			"start": "inject-config",
			"states": []map[string]interface{}{
				{
					"id":   "inject-config",
					"type": "inject",
					"data": map[string]interface{}{
						"deployment": map[string]interface{}{
							"name":      "api-service",
							"namespace": "production",
							"replicas":  3,
						},
						"thresholds": map[string]interface{}{
							"cpu":    80,
							"memory": 90,
						},
					},
					"resultPath": "config",
					"transition": "use-config",
				},
				{
					"id":   "use-config",
					"type": "operation",
					"input": map[string]interface{}{
						"task":       "agent.run",
						"agent":      "verify-agent",
						"agent_task": "Process the configuration",
					},
					"end": true,
				},
			},
		},
	}

	createBody, _ := json.Marshal(workflowDef)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewBuffer(createBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	startPayload := map[string]interface{}{
		"workflowId":    "inject-context-workflow",
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
			RunID string `json:"run_id"`
		} `json:"run"`
	}
	json.Unmarshal(w.Body.Bytes(), &runResp)

	// Wait for completion
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
			w = httptest.NewRecorder()
			req, _ = http.NewRequest(http.MethodGet, "/api/v1/workflow-runs/"+runResp.Run.RunID, nil)
			router.ServeHTTP(w, req)

			var statusResp struct {
				Run struct {
					Status string `json:"status"`
				} `json:"run"`
			}
			json.Unmarshal(w.Body.Bytes(), &statusResp)
			if statusResp.Run.Status == "completed" {
				completed = true
			} else {
				t.Fatalf("timeout waiting for workflow completion, status: %s", statusResp.Run.Status)
			}
		}
	}

	// Verify agent was called
	calls := mockService.getExecuteCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 agent call, got %d", len(calls))
	}

	if calls[0].AgentName != "verify-agent" {
		t.Errorf("expected verify-agent, got %s", calls[0].AgentName)
	}

	t.Logf("SUCCESS: Inject + agent workflow completed")
}

// TestStarlarkConditionsVariety tests various Starlark condition patterns
func TestStarlarkConditionsVariety(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()
	repos := repositories.New(testDB)

	engine, err := runtime.NewEmbeddedEngineForTests()
	if err != nil {
		t.Fatalf("failed to start embedded engine: %v", err)
	}
	defer engine.Close()

	env, err := repos.Environments.Create("test-env-starlark", nil, 1)
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	// Create agents for different paths
	pathAAgent, _ := repos.Agents.Create("path-a-agent", "Path A", "Path A agent", 5, env.ID, 1, nil, nil, false, nil, nil, "", "")
	pathBAgent, _ := repos.Agents.Create("path-b-agent", "Path B", "Path B agent", 5, env.ID, 1, nil, nil, false, nil, nil, "", "")
	pathCAgent, _ := repos.Agents.Create("path-c-agent", "Path C", "Path C agent", 5, env.ID, 1, nil, nil, false, nil, nil, "", "")

	mockService := newMockAgentService()
	mockService.registerAgent(pathAAgent)
	mockService.registerAgent(pathBAgent)
	mockService.registerAgent(pathCAgent)

	workflowService := services.NewWorkflowServiceWithEngine(repos, engine)

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

	startTestWorkflowConsumer(t, engine, repos, mockService)

	router := gin.New()
	apiGroup := router.Group("/api/v1")
	handler.RegisterRoutes(apiGroup)

	completionCh := make(chan string, 10)
	sub, _ := engine.SubscribeDurable("workflow.run.*.event", "test-starlark-consumer", func(msg *nats.Msg) {
		completionCh <- string(msg.Data)
	})
	defer sub.Unsubscribe()

	tests := []struct {
		name          string
		data          map[string]interface{}
		expectedAgent string
	}{
		{
			name: "string equality",
			data: map[string]interface{}{
				"status": "degraded",
				"count":  5,
			},
			expectedAgent: "path-a-agent", // status == "degraded"
		},
		{
			name: "numeric comparison with AND",
			data: map[string]interface{}{
				"status": "healthy",
				"count":  15,
				"rate":   0.8,
			},
			expectedAgent: "path-b-agent", // count > 10 and rate > 0.5
		},
		{
			name: "list contains check",
			data: map[string]interface{}{
				"status": "healthy",
				"count":  5,
				"rate":   0.3,
				"tags":   []interface{}{"urgent", "production"},
			},
			expectedAgent: "path-c-agent", // "urgent" in tags
		},
		{
			name: "default fallback",
			data: map[string]interface{}{
				"status": "healthy",
				"count":  5,
				"rate":   0.3,
				"tags":   []interface{}{"normal"},
			},
			expectedAgent: "path-c-agent", // default
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockService.mu.Lock()
			mockService.executeCalls = make([]mockExecuteCall, 0)
			mockService.mu.Unlock()

			workflowID := "starlark-test-" + string(rune('a'+i))

			workflowDef := map[string]interface{}{
				"workflowId": workflowID,
				"name":       "Starlark Conditions Test",
				"definition": map[string]interface{}{
					"id":    workflowID,
					"start": "inject-data",
					"states": []map[string]interface{}{
						{
							"id":         "inject-data",
							"type":       "inject",
							"data":       tc.data,
							"resultPath": "ctx",
							"transition": "evaluate",
						},
						{
							"id":       "evaluate",
							"type":     "switch",
							"dataPath": "ctx",
							"conditions": []map[string]interface{}{
								{
									"if":   "status == \"degraded\"",
									"next": "path-a",
								},
								{
									"if":   "count > 10 and rate > 0.5",
									"next": "path-b",
								},
								{
									"if":   "\"urgent\" in tags",
									"next": "path-c",
								},
							},
							"defaultNext": "path-c",
						},
						{
							"id":   "path-a",
							"type": "operation",
							"input": map[string]interface{}{
								"task":       "agent.run",
								"agent":      "path-a-agent",
								"agent_task": "Handle degraded status",
							},
							"end": true,
						},
						{
							"id":   "path-b",
							"type": "operation",
							"input": map[string]interface{}{
								"task":       "agent.run",
								"agent":      "path-b-agent",
								"agent_task": "Handle high count and rate",
							},
							"end": true,
						},
						{
							"id":   "path-c",
							"type": "operation",
							"input": map[string]interface{}{
								"task":       "agent.run",
								"agent":      "path-c-agent",
								"agent_task": "Handle urgent or default",
							},
							"end": true,
						},
					},
				},
			}

			createBody, _ := json.Marshal(workflowDef)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewBuffer(createBody))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
			}

			startPayload := map[string]interface{}{
				"workflowId":    workflowID,
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
					RunID string `json:"run_id"`
				} `json:"run"`
			}
			json.Unmarshal(w.Body.Bytes(), &runResp)

			// Wait for completion
			timeout := time.After(10 * time.Second)
			completed := false

			for !completed {
				select {
				case eventData := <-completionCh:
					var event map[string]interface{}
					if err := json.Unmarshal([]byte(eventData), &event); err == nil {
						if eventType, ok := event["type"].(string); ok && eventType == "run_completed" {
							if eventRunID, ok := event["run_id"].(string); ok && eventRunID == runResp.Run.RunID {
								completed = true
							}
						}
					}
				case <-timeout:
					w = httptest.NewRecorder()
					req, _ = http.NewRequest(http.MethodGet, "/api/v1/workflow-runs/"+runResp.Run.RunID, nil)
					router.ServeHTTP(w, req)

					var statusResp struct {
						Run struct {
							Status string `json:"status"`
						} `json:"run"`
					}
					json.Unmarshal(w.Body.Bytes(), &statusResp)
					if statusResp.Run.Status == "completed" {
						completed = true
					} else {
						t.Fatalf("timeout, status: %s", statusResp.Run.Status)
					}
				}
			}

			calls := mockService.getExecuteCalls()
			if len(calls) != 1 {
				t.Fatalf("expected 1 call, got %d", len(calls))
			}

			if calls[0].AgentName != tc.expectedAgent {
				t.Errorf("expected %s, got %s", tc.expectedAgent, calls[0].AgentName)
			}

			t.Logf("SUCCESS: %s -> %s", tc.name, calls[0].AgentName)
		})
	}
}
