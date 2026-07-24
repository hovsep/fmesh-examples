package styles

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	ColorPrimary   = lipgloss.Color("86")  // Cyan
	ColorSecondary = lipgloss.Color("212") // Pink
	ColorSuccess   = lipgloss.Color("42")  // Green
	ColorWarning   = lipgloss.Color("214") // Orange
	ColorDanger    = lipgloss.Color("196") // Red
	ColorMuted     = lipgloss.Color("240") // Gray
	ColorText      = lipgloss.Color("252") // Light gray
	ColorBorder    = lipgloss.Color("238") // Dark gray

	// Header styles
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	StatusAliveStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	StatusDeadStyle = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	// Tab styles
	TabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(lipgloss.Color("236")).
			Padding(0, 2)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Background(lipgloss.Color("234")).
				Padding(0, 2)

	// Panel styles
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	PanelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Underline(true)

	// Value styles
	ValueNormalStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	ValueWarningStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)

	ValueCriticalStyle = lipgloss.NewStyle().
				Foreground(ColorDanger).
				Bold(true)

	LabelStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	UnitStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Progress bar styles
	ProgressFullChar  = "█"
	ProgressEmptyChar = "░"

	// Help style
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)
)

// GetHealthStyle returns the style for a health status
func GetHealthStyle(isHealthy bool, isCritical bool) lipgloss.Style {
	if isCritical {
		return ValueCriticalStyle
	}
	if !isHealthy {
		return ValueWarningStyle
	}
	return ValueNormalStyle
}
