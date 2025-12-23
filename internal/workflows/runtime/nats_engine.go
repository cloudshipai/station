package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	natsserver_test "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

type Engine interface {
	PublishRunEvent(ctx context.Context, runID string, event any) error
	PublishStepSchedule(ctx context.Context, runID, stepID string, payload any) error
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
		return nil
	}
	subject := fmt.Sprintf("%s.run.%s.step.%s.schedule", e.opts.SubjectPrefix, runID, stepID)
	return e.publishJSON(subject, payload)
}

func (e *NATSEngine) SubscribeDurable(subject, consumer string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	if e == nil || e.js == nil {
		return nil, fmt.Errorf("engine not initialized")
	}

	if consumer == "" {
		consumer = e.opts.ConsumerName
	}

	sub, err := e.js.Subscribe(
		subject,
		handler,
		nats.Durable(consumer),
		nats.AckExplicit(),
		nats.DeliverAll(),
		nats.ManualAck(),
	)
	if err != nil {
		return nil, fmt.Errorf("jetstream subscribe failed: %w", err)
	}
	return sub, nil
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
