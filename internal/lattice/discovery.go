package lattice

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"station/internal/db/queries"
)

type AgentCollector struct {
	db *sql.DB
}

func NewAgentCollector(db *sql.DB) *AgentCollector {
	return &AgentCollector{db: db}
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
