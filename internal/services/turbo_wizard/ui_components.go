package turbo_wizard

import (
	"fmt"
	"strings"
	"station/internal/theme"

	"github.com/charmbracelet/lipgloss"
)

// UIRenderer handles all UI rendering for the wizard
type UIRenderer struct {
	width        int
	height       int
	themeManager *theme.ThemeManager
}

// centerContent centers text content within the available width
func (r *UIRenderer) centerContent(content string) string {
	if r.width <= 0 {
		return content
	}
	
	lines := strings.Split(content, "\n")
	var centeredLines []string
	
	for _, line := range lines {
		centeredLines = append(centeredLines, r.centerLine(line))
	}
	
	return strings.Join(centeredLines, "\n")
}

// centerLine centers a single line of text
func (r *UIRenderer) centerLine(line string) string {
	if r.width <= 0 {
		return line
	}
	
	lineWidth := lipgloss.Width(line)
	if lineWidth < r.width {
		leftPadding := (r.width - lineWidth) / 2
		padding := strings.Repeat(" ", leftPadding)
		return padding + line
	}
	return line
}

// NewUIRenderer creates a new UI renderer
func NewUIRenderer(width, height int, themeManager *theme.ThemeManager) *UIRenderer {
	return &UIRenderer{
		width:        width,
		height:       height,
		themeManager: themeManager,
	}
}

// getStyles returns theme-aware styles for the wizard
func (r *UIRenderer) getStyles() wizardStyles {
	if r.themeManager == nil {
		// Fallback to hardcoded styles if no theme manager
		return wizardStyles{
			header:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
			current:     lipgloss.NewStyle().Background(lipgloss.Color("240")),
			completed:   lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
			instruction: lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
			warning:     lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
			error:       lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
			config:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1).MarginBottom(1),
			snippet:     lipgloss.NewStyle().Foreground(lipgloss.Color("8")).MarginLeft(4),
			container:   lipgloss.NewStyle().Align(lipgloss.Center).Padding(1, 2),
		}
	}

	themeStyles := r.themeManager.GetStyles()
	palette := r.themeManager.GetPalette()

	return wizardStyles{
		header:      themeStyles.Header,
		current:     lipgloss.NewStyle().Background(lipgloss.Color(palette.Selected)).Foreground(lipgloss.Color(palette.BackgroundDark)),
		completed:   themeStyles.Success,
		instruction: themeStyles.Info,
		warning:     themeStyles.Warning,
		error:       themeStyles.Error,
		config:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(palette.Border)).Padding(1).MarginBottom(1),
		snippet:     themeStyles.Muted.Copy().MarginLeft(4),
		container:   themeStyles.Container,
	}
}

// wizardStyles contains all the styles used by the wizard
type wizardStyles struct {
	header      lipgloss.Style
	current     lipgloss.Style
	completed   lipgloss.Style
	instruction lipgloss.Style
	warning     lipgloss.Style
	error       lipgloss.Style
	config      lipgloss.Style
	snippet     lipgloss.Style
	container   lipgloss.Style
}

// RenderBlockSelection renders the initial block selection screen
func (r *UIRenderer) RenderBlockSelection(model *TurboWizardModel) string {
	var b strings.Builder
	styles := r.getStyles()

	// Header - centered
	header := styles.header.Render("🧙 MCP Server Configuration Wizard")
	b.WriteString(r.centerLine(header))
	b.WriteString("\n\n")

	subtitle := "Found MCP server configurations. Select which ones you want to configure:"
	b.WriteString(r.centerLine(subtitle))
	b.WriteString("\n\n")

	// Show blocks with enhanced information
	for i, block := range model.blocks {
		cursor := " "
		if i == model.currentBlock {
			cursor = ">"
		}

		checkbox := "☐"
		if model.selectedBlocks[i] {
			checkbox = "☑"
		}

		style := lipgloss.NewStyle()
		if i == model.currentBlock {
			style = styles.current
		}

		// Enhanced block info with transport type
		transportInfo := ""
		if block.Transport != "" {
			transportInfo = fmt.Sprintf(" [%s]", strings.ToUpper(string(block.Transport)))
		}

		line := fmt.Sprintf("%s %s %s%s - %s",
			cursor, checkbox, block.ServerName, transportInfo, block.Description)
		b.WriteString(style.Render(line))
		b.WriteString("\n")

		// Show snippet of raw block for current item
		if i == model.currentBlock {
			snippet := block.RawBlock
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			b.WriteString(styles.snippet.Render("Preview: " + snippet))
			b.WriteString("\n")
		}
	}

	// Instructions
	b.WriteString("\n")
	selectedCount := 0
	for _, selected := range model.selectedBlocks {
		if selected {
			selectedCount++
		}
	}

	if selectedCount > 0 {
		b.WriteString(styles.warning.Render(fmt.Sprintf("Selected: %d server(s)", selectedCount)))
		b.WriteString("\n")
	}

	controls := styles.instruction.Render("Controls: ↑/↓ navigate, SPACE toggle selection, N next, Q quit")
	b.WriteString(r.centerLine(controls))

	return styles.container.Width(r.width).Render(b.String())
}

// RenderServerConfiguration renders the server configuration screen
func (r *UIRenderer) RenderServerConfiguration(model *TurboWizardModel) string {
	var b strings.Builder
	styles := r.getStyles()

	// Header - centered
	block := model.blocks[model.currentBlock]
	header := styles.header.Render(fmt.Sprintf("🔧 Configuring: %s", block.ServerName))
	b.WriteString(r.centerLine(header))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Transport: %s\n", strings.ToUpper(string(model.currentConfig.Transport))))
	b.WriteString(fmt.Sprintf("Description: %s\n\n", block.Description))

	// Configuration steps based on transport type
	steps := r.getConfigSteps(model.currentConfig.Transport)

	b.WriteString("Configuration Steps:\n")
	for i, step := range steps {
		cursor := " "
		if i == model.configStep {
			cursor = ">"
		}

		style := lipgloss.NewStyle()
		if i == model.configStep {
			style = styles.current
		} else if i < model.configStep {
			style = styles.completed
		}

		line := fmt.Sprintf("%s %d. %s", cursor, i+1, step.Name)
		if step.Required {
			line += " *"
		}
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Show current step details
	if model.configStep < len(steps) {
		step := steps[model.configStep]
		b.WriteString(r.renderCurrentStep(model, step))
	}

	b.WriteString("\n")
	controls := styles.instruction.Render("Controls: Y accept, E edit, B back, Q quit")
	b.WriteString(r.centerLine(controls))

	return styles.container.Width(r.width).Render(b.String())
}

// RenderFieldEditor renders the field editing interface
func (r *UIRenderer) RenderFieldEditor(model *TurboWizardModel) string {
	var b strings.Builder
	styles := r.getStyles()

	b.WriteString(styles.header.Render(fmt.Sprintf("✏️ Editing: %s", model.editingField)))
	b.WriteString("\n\n")

	// Show current value
	b.WriteString(fmt.Sprintf("Current value: %s\n", model.fieldValue))
	b.WriteString("Enter new value (or press Enter to keep current):\n\n")

	// Input field representation
	inputStyle := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1)
	if r.themeManager != nil {
		palette := r.themeManager.GetPalette()
		inputStyle = inputStyle.BorderForeground(lipgloss.Color(palette.Border))
	}
	b.WriteString(inputStyle.Render(model.fieldValue + "█"))
	b.WriteString("\n\n")

	controls := styles.instruction.Render("Controls: Type to edit, Enter to save, Esc to cancel")
	b.WriteString(r.centerLine(controls))

	return styles.container.Width(r.width).Render(b.String())
}

// RenderEnvironmentSelection renders the environment selection screen
func (r *UIRenderer) RenderEnvironmentSelection(model *TurboWizardModel) string {
	var b strings.Builder
	styles := r.getStyles()

	b.WriteString(styles.header.Render("🌍 Select Environment"))
	b.WriteString("\n\n")

	b.WriteString("Choose the environment to deploy these MCP servers:\n\n")

	for i, env := range model.environments {
		cursor := " "
		if i == model.selectedEnv {
			cursor = ">"
		}

		style := lipgloss.NewStyle()
		if i == model.selectedEnv {
			style = styles.current
		}

		line := fmt.Sprintf("%s %s", cursor, env)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	controls := styles.instruction.Render("Controls: ↑/↓ navigate, Enter to select, B back, Q quit")
	b.WriteString(r.centerLine(controls))

	return styles.container.Width(r.width).Render(b.String())
}

// RenderReviewConfig renders the final configuration review
func (r *UIRenderer) RenderReviewConfig(model *TurboWizardModel) string {
	var b strings.Builder
	styles := r.getStyles()

	header := styles.header.Render("📋 Review Configuration")
	b.WriteString(r.centerLine(header))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("You have configured %d MCP server(s):\n\n", len(model.configurations)))

	for i, config := range model.configurations {
		var configText strings.Builder
		configText.WriteString(fmt.Sprintf("Server %d: %s (%s)\n", i+1, config.Name, config.Transport))

		switch config.Transport {
		case TransportSTDIO:
			configText.WriteString(fmt.Sprintf("Command: %s\n", config.Command))
			if len(config.Args) > 0 {
				configText.WriteString(fmt.Sprintf("Args: %v\n", config.Args))
			}
		case TransportDocker:
			configText.WriteString(fmt.Sprintf("Image: %s\n", config.Command))
			if len(config.DockerMounts) > 0 {
				configText.WriteString("Mounts:\n")
				for _, mount := range config.DockerMounts {
					configText.WriteString(fmt.Sprintf("  %s → %s\n", mount.Source, mount.Target))
				}
			}
		case TransportHTTP, TransportSSE:
			configText.WriteString(fmt.Sprintf("URL: %s\n", config.URL))
		}

		if len(config.Env) > 0 {
			configText.WriteString("Environment:\n")
			for k, v := range config.Env {
				if strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "secret") {
					configText.WriteString(fmt.Sprintf("  %s=***hidden***\n", k))
				} else {
					configText.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
				}
			}
		}

		b.WriteString(styles.config.Render(configText.String()))
		b.WriteString("\n")
	}

	controls := styles.instruction.Render("Controls: Y accept and save, B go back, Q quit")
	b.WriteString(r.centerLine(controls))

	return styles.container.Width(r.width).Render(b.String())
}

// Helper methods

func (r *UIRenderer) getConfigSteps(transport MCPTransportType) []ConfigStep {
	baseSteps := []ConfigStep{
		{Name: "Server Name", Description: "Unique name for this server", Required: true, FieldType: "text"},
	}

	switch transport {
	case TransportSTDIO:
		return append(baseSteps,
			ConfigStep{Name: "Command", Description: "Executable command", Required: true, FieldType: "text"},
			ConfigStep{Name: "Arguments", Description: "Command line arguments", Required: false, FieldType: "text"},
			ConfigStep{Name: "Environment", Description: "Environment variables", Required: false, FieldType: "env"},
		)
	case TransportDocker:
		return append(baseSteps,
			ConfigStep{Name: "Docker Image", Description: "Container image", Required: true, FieldType: "text"},
			ConfigStep{Name: "Mounts", Description: "File system mounts", Required: false, FieldType: "mounts"},
			ConfigStep{Name: "Environment", Description: "Environment variables", Required: false, FieldType: "env"},
		)
	case TransportHTTP, TransportSSE:
		return append(baseSteps,
			ConfigStep{Name: "URL", Description: "Server endpoint URL", Required: true, FieldType: "url"},
			ConfigStep{Name: "Authentication", Description: "API keys and auth", Required: false, FieldType: "env"},
		)
	default:
		return baseSteps
	}
}

func (r *UIRenderer) renderCurrentStep(model *TurboWizardModel, step ConfigStep) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Current Step: %s\n", step.Name))
	b.WriteString(fmt.Sprintf("Description: %s\n\n", step.Description))

	// Show current values based on step
	switch step.FieldType {
	case "text":
		switch step.Name {
		case "Server Name":
			b.WriteString(fmt.Sprintf("Value: %s\n", model.currentConfig.Name))
		case "Command", "Docker Image":
			b.WriteString(fmt.Sprintf("Value: %s\n", model.currentConfig.Command))
		case "Arguments":
			if len(model.currentConfig.Args) > 0 {
				b.WriteString(fmt.Sprintf("Value: %v\n", model.currentConfig.Args))
			} else {
				b.WriteString("Value: (none)\n")
			}
		}
	case "url":
		b.WriteString(fmt.Sprintf("Value: %s\n", model.currentConfig.URL))
	case "env":
		if len(model.currentConfig.RequiredEnv) > 0 {
			b.WriteString("Required Environment Variables:\n")
			for _, env := range model.currentConfig.RequiredEnv {
				status := "❌ Missing"
				if env.Value != "" {
					if env.Type == "api_key" {
						status = "✅ Set (hidden)"
					} else {
						status = fmt.Sprintf("✅ Set: %s", env.Value)
					}
				}
				b.WriteString(fmt.Sprintf("  %s (%s): %s\n", env.Name, env.Type, status))
			}
		} else {
			b.WriteString("No environment variables required\n")
		}
	case "mounts":
		if len(model.currentConfig.DockerMounts) > 0 {
			b.WriteString("Docker Mounts:\n")
			for _, mount := range model.currentConfig.DockerMounts {
				b.WriteString(fmt.Sprintf("  %s → %s (%s)\n", mount.Source, mount.Target, mount.Type))
			}
		} else {
			b.WriteString("No mounts configured\n")
		}
	}

	return b.String()
}