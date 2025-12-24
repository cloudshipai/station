package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"station/internal/workflows"
)

var (
	ErrToolServerRequired = errors.New("server is required for tool step")
	ErrToolNameRequired   = errors.New("tool is required for tool step")
	ErrToolNotFound       = errors.New("tool not found")
	ErrToolCallFailed     = errors.New("tool call failed")
)

type ToolExecutorDeps interface {
	GetMCPServerByName(ctx context.Context, name string, environmentID int64) (MCPServerInfo, error)
	CallTool(ctx context.Context, serverID int64, toolName string, input map[string]interface{}) (ToolCallResult, error)
}

type MCPServerInfo struct {
	ID   int64
	Name string
}

type ToolCallResult struct {
	Content  string
	IsError  bool
	Duration int64
}

type ToolExecutor struct {
	deps ToolExecutorDeps
}

func NewToolExecutor(deps ToolExecutorDeps) *ToolExecutor {
	return &ToolExecutor{deps: deps}
}

func (e *ToolExecutor) SupportedTypes() []workflows.ExecutionStepType {
	return []workflows.ExecutionStepType{workflows.StepTypeTool}
}

func (e *ToolExecutor) Execute(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
	raw := step.Raw

	serverName := raw.Server
	if serverName == "" {
		if s, ok := raw.Input["server"].(string); ok {
			serverName = s
		}
	}
	if serverName == "" {
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr("server is required"),
		}, ErrToolServerRequired
	}

	toolName := raw.Tool
	if toolName == "" {
		if t, ok := raw.Input["tool"].(string); ok {
			toolName = t
		}
	}
	if toolName == "" {
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr("tool is required"),
		}, ErrToolNameRequired
	}

	var environmentID int64 = 1
	if envID, ok := runContext["_environmentID"]; ok {
		switch v := envID.(type) {
		case float64:
			environmentID = int64(v)
		case int64:
			environmentID = v
		case int:
			environmentID = int64(v)
		}
	}

	server, err := e.deps.GetMCPServerByName(ctx, serverName, environmentID)
	if err != nil {
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr(fmt.Sprintf("server '%s' not found: %v", serverName, err)),
		}, ErrToolNotFound
	}

	toolInput := make(map[string]interface{})
	if raw.Input != nil {
		for k, v := range raw.Input {
			if k != "server" && k != "tool" {
				toolInput[k] = v
			}
		}
	}

	for k, v := range runContext {
		if k[0] != '_' {
			if _, exists := toolInput[k]; !exists {
				toolInput[k] = v
			}
		}
	}

	result, err := e.deps.CallTool(ctx, server.ID, toolName, toolInput)
	if err != nil {
		return StepResult{
			Status: StepStatusFailed,
			Error:  strPtr(fmt.Sprintf("tool call failed: %v", err)),
		}, ErrToolCallFailed
	}

	if result.IsError {
		return StepResult{
			Status: StepStatusFailed,
			Output: map[string]interface{}{
				"server":   serverName,
				"tool":     toolName,
				"error":    result.Content,
				"duration": result.Duration,
			},
			Error:    strPtr(result.Content),
			NextStep: step.Next,
			End:      step.End,
		}, ErrToolCallFailed
	}

	var output interface{}
	if err := json.Unmarshal([]byte(result.Content), &output); err != nil {
		output = result.Content
	}

	return StepResult{
		Status: StepStatusCompleted,
		Output: map[string]interface{}{
			"server":   serverName,
			"tool":     toolName,
			"result":   output,
			"duration": result.Duration,
		},
		NextStep: step.Next,
		End:      step.End,
	}, nil
}
