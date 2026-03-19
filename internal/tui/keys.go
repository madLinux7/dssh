package tui

import "github.com/charmbracelet/bubbles/key"

type appKeyMap struct {
	NextTab key.Binding
	PrevTab key.Binding
	Quit    key.Binding
}

var appKeys = appKeyMap{
	NextTab: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
	PrevTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev tab")),
	Quit:    key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
}
