package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"station/pkg/models"
)

func TestValidateDefinitionRequiresID(t *testing.T) {
	raw := json.RawMessage(`{"states":[{"id":"s1","type":"operation"}]}`)
	_, result, err := ValidateDefinition(raw)
	if err == nil {
		t.Fatalf("expected validation error for missing id")
	}
	if len(result.Errors) == 0 {
		t.Fatalf("expected at least one validation error for missing id")
	}
}

func TestValidateDefinitionValidatesTransitions(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "demo",
		"states": [
			{"id":"start","type":"operation","transition":"missing-target"}
		]
	}`)

	_, result, err := ValidateDefinition(raw)
	if err == nil {
		t.Fatalf("expected validation error for unknown transition target")
	}
	found := false
	for _, issue := range result.Errors {
		if issue.Code == "UNKNOWN_TRANSITION_TARGET" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected UNKNOWN_TRANSITION_TARGET error, got %+v", result.Errors)
	}
}

func TestValidateDefinitionAllowsWarnings(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "demo",
		"start": "start",
		"states": [
			{"id":"start","type":"operation","input":{},"output":{},"retry":{"max_attempts":3},"timeout":"5m","transition":"finish"},
			{"id":"finish","type":"operation","input":{},"output":{}}
		]
	}`)

	_, result, err := ValidateDefinition(raw)
	if err != nil {
		t.Fatalf("expected no validation errors, got %v (%+v)", err, result.Errors)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected zero errors, got %d", len(result.Errors))
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected warnings for optional fields")
	}
}

type mockAgentLookup struct {
	agents       map[string]*models.Agent
	environments map[string]int64
	err          error
}

func (m *mockAgentLookup) GetAgentByNameAndEnvironment(ctx context.Context, name string, environmentID int64) (*models.Agent, error) {
	if m.err != nil {
		return nil, m.err
	}
	if agent, ok := m.agents[name]; ok {
		return agent, nil
	}
	return nil, errors.New("agent not found")
}

func (m *mockAgentLookup) GetAgentByNameGlobal(ctx context.Context, name string) (*models.Agent, error) {
	if m.err != nil {
		return nil, m.err
	}
	if agent, ok := m.agents[name]; ok {
		return agent, nil
	}
	return nil, errors.New("agent not found globally")
}

func (m *mockAgentLookup) GetEnvironmentIDByName(ctx context.Context, name string) (int64, error) {
	if m.environments == nil {
		return 0, errors.New("environment not found")
	}
	if id, ok := m.environments[name]; ok {
		return id, nil
	}
	return 0, errors.New("environment not found")
}

func TestAgentValidator_AllAgentsExist(t *testing.T) {
	lookup := &mockAgentLookup{
		agents: map[string]*models.Agent{
			"triage-agent": {ID: 1, Name: "triage-agent"},
			"deploy-agent": {ID: 2, Name: "deploy-agent"},
		},
	}
	validator := NewAgentValidator(lookup)

	def := &Definition{
		ID: "test-workflow",
		States: []StateSpec{
			{
				ID:   "step1",
				Type: "operation",
				Input: map[string]interface{}{
					"task":  "agent.run",
					"agent": "triage-agent",
				},
				Transition: "step2",
			},
			{
				ID:   "step2",
				Type: "operation",
				Input: map[string]interface{}{
					"task":  "agent.run",
					"agent": "deploy-agent",
				},
			},
		},
	}

	result := validator.ValidateAgents(context.Background(), def, 1)

	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %d: %+v", len(result.Errors), result.Errors)
	}
}

func TestAgentValidator_AgentNotFound(t *testing.T) {
	lookup := &mockAgentLookup{
		agents: map[string]*models.Agent{
			"triage-agent": {ID: 1, Name: "triage-agent"},
		},
	}
	validator := NewAgentValidator(lookup)

	def := &Definition{
		ID: "test-workflow",
		States: []StateSpec{
			{
				ID:   "step1",
				Type: "operation",
				Input: map[string]interface{}{
					"task":  "agent.run",
					"agent": "nonexistent-agent",
				},
			},
		},
	}

	result := validator.ValidateAgents(context.Background(), def, 1)

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %+v", len(result.Errors), result.Errors)
	}

	if result.Errors[0].Code != "AGENT_NOT_FOUND" {
		t.Errorf("expected AGENT_NOT_FOUND error, got %s", result.Errors[0].Code)
	}

	if result.Errors[0].Actual != "nonexistent-agent" {
		t.Errorf("expected Actual to be 'nonexistent-agent', got %s", result.Errors[0].Actual)
	}
}

func TestAgentValidator_AgentNameRequired(t *testing.T) {
	lookup := &mockAgentLookup{agents: map[string]*models.Agent{}}
	validator := NewAgentValidator(lookup)

	def := &Definition{
		ID: "test-workflow",
		States: []StateSpec{
			{
				ID:   "step1",
				Type: "operation",
				Input: map[string]interface{}{
					"task": "agent.run",
				},
			},
		},
	}

	result := validator.ValidateAgents(context.Background(), def, 1)

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %+v", len(result.Errors), result.Errors)
	}

	if result.Errors[0].Code != "AGENT_NAME_REQUIRED" {
		t.Errorf("expected AGENT_NAME_REQUIRED error, got %s", result.Errors[0].Code)
	}
}

func TestAgentValidator_SchemaCompatibility(t *testing.T) {
	outputSchema := `{"type":"object","properties":{"status":{"type":"string"}},"required":["status"]}`
	inputSchema := `{"type":"object","properties":{"status":{"type":"string"},"count":{"type":"integer"}},"required":["status","count"]}`

	lookup := &mockAgentLookup{
		agents: map[string]*models.Agent{
			"producer": {ID: 1, Name: "producer", OutputSchema: &outputSchema},
			"consumer": {ID: 2, Name: "consumer", InputSchema: &inputSchema},
		},
	}
	validator := NewAgentValidator(lookup)

	def := &Definition{
		ID: "test-workflow",
		States: []StateSpec{
			{
				ID:   "produce",
				Type: "operation",
				Input: map[string]interface{}{
					"task":  "agent.run",
					"agent": "producer",
				},
				Transition: "consume",
			},
			{
				ID:   "consume",
				Type: "operation",
				Input: map[string]interface{}{
					"task":  "agent.run",
					"agent": "consumer",
				},
			},
		},
	}

	result := validator.ValidateAgents(context.Background(), def, 1)

	if len(result.Errors) == 0 {
		t.Fatal("expected schema incompatibility error")
	}

	foundSchemaError := false
	for _, err := range result.Errors {
		if err.Code == "SCHEMA_INCOMPATIBLE" {
			foundSchemaError = true
			break
		}
	}

	if !foundSchemaError {
		t.Errorf("expected SCHEMA_INCOMPATIBLE error, got: %+v", result.Errors)
	}
}

func TestAgentValidator_SchemaWarningNoInputSchema(t *testing.T) {
	outputSchema := `{"type":"object","properties":{"status":{"type":"string"}}}`

	lookup := &mockAgentLookup{
		agents: map[string]*models.Agent{
			"producer": {ID: 1, Name: "producer", OutputSchema: &outputSchema},
			"consumer": {ID: 2, Name: "consumer"},
		},
	}
	validator := NewAgentValidator(lookup)

	def := &Definition{
		ID: "test-workflow",
		States: []StateSpec{
			{
				ID:   "produce",
				Type: "operation",
				Input: map[string]interface{}{
					"task":  "agent.run",
					"agent": "producer",
				},
				Transition: "consume",
			},
			{
				ID:   "consume",
				Type: "operation",
				Input: map[string]interface{}{
					"task":  "agent.run",
					"agent": "consumer",
				},
			},
		},
	}

	result := validator.ValidateAgents(context.Background(), def, 1)

	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %d: %+v", len(result.Errors), result.Errors)
	}

	foundWarning := false
	for _, warn := range result.Warnings {
		if warn.Code == "SCHEMA_NO_INPUT" {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Errorf("expected SCHEMA_NO_INPUT warning, got: %+v", result.Warnings)
	}
}

func TestAgentValidator_NestedBranchAgents(t *testing.T) {
	lookup := &mockAgentLookup{
		agents: map[string]*models.Agent{
			"branch-agent": {ID: 1, Name: "branch-agent"},
		},
	}
	validator := NewAgentValidator(lookup)

	def := &Definition{
		ID: "test-workflow",
		States: []StateSpec{
			{
				ID:   "parallel-step",
				Type: "parallel",
				Branches: []BranchSpec{
					{
						States: []StateSpec{
							{
								ID:   "nested-step",
								Type: "operation",
								Input: map[string]interface{}{
									"task":  "agent.run",
									"agent": "missing-nested-agent",
								},
							},
						},
					},
				},
			},
		},
	}

	result := validator.ValidateAgents(context.Background(), def, 1)

	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error for nested agent, got %d: %+v", len(result.Errors), result.Errors)
	}

	if result.Errors[0].Code != "AGENT_NOT_FOUND" {
		t.Errorf("expected AGENT_NOT_FOUND, got %s", result.Errors[0].Code)
	}
}

func TestAgentValidator_ExplicitEnvironmentOverride(t *testing.T) {
	lookup := &mockAgentLookup{
		agents: map[string]*models.Agent{
			"prod-agent": {ID: 1, Name: "prod-agent"},
		},
		environments: map[string]int64{
			"production": 20,
		},
	}
	validator := NewAgentValidator(lookup)

	def := &Definition{
		ID: "test-workflow",
		States: []StateSpec{
			{
				ID:   "step1",
				Type: "operation",
				Input: map[string]interface{}{
					"task":  "agent.run",
					"agent": "prod-agent@production",
				},
			},
		},
	}

	// Should pass if prod-agent is found in environment 20
	// Note: mockAgentLookup.GetAgentByNameAndEnvironment simply checks map by name,
	// effectively ignoring environmentID but confirming lookup was called.
	result := validator.ValidateAgents(context.Background(), def, 1) // calling from env 1

	if len(result.Errors) != 0 {
		t.Errorf("expected no errors for valid override, got %d: %+v", len(result.Errors), result.Errors)
	}
}
