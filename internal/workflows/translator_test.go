package workflows

import "testing"

func TestCompileExecutionPlanMapsStepTypes(t *testing.T) {
	def := &Definition{
		ID:    "demo",
		Start: "start",
		States: []StateSpec{
			{ID: "start", Type: "operation", Input: map[string]interface{}{"task": "agent.run"}, Transition: "custom"},
			{ID: "custom", Type: "operation", Input: map[string]interface{}{"task": "custom.run"}, Transition: "wait"},
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
	if plan.Steps["custom"].Type != StepTypeCustom {
		t.Fatalf("expected custom step type %s, got %s", StepTypeCustom, plan.Steps["custom"].Type)
	}
	if plan.Steps["wait"].Type != StepTypeAwait {
		t.Fatalf("expected await step, got %s", plan.Steps["wait"].Type)
	}
}

func TestCompileExecutionPlanEndStates(t *testing.T) {
	tests := []struct {
		name     string
		state    StateSpec
		wantEnd  bool
		wantNext string
	}{
		{
			name:     "explicit end true",
			state:    StateSpec{ID: "final", Type: "operation", End: true},
			wantEnd:  true,
			wantNext: "",
		},
		{
			name:     "implicit end via empty next",
			state:    StateSpec{ID: "final", Type: "operation"},
			wantEnd:  true,
			wantNext: "",
		},
		{
			name:     "not end with transition",
			state:    StateSpec{ID: "step1", Type: "operation", Transition: "step2"},
			wantEnd:  false,
			wantNext: "step2",
		},
		{
			name:     "not end with next",
			state:    StateSpec{ID: "step1", Type: "operation", Next: "step2"},
			wantEnd:  false,
			wantNext: "step2",
		},
		{
			name:     "transition takes precedence over next",
			state:    StateSpec{ID: "step1", Type: "operation", Transition: "via-transition", Next: "via-next"},
			wantEnd:  false,
			wantNext: "via-transition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &Definition{
				ID:     "test",
				Start:  tt.state.ID,
				States: []StateSpec{tt.state},
			}

			plan := CompileExecutionPlan(def)
			step := plan.Steps[tt.state.ID]

			if step.End != tt.wantEnd {
				t.Errorf("End = %v, want %v", step.End, tt.wantEnd)
			}
			if step.Next != tt.wantNext {
				t.Errorf("Next = %q, want %q", step.Next, tt.wantNext)
			}
		})
	}
}
