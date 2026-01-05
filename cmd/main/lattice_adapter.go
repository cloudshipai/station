package main

import (
	"context"

	"station/internal/lattice"
	"station/internal/services"
)

type latticeRegistryAdapter struct {
	registry *lattice.Registry
}

func newLatticeRegistryAdapter(registry *lattice.Registry) services.LatticeRegistry {
	if registry == nil {
		return nil
	}
	return &latticeRegistryAdapter{registry: registry}
}

func (a *latticeRegistryAdapter) ListStationsInfo(ctx context.Context) ([]services.LatticeStationInfo, error) {
	stations, err := a.registry.ListStations(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]services.LatticeStationInfo, 0, len(stations))
	for _, station := range stations {
		agents := make([]services.LatticeAgentInfo, 0, len(station.Agents))
		for _, agent := range station.Agents {
			agents = append(agents, services.LatticeAgentInfo{
				ID:           agent.ID,
				Name:         agent.Name,
				Description:  agent.Description,
				Capabilities: agent.Capabilities,
			})
		}

		result = append(result, services.LatticeStationInfo{
			StationID:   station.StationID,
			StationName: station.StationName,
			Agents:      agents,
		})
	}

	return result, nil
}
