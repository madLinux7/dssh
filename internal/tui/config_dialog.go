package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/madLinux7/dssh/internal/model"
	"github.com/madLinux7/dssh/internal/sshconfig"
)

// configPage tracks which page the dialog is on.
type configPage int

const (
	configPageMain    configPage = iota // page 1: parse mode selection
	configPageSSHDest                   // page 2: ssh_config target destination
	configPageCreate                    // page 3: confirm file/dir creation
)

// configDialogModel is the Bubble Tea model for the first-run / dssh config dialog.
type configDialogModel struct {
	page     configPage
	cursor   int
	width    int
	height   int
	result   *model.RuntimeConfig
	quitting bool

	// Pending state carried from page 1 to page 2.
	pendingParseMode model.ParseMode

	// Page 2: custom path text input.
	pathInput    textinput.Model
	pathInputErr string

	// Page 3: path that needs creation.
	pendingPath   string
	pendingTarget string // the resolved path to store in config

	version string
}

// RunConfigDialog launches the standalone config dialog. Returns nil on cancel.
func RunConfigDialog(version string) *model.RuntimeConfig {
	ti := textinput.New()
	ti.Placeholder = "/path/to/ssh_config"
	ti.CharLimit = 256
	ti.Width = 40

	m := configDialogModel{
		pathInput: ti,
		version:   version,
	}
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
		switch m.page {
		case configPageMain:
			return m.updateMain(msg)
		case configPageSSHDest:
			return m.updateSSHDest(msg)
		case configPageCreate:
			return m.updateCreate(msg)
		}
	}
	return m, nil
}

// ── Page 1: parse mode ──────────────────────────────────────────────

func (m configDialogModel) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 2 {
			m.cursor++
		}
	case "enter":
		switch m.cursor {
		case 0: // SQLite only
			m.result = &model.RuntimeConfig{
				ParseMode:         model.ParseModeSQLiteOnly,
				DefaultSaveTarget: model.SaveTargetSQLite,
			}
			return m, tea.Quit
		case 1: // ssh_config only → page 2
			m.pendingParseMode = model.ParseModeSSHConfigOnly
			m.page = configPageSSHDest
			m.cursor = 0
			m.pathInput.Reset()
			m.pathInputErr = ""
		case 2: // both → page 2
			m.pendingParseMode = model.ParseModeBoth
			m.page = configPageSSHDest
			m.cursor = 0
			m.pathInput.Reset()
			m.pathInputErr = ""
		}
	}
	return m, nil
}

// ── Page 2: ssh_config destination ──────────────────────────────────

func (m configDialogModel) updateSSHDest(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When the text input is focused, delegate most keys to it.
	if m.cursor == 2 && m.pathInput.Focused() {
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			m.pathInput.Blur()
			return m, nil
		case "up":
			m.pathInput.Blur()
			m.cursor--
			return m, nil
		case "enter":
			return m.confirmSSHDest()
		default:
			var cmd tea.Cmd
			m.pathInput, cmd = m.pathInput.Update(msg)
			return m, cmd
		}
	}

	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.page = configPageMain
		m.cursor = 0
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 2 {
			m.cursor++
		}
		if m.cursor == 2 {
			m.pathInput.Focus()
			return m, textinput.Blink
		}
	case "enter":
		return m.confirmSSHDest()
	}
	return m, nil
}

func (m configDialogModel) confirmSSHDest() (tea.Model, tea.Cmd) {
	var dest string
	switch m.cursor {
	case 0:
		p, err := sshconfig.MainFilePath()
		if err != nil {
			m.pathInputErr = err.Error()
			return m, nil
		}
		dest = p
	case 1:
		p, err := sshconfig.DirectiveFilePath()
		if err != nil {
			m.pathInputErr = err.Error()
			return m, nil
		}
		dest = p
	case 2:
		raw := strings.TrimSpace(m.pathInput.Value())
		if raw == "" {
			m.pathInputErr = "path cannot be empty"
			return m, nil
		}
		// Expand ~ to home directory.
		if strings.HasPrefix(raw, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				raw = filepath.Join(home, raw[2:])
			}
		}
		dest = raw
	}

	// Check if the resolved path exists.
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		m.pendingPath = dest
		m.pendingTarget = dest
		m.page = configPageCreate
		m.cursor = 0
		return m, nil
	}

	m = m.finishWithDest(dest)
	return m, tea.Quit
}

func (m configDialogModel) finishWithDest(dest string) configDialogModel {
	m.result = &model.RuntimeConfig{
		ParseMode:         m.pendingParseMode,
		SSHConfigDest:     dest,
		DefaultSaveTarget: model.SaveTargetSQLite,
		BothViewMode:      model.SourceSQLite,
	}
	return m
}

// ── Page 3: create missing file ─────────────────────────────────────

func (m configDialogModel) updateCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		// Back to destination page.
		m.page = configPageSSHDest
		m.cursor = 0
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 1 {
			m.cursor++
		}
	case "enter":
		if m.cursor == 0 {
			// Create directory and file.
			dir := filepath.Dir(m.pendingPath)
			if err := os.MkdirAll(dir, 0700); err != nil {
				m.pathInputErr = fmt.Sprintf("mkdir failed: %s", err)
				m.page = configPageSSHDest
				return m, nil
			}
			f, err := os.OpenFile(m.pendingPath, os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				m.pathInputErr = fmt.Sprintf("create failed: %s", err)
				m.page = configPageSSHDest
				return m, nil
			}
			f.Close()

			m = m.finishWithDest(m.pendingTarget)
			return m, tea.Quit
		}
		// Go back.
		m.page = configPageSSHDest
		m.cursor = 0
		return m, nil
	}
	return m, nil
}

// ── Views ───────────────────────────────────────────────────────────

func (m configDialogModel) View() string {
	if m.quitting {
		return ""
	}

	var content string
	switch m.page {
	case configPageMain:
		content = m.viewMain()
	case configPageSSHDest:
		content = m.viewSSHDest()
	case configPageCreate:
		content = m.viewCreate()
	}

	return m.renderFrame(content)
}

// renderFrame wraps page content in the tab + content box frame.
func (m configDialogModel) renderFrame(content string) string {
	color := purple

	// Tab — reuse the same style as the main app but with purple.
	tabStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(brightWhite).
		Border(lipgloss.RoundedBorder()).
		BorderBottom(false).
		BorderForeground(color).
		Padding(0, 2)

	rendered := tabStyle.Render("Config")
	lines := strings.Split(rendered, "\n")
	tabTopLine := lines[0]
	tabBotLine := lines[1]
	tabWidth := lipgloss.Width(tabTopLine)

	// Version in top-right corner (on the bottom tab line, like the mode indicator).
	if m.version != "" {
		verStyle := lipgloss.NewStyle().Foreground(dimGray)
		verStr := verStyle.Render(m.version)
		verWidth := lipgloss.Width(verStr)
		gap := m.width - tabWidth - verWidth - 1
		if gap > 0 {
			tabBotLine += strings.Repeat(" ", gap) + verStr
		}
	}

	tabBar := tabTopLine + "\n" + tabBotLine

	// Connecting line: tab bottom merges into content box top border.
	var conn strings.Builder
	colorStyle := lipgloss.NewStyle().Foreground(color)
	conn.WriteRune('│')
	for i := 0; i < tabWidth-2; i++ {
		conn.WriteRune(' ')
	}
	conn.WriteRune('╰')
	for pos := tabWidth; pos < m.width-1; pos++ {
		conn.WriteRune('─')
	}
	conn.WriteRune('╮')
	connLine := colorStyle.Render(conn.String())

	// Content box height.
	contentHeight := m.height - 4
	if contentHeight < 2 {
		contentHeight = 2
	}

	// Pad content to push the hint to the bottom.
	contentLines := lipgloss.Height(content)
	padNeeded := contentHeight - contentLines
	if padNeeded > 0 {
		content += strings.Repeat("\n", padNeeded)
	}

	contentBox := contentStyle.
		BorderForeground(color).
		Width(m.width - 2).
		Height(contentHeight).
		Render(content)

	return lipgloss.JoinVertical(lipgloss.Left,
		tabBar,
		connLine,
		contentBox,
	)
}

// Shared styles for the config dialog pages.
var (
	cfgTitle    = lipgloss.NewStyle().Foreground(magenta).Bold(true)
	cfgSubtitle = lipgloss.NewStyle().Foreground(brightWhite)
	cfgHint     = lipgloss.NewStyle().Foreground(dimGray).Italic(true)
	cfgSel      = lipgloss.NewStyle().Foreground(magenta)
	cfgUnsel    = lipgloss.NewStyle().Foreground(brightWhite)
	cfgErr      = lipgloss.NewStyle().Foreground(warnRed).Bold(true)
	cfgCtrlL    = lipgloss.NewStyle().Foreground(purple).Bold(true)
)

// renderRadioOptions writes radio-button lines for the given labels at the cursor position.
func renderRadioOptions(b *strings.Builder, options []string, cursor int) {
	for i, opt := range options {
		style := cfgUnsel
		radio := "○"
		if i == cursor {
			style = cfgSel
			radio = "●"
		}
		fmt.Fprintf(b, "  %s %s\n", style.Render(radio), style.Render(opt))
	}
}

func (m configDialogModel) viewMain() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(cfgTitle.Render("Welcome to dssh!"))
	b.WriteString("\n\n")
	b.WriteString(cfgSubtitle.Render("Please select the mode to store and work with your connections."))
	b.WriteString("\n")
	fmt.Fprintf(&b, "You can change this anytime using %s", cfgTitle.Render("dssh config"))
	b.WriteString("\n\n")

	renderRadioOptions(&b, []string{
		fmt.Sprintf("Use SQLite only %s", cfgHint.Render("(~/.dssh/dssh.db)")),
		"Use ssh_config only — Select destination →",
		fmt.Sprintf("Use both — Toggle with %s — Select destination →", cfgCtrlL.Render("CTRL+L")),
	}, m.cursor)

	b.WriteString("\n")
	b.WriteString(cfgHint.Render(model.BuildHints(model.HintNav, model.HintEnterSelect, model.HintEscCancel)))
	return b.String()
}

func (m configDialogModel) viewSSHDest() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(cfgTitle.Render("Please select your ssh_config target destination"))
	b.WriteString("\n\n")

	renderRadioOptions(&b, []string{
		fmt.Sprintf("Main File: %s", cfgHint.Render("~/.ssh/config")),
		fmt.Sprintf("Directive: %s", cfgHint.Render("~/.ssh/config.d/dssh")),
		"Enter custom path",
	}, m.cursor)

	b.WriteString("\n")
	inputLabel := cfgUnsel
	if m.cursor == 2 {
		inputLabel = cfgSel
	}
	fmt.Fprintf(&b, "  %s  %s\n", inputLabel.Render("Path:"), m.pathInput.View())

	if m.pathInputErr != "" {
		fmt.Fprintf(&b, "  %s\n", cfgErr.Render(m.pathInputErr))
	}

	b.WriteString("\n")
	b.WriteString(cfgHint.Render(model.BuildHints(model.HintNav, model.HintEnterSelect, model.HintEscBack)))
	return b.String()
}

func (m configDialogModel) viewCreate() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(cfgTitle.Render("File not found"))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "%s\n", cfgSubtitle.Render(fmt.Sprintf("The path %s does not exist.", cfgHint.Render(m.pendingPath))))
	b.WriteString(cfgSubtitle.Render("Would you like to create it?"))
	b.WriteString("\n\n")

	renderRadioOptions(&b, []string{
		"Create file and parent directories",
		"← Go back",
	}, m.cursor)

	b.WriteString("\n")
	b.WriteString(cfgHint.Render(model.BuildHints(model.HintNav, model.HintEnterSelect, model.HintEscBack)))
	return b.String()
}
