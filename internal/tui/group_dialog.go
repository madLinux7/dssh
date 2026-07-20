package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/madLinux7/dssh/internal/model"
)

type GroupNameMode int

const (
	GroupNameCreate GroupNameMode = iota
	GroupNameRename
)

type GroupNameResult struct {
	Name           string
	GroupID        int64
	Mode           GroupNameMode
	FromAssignment bool
	Cancelled      bool
}

type GroupNameDialog struct {
	input          textinput.Model
	mode           GroupNameMode
	groupID        int64
	fromAssignment bool
	errMsg         string
}

func newGroupNameDialog(mode GroupNameMode, group model.Group, fromAssignment bool) GroupNameDialog {
	input := textinput.New()
	input.Placeholder = "group name"
	input.CharLimit = 64
	input.Width = 30
	input.Prompt = ""
	input.PromptStyle = focusedFieldStyle
	input.TextStyle = focusedFieldStyle
	input.Cursor.Style = focusedFieldStyle
	input.SetValue(group.Name)
	input.CursorEnd()
	input.Focus()
	return GroupNameDialog{input: input, mode: mode, groupID: group.ID, fromAssignment: fromAssignment}
}

func (m GroupNameDialog) Update(msg tea.KeyMsg) (GroupNameDialog, *GroupNameResult, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, &GroupNameResult{Cancelled: true}, nil
	case "enter":
		return m, &GroupNameResult{
			Name:           m.input.Value(),
			GroupID:        m.groupID,
			Mode:           m.mode,
			FromAssignment: m.fromAssignment,
		}, nil
	}
	m.errMsg = ""
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, nil, cmd
}

func (m *GroupNameDialog) SetError(err error) { m.errMsg = err.Error() }

func (m GroupNameDialog) BoxView() string {
	title, button := "Create Group", "[ Create ]"
	if m.mode == GroupNameRename {
		title, button = "Rename Group", "[ Rename ]"
	}
	var b strings.Builder
	b.WriteString(modalTitleStyle.Render(title))
	b.WriteString("\n\n")
	b.WriteString(modalLabelStyle.Render("Name"))
	b.WriteString(" ")
	b.WriteString(m.input.View())
	if m.errMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(modalErrorStyle.Render(m.errMsg))
	}
	b.WriteString("\n\n")
	b.WriteString(focusedFieldStyle.Render(button))
	b.WriteString("\n\n")
	b.WriteString(statusStyle.Render("ESC cancel • ENTER confirm"))
	return modalBoxStyle.Render(b.String())
}

type GroupDeleteResult struct{ Confirmed bool }

type GroupDeleteDialog struct{ group model.Group }

func newGroupDeleteDialog(group model.Group) GroupDeleteDialog {
	return GroupDeleteDialog{group: group}
}

func (m GroupDeleteDialog) Update(msg tea.KeyMsg) (GroupDeleteDialog, *GroupDeleteResult) {
	switch msg.String() {
	case "esc":
		return m, &GroupDeleteResult{Confirmed: false}
	case "enter":
		return m, &GroupDeleteResult{Confirmed: true}
	}
	return m, nil
}

func (m GroupDeleteDialog) BoxView() string {
	var b strings.Builder
	b.WriteString(modalTitleStyle.Render("Delete Group"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Delete %q?", m.group.Name))
	b.WriteString("\n")
	b.WriteString(statusStyle.Render("Connections will not be deleted."))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(warnRed).Bold(true).Render("[ Delete ]"))
	b.WriteString("\n\n")
	b.WriteString(statusStyle.Render("ESC cancel • ENTER confirm"))
	return modalBoxStyle.Copy().BorderForeground(warnRed).Render(b.String())
}
