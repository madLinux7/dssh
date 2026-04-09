package model

import "strings"

// Hint fragments for navigation/status bars.
const (
	HintNav             = "↑/↓ navigate"
	HintEscCancel       = "ESC cancel"
	HintEscBack         = "ESC back"
	HintEnterSelect     = "ENTER select"
	HintEnterNextSave   = "ENTER next/save"
	HintEnterNextSubmit = "ENTER next/submit"
	HintToggleAuth      = "CTRL+T toggle auth"
)

// BuildHints joins hint fragments with " • " separators.
func BuildHints(parts ...string) string {
	return strings.Join(parts, " • ")
}
