package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"station/internal/services"
	"station/pkg/models"
)

// MockAgentRunsRepo implements a mock AgentRuns repository for testing
type MockAgentRunsRepo struct {
	runs     map[int64]*models.AgentRun
	nextID   int64
	failNext bool
}

func NewMockAgentRunsRepo() *MockAgentRunsRepo {
	return &MockAgentRunsRepo{
		runs:   make(map[int64]*models.AgentRun),
		nextID: 1,
	}
}

func (m *MockAgentRunsRepo) Create(ctx context.Context, agentID, userID int64, task, finalResponse string, stepsTaken int64, toolCalls, executionSteps *models.JSONArray, status string, completedAt *time.Time) (*models.AgentRun, error) {
	if m.failNext {
		m.failNext = false
		return nil, assert.AnError
	}

	run := &models.AgentRun{
		ID:             m.nextID,
		AgentID:        agentID,
		UserID:         userID,
		Task:           task,
		FinalResponse:  finalResponse,
		StepsTaken:     stepsTaken,
		ToolCalls:      toolCalls,
		ExecutionSteps: executionSteps,
		Status:         status,
		StartedAt:      time.Now(),
		CompletedAt:    completedAt,
	}

	m.runs[m.nextID] = run
	m.nextID++
	return run, nil
}

func (m *MockAgentRunsRepo) SetFailNext(fail bool) {
	m.failNext = fail
}

// MockAgentExecutionEngine implements a mock execution engine for testing
type MockAgentExecutionEngine struct {
	shouldFail bool
	result     *services.AgentExecutionResult
}

func (m *MockAgentExecutionEngine) ExecuteAgent(ctx context.Context, agent *models.Agent, task string, runID int64) (*services.AgentExecutionResult, error) {
	if m.shouldFail {
		return nil, assert.AnError
	}

	if m.result != nil {
		return m.result, nil
	}

	return &services.AgentExecutionResult{
		Success:   true,
		Response:  "Mock response for task: " + task,
		ToolCalls: &models.JSONArray{},
		Duration:  time.Second,
		StepsUsed: 1,
		TokenUsage: map[string]interface{}{
			"input_tokens":  10,
			"output_tokens": 20,
			"total_tokens":  30,
		},
	}, nil
}

func TestAgentToolWrapper_Name(t *testing.T) {
	agent := &models.Agent{
		Name: "TestAgent",
	}

	wrapper := NewAgentToolWrapper(agent, "test-env", nil, 0)
	assert.Equal(t, "__agent_testagent", wrapper.Name())
}

func TestAgentToolWrapper_Description(t *testing.T) {
	agent := &models.Agent{
		Name:        "TestAgent",
		Description: "A test agent",
	}

	wrapper := NewAgentToolWrapper(agent, "test-env", nil, 0)
	assert.Equal(t, "A test agent", wrapper.Description())
}

func TestAgentToolWrapper_convertInputToTask(t *testing.T) {
	tests := []struct {
		name          string
		agent         *models.Agent
		input         map[string]interface{}
		expectedTask  string
		expectError   bool
		checkContains bool // New flag to check if result contains expected content
	}{
		{
			name: "simple query input",
			agent: &models.Agent{
				Name: "TestAgent",
			},
			input: map[string]interface{}{
				"query": "Analyze this code",
			},
			expectedTask: "Analyze this code",
			expectError:  false,
		},
		{
			name: "structured input without schema",
			agent: &models.Agent{
				Name: "TestAgent",
			},
			input: map[string]interface{}{
				"repository_path": "/home/user/project",
				"analysis_type":   "security",
			},
			expectedTask:  "repository_path: /home/user/project",
			expectError:   false,
			checkContains: true,
		},
		{
			name: "empty input",
			agent: &models.Agent{
				Name: "TestAgent",
			},
			input:        map[string]interface{}{},
			expectedTask: "Execute agent task",
			expectError:  false,
		},
		{
			name: "complex input with arrays",
			agent: &models.Agent{
				Name: "TestAgent",
			},
			input: map[string]interface{}{
				"files": []string{"file1.go", "file2.js"},
				"depth": 3,
			},
			expectedTask:  `["file1.go","file2.js"]`, // Just check it contains the JSON arrays
			expectError:   false,
			checkContains: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapper := &AgentToolWrapper{agent: tt.agent}
			task, err := wrapper.convertInputToTask(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkContains {
					assert.Contains(t, task, tt.expectedTask)
				} else {
					assert.Equal(t, tt.expectedTask, task)
				}
			}
		})
	}
}

func TestAgentToolWrapper_formatStructuredInput(t *testing.T) {
	wrapper := &AgentToolWrapper{}

	tests := []struct {
		name          string
		input         map[string]interface{}
		expectedParts []string // Check that all parts are present
	}{
		{
			name: "simple string values",
			input: map[string]interface{}{
				"path":     "/home/user",
				"filename": "test.txt",
			},
			expectedParts: []string{"path: /home/user", "filename: test.txt"},
		},
		{
			name: "mixed types",
			input: map[string]interface{}{
				"count": 5,
				"name":  "test",
			},
			expectedParts: []string{"count: 5", "name: test"},
		},
		{
			name:          "empty input",
			input:         map[string]interface{}{},
			expectedParts: []string{"Execute agent task"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapper.formatStructuredInput(tt.input)
			for _, part := range tt.expectedParts {
				assert.Contains(t, result, part)
			}
		})
	}
}

func TestAgentToolWrapper_formatToolResponse(t *testing.T) {
	wrapper := &AgentToolWrapper{}

	result := &services.AgentExecutionResult{
		Success:   true,
		Response:  "Analysis complete",
		ToolCalls: &models.JSONArray{},
		Duration:  2 * time.Second,
		StepsUsed: 3,
		TokenUsage: map[string]interface{}{
			"input_tokens":  100,
			"output_tokens": 200,
			"total_tokens":  300,
		},
		Error: "",
	}

	response := wrapper.formatToolResponse(result)

	assert.Equal(t, true, response["success"])
	assert.Equal(t, "Analysis complete", response["response"])
	assert.Equal(t, 2.0, response["duration"])
	assert.Equal(t, 3, response["steps_used"])
	assert.NotNil(t, response["token_usage"])
	assert.Nil(t, response["error"])
}

func TestAgentToolWrapper_formatToolResponse_WithError(t *testing.T) {
	wrapper := &AgentToolWrapper{}

	result := &services.AgentExecutionResult{
		Success:   false,
		Response:  "",
		ToolCalls: &models.JSONArray{},
		Duration:  1 * time.Second,
		StepsUsed: 1,
		Error:     "Execution failed",
	}

	response := wrapper.formatToolResponse(result)

	assert.Equal(t, false, response["success"])
	assert.Equal(t, "Execution failed", response["error"])
	assert.Equal(t, 1.0, response["duration"])
}

func TestAgentToolWrapper_GetInputSchema(t *testing.T) {
	extractor := &AgentSchemaExtractor{}

	tests := []struct {
		name         string
		agent        *models.Agent
		expectSchema bool
	}{
		{
			name: "agent with input schema",
			agent: &models.Agent{
				Name: "TestAgent",
				InputSchema: stringPtr(`{
					"type": "object",
					"properties": {
						"repository_path": {"type": "string", "description": "Path to repository"},
						"analysis_type": {"type": "string", "description": "Type of analysis"}
					},
					"required": ["repository_path"]
				}`),
			},
			expectSchema: true,
		},
		{
			name: "agent without input schema",
			agent: &models.Agent{
				Name:        "TestAgent",
				InputSchema: nil,
			},
			expectSchema: true, // Should return default schema
		},
		{
			name: "agent with invalid input schema",
			agent: &models.Agent{
				Name:        "TestAgent",
				InputSchema: stringPtr("invalid json"),
			},
			expectSchema: true, // Should return default schema on parse error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := extractor.ExtractInputSchema(tt.agent)
			assert.NotNil(t, schema)
			assert.Equal(t, "object", schema["type"])
			assert.NotNil(t, schema["properties"])

			if tt.agent.InputSchema != nil && *tt.agent.InputSchema != "invalid json" {
				// Should have the custom schema properties
				props := schema["properties"].(map[string]interface{})
				assert.NotNil(t, props["repository_path"])
				assert.NotNil(t, props["analysis_type"])
			} else if tt.agent.InputSchema == nil {
				// Should have default query property
				props := schema["properties"].(map[string]interface{})
				assert.NotNil(t, props["query"])
			}
		})
	}
}

func TestAgentSchemaExtractor_convertAgentInputSchema(t *testing.T) {
	extractor := &AgentSchemaExtractor{}

	validSchema := `{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File path"},
			"depth": {"type": "number", "description": "Search depth"}
		},
		"required": ["path"]
	}`

	schema := extractor.convertAgentInputSchema(validSchema)

	assert.Equal(t, "object", schema["type"])
	props := schema["properties"].(map[string]interface{})
	assert.NotNil(t, props["path"])
	assert.NotNil(t, props["depth"])

	pathProp := props["path"].(map[string]interface{})
	assert.Equal(t, "string", pathProp["type"])
	assert.Equal(t, "File path", pathProp["description"])
}

func TestAgentSchemaExtractor_createDefaultInputSchema(t *testing.T) {
	extractor := &AgentSchemaExtractor{}
	schema := extractor.createDefaultInputSchema()

	assert.Equal(t, "object", schema["type"])
	props := schema["properties"].(map[string]interface{})
	assert.NotNil(t, props["query"])

	queryProp := props["query"].(map[string]interface{})
	assert.Equal(t, "string", queryProp["type"])
	assert.Equal(t, "The task or query for the agent", queryProp["description"])

	required := schema["required"].([]string)
	assert.Contains(t, required, "query")
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
