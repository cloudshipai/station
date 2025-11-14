package services

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
)

// MockAgentService implements AgentServiceInterface for testing
type MockAgentService struct {
	agents map[int64]*models.Agent
}

func NewMockAgentService() *MockAgentService {
	return &MockAgentService{
		agents: make(map[int64]*models.Agent),
	}
}

func (m *MockAgentService) ExecuteAgent(ctx context.Context, agentID int64, task string, userVariables map[string]interface{}) (*Message, error) {
	return m.ExecuteAgentWithRunID(ctx, agentID, task, 0, userVariables)
}

func (m *MockAgentService) ExecuteAgentWithRunID(ctx context.Context, agentID int64, task string, runID int64, userVariables map[string]interface{}) (*Message, error) {
	agent, exists := m.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent %d not found", agentID)
	}

	// Simulate agent execution
	return &Message{
		Content: fmt.Sprintf("Agent %s executed task: %s", agent.Name, task),
		Role:    RoleAssistant,
	}, nil
}

func (m *MockAgentService) CreateAgent(ctx context.Context, config *AgentConfig) (*models.Agent, error) {
	return nil, fmt.Errorf("not implemented in mock")
}

func (m *MockAgentService) GetAgent(ctx context.Context, agentID int64) (*models.Agent, error) {
	agent, exists := m.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent %d not found", agentID)
	}
	return agent, nil
}

func (m *MockAgentService) ListAgentsByEnvironment(ctx context.Context, environmentID int64) ([]*models.Agent, error) {
	var result []*models.Agent
	for _, agent := range m.agents {
		if agent.EnvironmentID == environmentID {
			result = append(result, agent)
		}
	}
	return result, nil
}

func (m *MockAgentService) UpdateAgent(ctx context.Context, agentID int64, config *AgentConfig) (*models.Agent, error) {
	return nil, fmt.Errorf("not implemented in mock")
}

func (m *MockAgentService) UpdateAgentPrompt(ctx context.Context, agentID int64, prompt string) error {
	return fmt.Errorf("not implemented in mock")
}

func (m *MockAgentService) DeleteAgent(ctx context.Context, agentID int64) error {
	return fmt.Errorf("not implemented in mock")
}

func (m *MockAgentService) AddAgent(agent *models.Agent) {
	m.agents[agent.ID] = agent
}

// TestAgentAsToolWithRealService tests the AgentAsTool with a real agent service
func TestAgentAsToolWithRealService(t *testing.T) {
	mockService := NewMockAgentService()

	// Add test agents
	testAgent := &models.Agent{
		ID:            1,
		EnvironmentID: 1,
		Name:          "Test Agent",
		Description:   "A test agent for unit testing",
		Prompt:        "You are a test agent",
		MaxSteps:      5,
		CreatedBy:     1,
	}
	mockService.AddAgent(testAgent)

	t.Run("AgentAsTool with successful execution", func(t *testing.T) {
		tool := &AgentAsTool{
			agentID:       1,
			agentName:     "Test Agent",
			description:   "A test agent for unit testing",
			agentService:  mockService,
			environmentID: 1,
		}

		ctx := context.Background()
		input := map[string]interface{}{
			"task": "Analyze this code for security issues",
		}

		result, err := tool.RunRaw(ctx, input)
		if err != nil {
			t.Fatalf("RunRaw failed: %v", err)
		}

		if result == nil {
			t.Fatal("Result should not be nil")
		}

		resultStr, ok := result.(string)
		if !ok {
			t.Fatal("Result should be a string")
		}

		expected := "Agent Test Agent executed task: Analyze this code for security issues"
		if resultStr != expected {
			t.Errorf("Expected result '%s', got '%s'", expected, resultStr)
		}
	})

	t.Run("AgentAsTool with missing agent", func(t *testing.T) {
		tool := &AgentAsTool{
			agentID:       999, // Non-existent agent
			agentName:     "Missing Agent",
			description:   "A non-existent agent",
			agentService:  mockService,
			environmentID: 1,
		}

		ctx := context.Background()
		input := map[string]interface{}{
			"task": "This should fail",
		}

		_, err := tool.RunRaw(ctx, input)
		if err == nil {
			t.Fatal("RunRaw should fail for non-existent agent")
		}

		if !strings.Contains(err.Error(), "agent execution failed: agent Missing Agent (ID: 999") {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("AgentAsTool with nil agent service", func(t *testing.T) {
		tool := &AgentAsTool{
			agentID:       1,
			agentName:     "Test Agent",
			description:   "A test agent",
			agentService:  nil, // No service
			environmentID: 1,
		}

		ctx := context.Background()
		input := map[string]interface{}{
			"task": "This should fail",
		}

		_, err := tool.RunRaw(ctx, input)
		if err == nil {
			t.Fatal("RunRaw should fail with nil agent service")
		}

		if !strings.Contains(err.Error(), "agent service not available for agent tool Test Agent") {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	t.Run("AgentAsTool with invalid input", func(t *testing.T) {
		tool := &AgentAsTool{
			agentID:       1,
			agentName:     "Test Agent",
			description:   "A test agent",
			agentService:  mockService,
			environmentID: 1,
		}

		ctx := context.Background()

		// Test various invalid inputs
		invalidInputs := []interface{}{
			nil,
			"string instead of map",
			123,
			[]string{"array"},
			map[string]interface{}{},            // missing task
			map[string]interface{}{"task": 123}, // non-string task
		}

		for i, invalidInput := range invalidInputs {
			_, err := tool.RunRaw(ctx, invalidInput)
			if err == nil {
				t.Errorf("Invalid input %d should fail: %v", i, invalidInput)
			}
		}
	})
}

// TestAgentToolHierarchy tests hierarchical agent execution
func TestAgentToolHierarchy(t *testing.T) {
	mockService := NewMockAgentService()

	// Create a hierarchy of agents
	orchestrator := &models.Agent{
		ID:            1,
		EnvironmentID: 1,
		Name:          "Security Orchestrator",
		Description:   "Coordinates security analysis",
		Prompt:        "You coordinate security analysis across multiple agents",
		MaxSteps:      10,
		CreatedBy:     1,
	}

	analyzer := &models.Agent{
		ID:            2,
		EnvironmentID: 1,
		Name:          "Code Analyzer",
		Description:   "Analyzes code for security issues",
		Prompt:        "You analyze code for security vulnerabilities",
		MaxSteps:      5,
		CreatedBy:     1,
	}

	reporter := &models.Agent{
		ID:            3,
		EnvironmentID: 1,
		Name:          "Security Reporter",
		Description:   "Generates security reports",
		Prompt:        "You generate comprehensive security reports",
		MaxSteps:      3,
		CreatedBy:     1,
	}

	mockService.AddAgent(orchestrator)
	mockService.AddAgent(analyzer)
	mockService.AddAgent(reporter)

	t.Run("Hierarchical agent execution simulation", func(t *testing.T) {
		// Create tools for each agent
		orchestratorTool := &AgentAsTool{
			agentID:       1,
			agentName:     "Security Orchestrator",
			description:   "Coordinates security analysis",
			agentService:  mockService,
			environmentID: 1,
		}

		analyzerTool := &AgentAsTool{
			agentID:       2,
			agentName:     "Code Analyzer",
			description:   "Analyzes code for security issues",
			agentService:  mockService,
			environmentID: 1,
		}

		reporterTool := &AgentAsTool{
			agentID:       3,
			agentName:     "Security Reporter",
			description:   "Generates security reports",
			agentService:  mockService,
			environmentID: 1,
		}

		ctx := context.Background()

		// Simulate orchestrator calling other agents
		orchestratorResult, err := orchestratorTool.RunRaw(ctx, map[string]interface{}{
			"task": "Coordinate security analysis of the codebase",
		})
		if err != nil {
			t.Fatalf("Orchestrator failed: %v", err)
		}

		analyzerResult, err := analyzerTool.RunRaw(ctx, map[string]interface{}{
			"task": "Analyze the code for security vulnerabilities and OWASP issues",
		})
		if err != nil {
			t.Fatalf("Analyzer failed: %v", err)
		}

		reporterResult, err := reporterTool.RunRaw(ctx, map[string]interface{}{
			"task": "Generate comprehensive security report based on analysis findings",
		})
		if err != nil {
			t.Fatalf("Reporter failed: %v", err)
		}

		t.Logf("Orchestrator: %v", orchestratorResult)
		t.Logf("Analyzer: %v", analyzerResult)
		t.Logf("Reporter: %v", reporterResult)

		// Verify all results are meaningful
		results := []interface{}{orchestratorResult, analyzerResult, reporterResult}
		for i, result := range results {
			if result == nil {
				t.Errorf("Result %d should not be nil", i+1)
			}
			resultStr, ok := result.(string)
			if !ok || resultStr == "" {
				t.Errorf("Result %d should be a non-empty string", i+1)
			}
		}
	})
}

// TestAgentToolDefinition tests the tool definition generation
func TestAgentToolDefinition(t *testing.T) {
	t.Run("Tool definition structure", func(t *testing.T) {
		tool := &AgentAsTool{
			agentID:       1,
			agentName:     "Test Definition Agent",
			description:   "Tests tool definition generation",
			agentService:  nil,
			environmentID: 1,
		}

		def := tool.Definition()

		if def.Name != "__agent_test_definition_agent" {
			t.Errorf("Expected name '__agent_test_definition_agent', got '%s'", def.Name)
		}

		if def.Description != "Tests tool definition generation" {
			t.Errorf("Expected description 'Tests tool definition generation', got '%s'", def.Description)
		}

		// Validate input schema
		inputSchema := def.InputSchema
		if inputSchema["type"] != "object" {
			t.Error("Input schema type should be 'object'")
		}

		properties, ok := inputSchema["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("Input schema should have properties")
		}

		taskProp, ok := properties["task"].(map[string]interface{})
		if !ok {
			t.Fatal("Properties should have 'task' field")
		}

		if taskProp["type"] != "string" {
			t.Error("Task property type should be 'string'")
		}

		if taskProp["description"] == nil {
			t.Error("Task property should have description")
		}

		required, ok := inputSchema["required"].([]string)
		if !ok || len(required) != 1 || required[0] != "task" {
			t.Error("Input schema should require 'task' field")
		}

		// Validate output schema
		outputSchema := def.OutputSchema
		if outputSchema["type"] != "string" {
			t.Error("Output schema type should be 'string'")
		}
	})
}

// TestAgentToolRespondAndRestart tests the Respond and Restart methods
func TestAgentToolRespondAndRestart(t *testing.T) {
	tool := &AgentAsTool{
		agentID:       1,
		agentName:     "Test Tool Agent",
		description:   "Tests respond and restart functionality",
		agentService:  nil,
		environmentID: 1,
	}

	t.Run("Respond method", func(t *testing.T) {
		toolReq := &ai.Part{
			Kind: ai.PartToolRequest,
			ToolRequest: &ai.ToolRequest{
				Name:  "__agent_test_tool_agent",
				Input: map[string]interface{}{"task": "test task"},
			},
		}

		outputData := "Test output from agent execution"
		response := tool.Respond(toolReq, outputData, nil)

		if response == nil {
			t.Fatal("Respond should return a Part")
		}

		if !response.IsToolResponse() {
			t.Error("Response should be a tool response")
		}

		if response.ToolResponse == nil {
			t.Fatal("Response should have ToolResponse")
		}

		if response.ToolResponse.Output != outputData {
			t.Errorf("Expected output '%s', got '%v'", outputData, response.ToolResponse.Output)
		}
	})

	t.Run("Restart method with valid tool request", func(t *testing.T) {
		toolReq := &ai.Part{
			Kind: ai.PartToolRequest,
			ToolRequest: &ai.ToolRequest{
				Name:  "__agent_test_tool_agent",
				Input: map[string]interface{}{"task": "restart test"},
			},
		}

		restarted := tool.Restart(toolReq, nil)

		if restarted == nil {
			t.Fatal("Restart should return a Part")
		}

		if !restarted.IsToolRequest() {
			t.Error("Restarted part should be a tool request")
		}

		if restarted.ToolRequest == nil {
			t.Fatal("Restarted part should have ToolRequest")
		}

		if restarted.ToolRequest.Name != "__agent_test_tool_agent" {
			t.Errorf("Expected tool name '__agent_test_tool_agent', got '%s'", restarted.ToolRequest.Name)
		}

		inputMap, ok := restarted.ToolRequest.Input.(map[string]interface{})
		if !ok {
			t.Fatal("Restarted tool input should be a map")
		}
		if inputMap["task"] != "restart test" {
			t.Error("Restarted tool should preserve original input")
		}
	})

	t.Run("Restart method with nil tool request", func(t *testing.T) {
		toolReq := &ai.Part{
			Kind:        ai.PartToolRequest,
			ToolRequest: nil,
		}

		restarted := tool.Restart(toolReq, nil)

		if restarted == nil {
			t.Fatal("Restart should return a Part even with nil ToolRequest")
		}

		if !restarted.IsToolRequest() {
			t.Error("Restarted part should be a tool request")
		}

		if restarted.ToolRequest == nil {
			t.Fatal("Restarted part should have ToolRequest")
		}

		if restarted.ToolRequest.Name != "__agent_test_tool_agent" {
			t.Errorf("Expected tool name '__agent_test_tool_agent', got '%s'", restarted.ToolRequest.Name)
		}

		// Should have empty input as fallback
		inputMap, ok := restarted.ToolRequest.Input.(map[string]interface{})
		if !ok {
			t.Error("Restarted tool input should be a map")
		}
		if len(inputMap) != 0 {
			t.Error("Restarted tool should have empty input as fallback")
		}
	})
}
