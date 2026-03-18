package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/madLinux7/dssh-launcher/internal/model"
	"github.com/rivo/tview"
)

// RunPicker displays an inline alt-screen list for selecting a connection.
// Returns the selected connection or nil if the user cancelled.
func RunPicker(connections []model.Connection) *model.Connection {
	var selected *model.Connection

	app := tview.NewApplication()
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true)

	list.SetBorder(true).
		SetTitle(" dssh — Select Connection ").
		SetTitleAlign(tview.AlignLeft)

	for i, conn := range connections {
		c := connections[i] // capture for closure
		list.AddItem(conn.DisplayLabel(), "", 0, func() {
			selected = &c
			app.Stop()
		})
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			app.Stop()
			return nil
		}
		return event
	})

	if err := app.SetRoot(list, true).EnableMouse(false).Run(); err != nil {
		return nil
	}

	return selected
}
