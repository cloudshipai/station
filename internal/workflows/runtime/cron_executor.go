package runtime

import (
	"context"
	"time"

	"station/internal/workflows"
)

type CronExecutor struct{}

func NewCronExecutor() *CronExecutor {
	return &CronExecutor{}
}

func (e *CronExecutor) SupportedTypes() []workflows.ExecutionStepType {
	return []workflows.ExecutionStepType{workflows.StepTypeCron}
}

func (e *CronExecutor) Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
	raw := step.Raw

	if raw.Input != nil && len(raw.Input) > 0 {
		for k, v := range raw.Input {
			runContext[k] = v
		}
	}

	runContext["_cronTriggeredAt"] = time.Now().UTC().Format(time.RFC3339)

	if raw.Cron != "" {
		runContext["_cronExpression"] = raw.Cron
	}
	if raw.Timezone != "" {
		runContext["_cronTimezone"] = raw.Timezone
	}

	output := map[string]interface{}{
		"triggered_at": runContext["_cronTriggeredAt"],
	}
	if raw.Cron != "" {
		output["cron_expression"] = raw.Cron
	}

	return StepResult{
		Status:   StepStatusCompleted,
		Output:   output,
		NextStep: step.Next,
		End:      step.End,
	}, nil
}
