package load

import (
	"station/internal/services"
	"station/internal/theme"
)

// LoadMCPConfig configuration structure for load command
type LoadMCPConfig struct {
	Name        string                         `json:"name,omitempty"`
	Description string                         `json:"description,omitempty"`
	MCPServers  map[string]LoadMCPServerConfig `json:"mcpServers"`
	Templates   map[string]TemplateField       `json:"templates,omitempty"`
}

type LoadMCPServerConfig struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Type    string            `json:"type,omitempty"`
}

type TemplateField struct {
	Description string `json:"description"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Sensitive   bool   `json:"sensitive"`
	Default     string `json:"default,omitempty"`
	Help        string `json:"help,omitempty"`
}

// LoadHandler handles the "stn load" command
type LoadHandler struct {
	themeManager        *theme.ThemeManager
	placeholderAnalyzer *services.PlaceholderAnalyzer
}

func NewLoadHandler(themeManager *theme.ThemeManager) *LoadHandler {
	return &LoadHandler{
		themeManager:        themeManager,
		placeholderAnalyzer: nil, // Will be initialized when needed
	}
}

func NewLoadHandlerWithAI(themeManager *theme.ThemeManager, placeholderAnalyzer *services.PlaceholderAnalyzer) *LoadHandler {
	return &LoadHandler{
		themeManager:        themeManager,
		placeholderAnalyzer: placeholderAnalyzer,
	}
}
