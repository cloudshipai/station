package lattice

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type InvokeAgentRequest struct {
	AgentID   string            `json:"agent_id,omitempty"`
	AgentName string            `json:"agent_name,omitempty"`
	Task      string            `json:"task"`
	Context   map[string]string `json:"context,omitempty"`
}

type InvokeAgentResponse struct {
	Status     string  `json:"status"`
	Result     string  `json:"result"`
	Error      string  `json:"error,omitempty"`
	DurationMs float64 `json:"duration_ms"`
	ToolCalls  int     `json:"tool_calls"`
	StationID  string  `json:"station_id"`
}

type AgentExecutor interface {
	ExecuteAgentByID(ctx context.Context, agentID string, task string) (string, int, error)
	ExecuteAgentByName(ctx context.Context, agentName string, task string) (string, int, error)
}

type WorkflowExecutor interface {
	ExecuteWorkflow(ctx context.Context, workflowID string, input map[string]string) (runID string, status string, err error)
}

type Invoker struct {
	client           *Client
	stationID        string
	executor         AgentExecutor
	workflowExecutor WorkflowExecutor
	agentSub         *nats.Subscription
	workflowSub      *nats.Subscription
	telemetry        *Telemetry
}

func NewInvoker(client *Client, stationID string, executor AgentExecutor) *Invoker {
	return &Invoker{
		client:    client,
		stationID: stationID,
		executor:  executor,
		telemetry: NewTelemetry(),
	}
}

func (i *Invoker) SetWorkflowExecutor(executor WorkflowExecutor) {
	i.workflowExecutor = executor
}

func (i *Invoker) Start(ctx context.Context) error {
	if i.client == nil || !i.client.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	agentSubject := fmt.Sprintf("lattice.station.%s.agent.invoke", i.stationID)
	agentSub, err := i.client.conn.Subscribe(agentSubject, i.handleInvokeRequest)
	if err != nil {
		return fmt.Errorf("failed to subscribe to agent invoke subject: %w", err)
	}
	i.agentSub = agentSub
	fmt.Printf("[invoker] Listening for agent invocations on %s\n", agentSubject)

	workflowSubject := fmt.Sprintf("lattice.station.%s.workflow.run", i.stationID)
	workflowSub, err := i.client.conn.Subscribe(workflowSubject, i.handleWorkflowRequest)
	if err != nil {
		return fmt.Errorf("failed to subscribe to workflow run subject: %w", err)
	}
	i.workflowSub = workflowSub
	fmt.Printf("[invoker] Listening for workflow runs on %s\n", workflowSubject)

	return nil
}

func (i *Invoker) Stop() {
	if i.agentSub != nil {
		_ = i.agentSub.Unsubscribe()
		i.agentSub = nil
	}
	if i.workflowSub != nil {
		_ = i.workflowSub.Unsubscribe()
		i.workflowSub = nil
	}
}

func (i *Invoker) handleInvokeRequest(msg *nats.Msg) {
	start := time.Now()

	var req InvokeAgentRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		i.respondError(msg, "invalid request", err, start)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	_, span := i.telemetry.StartHandleAgentRequestSpan(ctx, i.stationID, req.AgentID, req.AgentName)

	var result string
	var toolCalls int
	var err error

	if req.AgentID != "" {
		result, toolCalls, err = i.executor.ExecuteAgentByID(ctx, req.AgentID, req.Task)
	} else if req.AgentName != "" {
		result, toolCalls, err = i.executor.ExecuteAgentByName(ctx, req.AgentName, req.Task)
	} else {
		validationErr := fmt.Errorf("agent_id or agent_name required")
		i.telemetry.EndSpan(span, validationErr)
		i.respondError(msg, "agent_id or agent_name required", nil, start)
		return
	}

	if err != nil {
		i.telemetry.EndSpan(span, err)
		i.respondError(msg, "execution failed", err, start)
		return
	}

	response := InvokeAgentResponse{
		Status:     "success",
		Result:     result,
		DurationMs: float64(time.Since(start).Milliseconds()),
		ToolCalls:  toolCalls,
		StationID:  i.stationID,
	}

	i.telemetry.EndSpan(span, nil)

	data, _ := json.Marshal(response)
	if err := msg.Respond(data); err != nil {
		fmt.Printf("[invoker] Failed to send response: %v\n", err)
	}
}

func (i *Invoker) respondError(msg *nats.Msg, message string, err error, start time.Time) {
	errMsg := message
	if err != nil {
		errMsg = fmt.Sprintf("%s: %v", message, err)
	}

	response := InvokeAgentResponse{
		Status:     "error",
		Error:      errMsg,
		DurationMs: float64(time.Since(start).Milliseconds()),
		StationID:  i.stationID,
	}

	data, _ := json.Marshal(response)
	_ = msg.Respond(data)
}

func (i *Invoker) handleWorkflowRequest(msg *nats.Msg) {
	start := time.Now()

	var req RunWorkflowRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		i.respondWorkflowError(msg, "invalid request", err, start)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	_, span := i.telemetry.StartHandleWorkflowRequestSpan(ctx, i.stationID, req.WorkflowID)

	if i.workflowExecutor == nil {
		executorErr := fmt.Errorf("workflow executor not configured")
		i.telemetry.EndSpan(span, executorErr)
		i.respondWorkflowError(msg, "workflow executor not configured", nil, start)
		return
	}

	runID, status, err := i.workflowExecutor.ExecuteWorkflow(ctx, req.WorkflowID, req.Input)
	if err != nil {
		i.telemetry.EndSpan(span, err)
		i.respondWorkflowError(msg, "workflow execution failed", err, start)
		return
	}

	response := RunWorkflowResponse{
		Status:    status,
		RunID:     runID,
		StationID: i.stationID,
	}

	i.telemetry.EndSpan(span, nil)

	data, _ := json.Marshal(response)
	if err := msg.Respond(data); err != nil {
		fmt.Printf("[invoker] Failed to send workflow response: %v\n", err)
	}
}

func (i *Invoker) respondWorkflowError(msg *nats.Msg, message string, err error, start time.Time) {
	errMsg := message
	if err != nil {
		errMsg = fmt.Sprintf("%s: %v", message, err)
	}

	response := RunWorkflowResponse{
		Status:    "error",
		Error:     errMsg,
		StationID: i.stationID,
	}

	data, _ := json.Marshal(response)
	_ = msg.Respond(data)
}

func (i *Invoker) InvokeRemoteAgent(ctx context.Context, targetStationID string, req InvokeAgentRequest) (*InvokeAgentResponse, error) {
	ctx, span := i.telemetry.StartAgentInvocationSpan(ctx, targetStationID, req.AgentID, req.AgentName, req.Task)
	timer := StartTimer()

	if i.client == nil || !i.client.IsConnected() {
		err := fmt.Errorf("client not connected")
		i.telemetry.EndAgentInvocationSpan(span, "error", "", 0, timer.ElapsedMs(), err)
		return nil, err
	}

	subject := fmt.Sprintf("lattice.station.%s.agent.invoke", targetStationID)
	data, err := json.Marshal(req)
	if err != nil {
		err = fmt.Errorf("failed to marshal request: %w", err)
		i.telemetry.EndAgentInvocationSpan(span, "error", "", 0, timer.ElapsedMs(), err)
		return nil, err
	}

	timeout := 5 * time.Minute
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	_, natsSpan := i.telemetry.StartNATSRequestSpan(ctx, subject)
	natsTimer := StartTimer()

	respMsg, err := i.client.Request(subject, data, timeout)

	i.telemetry.EndNATSRequestSpan(natsSpan, natsTimer.ElapsedMs(), err)

	if err != nil {
		err = fmt.Errorf("invoke request failed: %w", err)
		i.telemetry.EndAgentInvocationSpan(span, "error", "", 0, timer.ElapsedMs(), err)
		return nil, err
	}

	var response InvokeAgentResponse
	if err := json.Unmarshal(respMsg.Data, &response); err != nil {
		err = fmt.Errorf("failed to unmarshal response: %w", err)
		i.telemetry.EndAgentInvocationSpan(span, "error", "", 0, timer.ElapsedMs(), err)
		return nil, err
	}

	i.telemetry.EndAgentInvocationSpan(span, response.Status, response.Result, response.ToolCalls, timer.ElapsedMs(), nil)

	return &response, nil
}

type RunWorkflowRequest struct {
	WorkflowID string            `json:"workflow_id"`
	Input      map[string]string `json:"input,omitempty"`
}

type RunWorkflowResponse struct {
	Status    string `json:"status"`
	RunID     string `json:"run_id,omitempty"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	StationID string `json:"station_id"`
}

func (i *Invoker) InvokeRemoteWorkflow(ctx context.Context, targetStationID string, req RunWorkflowRequest) (*RunWorkflowResponse, error) {
	ctx, span := i.telemetry.StartWorkflowInvocationSpan(ctx, targetStationID, req.WorkflowID)

	if i.client == nil || !i.client.IsConnected() {
		err := fmt.Errorf("client not connected")
		i.telemetry.EndWorkflowInvocationSpan(span, "error", "", err)
		return nil, err
	}

	subject := fmt.Sprintf("lattice.station.%s.workflow.run", targetStationID)
	data, err := json.Marshal(req)
	if err != nil {
		err = fmt.Errorf("failed to marshal request: %w", err)
		i.telemetry.EndWorkflowInvocationSpan(span, "error", "", err)
		return nil, err
	}

	timeout := 10 * time.Minute
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	_, natsSpan := i.telemetry.StartNATSRequestSpan(ctx, subject)
	natsTimer := StartTimer()

	respMsg, err := i.client.Request(subject, data, timeout)

	i.telemetry.EndNATSRequestSpan(natsSpan, natsTimer.ElapsedMs(), err)

	if err != nil {
		err = fmt.Errorf("workflow invoke request failed: %w", err)
		i.telemetry.EndWorkflowInvocationSpan(span, "error", "", err)
		return nil, err
	}

	var response RunWorkflowResponse
	if err := json.Unmarshal(respMsg.Data, &response); err != nil {
		err = fmt.Errorf("failed to unmarshal response: %w", err)
		i.telemetry.EndWorkflowInvocationSpan(span, "error", "", err)
		return nil, err
	}

	i.telemetry.EndWorkflowInvocationSpan(span, response.Status, response.RunID, nil)

	return &response, nil
}
