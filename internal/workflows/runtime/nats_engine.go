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
	JetStream() nats.JetStreamContext
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
		port := opts.EmbeddedPort
		if port == 0 {
			port = 4222
		}
		srv, err := natsserver.NewServer(&natsserver.Options{
			Port:      port,
			JetStream: true,
			StoreDir:  "",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to start embedded nats on port %d: %w", port, err)
		}
		go srv.Start()
		if !srv.ReadyForConnections(5 * time.Second) {
			return nil, fmt.Errorf("embedded nats failed to start on port %d", port)
		}
		engine.server = srv
		engine.opts.URL = fmt.Sprintf("nats://127.0.0.1:%d", port)
		log.Printf("Embedded NATS started on port %d", port)
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

	storageType := nats.FileStorage
	if opts.Embedded {
		storageType = nats.MemoryStorage
		_ = js.DeleteStream(opts.Stream)
	}

	streamConfig := &nats.StreamConfig{
		Name:     opts.Stream,
		Subjects: []string{fmt.Sprintf("%s.>", opts.SubjectPrefix)},
		Storage:  storageType,
	}

	_, err = js.AddStream(streamConfig)
	if err != nil {
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

	log.Printf("NATS Engine: Setting up consumer=%s for subject=%s (embedded=%v)", consumer, subject, e.opts.Embedded)

	consumerConfig := &nats.ConsumerConfig{
		Durable:       consumer,
		FilterSubject: subject,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       60 * time.Second,
		MaxDeliver:    3,
		DeliverPolicy: nats.DeliverAllPolicy,
	}

	if e.opts.Embedded {
		_ = e.js.DeleteConsumer(e.opts.Stream, consumer)
		log.Printf("NATS Engine: Deleted existing consumer for clean embedded state")
	}

	_, err := e.js.AddConsumer(e.opts.Stream, consumerConfig)
	if err != nil {
		log.Printf("NATS Engine: Consumer setup note: %v", err)
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

	workerPoolSize := e.opts.WorkerPoolSize
	if workerPoolSize <= 0 {
		workerPoolSize = 10
	}
	go e.pullFetchLoop(sub, handler, workerPoolSize)

	return sub, nil
}

func (e *NATSEngine) pullFetchLoop(sub *nats.Subscription, handler func(msg *nats.Msg), workerPoolSize int) {
	log.Printf("NATS Engine: Starting pull fetch loop with %d concurrent workers", workerPoolSize)

	workCh := make(chan *nats.Msg, workerPoolSize*2)

	for i := 0; i < workerPoolSize; i++ {
		go func(workerID int) {
			for msg := range workCh {
				log.Printf("NATS Engine: [Worker %d] Processing message on subject=%s", workerID, msg.Subject)
				handler(msg)
			}
		}(i)
	}

	for {
		if !sub.IsValid() {
			log.Printf("NATS Engine: Subscription no longer valid, stopping fetch loop")
			close(workCh)
			return
		}

		msgs, err := sub.Fetch(workerPoolSize, nats.MaxWait(5*time.Second))
		if err != nil {
			if err == nats.ErrTimeout {
				continue
			}
			if err == nats.ErrConnectionClosed || err == nats.ErrConsumerDeleted {
				log.Printf("NATS Engine: Connection or consumer closed, stopping fetch loop")
				close(workCh)
				return
			}
			log.Printf("NATS Engine: Fetch error: %v", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		for _, msg := range msgs {
			log.Printf("NATS Engine: [PULL] Dispatching message to worker pool: subject=%s", msg.Subject)
			workCh <- msg
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

func (e *NATSEngine) JetStream() nats.JetStreamContext {
	if e == nil {
		return nil
	}
	return e.js
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
		Enabled:        true,
		URL:            srv.ClientURL(),
		Stream:         "WORKFLOW_EVENTS",
		SubjectPrefix:  "workflow",
		ConsumerName:   "test-consumer",
		Embedded:       false, // Server already started above, don't start another
		WorkerPoolSize: 10,
	}
	engine, err := NewEngine(opts)
	if err != nil {
		srv.Shutdown()
		return nil, err
	}
	engine.server = srv
	return engine, nil
}
