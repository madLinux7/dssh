package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	minimumTerminalWidth  = 80
	minimumTerminalHeight = 20
)

// renderSplitScreen renders the shared tab bar and an exact-height two-pane frame.
func renderSplitScreen(
	tabBar, leftBody, rightBody, leftStatus, rightStatus, leftHints, rightHints string,
	width, height int,
	accent lipgloss.Color,
) string {
	separator := width / 2
	leftFrameWidth := separator - 1
	rightFrameWidth := width - separator - 2
	leftContentWidth := max(1, leftFrameWidth-4)
	rightContentWidth := max(1, rightFrameWidth-4)
	interiorHeight := max(1, height-4)
	contentHeight := max(1, interiorHeight-2)

	leftLines := renderPaneBlock(leftBody, leftStatus, leftHints, leftContentWidth, contentHeight)
	rightLines := renderPaneBlock(rightBody, rightStatus, rightHints, rightContentWidth, contentHeight)

	barLines := strings.Split(tabBar, "\n")
	for len(barLines) < 3 {
		barLines = append(barLines, "")
	}
	baseline := []rune(ansi.Strip(barLines[2]))
	for len(baseline) < width {
		baseline = append(baseline, ' ')
	}
	if len(baseline) > width {
		baseline = baseline[:width]
	}
	baseline[separator] = '┬'
	barLines[2] = lipgloss.NewStyle().Foreground(accent).Render(string(baseline))

	borderStyle := lipgloss.NewStyle().Foreground(accent)
	frameLines := make([]string, 0, interiorHeight+1)
	for row := 0; row < interiorHeight; row++ {
		left := strings.Repeat(" ", leftContentWidth)
		right := strings.Repeat(" ", rightContentWidth)
		if row > 0 && row < interiorHeight-1 {
			contentRow := row - 1
			if contentRow < len(leftLines) {
				left = leftLines[contentRow]
			}
			if contentRow < len(rightLines) {
				right = rightLines[contentRow]
			}
		}
		line := borderStyle.Render("│") +
			"  " + left + "  " +
			borderStyle.Render("│") +
			"  " + right + "  " +
			borderStyle.Render("│")
		frameLines = append(frameLines, line)
	}
	bottom := borderStyle.Render("╰" +
		strings.Repeat("─", leftFrameWidth) +
		"┴" +
		strings.Repeat("─", rightFrameWidth) +
		"╯")
	frameLines = append(frameLines, bottom)

	return strings.Join(append(barLines[:3], frameLines...), "\n")
}

func renderPaneBlock(body, status, hints string, width, height int) []string {
	bodyLines := splitNonEmptyBlock(body)
	statusLines := splitWrappedBlock(status, width)
	hintLines := splitWrappedBlock(hints, width)

	reserved := len(statusLines) + len(hintLines)
	if status != "" && hints != "" {
		reserved++
	}
	bodyCapacity := max(0, height-reserved)
	if len(bodyLines) > bodyCapacity {
		bodyLines = bodyLines[:bodyCapacity]
	}

	lines := make([]string, 0, height)
	lines = append(lines, bodyLines...)
	gap := height - len(bodyLines) - reserved
	for i := 0; i < gap; i++ {
		lines = append(lines, "")
	}
	if status != "" && hints != "" {
		lines = append(lines, "")
	}
	lines = append(lines, statusLines...)
	lines = append(lines, hintLines...)
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = fitANSILine(lines[i], width)
	}
	return lines
}

func splitWrappedBlock(block string, width int) []string {
	if block == "" {
		return nil
	}
	var result []string
	for _, line := range strings.Split(block, "\n") {
		for ansi.StringWidth(line) > width {
			result = append(result, ansi.Cut(line, 0, width))
			line = ansi.Cut(line, width, ansi.StringWidth(line))
		}
		result = append(result, line)
	}
	return result
}

func splitNonEmptyBlock(block string) []string {
	if block == "" {
		return nil
	}
	return strings.Split(block, "\n")
}

func fitANSILine(line string, width int) string {
	line = ansi.Truncate(line, width, "")
	padding := width - ansi.StringWidth(line)
	if padding < 0 {
		padding = 0
	}
	return line + strings.Repeat(" ", padding)
}

// compositePopover overlays a centered dialog on a dimmed copy of the current view.
func compositePopover(background, foreground string, width, height int) string {
	backgroundLines := strings.Split(ansi.Strip(background), "\n")
	for len(backgroundLines) < height {
		backgroundLines = append(backgroundLines, "")
	}
	if len(backgroundLines) > height {
		backgroundLines = backgroundLines[:height]
	}
	for i := range backgroundLines {
		backgroundLines[i] = fitANSILine(backgroundLines[i], width)
	}

	foregroundLines := strings.Split(foreground, "\n")
	foregroundWidth := 0
	for _, line := range foregroundLines {
		foregroundWidth = max(foregroundWidth, ansi.StringWidth(line))
	}
	foregroundWidth = min(foregroundWidth, width)
	foregroundHeight := min(len(foregroundLines), height)
	x := max(0, (width-foregroundWidth)/2)
	y := max(0, (height-foregroundHeight)/2)
	dim := lipgloss.NewStyle().Foreground(dimGray)

	result := make([]string, height)
	for row := 0; row < height; row++ {
		backgroundLine := backgroundLines[row]
		if row < y || row >= y+foregroundHeight {
			result[row] = dim.Render(backgroundLine)
			continue
		}
		foregroundLine := fitANSILine(foregroundLines[row-y], foregroundWidth)
		left := ansi.Cut(backgroundLine, 0, x)
		right := ansi.Cut(backgroundLine, x+foregroundWidth, width)
		result[row] = dim.Render(left) + foregroundLine + dim.Render(right)
	}
	return strings.Join(result, "\n")
}
