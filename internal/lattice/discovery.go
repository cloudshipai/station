package lattice

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"station/internal/db/queries"
)

type ManifestCollector struct {
	db *sql.DB
}

func NewManifestCollector(db *sql.DB) *ManifestCollector {
	return &ManifestCollector{db: db}
}

type AgentCollector = ManifestCollector

func NewAgentCollector(db *sql.DB) *AgentCollector {
	return NewManifestCollector(db)
}

func (c *AgentCollector) CollectAgents(ctx context.Context) ([]AgentInfo, error) {
	if c.db == nil {
		return nil, nil
	}

	q := queries.New(c.db)
	agents, err := q.ListAgents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	var infos []AgentInfo
	for _, agent := range agents {
		infos = append(infos, AgentInfo{
			ID:           strconv.FormatInt(agent.ID, 10),
			Name:         agent.Name,
			Description:  agent.Description,
			Capabilities: extractCapabilities(agent),
		})
	}

	return infos, nil
}

func (c *AgentCollector) GetAgentByID(ctx context.Context, agentID string) (*queries.Agent, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	id, err := strconv.ParseInt(agentID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID: %w", err)
	}

	q := queries.New(c.db)
	agent, err := q.GetAgent(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	return &agent, nil
}

func (c *AgentCollector) GetAgentByName(ctx context.Context, name string) (*queries.Agent, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	q := queries.New(c.db)
	agent, err := q.GetAgentByNameGlobal(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	return &agent, nil
}

func extractCapabilities(agent queries.Agent) []string {
	var caps []string

	if agent.App.Valid && agent.App.String != "" {
		caps = append(caps, agent.App.String)
	}

	if agent.AppSubtype.Valid && agent.AppSubtype.String != "" {
		caps = append(caps, agent.AppSubtype.String)
	}

	return caps
}

func (c *ManifestCollector) CollectWorkflows(ctx context.Context) ([]WorkflowInfo, error) {
	if c.db == nil {
		return nil, nil
	}

	q := queries.New(c.db)
	workflows, err := q.ListLatestWorkflows(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}

	var infos []WorkflowInfo
	for _, wf := range workflows {
		if wf.Status != "active" && wf.Status != "enabled" {
			continue
		}

		desc := ""
		if wf.Description.Valid {
			desc = wf.Description.String
		}

		infos = append(infos, WorkflowInfo{
			ID:          wf.WorkflowID,
			Name:        wf.Name,
			Description: desc,
		})
	}

	return infos, nil
}

func (c *ManifestCollector) GetWorkflowByID(ctx context.Context, workflowID string) (*queries.Workflow, error) {
	if c.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	q := queries.New(c.db)
	wf, err := q.GetLatestWorkflow(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %w", err)
	}

	return &wf, nil
}

func (c *ManifestCollector) CollectFullManifest(ctx context.Context, stationID, stationName string) (*StationManifest, error) {
	agents, err := c.CollectAgents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect agents: %w", err)
	}

	workflows, err := c.CollectWorkflows(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect workflows: %w", err)
	}

	return &StationManifest{
		StationID:   stationID,
		StationName: stationName,
		Agents:      agents,
		Workflows:   workflows,
		Status:      StatusOnline,
	}, nil
}
