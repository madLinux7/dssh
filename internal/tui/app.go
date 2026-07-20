// Package tui implements the interactive terminal UI using Bubble Tea.
//
// Architecture: The TUI follows Bubble Tea's Elm-like pattern (Model → Update → View).
// AppModel is the top-level model that manages four tab sub-models (Create, Connect, Edit, Delete)
// and an optional passphrase modal overlay. Each sub-model handles its own input and
// rendering, while AppModel coordinates tab switching, saves, and result propagation
// back to the CLI layer.
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

	// Dual-list state for both/separate mode.
	activeSource   model.Source // current visible source in separate mode
	sqliteConns    []model.Connection
	sshConfigConns []model.Connection

	// Passphrase modal state.
	showModal     bool
	modal         PassphraseModal
	pendingWizard *WizardResult
	pendingEditID int64 // non-zero when edit password save is pending
}

// Run launches the TUI and returns the user's action.
func Run(connections []model.Connection, d *sql.DB, initialTab Tab, cfg *model.RuntimeConfig) *AppResult {
	// Sort connections ascending (A→Z).
	sort.Slice(connections, func(i, j int) bool {
		return strings.ToLower(connections[i].Name) < strings.ToLower(connections[j].Name)
	})

	connItems := make([]connectionItem, len(connections))
	for i, c := range connections {
		connItems[i] = connectionItem{conn: c}
	}

	m := AppModel{
		activeTab:    initialTab,
		tabs:         []string{"Create", "Connect", "Edit", "Delete"},
		connectModel: newConnectModel(connections, 80, 20),
		createModel:  newCreateModel(80, 20),
		editModel:    newEditModel(connItems, d, 80, 20),
		deleteModel:  newDeleteModel(connItems, d, 80, 20),
		database:     d,
		cfg:          cfg,
		activeSource: model.SourceSQLite,
	}

	// Respect persisted view mode.
	if cfg != nil && cfg.BothViewMode == model.SourceSSHConfig {
		m.activeSource = model.SourceSSHConfig
	}

	// Pass config to sub-models that need it.
	m.createModel.cfg = cfg
	if cfg != nil {
		m.createModel.saveTo = cfg.DefaultSaveTarget
		m.editModel.sshConfigDest = cfg.SSHConfigDest
		m.deleteModel.sshConfigDest = cfg.SSHConfigDest
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
	}

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

func (m AppModel) Init() tea.Cmd {
	return nil
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Content box: Width(w-2) includes padding (2×2), text area = w-2-4 = w-6.
		contentWidth := m.width - 6
		// Title(3) + filter(3) + gap(1) + status(1) = 8 lines of overhead.
		subModelHeight := m.height - 8
		if subModelHeight < 1 {
			subModelHeight = 1
		}
		if contentWidth < 1 {
			contentWidth = 1
		}
		m.connectModel.SetSize(contentWidth, subModelHeight)
		m.createModel.SetSize(contentWidth, subModelHeight)
		m.editModel.SetSize(contentWidth, subModelHeight)
		m.deleteModel.SetSize(contentWidth, subModelHeight)
		return m, nil

	case tea.KeyMsg:
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
						m.editModel.editing = false
						m.pendingEditID = 0
					}
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
		case "ctrl+l":
			if m.cfg != nil && m.cfg.ParseMode == model.ParseModeBoth && !m.editModel.editing {
				m = m.toggleSource()
				return m, nil
			}
		case "tab":
			// Connect/Edit-list/Delete: switch tabs.
			// Create/Edit-form: fall through to sub-model for field navigation.
			if m.canTabSwitchTab() {
				m = m.switchTabNext()
				return m, nil
			}
		case "shift+tab":
			if m.canTabSwitchTab() {
				m = m.switchTabPrev()
				return m, nil
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
	case TabCreate:
		var result *AppResult
		m.createModel, result, cmd = m.createModel.Update(msg)
		if result != nil {
			switch result.Action {
			case ActionNone:
				m.result = result
				return m, tea.Quit
			case ActionCreated:
				var quitCmd tea.Cmd
				m, quitCmd = m.handleSave(result.WizardResult)
				if quitCmd != nil {
					return m, quitCmd
				}
			}
		}
	case TabEdit:
		var result *AppResult
		m.editModel, result, cmd = m.editModel.Update(msg)
		if m.editModel.lastEdited != nil {
			info := m.editModel.lastEdited
			m.editModel.lastEdited = nil
			m.onConnectionEdited(info.oldName, info.newName, info.source)
			m.setStatus(fmt.Sprintf("%q updated", info.newName), successStyle)
		}
		if result != nil {
			switch result.Action {
			case ActionNone:
				m.result = result
				return m, tea.Quit
			case ActionEdited:
				m = m.handleEditPasswordSave(result)
			}
		}
	case TabDelete:
		var result *AppResult
		m.deleteModel, result, cmd = m.deleteModel.Update(msg)
		if m.deleteModel.lastDeleted != "" {
			name := m.deleteModel.lastDeleted
			m.connectModel.RemoveByName(name)
			m.editModel.RemoveByName(name)
			m.sqliteConns = removeConnByName(m.sqliteConns, name)
			m.sshConfigConns = removeConnByName(m.sshConfigConns, name)
			m.deleteModel.lastDeleted = ""
			m.setStatus(fmt.Sprintf("%q deleted", name), successStyle)
		}
		// Clear app-level status when delete tab shows its own confirmation text.
		if m.deleteModel.statusMsg != "" && m.statusMsg != "" {
			m.statusMsg = ""
		}
		if result != nil {
			m.result = result
			return m, tea.Quit
		}
	}

	return m, cmd
}

// switchTabNext advances the active tab (wraps around) and clears transient state.
func (m AppModel) switchTabNext() AppModel {
	m.activeTab = Tab((int(m.activeTab) + 1) % len(m.tabs))
	m.statusMsg = ""
	m.deleteModel.ResetConfirm()
	return m
}

// switchTabPrev moves to the previous tab (wraps around) and clears transient state.
func (m AppModel) switchTabPrev() AppModel {
	m.activeTab = Tab((int(m.activeTab) - 1 + len(m.tabs)) % len(m.tabs))
	m.statusMsg = ""
	m.deleteModel.ResetConfirm()
	return m
}

// canTabSwitchTab reports whether Tab/Shift+Tab should switch tabs.
// List tabs (Connect/Delete/Edit-list) switch; form contexts use Tab for field nav.
func (m AppModel) canTabSwitchTab() bool {
	switch m.activeTab {
	case TabConnect, TabDelete:
		return true
	case TabEdit:
		return !m.editModel.editing
	}
	return false
}

// canArrowSwitchTab reports whether Left/Right should switch tabs.
// In Connect/Delete/Edit-list: always. In forms: only when on the Save button
// or when the currently focused text field is empty. Save-To toggle (Create)
// is handled by the sub-model, so we return false there.
func (m AppModel) canArrowSwitchTab() bool {
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

	if err := m.insertConnection(conn, wr.SaveTo); err != nil {
		m.setError("%s", err)
		return m, nil
	}

	m.onConnectionSaved(conn.Name, wr.SaveTo)
	return m, nil
}

// insertConnection saves to the appropriate backend based on the save target.
func (m AppModel) insertConnection(conn *model.Connection, target model.SaveTarget) error {
	saveToSSH := target == model.SaveTargetSSHConfig ||
		(m.cfg != nil && m.cfg.ParseMode == model.ParseModeSSHConfigOnly)

	if saveToSSH {
		p, err := m.sshConfigPath()
		if err != nil {
			return err
		}
		conn.Source = model.SourceSSHConfig
		return sshconfig.Insert(p, conn)
	}
	conn.Source = model.SourceSQLite
	return db.Insert(m.database, conn)
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

	if err := m.insertConnection(conn, wr.SaveTo); err != nil {
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

	// If modal is active, render it as full-screen overlay.
	if m.showModal {
		return m.modal.View()
	}

	// Content from sub-model.
	var content string
	switch m.activeTab {
	case TabConnect:
		content = m.connectModel.View()
	case TabCreate:
		content = m.createModel.View()
	case TabEdit:
		content = m.editModel.View()
	case TabDelete:
		content = m.deleteModel.View()
	}

	// Inner content area height (inside border + padding).
	contentHeight := m.height - 4
	if contentHeight < 2 {
		contentHeight = 2
	}

	// Bottom status line.
	contentLines := lipgloss.Height(content)
	if m.statusMsg != "" {
		padNeeded := contentHeight - contentLines - 1
		if padNeeded > 0 {
			content += strings.Repeat("\n", padNeeded)
		}
		styled := m.statusMsgStyle.Render(m.statusMsg)
		if m.statusMsgRight {
			innerWidth := m.width - 6
			gap := innerWidth - lipgloss.Width(styled)
			if gap < 0 {
				gap = 0
			}
			content += "\n" + strings.Repeat(" ", gap) + styled
		} else {
			content += "\n" + styled
		}
	}

	contentBox := contentStyle.
		BorderForeground(accentColor).
		Width(m.width - 2).
		Height(contentHeight).
		Render(content)

	return lipgloss.JoinVertical(lipgloss.Left,
		tabBar,
		contentBox,
	)
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
		m.connectModel.AddItem(*saved)
		m.editModel.AddItem(*saved)
		m.deleteModel.AddItem(*saved)
	}
}

// handleEditPasswordSave starts the passphrase modal flow for an edited connection
// that has a new password.
func (m AppModel) handleEditPasswordSave(result *AppResult) AppModel {
	wr := result.WizardResult

	salt, err := db.GetSetting(m.database, "argon2_salt")
	if err != nil {
		m.setError("%s", err)
		m.editModel.editing = false
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

	port, _ := strconv.Atoi(wr.Port)

	salt, isNew, err := m.ensureSalt(passphrase)
	if err != nil {
		m.setError("%s", err)
		m.editModel.editing = false
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

	conn := &model.Connection{
		ID:        m.pendingEditID,
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
			m.editModel.editing = false
			m.pendingWizard = nil
			m.pendingEditID = 0
			return m
		}
		conn.EncryptedPass = ciphertext
		conn.PassNonce = nonce
	}

	oldName := m.editModel.origConn.Name
	if err := db.Update(m.database, conn); err != nil {
		m.setError("%s", err)
		m.editModel.editing = false
		m.pendingWizard = nil
		m.pendingEditID = 0
		return m
	}

	m.editModel.editing = false
	m.setStatus(fmt.Sprintf("%q updated", wr.Name), successStyle)
	m.pendingWizard = nil
	m.pendingEditID = 0
	m.onConnectionEdited(oldName, wr.Name, m.editModel.origConn.Source)
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
