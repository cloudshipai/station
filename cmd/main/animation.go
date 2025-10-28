package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// Animation frames and colors
var (
	// Tokyo Night color palette for animation
	neonPurple = lipgloss.Color("#bb9af7")
	neonBlue   = lipgloss.Color("#7dcfff")
	neonGreen  = lipgloss.Color("#9ece6a")
	neonOrange = lipgloss.Color("#ff9e64")
	neonRed    = lipgloss.Color("#f7768e")
	neonYellow = lipgloss.Color("#e0af68")
	darkBg     = lipgloss.Color("#1a1b26")
	terminalBg = lipgloss.Color("#24283b")
)

// Animation model for blastoff sequence
type blastoffModel struct {
	frame     int
	maxFrames int
	finished  bool
	width     int
	height    int
}

type frameMsg struct{}

func (m blastoffModel) Init() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return frameMsg{}
	})
}

func (m blastoffModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" || msg.String() == "esc" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case frameMsg:
		m.frame++
		if m.frame >= m.maxFrames {
			m.finished = true
			return m, tea.Quit
		}
		return m, tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
			return frameMsg{}
		})
	}
	return m, nil
}

func (m blastoffModel) View() string {
	if m.width == 0 {
		m.width = 80
		m.height = 24
	}

	// Create the animated frame
	content := m.renderFrame()

	// Center the content
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m blastoffModel) renderFrame() string {
	phase := m.frame / 15 // Each phase lasts ~15 frames

	switch phase {
	case 0:
		return m.renderIntro()
	case 1:
		return m.renderStationBuild()
	case 2:
		return m.renderRocketPrep()
	case 3:
		return m.renderCountdown()
	case 4:
		return m.renderBlastoff()
	case 5:
		return m.renderSpace()
	case 6:
		return m.renderSpaceshipBanner()
	default:
		return m.renderFinal()
	}
}

func (m blastoffModel) renderIntro() string {
	// Animated "STATION" title with pulsing colors
	cycleFrame := m.frame % 6
	colors := []lipgloss.Color{neonPurple, neonBlue, neonGreen, neonOrange, neonRed, neonYellow}

	titleStyle := lipgloss.NewStyle().
		Foreground(colors[cycleFrame]).
		Bold(true).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colors[(cycleFrame+2)%6]).
		Padding(1, 3).
		MarginBottom(2)

	title := titleStyle.Render("S T A T I O N")

	subtitle := lipgloss.NewStyle().
		Foreground(neonBlue).
		Italic(true).
		Render("🤖 AI Agent Management Platform 🤖")

	return title + "\n\n" + subtitle
}

func (m blastoffModel) renderStationBuild() string {
	// Build the space station frame by frame
	buildFrame := (m.frame - 15) % 15

	station := make([]string, 8)
	station[0] = "     ╭─────────────╮"
	station[1] = "     │  STATION-1  │"
	station[2] = "     ╰─┬─────────┬─╯"
	station[3] = "       │ ◉  ◉  ◉ │"
	station[4] = "       │ ███████ │"
	station[5] = "       │ ▓▓▓▓▓▓▓ │"
	station[6] = "       ╰─────────╯"
	station[7] = "         │  │  │"

	// Build station progressively
	visible := make([]string, min(buildFrame+1, len(station)))
	copy(visible, station[:len(visible)])

	stationStyle := lipgloss.NewStyle().
		Foreground(neonPurple).
		Bold(true)

	title := lipgloss.NewStyle().
		Foreground(neonGreen).
		Bold(true).
		Render("🏗️  CONSTRUCTING SPACE STATION  🏗️")

	return title + "\n\n" + stationStyle.Render(strings.Join(visible, "\n"))
}

func (m blastoffModel) renderRocketPrep() string {
	// Show rocket being prepared next to completed station
	rocket := []string{
		"        /\\",
		"       /  \\",
		"      /____\\",
		"      |🚀 |",
		"      |STAT|",
		"      |ION |",
		"      |____|",
		"       ||||",
		"      /    \\",
		"     ~~~~~~~~",
	}

	station := []string{
		"╭─────────────╮     ",
		"│  STATION-1  │     ",
		"╰─┬─────────┬─╯     ",
		"  │ ◉  ◉  ◉ │       ",
		"  │ ███████ │       ",
		"  │ ▓▓▓▓▓▓▓ │       ",
		"  ╰─────────╯       ",
		"    │  │  │         ",
		"                    ",
		"                    ",
	}

	// Combine station and rocket side by side
	var combined []string
	for i := 0; i < len(station); i++ {
		line := station[i]
		if i < len(rocket) {
			line += rocket[i]
		}
		combined = append(combined, line)
	}

	stationStyle := lipgloss.NewStyle().Foreground(neonPurple)
	rocketStyle := lipgloss.NewStyle().Foreground(neonOrange)

	title := lipgloss.NewStyle().
		Foreground(neonYellow).
		Bold(true).
		Render("🚀  ROCKET PREPARATION COMPLETE  🚀")

	result := title + "\n\n"
	for _, line := range combined {
		if strings.Contains(line, "STAT") || strings.Contains(line, "ION") {
			result += rocketStyle.Render(line) + "\n"
		} else {
			result += stationStyle.Render(line) + "\n"
		}
	}

	return result
}

func (m blastoffModel) renderCountdown() string {
	countFrame := (m.frame - 45) % 15
	countdown := []string{"10", "9", "8", "7", "6", "5", "4", "3", "2", "1", "IGNITION!", "LIFTOFF!", "🚀", "🌟", "✨"}

	countIndex := min(countFrame, len(countdown)-1)

	countStyle := lipgloss.NewStyle().
		Foreground(neonRed).
		Bold(true).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(neonOrange).
		Padding(2, 4).
		MarginBottom(3)

	rocket := lipgloss.NewStyle().
		Foreground(neonOrange).
		Bold(true).
		Render(`
        /\
       /  \
      /____\
      |🚀 |
      |STAT|
      |ION |
      |____|
       ||||
      /    \
     ~~~~~~~~`)

	title := lipgloss.NewStyle().
		Foreground(neonYellow).
		Bold(true).
		Render("🔥  COUNTDOWN SEQUENCE INITIATED  🔥")

	return title + "\n\n" + countStyle.Render(countdown[countIndex]) + "\n" + rocket
}

func (m blastoffModel) renderBlastoff() string {
	blastFrame := (m.frame - 60) % 15

	// Rocket moves up progressively
	rocketHeight := 10 - blastFrame
	if rocketHeight < 0 {
		rocketHeight = 0
	}

	// Create fire/exhaust trail
	fire := strings.Repeat("🔥", blastFrame+1)
	exhaust := strings.Repeat("💨", (blastFrame*2)+1)

	rocket := fmt.Sprintf(`%s
        /\
       /  \
      /____\
      |🚀 |
      |STAT|
      |ION |
      |____|
       ||||
      %s
     %s`, strings.Repeat("\n", rocketHeight), fire, exhaust)

	rocketStyle := lipgloss.NewStyle().
		Foreground(neonOrange).
		Bold(true)

	title := lipgloss.NewStyle().
		Foreground(neonRed).
		Bold(true).
		Render("🚀🔥  STATION BLASTOFF!  🔥🚀")

	return title + "\n" + rocketStyle.Render(rocket)
}

func (m blastoffModel) renderSpace() string {
	// Rocket in space with stars
	spaceFrame := (m.frame - 75) % 15

	// Generate some "stars"
	stars := make([]string, 15)
	for i := range stars {
		starLine := ""
		for j := 0; j < 40; j++ {
			if (i+j+spaceFrame)%7 == 0 {
				starLine += "✨"
			} else if (i+j+spaceFrame)%11 == 0 {
				starLine += "⭐"
			} else if (i+j+spaceFrame)%13 == 0 {
				starLine += "🌟"
			} else {
				starLine += " "
			}
		}
		stars[i] = starLine
	}

	// Small rocket in space
	rocket := `    🚀
   STATION
  ╱ ╲ ╱ ╲
 ╱   ╲   ╲
💫 MISSION 💫
  SUCCESS!`

	starsStyle := lipgloss.NewStyle().Foreground(neonYellow)
	rocketStyle := lipgloss.NewStyle().
		Foreground(neonPurple).
		Bold(true)

	title := lipgloss.NewStyle().
		Foreground(neonBlue).
		Bold(true).
		Render("🌌  WELCOME TO SPACE  🌌")

	return title + "\n\n" + starsStyle.Render(strings.Join(stars[:8], "\n")) + "\n\n" + rocketStyle.Render(rocket)
}

func (m blastoffModel) renderSpaceshipBanner() string {
	// Spaceship pulling a "WELCOME TO THE FUTURE" banner
	bannerFrame := (m.frame - 90) % 15

	// Spaceship design (inspired by the image style)
	spaceship := []string{
		"    ╭─╮",
		"   ╱   ╲",
		"  ╱  ◉  ╲",
		" ╱       ╲",
		"╱    ◈    ╲",
		"╲         ╱",
		" ╲   ▼   ╱",
		"  ╲_____╱",
		"    ║║║",
		"   🔥🔥🔥",
	}

	// Calculate banner position (slides in from right)
	screenWidth := 60
	bannerText := "═══╣ WELCOME TO THE FUTURE ╠═══"
	bannerStart := screenWidth - (bannerFrame * 4)
	if bannerStart < 0 {
		bannerStart = 0
	}

	// Create the banner line with proper spacing
	bannerLine := strings.Repeat(" ", bannerStart) + bannerText
	if len(bannerLine) > screenWidth {
		bannerLine = bannerLine[:screenWidth]
	}

	// Create connecting line between spaceship and banner
	connectionLength := max(1, bannerStart-15)
	connection := strings.Repeat("~", connectionLength)

	// Build the complete scene
	var scene []string

	// Add stars background
	for i := 0; i < 3; i++ {
		starLine := ""
		for j := 0; j < screenWidth; j++ {
			if (i+j+bannerFrame)%8 == 0 {
				starLine += "✨"
			} else if (i+j+bannerFrame)%12 == 0 {
				starLine += "⭐"
			} else {
				starLine += " "
			}
		}
		scene = append(scene, starLine)
	}

	// Add spaceship with banner
	spaceshipY := 4
	for i, shipLine := range spaceship {
		sceneY := spaceshipY + i

		// Ensure scene is large enough
		for len(scene) <= sceneY {
			scene = append(scene, strings.Repeat(" ", screenWidth))
		}

		// Add connection and banner on the middle line of spaceship
		if i == 4 { // Middle of spaceship
			fullLine := shipLine + connection + bannerText
			if len(fullLine) > screenWidth {
				fullLine = fullLine[:screenWidth]
			}
			scene[sceneY] = fullLine
		} else {
			scene[sceneY] = shipLine + strings.Repeat(" ", max(0, screenWidth-len(shipLine)))
		}
	}

	// Add more stars after spaceship
	for i := 0; i < 3; i++ {
		starLine := ""
		for j := 0; j < screenWidth; j++ {
			if (i+j+bannerFrame+10)%9 == 0 {
				starLine += "🌟"
			} else if (i+j+bannerFrame+5)%15 == 0 {
				starLine += "💫"
			} else {
				starLine += " "
			}
		}
		scene = append(scene, starLine)
	}

	// Style the scene
	spaceshipStyle := lipgloss.NewStyle().Foreground(neonBlue).Bold(true)
	bannerStyle := lipgloss.NewStyle().Foreground(neonPurple).Bold(true)
	starsStyle := lipgloss.NewStyle().Foreground(neonYellow)

	title := lipgloss.NewStyle().
		Foreground(neonGreen).
		Bold(true).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(neonBlue).
		Padding(0, 2).
		Render("🛸 SPACESHIP BANNER INCOMING 🛸")

	// Apply styles to different parts
	var styledScene []string
	for i, line := range scene {
		if i < 3 || i >= len(scene)-3 {
			// Star lines
			styledScene = append(styledScene, starsStyle.Render(line))
		} else if strings.Contains(line, "WELCOME") {
			// Banner line - style the banner part differently
			if strings.Contains(line, "╱") || strings.Contains(line, "╲") || strings.Contains(line, "◉") {
				// Spaceship + banner line
				parts := strings.Split(line, "WELCOME")
				if len(parts) == 2 {
					styledLine := spaceshipStyle.Render(parts[0]) + bannerStyle.Render("WELCOME"+parts[1])
					styledScene = append(styledScene, styledLine)
				} else {
					styledScene = append(styledScene, spaceshipStyle.Render(line))
				}
			} else {
				styledScene = append(styledScene, bannerStyle.Render(line))
			}
		} else {
			// Spaceship lines
			styledScene = append(styledScene, spaceshipStyle.Render(line))
		}
	}

	return title + "\n\n" + strings.Join(styledScene, "\n")
}

func (m blastoffModel) renderFinal() string {
	finalStyle := lipgloss.NewStyle().
		Foreground(neonPurple).
		Bold(true).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(neonBlue).
		Padding(2, 4).
		MarginBottom(2)

	logoStyle := lipgloss.NewStyle().
		Foreground(neonGreen).
		Bold(true)

	logo := `
    ╔═══════════════════╗
    ║    S T A T I O N   ║
    ║  🚀 ═══════════ 🚀 ║
    ║                   ║
    ║  Ready for Agent  ║
    ║    Management!    ║
    ╚═══════════════════╝
    
    🤖 AI • 🔧 MCP • 🌟 Tools
    
    Press any key to continue...`

	return finalStyle.Render("MISSION ACCOMPLISHED!") + "\n" + logoStyle.Render(logo)
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// runBlastoff implements the easter egg animation command
func runBlastoff(cmd *cobra.Command, args []string) error {
	model := blastoffModel{
		frame:     0,
		maxFrames: 135, // ~16 seconds of animation with spaceship banner
	}

	// Run the animation in full screen
	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err := program.Run()
	return err
}
