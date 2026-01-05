package services

import (
	"context"
	"fmt"
	"time"

	"station/internal/lattice/work"
	"station/internal/logging"

	"github.com/firebase/genkit/go/ai"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type LatticeAgentInfo struct {
	ID           string
	Name         string
	Description  string
	Capabilities []string
}

type LatticeStationInfo struct {
	StationID   string
	StationName string
	Agents      []LatticeAgentInfo
	Tags        []string
}

type LatticeRegistry interface {
	ListStationsInfo(ctx context.Context) ([]LatticeStationInfo, error)
}

type WorkToolFactory struct {
	dispatcher *work.Dispatcher
	registry   LatticeRegistry
	stationID  string
	tracer     trace.Tracer
}

func NewWorkToolFactory(dispatcher *work.Dispatcher, registry LatticeRegistry, stationID string) *WorkToolFactory {
	if dispatcher == nil {
		return nil
	}

	return &WorkToolFactory{
		dispatcher: dispatcher,
		registry:   registry,
		stationID:  stationID,
		tracer:     otel.Tracer("station.work_tools"),
	}
}

func (f *WorkToolFactory) IsEnabled() bool {
	return f != nil && f.dispatcher != nil
}

func (f *WorkToolFactory) ShouldAddTools(latticeEnabled bool) bool {
	return f.IsEnabled() && latticeEnabled
}

type AssignWorkRequest struct {
	AgentName     string            `json:"agent_name"`
	AgentID       string            `json:"agent_id,omitempty"`
	TargetStation string            `json:"target_station,omitempty"`
	Task          string            `json:"task"`
	Context       map[string]string `json:"context,omitempty"`
	TimeoutSecs   int               `json:"timeout_secs,omitempty"`
}

type AssignWorkResponse struct {
	Success bool   `json:"success"`
	WorkID  string `json:"work_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (f *WorkToolFactory) CreateAssignWorkTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_name": map[string]any{
				"type":        "string",
				"description": "Name of the agent to assign work to (required if agent_id not provided)",
			},
			"agent_id": map[string]any{
				"type":        "string",
				"description": "ID of the agent to assign work to (alternative to agent_name)",
			},
			"target_station": map[string]any{
				"type":        "string",
				"description": "Station ID to route work to (optional - defaults to local station or auto-discovered)",
			},
			"task": map[string]any{
				"type":        "string",
				"description": "The task description to send to the agent (required)",
			},
			"context": map[string]any{
				"type":        "object",
				"description": "Additional context key-value pairs to pass to the agent",
				"additionalProperties": map[string]any{
					"type": "string",
				},
			},
			"timeout_secs": map[string]any{
				"type":        "integer",
				"description": "Timeout in seconds for the work (default: 300)",
				"default":     300,
			},
		},
		"required": []string{"task"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		ctx := toolCtx.Context

		ctx, span := f.tracer.Start(ctx, "work_tool.assign_work",
			trace.WithAttributes(
				attribute.String("tool.name", "assign_work"),
			),
		)
		defer span.End()

		inputMap, ok := input.(map[string]any)
		if !ok {
			err := fmt.Errorf("assign_work: expected map[string]any input, got %T", input)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}

		req := f.parseAssignWorkRequest(inputMap)

		if req.AgentName == "" && req.AgentID == "" {
			err := fmt.Errorf("assign_work: either agent_name or agent_id is required")
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return &AssignWorkResponse{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		if req.Task == "" {
			err := fmt.Errorf("assign_work: task is required")
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return &AssignWorkResponse{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		span.SetAttributes(
			attribute.String("work.agent_name", req.AgentName),
			attribute.String("work.agent_id", req.AgentID),
			attribute.String("work.target_station", req.TargetStation),
			attribute.Int("work.timeout_secs", req.TimeoutSecs),
		)

		timeout := time.Duration(req.TimeoutSecs) * time.Second
		if timeout == 0 {
			timeout = 5 * time.Minute
		}

		assignment := &work.WorkAssignment{
			AgentName:     req.AgentName,
			AgentID:       req.AgentID,
			TargetStation: req.TargetStation,
			Task:          req.Task,
			Context:       req.Context,
			Timeout:       timeout,
		}

		if f.registry != nil && req.TargetStation == "" && req.AgentName != "" {
			if stationID := f.discoverStationForAgent(ctx, req.AgentName); stationID != "" {
				assignment.TargetStation = stationID
				span.SetAttributes(attribute.String("work.discovered_station", stationID))
			}
		}

		workID, err := f.dispatcher.AssignWork(ctx, assignment)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return &AssignWorkResponse{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		span.SetAttributes(attribute.String("work.work_id", workID))
		span.SetStatus(codes.Ok, "work assigned")

		logging.Info("[work_tool] Assigned work %s to agent %s (station: %s)",
			workID, req.AgentName, assignment.TargetStation)

		return &AssignWorkResponse{
			Success: true,
			WorkID:  workID,
		}, nil
	}

	return ai.NewToolWithInputSchema(
		"assign_work",
		"Assign work to another agent in the lattice. Returns a work_id immediately (non-blocking). Use await_work to get results.",
		schema,
		toolFunc,
	)
}

type AwaitWorkRequest struct {
	WorkID      string `json:"work_id"`
	TimeoutSecs int    `json:"timeout_secs,omitempty"`
}

type AwaitWorkResponse struct {
	Success    bool    `json:"success"`
	Status     string  `json:"status"`
	Result     string  `json:"result,omitempty"`
	Error      string  `json:"error,omitempty"`
	DurationMs float64 `json:"duration_ms,omitempty"`
	StationID  string  `json:"station_id,omitempty"`
}

func (f *WorkToolFactory) CreateAwaitWorkTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"work_id": map[string]any{
				"type":        "string",
				"description": "The work ID returned from assign_work (required)",
			},
			"timeout_secs": map[string]any{
				"type":        "integer",
				"description": "Additional timeout in seconds to wait (default: use assignment timeout)",
			},
		},
		"required": []string{"work_id"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		ctx := toolCtx.Context

		ctx, span := f.tracer.Start(ctx, "work_tool.await_work",
			trace.WithAttributes(
				attribute.String("tool.name", "await_work"),
			),
		)
		defer span.End()

		inputMap, ok := input.(map[string]any)
		if !ok {
			err := fmt.Errorf("await_work: expected map[string]any input, got %T", input)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}

		workID, _ := inputMap["work_id"].(string)
		if workID == "" {
			err := fmt.Errorf("await_work: work_id is required")
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return &AwaitWorkResponse{
				Success: false,
				Status:  "error",
				Error:   err.Error(),
			}, nil
		}

		span.SetAttributes(attribute.String("work.work_id", workID))

		if timeoutSecs, ok := inputMap["timeout_secs"].(float64); ok && timeoutSecs > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
			defer cancel()
		}

		response, err := f.dispatcher.AwaitWork(ctx, workID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return &AwaitWorkResponse{
				Success: false,
				Status:  "error",
				Error:   err.Error(),
			}, nil
		}

		span.SetAttributes(
			attribute.String("work.status", response.Type),
			attribute.Float64("work.duration_ms", response.DurationMs),
			attribute.String("work.station_id", response.StationID),
		)

		result := &AwaitWorkResponse{
			Success:    response.Type == work.MsgWorkComplete,
			Status:     response.Type,
			Result:     response.Result,
			Error:      response.Error,
			DurationMs: response.DurationMs,
			StationID:  response.StationID,
		}

		if result.Success {
			span.SetStatus(codes.Ok, "work completed")
		} else {
			span.SetStatus(codes.Error, response.Error)
		}

		logging.Info("[work_tool] Work %s completed: status=%s, duration=%.0fms",
			workID, response.Type, response.DurationMs)

		return result, nil
	}

	return ai.NewToolWithInputSchema(
		"await_work",
		"Wait for previously assigned work to complete. Blocks until the work finishes or times out.",
		schema,
		toolFunc,
	)
}

type CheckWorkRequest struct {
	WorkID string `json:"work_id"`
}

type CheckWorkResponse struct {
	Success    bool    `json:"success"`
	Status     string  `json:"status"`
	IsComplete bool    `json:"is_complete"`
	Result     string  `json:"result,omitempty"`
	Error      string  `json:"error,omitempty"`
	DurationMs float64 `json:"duration_ms,omitempty"`
}

func (f *WorkToolFactory) CreateCheckWorkTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"work_id": map[string]any{
				"type":        "string",
				"description": "The work ID to check status for (required)",
			},
		},
		"required": []string{"work_id"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		ctx := toolCtx.Context

		_, span := f.tracer.Start(ctx, "work_tool.check_work",
			trace.WithAttributes(
				attribute.String("tool.name", "check_work"),
			),
		)
		defer span.End()

		inputMap, ok := input.(map[string]any)
		if !ok {
			err := fmt.Errorf("check_work: expected map[string]any input, got %T", input)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}

		workID, _ := inputMap["work_id"].(string)
		if workID == "" {
			err := fmt.Errorf("check_work: work_id is required")
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return &CheckWorkResponse{
				Success: false,
				Status:  "error",
				Error:   err.Error(),
			}, nil
		}

		span.SetAttributes(attribute.String("work.work_id", workID))

		status, err := f.dispatcher.CheckWork(workID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return &CheckWorkResponse{
				Success: false,
				Status:  "error",
				Error:   err.Error(),
			}, nil
		}

		span.SetAttributes(attribute.String("work.status", status.Status))

		isComplete := status.Status == work.MsgWorkComplete ||
			status.Status == work.MsgWorkFailed ||
			status.Status == work.MsgWorkEscalate

		result := &CheckWorkResponse{
			Success:    true,
			Status:     status.Status,
			IsComplete: isComplete,
		}

		if status.Response != nil {
			result.Result = status.Response.Result
			result.Error = status.Response.Error
			result.DurationMs = status.Response.DurationMs
		}

		span.SetStatus(codes.Ok, "status checked")

		return result, nil
	}

	return ai.NewToolWithInputSchema(
		"check_work",
		"Check the status of previously assigned work without blocking. Returns current status (PENDING, ACCEPTED, COMPLETE, FAILED).",
		schema,
		toolFunc,
	)
}

type ListAgentsResponse struct {
	Success bool                `json:"success"`
	Agents  []WorkToolAgentInfo `json:"agents"`
	Error   string              `json:"error,omitempty"`
}

type WorkToolAgentInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	StationID   string   `json:"station_id"`
	StationName string   `json:"station_name,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func (f *WorkToolFactory) CreateListAgentsTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filter": map[string]any{
				"type":        "string",
				"description": "Optional filter to match agent names (case-insensitive substring match)",
			},
			"include_local": map[string]any{
				"type":        "boolean",
				"description": "Include agents from this station (default: true)",
				"default":     true,
			},
		},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		ctx := toolCtx.Context

		_, span := f.tracer.Start(ctx, "work_tool.list_agents",
			trace.WithAttributes(
				attribute.String("tool.name", "list_agents"),
			),
		)
		defer span.End()

		inputMap, ok := input.(map[string]any)
		if !ok {
			inputMap = make(map[string]any)
		}

		filter, _ := inputMap["filter"].(string)
		includeLocal := true
		if v, ok := inputMap["include_local"].(bool); ok {
			includeLocal = v
		}

		span.SetAttributes(
			attribute.String("filter", filter),
			attribute.Bool("include_local", includeLocal),
		)

		var agents []WorkToolAgentInfo

		if f.registry != nil {
			stations, err := f.registry.ListStationsInfo(ctx)
			if err != nil {
				span.RecordError(err)
				logging.Debug("[work_tool] list_agents: failed to list stations: %v", err)
			} else {
				for _, station := range stations {
					if !includeLocal && station.StationID == f.stationID {
						continue
					}

					for _, agent := range station.Agents {
						if filter != "" && !containsIgnoreCase(agent.Name, filter) {
							continue
						}

						agents = append(agents, WorkToolAgentInfo{
							Name:        agent.Name,
							Description: agent.Description,
							StationID:   station.StationID,
							StationName: station.StationName,
							Tags:        station.Tags,
						})
					}
				}
			}
		} else {
			span.SetAttributes(attribute.Bool("registry_available", false))
			logging.Debug("[work_tool] list_agents: registry not available")
		}

		span.SetAttributes(attribute.Int("agents.count", len(agents)))
		span.SetStatus(codes.Ok, "agents listed")

		return &ListAgentsResponse{
			Success: true,
			Agents:  agents,
		}, nil
	}

	return ai.NewToolWithInputSchema(
		"list_agents",
		"List available agents across the lattice mesh network. Use this to discover what agents can be delegated work to.",
		schema,
		toolFunc,
	)
}

func (f *WorkToolFactory) GetWorkTools() []ai.Tool {
	if !f.IsEnabled() {
		return nil
	}
	return []ai.Tool{
		f.CreateAssignWorkTool(),
		f.CreateAwaitWorkTool(),
		f.CreateCheckWorkTool(),
		f.CreateListAgentsTool(),
	}
}

func (f *WorkToolFactory) parseAssignWorkRequest(input map[string]any) AssignWorkRequest {
	req := AssignWorkRequest{
		TimeoutSecs: 300,
	}

	if v, ok := input["agent_name"].(string); ok {
		req.AgentName = v
	}
	if v, ok := input["agent_id"].(string); ok {
		req.AgentID = v
	}
	if v, ok := input["target_station"].(string); ok {
		req.TargetStation = v
	}
	if v, ok := input["task"].(string); ok {
		req.Task = v
	}
	if v, ok := input["timeout_secs"].(float64); ok {
		req.TimeoutSecs = int(v)
	}
	if v, ok := input["context"].(map[string]any); ok {
		req.Context = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				req.Context[k] = s
			}
		}
	}

	return req
}

func (f *WorkToolFactory) discoverStationForAgent(ctx context.Context, agentName string) string {
	if f.registry == nil {
		return ""
	}

	stations, err := f.registry.ListStationsInfo(ctx)
	if err != nil {
		return ""
	}

	for _, station := range stations {
		for _, agent := range station.Agents {
			if agent.Name == agentName {
				return station.StationID
			}
		}
	}

	return ""
}

func containsIgnoreCase(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)

	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
