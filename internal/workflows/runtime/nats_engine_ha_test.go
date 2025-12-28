package runtime

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	natsserver_test "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

func newFreshEngineForHATest(t *testing.T, streamName, subjectPrefix string) *NATSEngine {
	serverOpts := natsserver_test.DefaultTestOptions
	serverOpts.Port = -1
	serverOpts.JetStream = true
	serverOpts.StoreDir = t.TempDir()
	srv := natsserver_test.RunServer(&serverOpts)

	conn, err := nats.Connect(srv.ClientURL())
	if err != nil {
		srv.Shutdown()
		t.Fatalf("failed to connect: %v", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		srv.Shutdown()
		t.Fatalf("failed to get jetstream: %v", err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{fmt.Sprintf("%s.>", subjectPrefix)},
		Storage:  nats.MemoryStorage,
	})
	if err != nil {
		conn.Close()
		srv.Shutdown()
		t.Fatalf("failed to create stream: %v", err)
	}

	engine := &NATSEngine{
		opts: Options{
			Enabled:       true,
			URL:           srv.ClientURL(),
			Stream:        streamName,
			SubjectPrefix: subjectPrefix,
			ConsumerName:  "test-consumer",
			Embedded:      false,
		},
		server: srv,
		conn:   conn,
		js:     js,
	}

	return engine
}

func TestNATSEngine_SharedConsumer_WorkDistribution(t *testing.T) {
	engine := newFreshEngineForHATest(t, "HA_TEST_STREAM", "hatest")
	defer engine.Close()

	var consumer1Count, consumer2Count int32
	var totalProcessed int32
	messageCount := 10

	done := make(chan struct{})

	sub1, err := engine.SubscribeDurable("hatest.run.*.step.*.schedule", "shared-ha-consumer", func(msg *nats.Msg) {
		atomic.AddInt32(&consumer1Count, 1)
		if atomic.AddInt32(&totalProcessed, 1) == int32(messageCount) {
			close(done)
		}
		_ = msg.Ack()
	})
	if err != nil {
		t.Fatalf("consumer 1 subscribe failed: %v", err)
	}
	defer func() { _ = sub1.Unsubscribe() }()

	sub2, err := engine.SubscribeDurable("hatest.run.*.step.*.schedule", "shared-ha-consumer", func(msg *nats.Msg) {
		atomic.AddInt32(&consumer2Count, 1)
		if atomic.AddInt32(&totalProcessed, 1) == int32(messageCount) {
			close(done)
		}
		_ = msg.Ack()
	})
	if err != nil {
		t.Fatalf("consumer 2 subscribe failed: %v", err)
	}
	defer func() { _ = sub2.Unsubscribe() }()

	time.Sleep(200 * time.Millisecond)

	for i := 0; i < messageCount; i++ {
		subject := fmt.Sprintf("hatest.run.run-%d.step.step-1.schedule", i)
		_, err := engine.js.Publish(subject, []byte(fmt.Sprintf(`{"id":"step-%d","type":"agent"}`, i)))
		if err != nil {
			t.Fatalf("publish failed: %v", err)
		}
	}

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("timeout waiting for messages - got c1=%d c2=%d, expected total %d",
			consumer1Count, consumer2Count, messageCount)
	}

	total := consumer1Count + consumer2Count
	t.Logf("Consumer 1: %d messages, Consumer 2: %d messages, Total: %d",
		consumer1Count, consumer2Count, total)

	if total != int32(messageCount) {
		t.Errorf("DUPLICATE PROCESSING DETECTED: expected %d total messages, got %d", messageCount, total)
	}
}

func TestNATSEngine_SharedConsumer_NoDuplicates(t *testing.T) {
	engine := newFreshEngineForHATest(t, "DEDUP_TEST_STREAM", "deduptest")
	defer engine.Close()

	processedMessages := make(map[string]int)
	var mu sync.Mutex
	var totalProcessed int32
	messageCount := 20

	done := make(chan struct{})

	handler := func(consumerID string) func(msg *nats.Msg) {
		return func(msg *nats.Msg) {
			mu.Lock()
			key := msg.Subject
			processedMessages[key]++
			mu.Unlock()
			if atomic.AddInt32(&totalProcessed, 1) == int32(messageCount) {
				close(done)
			}
			_ = msg.Ack()
		}
	}

	sub1, err := engine.SubscribeDurable("deduptest.run.*.step.*.schedule", "dedup-consumer", handler("c1"))
	if err != nil {
		t.Fatalf("consumer 1 subscribe failed: %v", err)
	}
	defer func() { _ = sub1.Unsubscribe() }()

	sub2, err := engine.SubscribeDurable("deduptest.run.*.step.*.schedule", "dedup-consumer", handler("c2"))
	if err != nil {
		t.Fatalf("consumer 2 subscribe failed: %v", err)
	}
	defer func() { _ = sub2.Unsubscribe() }()

	sub3, err := engine.SubscribeDurable("deduptest.run.*.step.*.schedule", "dedup-consumer", handler("c3"))
	if err != nil {
		t.Fatalf("consumer 3 subscribe failed: %v", err)
	}
	defer func() { _ = sub3.Unsubscribe() }()

	time.Sleep(200 * time.Millisecond)

	for i := 0; i < messageCount; i++ {
		subject := fmt.Sprintf("deduptest.run.run-%d.step.step-1.schedule", i)
		_, err := engine.js.Publish(subject, []byte(fmt.Sprintf(`{"id":"step-%d","type":"agent"}`, i)))
		if err != nil {
			t.Fatalf("publish failed: %v", err)
		}
	}

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for messages")
	}

	mu.Lock()
	defer mu.Unlock()

	duplicates := 0
	for key, count := range processedMessages {
		if count > 1 {
			duplicates++
			t.Errorf("Message %s processed %d times (should be exactly 1)", key, count)
		}
	}

	if duplicates > 0 {
		t.Errorf("Found %d duplicated messages out of %d", duplicates, len(processedMessages))
	} else {
		t.Logf("SUCCESS: All %d messages processed exactly once", len(processedMessages))
	}
}

func TestNATSEngine_DifferentConsumers_DuplicateProcessing(t *testing.T) {
	serverOpts := natsserver_test.DefaultTestOptions
	serverOpts.Port = -1
	serverOpts.JetStream = true
	serverOpts.StoreDir = t.TempDir()
	srv := natsserver_test.RunServer(&serverOpts)
	defer srv.Shutdown()

	conn, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	js, err := conn.JetStream()
	if err != nil {
		t.Fatalf("failed to get jetstream: %v", err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "DUP_TEST",
		Subjects: []string{"duptest.>"},
		Storage:  nats.MemoryStorage,
	})
	if err != nil {
		t.Fatalf("failed to create stream: %v", err)
	}

	var consumer1Count, consumer2Count int32
	messageCount := 5
	expectedTotal := messageCount * 2

	done := make(chan struct{})
	var totalProcessed int32

	createConsumer := func(name string, counter *int32) *nats.Subscription {
		_, _ = js.AddConsumer("DUP_TEST", &nats.ConsumerConfig{
			Durable:       name,
			FilterSubject: "duptest.run.*.step.*.schedule",
			AckPolicy:     nats.AckExplicitPolicy,
			DeliverPolicy: nats.DeliverAllPolicy,
		})

		sub, err := js.PullSubscribe(
			"duptest.run.*.step.*.schedule",
			name,
			nats.Bind("DUP_TEST", name),
		)
		if err != nil {
			t.Fatalf("subscribe failed for %s: %v", name, err)
		}

		go func() {
			for {
				msgs, err := sub.Fetch(10, nats.MaxWait(1*time.Second))
				if err != nil {
					if err == nats.ErrTimeout {
						continue
					}
					return
				}
				for _, msg := range msgs {
					atomic.AddInt32(counter, 1)
					if atomic.AddInt32(&totalProcessed, 1) == int32(expectedTotal) {
						close(done)
					}
					_ = msg.Ack()
				}
			}
		}()

		return sub
	}

	sub1 := createConsumer("consumer-A", &consumer1Count)
	sub2 := createConsumer("consumer-B", &consumer2Count)
	defer func() { _ = sub1.Unsubscribe() }()
	defer func() { _ = sub2.Unsubscribe() }()

	time.Sleep(200 * time.Millisecond)

	for i := 0; i < messageCount; i++ {
		subject := fmt.Sprintf("duptest.run.run-%d.step.step-1.schedule", i)
		_, err := js.Publish(subject, []byte(fmt.Sprintf(`{"id":"step-%d"}`, i)))
		if err != nil {
			t.Fatalf("publish failed: %v", err)
		}
	}

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("timeout - got c1=%d c2=%d, expected %d each", consumer1Count, consumer2Count, messageCount)
	}

	t.Logf("Consumer A: %d, Consumer B: %d", consumer1Count, consumer2Count)

	if consumer1Count != int32(messageCount) || consumer2Count != int32(messageCount) {
		t.Errorf("Expected both consumers to get %d messages each (demonstrating duplicate bug), got c1=%d c2=%d",
			messageCount, consumer1Count, consumer2Count)
	} else {
		t.Logf("CONFIRMED: Different consumer names = duplicate processing (each got all %d messages)", messageCount)
	}
}
