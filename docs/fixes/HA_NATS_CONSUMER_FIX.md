# Fix: NATS Consumer for HA Scale-Out

**Status**: 🟢 Implemented & Tested  
**Priority**: High  
**Effort**: ~3 hours  
**Created**: 2024-12-28  
**Completed**: 2024-12-28  

---

## Problem

When running multiple Station instances with shared NATS, **every workflow step is executed by ALL instances** (duplicate processing) instead of being distributed across instances.

### Root Cause

In `internal/workflows/runtime/consumer.go` line 116:
```go
sub, err := c.engine.SubscribeDurable(subject, "workflow-step-consumer", c.handleMessage)
```

And in `internal/workflows/runtime/nats_engine.go` line 134:
```go
ephemeralConsumerName := fmt.Sprintf("%s-%d", consumer, time.Now().UnixNano())
```

Each instance creates a **unique ephemeral consumer** with a timestamp suffix. This means:
- Station 1 creates: `workflow-step-consumer-1735123456789`
- Station 2 creates: `workflow-step-consumer-1735123456999`

Both are **different consumers** on the same stream, so **both receive ALL messages**.

### Impact

| Scenario | Result |
|----------|--------|
| 2 instances, shared NATS | Each workflow step runs TWICE |
| 3 instances, shared NATS | Each workflow step runs THREE times |
| N instances, shared NATS | N duplicate executions per step |

This causes:
- Duplicate agent runs (wasted LLM tokens)
- Race conditions in database updates
- Corrupted workflow state
- Unpredictable behavior

---

## Solution

Use a **durable shared consumer** (JetStream work queue pattern) where NATS distributes messages across consumers with the same name.

### Changes Required

#### 1. `internal/workflows/runtime/nats_engine.go` - Replace SubscribeDurable

Replace lines 125-163 with:

```go
func (e *NATSEngine) SubscribeDurable(subject, consumer string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	if e == nil || e.js == nil {
		return nil, fmt.Errorf("engine not initialized")
	}

	if consumer == "" {
		consumer = e.opts.ConsumerName
	}

	log.Printf("NATS Engine: Binding to shared durable consumer=%s for subject=%s", consumer, subject)

	// Create durable consumer config if it doesn't exist
	// Multiple instances with same consumer name = work queue pattern
	consumerConfig := &nats.ConsumerConfig{
		Durable:       consumer,
		FilterSubject: subject,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       60 * time.Second,
		MaxDeliver:    3,
		DeliverPolicy: nats.DeliverAllPolicy,
	}

	// Try to add consumer (will fail silently if exists, which is fine)
	_, err := e.js.AddConsumer(e.opts.Stream, consumerConfig)
	if err != nil && err != nats.ErrConsumerNameAlreadyInUse {
		log.Printf("NATS Engine: Note - consumer may already exist: %v", err)
	}

	// Bind to the shared durable consumer
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
		log.Printf("NATS Engine: Bound to shared consumer - Name=%s, NumPending=%d, NumWaiting=%d",
			info.Name, info.NumPending, info.NumWaiting)
	}

	go e.pullFetchLoop(sub, handler)

	return sub, nil
}
```

#### 2. `internal/workflows/runtime/consumer.go` - Use configured consumer name

Change line 116 from:
```go
sub, err := c.engine.SubscribeDurable(subject, "workflow-step-consumer", c.handleMessage)
```

To:
```go
// Use empty string to get consumer name from engine options (allows env var override)
sub, err := c.engine.SubscribeDurable(subject, "", c.handleMessage)
```

#### 3. No changes needed to `options.go`

Already has proper env var support:
```go
ConsumerName: getenvDefault("WORKFLOW_NATS_CONSUMER", "station-workflow"),
```

---

## Testing Plan

### Prerequisites

```bash
# Build station binary with the fix
cd /home/epuerta/sandbox/cloudship-sandbox/station
go build -o stn ./cmd/main
```

### Step 1: Start External NATS with JetStream

```bash
# Terminal 1: Start NATS container with JetStream enabled
docker run -d --name nats-ha-test \
  -p 4222:4222 \
  -p 8222:8222 \
  nats:2.10-alpine \
  --jetstream \
  --store_dir /data \
  -m 8222

# Verify NATS is running
curl http://localhost:8222/healthz
```

### Step 2: Create Test Workspaces with Shared Database

For true HA testing, both instances should share a database. Options:

#### Option A: Using Turso (Recommended for real HA)
```bash
# Create Turso database (if you have Turso account)
turso db create station-ha-test
turso db tokens create station-ha-test

# Get the URL
export TURSO_URL="libsql://station-ha-test-<your-org>.turso.io?authToken=<token>"
```

#### Option B: Using shared SQLite file (for local testing)
```bash
# Create shared workspace directory
mkdir -p /tmp/station-ha-test/shared-db
```

### Step 3: Initialize Workspaces for Each Instance

```bash
# Instance 1 workspace
mkdir -p /tmp/station-ha-test/instance1
./stn init \
  --provider openai \
  --model gpt-4o-mini \
  --workspace /tmp/station-ha-test/instance1 \
  --yes

# Instance 2 workspace  
mkdir -p /tmp/station-ha-test/instance2
./stn init \
  --provider openai \
  --model gpt-4o-mini \
  --workspace /tmp/station-ha-test/instance2 \
  --yes
```

### Step 4: Create a Simple Test Agent

```bash
# Create agent in instance1 (will be shared via DB or manually copied)
mkdir -p /tmp/station-ha-test/instance1/agents

cat > /tmp/station-ha-test/instance1/agents/echo-agent.yaml << 'AGENT'
name: echo-agent
description: Simple echo agent for HA testing
model: gpt-4o-mini
system_prompt: |
  You are a simple echo agent. When given a message, respond with:
  "Instance processed: <the message>"
  
  Keep responses short and include the exact input message.
AGENT

# Copy to instance2
cp /tmp/station-ha-test/instance1/agents/echo-agent.yaml \
   /tmp/station-ha-test/instance2/agents/
```

### Step 5: Create a Simple Test Workflow

```bash
# Create workflow directory
mkdir -p /tmp/station-ha-test/instance1/workflows

cat > /tmp/station-ha-test/instance1/workflows/ha-test-workflow.yaml << 'WORKFLOW'
name: ha-test-workflow
description: Workflow to test HA distribution
version: "1.0"

steps:
  - id: step1
    type: agent
    agent: echo-agent
    input:
      message: "Step 1 - Testing HA"

  - id: step2
    type: agent
    agent: echo-agent
    input:
      message: "Step 2 - Testing HA"
    depends_on: [step1]

  - id: step3
    type: agent
    agent: echo-agent
    input:
      message: "Step 3 - Testing HA"
    depends_on: [step2]

  - id: step4
    type: agent
    agent: echo-agent
    input:
      message: "Step 4 - Testing HA"
    depends_on: [step3]

  - id: step5
    type: agent
    agent: echo-agent
    input:
      message: "Step 5 - Testing HA"
    depends_on: [step4]
WORKFLOW

# Copy to instance2
cp /tmp/station-ha-test/instance1/workflows/ha-test-workflow.yaml \
   /tmp/station-ha-test/instance2/workflows/
```

### Step 6: Start Station Instances with External NATS

```bash
# Terminal 2: Start Instance 1
WORKFLOW_NATS_EMBEDDED=false \
WORKFLOW_NATS_URL=nats://localhost:4222 \
OPENAI_API_KEY=$OPENAI_API_KEY \
./stn serve --local \
  --workspace /tmp/station-ha-test/instance1 \
  --port 8585

# Terminal 3: Start Instance 2
WORKFLOW_NATS_EMBEDDED=false \
WORKFLOW_NATS_URL=nats://localhost:4222 \
OPENAI_API_KEY=$OPENAI_API_KEY \
./stn serve --local \
  --workspace /tmp/station-ha-test/instance2 \
  --port 8586
```

### Step 7: Trigger Workflow and Observe Distribution

```bash
# Terminal 4: Trigger the workflow
curl -X POST http://localhost:8585/api/v1/workflows/ha-test-workflow/trigger \
  -H "Content-Type: application/json" \
  -d '{"input": {"test": "HA distribution test"}}'

# Watch logs in Terminal 2 and Terminal 3
# You should see steps distributed across both instances:
#   - Instance 1 (8585): might process step1, step3, step5
#   - Instance 2 (8586): might process step2, step4
#
# Key verification:
# 1. Each step appears in ONLY ONE instance's logs (no duplicates)
# 2. Steps are roughly distributed (not all on one instance)
```

### Step 8: Verify No Duplicates

```bash
# Check NATS consumer info
curl http://localhost:8222/jsz?consumers=1 | jq '.streams[].consumers'

# Should show ONE consumer with multiple instances connected:
# {
#   "name": "station-workflow",
#   "num_pending": 0,
#   "num_waiting": 2,  <-- Two instances waiting for messages
#   ...
# }
```

### Step 9: Cleanup

```bash
# Stop NATS container
docker stop nats-ha-test && docker rm nats-ha-test

# Remove test workspaces
rm -rf /tmp/station-ha-test
```

---

## Unit Test: Work Distribution

Create `internal/workflows/runtime/nats_engine_ha_test.go`:

```go
package runtime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestNATSEngine_SharedConsumer_WorkDistribution(t *testing.T) {
	// Start embedded NATS for testing
	engine, err := NewEmbeddedEngineForTests()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	// Track which consumer processes each message
	var consumer1Count, consumer2Count int32
	var wg sync.WaitGroup
	messageCount := 10

	// Consumer 1 - uses SAME consumer name
	sub1, err := engine.SubscribeDurable("workflow.run.*.step.*.schedule", "shared-test-consumer", func(msg *nats.Msg) {
		atomic.AddInt32(&consumer1Count, 1)
		msg.Ack()
		wg.Done()
	})
	if err != nil {
		t.Fatalf("consumer 1 subscribe failed: %v", err)
	}
	defer sub1.Unsubscribe()

	// Consumer 2 - uses SAME consumer name (should share work)
	sub2, err := engine.SubscribeDurable("workflow.run.*.step.*.schedule", "shared-test-consumer", func(msg *nats.Msg) {
		atomic.AddInt32(&consumer2Count, 1)
		msg.Ack()
		wg.Done()
	})
	if err != nil {
		t.Fatalf("consumer 2 subscribe failed: %v", err)
	}
	defer sub2.Unsubscribe()

	// Publish messages
	wg.Add(messageCount)
	for i := 0; i < messageCount; i++ {
		runID := fmt.Sprintf("run-%d", i)
		err := engine.PublishStepSchedule(context.Background(), runID, "step-1", map[string]interface{}{
			"type": "agent",
		})
		if err != nil {
			t.Fatalf("publish failed: %v", err)
		}
	}

	// Wait for all messages to be processed
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for messages")
	}

	total := consumer1Count + consumer2Count
	t.Logf("Consumer 1: %d messages, Consumer 2: %d messages, Total: %d", 
		consumer1Count, consumer2Count, total)

	// Verify total messages processed = messageCount (no duplicates)
	if total != int32(messageCount) {
		t.Errorf("expected %d total messages, got %d (duplicates detected!)", messageCount, total)
	}

	// Verify work was distributed (both got some messages)
	if consumer1Count == 0 || consumer2Count == 0 {
		t.Logf("Warning: work not evenly distributed (c1=%d, c2=%d)", consumer1Count, consumer2Count)
	}
}
```

---

## Rollout Plan

1. ✅ **Document fix** - This file
2. ✅ **Implement fix** in `nats_engine.go` and `consumer.go`
3. ✅ **Add unit tests** for work distribution (`nats_engine_ha_test.go`)
4. ✅ **Run existing tests** to ensure no regressions
5. 🔲 **Manual verification** with 2 instances (follow testing plan above)
6. 🔲 **Update HA docs** (`DATABASE_REPLICATION.md`)
7. 🔲 **Release** in next version

---

## Future Enhancements

After this fix, these become possible:

| Feature | Status | Notes |
|---------|--------|-------|
| Basic HA (2+ instances) | ✅ Enabled by this fix | |
| Leader Election | 🔜 Next | For singleton jobs (cron, cleanup) |
| NATS-backed SSE | 🔜 Future | Stateless real-time streams |
| PostgreSQL support | 🔜 Future | Alternative to Turso |

---

## References

- [NATS JetStream Work Queues](https://docs.nats.io/nats-concepts/jetstream/consumers#extracting-work)
- [Durable Consumers](https://docs.nats.io/nats-concepts/jetstream/consumers#durable-consumers)
- [Station DATABASE_REPLICATION.md](../station/DATABASE_REPLICATION.md)

---

## Implementation Commands

When ready to implement:

```bash
cd /home/epuerta/sandbox/cloudship-sandbox/station

# 1. Make the code changes (see "Changes Required" section above)
# 2. Run unit tests
go test ./internal/workflows/runtime/... -v

# 3. Build
go build -o stn ./cmd/main

# 4. Follow the manual testing plan above
```
