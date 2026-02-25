package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorBorder = lipgloss.Color("#3c3836")
	colorGreen  = lipgloss.Color("#a9b665")
	colorYellow = lipgloss.Color("#d8a657")
	colorBlue   = lipgloss.Color("#7daea3")
	colorGray   = lipgloss.Color("#928374")
	colorFg     = lipgloss.Color("#d4be98")
	colorRed    = lipgloss.Color("#ea6962")

	listPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	previewPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorFg)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorYellow)

	currentMarkerStyle = lipgloss.NewStyle().
				Foreground(colorYellow)

	attachedStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	detachedStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	windowStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	inputStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed)
)
