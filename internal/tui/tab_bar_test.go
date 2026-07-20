package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderMainTabBar(t *testing.T) {
	tabs := []string{"Create", "Connect", "Edit", "Delete"}
	got := ansi.Strip(renderMainTabBar(tabs, TabConnect, 60, "both", purple))
	want := strings.Join([]string{
		"╭──────────╮╭───────────╮╭────────╮╭──────────╮             ",
		"│  Create  ││  Connect  ││  Edit  ││  Delete  │        both ",
		"├──────────┴╯           ╰┴────────┴┴──────────┴────────────╮",
	}, "\n")

	if got != want {
		t.Fatalf("unexpected tab bar:\n%s\n\nwant:\n%s", got, want)
	}

	for i, line := range strings.Split(got, "\n") {
		if width := lipgloss.Width(line); width != 60 {
			t.Errorf("line %d width = %d, want 60", i, width)
		}
	}
}

func TestRenderMainTabBarCreateConnectionStaysVerticalWhenActive(t *testing.T) {
	tabs := []string{"Create", "Connect", "Edit", "Delete"}
	got := ansi.Strip(renderMainTabBar(tabs, TabCreate, 60, "", purple))
	bottom := strings.Split(got, "\n")[2]

	if !strings.HasPrefix(bottom, "│          ╰") {
		t.Fatalf("active Create connection = %q, want vertical connection", bottom)
	}
}

func TestRenderMainTabBarAtNarrowWidth(t *testing.T) {
	tabs := []string{"Create", "Connect", "Edit", "Delete"}
	got := ansi.Strip(renderMainTabBar(tabs, TabConnect, 20, "both", purple))
	lines := strings.Split(got, "\n")

	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3", len(lines))
	}
	for i, line := range lines {
		if width := lipgloss.Width(line); width != 47 {
			t.Errorf("line %d width = %d, want intrinsic tab width 47", i, width)
		}
	}
}

func TestRenderMainTabBarGeometryIsIndependentOfAccent(t *testing.T) {
	tabs := []string{"Create", "Connect", "Edit", "Delete"}
	purpleBar := ansi.Strip(renderMainTabBar(tabs, TabDelete, 60, "", purple))
	redBar := ansi.Strip(renderMainTabBar(tabs, TabDelete, 60, "", warnRed))

	if purpleBar != redBar {
		t.Fatal("changing the Delete accent changed tab geometry")
	}
}
