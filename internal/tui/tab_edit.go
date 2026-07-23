package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/madLinux7/dssh/internal/model"
)

// EditModel is the Bubble Tea model for the Edit tab.
// It has two modes: list mode (select a connection) and form mode (edit fields).
type EditModel struct {
	// List mode
	list       list.Model
	filterBox  FilterBox
	allItems   []list.Item
	groupNames map[string]bool

	// Form mode
	editing  bool
	origConn model.Connection
	inputs   [fieldCount]textinput.Model
	focused  int
	authType string
	atSave   bool

	// Shared
	statusMsg   string
	statusStyle lipgloss.Style
	width       int
	height      int
	active      bool
}

func newEditModel(conns []connectionItem, width, height int) EditModel {
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
		width:     width,
		height:    height,
		active:    true,
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

	inputs[fieldProxyJump] = textinput.New()
	inputs[fieldProxyJump].Placeholder = "user@bastion / host1,host2"
	inputs[fieldProxyJump].CharLimit = 255
	inputs[fieldProxyJump].Width = 40

	return inputs
}

func (m *EditModel) enterEditMode(conn model.Connection) {
	m.editing = true
	m.origConn = conn
	m.statusMsg = ""

	m.inputs[fieldName].SetValue(conn.Name)
	m.inputs[fieldUser].SetValue(conn.User)
	m.inputs[fieldHost].SetValue(conn.Host)
	m.inputs[fieldPort].SetValue(strconv.Itoa(conn.Port))
	m.inputs[fieldDirectory].SetValue(conn.Directory)
	m.inputs[fieldIdentityFile].SetValue(conn.IdentityFile)
	m.inputs[fieldProxyJump].SetValue(conn.ProxyJump)
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
		if i == m.focused && !m.atSave && m.active && m.editing {
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

		case "down", "tab":
			next, save := m.nextField()
			m.focused = next
			m.atSave = save
			m.updateFocus()
			return m, nil, nil

		case "up", "shift+tab":
			prev, save := m.prevField()
			m.focused = prev
			m.atSave = save
			m.updateFocus()
			return m, nil, nil

		case "ctrl+t":
			// Don't allow toggling to password for ssh_config entries.
			if m.origConn.Source == model.SourceSSHConfig {
				return m, nil, nil
			}
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
	m.statusMsg = ""
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

	if err := model.ValidateName(name); err != nil {
		m.statusMsg = fmt.Sprintf("Error: %s", err)
		m.statusStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
		return m, nil, nil
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		m.statusMsg = fmt.Sprintf("Error: invalid port: %s", portStr)
		m.statusStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
		return m, nil, nil
	}

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
			ProxyJump:    m.inputs[fieldProxyJump].Value(),
			Password:     password,
		},
	}, nil
}

func (m *EditModel) applyFilter() {
	selectedName := selectedConnectionName(m.list)
	query := strings.ToLower(m.filterBox.Value())
	var filtered []list.Item
	for _, item := range m.allItems {
		if ci, ok := item.(connectionItem); ok {
			if m.groupNames != nil && !m.groupNames[ci.conn.Name] {
				continue
			}
			label := strings.ToLower(ci.conn.DisplayLabel())
			if query == "" || strings.Contains(label, query) {
				filtered = append(filtered, item)
			}
		}
	}
	m.list.SetItems(filtered)
	selectConnectionByName(&m.list, selectedName)
}

func (m EditModel) View() string {
	if m.editing {
		return m.viewForm()
	}
	return m.viewList()
}

func (m EditModel) viewList() string {
	title := paneTitleStyle(m.active).Render("Edit Connection")
	listView := connectionListView(m.list, m.filterBox.Value() != "" || m.groupNames != nil)
	return lipgloss.JoinVertical(lipgloss.Left, title, m.filterBox.View(), "", listView)
}

func (m EditModel) viewForm() string {
	var b strings.Builder
	compact := m.height <= 14

	b.WriteString(paneTitleStyle(m.active).MarginBottom(0).Render("Edit Connection"))
	b.WriteString("\n")
	if !compact {
		b.WriteString("\n")
	}

	labels := [fieldCount]string{"Name", "User", "Host", "Port", "Directory", "Identity File", "Password", "ProxyJump"}

	for _, i := range m.visibleFields() {
		label := labelStyle.Render(labels[i])
		b.WriteString(fmt.Sprintf("%s %s\n", label, m.inputs[i].View()))
	}

	if !compact {
		b.WriteString("\n")
	}
	authLabel := labelStyle.Render("Auth Type")
	authHint := blurredFieldStyle.Render("  (ctrl+t to toggle)")
	if m.origConn.Source == model.SourceSSHConfig {
		authHint = statusStyle.Render("  (locked: ssh_config entry)")
	}
	b.WriteString(fmt.Sprintf("%s %s%s\n", authLabel, paneAccentStyle(m.active).Render(m.authType), authHint))
	if !compact {
		b.WriteString("\n")
	}

	if m.atSave {
		b.WriteString(paneAccentStyle(m.active).Render("[ Save ]"))
	} else {
		b.WriteString(blurredFieldStyle.Render("[ Save ]"))
	}

	return b.String()
}

func (m *EditModel) SetActive(active bool) {
	m.active = active
	m.filterBox.SetActive(active && !m.editing)
	accent := purple
	if active {
		accent = magenta
	}
	m.list.SetDelegate(connectionListDelegate(accent))
	m.updateFocus()
}

func (m *EditModel) SetFilterValue(value string) {
	m.filterBox.SetValue(value)
	m.applyFilter()
}

func (m EditModel) FilterValue() string       { return m.filterBox.Value() }
func (m EditModel) SelectedName() string      { return selectedConnectionName(m.list) }
func (m *EditModel) SelectByName(name string) { selectConnectionByName(&m.list, name) }

func (m *EditModel) SetGroupNames(names map[string]bool) {
	m.groupNames = names
	m.applyFilter()
}

func (m *EditModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.filterBox.SetWidth(w)
	m.list.SetSize(w, max(1, h-6))
	inputWidth := max(8, w-labelStyle.GetWidth()-2)
	for i := range m.inputs {
		m.inputs[i].Width = inputWidth
	}
}

// AddItem inserts a connection in ascending alphabetical position.
func (m *EditModel) AddItem(conn model.Connection) {
	m.allItems = insertItemSorted(m.allItems, conn)
	m.applyFilter()
}

// SetItems replaces the full connection list (used for source toggling).
func (m *EditModel) SetItems(items []connectionItem) {
	listItems := make([]list.Item, len(items))
	for i, ci := range items {
		listItems[i] = ci
	}
	m.allItems = listItems
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
