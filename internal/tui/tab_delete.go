package tui

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/madLinux7/dssh/internal/db"
	"github.com/madLinux7/dssh/internal/model"
	"github.com/madLinux7/dssh/internal/sshconfig"
)

// confirmExpiredMsg resets the confirmation sequence after a timeout.
// The generation field prevents stale timeouts from clearing a new sequence.
type confirmExpiredMsg struct {
	generation int
}

// DeleteModel is the Bubble Tea model for the Delete tab.
// Deletion requires pressing Enter 3 times on the same item within a 1-second
// window. Moving the cursor or letting the timer expire resets the count.
type DeleteModel struct {
	list              list.Model
	filterBox         FilterBox
	allItems          []list.Item
	groupNames        map[string]bool
	database          *sql.DB
	confirmTarget     int64
	confirmCount      int
	confirmGeneration int
	sshConfigDest     string
	lastDeleted       string // set after successful deletion, read + cleared by app model
	statusMsg         string
	statusStyle       lipgloss.Style
	width             int
	height            int
	active            bool
}

func newDeleteModel(conns []connectionItem, database *sql.DB, width, height int) DeleteModel {
	items := make([]list.Item, len(conns))
	for i, c := range conns {
		items[i] = c
	}

	l := newConnectionList(items, warnOrange, width, height)

	return DeleteModel{
		list:      l,
		filterBox: NewFilterBox(width, warnOrange),
		allItems:  items,
		database:  database,
		width:     width,
		height:    height,
		active:    true,
	}
}

func (m DeleteModel) Update(msg tea.Msg) (DeleteModel, *AppResult, tea.Cmd) {
	m.lastDeleted = "" // clear each update cycle

	switch msg := msg.(type) {
	case confirmExpiredMsg:
		if msg.generation == m.confirmGeneration {
			m.confirmCount = 0
			m.confirmTarget = 0
			m.statusMsg = ""
		}
		return m, nil, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.filterBox.Value() != "" {
				m.filterBox.SetValue("")
				m.applyFilter()
				m.confirmCount = 0
				m.confirmTarget = 0
				m.statusMsg = ""
				m.confirmGeneration++
				return m, nil, nil
			}
			return m, &AppResult{Action: ActionNone}, nil

		case "enter":
			item, ok := m.list.SelectedItem().(connectionItem)
			if !ok {
				return m, nil, nil
			}

			if m.confirmTarget != item.conn.ID {
				// New target — start confirmation sequence.
				m.confirmTarget = item.conn.ID
				m.confirmCount = 0
			}

			m.confirmCount++
			if m.confirmCount >= 3 {
				// Confirmed — delete from the appropriate source.
				var delErr error
				if item.conn.Source == model.SourceSSHConfig {
					delErr = sshconfig.Delete(m.sshConfigDest, item.conn.Name)
				} else {
					delErr = db.Delete(m.database, item.conn.Name)
				}
				if delErr != nil {
					m.statusMsg = fmt.Sprintf("Error: %s", delErr)
					m.statusStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
				} else {
					m.lastDeleted = item.conn.Name
					m.statusMsg = ""
					m.list.RemoveItem(m.list.Index())
					for i, ai := range m.allItems {
						if ci, ok := ai.(connectionItem); ok && ci.conn.Name == item.conn.Name {
							m.allItems = append(m.allItems[:i], m.allItems[i+1:]...)
							break
						}
					}
				}
				m.confirmCount = 0
				m.confirmTarget = 0
				m.confirmGeneration++
				return m, nil, nil
			}

			// Show remaining presses and start a 1-second timer.
			// Each press restarts the timer — if it expires, the sequence resets.
			m.confirmGeneration++
			gen := m.confirmGeneration
			remaining := 3 - m.confirmCount
			if remaining == 1 {
				m.statusMsg = fmt.Sprintf("Press Enter 1 more time to delete %q", item.conn.Name)
				m.statusStyle = lipgloss.NewStyle().Foreground(warnOrange).Bold(true)
			} else {
				m.statusMsg = fmt.Sprintf("Press Enter %d more times to delete %q", remaining, item.conn.Name)
				m.statusStyle = lipgloss.NewStyle().Foreground(warnYellow).Bold(true)
			}
			return m, nil, tea.Tick(time.Second, func(time.Time) tea.Msg {
				return confirmExpiredMsg{generation: gen}
			})

		case "up", "down", "pgup", "pgdown":
			prevIndex := m.list.Index()
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			if m.list.Index() != prevIndex {
				m.confirmCount = 0
				m.confirmTarget = 0
				m.statusMsg = ""
				m.confirmGeneration++
			}
			return m, nil, cmd
		}

		// All other keys go to the filter.
		prevVal := m.filterBox.Value()
		var cmd tea.Cmd
		m.filterBox, cmd = m.filterBox.Update(msg)
		if m.filterBox.Value() != prevVal {
			m.applyFilter()
			// Reset confirmation when filter changes.
			m.confirmCount = 0
			m.confirmTarget = 0
			m.statusMsg = ""
			m.confirmGeneration++
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
func (m *DeleteModel) applyFilter() {
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

func (m DeleteModel) View() string {
	titleStyle := paneTitleStyle(m.active)
	if m.active {
		titleStyle = titleStyle.Foreground(warnOrange)
	}
	title := titleStyle.Render("Delete Connection")
	listView := connectionListView(m.list, m.filterBox.Value() != "" || m.groupNames != nil)
	return lipgloss.JoinVertical(lipgloss.Left, title, m.filterBox.View(), "", listView)
}

func (m *DeleteModel) SetActive(active bool) {
	m.active = active
	m.filterBox.SetActive(active)
	m.filterBox.SetAccentColor(warnOrange)
	accent := purple
	if active {
		accent = warnOrange
	}
	m.list.SetDelegate(connectionListDelegate(accent))
}

func (m *DeleteModel) SetFilterValue(value string) {
	m.filterBox.SetValue(value)
	m.applyFilter()
}

func (m DeleteModel) FilterValue() string       { return m.filterBox.Value() }
func (m DeleteModel) SelectedName() string      { return selectedConnectionName(m.list) }
func (m *DeleteModel) SelectByName(name string) { selectConnectionByName(&m.list, name) }

func (m *DeleteModel) SetGroupNames(names map[string]bool) {
	m.groupNames = names
	m.applyFilter()
}

func (m *DeleteModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.filterBox.SetWidth(w)
	m.list.SetSize(w, max(1, h-6))
}

// AddItem inserts a connection in ascending alphabetical position.
func (m *DeleteModel) AddItem(conn model.Connection) {
	m.allItems = insertItemSorted(m.allItems, conn)
	m.applyFilter()
}

// SetItems replaces the full connection list (used for source toggling).
func (m *DeleteModel) SetItems(items []connectionItem) {
	listItems := make([]list.Item, len(items))
	for i, ci := range items {
		listItems[i] = ci
	}
	m.allItems = listItems
	m.applyFilter()
}

// ResetConfirm clears the delete confirmation sequence.
func (m *DeleteModel) ResetConfirm() {
	m.confirmCount = 0
	m.confirmTarget = 0
	m.confirmGeneration++
	m.statusMsg = ""
}

// RemoveByName removes the first item matching the given connection name.
func (m *DeleteModel) RemoveByName(name string) {
	for i, item := range m.allItems {
		if ci, ok := item.(connectionItem); ok && ci.conn.Name == name {
			m.allItems = append(m.allItems[:i], m.allItems[i+1:]...)
			break
		}
	}
	m.applyFilter()
}
