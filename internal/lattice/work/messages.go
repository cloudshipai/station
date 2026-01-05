package work

import (
	"time"
)

const (
	MsgWorkAssigned  = "WORK_ASSIGNED"
	MsgWorkCancelled = "WORK_CANCELLED"
	MsgWorkAccepted  = "WORK_ACCEPTED"
	MsgWorkProgress  = "WORK_PROGRESS"
	MsgWorkComplete  = "WORK_COMPLETE"
	MsgWorkFailed    = "WORK_FAILED"
	MsgWorkEscalate  = "WORK_ESCALATE"
)

type WorkAssignment struct {
	WorkID            string `json:"work_id"`
	OrchestratorRunID string `json:"orchestrator_run_id"`
	ParentWorkID      string `json:"parent_work_id,omitempty"`

	TargetStation string `json:"target_station,omitempty"`
	AgentID       string `json:"agent_id,omitempty"`
	AgentName     string `json:"agent_name,omitempty"`

	Task    string            `json:"task"`
	Context map[string]string `json:"context,omitempty"`

	AssignedAt time.Time     `json:"assigned_at"`
	Timeout    time.Duration `json:"timeout,omitempty"`
	Priority   int           `json:"priority,omitempty"`

	TraceID string `json:"trace_id,omitempty"`
	SpanID  string `json:"span_id,omitempty"`

	ReplySubject string `json:"reply_subject,omitempty"`
}

type WorkResponse struct {
	WorkID            string `json:"work_id"`
	OrchestratorRunID string `json:"orchestrator_run_id"`
	Type              string `json:"type"`

	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`

	ProgressPct int    `json:"progress_pct,omitempty"`
	ProgressMsg string `json:"progress_msg,omitempty"`

	EscalationReason  string            `json:"escalation_reason,omitempty"`
	EscalationContext map[string]string `json:"escalation_context,omitempty"`

	StationID  string    `json:"station_id"`
	LocalRunID int64     `json:"local_run_id,omitempty"`
	DurationMs float64   `json:"duration_ms,omitempty"`
	ToolCalls  int       `json:"tool_calls,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

type WorkStatus struct {
	WorkID   string        `json:"work_id"`
	Status   string        `json:"status"`
	Response *WorkResponse `json:"response,omitempty"`
}

func SubjectWorkAssign(stationID string) string {
	return "lattice.station." + stationID + ".work.assign"
}

func SubjectWorkResponse(workID string) string {
	return "lattice.work." + workID + ".response"
}
