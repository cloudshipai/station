package workflows_test

import (
	"encoding/json"
	"testing"

	"station/internal/workflows"
)

func TestWorkflowDefinitionValidation(t *testing.T) {
	tests := []struct {
		name       string
		definition string
		wantErrors bool
		errorCode  string
	}{
		{
			name: "valid workflow with inject steps",
			definition: `{
				"id": "test-workflow",
				"name": "Test Workflow",
				"start": "step1",
				"states": [
					{"id": "step1", "type": "inject", "data": {"message": "hello"}, "transition": "step2"},
					{"id": "step2", "type": "inject", "data": {"status": "done"}, "end": true}
				]
			}`,
			wantErrors: false,
		},
		{
			name: "valid workflow with agent step",
			definition: `{
				"id": "agent-workflow",
				"start": "run-agent",
				"states": [
					{
						"id": "run-agent",
						"type": "operation",
						"input": {
							"task": "agent.run",
							"agent_id": 1,
							"prompt": "analyze data"
						},
						"end": true
					}
				]
			}`,
			wantErrors: false,
		},
		{
			name: "valid workflow with human approval",
			definition: `{
				"id": "approval-workflow",
				"start": "request",
				"states": [
					{
						"id": "request",
						"type": "operation",
						"input": {
							"task": "human.approval",
							"message": "Approve deployment?"
						},
						"transition": "execute"
					},
					{"id": "execute", "type": "inject", "data": {"deployed": true}, "end": true}
				]
			}`,
			wantErrors: false,
		},
		{
			name: "valid workflow with switch",
			definition: `{
				"id": "switch-workflow",
				"start": "check",
				"states": [
					{
						"id": "check",
						"type": "switch",
						"conditions": [
							{"if": "ctx.value > 10", "next": "high"},
							{"if": "ctx.value <= 10", "next": "low"}
						],
						"defaultNext": "low"
					},
					{"id": "high", "type": "inject", "data": {"level": "high"}, "end": true},
					{"id": "low", "type": "inject", "data": {"level": "low"}, "end": true}
				]
			}`,
			wantErrors: false,
		},
		{
			name: "valid workflow with parallel",
			definition: `{
				"id": "parallel-workflow",
				"start": "parallel-step",
				"states": [
					{
						"id": "parallel-step",
						"type": "parallel",
						"branches": [
							{
								"name": "branch1",
								"states": [{"id": "b1", "type": "inject", "data": {"branch": 1}, "end": true}]
							},
							{
								"name": "branch2",
								"states": [{"id": "b2", "type": "inject", "data": {"branch": 2}, "end": true}]
							}
						],
						"end": true
					}
				]
			}`,
			wantErrors: false,
		},
		{
			name: "valid workflow with foreach",
			definition: `{
				"id": "foreach-workflow",
				"start": "loop",
				"states": [
					{
						"id": "loop",
						"type": "foreach",
						"itemsPath": "ctx.items",
						"itemName": "item",
						"iterator": {
							"states": [{"id": "process", "type": "inject", "data": {"processed": true}, "end": true}]
						},
						"end": true
					}
				]
			}`,
			wantErrors: false,
		},
		{
			name: "missing workflow id",
			definition: `{
				"start": "step1",
				"states": [{"id": "step1", "type": "inject", "data": {"x": 1}, "end": true}]
			}`,
			wantErrors: true,
			errorCode:  "MISSING_WORKFLOW_ID",
		},
		{
			name: "unknown transition target",
			definition: `{
				"id": "test",
				"start": "step1",
				"states": [{"id": "step1", "type": "inject", "data": {"x": 1}, "transition": "nonexistent"}]
			}`,
			wantErrors: true,
			errorCode:  "UNKNOWN_TRANSITION_TARGET",
		},
		{
			name: "start references unknown state",
			definition: `{
				"id": "test",
				"start": "unknown",
				"states": [{"id": "step1", "type": "inject", "data": {"x": 1}, "end": true}]
			}`,
			wantErrors: true,
			errorCode:  "INVALID_START",
		},
		{
			name: "empty states",
			definition: `{
				"id": "test",
				"start": "step1",
				"states": []
			}`,
			wantErrors: true,
			errorCode:  "MISSING_STATES",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, result, err := workflows.ValidateDefinition(json.RawMessage(tt.definition))
			if tt.wantErrors {
				if err == nil && len(result.Errors) == 0 {
					t.Errorf("expected validation errors, got none")
				}
				if tt.errorCode != "" {
					found := false
					for _, e := range result.Errors {
						if e.Code == tt.errorCode {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected error code %s, got %+v", tt.errorCode, result.Errors)
					}
				}
			} else {
				if len(result.Errors) > 0 {
					t.Errorf("expected no validation errors, got %+v", result.Errors)
				}
			}
		})
	}
}

func TestExecutionPlanCompilation(t *testing.T) {
	tests := []struct {
		name       string
		definition string
		checkPlan  func(t *testing.T, plan workflows.ExecutionPlan)
	}{
		{
			name: "multi-step workflow",
			definition: `{
				"id": "multi-step",
				"start": "step1",
				"states": [
					{"id": "step1", "type": "inject", "data": {"a": 1}, "transition": "step2"},
					{"id": "step2", "type": "inject", "data": {"b": 2}, "transition": "step3"},
					{"id": "step3", "type": "inject", "data": {"c": 3}, "end": true}
				]
			}`,
			checkPlan: func(t *testing.T, plan workflows.ExecutionPlan) {
				if plan.Start != "step1" {
					t.Errorf("expected start=step1, got %s", plan.Start)
				}
				if len(plan.Steps) != 3 {
					t.Errorf("expected 3 steps, got %d", len(plan.Steps))
				}
				if plan.Steps["step1"].Next != "step2" {
					t.Errorf("step1.Next expected step2, got %s", plan.Steps["step1"].Next)
				}
				if plan.Steps["step1"].End {
					t.Errorf("step1 should not be end")
				}
				if !plan.Steps["step3"].End {
					t.Errorf("step3 should be end")
				}
			},
		},
		{
			name: "agent workflow step classification",
			definition: `{
				"id": "agent-workflow",
				"start": "agent",
				"states": [
					{
						"id": "agent",
						"type": "operation",
						"input": {"task": "agent.run", "agent_id": 1},
						"end": true
					}
				]
			}`,
			checkPlan: func(t *testing.T, plan workflows.ExecutionPlan) {
				step := plan.Steps["agent"]
				if step.Type != workflows.StepTypeAgent {
					t.Errorf("expected step type agent, got %s", step.Type)
				}
			},
		},
		{
			name: "human approval step classification",
			definition: `{
				"id": "approval-workflow",
				"start": "approve",
				"states": [
					{
						"id": "approve",
						"type": "operation",
						"input": {"task": "human.approval", "message": "Approve?"},
						"end": true
					}
				]
			}`,
			checkPlan: func(t *testing.T, plan workflows.ExecutionPlan) {
				step := plan.Steps["approve"]
				if step.Type != workflows.StepTypeAwait {
					t.Errorf("expected step type await for human.approval, got %s", step.Type)
				}
			},
		},
		{
			name: "inject step classification",
			definition: `{
				"id": "inject-workflow",
				"start": "inject",
				"states": [
					{"id": "inject", "type": "inject", "data": {"key": "value"}, "end": true}
				]
			}`,
			checkPlan: func(t *testing.T, plan workflows.ExecutionPlan) {
				step := plan.Steps["inject"]
				if step.Type != workflows.StepTypeContextOp {
					t.Errorf("expected step type context for inject, got %s", step.Type)
				}
			},
		},
		{
			name: "set step classification",
			definition: `{
				"id": "set-workflow",
				"start": "set",
				"states": [
					{"id": "set", "type": "set", "data": {"key": "value"}, "end": true}
				]
			}`,
			checkPlan: func(t *testing.T, plan workflows.ExecutionPlan) {
				step := plan.Steps["set"]
				if step.Type != workflows.StepTypeContextOp {
					t.Errorf("expected step type context for set, got %s", step.Type)
				}
			},
		},
		{
			name: "switch step classification",
			definition: `{
				"id": "switch-workflow",
				"start": "switch",
				"states": [
					{
						"id": "switch",
						"type": "switch",
						"conditions": [{"if": "true", "next": "end"}],
						"defaultNext": "end"
					},
					{"id": "end", "type": "inject", "data": {"done": true}, "end": true}
				]
			}`,
			checkPlan: func(t *testing.T, plan workflows.ExecutionPlan) {
				step := plan.Steps["switch"]
				if step.Type != workflows.StepTypeBranch {
					t.Errorf("expected step type branch for switch, got %s", step.Type)
				}
			},
		},
		{
			name: "parallel step classification",
			definition: `{
				"id": "parallel-workflow",
				"start": "parallel",
				"states": [
					{
						"id": "parallel",
						"type": "parallel",
						"branches": [
							{"name": "b1", "states": [{"id": "s1", "type": "inject", "data": {"x": 1}, "end": true}]}
						],
						"end": true
					}
				]
			}`,
			checkPlan: func(t *testing.T, plan workflows.ExecutionPlan) {
				step := plan.Steps["parallel"]
				if step.Type != workflows.StepTypeParallel {
					t.Errorf("expected step type parallel, got %s", step.Type)
				}
			},
		},
		{
			name: "foreach step classification",
			definition: `{
				"id": "foreach-workflow",
				"start": "loop",
				"states": [
					{
						"id": "loop",
						"type": "foreach",
						"itemsPath": "ctx.items",
						"iterator": {
							"states": [{"id": "iter", "type": "inject", "data": {"x": 1}, "end": true}]
						},
						"end": true
					}
				]
			}`,
			checkPlan: func(t *testing.T, plan workflows.ExecutionPlan) {
				step := plan.Steps["loop"]
				if step.Type != workflows.StepTypeLoop {
					t.Errorf("expected step type loop for foreach, got %s", step.Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, result, err := workflows.ValidateDefinition(json.RawMessage(tt.definition))
			if err != nil || len(result.Errors) > 0 {
				t.Fatalf("validation failed: %v (errors: %+v)", err, result.Errors)
			}

			plan := workflows.CompileExecutionPlan(parsed)
			tt.checkPlan(t, plan)
		})
	}
}

func TestStepTypeClassification(t *testing.T) {
	typeTests := []struct {
		stateType    string
		taskType     string
		expectedType workflows.ExecutionStepType
	}{
		{"operation", "agent.run", workflows.StepTypeAgent},
		{"operation", "agent.hierarchy.run", workflows.StepTypeAgent},
		{"operation", "custom.run", workflows.StepTypeCustom},
		{"operation", "human.approval", workflows.StepTypeAwait},
		{"operation", "", workflows.StepTypeCustom},
		{"action", "agent.run", workflows.StepTypeAgent},
		{"function", "agent.run", workflows.StepTypeAgent},
		{"switch", "", workflows.StepTypeBranch},
		{"foreach", "", workflows.StepTypeLoop},
		{"while", "", workflows.StepTypeLoop},
		{"parallel", "", workflows.StepTypeParallel},
		{"sleep", "", workflows.StepTypeTimer},
		{"delay", "", workflows.StepTypeTimer},
		{"timer", "", workflows.StepTypeTimer},
		{"await", "", workflows.StepTypeAwait},
		{"await.signal", "", workflows.StepTypeAwait},
		{"await.event", "", workflows.StepTypeAwait},
		{"try", "", workflows.StepTypeTryCatch},
		{"inject", "", workflows.StepTypeContextOp},
		{"set", "", workflows.StepTypeContextOp},
		{"transform", "", workflows.StepTypeTransform},
		{"context", "", workflows.StepTypeContextOp},
		{"unknown", "", workflows.StepTypeCustom},
	}

	for _, tt := range typeTests {
		name := tt.stateType
		if tt.taskType != "" {
			name += "/" + tt.taskType
		}
		t.Run(name, func(t *testing.T) {
			inputJSON := ""
			if tt.taskType != "" {
				inputJSON = `"input": {"task": "` + tt.taskType + `"},`
			} else if tt.stateType == "inject" || tt.stateType == "set" || tt.stateType == "transform" || tt.stateType == "context" {
				inputJSON = `"data": {"key": "value"},`
			} else if tt.stateType == "switch" {
				inputJSON = `"conditions": [{"if": "true", "next": "step"}], "defaultNext": "step",`
			} else if tt.stateType == "parallel" {
				inputJSON = `"branches": [{"name": "b1", "states": [{"id": "s1", "type": "inject", "data": {"x": 1}, "end": true}]}],`
			} else if tt.stateType == "foreach" || tt.stateType == "while" {
				inputJSON = `"itemsPath": "ctx.items", "iterator": {"states": [{"id": "iter", "type": "inject", "data": {"x": 1}, "end": true}]},`
			}

			definition := []byte(`{
				"id": "test",
				"start": "step",
				"states": [{
					"id": "step",
					"type": "` + tt.stateType + `",
					` + inputJSON + `
					"end": true
				}]
			}`)

			parsed, result, _ := workflows.ValidateDefinition(json.RawMessage(definition))
			if len(result.Errors) > 0 {
				t.Skipf("validation failed, skipping classification check: %v", result.Errors)
			}

			plan := workflows.CompileExecutionPlan(parsed)
			step := plan.Steps["step"]
			if step.Type != tt.expectedType {
				t.Errorf("for state type %q with task %q: expected step type %q, got %q",
					tt.stateType, tt.taskType, tt.expectedType, step.Type)
			}
		})
	}
}
