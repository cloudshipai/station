package tabs

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/tui/components"
	"station/internal/tui/styles"
	"station/pkg/crypto"
	"station/pkg/models"
)

// MCPModel represents the MCP servers configuration tab
type MCPModel struct {
	BaseTabModel
	
	// UI components
	configEditor textarea.Model
	nameInput    textinput.Model
	environmentList list.Model
	
	// Data
	envs         []*models.Environment
	repos        *repositories.Repositories
	database     db.Database  // Need this to create services
	
	// Services - shared instances to avoid key ID mismatches
	mcpConfigSvc    *services.MCPConfigService
	toolDiscoverySvc *services.ToolDiscoveryService
	
	// State
	mode         MCPMode
	configs      []MCPConfigDisplay
	selectedIdx  int
	showEditor   bool
	selectedEnvID int64
	focusedField MCPField
}

type MCPField int

const (
	MCPFieldName MCPField = iota
	MCPFieldEnvironment
	MCPFieldConfig
)

type MCPMode int

const (
	MCPModeList MCPMode = iota
	MCPModeEdit
)

type MCPConfigDisplay struct {
	ID          int64
	Name        string
	Version     int64
	Updated     string
	Size        string
	ConfigJSON  string // Store the actual JSON content
}

type MCPConfigSavedMsg struct {
	Config  MCPConfigDisplay
	Message string
	Error   string
}

// NewMCPModel creates a new MCP model
func NewMCPModel(database db.Database) *MCPModel {
	repos := repositories.New(database)
	
	// Create textarea for config editing - scrollable
	ta := textarea.New()
	ta.Placeholder = "Paste your MCP server configuration here (JSON format)..."
	ta.SetWidth(60) // Will be adjusted dynamically in renderEditor
	ta.SetHeight(5)  // Will be adjusted dynamically in renderEditor
	
	// Create text input for name
	ti := textinput.New()
	ti.Placeholder = "Configuration name"
	ti.Width = 30
	
	// Create environment list
	envList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 4)
	envList.SetShowHelp(false)
	envList.SetShowStatusBar(false)
	envList.SetShowTitle(false)
	envList.SetFilteringEnabled(false)
	
	// Initialize shared services to avoid key ID mismatches
	keyManager, err := crypto.NewKeyManagerFromEnv()
	var mcpConfigSvc *services.MCPConfigService
	var toolDiscoverySvc *services.ToolDiscoveryService
	
	if err != nil {
		log.Printf("WARNING: Failed to create key manager: %v", err)
		// Create services without encryption support
		mcpConfigSvc = services.NewMCPConfigService(repos, nil)
		toolDiscoverySvc = services.NewToolDiscoveryService(repos, mcpConfigSvc)
	} else {
		mcpConfigSvc = services.NewMCPConfigService(repos, keyManager)
		toolDiscoverySvc = services.NewToolDiscoveryService(repos, mcpConfigSvc)
	}
	
	m := &MCPModel{
		BaseTabModel:     NewBaseTabModel(database, "MCP Servers"),
		configEditor:     ta,
		nameInput:        ti,
		environmentList:  envList,
		repos:            repos,
		database:         database,
		mcpConfigSvc:     mcpConfigSvc,
		toolDiscoverySvc: toolDiscoverySvc,
		mode:             MCPModeList,
		configs:          []MCPConfigDisplay{},
		selectedIdx:      0,
		showEditor:       false,
		selectedEnvID:    1, // Default to first environment
		focusedField:     MCPFieldName,
	}
	
	// Load environments
	m.loadEnvironments()
	
	return m
}

// Init initializes the MCP tab
func (m MCPModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadConfigs(),
		m.configEditor.Cursor.BlinkCmd(),
	)
}

// Update handles messages
func (m *MCPModel) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd
	
	// Debug: Log important incoming messages (filter out spam)
	switch v := msg.(type) {
	case MCPConfigsLoadedMsg:
		log.Printf("DEBUG: Update received MCPConfigsLoadedMsg with %d configs", len(v.Configs))
	case MCPConfigSavedMsg:
		log.Printf("DEBUG: Update received MCPConfigSavedMsg")
	case MCPConfigDeletedMsg:
		log.Printf("DEBUG: Update received MCPConfigDeletedMsg for config %d", v.ConfigID)
	case tea.KeyMsg:
		// Don't log key messages to avoid spam
	default:
		// Filter out common spam messages
		msgType := fmt.Sprintf("%T", msg)
		if !strings.Contains(msgType, "spinner.TickMsg") && 
		   !strings.Contains(msgType, "cursor.BlinkMsg") {
			log.Printf("DEBUG: Update received message type: %T", msg)
		}
	}
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		m.configEditor.SetWidth(msg.Width - 10)
		
	case tea.KeyMsg:
		log.Printf("DEBUG: MCP received key: '%s', mode: %d, focusedField: %d", msg.String(), m.mode, m.focusedField)
		switch msg.String() {
		case "n":
			if m.mode == MCPModeList {
				m.mode = MCPModeEdit
				m.showEditor = true
				m.nameInput.Focus()
				m.nameInput.SetValue("")
				m.configEditor.SetValue("")
				return m, nil
			}
		case "enter":
			log.Printf("DEBUG: Enter key pressed - mode: %d, focusedField: %d", m.mode, m.focusedField)
			if m.mode == MCPModeList && len(m.configs) > 0 {
				// Edit selected config
				m.mode = MCPModeEdit
				m.showEditor = true
				m.nameInput.Focus()
				// Pre-populate with selected config data
				selected := m.configs[m.selectedIdx]
				m.nameInput.SetValue(selected.Name)
				
				// Load the actual saved configuration JSON
				if selected.ConfigJSON != "" {
					m.configEditor.SetValue(selected.ConfigJSON)
				} else {
					// Fallback for configs that don't have JSON stored
					placeholderConfig := `{
  "mcpServers": {
    "` + selected.Name + `": {
      "command": "your-command-here",
      "args": ["arg1", "arg2"]
    }
  }
}`
					m.configEditor.SetValue(placeholderConfig)
				}
				return m, nil
			} else if m.mode == MCPModeEdit && m.focusedField != MCPFieldConfig {
				// Save when Enter is pressed in Name or Environment field
				log.Printf("DEBUG: Calling saveConfig() directly")
				
				// Do the save logic directly instead of using a command
				configText := m.configEditor.Value()
				log.Printf("DEBUG: configText length: %d", len(configText))
				
				if configText == "" {
					log.Printf("DEBUG: Config text is empty")
					return m, tea.Printf("Error: Configuration cannot be empty")
				}
				
				// Basic JSON validation
				var rawConfig map[string]interface{}
				if err := json.Unmarshal([]byte(configText), &rawConfig); err != nil {
					log.Printf("DEBUG: JSON validation failed: %v", err)
					// Set a persistent error state and show detailed feedback
					m.SetError(fmt.Sprintf("Invalid JSON: %v", err))
					// Also show the specific line/character if possible
					errorMsg := fmt.Sprintf("JSON Validation Error: %v\n\nPlease check your JSON syntax. Common issues:\n• Trailing commas in arrays/objects\n• Missing quotes around strings\n• Unclosed brackets or braces", err)
					return m, tea.Printf(errorMsg)
				}
				log.Printf("DEBUG: JSON validation passed")
				// Clear any previous error since JSON is now valid
				m.SetError("")
				
				// Save config to database
				configName := m.nameInput.Value()
				if configName == "" {
					configName = "Unnamed Configuration"
				}
				log.Printf("DEBUG: Config name: '%s'", configName)
				
				// Convert from UI format (mcpServers) to internal format (servers)
				var configData models.MCPConfigData
				configData.Servers = make(map[string]models.MCPServerConfig)
				
				// Handle both old format (mcpServers) and new format (servers)
				var serversData map[string]interface{}
				if mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{}); ok {
					serversData = mcpServers
				} else if servers, ok := rawConfig["servers"].(map[string]interface{}); ok {
					serversData = servers
				} else {
					m.SetError("Invalid config format: must contain 'mcpServers' or 'servers' field")
					return m, tea.Printf("Error: Invalid config format")
				}
				
				// Convert servers to the expected format
				for serverName, serverConfigRaw := range serversData {
					serverConfigMap, ok := serverConfigRaw.(map[string]interface{})
					if !ok {
						continue
					}
					
					serverConfig := models.MCPServerConfig{}
					if command, ok := serverConfigMap["command"].(string); ok {
						serverConfig.Command = command
					}
					if argsRaw, ok := serverConfigMap["args"].([]interface{}); ok {
						for _, arg := range argsRaw {
							if argStr, ok := arg.(string); ok {
								serverConfig.Args = append(serverConfig.Args, argStr)
							}
						}
					}
					if envRaw, ok := serverConfigMap["env"].(map[string]interface{}); ok {
						serverConfig.Env = make(map[string]string)
						for k, v := range envRaw {
							if vStr, ok := v.(string); ok {
								serverConfig.Env[k] = vStr
							}
						}
					}
					
					configData.Servers[serverName] = serverConfig
				}
				
				log.Printf("DEBUG: Converted to internal format with %d servers", len(configData.Servers))
				
				// Add the config name to the data structure
				configData.Name = configName
				
				// Save config using service (with proper encryption)
				savedConfig, err := m.mcpConfigSvc.UploadConfig(m.selectedEnvID, &configData)
				if err != nil {
					log.Printf("DEBUG: Failed to save config: %v", err)
					return m, tea.Printf("Error: Failed to save config: %v", err)
				}
				log.Printf("DEBUG: Saved encrypted config to database: %+v", savedConfig)
				
				// Trigger tool replacement for the saved config
				log.Printf("DEBUG: Starting tool replacement for config '%s' in environment %d", configName, m.selectedEnvID)
				go m.replaceToolsAsync(m.selectedEnvID, configName)
				
				// Stay in editor mode and show success toast
				log.Printf("DEBUG: Staying in editor mode, showing success toast")
				
				// Show success message but stay in editor
				return m, tea.Printf("✅ Configuration '%s' v%d saved successfully", configName, savedConfig.Version)
			}
		case "d":
			log.Printf("DEBUG: Received 'd' key - mode: %d, configs count: %d, selectedIdx: %d", m.mode, len(m.configs), m.selectedIdx)
			if m.mode == MCPModeList && len(m.configs) > 0 {
				log.Printf("DEBUG: Calling deleteConfig for config ID: %d", m.configs[m.selectedIdx].ID)
				// Delete selected config
				return m, m.deleteConfig(m.configs[m.selectedIdx].ID)
			} else {
				log.Printf("DEBUG: Delete conditions not met - mode: %d, configs: %d", m.mode, len(m.configs))
			}
		case "up", "k":
			log.Printf("DEBUG: Received 'k' or 'up' key, mode=%d, configs=%d", m.mode, len(m.configs))
			if m.mode == MCPModeList && len(m.configs) > 0 {
				m.selectedIdx = (m.selectedIdx - 1 + len(m.configs)) % len(m.configs)
				log.Printf("DEBUG: Moved selection up to index %d", m.selectedIdx)
				return m, nil
			}
		case "down", "j":
			log.Printf("DEBUG: Received 'j' or 'down' key, mode=%d, configs=%d", m.mode, len(m.configs))
			if m.mode == MCPModeList && len(m.configs) > 0 {
				m.selectedIdx = (m.selectedIdx + 1) % len(m.configs)
				log.Printf("DEBUG: Moved selection down to index %d", m.selectedIdx)
				return m, nil
			}
		case "tab":
			if m.mode == MCPModeEdit {
				// Cycle through fields
				switch m.focusedField {
				case MCPFieldName:
					m.nameInput.Blur()
					m.focusedField = MCPFieldEnvironment
				case MCPFieldEnvironment:
					m.focusedField = MCPFieldConfig
					m.configEditor.Focus()
				case MCPFieldConfig:
					m.configEditor.Blur()
					m.focusedField = MCPFieldName
					m.nameInput.Focus()
				}
				return m, nil
			}
		case "esc":
			if m.mode == MCPModeEdit {
				m.mode = MCPModeList
				m.showEditor = false
				m.nameInput.Blur()
				m.configEditor.Blur()
				m.focusedField = MCPFieldName
				// Reload configs when exiting editor to show any newly saved configs
				return m, m.loadConfigs()
			}
		case "F1":
			// F1 key as a reliable save - should work from any field
			if m.mode == MCPModeEdit {
				return m, m.saveConfig()
			}
		case "ctrl+s":
			if m.mode == MCPModeEdit {
				return m, m.saveConfig()
			}
		}
		
		// Handle input for components based on focused field
		if m.mode == MCPModeEdit {
			switch m.focusedField {
			case MCPFieldName:
				var cmd tea.Cmd
				m.nameInput, cmd = m.nameInput.Update(msg)
				cmds = append(cmds, cmd)
			case MCPFieldEnvironment:
				// Handle environment selection with up/down keys
				if msg.String() == "up" || msg.String() == "k" {
					if len(m.envs) > 0 {
						for i, env := range m.envs {
							if env.ID == m.selectedEnvID {
								prevIdx := (i - 1 + len(m.envs)) % len(m.envs)
								m.selectedEnvID = m.envs[prevIdx].ID
								break
							}
						}
					}
				} else if msg.String() == "down" || msg.String() == "j" {
					if len(m.envs) > 0 {
						for i, env := range m.envs {
							if env.ID == m.selectedEnvID {
								nextIdx := (i + 1) % len(m.envs)
								m.selectedEnvID = m.envs[nextIdx].ID
								break
							}
						}
					}
				}
			case MCPFieldConfig:
				// Clear error when user starts typing
				if msg.Type == tea.KeyRunes || msg.String() == "backspace" || msg.String() == "delete" {
					m.SetError("")
				}
				// Handle save keys in config field before passing to textarea
				if msg.String() == "ctrl+s" || msg.String() == "ctrl+enter" {
					return m, m.saveConfig()
				}
				var cmd tea.Cmd
				m.configEditor, cmd = m.configEditor.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
		
	case MCPConfigSavedMsg:
		log.Printf("DEBUG: Received MCPConfigSavedMsg: %+v", msg)
		// Handle config save response
		if msg.Error != "" {
			// Show error message
			log.Printf("DEBUG: Error in save: %s", msg.Error)
			return m, tea.Printf("Error: %s", msg.Error)
		} else {
			// Config saved successfully - return to list mode
			log.Printf("DEBUG: Save successful, returning to list mode")
			m.mode = MCPModeList
			m.showEditor = false
			m.nameInput.Blur()
			m.configEditor.Blur()
			m.focusedField = MCPFieldName
			m.nameInput.SetValue("")
			m.configEditor.SetValue("")
			return m, tea.Printf(msg.Message)
		}
		
	case MCPConfigsLoadedMsg:
		log.Printf("DEBUG: Received MCPConfigsLoadedMsg with %d configs", len(msg.Configs))
		// Handle configs loaded response
		if msg.Error != nil {
			log.Printf("DEBUG: Error loading configs: %v", msg.Error)
			m.SetLoading(false)
			return m, tea.Printf("Error loading configs: %v", msg.Error)
		}
		
		// Update configs and stop loading
		m.configs = msg.Configs
		m.SetLoading(false)
		log.Printf("DEBUG: Configs loaded successfully, loading state cleared")
		return m, nil
		
	case MCPConfigDeletedMsg:
		log.Printf("DEBUG: Received MCPConfigDeletedMsg for config %d", msg.ConfigID)
		// Handle config delete response
		if msg.Error != nil {
			log.Printf("DEBUG: Error deleting config: %v", msg.Error)
			return m, tea.Printf("Error: %s", msg.Message)
		}
		
		// Remove from local list and adjust selection
		for i, config := range m.configs {
			if config.ID == msg.ConfigID {
				m.configs = append(m.configs[:i], m.configs[i+1:]...)
				if m.selectedIdx >= len(m.configs) && len(m.configs) > 0 {
					m.selectedIdx = len(m.configs) - 1
				}
				break
			}
		}
		
		log.Printf("DEBUG: Config deleted successfully, %d configs remaining", len(m.configs))
		return m, tea.Printf(msg.Message)
	}
	
	// Update components when editor is shown - handled by focused field logic above
	// This section is now handled in the switch statement based on focusedField
	
	return m, tea.Batch(cmds...)
}

// View renders the MCP tab
func (m MCPModel) View() string {
	if m.IsLoading() {
		return components.RenderLoadingIndicator(0)
	}
	
	if m.showEditor {
		return m.renderEditor()
	}
	
	return m.renderConfigList()
}

// RefreshData reloads MCP configs from database
func (m *MCPModel) RefreshData() tea.Cmd {
	m.SetLoading(true)
	return m.loadConfigs()
}

// Render configuration list
func (m MCPModel) renderConfigList() string {
	// Show breadcrumb if available (though main view doesn't need it)
	breadcrumb := ""
	if !m.IsMainView() {
		breadcrumb = "MCP Servers › Configuration List"
	}
	
	header := components.RenderSectionHeader("MCP Server Configurations")
	
	// Config list
	var configItems []string
	configItems = append(configItems, styles.HeaderStyle.Render("Available Configurations:"))
	configItems = append(configItems, "")
	
	for i, config := range m.configs {
		prefix := "  "
		if i == m.selectedIdx {
			prefix = "▶ "
		}
		configLine := fmt.Sprintf("%s (v%d) - Updated %s - Size %s", 
			config.Name, config.Version, config.Updated, config.Size)
		configItems = append(configItems, prefix+configLine)
	}
	
	configList := strings.Join(configItems, "\n")
	
	helpText := styles.HelpStyle.Render("• ↑/↓/j/k: navigate • n: new config • enter: edit • d: delete selected")
	
	var sections []string
	if breadcrumb != "" {
		sections = append(sections, breadcrumb, "")
	}
	sections = append(sections, header, "", configList, "", "", helpText)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Render configuration editor - compact like dashboard
func (m MCPModel) renderEditor() string {
	var sections []string
	
	// Header
	header := components.RenderSectionHeader("MCP Configuration Editor")
	sections = append(sections, header)
	
	// Show error if there is one
	if m.GetError() != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Background(lipgloss.Color("#331111")).
			Padding(1).
			Margin(1).
			Bold(true)
		sections = append(sections, errorStyle.Render("❌ "+m.GetError()))
		sections = append(sections, "")
	}
	
	// Compact form layout - everything inline like dashboard stats  
	nameLabel := "Name:"
	envLabel := "Environment:"
	if m.focusedField == MCPFieldName {
		nameLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Name:")
	}
	if m.focusedField == MCPFieldEnvironment {
		envLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Environment:")
	}
	
	// Compact name and environment section
	nameSection := lipgloss.JoinHorizontal(lipgloss.Top, 
		nameLabel, " ", m.nameInput.View())
	
	// Environment selection - simple inline
	envName := "default"
	if len(m.envs) > 0 {
		for _, env := range m.envs {
			if env.ID == m.selectedEnvID {
				envName = env.Name
				break
			}
		}
	}
	envSection := lipgloss.JoinHorizontal(lipgloss.Top,
		envLabel, " ", envName)
	
	sections = append(sections, nameSection)
	sections = append(sections, envSection)
	sections = append(sections, "")
	
	// Config editor - simple and compact
	configLabel := "Configuration:"
	if m.focusedField == MCPFieldConfig {
		configLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Configuration:")
	}
	
	// Set reasonable size for config editor
	m.configEditor.SetWidth(m.width - 4)
	m.configEditor.SetHeight(m.height - 10) // Simple calculation like dashboard
	
	sections = append(sections, configLabel)
	sections = append(sections, m.configEditor.View())
	
	// Simple help text
	helpText := styles.HelpStyle.Render("tab: switch • enter: save (from name/env field) • ctrl+s: save • esc: cancel")
	sections = append(sections, "")
	sections = append(sections, helpText)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// MCPConfigsLoadedMsg represents loaded configs
type MCPConfigsLoadedMsg struct {
	Configs []MCPConfigDisplay
	Error   error
}

// Load configurations from database
func (m MCPModel) loadConfigs() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Loading configs from database")
		
		// Use shared services for decryption
		
		// Load all configs from all environments and group them by name
		configMap := make(map[string]*MCPConfigDisplay)
		
		// Get all environments first
		envs, err := m.repos.Environments.List()
		if err != nil {
			log.Printf("DEBUG: Failed to load environments: %v", err)
			return MCPConfigsLoadedMsg{Error: err}
		}
		
		// For each environment, load only the latest configs
		for _, env := range envs {
			configs, err := m.repos.MCPConfigs.GetLatestConfigs(env.ID)
			if err != nil {
				log.Printf("DEBUG: Failed to load latest configs for env %d: %v", env.ID, err)
				continue
			}
			
			// Convert to display format - these are already the latest versions
			for _, config := range configs {
				// Decrypt the config to extract name and JSON for display
				var configJSON string
				var configName string = fmt.Sprintf("config-%d", config.ID) // Default fallback
				
				if config.EncryptionKeyID == "" {
					// Config is not encrypted - parse to extract name and format for UI
					var rawConfig map[string]interface{}
					if err := json.Unmarshal([]byte(config.ConfigJSON), &rawConfig); err == nil {
						// Extract name if present
						if name, ok := rawConfig["name"].(string); ok && name != "" {
							configName = name
						}
						
						// Convert to UI format if needed
						if _, ok := rawConfig["mcpServers"]; ok {
							// Already in UI format
							configJSON = config.ConfigJSON
						} else if servers, ok := rawConfig["servers"]; ok {
							// Convert from internal format to UI format
							uiFormat := map[string]interface{}{
								"mcpServers": servers,
							}
							if uiJSON, err := json.MarshalIndent(uiFormat, "", "  "); err == nil {
								configJSON = string(uiJSON)
							} else {
								configJSON = config.ConfigJSON // fallback
							}
						} else {
							configJSON = config.ConfigJSON // fallback
						}
					} else {
						configJSON = config.ConfigJSON // fallback on parse error
					}
				} else {
					// Config is encrypted, decrypt it
					decryptedData, err := m.mcpConfigSvc.DecryptConfigWithKeyID(config.ConfigJSON, config.EncryptionKeyID)
					if err != nil {
						log.Printf("DEBUG: Failed to decrypt config %d: %v", config.ID, err)
						configName = fmt.Sprintf("config-%d (encrypted)", config.ID)
						configJSON = "{\"error\": \"failed to decrypt\"}"
					} else {
						// Use the name from the decrypted data if available
						if decryptedData.Name != "" {
							configName = decryptedData.Name
						}
						
						// Convert from internal format (servers) to UI format (mcpServers)
						uiFormat := map[string]interface{}{
							"mcpServers": decryptedData.Servers,
						}
						
						if decryptedJSON, err := json.MarshalIndent(uiFormat, "", "  "); err == nil {
							configJSON = string(decryptedJSON)
						} else {
							log.Printf("DEBUG: Failed to serialize decrypted config %d: %v", config.ID, err)  
							configJSON = "{\"error\": \"failed to serialize\"}"
						}
						
						// For encrypted configs, use a generated name since we don't store name separately
						// In the future, we could enhance this to extract name from decrypted data
					}
				}
				
				// Create display item
				displayConfig := &MCPConfigDisplay{
					ID:         config.ID,
					Name:       configName,
					Version:    config.Version,
					Updated:    config.UpdatedAt.Format("Jan 2 15:04"),
					Size:       fmt.Sprintf("%.1fKB", float64(len(configJSON))/1024),
					ConfigJSON: configJSON, // Use the decrypted JSON for display
				}
				
				// Keep only the latest version of each named config
				if existing, exists := configMap[configName]; !exists || config.Version > existing.Version {
					configMap[configName] = displayConfig
				}
			}
		}
		
		// Convert map to slice
		var displayConfigs []MCPConfigDisplay
		for _, config := range configMap {
			displayConfigs = append(displayConfigs, *config)
		}
		
		log.Printf("DEBUG: Loaded %d configs from database, returning MCPConfigsLoadedMsg", len(displayConfigs))
		return MCPConfigsLoadedMsg{Configs: displayConfigs}
	})
}

// Load environments from database
func (m *MCPModel) loadEnvironments() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		envs, err := m.repos.Environments.List()
		if err != nil {
			return err
		}
		
		m.envs = envs
		
		// Convert to list items
		items := make([]list.Item, len(envs))
		for i, env := range envs {
			items[i] = EnvironmentItem{env: *env}
		}
		m.environmentList.SetItems(items)
		
		// Set default selection to first environment
		if len(envs) > 0 {
			m.selectedEnvID = envs[0].ID
		}
		
		return nil
	})
}

// Save configuration with tool discovery
func (m MCPModel) saveConfig() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: saveConfig() called")
		// Parse the config JSON to validate it
		configText := m.configEditor.Value()
		log.Printf("DEBUG: configText length: %d", len(configText))
		if configText == "" {
			log.Printf("DEBUG: Config text is empty, returning error")
			return MCPConfigSavedMsg{Error: "Configuration cannot be empty"}
		}
		
		// Basic JSON validation
		var configData models.MCPConfigData
		if err := json.Unmarshal([]byte(configText), &configData); err != nil {
			log.Printf("DEBUG: JSON validation failed: %v", err)
			return MCPConfigSavedMsg{Error: fmt.Sprintf("Invalid JSON: %v", err)}
		}
		log.Printf("DEBUG: JSON validation passed")
		
		// Save config to database (simplified for now)
		// In a real implementation, this would use the MCPConfigService
		configName := m.nameInput.Value()
		if configName == "" {
			configName = "Unnamed Configuration"
		}
		log.Printf("DEBUG: Config name: '%s'", configName)
		
		// Create new config display item
		newConfig := MCPConfigDisplay{
			ID:      int64(len(m.configs) + 1), // Simple ID for now
			Name:    configName,
			Version: 1,
			Updated: "just now",
			Size:    fmt.Sprintf("%.1fKB", float64(len(configText))/1024),
		}
		log.Printf("DEBUG: Created newConfig: %+v", newConfig)
		
		// Add to configs list
		m.configs = append(m.configs, newConfig)
		log.Printf("DEBUG: Added to configs list, new length: %d", len(m.configs))
		
		// TODO: Trigger tool discovery for this config/environment
		// This would call: toolDiscoveryService.DiscoverTools(m.selectedEnvID)
		
		result := MCPConfigSavedMsg{
			Config: newConfig,
			Message: fmt.Sprintf("Configuration '%s' saved successfully", configName),
		}
		log.Printf("DEBUG: Returning MCPConfigSavedMsg: %+v", result)
		return result
	})
}

// MCPConfigDeletedMsg represents a deleted config
type MCPConfigDeletedMsg struct {
	ConfigID int64
	Error    error
	Message  string
}

// Delete configuration with cascade deletion of servers and tools
func (m MCPModel) deleteConfig(configID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Deleting MCP config %d with cascade deletion", configID)
		
		// First, get the config to extract its name and environment
		targetConfig, err := m.repos.MCPConfigs.GetByID(configID)
		if err != nil {
			log.Printf("DEBUG: Failed to get config %d: %v", configID, err)
			return MCPConfigDeletedMsg{
				ConfigID: configID,
				Error:    err,
				Message:  fmt.Sprintf("Configuration not found: %v", err),
			}
		}
		
		// Extract the config name to delete all versions with the same name
		var configName string
		var configData map[string]interface{}
		if err := json.Unmarshal([]byte(targetConfig.ConfigJSON), &configData); err == nil {
			if name, ok := configData["name"].(string); ok && name != "" {
				configName = name
			}
		}
		if configName == "" {
			configName = fmt.Sprintf("config-%d", configID)
		}
		log.Printf("DEBUG: Config name extracted: '%s', will delete all versions", configName)
		
		// Get all configs with the same name in the same environment
		allConfigs, err := m.repos.MCPConfigs.ListByEnvironment(targetConfig.EnvironmentID)
		if err != nil {
			log.Printf("DEBUG: Failed to get configs for environment %d: %v", targetConfig.EnvironmentID, err)
			return MCPConfigDeletedMsg{
				ConfigID: configID,
				Error:    err,
				Message:  fmt.Sprintf("Failed to get configs for deletion: %v", err),
			}
		}
		
		// Find all configs with the same name
		var configsToDelete []*models.MCPConfig
		for _, config := range allConfigs {
			var otherConfigData map[string]interface{}
			otherConfigName := fmt.Sprintf("config-%d", config.ID)
			if err := json.Unmarshal([]byte(config.ConfigJSON), &otherConfigData); err == nil {
				if name, ok := otherConfigData["name"].(string); ok && name != "" {
					otherConfigName = name
				}
			}
			
			if otherConfigName == configName {
				configsToDelete = append(configsToDelete, config)
			}
		}
		
		log.Printf("DEBUG: Found %d config versions to delete with name '%s'", len(configsToDelete), configName)
		
		// Delete each config version with cascade deletion
		for _, config := range configsToDelete {
			log.Printf("DEBUG: Deleting config version %d (ID: %d)", config.Version, config.ID)
			
			// Get all servers for this config to cascade delete tools
			servers, err := m.repos.MCPServers.GetByConfigID(config.ID)
			if err != nil {
				log.Printf("DEBUG: Failed to get servers for config %d: %v", config.ID, err)
				return MCPConfigDeletedMsg{
					ConfigID: configID,
					Error:    err,
					Message:  fmt.Sprintf("Failed to get servers for config %d: %v", config.ID, err),
				}
			}
			
			// Delete tools for each server
			for _, server := range servers {
				if err := m.repos.MCPTools.DeleteByServerID(server.ID); err != nil {
					log.Printf("DEBUG: Failed to delete tools for server %d: %v", server.ID, err)
					return MCPConfigDeletedMsg{
						ConfigID: configID,
						Error:    err,
						Message:  fmt.Sprintf("Failed to delete tools for server %s: %v", server.Name, err),
					}
				}
				log.Printf("DEBUG: Deleted tools for server %s (ID: %d)", server.Name, server.ID)
			}
			
			// Delete the servers
			if err := m.repos.MCPServers.DeleteByConfigID(config.ID); err != nil {
				log.Printf("DEBUG: Failed to delete servers for config %d: %v", config.ID, err)
				return MCPConfigDeletedMsg{
					ConfigID: configID,
					Error:    err,
					Message:  fmt.Sprintf("Failed to delete servers for config %d: %v", config.ID, err),
				}
			}
			
			// Delete the config itself
			if err := m.repos.MCPConfigs.Delete(config.ID); err != nil {
				log.Printf("DEBUG: Failed to delete config %d: %v", config.ID, err)
				return MCPConfigDeletedMsg{
					ConfigID: configID,
					Error:    err,
					Message:  fmt.Sprintf("Failed to delete configuration %d: %v", config.ID, err),
				}
			}
			
			log.Printf("DEBUG: Successfully deleted config version %d (ID: %d)", config.Version, config.ID)
		}
		
		log.Printf("DEBUG: Successfully deleted all %d versions of config '%s'", len(configsToDelete), configName)
		return MCPConfigDeletedMsg{
			ConfigID: configID,
			Message:  fmt.Sprintf("Configuration '%s' (all %d versions) deleted successfully", configName, len(configsToDelete)),
		}
	})
}

// IsMainView returns true if in main list view
func (m MCPModel) IsMainView() bool {
	return m.mode == MCPModeList
}


// replaceToolsAsync runs tool replacement in the background after config save  
func (m *MCPModel) replaceToolsAsync(environmentID int64, configName string) {
	// Run tool replacement using shared service with transaction support
	result, err := m.toolDiscoverySvc.ReplaceToolsWithTransaction(environmentID, configName)
	if err != nil {
		log.Printf("ERROR: Tool replacement failed for config '%s' in environment %d: %v", configName, environmentID, err)
		return
	}
	
	if !result.Success {
		log.Printf("ERROR: Tool replacement completed with errors for config '%s' in environment %d:", configName, environmentID)
		for _, discoveryErr := range result.Errors {
			log.Printf("  - %s: %s", discoveryErr.Type, discoveryErr.Message)
			if discoveryErr.Details != "" {
				log.Printf("    Details: %s", discoveryErr.Details)
			}
		}
	} else {
		log.Printf("DEBUG: Tool replacement completed successfully for config '%s' in environment %d - %d tools from %d/%d servers", 
			configName, environmentID, result.TotalTools, result.SuccessfulServers, result.TotalServers)
	}
}