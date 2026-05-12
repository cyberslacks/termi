package app

import "github.com/charmbracelet/lipgloss"

var (
	ColorPrimary   = lipgloss.Color("#7C3AED") // violet
	ColorSecondary = lipgloss.Color("#06B6D4") // cyan
	ColorSuccess   = lipgloss.Color("#10B981") // green
	ColorWarning   = lipgloss.Color("#F59E0B") // amber
	ColorError     = lipgloss.Color("#EF4444") // red
	ColorMuted     = lipgloss.Color("#6B7280") // gray
	ColorText      = lipgloss.Color("#F9FAFB") // near-white
	ColorBg        = lipgloss.Color("#111827") // dark

	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	StyleSubtitle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleHighlight = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning)
	StyleError   = lipgloss.NewStyle().Foreground(ColorError)
	StyleMuted   = lipgloss.NewStyle().Foreground(ColorMuted)

	StyleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText).
			Background(ColorPrimary).
			Padding(0, 1)

	StyleTabInactive = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 1)

	StyleStatusBar = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Background(lipgloss.Color("#1F2937"))

	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary)

	StyleFocusedBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorSecondary)

	StyleBroadcastBar = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorWarning).
				Border(lipgloss.NormalBorder(), true, false, false, false).
				BorderForeground(ColorWarning).
				Padding(0, 1)
)
