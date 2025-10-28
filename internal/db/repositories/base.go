package repositories

import (
	"database/sql"
	"station/internal/db"
)

type Repositories struct {
	Environments   *EnvironmentRepo
	Users          *UserRepo
	FileMCPConfigs *FileMCPConfigRepo
	MCPServers     *MCPServerRepo
	MCPTools       *MCPToolRepo
	ModelProviders *ModelProviderRepository
	Models         *ModelRepository
	Agents         *AgentRepo
	AgentTools     *AgentToolRepo
	AgentRuns      *AgentRunRepo
	Settings       *SettingsRepo
	db             db.Database // Store reference to database for transactions
}

func New(database db.Database) *Repositories {
	conn := database.Conn()

	return &Repositories{
		Environments:   NewEnvironmentRepo(conn),
		Users:          NewUserRepo(conn),
		FileMCPConfigs: NewFileMCPConfigRepo(conn),
		MCPServers:     NewMCPServerRepo(conn),
		MCPTools:       NewMCPToolRepo(conn),
		ModelProviders: NewModelProviderRepository(conn),
		Models:         NewModelRepository(conn),
		Agents:         NewAgentRepo(conn),
		AgentTools:     NewAgentToolRepo(conn),
		AgentRuns:      NewAgentRunRepo(conn),
		Settings:       NewSettingsRepo(conn),
		db:             database,
	}
}

// BeginTx starts a database transaction
func (r *Repositories) BeginTx() (*sql.Tx, error) {
	return r.db.Conn().Begin()
}
