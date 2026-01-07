package stream

import (
	"context"
	"testing"
	"time"
)

func testIdentifiers() StreamIdentifiers {
	return StreamIdentifiers{
		StationRunID:  "123",
		RunUUID:       "run-uuid-123",
		WorkflowRunID: "wf-456",
		SessionID:     "session-456",
		AgentID:       "agent-1",
		AgentName:     "test-agent",
		StationID:     "station-abc",
	}
}

func TestEvent_JSON(t *testing.T) {
	event := &Event{
		StationRunID:  "123",
		RunUUID:       "run-uuid-123",
		WorkflowRunID: "wf-456",
		SessionID:     "session-456",
		AgentID:       "agent-1",
		AgentName:     "test-agent",
		StationID:     "station-abc",
		Seq:           1,
		Timestamp:     time.Now(),
		Type:          EventToken,
		Data: TokenData{
			Content: "Hello",
			Done:    false,
		},
	}

	data, err := event.JSON()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty JSON")
	}
}

func TestStreamContext_EmitToken(t *testing.T) {
	pub := NewChannelPublisher(10)
	ctx := context.Background()
	ids := testIdentifiers()
	sc := NewStreamContext(ids, pub)

	err := sc.EmitToken(ctx, "Hello", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	select {
	case event := <-pub.Events():
		if event.Type != EventToken {
			t.Errorf("expected EventToken, got %v", event.Type)
		}
		if event.RunUUID != "run-uuid-123" {
			t.Errorf("expected run-uuid-123, got %s", event.RunUUID)
		}
		if event.StationRunID != "123" {
			t.Errorf("expected 123, got %s", event.StationRunID)
		}
		if event.WorkflowRunID != "wf-456" {
			t.Errorf("expected wf-456, got %s", event.WorkflowRunID)
		}
		if event.StationID != "station-abc" {
			t.Errorf("expected station-abc, got %s", event.StationID)
		}
		if event.AgentID != "agent-1" {
			t.Errorf("expected agent-1, got %s", event.AgentID)
		}
		if event.Seq != 1 {
			t.Errorf("expected seq 1, got %d", event.Seq)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestStreamContext_EmitToolStart(t *testing.T) {
	pub := NewChannelPublisher(10)
	ctx := context.Background()
	sc := NewStreamContext(testIdentifiers(), pub)

	err := sc.EmitToolStart(ctx, "bash", "tool-1", map[string]string{"command": "ls"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	select {
	case event := <-pub.Events():
		if event.Type != EventToolStart {
			t.Errorf("expected EventToolStart, got %v", event.Type)
		}
		data, ok := event.Data.(ToolStartData)
		if !ok {
			t.Fatal("expected ToolStartData")
		}
		if data.ToolName != "bash" {
			t.Errorf("expected bash, got %s", data.ToolName)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestStreamContext_EmitRunComplete(t *testing.T) {
	pub := NewChannelPublisher(10)
	ctx := context.Background()
	sc := NewStreamContext(testIdentifiers(), pub)

	err := sc.EmitRunComplete(ctx, true, 5, 1000, 5000, "agent_done", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	select {
	case event := <-pub.Events():
		if event.Type != EventRunComplete {
			t.Errorf("expected EventRunComplete, got %v", event.Type)
		}
		data, ok := event.Data.(RunCompleteData)
		if !ok {
			t.Fatal("expected RunCompleteData")
		}
		if !data.Success {
			t.Error("expected Success=true")
		}
		if data.TotalSteps != 5 {
			t.Errorf("expected 5 steps, got %d", data.TotalSteps)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestNoOpPublisher(t *testing.T) {
	pub := &NoOpPublisher{}
	ctx := context.Background()

	err := pub.Publish(ctx, &Event{Type: EventToken})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	err = pub.Close()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMultiPublisher(t *testing.T) {
	pub1 := NewChannelPublisher(10)
	pub2 := NewChannelPublisher(10)
	multi := NewMultiPublisher(pub1, pub2)
	ctx := context.Background()

	event := &Event{
		RunUUID:      "run-uuid-123",
		StationRunID: "123",
		Type:         EventToken,
		Data:         TokenData{Content: "test"},
	}

	err := multi.Publish(ctx, event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	select {
	case e := <-pub1.Events():
		if e.RunUUID != "run-uuid-123" {
			t.Errorf("pub1: expected run-uuid-123, got %s", e.RunUUID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for pub1 event")
	}

	select {
	case e := <-pub2.Events():
		if e.RunUUID != "run-uuid-123" {
			t.Errorf("pub2: expected run-uuid-123, got %s", e.RunUUID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for pub2 event")
	}
}

func TestChannelPublisher_ContextCancellation(t *testing.T) {
	pub := NewChannelPublisher(1)
	ctx, cancel := context.WithCancel(context.Background())

	pub.Publish(ctx, &Event{Type: EventToken})

	cancel()
	err := pub.Publish(ctx, &Event{Type: EventToken})
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestStreamContext_NilPublisher(t *testing.T) {
	sc := NewStreamContext(testIdentifiers(), nil)
	ctx := context.Background()

	err := sc.EmitToken(ctx, "test", false)
	if err != nil {
		t.Fatalf("expected no error with nil publisher, got %v", err)
	}
}

func TestStreamContext_SequenceIncrement(t *testing.T) {
	pub := NewChannelPublisher(10)
	ctx := context.Background()
	sc := NewStreamContext(testIdentifiers(), pub)

	sc.EmitToken(ctx, "a", false)
	sc.EmitToken(ctx, "b", false)
	sc.EmitToken(ctx, "c", true)

	var seqs []int64
	for i := 0; i < 3; i++ {
		select {
		case event := <-pub.Events():
			seqs = append(seqs, event.Seq)
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
	}

	if seqs[0] != 1 || seqs[1] != 2 || seqs[2] != 3 {
		t.Errorf("expected sequences [1,2,3], got %v", seqs)
	}
}

func TestStreamContext_Identifiers(t *testing.T) {
	ids := testIdentifiers()
	sc := NewStreamContext(ids, nil)

	got := sc.Identifiers()
	if got.StationRunID != ids.StationRunID {
		t.Errorf("StationRunID mismatch: %s vs %s", got.StationRunID, ids.StationRunID)
	}
	if got.RunUUID != ids.RunUUID {
		t.Errorf("RunUUID mismatch: %s vs %s", got.RunUUID, ids.RunUUID)
	}
	if got.WorkflowRunID != ids.WorkflowRunID {
		t.Errorf("WorkflowRunID mismatch: %s vs %s", got.WorkflowRunID, ids.WorkflowRunID)
	}
	if got.StationID != ids.StationID {
		t.Errorf("StationID mismatch: %s vs %s", got.StationID, ids.StationID)
	}
}
