// Package events provides CloudEvents-compliant event types for the Station Lattice.
// Events are 100% captured business facts (audit trail), distinct from traces (sampled debugging).
package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CloudEvents spec version
const CloudEventsSpecVersion = "1.0"

// Event source prefix for lattice events
const EventSourcePrefix = "station.lattice"

// Event types - following CloudEvents naming convention: {domain}.{entity}.{action}
const (
	// Station lifecycle events
	EventTypeStationJoined = "station.lattice.station.joined"
	EventTypeStationLeft   = "station.lattice.station.left"

	// Agent events
	EventTypeAgentRegistered   = "station.lattice.agent.registered"
	EventTypeAgentDeregistered = "station.lattice.agent.deregistered"
	EventTypeAgentInvoked      = "station.lattice.agent.invoked"

	// Work events
	EventTypeWorkAssigned  = "station.lattice.work.assigned"
	EventTypeWorkAccepted  = "station.lattice.work.accepted"
	EventTypeWorkProgress  = "station.lattice.work.progress"
	EventTypeWorkCompleted = "station.lattice.work.completed"
	EventTypeWorkFailed    = "station.lattice.work.failed"
	EventTypeWorkEscalated = "station.lattice.work.escalated"
	EventTypeWorkCancelled = "station.lattice.work.cancelled"
)

// CloudEvent represents a CloudEvents 1.0 compliant event
type CloudEvent struct {
	// Required attributes
	SpecVersion string    `json:"specversion"`
	Type        string    `json:"type"`
	Source      string    `json:"source"`
	ID          string    `json:"id"`
	Time        time.Time `json:"time"`

	// Optional attributes
	DataContentType string `json:"datacontenttype,omitempty"`
	Subject         string `json:"subject,omitempty"`

	// Extension attributes for distributed tracing
	TraceID      string `json:"traceid,omitempty"`
	SpanID       string `json:"spanid,omitempty"`
	TraceParent  string `json:"traceparent,omitempty"`
	TraceContext string `json:"tracecontext,omitempty"`

	// Extension attributes for lattice
	StationID   string `json:"stationid,omitempty"`
	StationName string `json:"stationname,omitempty"`

	// Event data (payload)
	Data json.RawMessage `json:"data,omitempty"`
}

// NewCloudEvent creates a new CloudEvent with required fields populated
func NewCloudEvent(eventType, source string) *CloudEvent {
	return &CloudEvent{
		SpecVersion:     CloudEventsSpecVersion,
		Type:            eventType,
		Source:          source,
		ID:              uuid.NewString(),
		Time:            time.Now().UTC(),
		DataContentType: "application/json",
	}
}

// WithData sets the event data payload
func (e *CloudEvent) WithData(data any) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	e.Data = bytes
	return nil
}

// WithSubject sets the event subject
func (e *CloudEvent) WithSubject(subject string) *CloudEvent {
	e.Subject = subject
	return e
}

// WithTracing sets distributed tracing context
func (e *CloudEvent) WithTracing(traceID, spanID string) *CloudEvent {
	e.TraceID = traceID
	e.SpanID = spanID
	return e
}

// WithStation sets station identity
func (e *CloudEvent) WithStation(stationID, stationName string) *CloudEvent {
	e.StationID = stationID
	e.StationName = stationName
	return e
}

// MarshalJSON implements json.Marshaler
func (e *CloudEvent) MarshalJSON() ([]byte, error) {
	type Alias CloudEvent
	return json.Marshal((*Alias)(e))
}

// UnmarshalJSON implements json.Unmarshaler
func (e *CloudEvent) UnmarshalJSON(data []byte) error {
	type Alias CloudEvent
	return json.Unmarshal(data, (*Alias)(e))
}

// --- Event Data Payloads ---

// StationJoinedData is the payload for station.joined events
type StationJoinedData struct {
	StationID   string    `json:"station_id"`
	StationName string    `json:"station_name"`
	Version     string    `json:"version,omitempty"`
	Environment string    `json:"environment,omitempty"`
	JoinedAt    time.Time `json:"joined_at"`
	AgentCount  int       `json:"agent_count,omitempty"`
}

// StationLeftData is the payload for station.left events
type StationLeftData struct {
	StationID   string    `json:"station_id"`
	StationName string    `json:"station_name"`
	LeftAt      time.Time `json:"left_at"`
	Reason      string    `json:"reason,omitempty"` // graceful, timeout, error
}

// AgentRegisteredData is the payload for agent.registered events
type AgentRegisteredData struct {
	StationID   string   `json:"station_id"`
	StationName string   `json:"station_name"`
	AgentID     string   `json:"agent_id"`
	AgentName   string   `json:"agent_name"`
	AgentType   string   `json:"agent_type,omitempty"`
	Tools       []string `json:"tools,omitempty"`
}

// AgentDeregisteredData is the payload for agent.deregistered events
type AgentDeregisteredData struct {
	StationID   string `json:"station_id"`
	StationName string `json:"station_name"`
	AgentID     string `json:"agent_id"`
	AgentName   string `json:"agent_name"`
	Reason      string `json:"reason,omitempty"`
}

// AgentInvokedData is the payload for agent.invoked events
type AgentInvokedData struct {
	StationID    string `json:"station_id"`
	AgentID      string `json:"agent_id"`
	AgentName    string `json:"agent_name"`
	Task         string `json:"task"`
	RunID        string `json:"run_id"`
	RequestedBy  string `json:"requested_by,omitempty"` // station ID that requested
	WorkID       string `json:"work_id,omitempty"`
	ParentWorkID string `json:"parent_work_id,omitempty"`
}

// WorkAssignedData is the payload for work.assigned events
type WorkAssignedData struct {
	WorkID            string    `json:"work_id"`
	OrchestratorRunID string    `json:"orchestrator_run_id,omitempty"`
	ParentWorkID      string    `json:"parent_work_id,omitempty"`
	SourceStation     string    `json:"source_station"`
	TargetStation     string    `json:"target_station"`
	AgentID           string    `json:"agent_id,omitempty"`
	AgentName         string    `json:"agent_name"`
	Task              string    `json:"task"`
	AssignedAt        time.Time `json:"assigned_at"`
}

// WorkAcceptedData is the payload for work.accepted events
type WorkAcceptedData struct {
	WorkID        string    `json:"work_id"`
	StationID     string    `json:"station_id"`
	AgentID       string    `json:"agent_id"`
	AgentName     string    `json:"agent_name"`
	AcceptedAt    time.Time `json:"accepted_at"`
	EstimatedTime int64     `json:"estimated_time_ms,omitempty"`
}

// WorkProgressData is the payload for work.progress events
type WorkProgressData struct {
	WorkID         string  `json:"work_id"`
	StationID      string  `json:"station_id"`
	AgentName      string  `json:"agent_name"`
	ProgressPct    float64 `json:"progress_pct,omitempty"`
	CurrentStep    string  `json:"current_step,omitempty"`
	StepsCompleted int     `json:"steps_completed,omitempty"`
	TotalSteps     int     `json:"total_steps,omitempty"`
	Message        string  `json:"message,omitempty"`
}

// WorkCompletedData is the payload for work.completed events
type WorkCompletedData struct {
	WorkID      string    `json:"work_id"`
	StationID   string    `json:"station_id"`
	AgentID     string    `json:"agent_id"`
	AgentName   string    `json:"agent_name"`
	Result      string    `json:"result,omitempty"` // truncated for audit
	CompletedAt time.Time `json:"completed_at"`
	DurationMs  int64     `json:"duration_ms"`
	ToolCalls   int       `json:"tool_calls,omitempty"`
}

// WorkFailedData is the payload for work.failed events
type WorkFailedData struct {
	WorkID     string    `json:"work_id"`
	StationID  string    `json:"station_id"`
	AgentID    string    `json:"agent_id"`
	AgentName  string    `json:"agent_name"`
	Error      string    `json:"error"`
	FailedAt   time.Time `json:"failed_at"`
	DurationMs int64     `json:"duration_ms"`
	ToolCalls  int       `json:"tool_calls,omitempty"`
	Retryable  bool      `json:"retryable,omitempty"`
}

// WorkEscalatedData is the payload for work.escalated events
type WorkEscalatedData struct {
	WorkID        string    `json:"work_id"`
	StationID     string    `json:"station_id"`
	AgentID       string    `json:"agent_id"`
	AgentName     string    `json:"agent_name"`
	Reason        string    `json:"reason"`
	RetryCount    int       `json:"retry_count"`
	EscalatedAt   time.Time `json:"escalated_at"`
	EscalatedTo   string    `json:"escalated_to,omitempty"` // target for escalation
	OriginalError string    `json:"original_error,omitempty"`
}

// WorkCancelledData is the payload for work.cancelled events
type WorkCancelledData struct {
	WorkID      string    `json:"work_id"`
	StationID   string    `json:"station_id"`
	AgentName   string    `json:"agent_name"`
	Reason      string    `json:"reason"`
	CancelledAt time.Time `json:"cancelled_at"`
	CancelledBy string    `json:"cancelled_by,omitempty"` // who cancelled
}
