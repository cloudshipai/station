package runtime

import (
	"context"
	"errors"
	"time"

	"station/internal/workflows"
)

var (
	ErrInvalidDuration = errors.New("invalid or missing duration for timer")
)

type StepStatusTimer StepStatus

const StepStatusWaitingTimer StepStatus = "waiting_timer"

type TimerExecutor struct{}

func NewTimerExecutor() *TimerExecutor {
	return &TimerExecutor{}
}

func (e *TimerExecutor) SupportedTypes() []workflows.ExecutionStepType {
	return []workflows.ExecutionStepType{workflows.StepTypeTimer}
}

func (e *TimerExecutor) Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
	raw := step.Raw

	durationStr := raw.Duration
	if durationStr == "" {
		if d, ok := raw.Input["duration"].(string); ok {
			durationStr = d
		}
	}

	if durationStr == "" {
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr("no duration specified for timer"),
		}, ErrInvalidDuration
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr("invalid duration format: " + err.Error()),
		}, ErrInvalidDuration
	}

	resumeAt := time.Now().Add(duration)

	runContext["_timerResumeAt"] = resumeAt.UTC().Format(time.RFC3339)
	runContext["_timerDuration"] = durationStr

	return StepResult{
		Status: StepStatusWaitingTimer,
		Output: map[string]interface{}{
			"duration":   durationStr,
			"resume_at":  resumeAt.UTC().Format(time.RFC3339),
			"started_at": time.Now().UTC().Format(time.RFC3339),
		},
		NextStep: step.Next,
		End:      step.End,
	}, nil
}

func (e *TimerExecutor) CheckTimerComplete(ctx context.Context, runContext map[string]interface{}) (bool, error) {
	resumeAtStr, ok := runContext["_timerResumeAt"].(string)
	if !ok {
		return false, nil
	}

	resumeAt, err := time.Parse(time.RFC3339, resumeAtStr)
	if err != nil {
		return false, err
	}

	return time.Now().After(resumeAt), nil
}
