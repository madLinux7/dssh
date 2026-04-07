package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/madLinux7/dssh/internal/model"
)

// configPage tracks which page the dialog is on.
type configPage int

const (
	configPageMain configPage = iota
	configPageBoth
)

// configDialogModel is the Bubble Tea model for the first-run / dssh config dialog.
type configDialogModel struct {
	page            configPage
	cursor          int
	width           int
	height          int
	result          *model.RuntimeConfig
	quitting        bool
	pendingSSHTarget model.SSHConfigTarget // set when entering page 2
}

// mainOptions is the list of options on page 1.
var mainOptions = []struct {
	label       string
	parseMode   model.ParseMode
	sshTarget   model.SSHConfigTarget
	hasSubPage  bool
}{
	{"Use SQLite only", model.ParseModeSQLiteOnly, "", false},
	{"Use ssh_config only (Main file: ~/.ssh/config)", model.ParseModeSSHConfigOnly, model.SSHConfigTargetMainFile, false},
	{"Use ssh_config only (Directive: ~/.ssh/config.d/dssh)", model.ParseModeSSHConfigOnly, model.SSHConfigTargetDirective, false},
	{"Use both (ssh_config Main File)", model.ParseModeBoth, model.SSHConfigTargetMainFile, true},
	{"Use both (ssh_config Directive)", model.ParseModeBoth, model.SSHConfigTargetDirective, true},
}

var bothOptions = []struct {
	label    string
	bothMode model.BothMode
	goBack   bool
}{
	{"Separated: Toggle between SQLite & ssh_config using CTRL+L", model.BothModeSeparate, false},
	{"Combined: One list for both SQLite & ssh_config", model.BothModeCombine, false},
	{"\u2190 Go back", "", true},
}

// RunConfigDialog launches the standalone config dialog. Returns nil on cancel.
func RunConfigDialog() *model.RuntimeConfig {
	m := configDialogModel{}
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil
	}
	if fm, ok := finalModel.(configDialogModel); ok {
		return fm.result
	}
	return nil
}

func (m configDialogModel) Init() tea.Cmd {
	return nil
}

func (m configDialogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "esc":
			if m.page == configPageBoth {
				m.page = configPageMain
				m.cursor = 0
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			max := m.maxCursor()
			if m.cursor < max {
				m.cursor++
			}
			return m, nil

		case "enter":
			return m.handleSelect()
		}
	}
	return m, nil
}

func (m configDialogModel) maxCursor() int {
	if m.page == configPageBoth {
		return len(bothOptions) - 1
	}
	return len(mainOptions) - 1
}

func (m configDialogModel) handleSelect() (tea.Model, tea.Cmd) {
	if m.page == configPageMain {
		opt := mainOptions[m.cursor]
		if opt.hasSubPage {
			// Store the ssh target for later, move to page 2.
			m.pendingSSHTarget = opt.sshTarget
			m.page = configPageBoth
			m.cursor = 0
			return m, nil
		}
		// Direct selection.
		m.result = &model.RuntimeConfig{
			ParseMode:         opt.parseMode,
			SSHConfigTarget:   opt.sshTarget,
			DefaultSaveTarget: model.SaveTargetSQLite,
			BothMode:          model.BothModeSeparate,
		}
		return m, tea.Quit
	}

	// Page 2: both sub-options.
	opt := bothOptions[m.cursor]
	if opt.goBack {
		m.page = configPageMain
		m.cursor = 0
		return m, nil
	}

	m.result = &model.RuntimeConfig{
		ParseMode:         model.ParseModeBoth,
		BothMode:          opt.bothMode,
		SSHConfigTarget:   m.pendingSSHTarget,
		DefaultSaveTarget: model.SaveTargetSQLite,
	}
	return m, tea.Quit
}

func (m configDialogModel) View() string {
	if m.quitting {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(magenta)).Bold(true)
	subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(brightWhite))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(dimGray)).Italic(true)
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(magenta))
	unselectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(dimGray))

	var b strings.Builder

	if m.page == configPageMain {
		b.WriteString(titleStyle.Render("Welcome to dssh!"))
		b.WriteString("\n\n")
		b.WriteString(subtitleStyle.Render("Please select the mode to store and work with your connections."))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("You can change this anytime using %s", titleStyle.Render("dssh config")))
		b.WriteString("\n\n")

		for i, opt := range mainOptions {
			cursor := "  "
			style := unselectedStyle
			if i == m.cursor {
				cursor = "> "
				style = selectedStyle
			}

			label := opt.label
			if opt.hasSubPage {
				label += " \u2192"
			}

			// Add path hints in italic gray
			switch i {
			case 0:
				label = addPathHint(label, "~/.dssh/dssh.db", hintStyle)
			}

			b.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(label)))
		}
	} else {
		b.WriteString(subtitleStyle.Render("How do you want to navigate both of your connections?"))
		b.WriteString("\n\n")

		for i, opt := range bothOptions {
			cursor := "  "
			style := unselectedStyle
			if i == m.cursor {
				cursor = "> "
				style = selectedStyle
			}

			label := opt.label
			if i == 0 {
				// Highlight CTRL+L in magenta
				label = strings.Replace(label, "CTRL+L", titleStyle.Render("CTRL+L"), 1)
				if i == m.cursor {
					// Re-apply selected style for surrounding text
					parts := strings.SplitN(opt.label, "CTRL+L", 2)
					label = selectedStyle.Render(parts[0]) + titleStyle.Render("CTRL+L") + selectedStyle.Render(parts[1])
				}
			}

			b.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(label)))
		}
	}

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/↓ navigate • enter select • esc cancel"))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}

func addPathHint(label, path string, style lipgloss.Style) string {
	return label + " " + style.Render("("+path+")")
}
