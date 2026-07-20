package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/madLinux7/dssh/internal/model"
)

type assignmentItem struct {
	group   model.Group
	checked bool
}

func (i assignmentItem) Title() string {
	marker := "☐"
	if i.checked {
		marker = "☑"
	}
	return marker + " " + i.group.Name
}

func (i assignmentItem) Description() string { return "" }
func (i assignmentItem) FilterValue() string { return i.group.Name }

type GroupAssignmentModel struct {
	list     list.Model
	groups   []model.Group
	selected map[int64]bool
	width    int
	height   int
	active   bool
}

func newGroupAssignmentModel(groups []model.Group, selectedIDs []int64, initialGroupID int64, width, height int) GroupAssignmentModel {
	delegate := assignmentListDelegate(magenta)
	l := list.New(nil, delegate, width, height-2)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	m := GroupAssignmentModel{
		list:     l,
		selected: make(map[int64]bool),
		width:    width,
		height:   height,
		active:   true,
	}
	m.Begin(groups, selectedIDs, initialGroupID)
	return m
}

func assignmentListDelegate(accent lipgloss.Color) list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)
	bullet := lipgloss.NormalBorder()
	bullet.Left = "•"
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(accent).
		BorderStyle(bullet).
		BorderLeftForeground(accent)
	return delegate
}

func (m GroupAssignmentModel) Update(msg tea.Msg) (GroupAssignmentModel, tea.Cmd) {
	if !m.active || len(m.groups) == 0 {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case " ":
			item, ok := m.list.SelectedItem().(assignmentItem)
			if ok {
				m.selected[item.group.ID] = !m.selected[item.group.ID]
				m.rebuildItems(item.group.ID)
			}
			return m, nil
		case "up", "down", "pgup", "pgdown", "home", "end":
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m GroupAssignmentModel) View() string {
	title := paneTitleStyle(m.active).Render("Assign Groups")
	if len(m.groups) == 0 {
		empty := lipgloss.NewStyle().Italic(true).Render("No existing groups yet")
		return lipgloss.JoinVertical(lipgloss.Left, title, "", empty)
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, m.list.View())
}

func (m *GroupAssignmentModel) Begin(groups []model.Group, selectedIDs []int64, initialGroupID int64) {
	m.groups = append(m.groups[:0], groups...)
	sort.SliceStable(m.groups, func(i, j int) bool {
		return strings.ToLower(m.groups[i].Name) > strings.ToLower(m.groups[j].Name)
	})
	m.selected = make(map[int64]bool, len(selectedIDs))
	for _, id := range selectedIDs {
		m.selected[id] = true
	}
	m.rebuildItems(initialGroupID)
}

func (m *GroupAssignmentModel) SetGroups(groups []model.Group) {
	cursorID := m.CursorGroupID()
	m.groups = append(m.groups[:0], groups...)
	sort.SliceStable(m.groups, func(i, j int) bool {
		return strings.ToLower(m.groups[i].Name) > strings.ToLower(m.groups[j].Name)
	})
	valid := make(map[int64]struct{}, len(m.groups))
	for _, group := range m.groups {
		valid[group.ID] = struct{}{}
	}
	for id := range m.selected {
		if _, ok := valid[id]; !ok {
			delete(m.selected, id)
		}
	}
	m.rebuildItems(cursorID)
}

func (m *GroupAssignmentModel) SelectAndCheck(id int64) {
	m.selected[id] = true
	m.rebuildItems(id)
}

func (m *GroupAssignmentModel) SelectCursor(id int64) {
	m.rebuildItems(id)
}

func (m GroupAssignmentModel) SelectedGroupIDs() []int64 {
	ids := make([]int64, 0, len(m.selected))
	for _, group := range m.groups {
		if m.selected[group.ID] {
			ids = append(ids, group.ID)
		}
	}
	return ids
}

func (m GroupAssignmentModel) CursorGroupID() int64 {
	item, ok := m.list.SelectedItem().(assignmentItem)
	if !ok {
		return 0
	}
	return item.group.ID
}

func (m *GroupAssignmentModel) rebuildItems(cursorID int64) {
	items := make([]list.Item, 0, len(m.groups))
	for _, group := range m.groups {
		items = append(items, assignmentItem{group: group, checked: m.selected[group.ID]})
	}
	m.list.SetItems(items)
	for i, item := range items {
		if item.(assignmentItem).group.ID == cursorID {
			m.list.Select(i)
			return
		}
	}
	if len(items) > 0 {
		m.list.Select(0)
	}
}

func (m *GroupAssignmentModel) SetActive(active bool) {
	m.active = active
	accent := purple
	if active {
		accent = magenta
	}
	m.list.SetDelegate(assignmentListDelegate(accent))
}

func (m *GroupAssignmentModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, max(1, height-2))
}
