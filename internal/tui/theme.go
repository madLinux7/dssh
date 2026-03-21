package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

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
			Border(lipgloss.RoundedBorder()).
			BorderBottom(false).
			BorderForeground(purple).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(brightWhite).
				Border(lipgloss.RoundedBorder()).
				BorderBottom(false).
				BorderForeground(purple).
				Padding(0, 2)

	contentStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderTop(false).
			BorderForeground(purple).
			Padding(0, 2)

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
			MarginTop(1).
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

// newConnectionList creates a styled list for connection items.
func newConnectionList(items []list.Item, accentColor lipgloss.Color, width, height int) list.Model {
	delegate := list.NewDefaultDelegate()
	bullet := lipgloss.NormalBorder()
	bullet.Left = "•"
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(accentColor).
		BorderStyle(bullet).
		BorderLeftForeground(accentColor)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(dimGray).
		BorderStyle(bullet).
		BorderLeftForeground(accentColor)
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)

	l := list.New(items, delegate, width, height-4)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	return l
}
