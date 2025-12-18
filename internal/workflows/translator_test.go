package workflows

import "testing"

func TestCompileExecutionPlanMapsStepTypes(t *testing.T) {
	def := &Definition{
		ID:    "demo",
		Start: "start",
		States: []StateSpec{
			{ID: "start", Type: "operation", Input: map[string]interface{}{"task": "agent.run"}, Transition: "tool"},
			{ID: "tool", Type: "operation", Input: map[string]interface{}{"task": "tool.call"}, Transition: "wait"},
			{ID: "wait", Type: "await", Transition: "done"},
			{ID: "done", Type: "operation", Input: map[string]interface{}{"task": "custom.run"}},
		},
	}

	plan := CompileExecutionPlan(def)
	if plan.Start != "start" {
		t.Fatalf("expected start 'start', got %s", plan.Start)
	}
	if plan.Steps["start"].Type != StepTypeAgent {
		t.Fatalf("expected start to map to agent, got %s", plan.Steps["start"].Type)
	}
	if plan.Steps["tool"].Type != StepTypeTool {
		t.Fatalf("expected tool step type %s, got %s", StepTypeTool, plan.Steps["tool"].Type)
	}
	if plan.Steps["wait"].Type != StepTypeAwait {
		t.Fatalf("expected await step, got %s", plan.Steps["wait"].Type)
	}
}
