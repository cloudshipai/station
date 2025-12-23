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

type testWorkflowResponse struct {
	Workflow struct {
		WorkflowID string `json:"workflow_id"`
		Version    int64  `json:"version"`
	} `json:"workflow"`
}

type testRunResponse struct {
	Run struct {
		RunID  string `json:"run_id"`
		Status string `json:"status"`
	} `json:"run"`
}

// Test end-to-end flow: definition -> validation -> start -> pause/resume -> complete with NATS scheduling.
func TestWorkflowAPIEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test database and repositories
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()
	repos := repositories.New(testDB)

	// Embedded NATS engine for scheduling
	engine, err := runtime.NewEmbeddedEngineForTests()
	if err != nil {
		t.Fatalf("failed to start embedded engine: %v", err)
	}
	defer engine.Close()

	// Workflow service with engine injected
	workflowService := services.NewWorkflowServiceWithEngine(repos, engine)

	// API handler with injected workflow service
	handler := NewAPIHandlers(repos, testDB.Conn(), services.NewToolDiscoveryService(repos), nil, true)
	handler.workflowService = workflowService

	router := gin.New()
	apiGroup := router.Group("/api/v1")
	handler.RegisterRoutes(apiGroup)

	// Subscribe to schedule messages before starting the run
	scheduleCh := make(chan string, 1)
	sub, err := engine.Subscribe("workflow.run.*.step.*.schedule", func(msg *nats.Msg) {
		scheduleCh <- string(msg.Data)
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer sub.Unsubscribe()

	// Create workflow definition
	defPayload := map[string]interface{}{
		"workflowId": "demo-api",
		"name":       "Demo API Workflow",
		"definition": map[string]interface{}{
			"id":    "demo-api",
			"start": "start",
			"states": []map[string]interface{}{
				{"id": "start", "type": "operation", "input": map[string]interface{}{"task": "agent.run"}, "transition": "finish"},
				{"id": "finish", "type": "operation", "input": map[string]interface{}{"task": "custom.run"}},
			},
		},
	}
	createBody, _ := json.Marshal(defPayload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/workflows", bytes.NewBuffer(createBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	// Start run
	startBody := []byte(`{"workflowId":"demo-api"}`)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/workflow-runs", bytes.NewBuffer(startBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("start run failed: status=%d body=%s", w.Code, w.Body.String())
	}
	var runResp testRunResponse
	if err := json.Unmarshal(w.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("failed to parse run response: %v", err)
	}
	runID := runResp.Run.RunID
	if runID == "" {
		t.Fatalf("run id should not be empty")
	}

	// Expect schedule message via NATS
	select {
	case <-scheduleCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("did not receive schedule message")
	}

	// Pause run
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/workflow-runs/"+runID+"/pause", bytes.NewBuffer([]byte(`{"reason":"maintenance"}`)))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("pause run failed: %d body=%s", w.Code, w.Body.String())
	}

	// Resume run
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/workflow-runs/"+runID+"/resume", bytes.NewBuffer([]byte(`{"name":"resume"}`)))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("resume run failed: %d body=%s", w.Code, w.Body.String())
	}

	// Complete run
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/workflow-runs/"+runID+"/complete", bytes.NewBuffer([]byte(`{"result":{"status":"ok"}}`)))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("complete run failed: %d body=%s", w.Code, w.Body.String())
	}

	// Fetch run to verify completion
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/workflow-runs/"+runID, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get run failed: %d body=%s", w.Code, w.Body.String())
	}
	var finalRun struct {
		Run struct {
			Status string `json:"status"`
		} `json:"run"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &finalRun); err != nil {
		t.Fatalf("failed to parse get run response: %v", err)
	}
	if finalRun.Run.Status != "completed" {
		t.Fatalf("expected completed status, got %s", finalRun.Run.Status)
	}
}
