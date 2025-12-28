package runtime

import (
	"context"
	"log/slog"

	"station/internal/workflows"
)

type CustomExecutor struct {
	logger *slog.Logger
}

func NewCustomExecutor(logger *slog.Logger) *CustomExecutor {
	if logger == nil {
		logger = slog.Default()
	}
	return &CustomExecutor{logger: logger}
}

func (e *CustomExecutor) SupportedTypes() []workflows.ExecutionStepType {
	return []workflows.ExecutionStepType{workflows.StepTypeCustom}
}

func (e *CustomExecutor) Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
	e.logger.Info("executing custom step (no-op placeholder)",
		"step_id", step.ID,
		"step_type", step.Raw.Type,
		"next_step", step.Next,
		"is_end", step.End,
	)

	output := map[string]interface{}{
		"message": "custom step completed (no-op)",
		"step_id": step.ID,
	}

	if step.Raw.Input != nil {
		output["input_received"] = step.Raw.Input
	}

	return StepResult{
		Status:   StepStatusCompleted,
		Output:   output,
		NextStep: step.Next,
		End:      step.End,
	}, nil
}
