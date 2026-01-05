package lattice

import (
	"context"
	"fmt"
)

type AgentRouter struct {
	registry  *Registry
	stationID string
}

func NewAgentRouter(registry *Registry, stationID string) *AgentRouter {
	return &AgentRouter{
		registry:  registry,
		stationID: stationID,
	}
}

type AgentLocation struct {
	StationID   string
	StationName string
	AgentID     string
	AgentName   string
	IsLocal     bool
}

func (r *AgentRouter) FindAgentByName(ctx context.Context, agentName string) ([]AgentLocation, error) {
	stations, err := r.registry.ListStations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list stations: %w", err)
	}

	var locations []AgentLocation
	for _, station := range stations {
		if station.Status != StatusOnline {
			continue
		}

		for _, agent := range station.Agents {
			if agent.Name == agentName {
				locations = append(locations, AgentLocation{
					StationID:   station.StationID,
					StationName: station.StationName,
					AgentID:     agent.ID,
					AgentName:   agent.Name,
					IsLocal:     station.StationID == r.stationID,
				})
			}
		}
	}

	return locations, nil
}

func (r *AgentRouter) FindAgentByCapability(ctx context.Context, capability string) ([]AgentLocation, error) {
	stations, err := r.registry.ListStations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list stations: %w", err)
	}

	var locations []AgentLocation
	for _, station := range stations {
		if station.Status != StatusOnline {
			continue
		}

		for _, agent := range station.Agents {
			for _, cap := range agent.Capabilities {
				if cap == capability {
					locations = append(locations, AgentLocation{
						StationID:   station.StationID,
						StationName: station.StationName,
						AgentID:     agent.ID,
						AgentName:   agent.Name,
						IsLocal:     station.StationID == r.stationID,
					})
					break
				}
			}
		}
	}

	return locations, nil
}

func (r *AgentRouter) FindBestAgent(ctx context.Context, agentName, capability string) (*AgentLocation, error) {
	var locations []AgentLocation
	var err error

	if agentName != "" {
		locations, err = r.FindAgentByName(ctx, agentName)
	} else if capability != "" {
		locations, err = r.FindAgentByCapability(ctx, capability)
	} else {
		return nil, fmt.Errorf("either agent_name or capability required")
	}

	if err != nil {
		return nil, err
	}

	if len(locations) == 0 {
		return nil, nil
	}

	for _, loc := range locations {
		if loc.IsLocal {
			return &loc, nil
		}
	}

	return &locations[0], nil
}

func (r *AgentRouter) ListAllAgents(ctx context.Context) ([]AgentLocation, error) {
	stations, err := r.registry.ListStations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list stations: %w", err)
	}

	var allAgents []AgentLocation
	for _, station := range stations {
		if station.Status != StatusOnline {
			continue
		}

		for _, agent := range station.Agents {
			allAgents = append(allAgents, AgentLocation{
				StationID:   station.StationID,
				StationName: station.StationName,
				AgentID:     agent.ID,
				AgentName:   agent.Name,
				IsLocal:     station.StationID == r.stationID,
			})
		}
	}

	return allAgents, nil
}
