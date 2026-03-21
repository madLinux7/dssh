// Package tui implements the interactive terminal UI using Bubble Tea.
//
// Architecture: The TUI follows Bubble Tea's Elm-like pattern (Model → Update → View).
// AppModel is the top-level model that manages three tab sub-models (Connect, New, Delete)
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
)

// Tab identifies a tab in the TUI.
type Tab int

const (
	TabNew Tab = iota
	TabConnect
	TabDelete
)

// AppModel is the top-level Bubble Tea model.
type AppModel struct {
	activeTab      Tab
	tabs           []string
	connectModel   ConnectModel
	newModel       NewModel
	deleteModel    DeleteModel
	database       *sql.DB
	result         *AppResult
	statusMsg      string
	statusMsgStyle lipgloss.Style
	width          int
	height         int

	// Passphrase modal state.
	showModal     bool
	modal         PassphraseModal
	pendingWizard *WizardResult
}

// Run launches the TUI and returns the user's action.
func Run(connections []model.Connection, d *sql.DB, initialTab Tab) *AppResult {
	// Sort connections descending (Z→A).
	sort.Slice(connections, func(i, j int) bool {
		return strings.ToLower(connections[i].Name) > strings.ToLower(connections[j].Name)
	})

	connItems := make([]connectionItem, len(connections))
	for i, c := range connections {
		connItems[i] = connectionItem{conn: c}
	}

	m := AppModel{
		activeTab:    initialTab,
		tabs:         []string{"New", "Connect", "Delete"},
		connectModel: newConnectModel(connections, 80, 20),
		newModel:     newNewModel(80, 20),
		deleteModel:  newDeleteModel(connItems, d, 80, 20),
		database:     d,
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
		// Content box: Width(w-4) includes padding (2×2), text area = w-4-4 = w-8.
		contentWidth := m.width - 8
		// Title(3) + filter(3) + gap(1) + status(1) = 8 lines of overhead.
		subModelHeight := m.height - 8
		if subModelHeight < 1 {
			subModelHeight = 1
		}
		if contentWidth < 1 {
			contentWidth = 1
		}
		m.connectModel.SetSize(contentWidth, subModelHeight)
		m.newModel.SetSize(contentWidth, subModelHeight)
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
					return m, nil
				}
				// Passphrase entered — finalize the save.
				m.showModal = false
				m = m.finalizePasswordSave(result.Passphrase)
				return m, nil
			}
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c":
			m.result = &AppResult{Action: ActionNone}
			return m, tea.Quit
		case "tab":
			m.activeTab = Tab((int(m.activeTab) + 1) % len(m.tabs))
			m.statusMsg = ""
			return m, nil
		case "shift+tab":
			m.activeTab = Tab((int(m.activeTab) - 1 + len(m.tabs)) % len(m.tabs))
			m.statusMsg = ""
			return m, nil
		case "1":
			if m.activeTab != TabNew {
				m.activeTab = TabNew
				m.statusMsg = ""
				return m, nil
			}
		case "2":
			if m.activeTab != TabNew {
				m.activeTab = TabConnect
				m.statusMsg = ""
				return m, nil
			}
		case "3":
			if m.activeTab != TabNew {
				m.activeTab = TabDelete
				m.statusMsg = ""
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
	case TabNew:
		var result *AppResult
		m.newModel, result, cmd = m.newModel.Update(msg)
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
	case TabDelete:
		var result *AppResult
		m.deleteModel, result, cmd = m.deleteModel.Update(msg)
		if m.deleteModel.lastDeleted != "" {
			m.connectModel.RemoveByName(m.deleteModel.lastDeleted)
			m.deleteModel.lastDeleted = ""
		}
		if result != nil {
			m.result = result
			return m, tea.Quit
		}
	}

	return m, cmd
}

// handleSave validates and saves a new connection.
// For key auth: saves directly, stays in TUI.
// For password auth: exits TUI so CLI can handle passphrase prompt.
func (m AppModel) handleSave(wr *WizardResult) (AppModel, tea.Cmd) {
	if wr.User == "" {
		wr.User = "root"
	}

	if wr.Name == "" || wr.Host == "" {
		m.setError("name and host are required")
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
		Name:     wr.Name,
		User:     wr.User,
		Host:     wr.Host,
		Port:     port,
		AuthType: model.AuthKey,
	}
	if wr.IdentityFile != "default" {
		conn.IdentityFile = expandTildeTUI(wr.IdentityFile)
	}

	if err := db.Insert(m.database, conn); err != nil {
		m.setError("%s", err)
		return m, nil
	}

	m.onConnectionSaved(conn.Name)
	return m, nil
}

// onConnectionSaved updates the Connect/Delete lists and resets the New form
// after a connection is successfully persisted.
func (m *AppModel) onConnectionSaved(name string) {
	saved, _ := db.GetByName(m.database, name)
	if saved != nil {
		m.connectModel.AddItem(*saved)
		m.deleteModel.AddItem(*saved)
	}
	m.statusMsg = fmt.Sprintf("%q added", name)
	m.statusMsgStyle = successStyle
	m.newModel = m.newModel.reset()
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
		Name:     wr.Name,
		User:     wr.User,
		Host:     wr.Host,
		Port:     port,
		AuthType: model.AuthPassword,
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

	if err := db.Insert(m.database, conn); err != nil {
		m.setError("%s", err)
		m.pendingWizard = nil
		return m
	}

	m.pendingWizard = nil
	m.onConnectionSaved(conn.Name)
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

	// Tab bar — render each tab and split into top/bottom lines.
	var tabTopParts, tabBotParts []string
	var tabWidths []int
	for i, tab := range m.tabs {
		style := inactiveTabStyle.BorderForeground(accentColor)
		if Tab(i) == m.activeTab {
			style = activeTabStyle.BorderForeground(accentColor)
		}
		rendered := style.Render(tab)
		lines := strings.Split(rendered, "\n")
		tabTopParts = append(tabTopParts, lines[0])
		tabBotParts = append(tabBotParts, lines[1])
		tabWidths = append(tabWidths, lipgloss.Width(lines[0]))
	}
	tabBar := strings.Join(tabTopParts, " ") + "\n" + strings.Join(tabBotParts, " ")

	// Connecting line: merges tab bottoms with content box top border.
	contentBoxWidth := m.width - 2 // content inner width + left/right borders
	var conn strings.Builder
	pos := 0
	for i, w := range tabWidths {
		if pos == 0 {
			conn.WriteRune('│')
		} else {
			conn.WriteRune('╯')
		}
		pos++
		for j := 0; j < w-2; j++ {
			conn.WriteRune(' ')
		}
		pos += w - 2
		conn.WriteRune('╰')
		pos++
		if i < len(tabWidths)-1 {
			conn.WriteRune('─') // gap between tabs
			pos++
		}
	}
	for pos < contentBoxWidth-1 {
		conn.WriteRune('─')
		pos++
	}
	conn.WriteRune('╮')
	connLine := lipgloss.NewStyle().Foreground(accentColor).Render(conn.String())

	// If modal is active, render it as full-screen overlay.
	if m.showModal {
		return m.modal.View()
	}

	// Content from sub-model.
	var content string
	switch m.activeTab {
	case TabConnect:
		content = m.connectModel.View()
	case TabNew:
		content = m.newModel.View()
	case TabDelete:
		content = m.deleteModel.View()
	}

	// Inner content area height (inside border + padding).
	contentHeight := m.height - 4
	if contentHeight < 2 {
		contentHeight = 2
	}

	// Pad content so the status line sits at the very bottom.
	contentLines := lipgloss.Height(content)
	padNeeded := contentHeight - contentLines - 1
	if padNeeded > 0 {
		content += strings.Repeat("\n", padNeeded)
	}

	// Status line (last line inside the box).
	if m.statusMsg != "" {
		content += "\n" + m.statusMsgStyle.Render(m.statusMsg)
	}

	contentBox := contentStyle.
		BorderForeground(accentColor).
		Width(m.width - 4).
		Height(contentHeight).
		Render(content)

	return lipgloss.JoinVertical(lipgloss.Left,
		tabBar,
		connLine,
		contentBox,
	)
}

// setError sets the status bar to an error message.
func (m *AppModel) setError(format string, a ...any) {
	m.statusMsg = fmt.Sprintf("Error: "+format, a...)
	m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
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
