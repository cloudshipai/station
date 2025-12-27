package runtime

import (
	"context"
	"errors"
	"testing"

	"station/internal/workflows"
)

type mockAgentDeps struct {
	agents       map[int64]AgentInfo
	agentsByName map[string]AgentInfo
	environments map[string]int64
	results      map[int64]AgentExecutionResult
	execErr      error
}

func (m *mockAgentDeps) GetAgentByID(ctx context.Context, id int64) (AgentInfo, error) {
	agent, ok := m.agents[id]
	if !ok {
		return AgentInfo{}, errors.New("agent not found")
	}
	return agent, nil
}

func (m *mockAgentDeps) GetAgentByNameAndEnvironment(ctx context.Context, name string, environmentID int64) (AgentInfo, error) {
	if m.agentsByName == nil {
		return AgentInfo{}, errors.New("agent not found")
	}
	// For mock, we'll assume name is unique or key is name
	// In real test we might want environment awareness but this mock is simple
	key := name
	agent, ok := m.agentsByName[key]
	if !ok {
		return AgentInfo{}, errors.New("agent not found in environment")
	}
	return agent, nil
}

func (m *mockAgentDeps) GetAgentByNameGlobal(ctx context.Context, name string) (AgentInfo, error) {
	if m.agentsByName == nil {
		return AgentInfo{}, errors.New("agent not found")
	}
	agent, ok := m.agentsByName[name]
	if !ok {
		return AgentInfo{}, errors.New("agent not found globally")
	}
	return agent, nil
}

func (m *mockAgentDeps) GetEnvironmentIDByName(ctx context.Context, name string) (int64, error) {
	if m.environments == nil {
		return 0, errors.New("environment not found")
	}
	id, ok := m.environments[name]
	if !ok {
		return 0, errors.New("environment not found")
	}
	return id, nil
}

func (m *mockAgentDeps) ExecuteAgent(ctx context.Context, agentID int64, task string, variables map[string]interface{}) (AgentExecutionResult, error) {
	if m.execErr != nil {
		return AgentExecutionResult{}, m.execErr
	}
	result, ok := m.results[agentID]
	if !ok {
		return AgentExecutionResult{Response: "executed"}, nil
	}
	return result, nil
}

func TestAgentRunExecutor_Execute(t *testing.T) {
	schemaRequiringNamespace := `{"type":"object","properties":{"namespace":{"type":"string"},"limit":{"type":"number"}},"required":["namespace"]}`

	deps := &mockAgentDeps{
		agents: map[int64]AgentInfo{
			1:  {ID: 1, Name: "test-agent"},
			2:  {ID: 2, Name: "analyzer"},
			10: {ID: 10, Name: "kubernetes-triage"},
			20: {ID: 20, Name: "schema-agent", InputSchema: &schemaRequiringNamespace},
		},
		agentsByName: map[string]AgentInfo{
			"kubernetes-triage": {ID: 10, Name: "kubernetes-triage"},
			"schema-agent":      {ID: 20, Name: "schema-agent", InputSchema: &schemaRequiringNamespace},
		},
		results: map[int64]AgentExecutionResult{
			1:  {Response: "Task completed successfully", StepCount: 3, ToolsUsed: 2},
			2:  {Response: "Analysis complete", StepCount: 5, ToolsUsed: 4},
			10: {Response: "Pods are healthy", StepCount: 2, ToolsUsed: 1},
			20: {Response: "Data processed", StepCount: 1, ToolsUsed: 0},
		},
	}

	executor := NewAgentRunExecutor(deps)

	tests := []struct {
		name        string
		step        workflows.ExecutionStep
		runContext  map[string]interface{}
		wantErr     bool
		errContains string
		checkOutput func(t *testing.T, result StepResult)
	}{
		{
			name: "successful execution with agent_id as float64",
			step: workflows.ExecutionStep{
				ID:   "run-agent",
				Type: workflows.StepTypeAgent,
				Next: "next-step",
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"agent_id": float64(1),
						"task":     "analyze data",
					},
				},
			},
			runContext: map[string]interface{}{"user": "test"},
			checkOutput: func(t *testing.T, result StepResult) {
				if result.Error != nil {
					t.Errorf("expected no error, got %s", *result.Error)
				}
				if result.NextStep != "next-step" {
					t.Errorf("expected next_step=next-step, got %s", result.NextStep)
				}
				if result.Output["response"] != "Task completed successfully" {
					t.Errorf("unexpected response: %v", result.Output["response"])
				}
				if result.Output["agent_name"] != "test-agent" {
					t.Errorf("expected agent_name=test-agent, got %v", result.Output["agent_name"])
				}
			},
		},
		{
			name: "successful execution with agent_id as int64",
			step: workflows.ExecutionStep{
				ID:   "run-agent-2",
				Type: workflows.StepTypeAgent,
				Next: "done",
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"agent_id": int64(2),
						"task":     "run analysis",
					},
				},
			},
			checkOutput: func(t *testing.T, result StepResult) {
				if result.Output["response"] != "Analysis complete" {
					t.Errorf("unexpected response: %v", result.Output["response"])
				}
				if result.Output["step_count"] != int64(5) {
					t.Errorf("expected step_count=5, got %v", result.Output["step_count"])
				}
			},
		},
		{
			name: "missing agent identifier",
			step: workflows.ExecutionStep{
				ID:   "no-agent",
				Type: workflows.StepTypeAgent,
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"task": "do something",
					},
				},
			},
			wantErr:     true,
			errContains: "agent name is required",
		},
		{
			name: "invalid agent_id type",
			step: workflows.ExecutionStep{
				ID:   "bad-agent-id",
				Type: workflows.StepTypeAgent,
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"agent_id": "not-a-number",
					},
				},
			},
			wantErr:     true,
			errContains: "invalid agent_id",
		},
		{
			name: "agent not found",
			step: workflows.ExecutionStep{
				ID:   "missing-agent",
				Type: workflows.StepTypeAgent,
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"agent_id": float64(999),
					},
				},
			},
			wantErr:     true,
			errContains: "agent not found",
		},
		{
			name: "default task when not provided",
			step: workflows.ExecutionStep{
				ID:   "default-task",
				Type: workflows.StepTypeAgent,
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"agent_id": float64(1),
					},
				},
			},
			checkOutput: func(t *testing.T, result StepResult) {
				if result.Error != nil {
					t.Errorf("expected no error, got %s", *result.Error)
				}
			},
		},
		{
			name: "successful execution with agent name",
			step: workflows.ExecutionStep{
				ID:   "run-by-name",
				Type: workflows.StepTypeAgent,
				Next: "next-step",
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"agent": "kubernetes-triage",
						"task":  "check pod health",
					},
				},
			},
			runContext: map[string]interface{}{"_environmentID": int64(1)},
			checkOutput: func(t *testing.T, result StepResult) {
				if result.Error != nil {
					t.Errorf("expected no error, got %s", *result.Error)
				}
				if result.NextStep != "next-step" {
					t.Errorf("expected next_step=next-step, got %s", result.NextStep)
				}
				if result.Output["agent_name"] != "kubernetes-triage" {
					t.Errorf("expected agent_name=kubernetes-triage, got %v", result.Output["agent_name"])
				}
				if result.Output["agent_id"] != int64(10) {
					t.Errorf("expected agent_id=10, got %v", result.Output["agent_id"])
				}
			},
		},
		{
			name: "global agent resolution without environment ID",
			step: workflows.ExecutionStep{
				ID:   "global-lookup",
				Type: workflows.StepTypeAgent,
				Next: "next-step",
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"agent": "kubernetes-triage",
						"task":  "check pods",
					},
				},
			},
			runContext: map[string]interface{}{},
			checkOutput: func(t *testing.T, result StepResult) {
				if result.Error != nil {
					t.Errorf("expected no error, got %s", *result.Error)
				}
				if result.Output["agent_name"] != "kubernetes-triage" {
					t.Errorf("expected agent_name=kubernetes-triage, got %v", result.Output["agent_name"])
				}
			},
		},
		{
			name: "agent not found by name",
			step: workflows.ExecutionStep{
				ID:   "missing-by-name",
				Type: workflows.StepTypeAgent,
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"agent": "nonexistent-agent",
						"task":  "do something",
					},
				},
			},
			runContext:  map[string]interface{}{"_environmentID": int64(1)},
			wantErr:     true,
			errContains: "agent not found",
		},
		{
			name: "agent with input schema - valid input",
			step: workflows.ExecutionStep{
				ID:   "schema-valid",
				Type: workflows.StepTypeAgent,
				Next: "done",
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"agent": "schema-agent",
						"task":  "process data",
						"variables": map[string]interface{}{
							"namespace": "production",
							"limit":     float64(10),
						},
					},
				},
			},
			runContext: map[string]interface{}{"_environmentID": int64(1)},
			checkOutput: func(t *testing.T, result StepResult) {
				if result.Error != nil {
					t.Errorf("expected no error, got %s", *result.Error)
				}
				if result.Output["agent_name"] != "schema-agent" {
					t.Errorf("expected agent_name=schema-agent, got %v", result.Output["agent_name"])
				}
			},
		},
		{
			name: "agent with input schema - missing required field",
			step: workflows.ExecutionStep{
				ID:   "schema-invalid",
				Type: workflows.StepTypeAgent,
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"agent": "schema-agent",
						"task":  "process data",
						"variables": map[string]interface{}{
							"limit": float64(10),
						},
					},
				},
			},
			runContext:  map[string]interface{}{"_environmentID": int64(1)},
			wantErr:     true,
			errContains: "missing required field: namespace",
		},
		{
			name: "explicit environment override",
			step: workflows.ExecutionStep{
				ID:   "env-override",
				Type: workflows.StepTypeAgent,
				Next: "done",
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"agent": "schema-agent@production",
						"task":  "process data",
						"variables": map[string]interface{}{
							"namespace": "prod-namespace",
						},
					},
				},
			},
			runContext: map[string]interface{}{"_environmentID": int64(1)}, // default env
			checkOutput: func(t *testing.T, result StepResult) {
				if result.Error != nil {
					t.Errorf("expected no error, got %s", *result.Error)
				}
				if result.Output["agent_name"] != "schema-agent" {
					t.Errorf("expected agent_name=schema-agent, got %v", result.Output["agent_name"])
				}
			},
		},
		{
			name: "JSONPath variables resolved before schema validation",
			step: workflows.ExecutionStep{
				ID:   "jsonpath-resolution",
				Type: workflows.StepTypeAgent,
				Next: "done",
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"agent": "schema-agent",
						"task":  "process data",
						"variables": map[string]interface{}{
							"namespace": "$.ns_from_context",
							"limit":     float64(5),
						},
					},
				},
			},
			runContext: map[string]interface{}{
				"_environmentID":  int64(1),
				"ns_from_context": "resolved-namespace",
			},
			checkOutput: func(t *testing.T, result StepResult) {
				if result.Error != nil {
					t.Errorf("expected no error (JSONPath should resolve before validation), got %s", *result.Error)
				}
				if result.Output["agent_name"] != "schema-agent" {
					t.Errorf("expected agent_name=schema-agent, got %v", result.Output["agent_name"])
				}
			},
		},
	}

	deps.environments = map[string]int64{
		"production": 20,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.Execute(context.Background(), tt.step, tt.runContext)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !errors.Is(err, ErrAgentNotFound) && !errors.Is(err, ErrAgentIDRequired) && !errors.Is(err, ErrInvalidAgentID) {
					if err.Error() != tt.errContains && !contains(err.Error(), tt.errContains) {
						t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
					}
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, result)
			}
		})
	}
}

func TestAgentRunExecutor_ExecutionError(t *testing.T) {
	deps := &mockAgentDeps{
		agents: map[int64]AgentInfo{
			1: {ID: 1, Name: "failing-agent"},
		},
		execErr: errors.New("execution timeout"),
	}

	executor := NewAgentRunExecutor(deps)

	step := workflows.ExecutionStep{
		ID:   "fail-step",
		Type: workflows.StepTypeAgent,
		Next: "error-handler",
		Raw: workflows.StateSpec{
			Input: map[string]interface{}{
				"agent_id": float64(1),
				"task":     "will fail",
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, nil)

	if err == nil {
		t.Error("expected error, got nil")
	}

	if !errors.Is(err, ErrExecutionFailed) {
		t.Errorf("expected ErrExecutionFailed, got %v", err)
	}

	if result.Error == nil {
		t.Error("expected result.Error to be set")
	} else if *result.Error != "execution timeout" {
		t.Errorf("expected error message 'execution timeout', got %s", *result.Error)
	}

	if result.NextStep != "error-handler" {
		t.Errorf("expected NextStep=error-handler, got %s", result.NextStep)
	}
}

func TestAgentRunExecutor_SupportedTypes(t *testing.T) {
	executor := NewAgentRunExecutor(nil)
	types := executor.SupportedTypes()

	if len(types) != 1 {
		t.Errorf("expected 1 supported type, got %d", len(types))
	}

	if types[0] != workflows.StepTypeAgent {
		t.Errorf("expected StepTypeAgent, got %v", types[0])
	}
}

func TestExecutorRegistry(t *testing.T) {
	registry := NewExecutorRegistry()

	deps := &mockAgentDeps{
		agents: map[int64]AgentInfo{
			1: {ID: 1, Name: "test"},
		},
	}
	agentExecutor := NewAgentRunExecutor(deps)
	registry.Register(agentExecutor)

	t.Run("get registered executor", func(t *testing.T) {
		executor, err := registry.GetExecutor(workflows.StepTypeAgent)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if executor == nil {
			t.Error("expected non-nil executor")
		}
	})

	t.Run("get unregistered executor", func(t *testing.T) {
		_, err := registry.GetExecutor(workflows.StepTypeParallel)
		if err == nil {
			t.Error("expected error for unregistered executor")
		}
		if !errors.Is(err, ErrExecutorNotFound) {
			t.Errorf("expected ErrExecutorNotFound, got %v", err)
		}
	})

	t.Run("execute via registry", func(t *testing.T) {
		step := workflows.ExecutionStep{
			ID:   "test-step",
			Type: workflows.StepTypeAgent,
			Raw: workflows.StateSpec{
				Input: map[string]interface{}{
					"agent_id": float64(1),
				},
			},
		}

		result, err := registry.Execute(context.Background(), step, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Output == nil {
			t.Error("expected non-nil output")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type mockApprovalDeps struct {
	approvals map[string]ApprovalInfo
	createErr error
	getErr    error
}

func (m *mockApprovalDeps) CreateApproval(ctx context.Context, params CreateApprovalParams) (ApprovalInfo, error) {
	if m.createErr != nil {
		return ApprovalInfo{}, m.createErr
	}
	info := ApprovalInfo{
		ID:     params.ApprovalID,
		Status: "pending",
	}
	if m.approvals == nil {
		m.approvals = make(map[string]ApprovalInfo)
	}
	m.approvals[params.ApprovalID] = info
	return info, nil
}

func (m *mockApprovalDeps) GetApproval(ctx context.Context, approvalID string) (ApprovalInfo, error) {
	if m.getErr != nil {
		return ApprovalInfo{}, m.getErr
	}
	info, ok := m.approvals[approvalID]
	if !ok {
		return ApprovalInfo{}, errors.New("approval not found")
	}
	return info, nil
}

func TestHumanApprovalExecutor_Execute(t *testing.T) {
	deps := &mockApprovalDeps{}
	executor := NewHumanApprovalExecutor(deps)
	runContext := map[string]interface{}{"_runID": "run-123"}

	tests := []struct {
		name       string
		step       workflows.ExecutionStep
		runCtx     map[string]interface{}
		wantStatus StepStatus
		wantErr    bool
	}{
		{
			name: "creates pending approval",
			step: workflows.ExecutionStep{
				ID:   "approval-step",
				Type: workflows.StepTypeAwait,
				Next: "next-step",
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"task":            "human.approval",
						"message":         "Approve deployment?",
						"timeout_seconds": float64(3600),
					},
				},
			},
			runCtx:     runContext,
			wantStatus: StepStatusWaitingApproval,
			wantErr:    false,
		},
		{
			name: "skips non-approval actions",
			step: workflows.ExecutionStep{
				ID:   "other-step",
				Type: workflows.StepTypeAwait,
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"task": "something.else",
					},
				},
			},
			runCtx:     runContext,
			wantStatus: StepStatusCompleted,
			wantErr:    false,
		},
		{
			name: "requires message",
			step: workflows.ExecutionStep{
				ID:   "no-message",
				Type: workflows.StepTypeAwait,
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"task": "human.approval",
					},
				},
			},
			runCtx:     runContext,
			wantStatus: StepStatusFailed,
			wantErr:    true,
		},
		{
			name: "requires runID in context",
			step: workflows.ExecutionStep{
				ID:   "approval-step",
				Type: workflows.StepTypeAwait,
				Raw: workflows.StateSpec{
					Input: map[string]interface{}{
						"task":    "human.approval",
						"message": "Approve?",
					},
				},
			},
			runCtx:     nil,
			wantStatus: StepStatusFailed,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.Execute(context.Background(), tt.step, tt.runCtx)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", result.Status, tt.wantStatus)
			}
		})
	}
}

func TestHumanApprovalExecutor_HandleDecision(t *testing.T) {
	step := workflows.ExecutionStep{
		ID:   "approval-step",
		Next: "next-step",
		End:  false,
	}

	tests := []struct {
		name       string
		approval   ApprovalInfo
		wantStatus StepStatus
		wantErr    error
	}{
		{
			name: "approved",
			approval: ApprovalInfo{
				ID:             "appr-1",
				Status:         "approved",
				DecidedBy:      "alice",
				DecisionReason: "Looks good",
			},
			wantStatus: StepStatusApproved,
			wantErr:    nil,
		},
		{
			name: "rejected",
			approval: ApprovalInfo{
				ID:             "appr-2",
				Status:         "rejected",
				DecidedBy:      "bob",
				DecisionReason: "Not ready",
			},
			wantStatus: StepStatusRejected,
			wantErr:    ErrApprovalRejected,
		},
		{
			name: "timed out",
			approval: ApprovalInfo{
				ID:     "appr-3",
				Status: "timed_out",
			},
			wantStatus: StepStatusTimedOut,
			wantErr:    ErrApprovalTimedOut,
		},
		{
			name: "still pending",
			approval: ApprovalInfo{
				ID:     "appr-4",
				Status: "pending",
			},
			wantStatus: StepStatusWaitingApproval,
			wantErr:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := &mockApprovalDeps{
				approvals: map[string]ApprovalInfo{
					tt.approval.ID: tt.approval,
				},
			}
			executor := NewHumanApprovalExecutor(deps)

			result, err := executor.HandleApprovalDecision(context.Background(), tt.approval.ID, step)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", result.Status, tt.wantStatus)
			}
		})
	}
}

func TestHumanApprovalExecutor_SupportedTypes(t *testing.T) {
	executor := NewHumanApprovalExecutor(nil)
	types := executor.SupportedTypes()

	if len(types) != 1 {
		t.Errorf("expected 1 supported type, got %d", len(types))
	}

	if types[0] != workflows.StepTypeAwait {
		t.Errorf("expected StepTypeAwait, got %v", types[0])
	}
}
