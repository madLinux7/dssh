package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// WizardResult holds the data collected by the wizard form.
type WizardResult struct {
	Name         string
	User         string
	Host         string
	Port         string
	AuthType     string // "key" or "password"
	IdentityFile string
	Password     string
	Submitted    bool
}

// RunWizard displays an alt-screen form for creating a new connection.
func RunWizard() *WizardResult {
	result := &WizardResult{
		Port:     "22",
		AuthType: "key",
	}

	app := tview.NewApplication()
	form := tview.NewForm()

	form.SetBorder(true).
		SetTitle(" dssh — New Connection ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderPadding(1, 1, 2, 2)

	form.SetBackgroundColor(tcell.ColorBlack)
	form.SetFieldBackgroundColor(tcell.ColorNavy)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetLabelColor(tcell.ColorWhite)
	form.SetButtonBackgroundColor(tcell.ColorNavy)
	form.SetButtonTextColor(tcell.ColorWhite)

	var usePwAuth bool

	form.AddInputField("Name:", "", 30, nil, func(t string) { result.Name = t })
	form.AddInputField("User:", "", 30, nil, func(t string) { result.User = t })
	form.AddInputField("Host:", "", 30, nil, func(t string) { result.Host = t })
	form.AddInputField("Port:", "22", 10, nil, func(t string) { result.Port = t })
	form.AddCheckbox("Password auth:", false, func(checked bool) {
		usePwAuth = checked
	})
	form.AddInputField("Identity File:", "default", 40, nil, func(t string) { result.IdentityFile = t })
	form.AddPasswordField("Password:", "", 30, '*', func(t string) { result.Password = t })

	form.AddButton("Save", func() {
		if usePwAuth {
			result.AuthType = "password"
		} else {
			result.AuthType = "key"
		}
		result.Submitted = true
		app.Stop()
	})
	form.AddButton("Cancel", func() {
		app.Stop()
	})

	// Up/Down to move between fields, Escape to cancel.
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			app.Stop()
			return nil
		case tcell.KeyUp:
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
		case tcell.KeyDown:
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		}
		return event
	})

	if err := app.SetRoot(form, true).EnableMouse(false).Run(); err != nil {
		return nil
	}

	return result
}
