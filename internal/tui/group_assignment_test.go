package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/madLinux7/dssh/internal/model"
)

func TestAssignmentPaneTogglesMultipleGroupsWithoutCounts(t *testing.T) {
	groups := []model.Group{
		{ID: 1, Name: "staging"},
		{ID: 2, Name: "Production"},
	}
	pane := newGroupAssignmentModel(groups, []int64{1}, 1, 36, 12)
	pane.SetActive(true)

	view := ansi.Strip(pane.View())
	if !strings.Contains(view, "☑ staging") || !strings.Contains(view, "☐ Production") {
		t.Fatalf("assignment markers missing:\n%s", view)
	}
	if strings.Contains(view, "connection") || strings.Contains(view, "(No Groups)") {
		t.Fatalf("assignment pane contains grouping metadata:\n%s", view)
	}

	pane, _ = pane.Update(tea.KeyMsg{Type: tea.KeySpace})
	pane, _ = pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	pane, _ = pane.Update(tea.KeyMsg{Type: tea.KeySpace})
	ids := pane.SelectedGroupIDs()
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("selected group IDs = %v, want [2]", ids)
	}
}

func TestAssignmentPaneEmptyState(t *testing.T) {
	pane := newGroupAssignmentModel(nil, nil, 0, 36, 12)
	view := ansi.Strip(pane.View())
	if !strings.Contains(view, "Assign Groups") || !strings.Contains(view, "No existing groups yet") {
		t.Fatalf("empty assignment pane missing expected content:\n%s", view)
	}
}
