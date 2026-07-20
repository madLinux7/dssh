package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestSplitScreenHasExactGeometryAndPerPaneMargins(t *testing.T) {
	const width, height = 100, 24
	tabBar := renderMainTabBar([]string{"Create", "Connect", "Edit", "Delete"}, TabConnect, width, "SQLite", purple)
	view := renderSplitScreen(tabBar, "LEFT", "RIGHT", "", "", "left hints", "right hints", width, height, purple)
	plain := ansi.Strip(view)
	lines := strings.Split(plain, "\n")
	if len(lines) != height {
		t.Fatalf("line count = %d, want %d", len(lines), height)
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != width {
			t.Fatalf("line %d width = %d, want %d: %q", i, got, width, line)
		}
	}

	separator := width / 2
	if got := []rune(lines[2])[separator]; got != '┬' {
		t.Fatalf("top separator = %q, want ┬", got)
	}
	if got := []rune(lines[height-1])[separator]; got != '┴' {
		t.Fatalf("bottom separator = %q, want ┴", got)
	}
	if got := []rune(lines[3])[separator]; got != '│' {
		t.Fatalf("center separator = %q, want │", got)
	}

	// Row 3 is the one-row vertical padding. Content starts on row 4 and
	// after the outer border plus two columns of horizontal padding.
	if !strings.HasPrefix(lines[4], "│  LEFT") {
		t.Fatalf("left pane padding/content = %q", lines[4])
	}
	if got := string([]rune(lines[4])[separator+1 : separator+8]); got != "  RIGHT" {
		t.Fatalf("right pane padding/content = %q, want %q", got, "  RIGHT")
	}
}

func TestCompositePopoverPreservesBackground(t *testing.T) {
	background := strings.Repeat("background line\n", 9) + "background line"
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Render("Dialog")
	view := compositePopover(background, box, 30, 10)
	plain := ansi.Strip(view)
	if !strings.Contains(plain, "Dialog") || !strings.Contains(plain, "background") {
		t.Fatalf("composite view lost foreground or background:\n%s", plain)
	}
	lines := strings.Split(plain, "\n")
	if len(lines) != 10 {
		t.Fatalf("composite line count = %d, want 10", len(lines))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != 30 {
			t.Fatalf("composite line %d width = %d, want 30", i, got)
		}
	}
}
