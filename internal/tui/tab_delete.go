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
	database          *sql.DB
	confirmTarget     int64
	confirmCount      int
	confirmGeneration int
	lastDeleted       string // set after successful deletion, read + cleared by app model
	statusMsg         string
	statusStyle       lipgloss.Style
	width             int
	height            int
}

func newDeleteModel(conns []connectionItem, database *sql.DB, width, height int) DeleteModel {
	items := make([]list.Item, len(conns))
	for i, c := range conns {
		items[i] = c
	}

	l := newConnectionList(items, warnRed, width, height)

	return DeleteModel{
		list:      l,
		filterBox: NewFilterBox(width, warnRed),
		allItems:  items,
		database:  database,
		width:     width,
		height:    height,
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
				m.confirmCount = 1
				m.confirmGeneration++
				gen := m.confirmGeneration
				m.statusMsg = fmt.Sprintf("Press Enter 2 more times to delete %q", item.conn.Name)
				m.statusStyle = lipgloss.NewStyle().Foreground(warnYellow).Bold(true)
				return m, nil, tea.Tick(time.Second, func(time.Time) tea.Msg {
					return confirmExpiredMsg{generation: gen}
				})
			}

			m.confirmCount++
			if m.confirmCount >= 3 {
				// Confirmed — delete.
				if err := db.Delete(m.database, item.conn.Name); err != nil {
					m.statusMsg = fmt.Sprintf("Error: %s", err)
					m.statusStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
				} else {
					m.lastDeleted = item.conn.Name
					m.statusMsg = fmt.Sprintf("Deleted %q", item.conn.Name)
					m.statusStyle = successStyle
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

			m.statusMsg = fmt.Sprintf("Press Enter 1 more time to delete %q", item.conn.Name)
			m.statusStyle = lipgloss.NewStyle().Foreground(warnOrange).Bold(true)
			return m, nil, nil

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

func (m DeleteModel) View() string {
	title := titleStyle.Foreground(warnRed).Render("Delete Connection")
	listView := m.list.View()

	if m.statusMsg != "" {
		lines := strings.Split(listView, "\n")
		last := lines[len(lines)-1]
		avail := m.width - lipgloss.Width(last) - 2
		if avail > 0 {
			msg := m.statusMsg
			if len(msg) > avail {
				msg = msg[:avail-1] + "…"
			}
			styled := m.statusStyle.Render(msg)
			pad := m.width - lipgloss.Width(last) - lipgloss.Width(styled)
			if pad < 1 {
				pad = 1
			}
			lines[len(lines)-1] = last + strings.Repeat(" ", pad) + styled
		}
		listView = strings.Join(lines, "\n")
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, m.filterBox.View(), "", listView)
}

func (m *DeleteModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.filterBox.SetWidth(w)
	m.list.SetSize(w, h-4)
}

// AddItem inserts a connection in ascending alphabetical position.
func (m *DeleteModel) AddItem(conn model.Connection) {
	m.allItems = insertItemSorted(m.allItems, conn)
	m.applyFilter()
}
