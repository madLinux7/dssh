package tui

import (
	"database/sql"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"

	_ "modernc.org/sqlite"
)

func newTUITestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := db.Initialize(d); err != nil {
		t.Fatal(err)
	}
	return d
}

func typeRunes(t *testing.T, app AppModel, value string) AppModel {
	t.Helper()
	for _, r := range value {
		app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return app
}

func TestGroupCreateDialogCompositesAndCreatesSelectedGroup(t *testing.T) {
	d := newTUITestDB(t)
	app := newAppModel(nil, d, TabConnect, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app = updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 24})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyCtrlN})

	view := ansi.Strip(app.View())
	if !strings.Contains(view, "Create Group") || !strings.Contains(view, "Select Connection") {
		t.Fatalf("dialog was not composited over panes:\n%s", view)
	}

	app = typeRunes(t, app, "Production")
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyEnter})
	groups, err := db.ListGroups(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Name != "Production" {
		t.Fatalf("created groups = %#v", groups)
	}
	if app.groupPane.SelectedGroupID() != groups[0].ID {
		t.Fatalf("selected group = %d, want %d", app.groupPane.SelectedGroupID(), groups[0].ID)
	}
}

func TestCreatingGroupFromAssignmentChecksItButCancelingFormKeepsGroup(t *testing.T) {
	d := newTUITestDB(t)
	app := newAppModel(nil, d, TabCreate, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app = updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 24})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyCtrlN})
	app = typeRunes(t, app, "New Group")
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyEnter})

	ids := app.createAssignment.SelectedGroupIDs()
	if len(ids) != 1 {
		t.Fatalf("draft assignment IDs = %v, want one checked group", ids)
	}
	groups, err := db.ListGroups(d)
	if err != nil || len(groups) != 1 {
		t.Fatalf("durable groups = %#v, err %v", groups, err)
	}
}

func TestCreatingGroupFromAssignmentClearsNonMatchingPreservedGroupSearch(t *testing.T) {
	d := newTUITestDB(t)
	_, _ = db.CreateGroup(d, "Alpha")
	app := newAppModel(nil, d, TabCreate, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app.groupPane.filterBox.SetValue("alpha")
	app.groupPane.applyFilter()

	app = app.commitGroupName(GroupNameResult{
		Name:           "Beta",
		Mode:           GroupNameCreate,
		FromAssignment: true,
	})

	if query := app.groupPane.SearchValue(); query != "" {
		t.Fatalf("preserved group search = %q, want cleared for non-matching created group", query)
	}
}

func TestGroupRenamePreservesIdentityAndDeleteRequiresConfirmation(t *testing.T) {
	d := newTUITestDB(t)
	group, _ := db.CreateGroup(d, "Production")
	app := newAppModel(nil, d, TabConnect, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app = updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 24})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyDown})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyCtrlR})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyCtrlU})
	app = typeRunes(t, app, "Renamed")
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyEnter})

	groups, err := db.ListGroups(d)
	if err != nil || len(groups) != 1 || groups[0].ID != group.ID || groups[0].Name != "Renamed" {
		t.Fatalf("groups after rename = %#v, err %v", groups, err)
	}
	if app.groupPane.SelectedGroupID() != group.ID {
		t.Fatalf("rename lost selected identity: %d", app.groupPane.SelectedGroupID())
	}

	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyCtrlD})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyEsc})
	groups, _ = db.ListGroups(d)
	if len(groups) != 1 {
		t.Fatal("Escape confirmed group deletion")
	}
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyCtrlD})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyEnter})
	groups, _ = db.ListGroups(d)
	if len(groups) != 0 || app.groupPane.SelectedGroupID() != 0 {
		t.Fatalf("delete result groups=%#v selected=%d", groups, app.groupPane.SelectedGroupID())
	}
}
