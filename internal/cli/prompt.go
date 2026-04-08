package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	promptSelectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D946EF"))
	promptUnselectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C5C5C"))
	promptHintStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C5C5C")).Italic(true)
)

// radioPrompt displays a radio-button selection in the terminal and returns
// the index of the chosen option, or -1 on cancel.
func radioPrompt(title string, options []string) int {
	m := radioModel{title: title, options: options}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return -1
	}
	if fm, ok := final.(radioModel); ok && fm.confirmed {
		return fm.cursor
	}
	return -1
}

type radioModel struct {
	title     string
	options   []string
	cursor    int
	confirmed bool
	cancelled bool
}

func (m radioModel) Init() tea.Cmd { return nil }

func (m radioModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			m.confirmed = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m radioModel) View() string {
	if m.cancelled || m.confirmed {
		return ""
	}
	s := m.title + "\n\n"
	for i, opt := range m.options {
		if i == m.cursor {
			s += fmt.Sprintf("  %s %s\n", promptSelectedStyle.Render("◉"), promptSelectedStyle.Render(opt))
		} else {
			s += fmt.Sprintf("  %s %s\n", promptUnselectedStyle.Render("◯"), promptUnselectedStyle.Render(opt))
		}
	}
	s += "\n" + promptHintStyle.Render("↑/↓ navigate • enter select • esc cancel")
	return s
}
