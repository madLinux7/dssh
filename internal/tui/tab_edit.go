package tui

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
)

// editedInfo carries old/new names so AppModel can sync all lists after an edit.
type editedInfo struct {
	oldName string
	newName string
}

// EditModel is the Bubble Tea model for the Edit tab.
// It has two modes: list mode (select a connection) and form mode (edit fields).
type EditModel struct {
	// List mode
	list      list.Model
	filterBox FilterBox
	allItems  []list.Item

	// Form mode
	editing   bool
	editingID int64
	origConn  model.Connection
	inputs    [fieldCount]textinput.Model
	focused   int
	authType  string
	atSave    bool

	// Shared
	database    *sql.DB
	lastEdited  *editedInfo
	statusMsg   string
	statusStyle lipgloss.Style
	width       int
	height      int
}

func newEditModel(conns []connectionItem, database *sql.DB, width, height int) EditModel {
	items := make([]list.Item, len(conns))
	for i, c := range conns {
		items[i] = c
	}

	l := newConnectionList(items, magenta, width, height)

	return EditModel{
		list:      l,
		filterBox: NewFilterBox(width, magenta),
		allItems:  items,
		inputs:    newEditInputs(),
		database:  database,
		width:     width,
		height:    height,
	}
}

func newEditInputs() [fieldCount]textinput.Model {
	var inputs [fieldCount]textinput.Model

	inputs[fieldName] = textinput.New()
	inputs[fieldName].Placeholder = "(required)"
	inputs[fieldName].CharLimit = 64
	inputs[fieldName].Width = 30

	inputs[fieldUser] = textinput.New()
	inputs[fieldUser].Placeholder = "root"
	inputs[fieldUser].CharLimit = 64
	inputs[fieldUser].Width = 30

	inputs[fieldHost] = textinput.New()
	inputs[fieldHost].Placeholder = "(required)"
	inputs[fieldHost].CharLimit = 255
	inputs[fieldHost].Width = 30

	inputs[fieldPort] = textinput.New()
	inputs[fieldPort].Placeholder = "22"
	inputs[fieldPort].CharLimit = 5
	inputs[fieldPort].Width = 10

	inputs[fieldDirectory] = textinput.New()
	inputs[fieldDirectory].Placeholder = "(default)"
	inputs[fieldDirectory].CharLimit = 255
	inputs[fieldDirectory].Width = 40

	inputs[fieldIdentityFile] = textinput.New()
	inputs[fieldIdentityFile].Placeholder = "(default)"
	inputs[fieldIdentityFile].CharLimit = 255
	inputs[fieldIdentityFile].Width = 40

	inputs[fieldPassword] = textinput.New()
	inputs[fieldPassword].Placeholder = "(empty to keep current)"
	inputs[fieldPassword].EchoMode = textinput.EchoPassword
	inputs[fieldPassword].EchoCharacter = '•'
	inputs[fieldPassword].CharLimit = 128
	inputs[fieldPassword].Width = 30

	return inputs
}

func (m *EditModel) enterEditMode(conn model.Connection) {
	m.editing = true
	m.editingID = conn.ID
	m.origConn = conn
	m.statusMsg = ""

	m.inputs[fieldName].SetValue(conn.Name)
	m.inputs[fieldUser].SetValue(conn.User)
	m.inputs[fieldHost].SetValue(conn.Host)
	m.inputs[fieldPort].SetValue(strconv.Itoa(conn.Port))
	m.inputs[fieldDirectory].SetValue(conn.Directory)
	m.inputs[fieldIdentityFile].SetValue(conn.IdentityFile)
	m.inputs[fieldPassword].SetValue("") // never show stored password

	m.authType = string(conn.AuthType)
	m.focused = fieldName
	m.atSave = false
	m.updateFocus()
}

func (m EditModel) visibleFields() []int {
	var fields []int
	for i := 0; i < fieldCount; i++ {
		if m.authType == "password" && i == fieldIdentityFile {
			continue
		}
		if m.authType == "key" && i == fieldPassword {
			continue
		}
		fields = append(fields, i)
	}
	return fields
}

func (m EditModel) nextField() (int, bool) {
	if m.atSave {
		return m.focused, true
	}
	fields := m.visibleFields()
	for i, f := range fields {
		if f == m.focused {
			if i+1 < len(fields) {
				return fields[i+1], false
			}
			return m.focused, true
		}
	}
	return m.focused, m.atSave
}

func (m EditModel) prevField() (int, bool) {
	if m.atSave {
		fields := m.visibleFields()
		return fields[len(fields)-1], false
	}
	fields := m.visibleFields()
	for i, f := range fields {
		if f == m.focused {
			if i > 0 {
				return fields[i-1], false
			}
			return m.focused, false
		}
	}
	return m.focused, false
}

func (m *EditModel) updateFocus() {
	for i := range m.inputs {
		if i == m.focused && !m.atSave {
			m.inputs[i].Focus()
			m.inputs[i].PromptStyle = focusedFieldStyle
			m.inputs[i].TextStyle = focusedFieldStyle
		} else {
			m.inputs[i].Blur()
			m.inputs[i].PromptStyle = blurredFieldStyle
			m.inputs[i].TextStyle = blurredFieldStyle
		}
	}
}

func (m EditModel) Update(msg tea.Msg) (EditModel, *AppResult, tea.Cmd) {
	m.lastEdited = nil

	if m.editing {
		return m.updateForm(msg)
	}
	return m.updateList(msg)
}

func (m EditModel) updateList(msg tea.Msg) (EditModel, *AppResult, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(connectionItem); ok {
				m.enterEditMode(item.conn)
			}
			return m, nil, nil
		case "esc":
			if m.filterBox.Value() != "" {
				m.filterBox.SetValue("")
				m.applyFilter()
				return m, nil, nil
			}
			return m, &AppResult{Action: ActionNone}, nil
		case "up", "down", "pgup", "pgdown":
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, nil, cmd
		}

		if msg.String() == "q" && m.filterBox.Value() == "" {
			return m, &AppResult{Action: ActionNone}, nil
		}

		prevVal := m.filterBox.Value()
		var cmd tea.Cmd
		m.filterBox, cmd = m.filterBox.Update(msg)
		if m.filterBox.Value() != prevVal {
			m.applyFilter()
		}
		return m, nil, cmd
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	var filterCmd tea.Cmd
	m.filterBox, filterCmd = m.filterBox.Update(msg)
	return m, nil, tea.Batch(cmd, filterCmd)
}

func (m EditModel) updateForm(msg tea.Msg) (EditModel, *AppResult, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.editing = false
			return m, nil, nil

		case "down":
			next, save := m.nextField()
			m.focused = next
			m.atSave = save
			m.updateFocus()
			return m, nil, nil

		case "up":
			prev, save := m.prevField()
			m.focused = prev
			m.atSave = save
			m.updateFocus()
			return m, nil, nil

		case "ctrl+t":
			if m.authType == "key" {
				m.authType = "password"
				if m.focused == fieldIdentityFile {
					m.focused = fieldPassword
					m.updateFocus()
				}
			} else {
				m.authType = "key"
				if m.focused == fieldPassword {
					m.focused = fieldIdentityFile
					m.updateFocus()
				}
			}
			return m, nil, nil

		case "enter":
			if m.atSave {
				return m.handleSave()
			}
			next, save := m.nextField()
			m.focused = next
			m.atSave = save
			m.updateFocus()
			return m, nil, nil
		}
	}

	if !m.atSave {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, nil, cmd
	}
	return m, nil, nil
}

func (m EditModel) handleSave() (EditModel, *AppResult, tea.Cmd) {
	name := m.inputs[fieldName].Value()
	user := m.inputs[fieldUser].Value()
	host := m.inputs[fieldHost].Value()
	portStr := m.inputs[fieldPort].Value()
	password := m.inputs[fieldPassword].Value()

	if user == "" {
		user = "root"
	}
	if portStr == "" {
		portStr = "22"
	}

	if name == "" || host == "" {
		m.statusMsg = "Error: name and host are required"
		m.statusStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
		return m, nil, nil
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		m.statusMsg = fmt.Sprintf("Error: invalid port: %s", portStr)
		m.statusStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
		return m, nil, nil
	}

	// Password auth with new password → needs passphrase modal via AppModel.
	if m.authType == "password" && password != "" {
		origConn := m.origConn
		return m, &AppResult{
			Action:     ActionEdited,
			Connection: &origConn,
			WizardResult: &WizardResult{
				Name:         name,
				User:         user,
				Host:         host,
				Port:         portStr,
				Directory:    m.inputs[fieldDirectory].Value(),
				AuthType:     m.authType,
				IdentityFile: m.inputs[fieldIdentityFile].Value(),
				Password:     password,
			},
		}, nil
	}

	// Direct save: key auth, or password auth with unchanged password.
	conn := &model.Connection{
		ID:       m.editingID,
		Name:     name,
		User:     user,
		Host:     host,
		Port:     port,
		AuthType: model.AuthType(m.authType),
	}

	conn.Directory = m.inputs[fieldDirectory].Value()

	if m.authType == "key" {
		idf := m.inputs[fieldIdentityFile].Value()
		if idf != "" && idf != "default" {
			conn.IdentityFile = expandTildeTUI(idf)
		}
	}

	// Preserve existing encrypted password when not changed.
	if m.authType == "password" {
		conn.EncryptedPass = m.origConn.EncryptedPass
		conn.PassNonce = m.origConn.PassNonce
	}

	if err := db.Update(m.database, conn); err != nil {
		m.statusMsg = fmt.Sprintf("Error: %s", err)
		m.statusStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
		return m, nil, nil
	}

	oldName := m.origConn.Name
	m.lastEdited = &editedInfo{oldName: oldName, newName: name}
	m.editing = false
	return m, nil, nil
}

func (m *EditModel) applyFilter() {
	query := strings.ToLower(m.filterBox.Value())
	if query == "" {
		m.list.SetItems(m.allItems)
		return
	}
	var filtered []list.Item
	for _, item := range m.allItems {
		if ci, ok := item.(connectionItem); ok {
			label := strings.ToLower(ci.conn.DisplayLabel())
			if strings.Contains(label, query) {
				filtered = append(filtered, item)
			}
		}
	}
	m.list.SetItems(filtered)
}

func (m EditModel) View() string {
	if m.editing {
		return m.viewForm()
	}
	return m.viewList()
}

func (m EditModel) viewList() string {
	title := titleStyle.Foreground(magenta).Render("Edit Connection")
	return lipgloss.JoinVertical(lipgloss.Left, title, m.filterBox.View(), "", m.list.View())
}

func (m EditModel) viewForm() string {
	var b strings.Builder

	b.WriteString(titleStyle.Foreground(magenta).Render("Edit Connection"))
	b.WriteString("\n\n")

	labels := [fieldCount]string{"Name", "User", "Host", "Port", "Directory", "Identity File", "Password"}

	for _, i := range m.visibleFields() {
		label := labelStyle.Render(labels[i])
		b.WriteString(fmt.Sprintf("%s %s\n", label, m.inputs[i].View()))
	}

	b.WriteString("\n")
	authLabel := labelStyle.Render("Auth Type")
	authHint := blurredFieldStyle.Render("  (ctrl+t to toggle)")
	b.WriteString(fmt.Sprintf("%s %s%s\n\n", authLabel, focusedFieldStyle.Render(m.authType), authHint))

	if m.atSave {
		b.WriteString(focusedFieldStyle.Render("[ Save ]"))
	} else {
		b.WriteString(blurredFieldStyle.Render("[ Save ]"))
	}

	if m.statusMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(m.statusStyle.Render(m.statusMsg))
	}

	b.WriteString("\n\n")
	b.WriteString(statusStyle.Render("esc: cancel  |  up/down: navigate  |  ctrl+t: toggle auth  |  enter: next/save"))

	return b.String()
}

func (m *EditModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.filterBox.SetWidth(w)
	m.list.SetSize(w, h-4)
}

// AddItem inserts a connection in ascending alphabetical position.
func (m *EditModel) AddItem(conn model.Connection) {
	m.allItems = insertItemSorted(m.allItems, conn)
	m.applyFilter()
}

// RemoveByName removes the first item matching the given connection name.
func (m *EditModel) RemoveByName(name string) {
	for i, item := range m.allItems {
		if ci, ok := item.(connectionItem); ok && ci.conn.Name == name {
			m.allItems = append(m.allItems[:i], m.allItems[i+1:]...)
			break
		}
	}
	m.applyFilter()
}
