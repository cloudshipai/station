package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"station/internal/workflows"
)

type WorkflowRunUpdater interface {
	UpdateRunStatus(ctx context.Context, runID, status string, currentStep *string, err *string) error
	GetRunContext(ctx context.Context, runID string) (map[string]interface{}, error)
	UpdateRunContext(ctx context.Context, runID string, context map[string]interface{}) error
	CompleteRun(ctx context.Context, runID string, result map[string]interface{}) error
	FailRun(ctx context.Context, runID string, errMsg string) error
}

type StepRecorder interface {
	RecordStepStart(ctx context.Context, runID, stepID string, stepType string) error
	RecordStepComplete(ctx context.Context, runID, stepID string, output map[string]interface{}) error
	RecordStepFailed(ctx context.Context, runID, stepID string, errMsg string) error
	RecordStepWaiting(ctx context.Context, runID, stepID string, approvalID string) error
}

type StepProvider interface {
	GetStep(ctx context.Context, runID, stepID string) (workflows.ExecutionStep, error)
}

type WorkflowConsumer struct {
	engine       *NATSEngine
	registry     *ExecutorRegistry
	runUpdater   WorkflowRunUpdater
	stepRecorder StepRecorder
	stepProvider StepProvider

	mu           sync.Mutex
	subscription *nats.Subscription
	running      bool
	stopCh       chan struct{}
}

func NewWorkflowConsumer(
	engine *NATSEngine,
	registry *ExecutorRegistry,
	runUpdater WorkflowRunUpdater,
	stepRecorder StepRecorder,
	stepProvider StepProvider,
) *WorkflowConsumer {
	return &WorkflowConsumer{
		engine:       engine,
		registry:     registry,
		runUpdater:   runUpdater,
		stepRecorder: stepRecorder,
		stepProvider: stepProvider,
		stopCh:       make(chan struct{}),
	}
}

func (c *WorkflowConsumer) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	if c.engine == nil {
		log.Println("Workflow consumer: NATS engine not configured, running in no-op mode")
		return nil
	}

	subject := fmt.Sprintf("%s.run.*.step.*.schedule", c.engine.opts.SubjectPrefix)
	log.Printf("Workflow consumer: subscribing to %s", subject)

	sub, err := c.engine.SubscribeDurable(subject, "workflow-step-consumer", c.handleMessage)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	c.subscription = sub
	c.running = true

	log.Println("Workflow consumer: started successfully")
	return nil
}

func (c *WorkflowConsumer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	if c.subscription != nil {
		_ = c.subscription.Drain()
	}

	close(c.stopCh)
	c.running = false
	log.Println("Workflow consumer: stopped")
}

func (c *WorkflowConsumer) handleMessage(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var step workflows.ExecutionStep
	if err := json.Unmarshal(msg.Data, &step); err != nil {
		log.Printf("Workflow consumer: failed to unmarshal step: %v", err)
		_ = msg.Nak()
		return
	}

	runID := extractRunIDFromSubject(msg.Subject)
	if runID == "" {
		log.Printf("Workflow consumer: could not extract runID from subject: %s", msg.Subject)
		_ = msg.Nak()
		return
	}

	log.Printf("Workflow consumer: executing step %s for run %s (type: %s)", step.ID, runID, step.Type)

	if err := c.executeStep(ctx, runID, step); err != nil {
		log.Printf("Workflow consumer: step %s failed: %v", step.ID, err)
	}

	_ = msg.Ack()
}

func (c *WorkflowConsumer) executeStep(ctx context.Context, runID string, step workflows.ExecutionStep) error {
	currentStep := step.ID
	if err := c.runUpdater.UpdateRunStatus(ctx, runID, "running", &currentStep, nil); err != nil {
		log.Printf("Workflow consumer: failed to update run status to running: %v", err)
	}

	if err := c.stepRecorder.RecordStepStart(ctx, runID, step.ID, string(step.Type)); err != nil {
		log.Printf("Workflow consumer: failed to record step start: %v", err)
	}

	runContext, err := c.runUpdater.GetRunContext(ctx, runID)
	if err != nil {
		log.Printf("Workflow consumer: failed to get run context: %v", err)
		runContext = make(map[string]interface{})
	}
	runContext["_runID"] = runID

	stepInput := c.resolveStepInput(step, runContext)
	runContext["_stepInput"] = stepInput

	result, execErr := c.registry.Execute(ctx, step, runContext)

	if execErr != nil {
		errStr := execErr.Error()
		_ = c.stepRecorder.RecordStepFailed(ctx, runID, step.ID, errStr)
		_ = c.runUpdater.FailRun(ctx, runID, errStr)
		return execErr
	}

	switch result.Status {
	case StepStatusCompleted, StepStatusApproved:
		_ = c.stepRecorder.RecordStepComplete(ctx, runID, step.ID, result.Output)

		if result.Output != nil {
			updatedContext := c.storeStepOutput(runContext, step.ID, result.Output)
			_ = c.runUpdater.UpdateRunContext(ctx, runID, updatedContext)
		}

		if result.End || result.NextStep == "" {
			_ = c.runUpdater.CompleteRun(ctx, runID, result.Output)
			log.Printf("Workflow consumer: run %s completed", runID)
		} else {
			_ = c.scheduleNextStep(ctx, runID, result.NextStep)
		}

	case StepStatusWaitingApproval:
		_ = c.stepRecorder.RecordStepWaiting(ctx, runID, step.ID, result.ApprovalID)
		_ = c.runUpdater.UpdateRunStatus(ctx, runID, "waiting_approval", &currentStep, nil)
		log.Printf("Workflow consumer: step %s waiting for approval %s", step.ID, result.ApprovalID)

	case StepStatusFailed, StepStatusRejected, StepStatusTimedOut:
		errMsg := "step failed"
		if result.Error != nil {
			errMsg = *result.Error
		}
		_ = c.stepRecorder.RecordStepFailed(ctx, runID, step.ID, errMsg)
		_ = c.runUpdater.FailRun(ctx, runID, errMsg)
	}

	return nil
}

func (c *WorkflowConsumer) resolveStepInput(step workflows.ExecutionStep, runContext map[string]interface{}) map[string]interface{} {
	if steps, ok := runContext["steps"].(map[string]interface{}); ok {
		var lastStepOutput map[string]interface{}
		for _, stepData := range steps {
			if sd, ok := stepData.(map[string]interface{}); ok {
				if output, ok := sd["output"].(map[string]interface{}); ok {
					lastStepOutput = output
				}
			}
		}
		if lastStepOutput != nil {
			return lastStepOutput
		}
	}

	if workflow, ok := runContext["workflow"].(map[string]interface{}); ok {
		if input, ok := workflow["input"].(map[string]interface{}); ok {
			return input
		}
	}

	result := make(map[string]interface{})
	for k, v := range runContext {
		if k != "_runID" && k != "_environmentID" && k != "_stepInput" && k != "steps" && k != "workflow" {
			result[k] = v
		}
	}
	return result
}

func (c *WorkflowConsumer) storeStepOutput(runContext map[string]interface{}, stepID string, output map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range runContext {
		result[k] = v
	}

	steps, ok := result["steps"].(map[string]interface{})
	if !ok {
		steps = make(map[string]interface{})
	}

	steps[stepID] = map[string]interface{}{
		"output": output,
	}
	result["steps"] = steps

	result[stepID] = output

	return result
}

func (c *WorkflowConsumer) scheduleNextStep(ctx context.Context, runID, nextStepID string) error {
	nextStep, err := c.stepProvider.GetStep(ctx, runID, nextStepID)
	if err != nil {
		return fmt.Errorf("failed to get next step %s: %w", nextStepID, err)
	}

	log.Printf("Workflow consumer: scheduling next step %s (type: %s) for run %s", nextStepID, nextStep.Type, runID)
	return c.engine.PublishStepSchedule(ctx, runID, nextStepID, nextStep)
}

func extractRunIDFromSubject(subject string) string {
	var runID string
	_, _ = fmt.Sscanf(subject, "workflow.run.%s", &runID)

	parts := splitSubject(subject)
	if len(parts) >= 3 && parts[0] == "workflow" && parts[1] == "run" {
		return parts[2]
	}
	return ""
}

func splitSubject(subject string) []string {
	var parts []string
	current := ""
	for _, ch := range subject {
		if ch == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func mergeContexts(base, overlay map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		result[k] = v
	}
	return result
}
