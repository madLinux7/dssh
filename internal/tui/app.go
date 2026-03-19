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
	TabConnect Tab = iota
	TabNew
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
		tabs:         []string{"Connect", "New", "Delete"},
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
		contentWidth := m.width - 6    // border(2) + padding(4)
		subModelHeight := m.height - 7 // content area minus 1 for status line
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
				m.activeTab = TabConnect
				m.statusMsg = ""
				return m, nil
			}
		case "2":
			if m.activeTab != TabNew {
				m.activeTab = TabNew
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
		m.statusMsg = "Error: name and host are required"
		m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
		return m, nil
	}

	port, err := strconv.Atoi(wr.Port)
	if err != nil || port < 1 || port > 65535 {
		m.statusMsg = fmt.Sprintf("Error: invalid port: %s", wr.Port)
		m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
		return m, nil
	}

	// Password auth — show passphrase modal.
	if wr.AuthType == "password" {
		salt, err := db.GetSetting(m.database, "argon2_salt")
		if err != nil {
			m.statusMsg = fmt.Sprintf("Error: %s", err)
			m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
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
		m.statusMsg = fmt.Sprintf("Error: %s", err)
		m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
		return m, nil
	}

	// Fetch saved connection (with ID) and update lists.
	saved, _ := db.GetByName(m.database, conn.Name)
	if saved != nil {
		m.connectModel.AddItem(*saved)
		m.deleteModel.AddItem(*saved)
	}

	m.statusMsg = fmt.Sprintf("%q added", conn.Name)
	m.statusMsgStyle = successStyle
	m.newModel = m.newModel.reset()
	return m, nil
}

// finalizePasswordSave encrypts the password with the given passphrase and saves the connection.
func (m AppModel) finalizePasswordSave(passphrase string) AppModel {
	wr := m.pendingWizard

	if wr == nil {
		m.statusMsg = "Error: no pending connection"
		m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
		return m
	}

	port, _ := strconv.Atoi(wr.Port)

	// Ensure salt exists (create if first time).
	salt, err := db.GetSetting(m.database, "argon2_salt")
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error: %s", err)
		m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
		m.pendingWizard = nil
		return m
	}
	if salt == nil {
		salt, err = crypto.GenerateSalt()
		if err != nil {
			m.statusMsg = fmt.Sprintf("Error: %s", err)
			m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
			m.pendingWizard = nil
			return m
		}
		if err := db.SetSetting(m.database, "argon2_salt", salt); err != nil {
			m.statusMsg = fmt.Sprintf("Error: %s", err)
			m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
			m.pendingWizard = nil
			return m
		}
		// Store verification token.
		key := crypto.DeriveKey(passphrase, salt)
		chk, chkNonce, err := crypto.Encrypt(key, []byte("dssh-verify"))
		if err != nil {
			m.statusMsg = fmt.Sprintf("Error: %s", err)
			m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
			m.pendingWizard = nil
			return m
		}
		if err := db.SetSetting(m.database, "passphrase_check", append(chkNonce, chk...)); err != nil {
			m.statusMsg = fmt.Sprintf("Error: %s", err)
			m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
			m.pendingWizard = nil
			return m
		}
	} else {
		// Verify passphrase against stored token.
		if err := m.verifyPassphraseTUI(passphrase, salt); err != nil {
			// Wrong passphrase — re-show modal with error.
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
			m.statusMsg = fmt.Sprintf("Error: %s", err)
			m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
			m.pendingWizard = nil
			return m
		}
		conn.EncryptedPass = ciphertext
		conn.PassNonce = nonce
	}

	if err := db.Insert(m.database, conn); err != nil {
		m.statusMsg = fmt.Sprintf("Error: %s", err)
		m.statusMsgStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
		m.pendingWizard = nil
		return m
	}

	m.pendingWizard = nil

	// Fetch saved connection (with ID) and update lists.
	saved, _ := db.GetByName(m.database, conn.Name)
	if saved != nil {
		m.connectModel.AddItem(*saved)
		m.deleteModel.AddItem(*saved)
	}

	m.statusMsg = fmt.Sprintf("%q added", conn.Name)
	m.statusMsgStyle = successStyle
	m.newModel = m.newModel.reset()
	return m
}

// verifyPassphraseTUI checks the passphrase against the stored verification token.
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
	// Tab bar.
	var tabBar strings.Builder
	for i, tab := range m.tabs {
		if Tab(i) == m.activeTab {
			tabBar.WriteString(activeTabStyle.Render(tab))
		} else {
			tabBar.WriteString(inactiveTabStyle.Render(tab))
		}
		if i < len(m.tabs)-1 {
			tabBar.WriteString(tabGapStyle.Render(" | "))
		}
	}

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
	contentHeight := m.height - 6
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
		Width(m.width - 4).
		Height(contentHeight).
		Render(content)

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		tabBar.String(),
		contentBox,
	)
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
