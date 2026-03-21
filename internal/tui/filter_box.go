package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FilterBox is a reusable search input rendered inside a bordered box.
type FilterBox struct {
	input textinput.Model
	width int // outer width of the box
}

func NewFilterBox(width int) FilterBox {
	input := textinput.New()
	input.Placeholder = "type to search..."
	input.Prompt = "/ "
	input.CharLimit = 64
	input.Width = filterInputWidth(width)
	input.Focus()
	input.PromptStyle = focusedFieldStyle

	return FilterBox{input: input, width: width}
}

func (f FilterBox) Update(msg tea.Msg) (FilterBox, tea.Cmd) {
	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)
	return f, cmd
}

func (f FilterBox) View() string {
	inner := f.width - 2
	if inner < 0 {
		inner = 0
	}

	gray := lipgloss.NewStyle().Foreground(dimGray)
	top := gray.Render("╭" + strings.Repeat("─", inner) + "╮")

	content := " " + f.input.View()
	visW := lipgloss.Width(content)
	pad := inner - visW
	if pad < 0 {
		pad = 0
	}
	mid := gray.Render("│") + content + strings.Repeat(" ", pad) + gray.Render("│")
	bot := gray.Render("╰" + strings.Repeat("─", inner) + "╯")

	return top + "\n" + mid + "\n" + bot
}

func (f FilterBox) Value() string {
	return f.input.Value()
}

func (f *FilterBox) SetValue(s string) {
	f.input.SetValue(s)
}

func (f *FilterBox) SetWidth(w int) {
	f.width = w
	f.input.Width = filterInputWidth(w)
}

func filterInputWidth(outerWidth int) int {
	// outer border (2) + 1 left pad + prompt "/ " (2) + 1 margin
	w := outerWidth - 6
	if w < 1 {
		return 1
	}
	return w
}
