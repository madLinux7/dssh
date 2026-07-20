package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
)

func (m AppModel) commitGroupName(result GroupNameResult) AppModel {
	if m.database == nil {
		m.groupNameDialog.SetError(fmt.Errorf("database is unavailable"))
		return m
	}

	var (
		group *model.Group
		err   error
	)
	if result.Mode == GroupNameCreate {
		group, err = db.CreateGroup(m.database, result.Name)
	} else {
		err = db.RenameGroup(m.database, result.GroupID, result.Name)
		if err == nil {
			displayName, _, _ := db.NormalizeGroupName(result.Name)
			group = &model.Group{ID: result.GroupID, Name: displayName}
		}
	}
	if err != nil {
		m.groupNameDialog.SetError(err)
		return m
	}

	m.showGroupNameDialog = false
	query := strings.ToLower(m.groupPane.SearchValue())
	if query != "" && !strings.Contains(strings.ToLower(group.Name), query) {
		m.groupPane.ClearSearch()
	}
	m.refreshGroups()
	if result.FromAssignment {
		if m.activeTab == TabCreate {
			m.createAssignment.SelectAndCheck(group.ID)
			m.createAssignmentInitialized = true
		} else {
			m.editAssignment.SelectAndCheck(group.ID)
		}
	} else {
		m.groupPane.SelectGroup(group.ID)
		m.applyGroupFilter()
	}
	verb := "created"
	if result.Mode == GroupNameRename {
		verb = "renamed"
	}
	m.setRightStatus(fmt.Sprintf("Group %q %s", group.Name, verb), successStyle)
	m.applyPaneFocus()
	return m
}

func (m AppModel) commitGroupDelete(group model.Group) AppModel {
	if m.database == nil {
		m.setRightStatus("Error: database is unavailable", modalErrorStyle)
		return m
	}
	if err := db.DeleteGroup(m.database, group.ID); err != nil {
		m.setRightStatus("Error: "+err.Error(), modalErrorStyle)
		return m
	}
	m.refreshGroups()
	m.applyGroupFilter()
	m.setRightStatus(fmt.Sprintf("Group %q deleted", group.Name), successStyle)
	return m
}

func (m AppModel) assignmentMode() bool {
	return m.activeTab == TabCreate || (m.activeTab == TabEdit && m.editModel.editing)
}

func (m *AppModel) applyPaneFocus() {
	m.connectModel.SetActive(false)
	m.createModel.SetActive(false)
	m.editModel.SetActive(false)
	m.deleteModel.SetActive(false)
	m.groupPane.SetActive(false)
	m.createAssignment.SetActive(false)
	m.editAssignment.SetActive(false)

	if m.activePane == PaneLeft {
		switch m.activeTab {
		case TabConnect:
			m.connectModel.SetActive(true)
		case TabCreate:
			m.createModel.SetActive(true)
		case TabEdit:
			m.editModel.SetActive(true)
		case TabDelete:
			m.deleteModel.SetActive(true)
		}
		return
	}

	if !m.assignmentMode() {
		m.groupPane.SetActive(true)
		return
	}
	if m.activeTab == TabCreate {
		m.createAssignment.SetActive(true)
	} else {
		m.editAssignment.SetActive(true)
	}
}

func (m *AppModel) syncNavigationFromActiveList() {
	switch m.activeTab {
	case TabConnect:
		m.connectionQuery = m.connectModel.FilterValue()
		m.selectedConnectionName = m.connectModel.SelectedName()
	case TabEdit:
		if !m.editModel.editing {
			m.connectionQuery = m.editModel.FilterValue()
			m.selectedConnectionName = m.editModel.SelectedName()
		}
	case TabDelete:
		m.connectionQuery = m.deleteModel.FilterValue()
		m.selectedConnectionName = m.deleteModel.SelectedName()
	}
	m.applyNavigationToLists()
}

func (m *AppModel) applyNavigationToLists() {
	m.connectModel.SetFilterValue(m.connectionQuery)
	m.editModel.SetFilterValue(m.connectionQuery)
	m.deleteModel.SetFilterValue(m.connectionQuery)
	m.connectModel.SelectByName(m.selectedConnectionName)
	m.editModel.SelectByName(m.selectedConnectionName)
	m.deleteModel.SelectByName(m.selectedConnectionName)
}

func (m *AppModel) applyGroupFilter() {
	var names map[string]bool
	if groupID := m.groupPane.SelectedGroupID(); groupID != 0 && m.database != nil {
		source, sourcePath := m.activeMembershipScope()
		connectionNames, err := db.ConnectionNamesForGroup(m.database, groupID, source, sourcePath)
		if err != nil {
			m.setError("%s", err)
		} else {
			names = make(map[string]bool, len(connectionNames))
			for _, name := range connectionNames {
				names[name] = true
			}
		}
	}
	m.connectModel.SetGroupNames(names)
	m.editModel.SetGroupNames(names)
	m.deleteModel.SetGroupNames(names)
	m.applyNavigationToLists()
	m.captureVisibleSelection()
}

func (m *AppModel) captureVisibleSelection() {
	var selected string
	switch m.activeTab {
	case TabConnect:
		selected = m.connectModel.SelectedName()
	case TabEdit:
		if !m.editModel.editing {
			selected = m.editModel.SelectedName()
		}
	case TabDelete:
		selected = m.deleteModel.SelectedName()
	}
	if selected != "" || !m.assignmentMode() {
		m.selectedConnectionName = selected
	}
	m.connectModel.SelectByName(selected)
	m.editModel.SelectByName(selected)
	m.deleteModel.SelectByName(selected)
}

func (m *AppModel) refreshGroups() {
	if m.database == nil {
		return
	}
	source, sourcePath := m.activeMembershipScope()
	groups, err := db.ListGroupsWithCounts(m.database, source, sourcePath)
	if err != nil {
		m.setError("%s", err)
		return
	}
	m.groupPane.SetGroups(groups)
	m.groups = make([]model.Group, len(groups))
	for i, group := range groups {
		m.groups[i] = group.Group
	}
	m.createAssignment.SetGroups(m.groups)
	m.editAssignment.SetGroups(m.groups)
}

func (m *AppModel) reconcileMemberships() {
	if m.database == nil {
		return
	}
	mode := model.ParseModeSQLiteOnly
	if m.cfg != nil {
		mode = m.cfg.ParseMode
	}
	if mode != model.ParseModeSSHConfigOnly {
		sqliteNames := make([]string, 0, len(m.sqliteConns))
		for _, connection := range m.sqliteConns {
			sqliteNames = append(sqliteNames, connection.Name)
		}
		if err := db.ReconcileConnectionMemberships(m.database, model.SourceSQLite, "", sqliteNames); err != nil {
			m.setError("reconcile SQLite group assignments: %s", err)
		}
	}

	if mode == model.ParseModeSSHConfigOnly || mode == model.ParseModeBoth {
		sshNames := make([]string, 0, len(m.sshConfigConns))
		for _, connection := range m.sshConfigConns {
			sshNames = append(sshNames, connection.Name)
		}
		if err := db.ReconcileConnectionMemberships(m.database, model.SourceSSHConfig, m.normalizedSSHConfigPath(), sshNames); err != nil {
			m.setError("reconcile ssh_config group assignments: %s", err)
		}
	}
}

func (m AppModel) activeMembershipScope() (model.Source, string) {
	if m.activeSource == model.SourceSSHConfig {
		return model.SourceSSHConfig, m.normalizedSSHConfigPath()
	}
	return model.SourceSQLite, ""
}

func (m AppModel) connectionRef(connection model.Connection) model.ConnectionRef {
	ref := model.ConnectionRef{Source: connection.Source, Name: connection.Name}
	if ref.Source == "" {
		ref.Source = m.activeSource
	}
	if ref.Source == model.SourceSSHConfig {
		ref.SourcePath = m.normalizedSSHConfigPath()
	}
	return ref
}

func (m AppModel) normalizedSSHConfigPath() string {
	path, err := m.sshConfigPath()
	if err != nil {
		return ""
	}
	path = expandTildeTUI(path)
	absolute, err := filepath.Abs(path)
	if err == nil {
		path = absolute
	}
	return filepath.Clean(path)
}
