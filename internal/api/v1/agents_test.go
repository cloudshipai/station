package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"station/internal/services"
	"station/internal/storage"
	"station/pkg/models"
)

// MockAgentService implements AgentServiceInterface for testing
type MockAgentService struct {
	agents      map[int64]*models.Agent
	nextID      int64
	shouldError bool
}

func NewMockAgentService() *MockAgentService {
	return &MockAgentService{
		agents: make(map[int64]*models.Agent),
		nextID: 1,
	}
}

func (m *MockAgentService) CreateAgent(ctx context.Context, config *services.AgentConfig) (*models.Agent, error) {
	if m.shouldError {
		return nil, assert.AnError
	}

	agent := &models.Agent{
		ID:            m.nextID,
		Name:          config.Name,
		Description:   config.Description,
		Prompt:        config.Prompt,
		MaxSteps:      config.MaxSteps,
		EnvironmentID: config.EnvironmentID,
		CreatedBy:     config.CreatedBy,
	}
	m.agents[agent.ID] = agent
	m.nextID++
	return agent, nil
}

func (m *MockAgentService) GetAgent(ctx context.Context, agentID int64) (*models.Agent, error) {
	if m.shouldError {
		return nil, assert.AnError
	}

	agent, exists := m.agents[agentID]
	if !exists {
		return nil, assert.AnError
	}
	return agent, nil
}

func (m *MockAgentService) ListAgentsByEnvironment(ctx context.Context, environmentID int64) ([]*models.Agent, error) {
	if m.shouldError {
		return nil, assert.AnError
	}

	var result []*models.Agent
	for _, agent := range m.agents {
		if environmentID == 0 || agent.EnvironmentID == environmentID {
			result = append(result, agent)
		}
	}
	return result, nil
}

func (m *MockAgentService) UpdateAgent(ctx context.Context, agentID int64, config *services.AgentConfig) (*models.Agent, error) {
	if m.shouldError {
		return nil, assert.AnError
	}

	agent, exists := m.agents[agentID]
	if !exists {
		return nil, assert.AnError
	}

	// Update fields
	if config.Name != "" {
		agent.Name = config.Name
	}
	if config.Description != "" {
		agent.Description = config.Description
	}
	if config.Prompt != "" {
		agent.Prompt = config.Prompt
	}
	if config.MaxSteps > 0 {
		agent.MaxSteps = config.MaxSteps
	}

	return agent, nil
}

func (m *MockAgentService) DeleteAgent(ctx context.Context, agentID int64) error {
	if m.shouldError {
		return assert.AnError
	}

	if _, exists := m.agents[agentID]; !exists {
		return assert.AnError
	}

	delete(m.agents, agentID)
	return nil
}

// ExecuteAgent and other methods not needed for these tests
func (m *MockAgentService) ExecuteAgent(ctx context.Context, agentID int64, task string, userVariables map[string]interface{}) (*services.Message, error) {
	return nil, assert.AnError
}

func (m *MockAgentService) ExecuteAgentWithRunID(ctx context.Context, agentID int64, task string, runID int64, userVariables map[string]interface{}) (*services.Message, error) {
	return nil, assert.AnError
}

func (m *MockAgentService) InitializeMCP(ctx context.Context) error {
	return nil
}

func (m *MockAgentService) GetExecutionEngine() interface{} {
	return nil
}

func (m *MockAgentService) UpdateAgentPrompt(ctx context.Context, agentID int64, prompt string) error {
	if m.shouldError {
		return assert.AnError
	}

	agent, exists := m.agents[agentID]
	if !exists {
		return assert.AnError
	}

	agent.Prompt = prompt
	return nil
}

func (m *MockAgentService) SetFileStore(store storage.FileStore) {}

// MockAgentExportService for testing - just set to nil since it's optional
// The real handler will check for nil and skip export

// Helper to create test API handlers with mock service
func setupTestAPIHandlers() *APIHandlers {
	mockService := NewMockAgentService()
	return &APIHandlers{
		agentService:       mockService,
		agentExportService: nil, // Skip export in tests
		localMode:          true,
	}
}

var assert = struct {
	AnError error
}{
	AnError: &TestError{"test error"},
}

type TestError struct {
	msg string
}

func (e *TestError) Error() string {
	return e.msg
}

func TestAPIHandlers_CreateAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		shouldError    bool
	}{
		{
			name: "valid agent creation",
			requestBody: map[string]interface{}{
				"name":           "Test Agent",
				"description":    "A test agent",
				"prompt":         "You are a test agent.",
				"environment_id": 1,
				"max_steps":      25,
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "missing required fields",
			requestBody: map[string]interface{}{
				"description": "A test agent without name",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "zero max_steps gets default",
			requestBody: map[string]interface{}{
				"name":           "Test Agent",
				"description":    "A test agent",
				"prompt":         "You are a test agent.",
				"environment_id": 1,
				"max_steps":      0,
			},
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers := setupTestAPIHandlers()
			if tt.shouldError {
				handlers.agentService.(*MockAgentService).shouldError = true
			}

			// Create request
			bodyBytes, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/agents", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Create Gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Call handler
			handlers.createAgent(c)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// If successful creation, verify response structure
			if tt.expectedStatus == http.StatusCreated && w.Code == http.StatusCreated {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				// Check required fields in response
				if _, exists := response["agent_id"]; !exists {
					t.Error("Response missing agent_id")
				}
				if _, exists := response["message"]; !exists {
					t.Error("Response missing message")
				}
			}
		})
	}
}

func TestAPIHandlers_GetAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := setupTestAPIHandlers()
	mockService := handlers.agentService.(*MockAgentService)

	// Create a test agent first
	testAgent := &models.Agent{
		ID:            1,
		Name:          "Test Agent",
		Description:   "A test agent",
		Prompt:        "You are a test agent.",
		MaxSteps:      25,
		EnvironmentID: 1,
		CreatedBy:     1,
	}
	mockService.agents[1] = testAgent

	tests := []struct {
		name           string
		agentID        string
		expectedStatus int
		shouldError    bool
	}{
		{
			name:           "existing agent",
			agentID:        "1",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-existent agent",
			agentID:        "999",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid agent ID",
			agentID:        "not-a-number",
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
			req := httptest.NewRequest("GET", "/agents/"+tt.agentID, nil)

			// Create response recorder
			w := httptest.NewRecorder()

			// Create Gin context with URL parameter
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = gin.Params{
				{Key: "id", Value: tt.agentID},
			}

			// Call handler
			handlers.getAgent(c)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// If successful, verify response contains agent data
			if tt.expectedStatus == http.StatusOK && w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				// Extract agent from response wrapper
				agent, exists := response["agent"].(map[string]interface{})
				if !exists {
					t.Fatal("Response missing agent object")
				}

				// Verify agent fields
				if agent["id"] != float64(testAgent.ID) {
					t.Errorf("Expected agent ID %d, got %v", testAgent.ID, agent["id"])
				}
				if agent["name"] != testAgent.Name {
					t.Errorf("Expected agent name %s, got %v", testAgent.Name, agent["name"])
				}
			}
		})
	}
}

func TestAPIHandlers_ListAgents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := setupTestAPIHandlers()
	mockService := handlers.agentService.(*MockAgentService)

	// Create test agents in different environments
	agent1 := &models.Agent{ID: 1, Name: "Agent 1", EnvironmentID: 1}
	agent2 := &models.Agent{ID: 2, Name: "Agent 2", EnvironmentID: 1}
	agent3 := &models.Agent{ID: 3, Name: "Agent 3", EnvironmentID: 2}

	mockService.agents[1] = agent1
	mockService.agents[2] = agent2
	mockService.agents[3] = agent3

	tests := []struct {
		name           string
		environmentID  string
		expectedCount  int
		expectedStatus int
	}{
		{
			name:           "all agents",
			environmentID:  "",
			expectedCount:  3,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "agents in environment 1",
			environmentID:  "1",
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "agents in environment 2",
			environmentID:  "2",
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid environment ID",
			environmentID:  "not-a-number",
			expectedCount:  0,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request with query parameter
			url := "/agents"
			if tt.environmentID != "" {
				url += "?environment_id=" + tt.environmentID
			}
			req := httptest.NewRequest("GET", url, nil)

			// Create response recorder
			w := httptest.NewRecorder()

			// Create Gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Call handler
			handlers.listAgents(c)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// If successful, verify agent count
			if tt.expectedStatus == http.StatusOK && w.Code == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				agents, exists := response["agents"].([]interface{})
				if !exists {
					t.Fatal("Response missing agents array")
				}

				if len(agents) != tt.expectedCount {
					t.Errorf("Expected %d agents, got %d", tt.expectedCount, len(agents))
				}
			}
		})
	}
}
