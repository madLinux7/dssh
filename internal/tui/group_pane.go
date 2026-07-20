package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/madLinux7/dssh/internal/model"
)

type groupListItem struct {
	group model.GroupWithCount
	all   bool
}

func (i groupListItem) Title() string {
	if i.all {
		return "(No Groups)"
	}
	return i.group.Name
}

func (i groupListItem) Description() string {
	if i.all {
		return "Show all connections"
	}
	if i.group.ConnectionCount == 1 {
		return "1 connection"
	}
	return fmt.Sprintf("%d connections", i.group.ConnectionCount)
}

func (i groupListItem) FilterValue() string { return i.group.Name }

type GroupPaneModel struct {
	list      list.Model
	filterBox FilterBox
	groups    []model.GroupWithCount
	width     int
	height    int
	active    bool
}

func newGroupPaneModel(groups []model.GroupWithCount, width, height int) GroupPaneModel {
	delegate := groupListDelegate(magenta)
	l := list.New(nil, delegate, width, max(1, height-6))
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	m := GroupPaneModel{
		list:      l,
		filterBox: NewFilterBox(width, magenta),
		width:     width,
		height:    height,
		active:    true,
	}
	m.SetGroups(groups)
	return m
}

func groupListDelegate(accent lipgloss.Color) list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(accent).
		BorderLeftForeground(accent)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(dimGray).
		BorderLeftForeground(accent)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Foreground(dimGray)
	return delegate
}

func (m GroupPaneModel) Update(msg tea.Msg) (GroupPaneModel, tea.Cmd) {
	if len(m.groups) == 0 || !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "down", "pgup", "pgdown", "home", "end":
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		previous := m.filterBox.Value()
		var cmd tea.Cmd
		m.filterBox, cmd = m.filterBox.Update(msg)
		if m.filterBox.Value() != previous {
			m.applyFilter()
		}
		return m, cmd
	}

	var listCmd, filterCmd tea.Cmd
	m.list, listCmd = m.list.Update(msg)
	m.filterBox, filterCmd = m.filterBox.Update(msg)
	return m, tea.Batch(listCmd, filterCmd)
}

func (m GroupPaneModel) View() string {
	title := paneTitleStyle(m.active).Render("Connection Groups")
	if len(m.groups) == 0 {
		empty := lipgloss.NewStyle().Italic(true).Render("No existing groups yet")
		return lipgloss.JoinVertical(lipgloss.Left, title, "", empty)
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, m.filterBox.View(), "", m.list.View())
}

func (m *GroupPaneModel) SetGroups(groups []model.GroupWithCount) {
	selectedID := m.SelectedGroupID()
	m.groups = append(m.groups[:0], groups...)
	m.applyFilter()
	m.SelectGroup(selectedID)
}

func (m *GroupPaneModel) applyFilter() {
	selectedID := m.SelectedGroupID()
	items := []list.Item{groupListItem{all: true}}
	query := strings.ToLower(m.filterBox.Value())
	for _, group := range m.groups {
		if query == "" || strings.Contains(strings.ToLower(group.Name), query) {
			items = append(items, groupListItem{group: group})
		}
	}
	m.list.SetItems(items)
	m.SelectGroup(selectedID)
}

func (m GroupPaneModel) SelectedGroupID() int64 {
	item, ok := m.list.SelectedItem().(groupListItem)
	if !ok || item.all {
		return 0
	}
	return item.group.ID
}

func (m GroupPaneModel) SelectedGroup() (model.GroupWithCount, bool) {
	item, ok := m.list.SelectedItem().(groupListItem)
	if !ok || item.all {
		return model.GroupWithCount{}, false
	}
	return item.group, true
}

func (m *GroupPaneModel) SelectGroup(id int64) {
	for index, item := range m.list.Items() {
		groupItem, ok := item.(groupListItem)
		if !ok {
			continue
		}
		if (id == 0 && groupItem.all) || (!groupItem.all && groupItem.group.ID == id) {
			m.list.Select(index)
			return
		}
	}
	m.list.Select(0)
}

func (m *GroupPaneModel) SetActive(active bool) {
	m.active = active
	m.filterBox.SetActive(active)
	accent := purple
	if active {
		accent = magenta
	}
	m.list.SetDelegate(groupListDelegate(accent))
}

func (m *GroupPaneModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.filterBox.SetWidth(width)
	m.list.SetSize(width, max(1, height-6))
}

func (m GroupPaneModel) SearchValue() string { return m.filterBox.Value() }

func (m *GroupPaneModel) ClearSearch() {
	m.filterBox.SetValue("")
	m.applyFilter()
}
