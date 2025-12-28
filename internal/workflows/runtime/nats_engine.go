package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	natsserver_test "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"

	"station/internal/workflows"
)

type Engine interface {
	PublishRunEvent(ctx context.Context, runID string, event any) error
	PublishStepSchedule(ctx context.Context, runID, stepID string, payload any) error
	PublishStepWithTrace(ctx context.Context, runID, stepID string, step workflows.ExecutionStep) error
	SubscribeDurable(subject, consumer string, handler func(msg *nats.Msg)) (*nats.Subscription, error)
	Close()
}

type NATSEngine struct {
	opts   Options
	server *natsserver.Server
	conn   *nats.Conn
	js     nats.JetStreamContext
}

func NewEngine(opts Options) (*NATSEngine, error) {
	if !opts.Enabled {
		return nil, nil
	}

	engine := &NATSEngine{opts: opts}
	if opts.Embedded {
		srv, err := natsserver.NewServer(&natsserver.Options{Port: -1, JetStream: true})
		if err != nil {
			return nil, fmt.Errorf("failed to start embedded nats: %w", err)
		}
		go srv.Start()
		if !srv.ReadyForConnections(5 * time.Second) {
			return nil, fmt.Errorf("embedded nats failed to start")
		}
		engine.server = srv
		engine.opts.URL = fmt.Sprintf("nats://%s", srv.Addr().String())
	}

	conn, err := nats.Connect(engine.opts.URL)
	if err != nil {
		engine.Close()
		return nil, fmt.Errorf("failed to connect to nats: %w", err)
	}
	engine.conn = conn

	js, err := conn.JetStream()
	if err != nil {
		engine.Close()
		return nil, fmt.Errorf("failed to init jetstream: %w", err)
	}
	engine.js = js

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     opts.Stream,
		Subjects: []string{fmt.Sprintf("%s.>", opts.SubjectPrefix)},
		Storage:  nats.FileStorage,
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		engine.Close()
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	return engine, nil
}

func (e *NATSEngine) PublishRunEvent(ctx context.Context, runID string, event any) error {
	if e == nil || e.js == nil {
		return nil
	}
	subject := fmt.Sprintf("%s.events.%s", e.opts.SubjectPrefix, runID)
	return e.publishJSON(subject, event)
}

func (e *NATSEngine) PublishStepSchedule(ctx context.Context, runID, stepID string, payload any) error {
	if e == nil || e.js == nil {
		log.Printf("NATS Engine: PublishStepSchedule called but engine is nil (runID=%s, stepID=%s)", runID, stepID)
		return nil
	}
	subject := fmt.Sprintf("%s.run.%s.step.%s.schedule", e.opts.SubjectPrefix, runID, stepID)
	log.Printf("NATS Engine: Publishing step schedule to subject=%s (runID=%s, stepID=%s)", subject, runID, stepID)
	err := e.publishJSON(subject, payload)
	if err != nil {
		log.Printf("NATS Engine: Failed to publish step schedule: %v", err)
	} else {
		log.Printf("NATS Engine: Successfully published step schedule for run=%s step=%s", runID, stepID)
	}
	return err
}

func (e *NATSEngine) PublishStepWithTrace(ctx context.Context, runID, stepID string, step workflows.ExecutionStep) error {
	if e == nil || e.js == nil {
		log.Printf("NATS Engine: PublishStepWithTrace called but engine is nil (runID=%s, stepID=%s)", runID, stepID)
		return nil
	}
	subject := fmt.Sprintf("%s.run.%s.step.%s.schedule", e.opts.SubjectPrefix, runID, stepID)
	log.Printf("NATS Engine: Publishing step with trace to subject=%s (runID=%s, stepID=%s)", subject, runID, stepID)

	data, err := MarshalStepWithTrace(ctx, step)
	if err != nil {
		log.Printf("NATS Engine: Failed to marshal step with trace: %v", err)
		return err
	}

	_, err = e.js.Publish(subject, data)
	if err != nil {
		log.Printf("NATS Engine: Failed to publish step with trace: %v", err)
	} else {
		log.Printf("NATS Engine: Successfully published step with trace for run=%s step=%s", runID, stepID)
	}
	return err
}

func (e *NATSEngine) SubscribeDurable(subject, consumer string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	if e == nil || e.js == nil {
		return nil, fmt.Errorf("engine not initialized")
	}

	if consumer == "" {
		consumer = e.opts.ConsumerName
	}

	log.Printf("NATS Engine: Binding to shared durable consumer=%s for subject=%s", consumer, subject)

	// Create durable consumer config if it doesn't exist
	// Multiple instances with same consumer name = work queue pattern (HA scale-out)
	consumerConfig := &nats.ConsumerConfig{
		Durable:       consumer,
		FilterSubject: subject,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       60 * time.Second,
		MaxDeliver:    3,
		DeliverPolicy: nats.DeliverAllPolicy,
	}

	// Try to add consumer (will return error if exists, which is fine - we'll bind to it)
	_, err := e.js.AddConsumer(e.opts.Stream, consumerConfig)
	if err != nil {
		// Consumer already exists is expected in HA setup - multiple instances bind to same consumer
		log.Printf("NATS Engine: Consumer setup note: %v (this is normal if consumer already exists)", err)
	}

	// Bind to the shared durable consumer
	// In HA mode, multiple instances bind to the SAME consumer = NATS distributes work
	sub, err := e.js.PullSubscribe(
		subject,
		consumer,
		nats.Bind(e.opts.Stream, consumer),
	)
	if err != nil {
		log.Printf("NATS Engine: PullSubscribe failed: %v", err)
		return nil, fmt.Errorf("jetstream pull subscribe failed: %w", err)
	}

	info, infoErr := sub.ConsumerInfo()
	if infoErr == nil {
		log.Printf("NATS Engine: Bound to shared consumer - Name=%s, NumPending=%d, NumWaiting=%d, NumAckPending=%d",
			info.Name, info.NumPending, info.NumWaiting, info.NumAckPending)
	}

	go e.pullFetchLoop(sub, handler)

	return sub, nil
}

func (e *NATSEngine) pullFetchLoop(sub *nats.Subscription, handler func(msg *nats.Msg)) {
	log.Printf("NATS Engine: Starting pull fetch loop")

	for {
		if !sub.IsValid() {
			log.Printf("NATS Engine: Subscription no longer valid, stopping fetch loop")
			return
		}

		msgs, err := sub.Fetch(10, nats.MaxWait(5*time.Second))
		if err != nil {
			if err == nats.ErrTimeout {
				continue
			}
			if err == nats.ErrConnectionClosed || err == nats.ErrConsumerDeleted {
				log.Printf("NATS Engine: Connection or consumer closed, stopping fetch loop")
				return
			}
			log.Printf("NATS Engine: Fetch error: %v", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		for _, msg := range msgs {
			log.Printf("NATS Engine: [PULL] Fetched message on subject=%s", msg.Subject)
			handler(msg)
		}
	}
}

func (e *NATSEngine) publishJSON(subject string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = e.js.Publish(subject, data)
	return err
}

func (e *NATSEngine) Close() {
	if e == nil {
		return
	}
	if e.conn != nil {
		e.conn.Drain()
		e.conn.Close()
	}
	if e.server != nil {
		e.server.Shutdown()
	}
}

func NewEmbeddedEngineForTests() (*NATSEngine, error) {
	serverOpts := natsserver_test.DefaultTestOptions
	serverOpts.Port = -1
	serverOpts.JetStream = true
	srv := natsserver_test.RunServer(&serverOpts)
	opts := Options{
		Enabled:       true,
		URL:           srv.ClientURL(),
		Stream:        "WORKFLOW_EVENTS",
		SubjectPrefix: "workflow",
		ConsumerName:  "test-consumer",
		Embedded:      false,
	}
	engine, err := NewEngine(opts)
	if err != nil {
		srv.Shutdown()
		return nil, err
	}
	engine.server = srv
	return engine, nil
}
