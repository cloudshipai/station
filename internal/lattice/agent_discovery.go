package lattice

import (
	"context"
	"fmt"
	"strings"
)

type DiscoveredAgent struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Location     string   `json:"location"`
	StationID    string   `json:"station_id"`
	StationName  string   `json:"station_name"`
	Capabilities []string `json:"capabilities"`
	InputSchema  string   `json:"input_schema,omitempty"`
	OutputSchema string   `json:"output_schema,omitempty"`
	IsLocal      bool     `json:"is_local"`
}

type AgentSchema struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	InputSchema  string   `json:"input_schema,omitempty"`
	OutputSchema string   `json:"output_schema,omitempty"`
	Examples     []string `json:"examples,omitempty"`
	Location     string   `json:"location"`
	StationID    string   `json:"station_id"`
	StationName  string   `json:"station_name"`
}

type AgentDiscovery struct {
	registry       *Registry
	localCollector *AgentCollector
	stationID      string
	stationName    string
}

func NewAgentDiscovery(registry *Registry, localCollector *AgentCollector, stationID, stationName string) *AgentDiscovery {
	return &AgentDiscovery{
		registry:       registry,
		localCollector: localCollector,
		stationID:      stationID,
		stationName:    stationName,
	}
}

type LocalAgentCollector interface {
	CollectAgents(ctx context.Context) ([]AgentInfo, error)
}

type agentDiscoveryWithMock struct {
	registry      *Registry
	mockCollector LocalAgentCollector
	stationID     string
	stationName   string
}

func NewAgentDiscoveryWithMock(registry *Registry, collector LocalAgentCollector, stationID, stationName string) *agentDiscoveryWithMock {
	return &agentDiscoveryWithMock{
		registry:      registry,
		mockCollector: collector,
		stationID:     stationID,
		stationName:   stationName,
	}
}

func (d *agentDiscoveryWithMock) ListAvailableAgents(ctx context.Context, capability string) ([]DiscoveredAgent, error) {
	var agents []DiscoveredAgent

	if d.mockCollector != nil {
		localAgents, err := d.mockCollector.CollectAgents(ctx)
		if err == nil {
			for _, agent := range localAgents {
				if capability != "" && !hasCapability(agent.Capabilities, capability) {
					continue
				}
				agents = append(agents, DiscoveredAgent{
					Name:         agent.Name,
					Description:  agent.Description,
					Location:     "local",
					StationID:    d.stationID,
					StationName:  d.stationName,
					Capabilities: agent.Capabilities,
					InputSchema:  agent.InputSchema,
					OutputSchema: agent.OutputSchema,
					IsLocal:      true,
				})
			}
		}
	}

	if d.registry != nil {
		stations, err := d.registry.ListStations(ctx)
		if err == nil {
			for _, station := range stations {
				if station.Status != StatusOnline {
					continue
				}
				if station.StationID == d.stationID {
					continue
				}

				for _, agent := range station.Agents {
					if capability != "" && !hasCapability(agent.Capabilities, capability) {
						continue
					}
					agents = append(agents, DiscoveredAgent{
						Name:         agent.Name,
						Description:  agent.Description,
						Location:     station.StationName,
						StationID:    station.StationID,
						StationName:  station.StationName,
						Capabilities: agent.Capabilities,
						InputSchema:  agent.InputSchema,
						OutputSchema: agent.OutputSchema,
						IsLocal:      false,
					})
				}
			}
		}
	}

	return agents, nil
}

func (d *agentDiscoveryWithMock) GetAgentSchema(ctx context.Context, agentName string) (*AgentSchema, error) {
	if d.mockCollector != nil {
		localAgents, err := d.mockCollector.CollectAgents(ctx)
		if err == nil {
			for _, agent := range localAgents {
				if agent.Name == agentName {
					return &AgentSchema{
						Name:         agent.Name,
						Description:  agent.Description,
						InputSchema:  agent.InputSchema,
						OutputSchema: agent.OutputSchema,
						Examples:     agent.Examples,
						Location:     "local",
						StationID:    d.stationID,
						StationName:  d.stationName,
					}, nil
				}
			}
		}
	}

	if d.registry != nil {
		stations, err := d.registry.ListStations(ctx)
		if err == nil {
			for _, station := range stations {
				if station.Status != StatusOnline {
					continue
				}

				for _, agent := range station.Agents {
					if agent.Name == agentName {
						return &AgentSchema{
							Name:         agent.Name,
							Description:  agent.Description,
							InputSchema:  agent.InputSchema,
							OutputSchema: agent.OutputSchema,
							Examples:     agent.Examples,
							Location:     station.StationName,
							StationID:    station.StationID,
							StationName:  station.StationName,
						}, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("agent '%s' not found", agentName)
}

func (d *agentDiscoveryWithMock) BuildAssignWorkDescription(ctx context.Context) string {
	var sb strings.Builder
	sb.WriteString("Assign work to an agent. Returns immediately with work_id.\n\n")
	sb.WriteString("Available agents:\n")

	agents, err := d.ListAvailableAgents(ctx, "")
	if err != nil || len(agents) == 0 {
		sb.WriteString("  (no agents available)\n")
	} else {
		for _, agent := range agents {
			location := "local"
			if !agent.IsLocal {
				location = "@" + agent.StationName
			}
			sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", agent.Name, location, truncateDescription(agent.Description, 60)))
		}
	}

	sb.WriteString("\nUse list_available_agents for full details and schemas.\n")
	sb.WriteString("Use await_work(work_id) to get results after assignment.")

	return sb.String()
}

func (d *AgentDiscovery) ListAvailableAgents(ctx context.Context, capability string) ([]DiscoveredAgent, error) {
	var agents []DiscoveredAgent

	if d.localCollector != nil {
		localAgents, err := d.localCollector.CollectAgents(ctx)
		if err == nil {
			for _, agent := range localAgents {
				if capability != "" && !hasCapability(agent.Capabilities, capability) {
					continue
				}
				agents = append(agents, DiscoveredAgent{
					Name:         agent.Name,
					Description:  agent.Description,
					Location:     "local",
					StationID:    d.stationID,
					StationName:  d.stationName,
					Capabilities: agent.Capabilities,
					InputSchema:  agent.InputSchema,
					OutputSchema: agent.OutputSchema,
					IsLocal:      true,
				})
			}
		}
	}

	if d.registry != nil {
		stations, err := d.registry.ListStations(ctx)
		if err == nil {
			for _, station := range stations {
				if station.Status != StatusOnline {
					continue
				}
				if station.StationID == d.stationID {
					continue
				}

				for _, agent := range station.Agents {
					if capability != "" && !hasCapability(agent.Capabilities, capability) {
						continue
					}
					agents = append(agents, DiscoveredAgent{
						Name:         agent.Name,
						Description:  agent.Description,
						Location:     station.StationName,
						StationID:    station.StationID,
						StationName:  station.StationName,
						Capabilities: agent.Capabilities,
						InputSchema:  agent.InputSchema,
						OutputSchema: agent.OutputSchema,
						IsLocal:      false,
					})
				}
			}
		}
	}

	return agents, nil
}

func (d *AgentDiscovery) GetAgentSchema(ctx context.Context, agentName string) (*AgentSchema, error) {
	if d.localCollector != nil {
		localAgents, err := d.localCollector.CollectAgents(ctx)
		if err == nil {
			for _, agent := range localAgents {
				if agent.Name == agentName {
					return &AgentSchema{
						Name:         agent.Name,
						Description:  agent.Description,
						InputSchema:  agent.InputSchema,
						OutputSchema: agent.OutputSchema,
						Examples:     agent.Examples,
						Location:     "local",
						StationID:    d.stationID,
						StationName:  d.stationName,
					}, nil
				}
			}
		}
	}

	if d.registry != nil {
		stations, err := d.registry.ListStations(ctx)
		if err == nil {
			for _, station := range stations {
				if station.Status != StatusOnline {
					continue
				}

				for _, agent := range station.Agents {
					if agent.Name == agentName {
						return &AgentSchema{
							Name:         agent.Name,
							Description:  agent.Description,
							InputSchema:  agent.InputSchema,
							OutputSchema: agent.OutputSchema,
							Examples:     agent.Examples,
							Location:     station.StationName,
							StationID:    station.StationID,
							StationName:  station.StationName,
						}, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("agent '%s' not found", agentName)
}

func (d *AgentDiscovery) BuildAssignWorkDescription(ctx context.Context) string {
	var sb strings.Builder
	sb.WriteString("Assign work to an agent. Returns immediately with work_id.\n\n")
	sb.WriteString("Available agents:\n")

	agents, err := d.ListAvailableAgents(ctx, "")
	if err != nil || len(agents) == 0 {
		sb.WriteString("  (no agents available)\n")
	} else {
		for _, agent := range agents {
			location := "local"
			if !agent.IsLocal {
				location = "@" + agent.StationName
			}
			sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", agent.Name, location, truncateDescription(agent.Description, 60)))
		}
	}

	sb.WriteString("\nUse list_available_agents for full details and schemas.\n")
	sb.WriteString("Use await_work(work_id) to get results after assignment.")

	return sb.String()
}

func hasCapability(capabilities []string, target string) bool {
	target = strings.ToLower(target)
	for _, cap := range capabilities {
		if strings.ToLower(cap) == target || strings.Contains(strings.ToLower(cap), target) {
			return true
		}
	}
	return false
}

func truncateDescription(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
