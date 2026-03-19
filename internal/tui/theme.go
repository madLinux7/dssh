package tui

import "github.com/charmbracelet/lipgloss"

var (
	purple      = lipgloss.Color("#7B2FBE")
	magenta     = lipgloss.Color("#D946EF")
	brightWhite = lipgloss.Color("#FFFFFF")
	dimGray     = lipgloss.Color("#5C5C5C")
	lightGray   = lipgloss.Color("#ABABAB")

	warnYellow = lipgloss.Color("#EAB308")
	warnOrange = lipgloss.Color("#F97316")
	warnRed    = lipgloss.Color("#EF4444")

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(brightWhite).
			Background(purple).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(dimGray).
				Padding(0, 2)

	tabGapStyle = lipgloss.NewStyle().
			Foreground(dimGray)

	contentStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(1, 2)

	focusedFieldStyle = lipgloss.NewStyle().
				Foreground(magenta).
				Bold(true)

	blurredFieldStyle = lipgloss.NewStyle().
				Foreground(lightGray)

	labelStyle = lipgloss.NewStyle().
			Foreground(brightWhite).
			Bold(true).
			Width(16)

	statusStyle = lipgloss.NewStyle().
			Foreground(dimGray).
			Italic(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22C55E")).
			Bold(true)

	titleStyle = lipgloss.NewStyle().
			Foreground(magenta).
			Bold(true).
			MarginBottom(1)

	// Passphrase modal styles.
	modalBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(magenta).
			Padding(1, 3).
			Width(48)

	modalTitleStyle = lipgloss.NewStyle().
			Foreground(brightWhite).
			Bold(true).
			MarginBottom(0)

	modalLabelStyle = lipgloss.NewStyle().
			Foreground(brightWhite).
			Bold(true).
			Width(12)

	modalErrorStyle = lipgloss.NewStyle().
			Foreground(warnRed).
			Bold(true)
)
