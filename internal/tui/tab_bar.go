package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderMainTabBar renders the main tabs and the shared baseline above the
// content frame. The mode label occupies the label row at the far right.
func renderMainTabBar(tabs []string, activeTab Tab, width int, modeLabel string, accentColor lipgloss.Color) string {
	renderedTabs := make([]string, 0, len(tabs))
	for i, tab := range tabs {
		style := inactiveTabStyle.BorderForeground(accentColor)
		border := inactiveTabBorder
		if Tab(i) == activeTab {
			style = activeTabStyle.BorderForeground(accentColor)
			border = activeTabBorder
		}

		// Clean Create tab (first tab; index 0) rendering
		if i == 0 {
			if Tab(i) == activeTab {
				border.BottomLeft = "│"
			} else {
				border.BottomLeft = "├"
			}
			style = style.Border(border)
		}
		renderedTabs = append(renderedTabs, style.Render(tab))
	}

	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	remaining := width - lipgloss.Width(tabRow)
	if remaining <= 0 {
		return tabRow
	}

	modeRow := strings.Repeat(" ", remaining)
	if modeLabel != "" {
		mode := lipgloss.NewStyle().Foreground(dimGray).Render(modeLabel)
		padding := remaining - lipgloss.Width(mode) - 1
		if padding > 0 {
			modeRow = strings.Repeat(" ", padding) + mode + " "
		}
	}

	baseline := strings.Repeat("─", max(0, remaining-1)) + "╮"
	baseline = lipgloss.NewStyle().Foreground(accentColor).Render(baseline)
	remainder := modeRow + "\n" + baseline

	return lipgloss.JoinHorizontal(lipgloss.Bottom, tabRow, remainder)
}
