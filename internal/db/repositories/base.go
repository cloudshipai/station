package repositories

import (
	"database/sql"
	"station/internal/db"
)

type Repositories struct {
	Environments      *EnvironmentRepo
	Users             *UserRepo
	MCPConfigs        *MCPConfigRepo
	FileMCPConfigs    *FileMCPConfigRepo
	MCPServers        *MCPServerRepo
	MCPTools          *MCPToolRepo
	ModelProviders    *ModelProviderRepository
	Models            *ModelRepository
	Agents            *AgentRepo
	AgentTools        *AgentToolRepo
	AgentRuns         *AgentRunRepo
	Settings          *SettingsRepo
	Webhooks          *WebhooksRepo
	WebhookDeliveries *WebhookDeliveriesRepo
	db                db.Database // Store reference to database for transactions
}

func New(database db.Database) *Repositories {
	conn := database.Conn()
	
	return &Repositories{
		Environments:      NewEnvironmentRepo(conn),
		Users:             NewUserRepo(conn),
		MCPConfigs:        NewMCPConfigRepo(conn),
		FileMCPConfigs:    NewFileMCPConfigRepo(conn),
		MCPServers:        NewMCPServerRepo(conn),
		MCPTools:          NewMCPToolRepo(conn),
		ModelProviders:    NewModelProviderRepository(conn),
		Models:            NewModelRepository(conn),
		Agents:            NewAgentRepo(conn),
		AgentTools:        NewAgentToolRepo(conn),
		AgentRuns:         NewAgentRunRepo(conn),
		Settings:          NewSettingsRepo(conn),
		Webhooks:          NewWebhooksRepo(conn),
		WebhookDeliveries: NewWebhookDeliveriesRepo(conn),
		db:                database,
	}
}

// BeginTx starts a database transaction
func (r *Repositories) BeginTx() (*sql.Tx, error) {
	return r.db.Conn().Begin()
}