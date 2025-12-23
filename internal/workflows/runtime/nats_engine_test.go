package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestEmbeddedEnginePublishesAndConsumes(t *testing.T) {
	engine, err := NewEmbeddedEngineForTests()
	if err != nil {
		t.Fatalf("failed to start embedded engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	var mu sync.Mutex
	var received []string

	sub, err := engine.SubscribeDurable("workflow.run.demo.step.*.schedule", "test-consumer", func(msg *nats.Msg) {
		mu.Lock()
		received = append(received, string(msg.Data))
		msg.Ack()
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	payload := map[string]interface{}{"step": "start"}
	if err := engine.PublishStepSchedule(ctx, "demo", "start", payload); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Fatalf("expected to receive schedule message")
	}
}
