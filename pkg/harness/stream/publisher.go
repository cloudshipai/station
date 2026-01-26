package stream

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"
)

type EventType string

const (
	EventToken        EventType = "token"
	EventThinking     EventType = "thinking"
	EventToolStart    EventType = "tool_start"
	EventToolResult   EventType = "tool_result"
	EventStepComplete EventType = "step_complete"
	EventRunStart     EventType = "run_start"
	EventRunComplete  EventType = "run_complete"
	EventError        EventType = "error"
)

type Event struct {
	StationRunID  string    `json:"station_run_id"`
	RunUUID       string    `json:"run_uuid"`
	WorkflowRunID string    `json:"workflow_run_id,omitempty"`
	SessionID     string    `json:"session_id,omitempty"`
	AgentID       string    `json:"agent_id"`
	AgentName     string    `json:"agent_name,omitempty"`
	StationID     string    `json:"station_id,omitempty"`
	Seq           int64     `json:"seq"`
	Timestamp     time.Time `json:"timestamp"`
	Type          EventType `json:"type"`
	Data          any       `json:"data"`
}

type TokenData struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

type ThinkingData struct {
	Content string `json:"content"`
}

type ToolStartData struct {
	ToolName string `json:"tool_name"`
	ToolID   string `json:"tool_id"`
	Input    any    `json:"input"`
}

type ToolResultData struct {
	ToolName   string `json:"tool_name"`
	ToolID     string `json:"tool_id"`
	Output     any    `json:"output"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

type StepCompleteData struct {
	StepNumber   int    `json:"step_number"`
	TotalTokens  int    `json:"total_tokens"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	FinishReason string `json:"finish_reason"`
}

type RunStartData struct {
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	Task      string `json:"task"`
	MaxSteps  int    `json:"max_steps"`
}

type RunCompleteData struct {
	Success      bool   `json:"success"`
	TotalSteps   int    `json:"total_steps"`
	TotalTokens  int    `json:"total_tokens"`
	DurationMs   int64  `json:"duration_ms"`
	FinishReason string `json:"finish_reason"`
	Error        string `json:"error,omitempty"`
}

type Publisher interface {
	Publish(ctx context.Context, event *Event) error
	Close() error
}

type StreamIdentifiers struct {
	StationRunID  string
	RunUUID       string
	WorkflowRunID string
	SessionID     string
	AgentID       string
	AgentName     string
	StationID     string
}

type StreamContext struct {
	ids       StreamIdentifiers
	seq       int64
	publisher Publisher
}

func NewStreamContext(ids StreamIdentifiers, publisher Publisher) *StreamContext {
	return &StreamContext{
		ids:       ids,
		seq:       0,
		publisher: publisher,
	}
}

func (s *StreamContext) nextSeq() int64 {
	return atomic.AddInt64(&s.seq, 1)
}

func (s *StreamContext) emit(ctx context.Context, eventType EventType, data any) error {
	if s.publisher == nil {
		return nil
	}

	event := &Event{
		StationRunID:  s.ids.StationRunID,
		RunUUID:       s.ids.RunUUID,
		WorkflowRunID: s.ids.WorkflowRunID,
		SessionID:     s.ids.SessionID,
		AgentID:       s.ids.AgentID,
		AgentName:     s.ids.AgentName,
		StationID:     s.ids.StationID,
		Seq:           s.nextSeq(),
		Timestamp:     time.Now(),
		Type:          eventType,
		Data:          data,
	}

	return s.publisher.Publish(ctx, event)
}

func (s *StreamContext) Identifiers() StreamIdentifiers {
	return s.ids
}

func (s *StreamContext) EmitRunStart(ctx context.Context, agentID, agentName, task string, maxSteps int) error {
	return s.emit(ctx, EventRunStart, RunStartData{
		AgentID:   agentID,
		AgentName: agentName,
		Task:      task,
		MaxSteps:  maxSteps,
	})
}

func (s *StreamContext) EmitToken(ctx context.Context, content string, done bool) error {
	return s.emit(ctx, EventToken, TokenData{
		Content: content,
		Done:    done,
	})
}

func (s *StreamContext) EmitThinking(ctx context.Context, content string) error {
	return s.emit(ctx, EventThinking, ThinkingData{
		Content: content,
	})
}

func (s *StreamContext) EmitToolStart(ctx context.Context, toolName, toolID string, input any) error {
	return s.emit(ctx, EventToolStart, ToolStartData{
		ToolName: toolName,
		ToolID:   toolID,
		Input:    input,
	})
}

func (s *StreamContext) EmitToolResult(ctx context.Context, toolName, toolID string, output any, durationMs int64, err error) error {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	return s.emit(ctx, EventToolResult, ToolResultData{
		ToolName:   toolName,
		ToolID:     toolID,
		Output:     output,
		DurationMs: durationMs,
		Error:      errStr,
	})
}

func (s *StreamContext) EmitStepComplete(ctx context.Context, stepNum, totalTokens, inputTokens, outputTokens int, finishReason string) error {
	return s.emit(ctx, EventStepComplete, StepCompleteData{
		StepNumber:   stepNum,
		TotalTokens:  totalTokens,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		FinishReason: finishReason,
	})
}

func (s *StreamContext) EmitRunComplete(ctx context.Context, success bool, steps, tokens int, durationMs int64, finishReason string, err error) error {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	return s.emit(ctx, EventRunComplete, RunCompleteData{
		Success:      success,
		TotalSteps:   steps,
		TotalTokens:  tokens,
		DurationMs:   durationMs,
		FinishReason: finishReason,
		Error:        errStr,
	})
}

func (s *StreamContext) EmitError(ctx context.Context, err error) error {
	return s.emit(ctx, EventError, map[string]string{"error": err.Error()})
}

type MultiPublisher struct {
	publishers []Publisher
}

func NewMultiPublisher(publishers ...Publisher) *MultiPublisher {
	return &MultiPublisher{publishers: publishers}
}

func (m *MultiPublisher) Publish(ctx context.Context, event *Event) error {
	for _, p := range m.publishers {
		if err := p.Publish(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiPublisher) Close() error {
	for _, p := range m.publishers {
		p.Close()
	}
	return nil
}

type NoOpPublisher struct{}

func (n *NoOpPublisher) Publish(ctx context.Context, event *Event) error { return nil }
func (n *NoOpPublisher) Close() error                                    { return nil }

type ChannelPublisher struct {
	ch chan *Event
}

func NewChannelPublisher(bufferSize int) *ChannelPublisher {
	return &ChannelPublisher{
		ch: make(chan *Event, bufferSize),
	}
}

func (c *ChannelPublisher) Publish(ctx context.Context, event *Event) error {
	select {
	case c.ch <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (c *ChannelPublisher) Events() <-chan *Event {
	return c.ch
}

func (c *ChannelPublisher) Close() error {
	close(c.ch)
	return nil
}

func (e *Event) JSON() ([]byte, error) {
	return json.Marshal(e)
}
