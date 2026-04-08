package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/madLinux7/dssh/internal/model"
)

const (
	fieldName = iota
	fieldUser
	fieldHost
	fieldPort
	fieldDirectory
	fieldIdentityFile
	fieldPassword
	fieldCount
)

// CreateModel is the Bubble Tea model for the New tab.
type CreateModel struct {
	inputs   [fieldCount]textinput.Model
	focused  int
	authType string // "key" or "password"
	saveTo   model.SaveTarget // only relevant when cfg.ParseMode == "both"
	atSaveTo bool             // cursor is on Save To toggle
	atSave   bool
	cfg      *model.RuntimeConfig
	width    int
	height   int
}

func newCreateModel(width, height int) CreateModel {
	var inputs [fieldCount]textinput.Model

	inputs[fieldName] = textinput.New()
	inputs[fieldName].Placeholder = "(required)"
	inputs[fieldName].CharLimit = 64
	inputs[fieldName].Width = 30
	inputs[fieldName].Focus()
	inputs[fieldName].PromptStyle = focusedFieldStyle
	inputs[fieldName].TextStyle = focusedFieldStyle

	inputs[fieldUser] = textinput.New()
	inputs[fieldUser].Placeholder = "root"
	inputs[fieldUser].CharLimit = 64
	inputs[fieldUser].Width = 30

	inputs[fieldHost] = textinput.New()
	inputs[fieldHost].Placeholder = "(required)"
	inputs[fieldHost].CharLimit = 255
	inputs[fieldHost].Width = 30

	inputs[fieldPort] = textinput.New()
	inputs[fieldPort].Placeholder = "22"
	inputs[fieldPort].CharLimit = 5
	inputs[fieldPort].Width = 10

	inputs[fieldDirectory] = textinput.New()
	inputs[fieldDirectory].Placeholder = "(default)"
	inputs[fieldDirectory].CharLimit = 255
	inputs[fieldDirectory].Width = 40

	inputs[fieldIdentityFile] = textinput.New()
	inputs[fieldIdentityFile].Placeholder = "(default)"
	inputs[fieldIdentityFile].CharLimit = 255
	inputs[fieldIdentityFile].Width = 40

	inputs[fieldPassword] = textinput.New()
	inputs[fieldPassword].Placeholder = "password"
	inputs[fieldPassword].EchoMode = textinput.EchoPassword
	inputs[fieldPassword].EchoCharacter = '•'
	inputs[fieldPassword].CharLimit = 128
	inputs[fieldPassword].Width = 30

	return CreateModel{
		inputs:   inputs,
		focused:  fieldName,
		authType: "key",
		saveTo:   model.SaveTargetSQLite,
		width:    width,
		height:   height,
	}
}

// reset clears all fields and returns focus to the name field.
func (m CreateModel) reset() CreateModel {
	for i := range m.inputs {
		m.inputs[i].SetValue("")
	}
	m.inputs[fieldPort].SetValue("22")
	m.inputs[fieldIdentityFile].SetValue("default")
	m.focused = fieldName
	m.atSave = false
	m.atSaveTo = false
	m.authType = "key"
	if m.cfg != nil {
		m.saveTo = m.cfg.DefaultSaveTarget
	} else {
		m.saveTo = model.SaveTargetSQLite
	}
	// In ssh_config_only mode, force key auth.
	if m.cfg != nil && m.cfg.ParseMode == model.ParseModeSSHConfigOnly {
		m.authType = "key"
	}
	m = m.updateFocus()
	return m
}

// visibleFields returns indices of fields visible for the current auth type.
func (m CreateModel) visibleFields() []int {
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

// showSaveTo returns true when the Save To toggle should be visible.
func (m CreateModel) showSaveTo() bool {
	return m.cfg != nil && m.cfg.ParseMode == model.ParseModeBoth
}

func (m CreateModel) nextField() (CreateModel, bool) {
	if m.atSave {
		return m, true
	}
	if m.atSaveTo {
		m.atSaveTo = false
		m.atSave = true
		return m, true
	}
	fields := m.visibleFields()
	for i, f := range fields {
		if f == m.focused {
			if i+1 < len(fields) {
				m.focused = fields[i+1]
				return m, false
			}
			// Past last text field — go to Save To or Save button.
			if m.showSaveTo() {
				m.atSaveTo = true
				return m, false
			}
			m.atSave = true
			return m, true
		}
	}
	return m, m.atSave
}

func (m CreateModel) prevField() (CreateModel, bool) {
	if m.atSave {
		if m.showSaveTo() {
			m.atSave = false
			m.atSaveTo = true
			return m, false
		}
		fields := m.visibleFields()
		m.atSave = false
		m.focused = fields[len(fields)-1]
		return m, false
	}
	if m.atSaveTo {
		m.atSaveTo = false
		fields := m.visibleFields()
		m.focused = fields[len(fields)-1]
		return m, false
	}
	fields := m.visibleFields()
	for i, f := range fields {
		if f == m.focused {
			if i > 0 {
				m.focused = fields[i-1]
				return m, false
			}
			return m, false
		}
	}
	return m, false
}

func (m CreateModel) Update(msg tea.Msg) (CreateModel, *AppResult, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, &AppResult{Action: ActionNone}, nil

		case "down":
			m, _ = m.nextField()
			m = m.updateFocus()
			return m, nil, nil

		case "up":
			m, _ = m.prevField()
			m = m.updateFocus()
			return m, nil, nil

		case "left", "right":
			// Toggle Save To when focused on it.
			if m.atSaveTo {
				m = m.toggleSaveTo()
				return m, nil, nil
			}

		case "ctrl+t":
			// Don't allow toggling to password when in ssh_config_only mode.
			if m.cfg != nil && m.cfg.ParseMode == model.ParseModeSSHConfigOnly {
				return m, nil, nil
			}
			if m.authType == "key" {
				m.authType = "password"
				if m.focused == fieldIdentityFile {
					m.focused = fieldPassword
					m = m.updateFocus()
				}
				// In both mode: password forces Save To = SQLite.
				if m.showSaveTo() {
					m.saveTo = model.SaveTargetSQLite
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
			if m.atSaveTo {
				m = m.toggleSaveTo()
				return m, nil, nil
			}
			if m.atSave {
				saveTo := m.saveTo
				if m.cfg != nil && m.cfg.ParseMode == model.ParseModeSSHConfigOnly {
					saveTo = model.SaveTargetSSHConfig
				} else if m.cfg != nil && m.cfg.ParseMode == model.ParseModeSQLiteOnly {
					saveTo = model.SaveTargetSQLite
				}
				return m, &AppResult{
					Action: ActionCreated,
					WizardResult: &WizardResult{
						Name:         m.inputs[fieldName].Value(),
						User:         m.inputs[fieldUser].Value(),
						Host:         m.inputs[fieldHost].Value(),
						Port:         m.inputs[fieldPort].Value(),
						Directory:    m.inputs[fieldDirectory].Value(),
						AuthType:     m.authType,
						IdentityFile: m.inputs[fieldIdentityFile].Value(),
						Password:     m.inputs[fieldPassword].Value(),
						SaveTo:       saveTo,
					},
				}, nil
			}
			m, _ = m.nextField()
			m = m.updateFocus()
			return m, nil, nil
		}
	}

	if !m.atSave && !m.atSaveTo {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, nil, cmd
	}
	return m, nil, nil
}

func (m CreateModel) toggleSaveTo() CreateModel {
	if m.saveTo == model.SaveTargetSQLite {
		// Don't allow ssh_config when password auth is selected.
		if m.authType == "password" {
			return m
		}
		m.saveTo = model.SaveTargetSSHConfig
	} else {
		m.saveTo = model.SaveTargetSQLite
	}
	// If saving to ssh_config, lock auth to key.
	if m.saveTo == model.SaveTargetSSHConfig && m.authType == "password" {
		m.authType = "key"
	}
	return m
}

func (m CreateModel) updateFocus() CreateModel {
	for i := range m.inputs {
		if i == m.focused && !m.atSave && !m.atSaveTo {
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

func (m CreateModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("New Connection"))
	b.WriteString("\n\n")

	labels := [fieldCount]string{"Name", "User", "Host", "Port", "Directory", "Identity File", "Password"}

	for _, i := range m.visibleFields() {
		label := labelStyle.Render(labels[i])
		b.WriteString(fmt.Sprintf("%s %s\n", label, m.inputs[i].View()))
	}

	b.WriteString("\n")

	// Auth Type line.
	authLabel := labelStyle.Render("Auth Type")
	authHint := blurredFieldStyle.Render("  (ctrl+t to toggle)")
	isSSHConfigOnly := m.cfg != nil && m.cfg.ParseMode == model.ParseModeSSHConfigOnly
	if isSSHConfigOnly {
		authHint = statusStyle.Render("  (locked: no passwords in ssh_config mode)")
	}
	b.WriteString(fmt.Sprintf("%s %s%s\n", authLabel, focusedFieldStyle.Render(m.authType), authHint))

	// Save To (only in "both" mode).
	if m.showSaveTo() {
		b.WriteString("\n")
		saveLabel := labelStyle.Render("Save To")
		pad := labelStyle.Render("")

		sqliteRadio, sshRadio := "○", "○"
		sqliteStyle, sshStyle := blurredFieldStyle, blurredFieldStyle
		if m.saveTo == model.SaveTargetSQLite {
			sqliteRadio = "●"
			sqliteStyle = focusedFieldStyle
		} else {
			sshRadio = "●"
			sshStyle = focusedFieldStyle
		}
		if m.atSaveTo {
			sqliteStyle = focusedFieldStyle
			sshStyle = focusedFieldStyle
		}

		hint := ""
		if m.authType == "password" {
			hint = statusStyle.Render("  (locked: passwords only in SQLite)")
		}
		b.WriteString(fmt.Sprintf("%s %s %s%s\n", saveLabel, sqliteStyle.Render(sqliteRadio), sqliteStyle.Render("SQLite"), hint))
		b.WriteString(fmt.Sprintf("%s %s %s\n", pad, sshStyle.Render(sshRadio), sshStyle.Render("ssh_config")))
	}

	b.WriteString("\n")

	if m.atSave {
		b.WriteString(focusedFieldStyle.Render("[ Save ]"))
	} else {
		b.WriteString(blurredFieldStyle.Render("[ Save ]"))
	}

	b.WriteString("\n\n")
	hints := "esc: cancel  |  up/down: navigate  |  ctrl+t: toggle auth  |  enter: next/save"
	b.WriteString(statusStyle.Render(hints))

	return b.String()
}

func (m *CreateModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
