package tui

import "github.com/madLinux7/dssh/internal/model"

// ActionKind describes what action the user took in the TUI.
type ActionKind int

const (
	ActionNone    ActionKind = iota // user quit without action
	ActionConnect                   // user selected a connection
	ActionCreated                   // user submitted the new connection form
	ActionEdited                    // user submitted the edit form (password changed, needs modal)
)

// WizardResult holds data collected from the new-connection form.
type WizardResult struct {
	Name         string
	User         string
	Host         string
	Port         string
	Directory    string
	AuthType     string // "key" or "password"
	IdentityFile string
	ProxyJump    string
	Password     string
	SaveTo       model.SaveTarget // "sqlite" or "ssh_config" (only relevant in "both" mode)
	GroupIDs     []int64
}

// AppResult is returned from the TUI to the CLI layer.
type AppResult struct {
	Action       ActionKind
	Connection   *model.Connection
	WizardResult *WizardResult
}
