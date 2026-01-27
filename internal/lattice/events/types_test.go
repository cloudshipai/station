package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewCloudEvent(t *testing.T) {
	event := NewCloudEvent(EventTypeStationJoined, EventSourcePrefix)

	if event.SpecVersion != CloudEventsSpecVersion {
		t.Errorf("expected specversion %s, got %s", CloudEventsSpecVersion, event.SpecVersion)
	}

	if event.Type != EventTypeStationJoined {
		t.Errorf("expected type %s, got %s", EventTypeStationJoined, event.Type)
	}

	if event.Source != EventSourcePrefix {
		t.Errorf("expected source %s, got %s", EventSourcePrefix, event.Source)
	}

	if event.ID == "" {
		t.Error("expected non-empty ID")
	}

	if event.Time.IsZero() {
		t.Error("expected non-zero time")
	}
}

func TestCloudEvent_WithData(t *testing.T) {
	event := NewCloudEvent(EventTypeWorkAssigned, EventSourcePrefix)

	data := &WorkAssignedData{
		WorkID:        "work-123",
		SourceStation: "station-a",
		TargetStation: "station-b",
		AgentName:     "test-agent",
		Task:          "do something",
		AssignedAt:    time.Now(),
	}

	if err := event.WithData(data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(event.Data) == 0 {
		t.Error("expected non-empty data")
	}

	var parsed WorkAssignedData
	if err := json.Unmarshal(event.Data, &parsed); err != nil {
		t.Fatalf("failed to parse data: %v", err)
	}

	if parsed.WorkID != data.WorkID {
		t.Errorf("expected work_id %s, got %s", data.WorkID, parsed.WorkID)
	}
}

func TestCloudEvent_WithStation(t *testing.T) {
	event := NewCloudEvent(EventTypeAgentInvoked, EventSourcePrefix)
	event.WithStation("station-123", "my-station")

	if event.StationID != "station-123" {
		t.Errorf("expected station_id station-123, got %s", event.StationID)
	}

	if event.StationName != "my-station" {
		t.Errorf("expected station_name my-station, got %s", event.StationName)
	}
}

func TestCloudEvent_WithTracing(t *testing.T) {
	event := NewCloudEvent(EventTypeWorkCompleted, EventSourcePrefix)
	event.WithTracing("trace-abc", "span-xyz")

	if event.TraceID != "trace-abc" {
		t.Errorf("expected trace_id trace-abc, got %s", event.TraceID)
	}

	if event.SpanID != "span-xyz" {
		t.Errorf("expected span_id span-xyz, got %s", event.SpanID)
	}
}

func TestCloudEvent_MarshalJSON(t *testing.T) {
	event := NewCloudEvent(EventTypeStationLeft, EventSourcePrefix)
	event.WithStation("station-1", "test")

	data := &StationLeftData{
		StationID:   "station-1",
		StationName: "test",
		LeftAt:      time.Now().UTC(),
		Reason:      "graceful",
	}
	if err := event.WithData(data); err != nil {
		t.Fatalf("failed to set data: %v", err)
	}

	bytes, err := event.MarshalJSON()
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed CloudEvent
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Type != EventTypeStationLeft {
		t.Errorf("expected type %s, got %s", EventTypeStationLeft, parsed.Type)
	}

	if parsed.StationID != "station-1" {
		t.Errorf("expected station_id station-1, got %s", parsed.StationID)
	}
}

func TestEventTypes(t *testing.T) {
	expectedTypes := map[string]string{
		"station.joined":     EventTypeStationJoined,
		"station.left":       EventTypeStationLeft,
		"agent.registered":   EventTypeAgentRegistered,
		"agent.deregistered": EventTypeAgentDeregistered,
		"agent.invoked":      EventTypeAgentInvoked,
		"work.assigned":      EventTypeWorkAssigned,
		"work.accepted":      EventTypeWorkAccepted,
		"work.progress":      EventTypeWorkProgress,
		"work.completed":     EventTypeWorkCompleted,
		"work.failed":        EventTypeWorkFailed,
		"work.escalated":     EventTypeWorkEscalated,
		"work.cancelled":     EventTypeWorkCancelled,
	}

	for name, eventType := range expectedTypes {
		if eventType == "" {
			t.Errorf("event type for %s is empty", name)
		}
		if eventType[:15] != "station.lattice" {
			t.Errorf("event type %s should start with station.lattice", eventType)
		}
	}
}
