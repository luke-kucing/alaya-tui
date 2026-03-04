package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorPrimary   = lipgloss.Color("#7C3AED") // purple
	colorSecondary = lipgloss.Color("#6366F1") // indigo
	colorSuccess   = lipgloss.Color("#22C55E") // green
	colorError     = lipgloss.Color("#EF4444") // red
	colorWarning   = lipgloss.Color("#F59E0B") // amber
	colorMuted     = lipgloss.Color("#6B7280") // gray
	colorBg        = lipgloss.Color("#1E1B2E") // dark purple bg
	colorSurface   = lipgloss.Color("#2D2B3E") // lighter surface

	// Tab bar
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorPrimary).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Padding(0, 2)

	tabGapStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			SetString(" | ")

	// Content area
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0"))

	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorError)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Panels
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(1, 2)

	selectedStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(colorSurface).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)
