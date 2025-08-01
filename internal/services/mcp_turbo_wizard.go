package services

import (
	tea "github.com/charmbracelet/bubbletea"
	"station/internal/services/turbo_wizard"
	"station/internal/theme"
	"station/pkg/models"
)

// Re-export types from turbo_wizard package for backward compatibility
type TurboWizardState = turbo_wizard.TurboWizardState
type TurboWizardModel = turbo_wizard.TurboWizardModel
type ServerConfig = turbo_wizard.ServerConfig
// Note: MCPServerBlock is already defined in github_discovery.go

// Re-export constants
const (
	StateShowingBlocks        = turbo_wizard.StateShowingBlocks
	StateConfiguringServer    = turbo_wizard.StateConfiguringServer
	StateEditingField         = turbo_wizard.StateEditingField
	StateSelectingEnvironment = turbo_wizard.StateSelectingEnvironment
	StateReviewConfig         = turbo_wizard.StateReviewConfig
	StateCompleted            = turbo_wizard.StateCompleted
)

// NewTurboWizardModel creates a new TurboTax-style wizard
func NewTurboWizardModel(blocks []MCPServerBlock) *TurboWizardModel {
	return NewTurboWizardModelWithTheme(blocks, nil)
}

// NewTurboWizardModelWithTheme creates a new TurboTax-style wizard with theme manager
func NewTurboWizardModelWithTheme(blocks []MCPServerBlock, themeManager *theme.ThemeManager) *TurboWizardModel {
	// Convert from the github_discovery MCPServerBlock to turbo_wizard MCPServerBlock
	turboBlocks := make([]turbo_wizard.MCPServerBlock, len(blocks))
	for i, block := range blocks {
		turboBlocks[i] = turbo_wizard.MCPServerBlock{
			ServerName:  block.ServerName,
			Description: block.Description,
			RawBlock:    block.RawBlock,
			Transport:   turbo_wizard.TransportSTDIO, // Default, will be detected by parser
		}
	}
	return turbo_wizard.NewTurboWizardModel(turboBlocks, themeManager)
}

// RunTurboWizard runs the complete TurboTax-style wizard flow
func RunTurboWizard(blocks []MCPServerBlock, environments []string) (*models.MCPConfigData, string, error) {
	return RunTurboWizardWithTheme(blocks, environments, nil)
}

// RunTurboWizardWithTheme runs the complete TurboTax-style wizard flow with theme manager
func RunTurboWizardWithTheme(blocks []MCPServerBlock, environments []string, themeManager *theme.ThemeManager) (*models.MCPConfigData, string, error) {
	wizard := NewTurboWizardModelWithTheme(blocks, themeManager)
	if len(environments) > 0 {
		wizard.SetEnvironments(environments)
	}
	
	program := tea.NewProgram(wizard, tea.WithAltScreen())
	
	finalModel, err := program.Run()
	if err != nil {
		return nil, "", err
	}
	
	wizardModel, ok := finalModel.(*TurboWizardModel)
	if !ok {
		return nil, "", err
	}
	
	if wizardModel.IsCancelled() {
		return nil, "", nil
	}
	
	if !wizardModel.IsCompleted() {
		return nil, "", nil
	}
	
	config := wizardModel.GetFinalMCPConfig()
	environment := wizardModel.GetSelectedEnvironment()
	
	return config, environment, nil
}