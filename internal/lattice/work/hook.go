package work

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type AgentExecutor interface {
	ExecuteAgentByID(ctx context.Context, agentID string, task string) (string, int, error)
	ExecuteAgentByName(ctx context.Context, agentName string, task string) (string, int, error)
}

type AgentExecutorWithContext interface {
	AgentExecutor
	ExecuteAgentByIDWithContext(ctx context.Context, agentID string, task string, orchCtx *OrchestratorContext) (string, int64, int, error)
	ExecuteAgentByNameWithContext(ctx context.Context, agentName string, task string, orchCtx *OrchestratorContext) (string, int64, int, error)
}

type Hook struct {
	client    NATSClient
	executor  AgentExecutor
	stationID string

	mu           sync.RWMutex
	subscription *nats.Subscription
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewHook(client NATSClient, executor AgentExecutor, stationID string) *Hook {
	return &Hook{
		client:    client,
		executor:  executor,
		stationID: stationID,
	}
}

func (h *Hook) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.ctx, h.cancel = context.WithCancel(ctx)

	subject := SubjectWorkAssign(h.stationID)
	sub, err := h.client.Subscribe(subject, h.handleWorkAssignment)
	if err != nil {
		return fmt.Errorf("failed to subscribe to work assignments: %w", err)
	}

	h.subscription = sub
	return nil
}

func (h *Hook) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cancel != nil {
		h.cancel()
	}

	if h.subscription != nil {
		h.subscription.Unsubscribe()
		h.subscription = nil
	}
}

func (h *Hook) handleWorkAssignment(msg *nats.Msg) {
	var assignment WorkAssignment
	if err := json.Unmarshal(msg.Data, &assignment); err != nil {
		h.sendErrorResponse(msg.Reply, "", "invalid assignment: "+err.Error())
		return
	}

	go h.executeWork(&assignment, msg.Reply)
}

func (h *Hook) executeWork(assignment *WorkAssignment, replySubject string) {
	h.sendResponse(replySubject, &WorkResponse{
		WorkID:            assignment.WorkID,
		OrchestratorRunID: assignment.OrchestratorRunID,
		Type:              MsgWorkAccepted,
		StationID:         h.stationID,
		Timestamp:         time.Now(),
	})

	startTime := time.Now()
	var result string
	var toolCalls int
	var localRunID int64
	var execErr error

	orchCtx := &OrchestratorContext{
		RunID:              assignment.OrchestratorRunID,
		ParentRunID:        assignment.ParentWorkID,
		OriginatingStation: h.stationID,
		TraceID:            assignment.TraceID,
		WorkID:             assignment.WorkID,
	}

	if executorWithCtx, ok := h.executor.(AgentExecutorWithContext); ok {
		if assignment.AgentID != "" {
			result, localRunID, toolCalls, execErr = executorWithCtx.ExecuteAgentByIDWithContext(h.ctx, assignment.AgentID, assignment.Task, orchCtx)
		} else if assignment.AgentName != "" {
			result, localRunID, toolCalls, execErr = executorWithCtx.ExecuteAgentByNameWithContext(h.ctx, assignment.AgentName, assignment.Task, orchCtx)
		} else {
			h.sendResponse(replySubject, &WorkResponse{
				WorkID:            assignment.WorkID,
				OrchestratorRunID: assignment.OrchestratorRunID,
				Type:              MsgWorkFailed,
				Error:             "no agent_id or agent_name specified",
				StationID:         h.stationID,
				Timestamp:         time.Now(),
			})
			return
		}
	} else {
		if assignment.AgentID != "" {
			result, toolCalls, execErr = h.executor.ExecuteAgentByID(h.ctx, assignment.AgentID, assignment.Task)
		} else if assignment.AgentName != "" {
			result, toolCalls, execErr = h.executor.ExecuteAgentByName(h.ctx, assignment.AgentName, assignment.Task)
		} else {
			h.sendResponse(replySubject, &WorkResponse{
				WorkID:            assignment.WorkID,
				OrchestratorRunID: assignment.OrchestratorRunID,
				Type:              MsgWorkFailed,
				Error:             "no agent_id or agent_name specified",
				StationID:         h.stationID,
				Timestamp:         time.Now(),
			})
			return
		}
	}

	duration := time.Since(startTime)

	response := &WorkResponse{
		WorkID:            assignment.WorkID,
		OrchestratorRunID: assignment.OrchestratorRunID,
		StationID:         h.stationID,
		LocalRunID:        localRunID,
		DurationMs:        float64(duration.Milliseconds()),
		ToolCalls:         toolCalls,
		Timestamp:         time.Now(),
	}

	if execErr != nil {
		response.Type = MsgWorkFailed
		response.Error = execErr.Error()
	} else {
		response.Type = MsgWorkComplete
		response.Result = result
	}

	h.sendResponse(replySubject, response)
}

func (h *Hook) sendResponse(subject string, response *WorkResponse) {
	if subject == "" {
		subject = SubjectWorkResponse(response.WorkID)
	}

	data, err := json.Marshal(response)
	if err != nil {
		return
	}

	h.client.Publish(subject, data)
}

func (h *Hook) sendErrorResponse(subject, workID, errMsg string) {
	h.sendResponse(subject, &WorkResponse{
		WorkID:    workID,
		Type:      MsgWorkFailed,
		Error:     errMsg,
		StationID: h.stationID,
		Timestamp: time.Now(),
	})
}

func (h *Hook) SendProgress(workID string, progressPct int, progressMsg string) error {
	response := &WorkResponse{
		WorkID:      workID,
		Type:        MsgWorkProgress,
		ProgressPct: progressPct,
		ProgressMsg: progressMsg,
		StationID:   h.stationID,
		Timestamp:   time.Now(),
	}

	data, err := json.Marshal(response)
	if err != nil {
		return err
	}

	return h.client.Publish(SubjectWorkResponse(workID), data)
}

func (h *Hook) Escalate(workID, reason string, ctx map[string]string) error {
	response := &WorkResponse{
		WorkID:            workID,
		Type:              MsgWorkEscalate,
		EscalationReason:  reason,
		EscalationContext: ctx,
		StationID:         h.stationID,
		Timestamp:         time.Now(),
	}

	data, err := json.Marshal(response)
	if err != nil {
		return err
	}

	return h.client.Publish(SubjectWorkResponse(workID), data)
}
