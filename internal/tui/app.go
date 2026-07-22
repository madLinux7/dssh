// Package tui implements the interactive terminal UI using Bubble Tea.
//
// Architecture: The TUI follows Bubble Tea's Elm-like pattern (Model → Update → View).
// AppModel is the top-level model that manages four tab sub-models, shared
// connection/group navigation, assignment drafts, and composited dialogs. Each
// sub-model owns its local controls while AppModel coordinates panes, persistence,
// and result propagation back to the CLI layer.
package tui

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/madLinux7/dssh/internal/crypto"
	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
	"github.com/madLinux7/dssh/internal/sshconfig"
)

// Tab identifies a tab in the TUI.
type Tab int

const (
	TabCreate Tab = iota
	TabConnect
	TabEdit
	TabDelete
)

type Pane int

const (
	PaneLeft Pane = iota
	PaneRight
)

const settingShowHints = "tui_show_hints"

// AppModel is the top-level Bubble Tea model.
type AppModel struct {
	activeTab      Tab
	tabs           []string
	connectModel   ConnectModel
	createModel    CreateModel
	editModel      EditModel
	deleteModel    DeleteModel
	database       *sql.DB
	cfg            *model.RuntimeConfig
	result         *AppResult
	statusMsg      string
	statusMsgStyle lipgloss.Style
	statusMsgRight bool // render status right-aligned
	width          int
	height         int
	activePane     Pane
	showHints      bool

	// Session-level navigation shared by the connection tabs.
	connectionQuery        string
	selectedConnectionName string

	// Group filtering and per-form assignment drafts.
	groups                      []model.Group
	groupPane                   GroupPaneModel
	createAssignment            GroupAssignmentModel
	editAssignment              GroupAssignmentModel
	createAssignmentInitialized bool

	// Dual-list state for both/separate mode.
	activeSource   model.Source // current visible source in separate mode
	sqliteConns    []model.Connection
	sshConfigConns []model.Connection

	// Passphrase modal state.
	showModal     bool
	modal         PassphraseModal
	pendingWizard *WizardResult
	pendingEditID int64 // non-zero when edit password save is pending

	showGroupNameDialog bool
	groupNameDialog     GroupNameDialog
	showGroupDelete     bool
	groupDeleteDialog   GroupDeleteDialog
}

// Run launches the TUI and returns the user's action.
func Run(connections []model.Connection, d *sql.DB, initialTab Tab, cfg *model.RuntimeConfig) *AppResult {
	m := newAppModel(connections, d, initialTab, cfg)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return &AppResult{Action: ActionNone}
	}

	if fm, ok := finalModel.(AppModel); ok && fm.result != nil {
		return fm.result
	}
	return &AppResult{Action: ActionNone}
}

func newAppModel(connections []model.Connection, d *sql.DB, initialTab Tab, cfg *model.RuntimeConfig) AppModel {
	// Sort connections ascending (A→Z).
	sort.Slice(connections, func(i, j int) bool {
		return strings.ToLower(connections[i].Name) < strings.ToLower(connections[j].Name)
	})

	connItems := make([]connectionItem, len(connections))
	for i, c := range connections {
		connItems[i] = connectionItem{conn: c}
	}

	m := AppModel{
		activeTab:                   initialTab,
		tabs:                        []string{"Create", "Connect", "Edit", "Delete"},
		connectModel:                newConnectModel(connections, 80, 20),
		createModel:                 newCreateModel(80, 20),
		editModel:                   newEditModel(connItems, 80, 20),
		deleteModel:                 newDeleteModel(connItems, d, 80, 20),
		database:                    d,
		cfg:                         cfg,
		activeSource:                model.SourceSQLite,
		activePane:                  PaneLeft,
		showHints:                   true,
		groupPane:                   newGroupPaneModel(nil, 34, 12),
		createAssignment:            newGroupAssignmentModel(nil, nil, 0, 34, 12),
		editAssignment:              newGroupAssignmentModel(nil, nil, 0, 34, 12),
		createAssignmentInitialized: initialTab == TabCreate,
	}
	
	if d != nil {
		if value, err := db.GetSetting(d, settingShowHints); err == nil && string(value) == "false" {
			m.showHints = false
		}
	}

	// Respect persisted view mode.
	if cfg != nil && cfg.BothViewMode == model.SourceSSHConfig {
		m.activeSource = model.SourceSSHConfig
	}
	if cfg != nil && cfg.ParseMode == model.ParseModeSSHConfigOnly {
		m.activeSource = model.SourceSSHConfig
	}

	// Pass config to sub-models that need it.
	m.createModel.cfg = cfg
	if cfg != nil {
		m.createModel.saveTo = cfg.DefaultSaveTarget
	}
	if sshPath, err := m.sshConfigPath(); err == nil {
		m.deleteModel.sshConfigDest = sshPath
	}

	// In "both" mode, split connections by source for CTRL+L toggling
	// Show only the active source's connections.
	if cfg != nil && cfg.ParseMode == model.ParseModeBoth {
		for _, c := range connections {
			switch c.Source {
			case model.SourceSSHConfig:
				m.sshConfigConns = append(m.sshConfigConns, c)
			default:
				m.sqliteConns = append(m.sqliteConns, c)
			}
		}
		// Filter sub-models to only show the active source.
		var visible []model.Connection
		if m.activeSource == model.SourceSSHConfig {
			visible = m.sshConfigConns
		} else {
			visible = m.sqliteConns
		}
		visibleItems := make([]connectionItem, len(visible))
		for i, c := range visible {
			visibleItems[i] = connectionItem{conn: c}
		}
		m.connectModel.SetItems(visible)
		m.editModel.SetItems(visibleItems)
		m.deleteModel.SetItems(visibleItems)
	} else {
		for _, connection := range connections {
			if connection.Source == model.SourceSSHConfig || (cfg != nil && cfg.ParseMode == model.ParseModeSSHConfigOnly) {
				m.sshConfigConns = append(m.sshConfigConns, connection)
			} else {
				m.sqliteConns = append(m.sqliteConns, connection)
			}
		}
	}

	m.reconcileMemberships()
	m.refreshGroups()
	m.applyGroupFilter()
	m.applyPaneFocus()
	return m
}

func (m AppModel) Init() tea.Cmd {
	return nil
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		separator := m.width / 2
		leftContentWidth := max(1, separator-5)
		rightContentWidth := max(1, m.width-separator-6)
		paneHeight := max(1, m.height-6)
		paneBodyHeight := max(1, paneHeight-2)
		m.connectModel.SetSize(leftContentWidth, paneBodyHeight)
		m.createModel.SetSize(leftContentWidth, paneBodyHeight)
		m.editModel.SetSize(leftContentWidth, paneBodyHeight)
		m.deleteModel.SetSize(leftContentWidth, paneBodyHeight)
		m.groupPane.SetSize(rightContentWidth, paneBodyHeight)
		m.createAssignment.SetSize(rightContentWidth, paneBodyHeight)
		m.editAssignment.SetSize(rightContentWidth, paneBodyHeight)
		m.modal.SetSize(m.width, m.height)
		return m, nil

	case tea.KeyMsg:
		if m.showGroupNameDialog {
			var result *GroupNameResult
			var cmd tea.Cmd
			m.groupNameDialog, result, cmd = m.groupNameDialog.Update(msg)
			if result == nil {
				return m, cmd
			}
			if result.Cancelled {
				m.showGroupNameDialog = false
				m.applyPaneFocus()
				return m, nil
			}
			m = m.commitGroupName(*result)
			return m, nil
		}
		if m.showGroupDelete {
			var result *GroupDeleteResult
			m.groupDeleteDialog, result = m.groupDeleteDialog.Update(msg)
			if result == nil {
				return m, nil
			}
			m.showGroupDelete = false
			if result.Confirmed {
				m = m.commitGroupDelete(m.groupDeleteDialog.group)
			}
			m.applyPaneFocus()
			return m, nil
		}
		// When modal is active, delegate everything to the modal.
		if m.showModal {
			var result *PassphraseResult
			var cmd tea.Cmd
			m.modal, result, cmd = m.modal.Update(msg)
			if result != nil {
				if result.Cancelled {
					m.showModal = false
					m.pendingWizard = nil
					if m.pendingEditID != 0 {
						m.pendingEditID = 0
					}
					m.applyPaneFocus()
					return m, nil
				}
				// Passphrase entered — finalize the save.
				m.showModal = false
				if m.pendingEditID != 0 {
					m = m.finalizeEditPasswordSave(result.Passphrase)
				} else {
					m = m.finalizePasswordSave(result.Passphrase)
				}
				return m, nil
			}
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c":
			m.result = &AppResult{Action: ActionNone}
			return m, tea.Quit
		case "?":
			m = m.toggleHints()
			return m, nil
		case "ctrl+s":
			switch {
			case m.activeTab == TabCreate:
				return m.saveCreateForm()
			case m.activeTab == TabEdit && m.editModel.editing:
				return m.saveEditForm(), nil
			}
		case "ctrl+l":
			if m.cfg != nil && m.cfg.ParseMode == model.ParseModeBoth && !m.editModel.editing {
				m = m.toggleSource()
				return m, nil
			}
		case "ctrl+n":
			if m.activePane == PaneRight {
				m.groupNameDialog = newGroupNameDialog(GroupNameCreate, model.Group{}, m.assignmentMode())
				m.showGroupNameDialog = true
				return m, nil
			}
		case "ctrl+r":
			if m.activePane == PaneRight {
				if !m.assignmentMode() {
					if group, ok := m.groupPane.SelectedGroup(); ok {
						m.groupNameDialog = newGroupNameDialog(GroupNameRename, group.Group, false)
						m.showGroupNameDialog = true
					}
				}
				return m, nil
			}
		case "ctrl+d":
			if m.activePane == PaneRight {
				if !m.assignmentMode() {
					if group, ok := m.groupPane.SelectedGroup(); ok {
						m.groupDeleteDialog = newGroupDeleteDialog(group.Group)
						m.showGroupDelete = true
					}
				}
				return m, nil
			}
		case "tab", "shift+tab":
			if m.activePane == PaneLeft {
				m.activePane = PaneRight
			} else {
				m.activePane = PaneLeft
			}
			m.deleteModel.ResetConfirm()
			m.applyPaneFocus()
			return m, nil
		case "esc":
			if m.activePane == PaneRight && m.assignmentMode() {
				if m.activeTab == TabEdit {
					m.editModel.editing = false
					m.editAssignment.Begin(m.groups, nil, m.groupPane.SelectedGroupID())
					m.applyPaneFocus()
					return m, nil
				}
				m.result = &AppResult{Action: ActionNone}
				return m, tea.Quit
			}
			if m.activePane == PaneRight && !m.assignmentMode() {
				if m.groupPane.SearchValue() != "" {
					m.groupPane.ClearSearch()
					m.applyGroupFilter()
					return m, nil
				}
				m.result = &AppResult{Action: ActionNone}
				return m, tea.Quit
			}
		case "left":
			if m.canArrowSwitchTab() {
				m = m.switchTabPrev()
				return m, nil
			}
		case "right":
			if m.canArrowSwitchTab() {
				m = m.switchTabNext()
				return m, nil
			}
		}
	}

	if m.activePane == PaneRight {
		var cmd tea.Cmd
		if m.assignmentMode() {
			if m.activeTab == TabCreate {
				m.createAssignment, cmd = m.createAssignment.Update(msg)
			} else {
				m.editAssignment, cmd = m.editAssignment.Update(msg)
			}
			return m, cmd
		}
		previousGroupID := m.groupPane.SelectedGroupID()
		m.groupPane, cmd = m.groupPane.Update(msg)
		if m.groupPane.SelectedGroupID() != previousGroupID {
			m.deleteModel.ResetConfirm()
			m.applyGroupFilter()
		}
		return m, cmd
	}

	// Delegate to active sub-model.
	var cmd tea.Cmd
	switch m.activeTab {
	case TabConnect:
		var result *AppResult
		m.connectModel, result, cmd = m.connectModel.Update(msg)
		if result != nil {
			m.result = result
			return m, tea.Quit
		}
		m.syncNavigationFromActiveList()
	case TabCreate:
		var result *AppResult
		m.createModel, result, cmd = m.createModel.Update(msg)
		if result != nil {
			switch result.Action {
			case ActionNone:
				m.result = result
				return m, tea.Quit
			case ActionCreated:
				result.WizardResult.GroupIDs = m.createAssignment.SelectedGroupIDs()
				var quitCmd tea.Cmd
				m, quitCmd = m.handleSave(result.WizardResult)
				if quitCmd != nil {
					return m, quitCmd
				}
			}
		}
	case TabEdit:
		wasEditing := m.editModel.editing
		var result *AppResult
		m.editModel, result, cmd = m.editModel.Update(msg)
		if !wasEditing && m.editModel.editing {
			var ids []int64
			if m.database != nil {
				var err error
				ids, err = db.GroupIDsForConnection(m.database, m.connectionRef(m.editModel.origConn))
				if err != nil {
					m.setError("%s", err)
					ids = nil
				}
			}
			m.editAssignment.Begin(m.groups, ids, m.groupPane.SelectedGroupID())
			m.applyPaneFocus()
		}
		if wasEditing && !m.editModel.editing && result == nil {
			m.editAssignment.Begin(m.groups, nil, m.groupPane.SelectedGroupID())
			m.applyPaneFocus()
		}
		if result != nil {
			switch result.Action {
			case ActionNone:
				m.result = result
				return m, tea.Quit
			case ActionEdited:
				result.WizardResult.GroupIDs = m.editAssignment.SelectedGroupIDs()
				m = m.handleEditSave(result)
			}
		}
		if !m.editModel.editing {
			m.syncNavigationFromActiveList()
		}
	case TabDelete:
		var result *AppResult
		m.deleteModel, result, cmd = m.deleteModel.Update(msg)
		if m.deleteModel.lastDeleted != "" {
			name := m.deleteModel.lastDeleted
			if m.database != nil {
				_, sourcePath := m.activeMembershipScope()
				_ = db.DeleteConnectionMemberships(m.database, model.ConnectionRef{Source: m.activeSource, SourcePath: sourcePath, Name: name})
			}
			m.connectModel.RemoveByName(name)
			m.editModel.RemoveByName(name)
			if m.activeSource == model.SourceSSHConfig {
				m.sshConfigConns = removeConnByName(m.sshConfigConns, name)
			} else {
				m.sqliteConns = removeConnByName(m.sqliteConns, name)
			}
			m.deleteModel.lastDeleted = ""
			m.setStatus(fmt.Sprintf("%q deleted", name), successStyle)
			m.refreshGroups()
			m.applyGroupFilter()
		}
		// Clear app-level status when delete tab shows its own confirmation text.
		if m.deleteModel.statusMsg != "" && m.statusMsg != "" {
			m.statusMsg = ""
		}
		if result != nil {
			m.result = result
			return m, tea.Quit
		}
		m.syncNavigationFromActiveList()
	}

	return m, cmd
}

func (m AppModel) saveCreateForm() (AppModel, tea.Cmd) {
	m.createModel.atSave = true
	var result *AppResult
	var cmd tea.Cmd
	m.createModel, result, cmd = m.createModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if result == nil || result.Action != ActionCreated {
		return m, cmd
	}
	result.WizardResult.GroupIDs = m.createAssignment.SelectedGroupIDs()
	return m.handleSave(result.WizardResult)
}

func (m AppModel) saveEditForm() AppModel {
	var result *AppResult
	m.editModel, result, _ = m.editModel.handleSave()
	if result == nil || result.Action != ActionEdited {
		return m
	}
	result.WizardResult.GroupIDs = m.editAssignment.SelectedGroupIDs()
	return m.handleEditSave(result)
}

// switchTabNext advances the active tab (wraps around) and clears transient state.
func (m AppModel) switchTabNext() AppModel {
	m.activeTab = Tab((int(m.activeTab) + 1) % len(m.tabs))
	m.statusMsg = ""
	m.deleteModel.ResetConfirm()
	m.prepareActiveTab()
	m.applyNavigationToLists()
	m.applyPaneFocus()
	return m
}

// switchTabPrev moves to the previous tab (wraps around) and clears transient state.
func (m AppModel) switchTabPrev() AppModel {
	m.activeTab = Tab((int(m.activeTab) - 1 + len(m.tabs)) % len(m.tabs))
	m.statusMsg = ""
	m.deleteModel.ResetConfirm()
	m.prepareActiveTab()
	m.applyNavigationToLists()
	m.applyPaneFocus()
	return m
}

func (m *AppModel) prepareActiveTab() {
	if m.activeTab == TabCreate && !m.createAssignmentInitialized {
		m.createAssignment.SelectCursor(m.groupPane.SelectedGroupID())
		m.createAssignmentInitialized = true
	}
}

// canArrowSwitchTab reports whether Left/Right should switch tabs.
// In Connect/Delete/Edit-list: always. In forms: only when on the Save button
// or when the currently focused text field is empty. Save-To toggle (Create)
// is handled by the sub-model, so we return false there.
func (m AppModel) canArrowSwitchTab() bool {
	if m.activePane == PaneRight {
		return true
	}
	switch m.activeTab {
	case TabConnect, TabDelete:
		return true
	case TabEdit:
		if !m.editModel.editing {
			return true
		}
		if m.editModel.atSave {
			return true
		}
		return m.editModel.inputs[m.editModel.focused].Value() == ""
	case TabCreate:
		if m.createModel.atSave {
			return true
		}
		if m.createModel.atSaveTo {
			return false
		}
		return m.createModel.inputs[m.createModel.focused].Value() == ""
	}
	return false
}

// handleSave validates and saves a new connection.
// For key auth: saves directly, stays in TUI.
// For password auth: exits TUI so CLI can handle passphrase prompt.
func (m AppModel) handleSave(wr *WizardResult) (AppModel, tea.Cmd) {
	if wr.User == "" {
		wr.User = "root"
	}
	if wr.Port == "" {
		wr.Port = "22"
	}
	if wr.IdentityFile == "" {
		wr.IdentityFile = "default"
	}

	if wr.Name == "" || wr.Host == "" {
		m.setError("name and host are required")
		return m, nil
	}

	if err := model.ValidateName(wr.Name); err != nil {
		m.setError("%s", err)
		return m, nil
	}

	port, err := strconv.Atoi(wr.Port)
	if err != nil || port < 1 || port > 65535 {
		m.setError("invalid port: %s", wr.Port)
		return m, nil
	}

	// Password auth — show passphrase modal instead of saving directly.
	// The modal collects the master passphrase, then finalizePasswordSave
	// encrypts the SSH password and persists the connection.
	if wr.AuthType == "password" {
		salt, err := db.GetSetting(m.database, "argon2_salt")
		if err != nil {
			m.setError("%s", err)
			return m, nil
		}
		isNew := salt == nil
		m.modal = newPassphraseModal(isNew, m.width, m.height)
		m.showModal = true
		m.pendingWizard = wr
		return m, nil
	}

	// Key auth — save directly in TUI.
	conn := &model.Connection{
		Name:      wr.Name,
		User:      wr.User,
		Host:      wr.Host,
		Port:      port,
		Directory: wr.Directory,
		AuthType:  model.AuthKey,
	}
	if wr.IdentityFile != "default" {
		conn.IdentityFile = expandTildeTUI(wr.IdentityFile)
	}

	if err := m.insertConnection(conn, wr.SaveTo, wr.GroupIDs); err != nil {
		m.setError("%s", err)
		return m, nil
	}

	m.onConnectionSaved(conn.Name, wr.SaveTo)
	return m, nil
}

// insertConnection saves to the appropriate backend based on the save target.
func (m AppModel) insertConnection(conn *model.Connection, target model.SaveTarget, groupIDs []int64) error {
	saveToSSH := target == model.SaveTargetSSHConfig ||
		(m.cfg != nil && m.cfg.ParseMode == model.ParseModeSSHConfigOnly)

	if saveToSSH {
		p, err := m.sshConfigPath()
		if err != nil {
			return err
		}
		conn.Source = model.SourceSSHConfig
		if err := sshconfig.Insert(p, conn); err != nil {
			return err
		}
		ref := model.ConnectionRef{Source: model.SourceSSHConfig, SourcePath: m.normalizedSSHConfigPath(), Name: conn.Name}
		if err := db.SetConnectionGroups(m.database, ref, groupIDs); err != nil {
			_ = sshconfig.Delete(p, conn.Name)
			return err
		}
		return nil
	}
	conn.Source = model.SourceSQLite
	ref := model.ConnectionRef{Source: model.SourceSQLite, Name: conn.Name}
	return db.InsertWithGroups(m.database, conn, ref, groupIDs)
}

// sshConfigPath returns the ssh_config file path based on the current config.
func (m AppModel) sshConfigPath() (string, error) {
	if m.cfg == nil || m.cfg.SSHConfigDest == "" {
		return sshconfig.MainFilePath()
	}
	return m.cfg.SSHConfigDest, nil
}

// onConnectionSaved updates the Connect/Delete lists and resets the New form
// after a connection is successfully persisted.
func (m *AppModel) onConnectionSaved(name string, target model.SaveTarget) {
	// Update default save target so the next create defaults to what was just used.
	if m.cfg != nil && m.cfg.ParseMode == model.ParseModeBoth {
		m.cfg.DefaultSaveTarget = target
		if m.database != nil {
			_ = db.SetSetting(m.database, "parse_both_default_save_target", []byte(target))
		}
	}

	var saved *model.Connection

	saveToSSH := target == model.SaveTargetSSHConfig ||
		(m.cfg != nil && m.cfg.ParseMode == model.ParseModeSSHConfigOnly)

	if saveToSSH {
		p, _ := m.sshConfigPath()
		saved, _ = sshconfig.GetByName(p, name)
	} else {
		saved, _ = db.GetByName(m.database, name)
		if saved != nil {
			saved.Source = model.SourceSQLite
		}
	}

	if saved != nil {
		// In "both" mode, update caches and only add to visible lists if matching active source.
		if m.cfg != nil && m.cfg.ParseMode == model.ParseModeBoth {
			if saved.Source == model.SourceSSHConfig {
				m.sshConfigConns = append(m.sshConfigConns, *saved)
			} else {
				m.sqliteConns = append(m.sqliteConns, *saved)
			}
			if saved.Source == m.activeSource {
				m.connectModel.AddItem(*saved)
				m.editModel.AddItem(*saved)
				m.deleteModel.AddItem(*saved)
			}
		} else {
			m.connectModel.AddItem(*saved)
			m.editModel.AddItem(*saved)
			m.deleteModel.AddItem(*saved)
		}
	}
	m.setStatus(fmt.Sprintf("%q added", name), successStyle)
	m.createModel = m.createModel.reset()
	m.createAssignment.Begin(m.groups, nil, m.groupPane.SelectedGroupID())
	m.createAssignmentInitialized = true
	m.refreshGroups()
	m.applyGroupFilter()
}

// finalizePasswordSave encrypts the password with the given passphrase and saves the connection.
// Called after the passphrase modal returns a passphrase. On first use, it creates the
// Argon2id salt and stores an encrypted verification token ("dssh-verify") so future
// passphrase entries can be validated without storing the passphrase itself.
func (m AppModel) finalizePasswordSave(passphrase string) AppModel {
	wr := m.pendingWizard
	if wr == nil {
		m.setError("no pending connection")
		return m
	}

	port, _ := strconv.Atoi(wr.Port)

	salt, isNew, err := m.ensureSalt(passphrase)
	if err != nil {
		m.setError("%s", err)
		m.pendingWizard = nil
		return m
	}

	// If salt already existed, verify the passphrase against the stored token.
	if !isNew {
		if err := m.verifyPassphraseTUI(passphrase, salt); err != nil {
			// Wrong passphrase — re-show modal with error for retry.
			m.modal = newPassphraseModal(false, m.width, m.height)
			m.modal.errMsg = err.Error()
			m.showModal = true
			return m
		}
	}

	key := crypto.DeriveKey(passphrase, salt)

	conn := &model.Connection{
		Name:      wr.Name,
		User:      wr.User,
		Host:      wr.Host,
		Port:      port,
		Directory: wr.Directory,
		AuthType:  model.AuthPassword,
	}

	if wr.Password != "" {
		ciphertext, nonce, err := crypto.Encrypt(key, []byte(wr.Password))
		if err != nil {
			m.setError("%s", err)
			m.pendingWizard = nil
			return m
		}
		conn.EncryptedPass = ciphertext
		conn.PassNonce = nonce
	}

	if err := m.insertConnection(conn, wr.SaveTo, wr.GroupIDs); err != nil {
		m.setError("%s", err)
		m.pendingWizard = nil
		return m
	}

	m.pendingWizard = nil
	m.onConnectionSaved(conn.Name, wr.SaveTo)
	return m
}

// ensureSalt returns the existing Argon2id salt or creates a new one (first-time setup).
// On first use it also stores an encrypted verification token so the passphrase can be
// validated on subsequent entries without storing it in plaintext.
// Does NOT verify the passphrase — caller must do that when salt already exists.
func (m AppModel) ensureSalt(passphrase string) ([]byte, bool, error) {
	salt, err := db.GetSetting(m.database, "argon2_salt")
	if err != nil {
		return nil, false, err
	}
	if salt != nil {
		return salt, false, nil
	}

	// First time — generate salt and store verification token.
	salt, err = crypto.GenerateSalt()
	if err != nil {
		return nil, true, err
	}
	if err := db.SetSetting(m.database, "argon2_salt", salt); err != nil {
		return nil, true, err
	}

	key := crypto.DeriveKey(passphrase, salt)
	chk, chkNonce, err := crypto.Encrypt(key, []byte("dssh-verify"))
	if err != nil {
		return nil, true, err
	}
	if err := db.SetSetting(m.database, "passphrase_check", append(chkNonce, chk...)); err != nil {
		return nil, true, err
	}

	return salt, true, nil
}

// verifyPassphraseTUI checks the passphrase against the stored verification token.
// The token is stored as nonce (12 bytes) + ciphertext, encrypted with the derived key.
// If decryption yields "dssh-verify", the passphrase is correct.
func (m AppModel) verifyPassphraseTUI(passphrase string, salt []byte) error {
	chkData, err := db.GetSetting(m.database, "passphrase_check")
	if err != nil {
		return err
	}
	if chkData == nil {
		// No verification token (legacy data) — skip.
		return nil
	}
	if len(chkData) <= 12 {
		return fmt.Errorf("corrupted passphrase verification data")
	}
	chkNonce := chkData[:12]
	chkCiphertext := chkData[12:]
	key := crypto.DeriveKey(passphrase, salt)
	plain, err := crypto.Decrypt(key, chkCiphertext, chkNonce)
	if err != nil {
		return fmt.Errorf("wrong master passphrase")
	}
	if string(plain) != "dssh-verify" {
		return fmt.Errorf("wrong master passphrase")
	}
	return nil
}

func (m AppModel) View() string {
	// Accent color — red for Delete tab, purple otherwise.
	accentColor := purple
	if m.activeTab == TabDelete {
		accentColor = warnRed
	}

	// Mode indicator in the top-right corner of the tab label row.
	modeLabel := ""
	if m.cfg != nil {
		modeLabel = model.ParseModeLabel(m.cfg.ParseMode)
	}
	tabBar := renderMainTabBar(m.tabs, m.activeTab, m.width, modeLabel, accentColor)
	if m.width < minimumTerminalWidth || m.height < minimumTerminalHeight {
		message := fmt.Sprintf("Terminal too small — minimum %dx%d", minimumTerminalWidth, minimumTerminalHeight)
		return lipgloss.Place(max(1, m.width), max(1, m.height), lipgloss.Center, lipgloss.Center, statusStyle.Render(message))
	}

	var leftContent string
	switch m.activeTab {
	case TabConnect:
		leftContent = m.connectModel.View()
	case TabCreate:
		leftContent = m.createModel.View()
	case TabEdit:
		leftContent = m.editModel.View()
	case TabDelete:
		leftContent = m.deleteModel.View()
	}

	var rightContent string
	if m.assignmentMode() {
		if m.activeTab == TabCreate {
			rightContent = m.createAssignment.View()
		} else {
			rightContent = m.editAssignment.View()
		}
	} else {
		rightContent = m.groupPane.View()
	}

	leftStatus, rightStatus := "", ""
	if m.statusMsg != "" {
		styled := m.statusMsgStyle.Render(m.statusMsg)
		if m.statusMsgRight {
			rightStatus = styled
		} else {
			leftStatus = styled
		}
	}
	if m.activeTab == TabEdit && m.editModel.statusMsg != "" {
		leftStatus = m.editModel.statusStyle.Render(m.editModel.statusMsg)
	}
	if m.activeTab == TabDelete && m.deleteModel.statusMsg != "" {
		leftStatus = m.deleteModel.statusStyle.Render(m.deleteModel.statusMsg)
	}

	base := renderSplitScreen(
		tabBar,
		leftContent,
		rightContent,
		leftStatus,
		rightStatus,
		m.leftPaneHints(),
		m.rightPaneHints(),
		m.width,
		m.height,
		accentColor,
	)
	if m.showModal {
		return compositePopover(base, m.modal.BoxView(), m.width, m.height)
	}
	if m.showGroupNameDialog {
		return compositePopover(base, m.groupNameDialog.BoxView(), m.width, m.height)
	}
	if m.showGroupDelete {
		return compositePopover(base, m.groupDeleteDialog.BoxView(), m.width, m.height)
	}
	return base
}

func (m AppModel) leftPaneHints() string {
	if !m.showHints {
		return statusStyle.Render("? help")
	}

	var hints string
	switch m.activeTab {
	case TabConnect:
		hints = "TAB pane • ↑/↓ navigate • ENTER connect • ←/→ tabs • ESC exit"
	case TabCreate:
		hints = "TAB pane • ↑/↓ fields • CTRL+T auth • CTRL+S save • ENTER next/save • ESC exit"
	case TabEdit:
		if m.editModel.editing {
			hints = "TAB pane • ↑/↓ fields • CTRL+T auth • CTRL+S save • ENTER next/save • ESC cancel"
		} else {
			hints = "TAB pane • ↑/↓ navigate • ENTER edit • ←/→ tabs • ESC exit"
		}
	case TabDelete:
		hints = "TAB pane • ↑/↓ navigate • ENTER ×3 delete • ←/→ tabs • ESC exit"
	}
	return statusStyle.Render(hints)
}

func (m AppModel) rightPaneHints() string {
	if !m.showHints {
		return ""
	}

	if m.assignmentMode() {
		return statusStyle.Render("TAB pane • ↑/↓ navigate • SPACE assign • CTRL+N new • CTRL+S save")
	}
	return statusStyle.Render("TAB pane • ↑/↓ navigate • CTRL+N new • CTRL+R rename • CTRL+D delete")
}

func (m AppModel) toggleHints() AppModel {
	showHints := !m.showHints
	if m.database != nil {
		if err := db.SetSetting(m.database, settingShowHints, []byte(strconv.FormatBool(showHints))); err != nil {
			m.setError("save hint visibility: %s", err)
			return m
		}
	}
	m.showHints = showHints
	return m
}

// onConnectionEdited syncs all tab lists after an edit.
func (m *AppModel) onConnectionEdited(oldName, newName string, source model.Source) {
	m.connectModel.RemoveByName(oldName)
	m.editModel.RemoveByName(oldName)
	m.deleteModel.RemoveByName(oldName)

	var saved *model.Connection
	if source == model.SourceSSHConfig {
		p, _ := m.sshConfigPath()
		saved, _ = sshconfig.GetByName(p, newName)
	} else {
		saved, _ = db.GetByName(m.database, newName)
		if saved != nil {
			saved.Source = model.SourceSQLite
		}
	}
	if saved != nil {
		if saved.Source == model.SourceSSHConfig {
			m.sshConfigConns = removeConnByName(m.sshConfigConns, oldName)
			m.sshConfigConns = append(m.sshConfigConns, *saved)
			sort.Slice(m.sshConfigConns, func(i, j int) bool {
				return strings.ToLower(m.sshConfigConns[i].Name) < strings.ToLower(m.sshConfigConns[j].Name)
			})
		} else {
			m.sqliteConns = removeConnByName(m.sqliteConns, oldName)
			m.sqliteConns = append(m.sqliteConns, *saved)
			sort.Slice(m.sqliteConns, func(i, j int) bool {
				return strings.ToLower(m.sqliteConns[i].Name) < strings.ToLower(m.sqliteConns[j].Name)
			})
		}
		m.connectModel.AddItem(*saved)
		m.editModel.AddItem(*saved)
		m.deleteModel.AddItem(*saved)
	}
}

// handleEditSave either saves directly or starts the passphrase flow when a new
// password must be encrypted.
func (m AppModel) handleEditSave(result *AppResult) AppModel {
	wr := result.WizardResult
	if wr.AuthType != "password" || wr.Password == "" {
		return m.finalizeEditSave(wr, nil, nil)
	}

	salt, err := db.GetSetting(m.database, "argon2_salt")
	if err != nil {
		m.setError("%s", err)
		return m
	}
	isNew := salt == nil
	m.modal = newPassphraseModal(isNew, m.width, m.height)
	m.showModal = true
	m.pendingWizard = wr
	m.pendingEditID = result.Connection.ID
	return m
}

// finalizeEditPasswordSave encrypts the new password and updates the connection.
func (m AppModel) finalizeEditPasswordSave(passphrase string) AppModel {
	wr := m.pendingWizard
	if wr == nil {
		m.setError("no pending edit")
		return m
	}

	salt, isNew, err := m.ensureSalt(passphrase)
	if err != nil {
		m.setError("%s", err)
		m.pendingWizard = nil
		m.pendingEditID = 0
		return m
	}

	if !isNew {
		if err := m.verifyPassphraseTUI(passphrase, salt); err != nil {
			m.modal = newPassphraseModal(false, m.width, m.height)
			m.modal.errMsg = err.Error()
			m.showModal = true
			return m
		}
	}

	key := crypto.DeriveKey(passphrase, salt)

	var ciphertext, nonce []byte
	if wr.Password != "" {
		ciphertext, nonce, err = crypto.Encrypt(key, []byte(wr.Password))
		if err != nil {
			m.setError("%s", err)
			m.pendingWizard = nil
			m.pendingEditID = 0
			return m
		}
	}
	m.pendingWizard = nil
	m.pendingEditID = 0
	return m.finalizeEditSave(wr, ciphertext, nonce)
}

func (m AppModel) finalizeEditSave(wr *WizardResult, encryptedPassword, passNonce []byte) AppModel {
	port, _ := strconv.Atoi(wr.Port)
	original := m.editModel.origConn
	connection := &model.Connection{
		ID:        original.ID,
		Name:      wr.Name,
		User:      wr.User,
		Host:      wr.Host,
		Port:      port,
		Directory: wr.Directory,
		AuthType:  model.AuthType(wr.AuthType),
		ProxyJump: wr.ProxyJump,
		Source:    original.Source,
	}
	if connection.Source == "" {
		connection.Source = model.SourceSQLite
	}
	if connection.AuthType == model.AuthPassword {
		if encryptedPassword == nil {
			connection.EncryptedPass = original.EncryptedPass
			connection.PassNonce = original.PassNonce
		} else {
			connection.EncryptedPass = encryptedPassword
			connection.PassNonce = passNonce
		}
	} else if wr.IdentityFile != "" && wr.IdentityFile != "default" {
		connection.IdentityFile = expandTildeTUI(wr.IdentityFile)
	}

	oldRef := m.connectionRef(original)
	newRef := m.connectionRef(*connection)
	var saveErr error
	if connection.Source == model.SourceSSHConfig {
		path, err := m.sshConfigPath()
		if err != nil {
			saveErr = err
		} else if err := sshconfig.Update(path, original.Name, connection); err != nil {
			saveErr = err
		} else if err := db.ReplaceConnectionGroups(m.database, oldRef, newRef, wr.GroupIDs); err != nil {
			if rollbackErr := sshconfig.Update(path, connection.Name, &original); rollbackErr != nil {
				saveErr = fmt.Errorf("%w (ssh_config rollback failed: %v)", err, rollbackErr)
			} else {
				saveErr = err
			}
		}
	} else {
		saveErr = db.UpdateWithGroups(m.database, connection, oldRef, newRef, wr.GroupIDs)
	}
	if saveErr != nil {
		m.setError("%s", saveErr)
		return m
	}

	m.editModel.editing = false
	m.editModel.statusMsg = ""
	m.pendingWizard = nil
	m.pendingEditID = 0
	m.selectedConnectionName = connection.Name
	m.onConnectionEdited(original.Name, connection.Name, connection.Source)
	m.editAssignment.Begin(m.groups, nil, m.groupPane.SelectedGroupID())
	m.refreshGroups()
	m.applyGroupFilter()
	m.applyPaneFocus()
	m.setStatus(fmt.Sprintf("%q updated", connection.Name), successStyle)
	return m
}

// toggleSource swaps the visible connection list between SQLite and ssh_config.
func (m AppModel) toggleSource() AppModel {
	if m.activeSource == model.SourceSQLite {
		m.activeSource = model.SourceSSHConfig
	} else {
		m.activeSource = model.SourceSQLite
	}

	// Persist the new view mode.
	if m.cfg != nil {
		m.cfg.BothViewMode = m.activeSource
	}
	if m.database != nil {
		_ = db.SetSetting(m.database, "parse_both_view_mode", []byte(m.activeSource))
	}

	var conns []model.Connection
	if m.activeSource == model.SourceSQLite {
		conns = m.sqliteConns
	} else {
		conns = m.sshConfigConns
	}

	items := make([]connectionItem, len(conns))
	for i, c := range conns {
		items[i] = connectionItem{conn: c}
	}

	m.connectModel.SetItems(conns)
	m.editModel.SetItems(items)
	m.deleteModel.SetItems(items)
	m.refreshGroups()
	m.applyGroupFilter()
	m.applyPaneFocus()

	src := "SQLite"
	if m.activeSource == model.SourceSSHConfig {
		src = "ssh_config"
	}
	m.statusMsg = fmt.Sprintf("Showing %s connections", src)
	m.statusMsgStyle = statusStyle
	m.statusMsgRight = true
	return m
}

// setStatus sets the status bar with left alignment.
func (m *AppModel) setStatus(msg string, style lipgloss.Style) {
	m.statusMsg = msg
	m.statusMsgStyle = style
	m.statusMsgRight = false
}

func (m *AppModel) setRightStatus(msg string, style lipgloss.Style) {
	m.statusMsg = msg
	m.statusMsgStyle = style
	m.statusMsgRight = true
}

// setError sets the status bar to an error message.
func (m *AppModel) setError(format string, a ...any) {
	m.statusMsg = fmt.Sprintf("Error: "+format, a...)
	m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
	m.statusMsgRight = false
}

func removeConnByName(conns []model.Connection, name string) []model.Connection {
	for i, c := range conns {
		if c.Name == name {
			return append(conns[:i], conns[i+1:]...)
		}
	}
	return conns
}

func expandTildeTUI(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
