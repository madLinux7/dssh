package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
)

func TestCreateSaveCommitsConnectionAndGroupAssignmentsTogether(t *testing.T) {
	d := newTUITestDB(t)
	group, err := db.CreateGroup(d, "Production")
	if err != nil {
		t.Fatal(err)
	}
	app := newAppModel(nil, d, TabCreate, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app, _ = app.handleSave(&WizardResult{
		Name:     "api",
		User:     "root",
		Host:     "api.example",
		Port:     "22",
		AuthType: "key",
		SaveTo:   model.SaveTargetSQLite,
		GroupIDs: []int64{group.ID},
	})

	ids, err := db.GroupIDsForConnection(d, model.ConnectionRef{Source: model.SourceSQLite, Name: "api"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != group.ID {
		t.Fatalf("saved group IDs = %v, want [%d]", ids, group.ID)
	}
	if selected := app.createAssignment.SelectedGroupIDs(); len(selected) != 0 {
		t.Fatalf("create draft was not reset after save: %v", selected)
	}
}

func TestEditFormCommitsAssignmentDraftOnSave(t *testing.T) {
	d := newTUITestDB(t)
	oldGroup, _ := db.CreateGroup(d, "Old")
	newGroup, _ := db.CreateGroup(d, "New")
	connection := model.Connection{
		Name: "api", User: "root", Host: "api.example", Port: 22,
		AuthType: model.AuthKey, Source: model.SourceSQLite,
	}
	if err := db.InsertWithGroups(d, &connection, model.ConnectionRef{Source: model.SourceSQLite, Name: "api"}, []int64{oldGroup.ID}); err != nil {
		t.Fatal(err)
	}

	app := newAppModel([]model.Connection{connection}, d, TabEdit, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app = updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 24})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyEnter})
	if !app.editModel.editing {
		t.Fatal("Enter did not open the Edit form")
	}
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	// Groups are descending: Old first, then New. Uncheck Old and check New.
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeySpace})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyDown})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeySpace})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	for range 7 {
		app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyDown})
	}
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyEnter})

	ids, err := db.GroupIDsForConnection(d, model.ConnectionRef{Source: model.SourceSQLite, Name: "api"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != newGroup.ID {
		t.Fatalf("edited group IDs = %v, want [%d]", ids, newGroup.ID)
	}
	if app.editModel.editing {
		t.Fatal("successful edit did not return to list mode")
	}
}

func TestEscapeFromEditAssignmentDiscardsEntireDraft(t *testing.T) {
	d := newTUITestDB(t)
	group, _ := db.CreateGroup(d, "Production")
	connection := model.Connection{
		Name: "api", User: "root", Host: "api.example", Port: 22,
		AuthType: model.AuthKey, Source: model.SourceSQLite,
	}
	if err := db.InsertWithGroups(d, &connection, model.ConnectionRef{Source: model.SourceSQLite, Name: "api"}, []int64{group.ID}); err != nil {
		t.Fatal(err)
	}
	app := newAppModel([]model.Connection{connection}, d, TabEdit, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app = updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 24})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyEnter})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeySpace})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyEsc})

	if app.editModel.editing {
		t.Fatal("Escape from assignment pane did not cancel Edit form")
	}
	ids, err := db.GroupIDsForConnection(d, model.ConnectionRef{Source: model.SourceSQLite, Name: "api"})
	if err != nil || len(ids) != 1 || ids[0] != group.ID {
		t.Fatalf("cancel changed durable assignments: %v, err %v", ids, err)
	}
}

func TestHidingRightPaneDiscardsCreateAssignmentDraft(t *testing.T) {
	d := newTUITestDB(t)
	group, _ := db.CreateGroup(d, "Production")
	app := newAppModel(nil, d, TabCreate, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app = updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 24})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyDown})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeySpace})
	if ids := app.createAssignment.SelectedGroupIDs(); len(ids) != 1 || ids[0] != group.ID {
		t.Fatalf("create draft IDs = %v, want [%d]", ids, group.ID)
	}

	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyCtrlP})
	if ids := app.createAssignment.SelectedGroupIDs(); len(ids) != 0 {
		t.Fatalf("hidden pane kept create assignment draft: %v", ids)
	}
}

func TestHidingRightPaneRestoresPersistedEditAssignments(t *testing.T) {
	d := newTUITestDB(t)
	group, _ := db.CreateGroup(d, "Production")
	connection := model.Connection{
		Name: "api", User: "root", Host: "api.example", Port: 22,
		AuthType: model.AuthKey, Source: model.SourceSQLite,
	}
	if err := db.InsertWithGroups(d, &connection, model.ConnectionRef{Source: model.SourceSQLite, Name: "api"}, []int64{group.ID}); err != nil {
		t.Fatal(err)
	}

	app := newAppModel([]model.Connection{connection}, d, TabEdit, &model.RuntimeConfig{ParseMode: model.ParseModeSQLiteOnly})
	app = updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 24})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyEnter})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyTab})
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeySpace})
	if ids := app.editAssignment.SelectedGroupIDs(); len(ids) != 0 {
		t.Fatalf("edit draft IDs = %v, want no groups", ids)
	}

	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyCtrlP})
	if ids := app.editAssignment.SelectedGroupIDs(); len(ids) != 1 || ids[0] != group.ID {
		t.Fatalf("hidden pane did not restore persisted assignments: %v", ids)
	}
	app = updateApp(t, app, tea.KeyMsg{Type: tea.KeyCtrlS})

	ids, err := db.GroupIDsForConnection(d, model.ConnectionRef{Source: model.SourceSQLite, Name: "api"})
	if err != nil || len(ids) != 1 || ids[0] != group.ID {
		t.Fatalf("save after hiding pane changed durable assignments: %v, err %v", ids, err)
	}
}
