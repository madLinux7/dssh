package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/madLinux7/dssh/internal/model"
)

// connectionItem wraps a Connection for the bubbles/list component.
// Implements list.Item (Title, Description, FilterValue) for use in
// both the Connect and Delete tab lists.
type connectionItem struct {
	conn model.Connection
}

func (i connectionItem) Title() string       { return i.conn.DisplayLabel() }
func (i connectionItem) Description() string { return "" }
func (i connectionItem) FilterValue() string { return i.conn.Name }

// ConnectModel is the Bubble Tea model for the Connect tab.
type ConnectModel struct {
	list      list.Model
	filterBox FilterBox
	allItems  []list.Item
	width     int
	height    int
}

func newConnectModel(conns []model.Connection, width, height int) ConnectModel {
	items := make([]list.Item, len(conns))
	for i, c := range conns {
		items[i] = connectionItem{conn: c}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(magenta).
		BorderLeftForeground(magenta)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(dimGray).
		BorderLeftForeground(magenta)
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)

	l := list.New(items, delegate, width, height-4)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	return ConnectModel{
		list:      l,
		filterBox: NewFilterBox(width - 2),
		allItems:  items,
		width:     width,
		height:    height,
	}
}

func (m ConnectModel) Update(msg tea.Msg) (ConnectModel, *AppResult, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(connectionItem); ok {
				conn := item.conn
				return m, &AppResult{Action: ActionConnect, Connection: &conn}, nil
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

		// "q" quits only when filter is empty.
		if msg.String() == "q" && m.filterBox.Value() == "" {
			return m, &AppResult{Action: ActionNone}, nil
		}

		// All other keys go to the filter.
		prevVal := m.filterBox.Value()
		var cmd tea.Cmd
		m.filterBox, cmd = m.filterBox.Update(msg)
		if m.filterBox.Value() != prevVal {
			m.applyFilter()
		}
		return m, nil, cmd
	}

	// Non-key messages (cursor blink, etc.) go to both sub-models.
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	var filterCmd tea.Cmd
	m.filterBox, filterCmd = m.filterBox.Update(msg)
	return m, nil, tea.Batch(cmd, filterCmd)
}

// applyFilter updates the list to show only items matching the current filter text.
func (m *ConnectModel) applyFilter() {
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

func (m ConnectModel) View() string {
	title := titleStyle.Render("Select Connection")
	return lipgloss.JoinVertical(lipgloss.Left, title, m.filterBox.View(), "", m.list.View())
}

func (m *ConnectModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.filterBox.SetWidth(w - 2)
	m.list.SetSize(w, h-4)
}

// AddItem inserts a connection in descending alphabetical position.
func (m *ConnectModel) AddItem(conn model.Connection) {
	newItem := list.Item(connectionItem{conn: conn})
	newName := strings.ToLower(conn.Name)
	pos := len(m.allItems)
	for i, item := range m.allItems {
		if ci, ok := item.(connectionItem); ok {
			if newName > strings.ToLower(ci.conn.Name) {
				pos = i
				break
			}
		}
	}
	m.allItems = append(m.allItems, nil)
	copy(m.allItems[pos+1:], m.allItems[pos:])
	m.allItems[pos] = newItem
	m.applyFilter()
}

// RemoveByName removes the first item matching the given connection name.
func (m *ConnectModel) RemoveByName(name string) {
	for i, item := range m.allItems {
		if ci, ok := item.(connectionItem); ok && ci.conn.Name == name {
			m.allItems = append(m.allItems[:i], m.allItems[i+1:]...)
			break
		}
	}
	m.applyFilter()
}
