package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"station/internal/config"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Provider configuration
type ProviderConfig struct {
	Provider string
	Model    string
	BaseURL  string
}

// getProviderModels returns example models for each provider (any model string is accepted)
func getProviderModels() map[string][]string {
	return map[string][]string{
		"openai": config.GetSupportedOpenAIModels(),
		"gemini": {
			"gemini-2.5-flash",
			"gemini-2.5-pro",
		},
	}
}

// getDefaultProvider returns the default provider and model
func getDefaultProvider() (string, string) {
	return "openai", "gpt-5-mini"
}

// Provider descriptions for better UX
var providerDescriptions = map[string]string{
	"openai": "OpenAI models - GPT-5, GPT-4o, and more (any model accepted)",
	"gemini": "Google's Gemini models - Fast, capable, and cost-effective",
	"custom": "Configure a custom provider (any OpenAI-compatible endpoint)",
}

// setupProviderInteractively runs the interactive provider setup
func setupProviderInteractively() (*ProviderConfig, error) {
	// Check for environment variables first
	detectedProvider := ""
	if provider, _ := detectProviderFromEnv(); provider != "" {
		fmt.Print("Use this provider? [Y/n]: ")

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "n" && strings.ToLower(response) != "no" {
			detectedProvider = provider
		}
	}

	log.Printf("\n🤖 Let's set up your AI provider...\n")
	log.Printf("This will be used for intelligent agent execution and MCP tool management.\n\n")

	// Step 1: Provider selection (or use detected)
	var provider string
	var err error
	if detectedProvider != "" {
		provider = detectedProvider
		log.Printf("Using detected provider: %s\n\n", provider)
	} else {
		provider, err = selectProvider()
		if err != nil {
			return nil, err
		}
	}

	var model string
	if provider == "custom" {
		// Step 2a: Custom provider setup
		customProvider, customModel, err := setupCustomProvider()
		if err != nil {
			return nil, err
		}
		return &ProviderConfig{Provider: customProvider, Model: customModel, BaseURL: ""}, nil
	} else {
		// Step 2b: Model selection for known provider
		log.Printf("Selecting model for provider: '%s'\n", provider)
		model, err = selectModel(provider)
		if err != nil {
			return nil, fmt.Errorf("model selection failed for provider '%s': %w", provider, err)
		}
		log.Printf("Selected model: '%s'\n", model)
	}

	return &ProviderConfig{Provider: provider, Model: model, BaseURL: ""}, nil
}

// detectProviderFromEnv checks environment variables for API keys
func detectProviderFromEnv() (string, string) {
	if os.Getenv("OPENAI_API_KEY") != "" {
		recommended := config.GetRecommendedOpenAIModels()
		return "openai", recommended["cost_effective"]
	}
	if os.Getenv("GEMINI_API_KEY") != "" || os.Getenv("GOOGLE_API_KEY") != "" {
		return "gemini", "gemini-2.5-flash"
	}
	return "", ""
}

// Provider selection model
type providerModel struct {
	list     list.Model
	choice   string
	quitting bool
}

func (m providerModel) Init() tea.Cmd {
	return nil
}

func (m providerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = string(i)
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m providerModel) View() string {
	if m.choice != "" {
		return quitTextStyle.Render(fmt.Sprintf("You chose: %s", m.choice))
	}
	if m.quitting {
		return quitTextStyle.Render("Cancelled.")
	}
	return "\n" + m.list.View()
}

// Custom provider input model
type customProviderModel struct {
	providerInput textinput.Model
	modelInput    textinput.Model
	focusIndex    int
	inputs        []textinput.Model
	provider      string
	model         string
	done          bool
}

func newCustomProviderModel() customProviderModel {
	m := customProviderModel{
		inputs: make([]textinput.Model, 2),
	}

	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 64

		switch i {
		case 0:
			t.Placeholder = "Enter provider name (e.g., anthropic, ollama)"
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
		case 1:
			t.Placeholder = "Enter model name (e.g., claude-3-sonnet, llama2)"
		}

		m.inputs[i] = t
	}

	return m
}

func (m customProviderModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m customProviderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			if s == "enter" && m.focusIndex == len(m.inputs) {
				m.provider = m.inputs[0].Value()
				m.model = m.inputs[1].Value()
				m.done = true
				return m, tea.Quit
			}

			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > len(m.inputs) {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs)
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i <= len(m.inputs)-1; i++ {
				if i == m.focusIndex {
					cmds[i] = m.inputs[i].Focus()
					m.inputs[i].PromptStyle = focusedStyle
					m.inputs[i].TextStyle = focusedStyle
					continue
				}
				m.inputs[i].Blur()
				m.inputs[i].PromptStyle = noStyle
				m.inputs[i].TextStyle = noStyle
			}

			return m, tea.Batch(cmds...)
		}
	}

	cmd := m.updateInputs(msg)
	return m, cmd
}

func (m *customProviderModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m customProviderModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🔧 Custom Provider Setup"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Configure your custom AI provider (e.g., Anthropic, Ollama, local models)"))
	b.WriteString("\n\n")

	labels := []string{"Provider Name:", "Model Name:"}
	for i := range m.inputs {
		b.WriteString(labels[i])
		b.WriteString("\n")
		b.WriteString(m.inputs[i].View())
		if i < len(m.inputs)-1 {
			b.WriteString("\n\n")
		}
	}

	button := blurredButton
	if m.focusIndex == len(m.inputs) {
		button = focusedButton
	}
	b.WriteString(fmt.Sprintf("\n\n%s\n\n", button))

	helpText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("Tab/Shift+Tab: Navigate • Enter: Submit • Esc: Cancel")
	b.WriteString(helpText)

	return b.String()
}

// List item for providers/models
type item string

func (i item) FilterValue() string { return "" }

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#FF6B9D")).
			Padding(0, 2).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			MarginBottom(1)

	quitTextStyle = lipgloss.NewStyle().
			Margin(1, 0, 2, 4).
			Foreground(lipgloss.Color("#04B575"))

	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B9D"))
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	cursorStyle  = focusedStyle.Copy()
	noStyle      = lipgloss.NewStyle()

	focusedButton = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")).
			Background(lipgloss.Color("#FF6B9D")).
			Padding(0, 3).
			Render("Submit")
	blurredButton = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Background(lipgloss.Color("#3C3C3C")).
			Padding(0, 3).
			Render("Submit")
)

// selectProvider shows interactive provider selection
func selectProvider() (string, error) {
	items := []list.Item{
		item("openai"),
		item("gemini"),
		item("custom"),
	}

	const defaultWidth = 60
	const listHeight = 14

	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = "Select AI Provider"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	l.Styles.HelpStyle = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)

	m := providerModel{list: l}

	finalModel, err := tea.NewProgram(m).Run()
	if err != nil {
		return "", fmt.Errorf("error running provider selection: %w", err)
	}

	// Cast the returned model back to providerModel to access the choice
	if providerModel, ok := finalModel.(providerModel); ok {
		if providerModel.choice == "" {
			return "", fmt.Errorf("no provider selected")
		}
		return providerModel.choice, nil
	}

	return "", fmt.Errorf("failed to get provider selection")
}

// selectModel shows interactive model selection for a provider
func selectModel(provider string) (string, error) {
	providerModels := getProviderModels()
	models, exists := providerModels[provider]
	if !exists {
		return "", fmt.Errorf("unknown provider: %s", provider)
	}

	items := make([]list.Item, len(models))
	for i, model := range models {
		items[i] = item(model)
	}

	const defaultWidth = 60
	const listHeight = 14

	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = fmt.Sprintf("Select %s Model", strings.Title(provider))
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	l.Styles.HelpStyle = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)

	m := providerModel{list: l}

	finalModel, err := tea.NewProgram(m).Run()
	if err != nil {
		return "", fmt.Errorf("error running model selection: %w", err)
	}

	// Cast the returned model back to providerModel to access the choice
	if providerModel, ok := finalModel.(providerModel); ok {
		if providerModel.choice == "" {
			return "", fmt.Errorf("no model selected")
		}
		return providerModel.choice, nil
	}

	return "", fmt.Errorf("failed to get model selection")
}

// setupCustomProvider handles custom provider input
func setupCustomProvider() (string, string, error) {
	m := newCustomProviderModel()

	finalModel, err := tea.NewProgram(m).Run()
	if err != nil {
		return "", "", fmt.Errorf("error running custom provider setup: %w", err)
	}

	// Cast the returned model back to customProviderModel to access the results
	if customModel, ok := finalModel.(customProviderModel); ok {
		if !customModel.done {
			return "", "", fmt.Errorf("setup cancelled")
		}
		return customModel.provider, customModel.model, nil
	}

	return "", "", fmt.Errorf("failed to get custom provider setup")
}

// itemDelegate for list rendering
type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 2 }
func (d itemDelegate) Spacing() int                            { return 1 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	providerName := string(i)
	description := providerDescriptions[providerName]

	if index == m.Index() {
		// Selected item
		title := selectedItemStyle.Render(fmt.Sprintf("▶ %s", providerName))
		desc := selectedDescStyle.Render(description)
		_, _ = fmt.Fprintf(w, "%s\n%s", title, desc)
	} else {
		// Unselected item
		title := itemStyle.Render(fmt.Sprintf("  %s", providerName))
		desc := descStyle.Render(description)
		_, _ = fmt.Fprintf(w, "%s\n%s", title, desc)
	}
}

var (
	itemStyle = lipgloss.NewStyle().
			PaddingLeft(4).
			Foreground(lipgloss.Color("#FAFAFA"))

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("#FF6B9D")).
				Bold(true)

	descStyle = lipgloss.NewStyle().
			PaddingLeft(6).
			Foreground(lipgloss.Color("#626262")).
			Italic(true)

	selectedDescStyle = lipgloss.NewStyle().
				PaddingLeft(4).
				Foreground(lipgloss.Color("#FFA726")).
				Italic(true)
)
