package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	fieldName = iota
	fieldUser
	fieldHost
	fieldPort
	fieldIdentityFile
	fieldPassword
	fieldCount
)

// NewModel is the Bubble Tea model for the New (wizard) tab.
type NewModel struct {
	inputs   [fieldCount]textinput.Model
	focused  int
	authType string // "key" or "password"
	atSave   bool
	width    int
	height   int
}

func newNewModel(width, height int) NewModel {
	var inputs [fieldCount]textinput.Model

	inputs[fieldName] = textinput.New()
	inputs[fieldName].Placeholder = "my-server"
	inputs[fieldName].CharLimit = 64
	inputs[fieldName].Width = 30
	inputs[fieldName].Focus()
	inputs[fieldName].PromptStyle = focusedFieldStyle
	inputs[fieldName].TextStyle = focusedFieldStyle

	inputs[fieldUser] = textinput.New()
	inputs[fieldUser].Placeholder = "root"
	inputs[fieldUser].SetValue("root")
	inputs[fieldUser].CharLimit = 64
	inputs[fieldUser].Width = 30

	inputs[fieldHost] = textinput.New()
	inputs[fieldHost].Placeholder = "192.168.1.1"
	inputs[fieldHost].CharLimit = 255
	inputs[fieldHost].Width = 30

	inputs[fieldPort] = textinput.New()
	inputs[fieldPort].Placeholder = "22"
	inputs[fieldPort].SetValue("22")
	inputs[fieldPort].CharLimit = 5
	inputs[fieldPort].Width = 10

	inputs[fieldIdentityFile] = textinput.New()
	inputs[fieldIdentityFile].Placeholder = "~/.ssh/id_rsa"
	inputs[fieldIdentityFile].SetValue("default")
	inputs[fieldIdentityFile].CharLimit = 255
	inputs[fieldIdentityFile].Width = 40

	inputs[fieldPassword] = textinput.New()
	inputs[fieldPassword].Placeholder = "password"
	inputs[fieldPassword].EchoMode = textinput.EchoPassword
	inputs[fieldPassword].EchoCharacter = '•'
	inputs[fieldPassword].CharLimit = 128
	inputs[fieldPassword].Width = 30

	return NewModel{
		inputs:   inputs,
		focused:  fieldName,
		authType: "key",
		width:    width,
		height:   height,
	}
}

// reset clears all fields and returns focus to the name field.
func (m NewModel) reset() NewModel {
	for i := range m.inputs {
		m.inputs[i].SetValue("")
	}
	m.inputs[fieldPort].SetValue("22")
	m.inputs[fieldIdentityFile].SetValue("default")
	m.focused = fieldName
	m.atSave = false
	m.authType = "key"
	m = m.updateFocus()
	return m
}

// visibleFields returns indices of fields visible for the current auth type.
func (m NewModel) visibleFields() []int {
	var fields []int
	for i := 0; i < fieldCount; i++ {
		if m.authType == "password" && i == fieldIdentityFile {
			continue
		}
		if m.authType == "key" && i == fieldPassword {
			continue
		}
		fields = append(fields, i)
	}
	return fields
}

func (m NewModel) nextField() (int, bool) {
	if m.atSave {
		return m.focused, true
	}
	fields := m.visibleFields()
	for i, f := range fields {
		if f == m.focused {
			if i+1 < len(fields) {
				return fields[i+1], false
			}
			return m.focused, true // move to save button
		}
	}
	return m.focused, m.atSave
}

func (m NewModel) prevField() (int, bool) {
	if m.atSave {
		fields := m.visibleFields()
		return fields[len(fields)-1], false
	}
	fields := m.visibleFields()
	for i, f := range fields {
		if f == m.focused {
			if i > 0 {
				return fields[i-1], false
			}
			return m.focused, false
		}
	}
	return m.focused, false
}

func (m NewModel) Update(msg tea.Msg) (NewModel, *AppResult, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, &AppResult{Action: ActionNone}, nil

		case "down":
			next, save := m.nextField()
			m.focused = next
			m.atSave = save
			m = m.updateFocus()
			return m, nil, nil

		case "up":
			prev, save := m.prevField()
			m.focused = prev
			m.atSave = save
			m = m.updateFocus()
			return m, nil, nil

		case "ctrl+t":
			if m.authType == "key" {
				m.authType = "password"
				if m.focused == fieldIdentityFile {
					m.focused = fieldPassword
					m = m.updateFocus()
				}
			} else {
				m.authType = "key"
				if m.focused == fieldPassword {
					m.focused = fieldIdentityFile
					m = m.updateFocus()
				}
			}
			return m, nil, nil

		case "enter":
			if m.atSave {
				return m, &AppResult{
					Action: ActionCreated,
					WizardResult: &WizardResult{
						Name:         m.inputs[fieldName].Value(),
						User:         m.inputs[fieldUser].Value(),
						Host:         m.inputs[fieldHost].Value(),
						Port:         m.inputs[fieldPort].Value(),
						AuthType:     m.authType,
						IdentityFile: m.inputs[fieldIdentityFile].Value(),
						Password:     m.inputs[fieldPassword].Value(),
					},
				}, nil
			}
			next, save := m.nextField()
			m.focused = next
			m.atSave = save
			m = m.updateFocus()
			return m, nil, nil
		}
	}

	if !m.atSave {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, nil, cmd
	}
	return m, nil, nil
}

func (m NewModel) updateFocus() NewModel {
	for i := range m.inputs {
		if i == m.focused && !m.atSave {
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

func (m NewModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("New Connection"))
	b.WriteString("\n\n")

	labels := [fieldCount]string{"Name", "User", "Host", "Port", "Identity File", "Password"}

	for _, i := range m.visibleFields() {
		label := labelStyle.Render(labels[i])
		b.WriteString(fmt.Sprintf("%s %s\n", label, m.inputs[i].View()))
	}

	b.WriteString("\n")
	authLabel := labelStyle.Render("Auth Type")
	authHint := blurredFieldStyle.Render("  (ctrl+t to toggle)")
	b.WriteString(fmt.Sprintf("%s %s%s\n\n", authLabel, focusedFieldStyle.Render(m.authType), authHint))

	if m.atSave {
		b.WriteString(focusedFieldStyle.Render("[ Save ]"))
	} else {
		b.WriteString(blurredFieldStyle.Render("[ Save ]"))
	}

	b.WriteString("\n\n")
	b.WriteString(statusStyle.Render("esc: cancel  |  up/down: navigate  |  ctrl+t: toggle auth  |  enter: next/save"))

	return b.String()
}

func (m *NewModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
