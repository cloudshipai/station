package workflows

// ExecutionStepType classifies Station-supported workflow step kinds.
type ExecutionStepType string

const (
	StepTypeAgent     ExecutionStepType = "agent"
	StepTypeTool      ExecutionStepType = "tool"
	StepTypeCustom    ExecutionStepType = "custom"
	StepTypeAwait     ExecutionStepType = "await"
	StepTypeTimer     ExecutionStepType = "timer"
	StepTypeBranch    ExecutionStepType = "branch"
	StepTypeLoop      ExecutionStepType = "loop"
	StepTypeParallel  ExecutionStepType = "parallel"
	StepTypeTryCatch  ExecutionStepType = "trycatch"
	StepTypeContextOp ExecutionStepType = "context"
)

// ExecutionStep captures the minimal execution metadata the runtime needs.
type ExecutionStep struct {
	ID   string            `json:"id"`
	Type ExecutionStepType `json:"type"`
	Next string            `json:"next,omitempty"`
	Raw  StateSpec         `json:"raw"`
}

// ExecutionPlan is the compiled representation of a workflow definition.
type ExecutionPlan struct {
	Start string                   `json:"start"`
	Steps map[string]ExecutionStep `json:"steps"`
}

// CompileExecutionPlan converts a parsed definition into a runtime-friendly plan.
func CompileExecutionPlan(def *Definition) ExecutionPlan {
	plan := ExecutionPlan{
		Start: def.Start,
		Steps: make(map[string]ExecutionStep, len(def.States)),
	}

	for _, state := range def.States {
		stepType := classifyStep(state)
		next := state.Transition
		if next == "" {
			next = state.Next
		}

		plan.Steps[state.StableID()] = ExecutionStep{
			ID:   state.StableID(),
			Type: stepType,
			Next: next,
			Raw:  state,
		}
	}

	return plan
}

func classifyStep(state StateSpec) ExecutionStepType {
	switch state.Type {
	case "operation", "action", "function":
		// Map operation/action to agent/tool/custom based on task hints
		if task, ok := state.Input["task"]; ok {
			if taskStr, ok := task.(string); ok {
				if taskStr == "agent.run" || taskStr == "agent.hierarchy.run" {
					return StepTypeAgent
				}
				if taskStr == "tool.call" {
					return StepTypeTool
				}
				if taskStr == "custom.run" {
					return StepTypeCustom
				}
			}
		}
		return StepTypeCustom
	case "switch":
		return StepTypeBranch
	case "foreach", "while":
		return StepTypeLoop
	case "parallel":
		return StepTypeParallel
	case "sleep", "delay", "timer":
		return StepTypeTimer
	case "await", "await.signal", "await.event":
		return StepTypeAwait
	case "try":
		return StepTypeTryCatch
	case "set", "transform", "context":
		return StepTypeContextOp
	default:
		return StepTypeCustom
	}
}
