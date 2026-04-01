package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PassphraseResult is returned when the user submits or cancels the modal.
type PassphraseResult struct {
	Passphrase string
	Cancelled  bool
}

// PassphraseModal is a Bubble Tea sub-model rendered as a centered overlay.
// It has two modes: "create" (first-time setup, two fields with confirmation)
// and "enter" (subsequent use, single field). When active, it captures all
// keyboard input, preventing interaction with the underlying tab.
type PassphraseModal struct {
	isNew    bool // true = create (two fields), false = enter (one field)
	inputs   [2]textinput.Model
	focused  int
	atSubmit bool
	errMsg   string
	width    int
	height   int
}

func newPassphraseModal(isNew bool, width, height int) PassphraseModal {
	var inputs [2]textinput.Model

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "master passphrase"
	inputs[0].EchoMode = textinput.EchoPassword
	inputs[0].EchoCharacter = '*'
	inputs[0].CharLimit = 128
	inputs[0].Width = 30
	inputs[0].Focus()
	inputs[0].PromptStyle = focusedFieldStyle
	inputs[0].TextStyle = focusedFieldStyle

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "confirm passphrase"
	inputs[1].EchoMode = textinput.EchoPassword
	inputs[1].EchoCharacter = '*'
	inputs[1].CharLimit = 128
	inputs[1].Width = 30
	inputs[1].PromptStyle = blurredFieldStyle
	inputs[1].TextStyle = blurredFieldStyle

	return PassphraseModal{
		isNew:  isNew,
		inputs: inputs,
		width:  width,
		height: height,
	}
}

func (m PassphraseModal) fieldCount() int {
	if m.isNew {
		return 2
	}
	return 1
}

func (m PassphraseModal) Update(msg tea.Msg) (PassphraseModal, *PassphraseResult, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, &PassphraseResult{Cancelled: true}, nil

		case "down", "tab":
			m.errMsg = ""
			if !m.atSubmit {
				if m.focused+1 < m.fieldCount() {
					m.focused++
				} else {
					m.atSubmit = true
				}
				m = m.updateFocus()
			}
			return m, nil, nil

		case "up", "shift+tab":
			m.errMsg = ""
			if m.atSubmit {
				m.atSubmit = false
				m.focused = m.fieldCount() - 1
				m = m.updateFocus()
			} else if m.focused > 0 {
				m.focused--
				m = m.updateFocus()
			}
			return m, nil, nil

		case "enter":
			if m.atSubmit {
				return m.submit()
			}
			// Advance to next field or submit button.
			if m.focused+1 < m.fieldCount() {
				m.focused++
			} else {
				m.atSubmit = true
			}
			m = m.updateFocus()
			return m, nil, nil
		}
	}

	if !m.atSubmit {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, nil, cmd
	}
	return m, nil, nil
}

func (m PassphraseModal) submit() (PassphraseModal, *PassphraseResult, tea.Cmd) {
	p1 := m.inputs[0].Value()
	if p1 == "" {
		m.errMsg = "passphrase cannot be empty"
		return m, nil, nil
	}
	if m.isNew {
		p2 := m.inputs[1].Value()
		if p1 != p2 {
			m.errMsg = "passphrases do not match"
			return m, nil, nil
		}
	}
	return m, &PassphraseResult{Passphrase: p1}, nil
}

func (m PassphraseModal) updateFocus() PassphraseModal {
	for i := 0; i < 2; i++ {
		if i == m.focused && !m.atSubmit {
			m.inputs[i].Focus()
			m.inputs[i].PromptStyle = focusedFieldStyle
			m.inputs[i].TextStyle = focusedFieldStyle
		} else {
			m.inputs[i].Blur()
			m.inputs[i].PromptStyle = blurredFieldStyle
			m.inputs[i].TextStyle = blurredFieldStyle
		}
	}
	return m
}

func (m PassphraseModal) View() string {
	var b strings.Builder

	// Title.
	if m.isNew {
		b.WriteString(modalTitleStyle.Render("Create Master Passphrase"))
		b.WriteString("\n")
		b.WriteString(blurredFieldStyle.Render("Don't forget it or you'll need to reset!"))
	} else {
		b.WriteString(modalTitleStyle.Render("Enter Master Passphrase"))
	}
	b.WriteString("\n\n")

	// Input field(s).
	labels := [2]string{"Passphrase", "Confirm"}
	for i := 0; i < m.fieldCount(); i++ {
		label := modalLabelStyle.Render(labels[i])
		b.WriteString(label + " " + m.inputs[i].View() + "\n")
	}

	// Error message.
	if m.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(modalErrorStyle.Render(m.errMsg))
	}

	// Submit button.
	b.WriteString("\n")
	if m.atSubmit {
		b.WriteString(focusedFieldStyle.Render("[ Submit ]"))
	} else {
		b.WriteString(blurredFieldStyle.Render("[ Submit ]"))
	}

	b.WriteString("\n\n")
	b.WriteString(statusStyle.Render("esc: cancel  |  enter: next/submit"))

	inner := b.String()

	// The modal box.
	box := modalBoxStyle.Render(inner)

	// Center the box on screen.
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *PassphraseModal) SetSize(w, h int) {
	m.width = w
	m.height = h
}
