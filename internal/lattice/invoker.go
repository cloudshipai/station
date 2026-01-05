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

type Invoker struct {
	client    *Client
	stationID string
	executor  AgentExecutor
	sub       *nats.Subscription
}

func NewInvoker(client *Client, stationID string, executor AgentExecutor) *Invoker {
	return &Invoker{
		client:    client,
		stationID: stationID,
		executor:  executor,
	}
}

func (i *Invoker) Start(ctx context.Context) error {
	if i.client == nil || !i.client.IsConnected() {
		return fmt.Errorf("client not connected")
	}

	subject := fmt.Sprintf("lattice.station.%s.agent.invoke", i.stationID)
	sub, err := i.client.conn.Subscribe(subject, i.handleInvokeRequest)
	if err != nil {
		return fmt.Errorf("failed to subscribe to invoke subject: %w", err)
	}

	i.sub = sub
	fmt.Printf("[invoker] Listening for agent invocations on %s\n", subject)

	return nil
}

func (i *Invoker) Stop() {
	if i.sub != nil {
		_ = i.sub.Unsubscribe()
		i.sub = nil
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

	var result string
	var toolCalls int
	var err error

	if req.AgentID != "" {
		result, toolCalls, err = i.executor.ExecuteAgentByID(ctx, req.AgentID, req.Task)
	} else if req.AgentName != "" {
		result, toolCalls, err = i.executor.ExecuteAgentByName(ctx, req.AgentName, req.Task)
	} else {
		i.respondError(msg, "agent_id or agent_name required", nil, start)
		return
	}

	if err != nil {
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

func (i *Invoker) InvokeRemoteAgent(ctx context.Context, targetStationID string, req InvokeAgentRequest) (*InvokeAgentResponse, error) {
	if i.client == nil || !i.client.IsConnected() {
		return nil, fmt.Errorf("client not connected")
	}

	subject := fmt.Sprintf("lattice.station.%s.agent.invoke", targetStationID)
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	timeout := 5 * time.Minute
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	respMsg, err := i.client.Request(subject, data, timeout)
	if err != nil {
		return nil, fmt.Errorf("invoke request failed: %w", err)
	}

	var response InvokeAgentResponse
	if err := json.Unmarshal(respMsg.Data, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

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
	if i.client == nil || !i.client.IsConnected() {
		return nil, fmt.Errorf("client not connected")
	}

	subject := fmt.Sprintf("lattice.station.%s.workflow.run", targetStationID)
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	timeout := 10 * time.Minute
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	respMsg, err := i.client.Request(subject, data, timeout)
	if err != nil {
		return nil, fmt.Errorf("workflow invoke request failed: %w", err)
	}

	var response RunWorkflowResponse
	if err := json.Unmarshal(respMsg.Data, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}
