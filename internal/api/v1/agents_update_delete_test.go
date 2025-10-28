package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"station/pkg/models"
)

func TestAPIHandlers_UpdateAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := setupTestAPIHandlers()
	mockService := handlers.agentService.(*MockAgentService)

	// Create a test agent first
	testAgent := &models.Agent{
		ID:            1,
		Name:          "Original Agent",
		Description:   "Original description",
		Prompt:        "Original prompt.",
		MaxSteps:      25,
		EnvironmentID: 1,
		CreatedBy:     1,
	}
	mockService.agents[1] = testAgent

	tests := []struct {
		name           string
		agentID        string
		requestBody    map[string]interface{}
		expectedStatus int
		shouldError    bool
	}{
		{
			name:    "valid agent update",
			agentID: "1",
			requestBody: map[string]interface{}{
				"name":        "Updated Agent",
				"description": "Updated description",
				"prompt":      "Updated prompt.",
				"max_steps":   50,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "partial agent update",
			agentID: "1",
			requestBody: map[string]interface{}{
				"name": "Partially Updated Agent",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-existent agent",
			agentID:        "999",
			requestBody:    map[string]interface{}{"name": "Updated Name"},
			expectedStatus: http.StatusInternalServerError, // MockAgentService returns error for non-existent
		},
		{
			name:           "invalid agent ID",
			agentID:        "not-a-number",
			requestBody:    map[string]interface{}{"name": "Updated Name"},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldError {
				mockService.shouldError = true
			} else {
				mockService.shouldError = false
			}

			// Create request
			bodyBytes, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("PUT", "/agents/"+tt.agentID, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Create Gin context with URL parameter
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = gin.Params{
				{Key: "id", Value: tt.agentID},
			}

			// Call handler
			handlers.updateAgent(c)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// If successful update, verify response structure
			if tt.expectedStatus == http.StatusOK && w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				// Check required fields in response - updateAgent only returns message
				if _, exists := response["message"]; !exists {
					t.Error("Response missing message")
				}
			}
		})
	}
}

func TestAPIHandlers_DeleteAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		agentID        string
		expectedStatus int
		shouldError    bool
	}{
		{
			name:           "delete existing agent",
			agentID:        "1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "delete non-existent agent",
			agentID:        "999",
			expectedStatus: http.StatusInternalServerError, // MockAgentService returns error for non-existent
		},
		{
			name:           "invalid agent ID",
			agentID:        "not-a-number",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers := setupTestAPIHandlers()
			mockService := handlers.agentService.(*MockAgentService)

			// Create a test agent first for valid deletion
			if tt.agentID == "1" {
				testAgent := &models.Agent{
					ID:   1,
					Name: "Agent To Delete",
				}
				mockService.agents[1] = testAgent
			}

			if tt.shouldError {
				mockService.shouldError = true
			} else {
				mockService.shouldError = false
			}

			// Create request
			req := httptest.NewRequest("DELETE", "/agents/"+tt.agentID, nil)

			// Create response recorder
			w := httptest.NewRecorder()

			// Create Gin context with URL parameter
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = gin.Params{
				{Key: "id", Value: tt.agentID},
			}

			// Call handler
			handlers.deleteAgent(c)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// If successful deletion, verify response structure
			if tt.expectedStatus == http.StatusOK && w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				// Check message in response
				if _, exists := response["message"]; !exists {
					t.Error("Response missing message")
				}
			}
		})
	}
}

func TestAPIHandlers_CallAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := setupTestAPIHandlers()
	mockService := handlers.agentService.(*MockAgentService)

	// Create a test agent
	testAgent := &models.Agent{
		ID:   1,
		Name: "Test Execution Agent",
	}
	mockService.agents[1] = testAgent

	tests := []struct {
		name           string
		agentID        string
		requestBody    map[string]interface{}
		expectedStatus int
		shouldError    bool
	}{
		{
			name:    "valid agent execution",
			agentID: "1",
			requestBody: map[string]interface{}{
				"task": "Test task",
			},
			expectedStatus: http.StatusAccepted, // CallAgent returns 202 Accepted
		},
		{
			name:           "missing task",
			agentID:        "1",
			requestBody:    map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:    "non-existent agent",
			agentID: "999",
			requestBody: map[string]interface{}{
				"task": "Test task",
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid agent ID",
			agentID:        "not-a-number",
			requestBody:    map[string]interface{}{"task": "Test task"},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldError {
				mockService.shouldError = true
			} else {
				mockService.shouldError = false
			}

			// Create request
			bodyBytes, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/agents/"+tt.agentID+"/execute", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Create Gin context with URL parameter
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = gin.Params{
				{Key: "id", Value: tt.agentID},
			}

			// Call handler
			handlers.callAgent(c)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// If successful execution, verify response structure
			if tt.expectedStatus == http.StatusAccepted && w.Code == http.StatusAccepted {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				// Check required fields in response
				if _, exists := response["message"]; !exists {
					t.Error("Response missing message")
				}
				if _, exists := response["agent_id"]; !exists {
					t.Error("Response missing agent_id")
				}
				if _, exists := response["task"]; !exists {
					t.Error("Response missing task")
				}
			}
		})
	}
}
