package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
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

// insertItemSorted inserts a connection into a sorted (ascending A→Z) item slice
// and returns the updated slice.
func insertItemSorted(items []list.Item, conn model.Connection) []list.Item {
	newItem := list.Item(connectionItem{conn: conn})
	newName := strings.ToLower(conn.Name)
	pos := len(items)
	for i, item := range items {
		if ci, ok := item.(connectionItem); ok {
			if newName < strings.ToLower(ci.conn.Name) {
				pos = i
				break
			}
		}
	}
	items = append(items, nil)
	copy(items[pos+1:], items[pos:])
	items[pos] = newItem
	return items
}

// newConnectionList creates a styled list for connection items.
func newConnectionList(items []list.Item, accentColor lipgloss.Color, width, height int) list.Model {
	delegate := connectionListDelegate(accentColor)

	l := list.New(items, delegate, width, max(1, height-6))
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	return l
}

func connectionListDelegate(accentColor lipgloss.Color) list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	bullet := lipgloss.NormalBorder()
	bullet.Left = "•"
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(accentColor).
		BorderStyle(bullet).
		BorderLeftForeground(accentColor)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(dimGray).
		BorderStyle(bullet).
		BorderLeftForeground(accentColor)
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)
	return delegate
}

func selectedConnectionName(l list.Model) string {
	item, ok := l.SelectedItem().(connectionItem)
	if !ok {
		return ""
	}
	return item.conn.Name
}

func selectConnectionByName(l *list.Model, name string) bool {
	for index, item := range l.Items() {
		connection, ok := item.(connectionItem)
		if ok && connection.conn.Name == name {
			l.Select(index)
			return true
		}
	}
	if len(l.Items()) > 0 {
		l.Select(0)
	}
	return false
}

func connectionListView(l list.Model, filtered bool) string {
	if len(l.Items()) > 0 {
		return l.View()
	}
	message := "No existing connections yet"
	if filtered {
		message = "No matching connections"
	}
	return lipgloss.NewStyle().Italic(true).Render(message)
}
