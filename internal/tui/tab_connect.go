package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
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
	list   list.Model
	width  int
	height int
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

	l := list.New(items, delegate, width, height)
	l.Title = "Select Connection"
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return ConnectModel{list: l, width: width, height: height}
}

func (m ConnectModel) Update(msg tea.Msg) (ConnectModel, *AppResult, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(connectionItem); ok {
				conn := item.conn
				return m, &AppResult{Action: ActionConnect, Connection: &conn}, nil
			}
		case "q", "esc":
			return m, &AppResult{Action: ActionNone}, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, nil, cmd
}

func (m ConnectModel) View() string {
	return m.list.View()
}

func (m *ConnectModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.list.SetSize(w, h)
}

// AddItem inserts a connection in descending alphabetical position.
func (m *ConnectModel) AddItem(conn model.Connection) {
	insertItemSorted(&m.list, conn)
}

// insertItemSorted inserts a connectionItem into a bubbles/list in
// descending alphabetical order (Z→A) by connection name.
// Used by both ConnectModel and DeleteModel to keep lists sorted after adds.
func insertItemSorted(l *list.Model, conn model.Connection) {
	items := l.Items()
	newName := strings.ToLower(conn.Name)
	pos := len(items)
	for i, item := range items {
		if ci, ok := item.(connectionItem); ok {
			if newName > strings.ToLower(ci.conn.Name) {
				pos = i
				break
			}
		}
	}
	l.InsertItem(pos, connectionItem{conn: conn})
}

// RemoveByName removes the first item matching the given connection name.
func (m *ConnectModel) RemoveByName(name string) {
	for i, item := range m.list.Items() {
		if ci, ok := item.(connectionItem); ok && ci.conn.Name == name {
			m.list.RemoveItem(i)
			return
		}
	}
}
