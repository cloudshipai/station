package work

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/google/uuid"
)

type OrchestratorContext struct {
	RunID              string
	ParentRunID        string
	RootRunID          string
	OriginatingStation string
	Depth              int
	TraceID            string
	WorkID             string

	childCounter atomic.Int64
}

func NewRootContext(stationID, traceID string) *OrchestratorContext {
	rootRunID := uuid.NewString()
	return &OrchestratorContext{
		RunID:              rootRunID,
		ParentRunID:        "",
		RootRunID:          rootRunID,
		OriginatingStation: stationID,
		Depth:              0,
		TraceID:            traceID,
		WorkID:             "",
	}
}

func (c *OrchestratorContext) NewChildContext() *OrchestratorContext {
	childIndex := c.childCounter.Add(1)
	childRunID := fmt.Sprintf("%s-%d", c.RootRunID, childIndex)

	return &OrchestratorContext{
		RunID:              childRunID,
		ParentRunID:        c.RunID,
		RootRunID:          c.RootRunID,
		OriginatingStation: c.OriginatingStation,
		Depth:              c.Depth + 1,
		TraceID:            c.TraceID,
		WorkID:             "",
	}
}

func (c *OrchestratorContext) WithWorkID(workID string) *OrchestratorContext {
	return &OrchestratorContext{
		RunID:              c.RunID,
		ParentRunID:        c.ParentRunID,
		RootRunID:          c.RootRunID,
		OriginatingStation: c.OriginatingStation,
		Depth:              c.Depth,
		TraceID:            c.TraceID,
		WorkID:             workID,
	}
}

type orchestratorContextKey struct{}

func WithContext(ctx context.Context, oc *OrchestratorContext) context.Context {
	return context.WithValue(ctx, orchestratorContextKey{}, oc)
}

func FromContext(ctx context.Context) *OrchestratorContext {
	if oc, ok := ctx.Value(orchestratorContextKey{}).(*OrchestratorContext); ok {
		return oc
	}
	return nil
}

func (c *OrchestratorContext) ToWorkAssignment(agentName, task string, targetStation string, timeout int64) *WorkAssignment {
	child := c.NewChildContext()
	return &WorkAssignment{
		WorkID:            uuid.NewString(),
		OrchestratorRunID: child.RunID,
		ParentWorkID:      c.WorkID,
		TargetStation:     targetStation,
		AgentName:         agentName,
		Task:              task,
		TraceID:           c.TraceID,
	}
}
