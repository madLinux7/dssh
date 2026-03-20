package tui

import (
	"database/sql"
	"fmt"
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

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(warnRed).
		BorderLeftForeground(warnRed)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(dimGray).
		BorderLeftForeground(warnRed)
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)

	l := list.New(items, delegate, width, height)
	l.Title = "Delete Connection"
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	return DeleteModel{
		list:     l,
		database: database,
		width:    width,
		height:   height,
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
		case "q", "esc":
			return m, &AppResult{Action: ActionNone}, nil

		case "enter":
			item, ok := m.list.SelectedItem().(connectionItem)
			if !ok {
				break
			}

			if m.confirmTarget != item.conn.ID {
				// New target — start confirmation sequence.
				m.confirmTarget = item.conn.ID
				m.confirmCount = 1
				m.confirmGeneration++
				gen := m.confirmGeneration
				m.statusMsg = fmt.Sprintf("  Press Enter 2 more times to delete %q", item.conn.Name)
				m.statusStyle = lipgloss.NewStyle().Foreground(warnYellow).Bold(true)
				return m, nil, tea.Tick(time.Second, func(time.Time) tea.Msg {
					return confirmExpiredMsg{generation: gen}
				})
			}

			m.confirmCount++
			if m.confirmCount >= 3 {
				// Confirmed — delete.
				if err := db.Delete(m.database, item.conn.Name); err != nil {
					m.statusMsg = fmt.Sprintf("  Error: %s", err)
					m.statusStyle = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
				} else {
					m.lastDeleted = item.conn.Name
					m.statusMsg = fmt.Sprintf("  Deleted %q", item.conn.Name)
					m.statusStyle = successStyle
					m.list.RemoveItem(m.list.Index())
				}
				m.confirmCount = 0
				m.confirmTarget = 0
				m.confirmGeneration++
				return m, nil, nil
			}

			remaining := 3 - m.confirmCount
			m.statusMsg = fmt.Sprintf("  Press Enter %d more time(s) to delete %q", remaining, item.conn.Name)
			if m.confirmCount == 2 {
				m.statusStyle = lipgloss.NewStyle().Foreground(warnOrange).Bold(true)
			}
			return m, nil, nil
		}
	}

	// Reset confirmation on cursor movement.
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

func (m DeleteModel) View() string {
	s := m.list.View()
	if m.statusMsg != "" {
		s += "\n" + m.statusStyle.Render(m.statusMsg)
	}
	return s
}

func (m *DeleteModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.list.SetSize(w, h)
}

// AddItem inserts a connection in descending alphabetical position.
func (m *DeleteModel) AddItem(conn model.Connection) {
	insertItemSorted(&m.list, conn)
}
