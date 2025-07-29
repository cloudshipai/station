package repositories

import (
	"station/internal/db"
)

type Repositories struct {
	Environments   *EnvironmentRepo
	Users          *UserRepo
	MCPConfigs     *MCPConfigRepo
	MCPServers     *MCPServerRepo
	MCPTools       *MCPToolRepo
	ModelProviders *ModelProviderRepository
	Models         *ModelRepository
	Agents         *AgentRepo
	AgentTools     *AgentToolRepo
	AgentRuns      *AgentRunRepo
}

func New(database db.Database) *Repositories {
	conn := database.Conn()
	
	return &Repositories{
		Environments:   NewEnvironmentRepo(conn),
		Users:          NewUserRepo(conn),
		MCPConfigs:     NewMCPConfigRepo(conn),
		MCPServers:     NewMCPServerRepo(conn),
		MCPTools:       NewMCPToolRepo(conn),
		ModelProviders: NewModelProviderRepository(conn),
		Models:         NewModelRepository(conn),
		Agents:         NewAgentRepo(conn),
		AgentTools:     NewAgentToolRepo(conn),
		AgentRuns:      NewAgentRunRepo(conn),
	}
}