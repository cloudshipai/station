package work

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type NATSClient interface {
	Publish(subject string, data []byte) error
	Subscribe(subject string, handler nats.MsgHandler) (*nats.Subscription, error)
	IsConnected() bool
	StationID() string
}

type PendingWork struct {
	Assignment   *WorkAssignment
	ResultChan   chan *WorkResponse
	ProgressChan chan *WorkResponse
	Done         chan struct{}
	completed    atomic.Bool
}

type Dispatcher struct {
	client      NATSClient
	stationID   string
	pendingWork sync.Map
	childIndex  atomic.Int64
	store       *WorkStore

	mu           sync.RWMutex
	subscription *nats.Subscription
}

type DispatcherOption func(*Dispatcher)

func WithWorkStore(store *WorkStore) DispatcherOption {
	return func(d *Dispatcher) {
		d.store = store
	}
}

func NewDispatcher(client NATSClient, stationID string, opts ...DispatcherOption) *Dispatcher {
	d := &Dispatcher{
		client:    client,
		stationID: stationID,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func (d *Dispatcher) Start(ctx context.Context) error {
	responseSubject := "lattice.work.*.response"
	sub, err := d.client.Subscribe(responseSubject, d.handleResponse)
	if err != nil {
		return fmt.Errorf("failed to subscribe to work responses: %w", err)
	}

	d.mu.Lock()
	d.subscription = sub
	d.mu.Unlock()

	return nil
}

func (d *Dispatcher) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.subscription != nil {
		d.subscription.Unsubscribe()
		d.subscription = nil
	}

	d.pendingWork.Range(func(key, value any) bool {
		pending := value.(*PendingWork)
		close(pending.Done)
		return true
	})
}

func (d *Dispatcher) AssignWork(ctx context.Context, assignment *WorkAssignment) (string, error) {
	if assignment.WorkID == "" {
		assignment.WorkID = uuid.NewString()
	}
	if assignment.OrchestratorRunID == "" {
		assignment.OrchestratorRunID = uuid.NewString()
	}
	assignment.AssignedAt = time.Now()
	assignment.ReplySubject = SubjectWorkResponse(assignment.WorkID)

	targetStation := assignment.TargetStation
	if targetStation == "" {
		targetStation = assignment.TargetStation
		if targetStation == "" {
			targetStation = d.stationID
		}
	}

	if d.store != nil {
		record := &WorkRecord{
			WorkID:            assignment.WorkID,
			OrchestratorRunID: assignment.OrchestratorRunID,
			ParentWorkID:      assignment.ParentWorkID,
			SourceStation:     d.stationID,
			TargetStation:     targetStation,
			AgentID:           assignment.AgentID,
			AgentName:         assignment.AgentName,
			Task:              assignment.Task,
			Context:           assignment.Context,
			TraceID:           assignment.TraceID,
			SpanID:            assignment.SpanID,
		}
		if err := d.store.Assign(ctx, record); err != nil {
			return "", fmt.Errorf("failed to persist work assignment: %w", err)
		}
	}

	pending := &PendingWork{
		Assignment:   assignment,
		ResultChan:   make(chan *WorkResponse, 1),
		ProgressChan: make(chan *WorkResponse, 10),
		Done:         make(chan struct{}),
	}
	d.pendingWork.Store(assignment.WorkID, pending)

	subject := SubjectWorkAssign(targetStation)
	data, err := json.Marshal(assignment)
	if err != nil {
		d.pendingWork.Delete(assignment.WorkID)
		return "", fmt.Errorf("failed to marshal assignment: %w", err)
	}

	if err := d.client.Publish(subject, data); err != nil {
		d.pendingWork.Delete(assignment.WorkID)
		return "", fmt.Errorf("failed to publish work assignment: %w", err)
	}

	return assignment.WorkID, nil
}

func (d *Dispatcher) AwaitWork(ctx context.Context, workID string) (*WorkResponse, error) {
	val, ok := d.pendingWork.Load(workID)
	if !ok {
		return nil, fmt.Errorf("work %s not found", workID)
	}
	pending := val.(*PendingWork)

	timeout := pending.Assignment.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	select {
	case result := <-pending.ResultChan:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, fmt.Errorf("work %s timed out after %v", workID, timeout)
	case <-pending.Done:
		return nil, fmt.Errorf("work %s cancelled", workID)
	}
}

func (d *Dispatcher) CheckWork(workID string) (*WorkStatus, error) {
	val, ok := d.pendingWork.Load(workID)
	if !ok {
		return nil, fmt.Errorf("work %s not found", workID)
	}
	pending := val.(*PendingWork)

	select {
	case result := <-pending.ResultChan:
		pending.ResultChan <- result
		return &WorkStatus{
			WorkID:   workID,
			Status:   result.Type,
			Response: result,
		}, nil
	default:
		return &WorkStatus{
			WorkID: workID,
			Status: "PENDING",
		}, nil
	}
}

func (d *Dispatcher) StreamProgress(workID string) (<-chan *WorkResponse, error) {
	val, ok := d.pendingWork.Load(workID)
	if !ok {
		return nil, fmt.Errorf("work %s not found", workID)
	}
	return val.(*PendingWork).ProgressChan, nil
}

func (d *Dispatcher) CancelWork(workID string) error {
	val, ok := d.pendingWork.Load(workID)
	if !ok {
		return fmt.Errorf("work %s not found", workID)
	}
	pending := val.(*PendingWork)

	if pending.completed.Load() {
		return nil
	}

	close(pending.Done)
	d.pendingWork.Delete(workID)
	return nil
}

func (d *Dispatcher) handleResponse(msg *nats.Msg) {
	var response WorkResponse
	if err := json.Unmarshal(msg.Data, &response); err != nil {
		return
	}

	if d.store != nil {
		d.updateStoreFromResponse(&response)
	}

	val, ok := d.pendingWork.Load(response.WorkID)
	if !ok {
		return
	}
	pending := val.(*PendingWork)

	switch response.Type {
	case MsgWorkAccepted:
		select {
		case pending.ProgressChan <- &response:
		default:
		}

	case MsgWorkProgress:
		select {
		case pending.ProgressChan <- &response:
		default:
		}

	case MsgWorkComplete, MsgWorkFailed, MsgWorkEscalate:
		if pending.completed.CompareAndSwap(false, true) {
			select {
			case pending.ResultChan <- &response:
			default:
			}
			close(pending.ProgressChan)
		}
	}
}

func (d *Dispatcher) updateStoreFromResponse(response *WorkResponse) {
	ctx := context.Background()

	var status string
	var result *WorkResult

	switch response.Type {
	case MsgWorkAccepted:
		status = StatusAccepted
	case MsgWorkComplete:
		status = StatusComplete
		result = &WorkResult{
			Result:     response.Result,
			DurationMs: response.DurationMs,
			ToolCalls:  response.ToolCalls,
		}
	case MsgWorkFailed:
		status = StatusFailed
		result = &WorkResult{
			Error:      response.Error,
			DurationMs: response.DurationMs,
			ToolCalls:  response.ToolCalls,
		}
	case MsgWorkEscalate:
		status = StatusEscalated
		result = &WorkResult{
			Error:      response.EscalationReason,
			DurationMs: response.DurationMs,
			ToolCalls:  response.ToolCalls,
		}
	default:
		return
	}

	_ = d.store.UpdateStatus(ctx, response.WorkID, status, result)
}

func (d *Dispatcher) GenerateChildWorkID(parentWorkID string) string {
	idx := d.childIndex.Add(1)
	return fmt.Sprintf("%s-%d", parentWorkID, idx)
}
