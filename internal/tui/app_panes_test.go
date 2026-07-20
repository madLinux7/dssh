package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
)

func updateApp(t *testing.T, app AppModel, msg tea.Msg) AppModel {
	t.Helper()
	updated, _ := app.Update(msg)
	next, ok := updated.(AppModel)
	if !ok {
		t.Fatalf("updated model type = %T, want AppModel", updated)
	}
	return next
}

func TestGroupNavigationFiltersConnectionsAndTextSearchIntersects(t *testing.T) {
	d := newTUITestDB(t)
	group, err := db.CreateGroup(d, "Production")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetConnectionGroups(d, model.ConnectionRef{Source: model.SourceSQLite, Name: "beta"}, []int64{group.ID}); err != nil {
		t.Fatal(err)
	}
	connections := []model.Connection{
		{Name: "alpha", User: "root", Host: "alpha.example", Port: 22, Source: model.SourceSQLite},
		{Name: "beta", User: "root", Host: "beta.example", Port: 22, Source: model.SourceSQLite},
	}
	app := newAppModel(connections, d, TabConnect, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app = updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 24})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyDown})
	if got := app.connectModel.SelectedName(); got != "beta" {
		t.Fatalf("group-filtered selection = %q, want beta", got)
	}

	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	app = typeRunes(t, app, "alpha")
	if got := app.connectModel.SelectedName(); got != "" {
		t.Fatalf("intersection unexpectedly selected %q", got)
	}
	if view := ansi.Strip(app.View()); !strings.Contains(view, "No matching connections") {
		t.Fatalf("empty intersection message missing:\n%s", view)
	}
	selectedGroup, ok := app.groupPane.SelectedGroup()
	if !ok || selectedGroup.ConnectionCount != 1 {
		t.Fatalf("connection search changed group count: %#v", selectedGroup)
	}
}

func TestBackendSwitchPreservesGroupFilterAndRecalculatesCounts(t *testing.T) {
	d := newTUITestDB(t)
	group, _ := db.CreateGroup(d, "Production")
	sshPath := filepath.Clean("/configs/a")
	_ = db.SetConnectionGroups(d, model.ConnectionRef{Source: model.SourceSQLite, Name: "sqlite-api"}, []int64{group.ID})
	_ = db.SetConnectionGroups(d, model.ConnectionRef{Source: model.SourceSSHConfig, SourcePath: sshPath, Name: "ssh-api"}, []int64{group.ID})
	connections := []model.Connection{
		{Name: "sqlite-api", User: "root", Host: "sqlite.example", Port: 22, Source: model.SourceSQLite},
		{Name: "ssh-api", User: "root", Host: "ssh.example", Port: 22, Source: model.SourceSSHConfig},
	}
	cfg := &model.RuntimeConfig{ParseMode: model.ParseModeBoth, BothViewMode: model.SourceSQLite, SSHConfigDest: sshPath}
	app := newAppModel(connections, d, TabConnect, cfg)
	app = updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 24})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyDown})
	if app.connectModel.SelectedName() != "sqlite-api" {
		t.Fatalf("SQLite group selection = %q", app.connectModel.SelectedName())
	}

	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyCtrlL})
	if app.activeSource != model.SourceSSHConfig || app.groupPane.SelectedGroupID() != group.ID {
		t.Fatalf("backend/group after toggle = %q/%d", app.activeSource, app.groupPane.SelectedGroupID())
	}
	if app.connectModel.SelectedName() != "ssh-api" {
		t.Fatalf("ssh_config group selection = %q", app.connectModel.SelectedName())
	}
	selected, ok := app.groupPane.SelectedGroup()
	if !ok || selected.ConnectionCount != 1 {
		t.Fatalf("ssh_config count = %#v", selected)
	}
}

func TestStartupReconciliationOnlyTouchesConfiguredSources(t *testing.T) {
	d := newTUITestDB(t)
	group, _ := db.CreateGroup(d, "Production")
	sqliteRef := model.ConnectionRef{Source: model.SourceSQLite, Name: "sqlite-api"}
	sshPath := filepath.Clean("/configs/a")
	sshRef := model.ConnectionRef{Source: model.SourceSSHConfig, SourcePath: sshPath, Name: "removed-ssh-host"}
	if err := db.SetConnectionGroups(d, sqliteRef, []int64{group.ID}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetConnectionGroups(d, sshRef, []int64{group.ID}); err != nil {
		t.Fatal(err)
	}

	_ = newAppModel(nil, d, TabConnect, &model.RuntimeConfig{
		ParseMode:     model.ParseModeSSHConfigOnly,
		SSHConfigDest: sshPath,
	})
	ids, err := db.GroupIDsForConnection(d, sqliteRef)
	if err != nil || len(ids) != 1 || ids[0] != group.ID {
		t.Fatalf("ssh_config-only startup changed SQLite assignments: %v, err %v", ids, err)
	}
	if err := db.SetConnectionGroups(d, sshRef, []int64{group.ID}); err != nil {
		t.Fatal(err)
	}

	_ = newAppModel([]model.Connection{{
		Name: "sqlite-api", Source: model.SourceSQLite,
	}}, d, TabConnect, &model.RuntimeConfig{
		ParseMode:     model.ParseModeBoth,
		BothViewMode:  model.SourceSQLite,
		SSHConfigDest: sshPath,
	})
	ids, err = db.GroupIDsForConnection(d, sshRef)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("both-mode startup kept stale current-path ssh_config assignments: %v", ids)
	}
}

func TestTabSwitchesPanesAndConnectionNavigationIsSharedAcrossTabs(t *testing.T) {
	connections := []model.Connection{
		{Name: "alpha", User: "root", Host: "alpha.example", Port: 22, Source: model.SourceSQLite},
		{Name: "beta", User: "root", Host: "beta.example", Port: 22, Source: model.SourceSQLite},
	}
	app := newAppModel(connections, nil, TabConnect, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app = updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 24})

	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bet")})
	if app.connectionQuery != "bet" || app.connectModel.SelectedName() != "beta" {
		t.Fatalf("Connect navigation state = query %q selected %q", app.connectionQuery, app.connectModel.SelectedName())
	}

	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	if app.activePane != PaneRight || app.activeTab != TabConnect {
		t.Fatalf("Tab produced pane=%v tab=%v, want right/Connect", app.activePane, app.activeTab)
	}
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyRight})
	if app.activeTab != TabEdit || app.activePane != PaneRight {
		t.Fatalf("Right produced pane=%v tab=%v, want right/Edit", app.activePane, app.activeTab)
	}
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	if app.editModel.FilterValue() != "bet" || app.editModel.SelectedName() != "beta" {
		t.Fatalf("Edit did not restore shared navigation: query %q selected %q", app.editModel.FilterValue(), app.editModel.SelectedName())
	}
}

func TestAppRendersTwoPaneFrameAndAssignmentMode(t *testing.T) {
	app := newAppModel(nil, nil, TabCreate, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app = updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 24})
	plain := ansi.Strip(app.View())
	if !strings.Contains(plain, "New Connection") || !strings.Contains(plain, "Assign Groups") {
		t.Fatalf("Create panes missing:\n%s", plain)
	}
	lines := strings.Split(plain, "\n")
	if len(lines) != 24 {
		t.Fatalf("view height = %d, want 24", len(lines))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != 100 {
			t.Fatalf("line %d width = %d, want 100", i, got)
		}
	}
}

func TestMinimumSupportedSizeKeepsFormSaveAndBottomHintsVisible(t *testing.T) {
	app := newAppModel(nil, nil, TabCreate, &model.RuntimeConfig{ParseMode: model.ParseModeBoth})
	app = updateApp(t, app, tea.WindowSizeMsg{Width: minimumTerminalWidth, Height: minimumTerminalHeight})
	plain := ansi.Strip(app.View())
	if !strings.Contains(plain, "[ Save ]") || !strings.Contains(plain, "CTRL+N new") {
		t.Fatalf("minimum-size form clipped controls or hints:\n%s", plain)
	}
}

func TestPassphraseDialogIsCompositedOverCurrentPanes(t *testing.T) {
	app := newAppModel(nil, nil, TabCreate, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app = updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 24})
	app.modal = newPassphraseModal(true, 100, 24)
	app.showModal = true
	plain := ansi.Strip(app.View())
	if !strings.Contains(plain, "Create Master Passphrase") || !strings.Contains(plain, "New Connection") || !strings.Contains(plain, "Assign Groups") {
		t.Fatalf("passphrase dialog did not preserve pane background:\n%s", plain)
	}
}

func TestOpeningCreateFromGroupContextPositionsButDoesNotCheckAssignment(t *testing.T) {
	d := newTUITestDB(t)
	group, _ := db.CreateGroup(d, "Production")
	app := newAppModel(nil, d, TabConnect, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app = updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 24})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyDown})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyLeft})
	if app.activeTab != TabCreate || app.activePane != PaneRight {
		t.Fatalf("navigation landed on tab=%v pane=%v", app.activeTab, app.activePane)
	}
	if app.createAssignment.CursorGroupID() != group.ID {
		t.Fatalf("assignment cursor = %d, want %d", app.createAssignment.CursorGroupID(), group.ID)
	}
	if ids := app.createAssignment.SelectedGroupIDs(); len(ids) != 0 {
		t.Fatalf("group context implicitly assigned new connection: %v", ids)
	}
}
