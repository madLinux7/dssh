package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/madLinux7/dssh/internal/model"
)

func TestGroupPanePinsShowAllAndAppliesSelectionWhileNavigating(t *testing.T) {
	pane := newGroupPaneModel([]model.GroupWithCount{
		{Group: model.Group{ID: 1, Name: "staging"}, ConnectionCount: 2},
		{Group: model.Group{ID: 2, Name: "Production"}, ConnectionCount: 1},
	}, 40, 16)
	pane.SetActive(true)

	view := ansi.Strip(pane.View())
	noGroups := strings.Index(view, "(No Groups)")
	staging := strings.Index(view, "staging")
	production := strings.Index(view, "Production")
	if noGroups < 0 || staging < 0 || production < 0 {
		t.Fatalf("group pane is missing expected rows:\n%s", view)
	}
	if !(noGroups < staging && staging < production) {
		t.Fatalf("row order is not (No Groups) then descending names:\n%s", view)
	}
	if !strings.Contains(view, "Show all connections") || !strings.Contains(view, "2 connections") || !strings.Contains(view, "1 connection") {
		t.Fatalf("group descriptions missing:\n%s", view)
	}

	pane, _ = pane.Update(tea.KeyMsg{Type: tea.KeyDown})
	if pane.SelectedGroupID() != 1 {
		t.Fatalf("selected group ID = %d, want staging ID 1", pane.SelectedGroupID())
	}
}

func TestGroupPaneKeepsShowAllPinnedWhileSearching(t *testing.T) {
	pane := newGroupPaneModel([]model.GroupWithCount{
		{Group: model.Group{ID: 1, Name: "staging"}},
		{Group: model.Group{ID: 2, Name: "Production"}},
	}, 40, 16)
	pane.SetActive(true)

	for _, r := range "prod" {
		pane, _ = pane.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	view := ansi.Strip(pane.View())
	if !strings.Contains(view, "(No Groups)") || !strings.Contains(view, "Production") {
		t.Fatalf("pinned or matching item missing:\n%s", view)
	}
	if strings.Contains(view, "staging") {
		t.Fatalf("non-matching group remained visible:\n%s", view)
	}
}

func TestGroupPaneEmptyStateHasNoSearchOrList(t *testing.T) {
	pane := newGroupPaneModel(nil, 40, 16)
	pane.SetActive(true)
	view := ansi.Strip(pane.View())
	if !strings.Contains(view, "No existing groups yet") {
		t.Fatalf("empty-state text missing:\n%s", view)
	}
	if strings.Contains(view, "type to search") || strings.Contains(view, "(No Groups)") {
		t.Fatalf("empty pane rendered search/list:\n%s", view)
	}
}
