package tabs

// Message types for MCP configuration management

type MCPConfigSavedMsg struct {
	Config  MCPConfigDisplay
	Message string
	Error   string
}

// MCPConfigsLoadedMsg represents loaded configs
type MCPConfigsLoadedMsg struct {
	Configs []MCPConfigDisplay
	Error   error
}

// MCPConfigDeletedMsg represents a deleted config
type MCPConfigDeletedMsg struct {
	ConfigID int64
	Error    error
	Message  string
}

// Message types for version handling
type MCPConfigVersionsLoadedMsg struct {
	Versions []MCPConfigDisplay
	Error    error
}

type MCPVersionLoadedMsg struct {
	ConfigJSON string
	ConfigName string
	Error      error
}

// Message for tool discovery completion
type MCPToolDiscoveryCompletedMsg struct {
	ConfigName string
	Success    bool
	ToolCount  int
	Error      error
}