package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderOverlay renders content on top of a background with optional backdrop
func RenderOverlay(width, height int, background, overlay string, withBackdrop bool, backgroundColor lipgloss.Color) string {
	if overlay == "" {
		return background
	}

	// Add backdrop if requested
	if withBackdrop {
		backdropStyle := lipgloss.NewStyle().
			Width(width).
			Height(height).
			Background(backgroundColor.Darker(0.8))
		
		background = backdropStyle.Render(background)
	}

	// Calculate overlay position (centered)
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := lipgloss.Height(overlay)
	
	x := (width - overlayWidth) / 2
	y := (height - overlayHeight) / 2
	
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	// Place overlay on background
	return PlaceOverlay(x, y, overlay, background)
}

// PlaceOverlay places content at specific coordinates on a background
func PlaceOverlay(x, y int, overlay, background string) string {
	backgroundLines := strings.Split(background, "\n")
	overlayLines := strings.Split(overlay, "\n")
	
	// Ensure background has enough lines
	for len(backgroundLines) < y+len(overlayLines) {
		backgroundLines = append(backgroundLines, "")
	}
	
	// Place overlay lines onto background
	for i, overlayLine := range overlayLines {
		bgLineIndex := y + i
		if bgLineIndex >= 0 && bgLineIndex < len(backgroundLines) {
			bgLine := backgroundLines[bgLineIndex]
			
			// Extend background line if needed
			for len(bgLine) < x+lipgloss.Width(overlayLine) {
				bgLine += " "
			}
			
			// Replace the section with overlay content
			if x >= 0 && x < len(bgLine) {
				before := bgLine[:x]
				overlayWidth := lipgloss.Width(overlayLine)
				after := ""
				if x+overlayWidth < len(bgLine) {
					after = bgLine[x+overlayWidth:]
				}
				backgroundLines[bgLineIndex] = before + overlayLine + after
			}
		}
	}
	
	return strings.Join(backgroundLines, "\n")
}