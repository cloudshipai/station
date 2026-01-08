package events

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type PublisherConfig struct {
	Enabled     bool
	StationID   string
	StationName string
	Async       bool
	BatchSize   int
	FlushPeriod time.Duration
}

func DefaultPublisherConfig() PublisherConfig {
	return PublisherConfig{
		Enabled:     true,
		Async:       true,
		BatchSize:   100,
		FlushPeriod: 100 * time.Millisecond,
	}
}

type Publisher struct {
	js     nats.JetStreamContext
	config PublisherConfig

	mu      sync.RWMutex
	batch   []*pendingEvent
	stopCh  chan struct{}
	started bool
}

type pendingEvent struct {
	subject string
	event   *CloudEvent
}

func NewPublisher(js nats.JetStreamContext, config PublisherConfig) (*Publisher, error) {
	if js == nil {
		return nil, fmt.Errorf("JetStream context is required")
	}

	return &Publisher{
		js:     js,
		config: config,
		batch:  make([]*pendingEvent, 0, config.BatchSize),
		stopCh: make(chan struct{}),
	}, nil
}

func (p *Publisher) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return nil
	}

	if !p.config.Enabled {
		return nil
	}

	if p.config.Async {
		go p.flushLoop()
	}

	p.started = true
	return nil
}

func (p *Publisher) Stop() error {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return nil
	}
	p.started = false
	p.mu.Unlock()

	close(p.stopCh)

	return p.flush()
}

func (p *Publisher) flushLoop() {
	ticker := time.NewTicker(p.config.FlushPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			if err := p.flush(); err != nil {
				fmt.Printf("[events] Flush error: %v\n", err)
			}
		}
	}
}

func (p *Publisher) flush() error {
	p.mu.Lock()
	if len(p.batch) == 0 {
		p.mu.Unlock()
		return nil
	}
	toFlush := p.batch
	p.batch = make([]*pendingEvent, 0, p.config.BatchSize)
	p.mu.Unlock()

	var lastErr error
	for _, pe := range toFlush {
		if err := p.publishDirect(pe.subject, pe.event); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (p *Publisher) Publish(ctx context.Context, eventType string, data any) error {
	if !p.config.Enabled {
		return nil
	}

	event := NewCloudEvent(eventType, EventSourcePrefix)
	event.WithStation(p.config.StationID, p.config.StationName)

	if err := event.WithData(data); err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	subject := p.subjectForType(eventType)

	if p.config.Async {
		return p.publishAsync(subject, event)
	}
	return p.publishDirect(subject, event)
}

func (p *Publisher) PublishEvent(ctx context.Context, event *CloudEvent) error {
	if !p.config.Enabled {
		return nil
	}

	if event.StationID == "" {
		event.WithStation(p.config.StationID, p.config.StationName)
	}

	subject := p.subjectForType(event.Type)

	if p.config.Async {
		return p.publishAsync(subject, event)
	}
	return p.publishDirect(subject, event)
}

func (p *Publisher) publishAsync(subject string, event *CloudEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.batch = append(p.batch, &pendingEvent{subject: subject, event: event})

	if len(p.batch) >= p.config.BatchSize {
		go func() {
			if err := p.flush(); err != nil {
				fmt.Printf("[events] Batch flush error: %v\n", err)
			}
		}()
	}

	return nil
}

func (p *Publisher) publishDirect(subject string, event *CloudEvent) error {
	data, err := event.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = p.js.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish event to %s: %w", subject, err)
	}

	return nil
}

func (p *Publisher) subjectForType(eventType string) string {
	return fmt.Sprintf("lattice.events.%s", eventType)
}

func (p *Publisher) PublishStationJoined(ctx context.Context, data *StationJoinedData) error {
	return p.Publish(ctx, EventTypeStationJoined, data)
}

func (p *Publisher) PublishStationLeft(ctx context.Context, data *StationLeftData) error {
	return p.Publish(ctx, EventTypeStationLeft, data)
}

func (p *Publisher) PublishAgentRegistered(ctx context.Context, data *AgentRegisteredData) error {
	return p.Publish(ctx, EventTypeAgentRegistered, data)
}

func (p *Publisher) PublishAgentDeregistered(ctx context.Context, data *AgentDeregisteredData) error {
	return p.Publish(ctx, EventTypeAgentDeregistered, data)
}

func (p *Publisher) PublishAgentInvoked(ctx context.Context, data *AgentInvokedData) error {
	return p.Publish(ctx, EventTypeAgentInvoked, data)
}

func (p *Publisher) PublishWorkAssigned(ctx context.Context, data *WorkAssignedData) error {
	return p.Publish(ctx, EventTypeWorkAssigned, data)
}

func (p *Publisher) PublishWorkAccepted(ctx context.Context, data *WorkAcceptedData) error {
	return p.Publish(ctx, EventTypeWorkAccepted, data)
}

func (p *Publisher) PublishWorkProgress(ctx context.Context, data *WorkProgressData) error {
	return p.Publish(ctx, EventTypeWorkProgress, data)
}

func (p *Publisher) PublishWorkCompleted(ctx context.Context, data *WorkCompletedData) error {
	return p.Publish(ctx, EventTypeWorkCompleted, data)
}

func (p *Publisher) PublishWorkFailed(ctx context.Context, data *WorkFailedData) error {
	return p.Publish(ctx, EventTypeWorkFailed, data)
}

func (p *Publisher) PublishWorkEscalated(ctx context.Context, data *WorkEscalatedData) error {
	return p.Publish(ctx, EventTypeWorkEscalated, data)
}

func (p *Publisher) PublishWorkCancelled(ctx context.Context, data *WorkCancelledData) error {
	return p.Publish(ctx, EventTypeWorkCancelled, data)
}
